package hermes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	txtypes "cosmossdk.io/api/cosmos/tx/v1beta1"
	sdkmath "cosmossdk.io/math"
	"gen/tests/ibcutil"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/LumeraProtocol/sdk-go/blockchain"
	"github.com/LumeraProtocol/sdk-go/cascade"
	"github.com/LumeraProtocol/sdk-go/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkbech32 "github.com/cosmos/cosmos-sdk/types/bech32"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
)

const icaAckRetries = 120

type supernodeInfoLogger struct {
	logger *stdlog.Logger
}

func newSupernodeInfoLogger() *supernodeInfoLogger {
	return &supernodeInfoLogger{logger: stdlog.New(os.Stdout, "", stdlog.LstdFlags)}
}

func (l *supernodeInfoLogger) Printf(format string, v ...interface{}) {
	l.logf(format, v...)
}

func (l *supernodeInfoLogger) Infof(format string, v ...interface{}) {
	l.logf(format, v...)
}

func (l *supernodeInfoLogger) Warnf(format string, v ...interface{}) {
	l.logf(format, v...)
}

func (l *supernodeInfoLogger) logf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if strings.Contains(msg, "[supernode] DEBUG") {
		return
	}
	l.logger.Printf("%s", msg)
}

func (s *ibcSimdSuite) TestICARequestActionAppPubkeyRequired() {
	ctx, cancel := context.WithTimeout(context.Background(), icaTestTimeout)
	defer cancel()

	s.T().Log("ica: resolve simd owner address")
	ownerAddr, err := s.resolveSimdOwnerAddress(ctx)
	s.Require().NoError(err, "resolve simd owner address")

	s.T().Log("ica: ensure ICA address")
	icaAddr, err := s.ensureICAAddress(ctx, ownerAddr)
	s.Require().NoError(err, "ensure ICA address")
	s.Require().NotEmpty(icaAddr, "ICA address is empty")

	s.T().Log("ica: load lumera keyring")
	kr, _, lumeraAddr, err := loadKeyringFromMnemonic(s.lumera.KeyName, s.lumera.MnemonicFile)
	s.Require().NoError(err, "load lumera keyring")
	s.Require().NotEmpty(lumeraAddr, "lumera address is empty")

	s.T().Log("ica: load simd key for app pubkey")
	simdPubkey, simdAddr, err := importKeyFromMnemonic(kr, s.simd.KeyName, s.simd.MnemonicFile, "cosmos")
	s.Require().NoError(err, "load simd key")
	if ownerAddr != "" {
		s.Require().Equal(ownerAddr, simdAddr, "simd owner address mismatch")
	}

	s.T().Log("ica: create lumera blockchain client")
	lumeraClient, err := newLumeraBlockchainClient(ctx, s.lumera.ChainID, s.lumera.GRPC, kr, s.lumera.KeyName)
	s.Require().NoError(err, "create lumera client")
	defer lumeraClient.Close()
	actionClient := lumeraClient.Action

	s.T().Log("ica: ensure ICA account funded")
	err = s.ensureICAFunded(ctx, lumeraClient, lumeraAddr, icaAddr)
	s.Require().NoError(err, "fund ICA account")

	s.T().Log("ica: create cascade client")
	cascadeClient, err := cascade.New(ctx, cascade.Config{
		ChainID:         s.lumera.ChainID,
		GRPCAddr:        s.lumera.GRPC,
		Address:         lumeraAddr,
		KeyName:         s.lumera.KeyName,
		ICAOwnerKeyName: s.simd.KeyName,
		ICAOwnerHRP:     "cosmos",
		Timeout:         30 * time.Second,
	}, kr)
	s.Require().NoError(err, "create cascade client")
	cascadeClient.SetLogger(newSupernodeInfoLogger())
	defer cascadeClient.Close()

	files, err := createICATestFiles(s.T().TempDir())
	s.Require().NoError(err, "create test files")
	s.Require().NotEmpty(files, "no test files created")

	options := &cascade.UploadOptions{ICACreatorAddress: icaAddr}
	msg, _, err := cascadeClient.CreateRequestActionMessage(ctx, icaAddr, files[0].path, options)
	s.Require().NoError(err, "build request action without app_pubkey")

	_, err = s.sendICARequestTx(ctx, []*actiontypes.MsgRequestAction{msg})
	s.Require().Error(err, "expected app_pubkey requirement failure")
	if err != nil {
		s.Contains(err.Error(), "app_pubkey")
	}

	options.AppPubkey = simdPubkey
	msg, _, err = cascadeClient.CreateRequestActionMessage(ctx, icaAddr, files[0].path, options)
	s.Require().NoError(err, "build request action with app_pubkey")

	actionIDs, err := s.sendICARequestTx(ctx, []*actiontypes.MsgRequestAction{msg})
	s.Require().NoError(err, "send request with app_pubkey")
	s.Require().NotEmpty(actionIDs, "no action ids returned")
	s.Require().NotEmpty(actionIDs[0], "empty action id response")
	s.Require().NoError(waitForActionState(ctx, actionClient, actionIDs[0], types.ActionStatePending))
}

func (s *ibcSimdSuite) TestICACascadeFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), icaTestTimeout)
	defer cancel()

	s.T().Log("ica: resolve simd owner address")
	ownerAddr, err := s.resolveSimdOwnerAddress(ctx)
	s.Require().NoError(err, "resolve simd owner address")
	s.T().Logf("ica: simd owner address=%s", ownerAddr)

	s.T().Log("ica: ensure ICA address")
	icaAddr, err := s.ensureICAAddress(ctx, ownerAddr)
	s.Require().NoError(err, "ensure ICA address")
	s.Require().NotEmpty(icaAddr, "ICA address is empty")
	s.T().Logf("ica: interchain account address=%s", icaAddr)

	s.T().Log("ica: load lumera keyring")
	kr, _, lumeraAddr, err := loadKeyringFromMnemonic(s.lumera.KeyName, s.lumera.MnemonicFile)
	s.Require().NoError(err, "load lumera keyring")
	s.Require().NotEmpty(lumeraAddr, "lumera address is empty")
	s.T().Logf("ica: lumera address=%s", lumeraAddr)

	s.T().Log("ica: load simd key for app pubkey")
	simdPubkey, simdAddr, err := importKeyFromMnemonic(kr, s.simd.KeyName, s.simd.MnemonicFile, "cosmos")
	s.Require().NoError(err, "load simd key")
	s.T().Logf("ica: simd key address=%s app_pubkey_len=%d", simdAddr, len(simdPubkey))
	if ownerAddr != "" {
		s.Require().Equal(ownerAddr, simdAddr, "simd owner address mismatch")
	}

	s.T().Log("ica: create lumera blockchain client")
	lumeraClient, err := newLumeraBlockchainClient(ctx, s.lumera.ChainID, s.lumera.GRPC, kr, s.lumera.KeyName)
	s.Require().NoError(err, "create lumera client")
	defer lumeraClient.Close()
	actionClient := lumeraClient.Action

	s.T().Log("ica: ensure ICA account funded")
	err = s.ensureICAFunded(ctx, lumeraClient, lumeraAddr, icaAddr)
	s.Require().NoError(err, "fund ICA account")

	s.T().Log("ica: create cascade client")
	// Create cascade client for metadata, upload, and download helpers.
	cascadeClient, err := cascade.New(ctx, cascade.Config{
		ChainID:         s.lumera.ChainID,
		GRPCAddr:        s.lumera.GRPC,
		Address:         lumeraAddr,
		KeyName:         s.lumera.KeyName,
		ICAOwnerKeyName: s.simd.KeyName,
		ICAOwnerHRP:     "cosmos",
		Timeout:         30 * time.Second,
	}, kr)
	s.Require().NoError(err, "create cascade client")
	cascadeClient.SetLogger(newSupernodeInfoLogger())
	defer cascadeClient.Close()

	s.T().Log("ica: create test files")
	// Prepare local test files of varying sizes.
	files, err := createICATestFiles(s.T().TempDir())
	s.Require().NoError(err, "create test files")

	s.T().Log("ica: register actions via ICA")
	// ICA send hook: build packet, submit via simd controller, and resolve action ID from ack.
	sendFunc := func(ctx context.Context, msg *actiontypes.MsgRequestAction, _ []byte, filePath string, _ *cascade.UploadOptions) (*types.ActionResult, error) {
		s.T().Logf("ica: send request for %s", filepath.Base(filePath))
		actionIDs, err := s.sendICARequestTx(ctx, []*actiontypes.MsgRequestAction{msg})
		if err != nil {
			return nil, err
		}
		if len(actionIDs) == 0 {
			return nil, fmt.Errorf("no action ids returned for %s", filepath.Base(filePath))
		}
		return &types.ActionResult{ActionID: actionIDs[0]}, nil
	}

	// Register actions over ICA and track action IDs per file.
	actionIDs := make(map[string]string)
	for _, f := range files {
		s.T().Logf("ica: upload %s", filepath.Base(f.path))
		res, err := cascadeClient.Upload(ctx, icaAddr, nil, f.path,
			cascade.WithICACreatorAddress(icaAddr),
			cascade.WithAppPubkey(simdPubkey),
			cascade.WithICASendFunc(sendFunc),
		)
		s.Require().NoError(err, "upload via ICA for %s", f.path)
		s.Require().NotEmpty(res.ActionID, "missing action id for %s", f.path)
		actionIDs[f.path] = res.ActionID
		s.T().Logf("ica: action id for %s -> %s", filepath.Base(f.path), res.ActionID)
	}

	s.T().Log("ica: download and verify files")
	// Download each action payload and compare to the original.
	downloadDir := s.T().TempDir()
	for _, f := range files {
		actionID := actionIDs[f.path]
		_, err := cascadeClient.Download(ctx, actionID, downloadDir, cascade.WithDownloadSignerAddress(ownerAddr))
		s.Require().NoError(err, "download action %s", actionID)

		downloadedPath := filepath.Join(downloadDir, actionID, filepath.Base(f.path))
		downloaded, err := os.ReadFile(downloadedPath)
		s.Require().NoError(err, "read downloaded file %s", downloadedPath)
		s.True(bytes.Equal(f.content, downloaded), "downloaded content mismatch for %s", downloadedPath)
	}

	s.T().Log("ica: wait for DONE and build approve messages")
	for _, f := range files {
		actionID := actionIDs[f.path]
		err := waitForActionState(ctx, actionClient, actionID, types.ActionStateDone)
		s.Require().NoError(err, "wait for action done %s", actionID)
		s.T().Logf("ica: action %s is DONE", actionID)

		msg, err := cascade.CreateApproveActionMessage(ctx, actionID, cascade.WithApproveCreator(icaAddr))
		s.Require().NoError(err, "build approve message for %s", actionID)
		s.T().Logf("ica: send approve tx via ICA (action_id=%s)", actionID)
		err = s.sendICAApproveTx(ctx, []*actiontypes.MsgApproveAction{msg})
		s.Require().NoError(err, "send ICA approve tx for %s", actionID)
	}

	s.T().Log("ica: wait for APPROVED")
	// Confirm actions reach APPROVED on the host chain.
	for _, f := range files {
		actionID := actionIDs[f.path]
		err := waitForActionState(ctx, actionClient, actionID, types.ActionStateApproved)
		s.Require().NoError(err, "wait for action approved %s", actionID)
	}
}

type icaTestFile struct {
	path    string
	content []byte
}

func newLumeraBlockchainClient(ctx context.Context, chainID, grpcAddr string, kr keyring.Keyring, keyName string) (*blockchain.Client, error) {
	if grpcAddr == "" {
		return nil, fmt.Errorf("grpc address is required")
	}
	cfg := blockchain.Config{
		ChainID:        chainID,
		GRPCAddr:       grpcAddr,
		Timeout:        30 * time.Second,
		MaxRecvMsgSize: 10 * 1024 * 1024,
		MaxSendMsgSize: 10 * 1024 * 1024,
		InsecureGRPC:   true,
	}
	return blockchain.New(ctx, cfg, kr, keyName)
}

func createICATestFiles(dir string) ([]icaTestFile, error) {
	base := time.Now().UnixNano()
	sizes := []int{128, 2048, 8192}
	files := make([]icaTestFile, 0, len(sizes))

	for i, size := range sizes {
		name := fmt.Sprintf("ica-test-%d-%d.txt", base, i)
		path := filepath.Join(dir, name)
		content := bytes.Repeat([]byte{byte('a' + i)}, size)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return nil, err
		}
		files = append(files, icaTestFile{path: path, content: content})
	}

	return files, nil
}

func (s *ibcSimdSuite) ensureICAFunded(ctx context.Context, client *blockchain.Client, fromAddr, icaAddr string) error {
	if client == nil {
		return fmt.Errorf("lumera client is nil")
	}
	if fromAddr == "" || icaAddr == "" {
		return fmt.Errorf("from/ICA address is empty")
	}
	if s.lumera.Denom == "" {
		return fmt.Errorf("lumera denom is empty")
	}
	if s.lumeraICAFund == "" {
		return fmt.Errorf("lumera ICA fund amount is empty")
	}
	amount, ok := sdkmath.NewIntFromString(s.lumeraICAFund)
	if !ok {
		return fmt.Errorf("invalid lumera ICA fund amount: %s", s.lumeraICAFund)
	}
	if amount.IsZero() {
		s.T().Log("ica: skip funding (amount=0)")
		return nil
	}
	if s.lumera.REST != "" && amount.IsInt64() {
		bal, err := ibcutil.QueryBalanceREST(s.lumera.REST, icaAddr, s.lumera.Denom)
		if err == nil && bal >= amount.Int64() {
			s.T().Logf("ica: ICA already funded (balance=%d%s)", bal, s.lumera.Denom)
			return nil
		}
		if err != nil {
			s.T().Logf("ica: ICA balance query failed: %s", err)
		}
	}
	feeBuffer := int64(0)
	if s.lumeraICAFeeBuffer != "" {
		buf, err := strconv.ParseInt(s.lumeraICAFeeBuffer, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid lumera ICA fee buffer: %s", s.lumeraICAFeeBuffer)
		}
		if buf > 0 {
			feeBuffer = buf
		}
	}
	if s.lumera.REST != "" && amount.IsInt64() {
		fromBal, err := ibcutil.QueryBalanceREST(s.lumera.REST, fromAddr, s.lumera.Denom)
		if err != nil {
			s.T().Logf("ica: funding source balance query failed: %s", err)
		} else {
			maxSend := fromBal - feeBuffer
			if maxSend <= 0 {
				return fmt.Errorf("insufficient funds for ICA funding: balance=%d%s buffer=%d", fromBal, s.lumera.Denom, feeBuffer)
			}
			requested := amount.Int64()
			if requested > maxSend {
				s.T().Logf("ica: funding amount reduced to %d%s (requested=%d%s balance=%d%s)",
					maxSend, s.lumera.Denom, requested, s.lumera.Denom, fromBal, s.lumera.Denom)
				amount = sdkmath.NewInt(maxSend)
			}
		}
	}

	fromAcc, err := sdk.AccAddressFromBech32(fromAddr)
	if err != nil {
		return fmt.Errorf("parse funding address: %w", err)
	}
	icaAcc, err := sdk.AccAddressFromBech32(icaAddr)
	if err != nil {
		return fmt.Errorf("parse ICA address: %w", err)
	}
	coin := sdk.NewCoin(s.lumera.Denom, amount)
	msg := banktypes.NewMsgSend(fromAcc, icaAcc, sdk.NewCoins(coin))
	txBytes, err := client.BuildAndSignTxWithGasAdjustment(ctx, msg, "fund ica account", 2.0)
	if err != nil {
		return fmt.Errorf("build ICA fund tx: %w", err)
	}
	txHash, err := client.Broadcast(ctx, txBytes, txtypes.BroadcastMode_BROADCAST_MODE_SYNC)
	if err != nil {
		return fmt.Errorf("broadcast ICA fund tx: %w", err)
	}
	resp, err := client.WaitForTxInclusion(ctx, txHash)
	if err != nil {
		return fmt.Errorf("wait for ICA fund tx: %w", err)
	}
	if resp.TxResponse == nil {
		return fmt.Errorf("ICA fund tx response is empty")
	}
	if resp.TxResponse.Code != 0 {
		return fmt.Errorf("ICA fund tx failed: code=%d log=%s", resp.TxResponse.Code, resp.TxResponse.RawLog)
	}
	s.T().Logf("ica: ICA funded (tx=%s amount=%s%s)", txHash, amount.String(), s.lumera.Denom)
	return nil
}

func loadKeyringFromMnemonic(keyName, mnemonicFile string) (keyring.Keyring, []byte, string, error) {
	if keyName == "" {
		return nil, nil, "", fmt.Errorf("key name is required")
	}
	mnemonicRaw, err := os.ReadFile(mnemonicFile)
	if err != nil {
		return nil, nil, "", fmt.Errorf("read mnemonic file: %w", err)
	}
	mnemonic := strings.TrimSpace(string(mnemonicRaw))
	if mnemonic == "" {
		return nil, nil, "", fmt.Errorf("mnemonic file is empty")
	}

	krDir, err := os.MkdirTemp("", "lumera-keyring-*")
	if err != nil {
		return nil, nil, "", fmt.Errorf("create keyring dir: %w", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	krCodec := codec.NewProtoCodec(registry)
	kr, err := keyring.New("lumera", "test", krDir, strings.NewReader(""), krCodec)
	if err != nil {
		return nil, nil, "", fmt.Errorf("create keyring: %w", err)
	}
	if _, err := kr.NewAccount(keyName, mnemonic, "", sdk.FullFundraiserPath, hd.Secp256k1); err != nil {
		return nil, nil, "", fmt.Errorf("import key: %w", err)
	}

	addr, err := addressFromKey(kr, keyName, "lumera")
	if err != nil {
		return nil, nil, "", fmt.Errorf("derive address: %w", err)
	}

	rec, err := kr.Key(keyName)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load key: %w", err)
	}
	pub, err := rec.GetPubKey()
	if err != nil {
		return nil, nil, "", fmt.Errorf("get pubkey: %w", err)
	}
	if pub == nil {
		return nil, nil, "", fmt.Errorf("pubkey is nil")
	}

	return kr, pub.Bytes(), addr, nil
}

func importKeyFromMnemonic(kr keyring.Keyring, keyName, mnemonicFile, hrp string) ([]byte, string, error) {
	if kr == nil {
		return nil, "", fmt.Errorf("keyring is nil")
	}
	if keyName == "" {
		return nil, "", fmt.Errorf("key name is required")
	}
	mnemonicRaw, err := os.ReadFile(mnemonicFile)
	if err != nil {
		return nil, "", fmt.Errorf("read mnemonic file: %w", err)
	}
	mnemonic := strings.TrimSpace(string(mnemonicRaw))
	if mnemonic == "" {
		return nil, "", fmt.Errorf("mnemonic file is empty")
	}

	if _, err := kr.Key(keyName); err != nil {
		if _, err := kr.NewAccount(keyName, mnemonic, "", sdk.FullFundraiserPath, hd.Secp256k1); err != nil {
			return nil, "", fmt.Errorf("import key: %w", err)
		}
	}

	addr, err := addressFromKey(kr, keyName, hrp)
	if err != nil {
		return nil, "", fmt.Errorf("derive address: %w", err)
	}
	rec, err := kr.Key(keyName)
	if err != nil {
		return nil, "", fmt.Errorf("load key: %w", err)
	}
	pub, err := rec.GetPubKey()
	if err != nil {
		return nil, "", fmt.Errorf("get pubkey: %w", err)
	}
	if pub == nil {
		return nil, "", fmt.Errorf("pubkey is nil")
	}
	return pub.Bytes(), addr, nil
}

func addressFromKey(kr keyring.Keyring, keyName, hrp string) (string, error) {
	if kr == nil {
		return "", fmt.Errorf("keyring is required")
	}
	if keyName == "" {
		return "", fmt.Errorf("key name is required")
	}
	rec, err := kr.Key(keyName)
	if err != nil {
		return "", fmt.Errorf("key %s not found: %w", keyName, err)
	}
	pub, err := rec.GetPubKey()
	if err != nil {
		return "", fmt.Errorf("get pubkey: %w", err)
	}
	if pub == nil {
		return "", fmt.Errorf("pubkey is nil")
	}
	addrBz := pub.Address()
	bech, err := sdkbech32.ConvertAndEncode(hrp, addrBz)
	if err != nil {
		return "", fmt.Errorf("bech32 encode: %w", err)
	}
	return bech, nil
}

func waitForActionState(ctx context.Context, client *blockchain.ActionClient, actionID string, state types.ActionState) error {
	for i := 0; i < actionPollRetries; i++ {
		action, err := client.GetAction(ctx, actionID)
		if err == nil && action != nil && action.State == state {
			return nil
		}
		time.Sleep(actionPollDelay)
	}
	return fmt.Errorf("action %s did not reach state %s", actionID, state)
}

func (s *ibcSimdSuite) resolveSimdOwnerAddress(ctx context.Context) (string, error) {
	if s.simdAddrFile != "" {
		if addr, err := ibcutil.ReadAddress(s.simdAddrFile); err == nil {
			return addr, nil
		}
	}
	out, err := s.runSimdCmd(ctx, simdQueryTimeout, "keys", "show", s.simd.KeyName, "-a", "--keyring-backend", s.simdKeyring)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ibcSimdSuite) ensureICAAddress(ctx context.Context, ownerAddr string) (string, error) {
	addr, err := s.queryICAAddress(ctx, ownerAddr)
	if err == nil && addr != "" {
		s.T().Logf("ica: existing ICA address found=%s", addr)
		return addr, nil
	}
	s.T().Log("ica: ICA address not found; registering controller")
	if err := s.registerICA(ctx); err != nil {
		return "", err
	}
	if err := s.waitForICAChannel(ctx, ownerAddr); err != nil {
		s.T().Logf("ica: channel not open yet: %s", err)
	}
	for i := 0; i < actionPollRetries; i++ {
		addr, err = s.queryICAAddress(ctx, ownerAddr)
		if err == nil && addr != "" {
			s.T().Logf("ica: ICA address registered=%s", addr)
			return addr, nil
		}
		if i%10 == 0 {
			s.T().Logf("ica: waiting for ICA address (attempt %d/%d)", i+1, actionPollRetries)
		}
		time.Sleep(actionPollDelay)
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("ICA address not found")
}

func (s *ibcSimdSuite) waitForICAChannel(_ context.Context, ownerAddr string) error {
	portID := fmt.Sprintf("icacontroller-%s", ownerAddr)
	for i := 0; i < actionPollRetries; i++ {
		channels, err := ibcutil.QueryChannels(s.simdBin, s.simd.RPC)
		if err != nil {
			return err
		}
		if i%10 == 0 {
			s.T().Logf("ica: waiting for controller channel open (attempt %d/%d channels=%d)", i+1, actionPollRetries, len(channels))
		}
		for _, ch := range channels {
			if ch.PortID == portID && ibcutil.IsOpenState(ch.State) {
				s.T().Logf("ica: controller channel open (port=%s channel=%s)", ch.PortID, ch.ChannelID)
				return nil
			}
			if ch.PortID == portID && !ibcutil.IsOpenState(ch.State) && i%10 == 0 {
				s.T().Logf("ica: controller channel not open yet (port=%s channel=%s state=%s)", ch.PortID, ch.ChannelID, ch.State)
			}
		}
		time.Sleep(actionPollDelay)
	}
	return fmt.Errorf("controller channel %s not open after %d retries", portID, actionPollRetries)
}

func (s *ibcSimdSuite) queryICAAddress(ctx context.Context, ownerAddr string) (string, error) {
	args := []string{
		"q", "interchain-accounts", "controller", "interchain-account",
		ownerAddr, s.connection.ID,
		"--output", "json",
	}
	if s.simd.RPC != "" {
		args = append(args, "--node", s.simd.RPC)
	}
	out, err := s.runSimdCmd(ctx, simdQueryTimeout, args...)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "failed to retrieve account address") || strings.Contains(msg, "key not found") {
			s.T().Logf("ica: ICA address not found yet (%s)", ownerAddr)
			return "", nil
		}
		return "", err
	}
	var resp struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Address), nil
}

func (s *ibcSimdSuite) registerICA(ctx context.Context) error {
	version := icatypes.NewDefaultMetadataString(s.connection.ID, s.connection.Counterparty.ConnectionID)
	s.T().Logf("ica: register controller (connection=%s counterparty=%s version=%s)", s.connection.ID, s.connection.Counterparty.ConnectionID, version)
	s.logICAConnectionDetails(ctx)
	args := []string{
		"tx", "interchain-accounts", "controller", "register", s.connection.ID,
		"--version", version,
		"--from", s.simd.KeyName,
		"--chain-id", s.simd.ChainID,
		"--keyring-backend", s.simdKeyring,
		"--gas", "auto",
		"--gas-adjustment", "1.3",
		"--broadcast-mode", "sync",
		"--yes",
	}
	if s.simdGasPrices != "" {
		args = append(args, "--gas-prices", s.simdGasPrices)
	}
	if s.simd.RPC != "" {
		args = append(args, "--node", s.simd.RPC)
	}
	out, err := s.runSimdCmd(ctx, simdTxTimeout, args...)
	if err != nil {
		return err
	}
	s.T().Logf("ica: register controller response: %s", trimLog(out))
	if resp, ok := parseTxResponse(out); ok {
		if resp.Code != 0 {
			return fmt.Errorf("ica register failed (code=%d codespace=%s tx=%s log=%s)", resp.Code, resp.Codespace, resp.TxHash, resp.RawLog)
		}
		if resp.TxHash != "" {
			s.T().Logf("ica: register controller tx hash=%s", resp.TxHash)
		}
	}
	return nil
}

func (s *ibcSimdSuite) logICAConnectionDetails(ctx context.Context) {
	if s.connection == nil {
		return
	}

	s.T().Logf("ica: simd connection detail (id=%s client_id=%s counterparty_client_id=%s counterparty_connection_id=%s)",
		s.connection.ID, s.connection.ClientID, s.connection.Counterparty.ClientID, s.connection.Counterparty.ConnectionID)
	simdClientChainID := s.querySimdClientChainID(ctx, s.connection.ClientID)
	if simdClientChainID != "" {
		s.T().Logf("ica: simd client-state chain_id=%s (client_id=%s)", simdClientChainID, s.connection.ClientID)
		if s.lumera.ChainID != "" && simdClientChainID != s.lumera.ChainID {
			s.T().Logf("ica: WARNING simd client chain_id mismatch (expected %s)", s.lumera.ChainID)
		}
	}

	// Query the lumera side for the counterparty connection via REST, if available.
	if s.connection.Counterparty.ConnectionID == "" || s.lumera.REST == "" {
		return
	}

	url := strings.TrimSuffix(s.lumera.REST, "/") + "/ibc/core/connection/v1/connections/" + s.connection.Counterparty.ConnectionID
	reqCtx, cancel := context.WithTimeout(ctx, simdQueryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		s.T().Logf("ica: lumera connection request build failed: %s", err)
		return
	}
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.T().Logf("ica: lumera connection request failed: %s", err)
		return
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		s.T().Logf("ica: lumera connection read failed: %s", err)
		return
	}

	var connResp struct {
		Connection struct {
			ClientID     string `json:"client_id"`
			State        string `json:"state"`
			Counterparty struct {
				ClientID     string `json:"client_id"`
				ConnectionID string `json:"connection_id"`
			} `json:"counterparty"`
		} `json:"connection"`
	}
	if err := json.Unmarshal(body, &connResp); err != nil {
		s.T().Logf("ica: lumera connection parse failed: %s", err)
		return
	}
	s.T().Logf("ica: lumera connection detail (id=%s client_id=%s state=%s counterparty_client_id=%s counterparty_connection_id=%s)",
		s.connection.Counterparty.ConnectionID, connResp.Connection.ClientID, connResp.Connection.State, connResp.Connection.Counterparty.ClientID, connResp.Connection.Counterparty.ConnectionID)
	lumeraClientChainID := s.queryLumeraClientChainID(ctx, connResp.Connection.ClientID)
	if lumeraClientChainID != "" {
		s.T().Logf("ica: lumera client-state chain_id=%s (client_id=%s)", lumeraClientChainID, connResp.Connection.ClientID)
		if s.simd.ChainID != "" && lumeraClientChainID != s.simd.ChainID {
			s.T().Logf("ica: WARNING lumera client chain_id mismatch (expected %s)", s.simd.ChainID)
		}
	}
}

func (s *ibcSimdSuite) querySimdClientChainID(ctx context.Context, clientID string) string {
	if clientID == "" {
		return ""
	}
	args := []string{"q", "ibc", "client", "state", clientID, "--output", "json"}
	if s.simd.RPC != "" {
		args = append(args, "--node", s.simd.RPC)
	}
	out, err := s.runSimdCmd(ctx, simdQueryTimeout, args...)
	if err != nil {
		s.T().Logf("ica: simd client-state query failed (client_id=%s): %s", clientID, err)
		return ""
	}
	chainID := extractClientChainID([]byte(out))
	if chainID == "" {
		s.T().Logf("ica: simd client-state missing chain_id (client_id=%s)", clientID)
		return ""
	}
	return chainID
}

func (s *ibcSimdSuite) queryLumeraClientChainID(ctx context.Context, clientID string) string {
	if clientID == "" || s.lumera.REST == "" {
		return ""
	}
	url := strings.TrimSuffix(s.lumera.REST, "/") + "/ibc/core/client/v1/client_states/" + clientID
	reqCtx, cancel := context.WithTimeout(ctx, simdQueryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		s.T().Logf("ica: lumera client-state request build failed (client_id=%s): %s", clientID, err)
		return ""
	}
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.T().Logf("ica: lumera client-state request failed (client_id=%s): %s", clientID, err)
		return ""
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		s.T().Logf("ica: lumera client-state read failed (client_id=%s): %s", clientID, err)
		return ""
	}
	chainID := extractClientChainID(body)
	if chainID == "" {
		s.T().Logf("ica: lumera client-state missing chain_id (client_id=%s)", clientID)
		return ""
	}
	return chainID
}

func extractClientChainID(raw []byte) string {
	var resp map[string]any
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ""
	}
	clientState := mapPath(resp, "identified_client_state", "client_state")
	if len(clientState) == 0 {
		clientState = mapPath(resp, "client_state")
	}
	if len(clientState) == 0 {
		return ""
	}
	return stringPath(clientState, "chain_id")
}

func mapPath(m map[string]any, path ...string) map[string]any {
	var cur any = m
	for _, p := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = next[p]
	}
	if out, ok := cur.(map[string]any); ok {
		return out
	}
	return nil
}

func stringPath(m map[string]any, path ...string) string {
	var cur any = m
	for _, p := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = next[p]
	}
	return stringFromAny(cur)
}

func stringFromAny(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatInt(int64(val), 10)
	case json.Number:
		return val.String()
	default:
		return ""
	}
}

func (s *ibcSimdSuite) waitForPacketInfo(ctx context.Context, txHash string) (cascade.PacketInfo, error) {
	for i := 0; i < actionPollRetries; i++ {
		info, err := s.queryPacketInfo(ctx, txHash)
		if err == nil {
			s.T().Logf("ica: packet info found (tx=%s port=%s channel=%s seq=%d)", txHash, info.Port, info.Channel, info.Sequence)
			return info, nil
		}
		time.Sleep(actionPollDelay)
	}
	return cascade.PacketInfo{}, fmt.Errorf("packet info not found for tx %s", txHash)
}

func (s *ibcSimdSuite) queryPacketInfo(ctx context.Context, txHash string) (cascade.PacketInfo, error) {
	args := []string{"q", "tx", txHash, "--output", "json"}
	if s.simd.RPC != "" {
		args = append(args, "--node", s.simd.RPC)
	}
	out, err := s.runSimdCmd(ctx, simdQueryTimeout, args...)
	if err != nil {
		return cascade.PacketInfo{}, err
	}
	return cascade.ExtractPacketInfoFromTxJSON([]byte(out))
}

func (s *ibcSimdSuite) waitForPacketAcknowledgement(ctx context.Context, info cascade.PacketInfo) ([]byte, error) {
	hostPort, hostChannel := s.resolveHostPacketRoute(ctx, info)
	s.T().Logf("ica: wait for host ack (port=%s channel=%s seq=%d)", hostPort, hostChannel, info.Sequence)
	var lastErr error
	for i := 0; i < icaAckRetries; i++ {
		ack, err := s.queryLumeraPacketAcknowledgement(ctx, hostPort, hostChannel, info.Sequence)
		if err == nil {
			s.T().Logf("ica: packet ack received (port=%s channel=%s seq=%d len=%d)", info.Port, info.Channel, info.Sequence, len(ack))
			return ack, nil
		}
		lastErr = err
		if !isRetryableAckErr(err) {
			return nil, err
		}
		if i%10 == 0 {
			s.T().Logf("ica: ack not found yet (port=%s channel=%s seq=%d err=%s)", info.Port, info.Channel, info.Sequence, err)
			s.logAckDebug(ctx, info)
		}
		time.Sleep(actionPollDelay)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("acknowledgement not found for %s/%s/%d: %v", info.Port, info.Channel, info.Sequence, lastErr)
	}
	return nil, fmt.Errorf("acknowledgement not found for %s/%s/%d", info.Port, info.Channel, info.Sequence)
}

func isRetryableAckErr(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "NotFound") ||
		strings.Contains(msg, "invalid acknowledgement")
}

func (s *ibcSimdSuite) logAckDebug(ctx context.Context, info cascade.PacketInfo) {
	s.logSimdPacketQuery(ctx, "packet-commitment", info)
	s.logSimdPacketQuery(ctx, "packet-receipt", info)

	state, cpPort, cpChannel, err := s.querySimdChannelEnd(ctx, info.Port, info.Channel)
	if err != nil {
		s.T().Logf("ica: simd channel end query failed (port=%s channel=%s): %s", info.Port, info.Channel, err)
	} else {
		s.T().Logf("ica: simd channel end state=%s counterparty_port=%s counterparty_channel=%s", state, cpPort, cpChannel)
	}

	lumeraPort, lumeraChannel := s.resolveHostPacketRoute(ctx, info)
	s.logLumeraPacketQuery(ctx, "packet_receipts", lumeraPort, lumeraChannel, info.Sequence)
	s.logLumeraPacketQuery(ctx, "packet_acks", lumeraPort, lumeraChannel, info.Sequence)
}

func (s *ibcSimdSuite) logSimdPacketQuery(ctx context.Context, subcommand string, info cascade.PacketInfo) {
	args := []string{
		"q", "ibc", "channel", subcommand,
		info.Port, info.Channel, strconv.FormatUint(info.Sequence, 10),
		"--output", "json",
	}
	if s.simd.RPC != "" {
		args = append(args, "--node", s.simd.RPC)
	}
	out, err := s.runSimdCmd(ctx, simdQueryTimeout, args...)
	if err != nil {
		s.T().Logf("ica: simd %s error: %s", subcommand, err)
		return
	}
	s.T().Logf("ica: simd %s response: %s", subcommand, trimLog(out))
}

func (s *ibcSimdSuite) querySimdChannelEnd(ctx context.Context, port, channel string) (string, string, string, error) {
	args := []string{
		"q", "ibc", "channel", "end",
		port, channel,
		"--output", "json",
	}
	if s.simd.RPC != "" {
		args = append(args, "--node", s.simd.RPC)
	}
	out, err := s.runSimdCmd(ctx, simdQueryTimeout, args...)
	if err != nil {
		return "", "", "", err
	}
	var resp struct {
		Channel struct {
			State        string `json:"state"`
			Counterparty struct {
				PortID    string `json:"port_id"`
				ChannelID string `json:"channel_id"`
			} `json:"counterparty"`
		} `json:"channel"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", "", "", fmt.Errorf("parse channel end: %w", err)
	}
	return resp.Channel.State, resp.Channel.Counterparty.PortID, resp.Channel.Counterparty.ChannelID, nil
}

func (s *ibcSimdSuite) logLumeraPacketQuery(ctx context.Context, endpoint, port, channel string, sequence uint64) {
	if s.lumera.REST == "" {
		return
	}
	url := fmt.Sprintf("%s/ibc/core/channel/v1/channels/%s/ports/%s/%s/%d",
		strings.TrimSuffix(s.lumera.REST, "/"), channel, port, endpoint, sequence)
	reqCtx, cancel := context.WithTimeout(ctx, simdQueryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		s.T().Logf("ica: lumera %s request build failed: %s", endpoint, err)
		return
	}
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.T().Logf("ica: lumera %s request failed: %s", endpoint, err)
		return
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		s.T().Logf("ica: lumera %s read failed: %s", endpoint, err)
		return
	}
	s.T().Logf("ica: lumera %s response (status=%d): %s", endpoint, httpResp.StatusCode, trimLog(string(body)))
}

func (s *ibcSimdSuite) resolveHostPacketRoute(ctx context.Context, info cascade.PacketInfo) (string, string) {
	hostPort := info.Port
	hostChannel := info.Channel
	if _, cpPort, cpChannel, err := s.querySimdChannelEnd(ctx, info.Port, info.Channel); err == nil {
		if cpPort != "" {
			hostPort = cpPort
		}
		if cpChannel != "" {
			hostChannel = cpChannel
		}
	}
	if hostPort == info.Port && strings.HasPrefix(info.Port, "icacontroller-") {
		hostPort = "icahost"
	}
	return hostPort, hostChannel
}

func (s *ibcSimdSuite) queryLumeraPacketAcknowledgement(ctx context.Context, port, channel string, sequence uint64) ([]byte, error) {
	return s.queryLumeraAckHex(ctx, port, channel, sequence)
}

func (s *ibcSimdSuite) queryLumeraAckHex(ctx context.Context, port, channel string, sequence uint64) ([]byte, error) {
	if s.lumera.RPC == "" {
		return nil, fmt.Errorf("lumera RPC address is empty")
	}
	base := strings.TrimSuffix(s.lumera.RPC, "/") + "/tx_search"
	values := url.Values{}
	queryExpr := fmt.Sprintf("write_acknowledgement.packet_dst_port='%s' AND write_acknowledgement.packet_dst_channel='%s' AND write_acknowledgement.packet_sequence='%d'", port, channel, sequence)
	// CometBFT expects JSON-string encoded query params.
	values.Add("query", fmt.Sprintf("%q", queryExpr))
	values.Add("prove", "false")
	values.Add("page", "1")
	values.Add("per_page", "5")
	reqURL := base + "?" + values.Encode()

	reqCtx, cancel := context.WithTimeout(ctx, simdQueryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build lumera ack events request: %w", err)
	}
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lumera ack events request failed: %w", err)
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read lumera ack events response: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lumera ack events query failed (status=%d): %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}
	type eventAttr struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	type event struct {
		Type       string      `json:"type"`
		Attributes []eventAttr `json:"attributes"`
	}
	var resp struct {
		Result struct {
			Txs []struct {
				TxResult struct {
					Events []event `json:"events"`
				} `json:"tx_result"`
			} `json:"txs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse lumera ack events response: %w", err)
	}

	for _, tx := range resp.Result.Txs {
		var ackHex string
		var ackErr string
		var ackSuccess *bool
		for _, evt := range tx.TxResult.Events {
			evtType := strings.TrimSpace(decodeEventValue(evt.Type))
			if evtType == "" {
				evtType = evt.Type
			}
			if evtType != "write_acknowledgement" {
				if strings.Contains(evtType, "ics27_packet") {
					attr := make(map[string]string)
					for _, a := range evt.Attributes {
						key := strings.TrimSpace(decodeEventValue(a.Key))
						val := strings.TrimSpace(decodeEventValue(a.Value))
						if key != "" {
							attr[key] = val
						}
					}
					if v, ok := attr["success"]; ok {
						success := strings.EqualFold(v, "true")
						ackSuccess = &success
					}
					if v, ok := attr["ibccallbackerror-success"]; ok {
						success := strings.EqualFold(v, "true")
						ackSuccess = &success
					}
					if v := attr["error"]; v != "" {
						ackErr = v
					}
					if v := attr["ibccallbackerror-error"]; v != "" {
						ackErr = v
					}
				}
				continue
			}
			attr := make(map[string]string)
			for _, a := range evt.Attributes {
				key := strings.TrimSpace(decodeEventValue(a.Key))
				val := strings.TrimSpace(decodeEventValue(a.Value))
				if key != "" {
					attr[key] = val
				}
			}
			if attr["packet_dst_port"] != port ||
				attr["packet_dst_channel"] != channel ||
				attr["packet_sequence"] != strconv.FormatUint(sequence, 10) {
				continue
			}
			ackHex = attr["packet_ack_hex"]
		}
		if ackHex == "" {
			continue
		}
		if ackSuccess != nil && !*ackSuccess {
			if ackErr != "" {
				return nil, fmt.Errorf("ica host ack error: %s", ackErr)
			}
			return nil, fmt.Errorf("ica host ack error: unknown failure")
		}
		ack, err := hex.DecodeString(ackHex)
		if err != nil {
			return nil, fmt.Errorf("decode acknowledgement hex: %w", err)
		}
		return ack, nil
	}

	return nil, fmt.Errorf("acknowledgement event not found for %s/%s/%d", port, channel, sequence)
}

func decodeEventValue(raw string) string {
	if raw == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return raw
	}
	if !isMostlyPrintableASCII(decoded) {
		return raw
	}
	return string(decoded)
}

func isMostlyPrintableASCII(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	printable := 0
	for _, b := range data {
		if b == '\n' || b == '\r' || b == '\t' || (b >= 32 && b <= 126) {
			printable++
		}
	}
	return printable*100/len(data) >= 90
}

func trimLog(payload string) string {
	out := strings.TrimSpace(payload)
	const maxLen = 600
	if len(out) <= maxLen {
		return out
	}
	return out[:maxLen] + "…"
}

func parseTxHash(output string) (string, error) {
	if hash, err := cascade.ParseTxHashJSON([]byte(output)); err == nil {
		return hash, nil
	}
	if hash, err := parseTxHashFromLines(output); err == nil {
		return hash, nil
	}
	return "", fmt.Errorf("unable to parse tx hash from output")
}

type txResponse struct {
	Code      int64  `json:"code"`
	Codespace string `json:"codespace"`
	RawLog    string `json:"raw_log"`
	TxHash    string `json:"txhash"`
}

func parseTxResponse(output string) (txResponse, bool) {
	if resp, ok := parseTxResponseJSON([]byte(strings.TrimSpace(output))); ok {
		return resp, true
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[{") {
			if resp, ok := parseTxResponseJSON([]byte(line)); ok {
				return resp, true
			}
		}
	}
	start := strings.Index(output, "{")
	if start >= 0 {
		if resp, ok := parseTxResponseJSON([]byte(output[start:])); ok {
			return resp, true
		}
	}
	return txResponse{}, false
}

func parseTxResponseJSON(data []byte) (txResponse, bool) {
	var resp txResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return txResponse{}, false
	}
	return resp, true
}

func parseTxHashFromLines(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[{") {
			if hash, err := cascade.ParseTxHashJSON([]byte(line)); err == nil {
				return hash, nil
			}
		}
	}
	start := strings.Index(output, "{")
	if start >= 0 {
		if hash, err := cascade.ParseTxHashJSON([]byte(output[start:])); err == nil {
			return hash, nil
		}
	}
	return "", fmt.Errorf("tx hash not found in output")
}

func (s *ibcSimdSuite) sendICARequestTx(ctx context.Context, msgs []*actiontypes.MsgRequestAction) ([]string, error) {
	var anys []*codectypes.Any
	for _, msg := range msgs {
		any, err := packRequestAny(msg)
		if err != nil {
			return nil, err
		}
		anys = append(anys, any)
	}
	_, ackBytes, err := s.sendICAAnysWithAck(ctx, anys)
	if err != nil {
		return nil, err
	}
	actionIDs, err := cascade.ExtractRequestActionIDsFromAck(ackBytes)
	if err != nil {
		return nil, err
	}
	return actionIDs, nil
}

func (s *ibcSimdSuite) sendICAApproveTx(ctx context.Context, msgs []*actiontypes.MsgApproveAction) error {
	s.T().Logf("ica: build approve ICA packet (msgs=%d)", len(msgs))
	var anys []*codectypes.Any
	for _, msg := range msgs {
		s.T().Logf("ica: approve msg action_id=%s creator=%s", msg.ActionId, msg.Creator)
		any, err := packApproveAny(msg)
		if err != nil {
			return err
		}
		anys = append(anys, any)
	}
	info, ackBytes, err := s.sendICAAnysWithAck(ctx, anys)
	if err != nil {
		return err
	}
	s.T().Logf("ica: approve ack received (port=%s channel=%s seq=%d len=%d)", info.Port, info.Channel, info.Sequence, len(ackBytes))
	return nil
}

func (s *ibcSimdSuite) sendICAAnysWithAck(ctx context.Context, anys []*codectypes.Any) (cascade.PacketInfo, []byte, error) {
	s.T().Logf("ica: build ICA packet data (msgs=%d connection=%s)", len(anys), s.connection.ID)
	packet, err := cascade.BuildICAPacketData(anys)
	if err != nil {
		return cascade.PacketInfo{}, nil, err
	}
	packetJSON, err := marshalICAPacketJSON(packet)
	if err != nil {
		return cascade.PacketInfo{}, nil, err
	}

	tmpFile, err := os.CreateTemp("", "ica-packet-*.json")
	if err != nil {
		return cascade.PacketInfo{}, nil, err
	}
	if _, err := tmpFile.Write(packetJSON); err != nil {
		_ = tmpFile.Close()
		return cascade.PacketInfo{}, nil, err
	}
	if err := tmpFile.Close(); err != nil {
		return cascade.PacketInfo{}, nil, err
	}

	args := []string{
		"tx", "interchain-accounts", "controller", "send-tx", s.connection.ID, tmpFile.Name(),
		"--from", s.simd.KeyName,
		"--chain-id", s.simd.ChainID,
		"--keyring-backend", s.simdKeyring,
		"--gas", "auto",
		"--gas-adjustment", "1.3",
		"--broadcast-mode", "sync",
		"--output", "json",
		"--yes",
	}
	if s.simdGasPrices != "" {
		args = append(args, "--gas-prices", s.simdGasPrices)
	}
	if s.simd.RPC != "" {
		args = append(args, "--node", s.simd.RPC)
	}

	out, err := s.runSimdCmd(ctx, simdTxTimeout, args...)
	if err != nil {
		return cascade.PacketInfo{}, nil, err
	}

	txHash, err := parseTxHash(out)
	if err != nil {
		return cascade.PacketInfo{}, nil, err
	}

	packetInfo, err := s.waitForPacketInfo(ctx, txHash)
	if err != nil {
		return cascade.PacketInfo{}, nil, err
	}

	ackBytes, err := s.waitForPacketAcknowledgement(ctx, packetInfo)
	if err != nil {
		return cascade.PacketInfo{}, nil, err
	}
	return packetInfo, ackBytes, nil
}

func packRequestAny(msg *actiontypes.MsgRequestAction) (*codectypes.Any, error) {
	anyBytes, err := cascade.PackRequestForICA(msg)
	if err != nil {
		return nil, err
	}
	var any codectypes.Any
	if err := gogoproto.Unmarshal(anyBytes, &any); err != nil {
		return nil, err
	}
	return &any, nil
}

func packApproveAny(msg *actiontypes.MsgApproveAction) (*codectypes.Any, error) {
	anyBytes, err := cascade.PackApproveForICA(msg)
	if err != nil {
		return nil, err
	}
	var any codectypes.Any
	if err := gogoproto.Unmarshal(anyBytes, &any); err != nil {
		return nil, err
	}
	return &any, nil
}

func marshalICAPacketJSON(packet icatypes.InterchainAccountPacketData) ([]byte, error) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	return cdc.MarshalJSON(&packet)
}

func (s *ibcSimdSuite) runSimdCmd(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	cmdArgs := append([]string{}, args...)
	if s.simdHome != "" {
		cmdArgs = append([]string{"--home", s.simdHome}, cmdArgs...)
	}
	s.T().Logf("ica: simd cmd: %s", formatCmd(s.simdBin, cmdArgs))
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, s.simdBin, cmdArgs...)
	out, err := cmd.CombinedOutput()
	if cmdCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("simd command timed out: %s", strings.TrimSpace(string(out)))
	}
	if err != nil {
		return "", fmt.Errorf("simd command failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func formatCmd(bin string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(bin))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	safe := true
	for _, r := range value {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' || r == '/' || r == ':' {
			continue
		}
		safe = false
		break
	}
	if safe {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
