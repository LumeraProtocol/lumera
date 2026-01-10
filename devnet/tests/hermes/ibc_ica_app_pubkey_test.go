package hermes

import (
	"context"
	"fmt"
	stdlog "log"
	"os"
	"strings"
	"time"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/LumeraProtocol/sdk-go/cascade"
	sdkcrypto "github.com/LumeraProtocol/sdk-go/pkg/crypto"
	"github.com/LumeraProtocol/sdk-go/types"
)

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
	kr, _, lumeraAddr, err := sdkcrypto.LoadKeyringFromMnemonic(s.lumera.KeyName, s.lumera.MnemonicFile)
	s.Require().NoError(err, "load lumera keyring")
	s.Require().NotEmpty(lumeraAddr, "lumera address is empty")

	s.T().Log("ica: load simd key for app pubkey")
	simdPubkey, simdAddr, err := sdkcrypto.ImportKeyFromMnemonic(kr, s.simd.KeyName, s.simd.MnemonicFile, "cosmos")
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
	_, err = actionClient.WaitForState(ctx, actionIDs[0], types.ActionStatePending, actionPollDelay)
	s.Require().NoError(err)
}
