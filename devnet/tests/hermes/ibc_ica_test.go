package hermes

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	txtypes "cosmossdk.io/api/cosmos/tx/v1beta1"
	sdkmath "cosmossdk.io/math"
	"gen/tests/ibcutil"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/LumeraProtocol/sdk-go/blockchain"
	"github.com/LumeraProtocol/sdk-go/blockchain/base"
	"github.com/LumeraProtocol/sdk-go/cascade"
	"github.com/LumeraProtocol/sdk-go/ica"
	sdkcrypto "github.com/LumeraProtocol/sdk-go/pkg/crypto"
	"github.com/LumeraProtocol/sdk-go/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	icaAckRetries = 120
	simdOwnerHRP  = "cosmos"
)

// TestICACascadeFlow exercises an end-to-end ICA upload/download/approve flow.
// Workflow:
// - Resolve owner + ICA addresses and load key material.
// - Ensure the ICA account is funded on the host chain.
// - Upload test files over ICA and collect action IDs from acknowledgements.
// - Download each action payload and verify content matches the source.
// - Approve each action over ICA and wait until the host chain marks them approved.
func (s *ibcSimdSuite) TestICACascadeFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), icaTestTimeout)
	defer cancel()

	// Load key material used to sign Lumera-side transactions.
	s.logInfo("ica: load lumera keyring")
	kr, _, lumeraAddr, err := sdkcrypto.LoadKeyringFromMnemonic(s.lumera.KeyName, s.lumera.MnemonicFile)
	s.Require().NoError(err, "load lumera keyring")
	s.Require().NotEmpty(lumeraAddr, "lumera address is empty")
	s.logInfof("ica: lumera address=%s", lumeraAddr)

	// Load the simd key to derive the app pubkey for ICA requests.
	s.logInfo("ica: load simd key for app pubkey")
	simdPubkey, simdAddr, err := sdkcrypto.ImportKeyFromMnemonic(kr, s.simd.KeyName, s.simd.MnemonicFile, simdOwnerHRP)
	s.Require().NoError(err, "load simd key")
	s.logInfof("ica: simd key address=%s app_pubkey_len=%d", simdAddr, len(simdPubkey))

	// Create the ICA controller client for controller-chain gRPC transactions.
	s.logInfo("ica: create ICA controller (grpc)")
	icaController, err := s.newICAController(ctx, kr, s.simd.KeyName)
	s.Require().NoError(err, "create ICA controller")
	defer icaController.Close()

	ownerAddr := icaController.OwnerAddress()
	s.Require().NotEmpty(ownerAddr, "simd owner address is empty")
	s.logInfof("ica: simd owner address=%s", ownerAddr)
	if simdAddr != "" {
		s.Require().Equal(ownerAddr, simdAddr, "simd owner address mismatch")
	}

	// Establish the ICA address, registering the controller if needed.
	s.logInfo("ica: ensure ICA address")
	icaAddr, err := icaController.EnsureICAAddress(ctx)
	s.Require().NoError(err, "ensure ICA address")
	s.Require().NotEmpty(icaAddr, "ICA address is empty")
	s.logInfof("ica: interchain account address=%s", icaAddr)

	// Create the host-chain client used for action queries and funding.
	s.logInfo("ica: create lumera blockchain client")
	lumeraClient, err := newLumeraBlockchainClient(ctx, s.lumera.ChainID, s.lumera.GRPC, kr, s.lumera.KeyName)
	s.Require().NoError(err, "create lumera client")
	defer lumeraClient.Close()
	actionClient := lumeraClient.Action

	// Fund the ICA account so it can pay fees on the host chain.
	s.logInfo("ica: ensure ICA account funded")
	err = s.ensureICAFunded(ctx, lumeraClient, lumeraAddr, icaAddr)
	s.Require().NoError(err, "fund ICA account")

	// Create cascade client for metadata, upload, and download helpers.
	s.logInfo("ica: create cascade client")
	cascadeClient, err := cascade.New(ctx, cascade.Config{
		ChainID:         s.lumera.ChainID,
		GRPCAddr:        s.lumera.GRPC,
		Address:         lumeraAddr,
		KeyName:         s.lumera.KeyName,
		ICAOwnerKeyName: s.simd.KeyName,
		ICAOwnerHRP:     simdOwnerHRP,
		Timeout:         30 * time.Second,
	}, kr)
	s.Require().NoError(err, "create cascade client")
	cascadeClient.SetLogger(newSupernodeLogger())
	defer cascadeClient.Close()

	// Prepare local test files of varying sizes.
	s.logInfo("ica: create test files")
	files, err := createICATestFiles(s.T().TempDir())
	s.Require().NoError(err, "create test files")

	// ICA send hook: build packet, submit via simd controller, and resolve action ID from ack.
	s.logInfo("ica: register actions via ICA")
	sendFunc := func(ctx context.Context, msg *actiontypes.MsgRequestAction, _ []byte, filePath string, _ *cascade.UploadOptions) (*types.ActionResult, error) {
		s.logInfof("ica: send request for %s", filepath.Base(filePath))
		res, err := icaController.SendRequestAction(ctx, msg)
		if err != nil {
			return nil, err
		}
		if res == nil || res.ActionID == "" {
			return nil, fmt.Errorf("no action id returned for %s", filepath.Base(filePath))
		}
		return res, nil
	}

	// Register actions over ICA and track action IDs per file.
	actionIDs := make(map[string]string)
	for _, f := range files {
		s.logInfof("ica: upload %s", filepath.Base(f.path))
		res, err := cascadeClient.Upload(ctx, icaAddr, nil, f.path,
			cascade.WithICACreatorAddress(icaAddr),
			cascade.WithAppPubkey(simdPubkey),
			cascade.WithICASendFunc(sendFunc),
		)
		s.Require().NoError(err, "upload via ICA for %s", f.path)
		s.Require().NotEmpty(res.ActionID, "missing action id for %s", f.path)
		actionIDs[f.path] = res.ActionID
		s.logInfof("ica: action id for %s -> %s", filepath.Base(f.path), res.ActionID)
	}

	// Download each action payload and compare to the original.
	// The action ID serves as the download handle for each payload.
	s.logInfo("ica: download and verify files")
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

	// Wait for DONE to ensure the host chain processed the uploads.
	// Then build and submit approvals via ICA.
	s.logInfo("ica: wait for DONE and build approve messages")
	for _, f := range files {
		actionID := actionIDs[f.path]
		_, err := actionClient.WaitForState(ctx, actionID, types.ActionStateDone, actionPollDelay)
		s.Require().NoError(err, "wait for action done %s", actionID)
		s.logInfof("ica: action %s is DONE", actionID)

		msg, err := cascade.CreateApproveActionMessage(ctx, actionID, cascade.WithApproveCreator(icaAddr))
		s.Require().NoError(err, "build approve message for %s", actionID)
		s.logInfof("ica: send approve tx via ICA (action_id=%s)", actionID)
		txHash, err := icaController.SendApproveAction(ctx, msg)
		s.Require().NoError(err, "send ICA approve tx for %s", actionID)
		if txHash != "" {
			s.logInfof("ica: approve tx hash=%s", txHash)
		}
	}

	// Confirm actions reach APPROVED on the host chain.
	s.logInfo("ica: wait for APPROVED")
	for _, f := range files {
		actionID := actionIDs[f.path]
		_, err := actionClient.WaitForState(ctx, actionID, types.ActionStateApproved, actionPollDelay)
		s.Require().NoError(err, "wait for action approved %s", actionID)
	}
}

type icaTestFile struct {
	path    string
	content []byte
}

// newLumeraBlockchainClient creates a Lumera blockchain client with test defaults.
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

// createICATestFiles writes a small set of deterministic test files.
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

// ensureICAFunded tops up the ICA account if the balance is below the target.
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
		s.logInfo("ica: skip funding (amount=0)")
		return nil
	}
	if s.lumera.REST != "" && amount.IsInt64() {
		bal, err := ibcutil.QueryBalanceREST(s.lumera.REST, icaAddr, s.lumera.Denom)
		if err == nil && bal >= amount.Int64() {
			s.logInfof("ica: ICA already funded (balance=%d%s)", bal, s.lumera.Denom)
			return nil
		}
		if err != nil {
			s.logInfof("ica: ICA balance query failed: %s", err)
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
			s.logInfof("ica: funding source balance query failed: %s", err)
		} else {
			maxSend := fromBal - feeBuffer
			if maxSend <= 0 {
				return fmt.Errorf("insufficient funds for ICA funding: balance=%d%s buffer=%d", fromBal, s.lumera.Denom, feeBuffer)
			}
			requested := amount.Int64()
			if requested > maxSend {
				s.logInfof("ica: funding amount reduced to %d%s (requested=%d%s balance=%d%s)",
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
	s.logInfof("ica: ICA funded (tx=%s amount=%s%s)", txHash, amount.String(), s.lumera.Denom)
	return nil
}

func (s *ibcSimdSuite) newICAController(ctx context.Context, kr keyring.Keyring, keyName string) (*ica.Controller, error) {
	if kr == nil {
		return nil, fmt.Errorf("keyring is nil")
	}
	if s.connection == nil {
		return nil, fmt.Errorf("simd connection is nil")
	}
	if strings.TrimSpace(s.simd.GRPC) == "" {
		return nil, fmt.Errorf("simd gRPC address is empty")
	}
	if strings.TrimSpace(s.lumera.GRPC) == "" {
		return nil, fmt.Errorf("lumera gRPC address is empty")
	}
	gasPrice, feeDenom, err := parseGasPrice(s.simdGasPrices, s.simd.Denom)
	if err != nil {
		return nil, err
	}

	hostHRP := strings.TrimSpace(sdk.GetConfig().GetBech32AccountAddrPrefix())
	if hostHRP == "" {
		hostHRP = "lumera"
	}

	cfg := ica.Config{
		Controller: base.Config{
			ChainID:        s.simd.ChainID,
			GRPCAddr:       s.simd.GRPC,
			RPCEndpoint:    s.simd.RPC,
			AccountHRP:     simdOwnerHRP,
			FeeDenom:       feeDenom,
			GasPrice:       gasPrice,
			Timeout:        30 * time.Second,
			MaxRecvMsgSize: 10 * 1024 * 1024,
			MaxSendMsgSize: 10 * 1024 * 1024,
			InsecureGRPC:   true,
		},
		Host: base.Config{
			ChainID:        s.lumera.ChainID,
			GRPCAddr:       s.lumera.GRPC,
			RPCEndpoint:    s.lumera.RPC,
			AccountHRP:     hostHRP,
			FeeDenom:       s.lumera.Denom,
			GasPrice:       sdkmath.LegacyNewDecWithPrec(25, 3),
			Timeout:        30 * time.Second,
			MaxRecvMsgSize: 10 * 1024 * 1024,
			MaxSendMsgSize: 10 * 1024 * 1024,
			InsecureGRPC:   true,
		},
		Keyring:                  kr,
		KeyName:                  keyName,
		ConnectionID:             s.connection.ID,
		CounterpartyConnectionID: s.connection.Counterparty.ConnectionID,
		PollDelay:                actionPollDelay,
		PollRetries:              actionPollRetries,
		AckRetries:               icaAckRetries,
	}

	return ica.NewController(ctx, cfg)
}

func parseGasPrice(input, fallbackDenom string) (sdkmath.LegacyDec, string, error) {
	price := strings.TrimSpace(input)
	if price == "" {
		if strings.TrimSpace(fallbackDenom) == "" {
			return sdkmath.LegacyDec{}, "", fmt.Errorf("gas price and denom are empty")
		}
		return sdkmath.LegacyNewDecWithPrec(25, 3), fallbackDenom, nil
	}
	first := strings.TrimSpace(strings.Split(price, ",")[0])
	if first == "" {
		return sdkmath.LegacyDec{}, "", fmt.Errorf("invalid gas price: %s", input)
	}

	split := -1
	for i, r := range first {
		if (r >= '0' && r <= '9') || r == '.' {
			continue
		}
		split = i
		break
	}
	if split <= 0 || split == len(first) {
		return sdkmath.LegacyDec{}, "", fmt.Errorf("invalid gas price: %s", input)
	}
	amountStr := first[:split]
	denom := strings.TrimSpace(first[split:])
	if denom == "" {
		return sdkmath.LegacyDec{}, "", fmt.Errorf("gas price denom is empty: %s", input)
	}
	amount, err := sdkmath.LegacyNewDecFromStr(amountStr)
	if err != nil {
		return sdkmath.LegacyDec{}, "", fmt.Errorf("parse gas price amount: %w", err)
	}
	if amount.IsZero() {
		return sdkmath.LegacyDec{}, "", fmt.Errorf("gas price amount is zero: %s", input)
	}
	return amount, denom, nil
}
