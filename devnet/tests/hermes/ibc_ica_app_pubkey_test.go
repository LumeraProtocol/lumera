package hermes

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/LumeraProtocol/sdk-go/cascade"
	sdkcrypto "github.com/LumeraProtocol/sdk-go/pkg/crypto"
	"github.com/LumeraProtocol/sdk-go/types"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var plainEncoderPool = buffer.NewPool()

type plainEncoder struct {
	zapcore.Encoder
	cfg zapcore.EncoderConfig
}

func (e *plainEncoder) Clone() zapcore.Encoder {
	return &plainEncoder{
		Encoder: e.Encoder.Clone(),
		cfg:     e.cfg,
	}
}

func (e *plainEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	buf := plainEncoderPool.Get()
	level := entry.Level.CapitalString()
	ts := entry.Time.Format("01/02/2006 15:04:05.000")

	buf.AppendString(level)
	buf.AppendByte(' ')
	buf.AppendString(ts)
	if entry.Message != "" {
		buf.AppendByte(' ')
		buf.AppendString(entry.Message)
	}
	if len(fields) > 0 {
		buf.AppendByte(' ')
		appendZapFields(buf, fields)
	}
	if e.cfg.LineEnding != "" {
		buf.AppendString(e.cfg.LineEnding)
	}
	return buf, nil
}

func appendZapFields(buf *buffer.Buffer, fields []zapcore.Field) {
	enc := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(enc)
	}
	keys := make([]string, 0, len(enc.Fields))
	for k := range enc.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			buf.AppendByte(' ')
		}
		buf.AppendString(k)
		buf.AppendByte('=')
		buf.AppendString(fmt.Sprint(enc.Fields[k]))
	}
}

func newSupernodeLogger() *zap.Logger {
	cfg := zapcore.EncoderConfig{
		LineEnding: zapcore.DefaultLineEnding,
	}
	baseEnc := zapcore.NewConsoleEncoder(cfg)
	enc := &plainEncoder{
		Encoder: baseEnc,
		cfg:     cfg,
	}
	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), zap.NewAtomicLevelAt(zap.InfoLevel))
	return zap.New(core)
}

func (s *ibcSimdSuite) TestICARequestActionAppPubkeyRequired() {
	ctx, cancel := context.WithTimeout(context.Background(), icaTestTimeout)
	defer cancel()

	s.logInfo("ica: load lumera keyring")
	kr, _, lumeraAddr, err := sdkcrypto.LoadKeyringFromMnemonic(s.lumera.KeyName, s.lumera.MnemonicFile)
	s.Require().NoError(err, "load lumera keyring")
	s.Require().NotEmpty(lumeraAddr, "lumera address is empty")

	s.logInfo("ica: load simd key for app pubkey")
	simdPubkey, simdAddr, err := sdkcrypto.ImportKeyFromMnemonic(kr, s.simd.KeyName, s.simd.MnemonicFile, simdOwnerHRP)
	s.Require().NoError(err, "load simd key")

	s.logInfo("ica: create ICA controller (grpc)")
	icaController, err := s.newICAController(ctx, kr, s.simd.KeyName)
	s.Require().NoError(err, "create ICA controller")
	defer icaController.Close()

	ownerAddr := icaController.OwnerAddress()
	s.Require().NotEmpty(ownerAddr, "simd owner address is empty")
	if simdAddr != "" {
		s.Require().Equal(ownerAddr, simdAddr, "simd owner address mismatch")
	}

	s.logInfo("ica: ensure ICA address")
	icaAddr, err := icaController.EnsureICAAddress(ctx)
	s.Require().NoError(err, "ensure ICA address")
	s.Require().NotEmpty(icaAddr, "ICA address is empty")

	s.logInfo("ica: create lumera blockchain client")
	lumeraClient, err := newLumeraBlockchainClient(ctx, s.lumera.ChainID, s.lumera.GRPC, kr, s.lumera.KeyName)
	s.Require().NoError(err, "create lumera client")
	defer lumeraClient.Close()
	actionClient := lumeraClient.Action

	s.logInfo("ica: ensure ICA account funded")
	err = s.ensureICAFunded(ctx, lumeraClient, lumeraAddr, icaAddr)
	s.Require().NoError(err, "fund ICA account")

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

	files, err := createICATestFiles(s.T().TempDir())
	s.Require().NoError(err, "create test files")
	s.Require().NotEmpty(files, "no test files created")

	options := &cascade.UploadOptions{ICACreatorAddress: icaAddr}
	msg, _, err := cascadeClient.CreateRequestActionMessage(ctx, icaAddr, files[0].path, options)
	s.Require().NoError(err, "build request action without app_pubkey")

	_, err = icaController.SendRequestAction(ctx, msg)
	s.Require().Error(err, "expected app_pubkey requirement failure")
	if err != nil {
		s.Contains(err.Error(), "app_pubkey")
	}

	options.AppPubkey = simdPubkey
	msg, _, err = cascadeClient.CreateRequestActionMessage(ctx, icaAddr, files[0].path, options)
	s.Require().NoError(err, "build request action with app_pubkey")

	actionRes, err := icaController.SendRequestAction(ctx, msg)
	s.Require().NoError(err, "send request with app_pubkey")
	s.Require().NotNil(actionRes, "action result missing")
	s.Require().NotEmpty(actionRes.ActionID, "empty action id response")
	_, err = actionClient.WaitForState(ctx, actionRes.ActionID, types.ActionStatePending, actionPollDelay)
	s.Require().NoError(err)
}
