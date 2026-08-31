package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/LumeraProtocol/sdk-go/cascade"
	lumerasdk "github.com/LumeraProtocol/sdk-go/client"
	clientconfig "github.com/LumeraProtocol/sdk-go/client/config"
	sdkcrypto "github.com/LumeraProtocol/sdk-go/pkg/crypto"
	sdktypes "github.com/LumeraProtocol/sdk-go/types"
	"go.uber.org/zap"

	"gen/tests/common"
)

// sdkActionCreator creates CASCADE actions via the Lumera sdk-go client. It
// implements actionCreator. Unlike the migration tooling, it selects the SDK
// key type from each account's recorded key style so EVM (eth_secp256k1/coin-60)
// accounts sign correctly.
type sdkActionCreator struct {
	chainID  string
	grpcAddr string
	rpcAddr  string
	cli      *common.ChainCLI // used for the inter-attempt block wait on sequence retry
	logger   *zap.Logger
}

func newSDKActionCreator(cfg *Config, cli *common.ChainCLI) *sdkActionCreator {
	return &sdkActionCreator{
		chainID:  cfg.ChainID,
		grpcAddr: resolveGRPCAddr(cfg.GRPC, cfg.RPC),
		rpcAddr:  rpcForSDK(cfg.RPC),
		cli:      cli,
		logger:   zap.NewNop(),
	}
}

var _ actionCreator = (*sdkActionCreator)(nil)

// CreateAction creates one CASCADE action in the target state for the account,
// retrying on account-sequence mismatch.
func (s *sdkActionCreator) CreateAction(acct *AccountRecord, state common.ActionState, idx int) (common.ActionActivity, error) {
	if strings.TrimSpace(acct.Mnemonic) == "" {
		return common.ActionActivity{}, fmt.Errorf("account %s has no recorded mnemonic; cannot sign SDK action", acct.Name)
	}
	filePath, cleanup, err := createSampleFile(acct, idx)
	if err != nil {
		return common.ActionActivity{}, err
	}
	defer cleanup()

	keyType := sdkKeyType(acct.KeyStyle)
	var result common.ActionActivity
	err = s.withSequenceRetry(acct, keyType, func(ctx context.Context, client *lumerasdk.Client) error {
		var e error
		result, e = s.createInState(ctx, client, acct, state, filePath)
		return e
	})
	return result, err
}

// withSequenceRetry builds a fresh unified client and runs fn, retrying up to
// three times on account-sequence mismatch with a block wait between attempts.
func (s *sdkActionCreator) withSequenceRetry(acct *AccountRecord, keyType sdkcrypto.KeyType, fn func(context.Context, *lumerasdk.Client) error) error {
	ctx := context.Background()
	var lastErr error
	for attempt := range 3 {
		client, err := s.unifiedClient(ctx, acct, keyType)
		if err != nil {
			return fmt.Errorf("create SDK client for %s: %w", acct.Name, err)
		}
		err = fn(ctx, client)
		_ = client.Close()
		if err == nil {
			return nil
		}
		lastErr = err
		if _, _, ok := common.ParseIncorrectAccountSequence(err); !ok || attempt == 2 {
			return err
		}
		log.Printf("  INFO: retrying SDK action for %s after sequence mismatch (retry %d/2)", acct.Name, attempt+1)
		_ = s.cli.WaitForNextBlock(20 * time.Second)
	}
	return lastErr
}

// unifiedClient builds an sdk-go client backed by an in-memory keyring holding
// the account's key, derived with the matching key type.
func (s *sdkActionCreator) unifiedClient(ctx context.Context, acct *AccountRecord, keyType sdkcrypto.KeyType) (*lumerasdk.Client, error) {
	kr, err := sdkcrypto.NewKeyring(sdkcrypto.KeyringParams{
		AppName: "lumera-gen-activity",
		Backend: "memory",
		Input:   strings.NewReader(""),
	})
	if err != nil {
		return nil, fmt.Errorf("create keyring: %w", err)
	}
	if _, err := kr.NewAccount(acct.Name, acct.Mnemonic, "", keyType.HDPath(), keyType.SigningAlgo()); err != nil {
		return nil, fmt.Errorf("import key %s: %w", acct.Name, err)
	}

	waitCfg := clientconfig.DefaultWaitTxConfig()
	waitCfg.PollInterval = time.Second
	waitCfg.PollMaxRetries = 0

	return lumerasdk.New(ctx, lumerasdk.Config{
		ChainID:           s.chainID,
		GRPCEndpoint:      s.grpcAddr,
		RPCEndpoint:       s.rpcAddr,
		Address:           acct.Address,
		KeyName:           acct.Name,
		BlockchainTimeout: 30 * time.Second,
		StorageTimeout:    5 * time.Minute,
		WaitTx:            waitCfg,
		LogLevel:          "error",
		Logger:            s.logger,
	}, kr)
}

// createInState dispatches to the right cascade flow for the target state and
// returns the recorded activity.
func (s *sdkActionCreator) createInState(ctx context.Context, client *lumerasdk.Client, acct *AccountRecord, state common.ActionState, filePath string) (common.ActionActivity, error) {
	switch state {
	case common.ActionStatePending:
		msg, _, err := client.Cascade.CreateRequestActionMessage(ctx, acct.Address, filePath, &cascade.UploadOptions{Public: true})
		if err != nil {
			return common.ActionActivity{}, fmt.Errorf("create request action message: %w", err)
		}
		ar, err := client.Cascade.SendRequestActionMessage(ctx, client.Blockchain, msg, "", nil)
		if err != nil {
			return common.ActionActivity{}, fmt.Errorf("send request action message: %w", err)
		}
		log.Printf("  %s registered CASCADE action %s (PENDING)", acct.Name, ar.ActionID)
		return common.ActionActivity{
			ActionID: ar.ActionID, ActionType: "CASCADE", Price: msg.Price,
			Expiration: msg.ExpirationTime, State: "ACTION_STATE_PENDING",
			Metadata: msg.Metadata, BlockHeight: ar.Height, CreatedViaSDK: true,
		}, nil

	case common.ActionStateDone:
		result, err := client.Cascade.Upload(ctx, acct.Address, client.Blockchain, filePath, cascade.WithPublic(true))
		if err != nil {
			return common.ActionActivity{}, fmt.Errorf("cascade upload: %w", err)
		}
		log.Printf("  %s uploaded CASCADE action %s (DONE, task=%s)", acct.Name, result.ActionID, result.TaskID)
		return s.recordFromChain(ctx, client, result.ActionID, "ACTION_STATE_DONE", result.Height), nil

	case common.ActionStateApproved:
		result, err := client.Cascade.Upload(ctx, acct.Address, client.Blockchain, filePath, cascade.WithPublic(true))
		if err != nil {
			return common.ActionActivity{}, fmt.Errorf("cascade upload: %w", err)
		}
		doneCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if _, err := client.Blockchain.Action.WaitForState(doneCtx, result.ActionID, sdktypes.ActionStateDone, time.Second); err != nil {
			return common.ActionActivity{}, fmt.Errorf("wait for DONE before approve: %w", err)
		}
		if _, err := client.Blockchain.ApproveActionTx(ctx, acct.Address, result.ActionID, ""); err != nil {
			return common.ActionActivity{}, fmt.Errorf("approve action: %w", err)
		}
		log.Printf("  %s approved CASCADE action %s (APPROVED)", acct.Name, result.ActionID)
		return s.recordFromChain(ctx, client, result.ActionID, "ACTION_STATE_APPROVED", result.Height), nil

	default:
		return common.ActionActivity{}, fmt.Errorf("unsupported target action state %q", state)
	}
}

// recordFromChain builds an ActionActivity, enriching it with on-chain details
// when the action query succeeds (falling back to the provided defaults).
func (s *sdkActionCreator) recordFromChain(ctx context.Context, client *lumerasdk.Client, actionID, defaultState string, defaultHeight int64) common.ActionActivity {
	act := common.ActionActivity{
		ActionID: actionID, ActionType: "CASCADE", State: defaultState,
		BlockHeight: defaultHeight, CreatedViaSDK: true,
	}
	action, err := client.Blockchain.Action.GetAction(ctx, actionID)
	if err != nil {
		log.Printf("  WARN: query action %s after creation: %v", actionID, err)
		return act
	}
	if action != nil {
		act.State = string(action.State)
		act.SuperNodes = action.SuperNodes
		act.Price = action.Price
		act.Expiration = fmt.Sprintf("%d", action.ExpirationTime.Unix())
		act.BlockHeight = action.BlockHeight
	}
	return act
}

// sdkKeyType maps a recorded key-style name to the sdk-go key type.
func sdkKeyType(styleName string) sdkcrypto.KeyType {
	if styleName == common.KeyStyleEVM.Name() {
		return sdkcrypto.KeyTypeEVM
	}
	return sdkcrypto.KeyTypeCosmos
}

// resolveGRPCAddr returns the gRPC endpoint, deriving it from the RPC host when
// not set explicitly (devnet convention: RPC tcp://host:26657 -> host:9090).
func resolveGRPCAddr(grpc, rpc string) string {
	if grpc != "" {
		return grpc
	}
	host := rpc
	for _, p := range []string{"tcp://", "http://", "https://"} {
		host = strings.TrimPrefix(host, p)
	}
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}
	return host + ":9090"
}

// rpcForSDK converts the --rpc value to the http:// form the SDK expects.
func rpcForSDK(rpc string) string {
	return strings.Replace(rpc, "tcp://", "http://", 1)
}

// createSampleFile writes a small deterministic temp file for cascade upload.
func createSampleFile(acct *AccountRecord, idx int) (string, func(), error) {
	content := fmt.Sprintf("gen-activity-%s-%d-%d\n", acct.Name, idx, time.Now().UnixNano())
	for len(content) < 1024 {
		content += "padding-data-for-cascade-upload\n"
	}
	f, err := os.CreateTemp("", fmt.Sprintf("genact-%s-%d-*.bin", acct.Name, idx))
	if err != nil {
		return "", nil, fmt.Errorf("create temp file: %w", err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("close temp file: %w", err)
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}
