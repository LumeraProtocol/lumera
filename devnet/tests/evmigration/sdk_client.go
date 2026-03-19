// sdk_client.go provides SDK client factories and helpers for interacting with
// the chain via sdk-go. It supports both mnemonic-backed (in-memory keyring)
// and filesystem-backed (test keyring) clients, and includes helpers for bank
// sends, action queries, cascade uploads, and sample file creation.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	txtypes "cosmossdk.io/api/cosmos/tx/v1beta1"
	sdkblockchain "github.com/LumeraProtocol/sdk-go/blockchain"
	sdkbase "github.com/LumeraProtocol/sdk-go/blockchain/base"
	"github.com/LumeraProtocol/sdk-go/cascade"
	lumerasdk "github.com/LumeraProtocol/sdk-go/client"
	clientconfig "github.com/LumeraProtocol/sdk-go/client/config"
	sdkcrypto "github.com/LumeraProtocol/sdk-go/pkg/crypto"
	sdktypes "github.com/LumeraProtocol/sdk-go/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"go.uber.org/zap"
)

var (
	sdkClientConfigLogOnce        sync.Once
	sdkKeyringClientConfigLogOnce sync.Once
	sdkLoggerOnce                 sync.Once
	sdkLogger                     *zap.Logger
	sdkLoggerErr                  error
)

// sdkUnifiedClient creates a unified lumerasdk.Client backed by an in-memory
// keyring that holds a single key imported from the given mnemonic. The client
// provides both blockchain and cascade (supernode upload) functionality.
// The caller must call Close() on the returned client when done.
func sdkUnifiedClient(ctx context.Context, keyName, mnemonic, address string) (*lumerasdk.Client, error) {
	grpcAddr := resolveGRPC()
	rpcAddr := rpcForSDK(*flagRPC)
	waitCfg := sdkWaitTxConfig()
	logger, err := getSDKLogger()
	if err != nil {
		return nil, fmt.Errorf("create sdk logger: %w", err)
	}

	sdkClientConfigLogOnce.Do(func() {
		log.Printf("sdk-go unified client config: chain_id=%s grpc=%s rpc=%s wait_tx={setup=%s poll=%s max_retries=%d max_backoff=%s} log_level=debug",
			*flagChainID,
			grpcAddr,
			rpcAddr,
			waitCfg.SubscriberSetupTimeout,
			waitCfg.PollInterval,
			waitCfg.PollMaxRetries,
			waitCfg.PollBackoffMaxInterval,
		)
	})

	kr, err := sdkcrypto.NewKeyring(sdkcrypto.KeyringParams{
		AppName: "lumera-evmigration-test",
		Backend: "memory",
		Input:   strings.NewReader(""),
	})
	if err != nil {
		return nil, fmt.Errorf("create keyring: %w", err)
	}

	// Import legacy key (coin-type 118 / secp256k1).
	_, err = kr.NewAccount(keyName, mnemonic, "", sdkcrypto.KeyTypeCosmos.HDPath(), sdkcrypto.KeyTypeCosmos.SigningAlgo())
	if err != nil {
		return nil, fmt.Errorf("import key %s: %w", keyName, err)
	}

	client, err := lumerasdk.New(ctx, lumerasdk.Config{
		ChainID:           *flagChainID,
		GRPCEndpoint:      grpcAddr,
		RPCEndpoint:       rpcAddr,
		Address:           address,
		KeyName:           keyName,
		BlockchainTimeout: 30 * time.Second,
		StorageTimeout:    5 * time.Minute,
		WaitTx:            waitCfg,
		LogLevel:          "debug",
		Logger:            logger,
	}, kr)
	if err != nil {
		return nil, fmt.Errorf("create SDK client: %w", err)
	}
	return client, nil
}

// sdkKeyringClient creates a lumerasdk.Client backed by the local filesystem
// test keyring. Used for operations that need an existing key (e.g. funder).
func sdkKeyringClient(ctx context.Context, keyName, address string) (*lumerasdk.Client, error) {
	grpcAddr := resolveGRPC()
	rpcAddr := rpcForSDK(*flagRPC)
	waitCfg := sdkWaitTxConfig()
	logger, err := getSDKLogger()
	if err != nil {
		return nil, fmt.Errorf("create sdk logger: %w", err)
	}

	sdkKeyringClientConfigLogOnce.Do(func() {
		log.Printf("sdk-go keyring client config: chain_id=%s grpc=%s rpc=%s wait_tx={setup=%s poll=%s max_retries=%d max_backoff=%s} log_level=debug",
			*flagChainID,
			grpcAddr,
			rpcAddr,
			waitCfg.SubscriberSetupTimeout,
			waitCfg.PollInterval,
			waitCfg.PollMaxRetries,
			waitCfg.PollBackoffMaxInterval,
		)
	})

	krParams := sdkcrypto.KeyringParams{
		AppName: "lumera",
		Backend: "test",
		Input:   strings.NewReader(""),
	}
	if strings.TrimSpace(*flagHome) != "" {
		krParams.Dir = *flagHome
	}
	kr, err := sdkcrypto.NewKeyring(krParams)
	if err != nil {
		return nil, fmt.Errorf("create keyring: %w", err)
	}

	client, err := lumerasdk.New(ctx, lumerasdk.Config{
		ChainID:           *flagChainID,
		GRPCEndpoint:      grpcAddr,
		RPCEndpoint:       rpcAddr,
		Address:           address,
		KeyName:           keyName,
		BlockchainTimeout: 30 * time.Second,
		StorageTimeout:    5 * time.Minute,
		WaitTx:            waitCfg,
		LogLevel:          "debug",
		Logger:            logger,
	}, kr)
	if err != nil {
		return nil, fmt.Errorf("create keyring SDK client: %w", err)
	}
	return client, nil
}

// getSDKLogger returns a lazily-initialized debug-level zap logger for the SDK client.
func getSDKLogger() (*zap.Logger, error) {
	sdkLoggerOnce.Do(func() {
		cfg := zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		sdkLogger, sdkLoggerErr = cfg.Build()
	})
	return sdkLogger, sdkLoggerErr
}

// sdkWaitTxConfig returns the WaitTxConfig with a 1-second poll interval.
func sdkWaitTxConfig() clientconfig.WaitTxConfig {
	waitCfg := clientconfig.DefaultWaitTxConfig()
	waitCfg.PollInterval = time.Second
	waitCfg.PollMaxRetries = 0
	return waitCfg
}

// resolveGRPC returns the gRPC endpoint to use for the SDK client.
func resolveGRPC() string {
	if *flagGRPC != "" {
		return *flagGRPC
	}
	return grpcFromRPC(*flagRPC)
}

// grpcFromRPC derives a gRPC address from the RPC endpoint.
// Typical devnet pattern: RPC is tcp://host:26657, gRPC is host:9090.
func grpcFromRPC(rpc string) string {
	host := rpc
	host = strings.TrimPrefix(host, "tcp://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}
	return host + ":9090"
}

// rpcForSDK converts the --rpc flag value to the format expected by the SDK
// (http:// prefix instead of tcp://).
func rpcForSDK(rpc string) string {
	return strings.Replace(rpc, "tcp://", "http://", 1)
}

// sdkGetAction queries an action by ID using the SDK unified client.
func sdkGetAction(ctx context.Context, client *lumerasdk.Client, actionID string) (*sdktypes.Action, error) {
	return client.Blockchain.Action.GetAction(ctx, actionID)
}

// sdkSendBankTx builds, signs, and broadcasts a bank MsgSend via the SDK blockchain client.
func sdkSendBankTx(
	ctx context.Context,
	client *sdkblockchain.Client,
	fromAddr, toAddr, amount string,
	accountNumber, sequence *uint64,
) (string, error) {
	coins, err := sdk.ParseCoinsNormalized(amount)
	if err != nil {
		return "", fmt.Errorf("parse amount %s: %w", amount, err)
	}

	msg := &banktypes.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      coins,
	}

	txBytes, err := client.BuildAndSignTxWithOptions(ctx, sdkbase.TxBuildOptions{
		Messages:       []sdk.Msg{msg},
		GasLimit:       250000,
		SkipSimulation: true,
		AccountNumber:  accountNumber,
		Sequence:       sequence,
	})
	if err != nil {
		return "", fmt.Errorf("build and sign bank send: %w", err)
	}

	txHash, err := client.Broadcast(ctx, txBytes, txtypes.BroadcastMode_BROADCAST_MODE_SYNC)
	if err != nil {
		return "", fmt.Errorf("broadcast bank send: %w", err)
	}
	return txHash, nil
}

// waitForSDKTxResult waits for tx inclusion and returns an error if the tx failed.
func waitForSDKTxResult(ctx context.Context, client *sdkblockchain.Client, txHash string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := client.WaitForTxInclusion(waitCtx, txHash)
	if err != nil {
		return fmt.Errorf("wait for tx inclusion %s: %w", txHash, err)
	}
	if resp == nil || resp.TxResponse == nil {
		return fmt.Errorf("wait for tx inclusion %s: empty tx response", txHash)
	}
	if resp.TxResponse.Code != 0 {
		return fmt.Errorf("tx deliver failed code=%d raw_log=%s", resp.TxResponse.Code, resp.TxResponse.RawLog)
	}
	return nil
}

// createSampleFile creates a temporary file with deterministic content for
// uploading to supernodes. The file is named after the account and action index.
func createSampleFile(rec *AccountRecord, actionIndex int) (string, func(), error) {
	content := fmt.Sprintf("evmigration-test-data-%s-%d-%d\n", rec.Name, actionIndex, time.Now().UnixNano())
	// Pad to make it at least 1KB so the cascade pipeline treats it as a real file.
	for len(content) < 1024 {
		content += "padding-data-for-cascade-upload\n"
	}

	f, err := os.CreateTemp("", fmt.Sprintf("evmig-%s-%d-*.bin", rec.Name, actionIndex))
	if err != nil {
		return "", nil, fmt.Errorf("create temp file: %w", err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("close temp file: %w", err)
	}
	cleanup := func() { os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// createActionsWithSDK creates CASCADE actions for a single account using the
// unified SDK client (blockchain + supernode upload). Actions are left in
// different end-states for migration testing:
//   - nPending: registered on-chain only (no supernode upload) → PENDING
//   - nDone:    registered + uploaded to supernodes (auto-finalized) → DONE
//   - nApproved: registered + uploaded + approved by creator → APPROVED
func createActionsWithSDK(
	ctx context.Context,
	rec *AccountRecord,
	nPending, nDone, nApproved int,
) error {
	total := nPending + nDone + nApproved
	if total == 0 {
		return nil
	}
	if strings.TrimSpace(rec.Mnemonic) == "" {
		return fmt.Errorf("account %s has no mnemonic, cannot create SDK client", rec.Name)
	}

	actionIndex := len(rec.Actions)

	for i := 0; i < total; i++ {
		targetState := "PENDING"
		if i >= nPending && i < nPending+nDone {
			targetState = "DONE"
		} else if i >= nPending+nDone {
			targetState = "APPROVED"
		}
		idx := actionIndex + i

		switch targetState {
		case "PENDING":
			// Register action on-chain only — no upload.
			if err := createPendingAction(ctx, rec, idx); err != nil {
				log.Printf("  WARN: sdk pending action %s #%d: %v", rec.Name, idx, err)
				continue
			}

		case "DONE":
			// Register + upload to supernodes (auto-finalized → DONE).
			if err := createDoneAction(ctx, rec, idx); err != nil {
				log.Printf("  WARN: sdk done action %s #%d: %v", rec.Name, idx, err)
				continue
			}

		case "APPROVED":
			// Register + upload + approve.
			if err := createApprovedAction(ctx, rec, idx); err != nil {
				log.Printf("  WARN: sdk approved action %s #%d: %v", rec.Name, idx, err)
				continue
			}
		}
	}
	return nil
}

// runSDKActionWithSequenceRetry executes an SDK action function with up to
// 3 retries on account sequence mismatches.
func runSDKActionWithSequenceRetry(
	ctx context.Context,
	rec *AccountRecord,
	actionLabel string,
	fn func(*lumerasdk.Client) error,
) error {
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		client, err := sdkUnifiedClient(ctx, rec.Name, rec.Mnemonic, rec.Address)
		if err != nil {
			return fmt.Errorf("create SDK client for %s: %w", rec.Name, err)
		}

		err = fn(client)
		client.Close()
		if err == nil {
			return nil
		}

		lastErr = err
		expectedSeq, gotSeq, ok := parseIncorrectAccountSequence(err)
		if !ok || attempt == 2 {
			return err
		}

		log.Printf("  INFO: retrying SDK %s for %s after sequence mismatch (expected=%d got=%d, retry %d/2)",
			actionLabel, rec.Name, expectedSeq, gotSeq, attempt+1)
		if waitErr := waitForNextBlock(20 * time.Second); waitErr != nil {
			log.Printf("  WARN: wait for next block after SDK sequence mismatch: %v", waitErr)
		}
	}

	return lastErr
}

// createPendingAction registers a CASCADE action on-chain using the SDK but
// does NOT upload to supernodes, leaving it in PENDING state.
func createPendingAction(ctx context.Context, rec *AccountRecord, actionIndex int) error {
	filePath, cleanup, err := createSampleFile(rec, actionIndex)
	if err != nil {
		return err
	}
	defer cleanup()

	return runSDKActionWithSequenceRetry(ctx, rec, "pending action", func(client *lumerasdk.Client) error {
		// Use cascade to build the message (metadata + signature) then send it,
		// but skip the supernode upload step.
		msg, _, err := client.Cascade.CreateRequestActionMessage(ctx, rec.Address, filePath, &cascade.UploadOptions{
			Public: true,
		})
		if err != nil {
			return fmt.Errorf("create request action message: %w", err)
		}

		ar, err := client.Cascade.SendRequestActionMessage(ctx, client.Blockchain, msg, "", nil)
		if err != nil {
			return fmt.Errorf("send request action message: %w", err)
		}

		log.Printf("  %s registered CASCADE action %s via SDK (target=PENDING, price=%s)", rec.Name, ar.ActionID, msg.Price)

		rec.addActionFull(ar.ActionID, "CASCADE", msg.Price,
			msg.ExpirationTime, "ACTION_STATE_PENDING",
			msg.Metadata, nil, ar.Height, true)

		return nil
	})
}

// createDoneAction registers a CASCADE action on-chain and uploads the sample
// file to supernodes. The supernode auto-finalizes the action → DONE state.
func createDoneAction(ctx context.Context, rec *AccountRecord, actionIndex int) error {
	filePath, cleanup, err := createSampleFile(rec, actionIndex)
	if err != nil {
		return err
	}
	defer cleanup()

	return runSDKActionWithSequenceRetry(ctx, rec, "done action upload", func(client *lumerasdk.Client) error {
		result, err := client.Cascade.Upload(ctx, rec.Address, client.Blockchain, filePath,
			cascade.WithPublic(true),
		)
		if err != nil {
			return fmt.Errorf("cascade upload: %w", err)
		}

		log.Printf("  %s uploaded CASCADE action %s via SDK (target=DONE, taskID=%s)", rec.Name, result.ActionID, result.TaskID)

		// Query the action to get its full on-chain details.
		action, err := sdkGetAction(ctx, client, result.ActionID)
		if err != nil {
			log.Printf("  WARN: query action %s after upload: %v", result.ActionID, err)
		}

		state := "ACTION_STATE_DONE"
		var superNodes []string
		var price, expiration, metadata string
		blockHeight := result.Height
		if action != nil {
			state = string(action.State)
			superNodes = action.SuperNodes
			price = action.Price
			expiration = fmt.Sprintf("%d", action.ExpirationTime.Unix())
			blockHeight = action.BlockHeight
		}

		rec.addActionFull(result.ActionID, "CASCADE", price, expiration, state,
			metadata, superNodes, blockHeight, true)

		return nil
	})
}

// createApprovedAction registers a CASCADE action, uploads to supernodes
// (auto-finalized → DONE), then approves it → APPROVED.
func createApprovedAction(ctx context.Context, rec *AccountRecord, actionIndex int) error {
	filePath, cleanup, err := createSampleFile(rec, actionIndex)
	if err != nil {
		return err
	}
	defer cleanup()

	return runSDKActionWithSequenceRetry(ctx, rec, "approved action upload", func(client *lumerasdk.Client) error {
		result, err := client.Cascade.Upload(ctx, rec.Address, client.Blockchain, filePath,
			cascade.WithPublic(true),
		)
		if err != nil {
			return fmt.Errorf("cascade upload: %w", err)
		}
		log.Printf("  %s uploaded CASCADE action %s via SDK (target=APPROVED, taskID=%s)", rec.Name, result.ActionID, result.TaskID)

		// Wait for action to reach DONE state before approving.
		doneCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		_, err = client.Blockchain.Action.WaitForState(doneCtx, result.ActionID, sdktypes.ActionStateDone, time.Second)
		if err != nil {
			return fmt.Errorf("wait for DONE state: %w", err)
		}

		// Approve the action.
		_, err = client.Blockchain.ApproveActionTx(ctx, rec.Address, result.ActionID, "")
		if err != nil {
			return fmt.Errorf("approve action: %w", err)
		}
		log.Printf("  %s approved action %s -> APPROVED", rec.Name, result.ActionID)

		// Query final action details.
		action, err := sdkGetAction(ctx, client, result.ActionID)
		if err != nil {
			log.Printf("  WARN: query action %s after approve: %v", result.ActionID, err)
		}

		state := "ACTION_STATE_APPROVED"
		var superNodes []string
		var price, expiration string
		blockHeight := result.Height
		if action != nil {
			state = string(action.State)
			superNodes = action.SuperNodes
			price = action.Price
			expiration = fmt.Sprintf("%d", action.ExpirationTime.Unix())
			blockHeight = action.BlockHeight
		}

		rec.addActionFull(result.ActionID, "CASCADE", price, expiration, state,
			"", superNodes, blockHeight, true)

		return nil
	})
}
