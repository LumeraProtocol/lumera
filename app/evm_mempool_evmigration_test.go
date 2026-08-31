package app_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	lcfg "github.com/LumeraProtocol/lumera/config"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// testChainID matches the chain-id used by Setup(t) (see app/test_helpers.go).
const testChainID = "testing"

const testLegacyBech32 = "lumera1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc58av9gw"

// TestEVMMempool_CheckTxAcceptsZeroSignerMigrationTx is the end-to-end
// regression test for the production bug behind PR #167.
//
// Real flow on a v1.20.0 mainnet binary: an operator runs
//
//	lumerad tx evmigration submit-proof tx.json
//
// which posts the encoded tx bytes to BaseApp.CheckTx. CheckTx (with the
// experimental EVM mempool wired) runs:
//
//  1. ante chain (migrationCosmosAnte for migration-only txs — accepts
//     zero-signer txs by design)
//  2. app.mempool.Insert(ctx, tx) — which delegates signer extraction to
//     the configured SignerExtractionAdapter.
//
// Before the fix, step (2) used DefaultSignerExtractionAdapter which returns
// an empty []SignerData for a zero-signer migration tx, causing
// PriorityNonceMempool.Insert to reject with
// "tx must have at least one signer". This test goes through the EXACT same
// CheckTx entry point an operator hits and asserts the response succeeds.
//
// This is a stronger test than calling app.GetMempool().Insert(...) directly
// because it drives the same code path the live binary uses on broadcast.
func TestEVMMempool_CheckTxAcceptsZeroSignerMigrationTx(t *testing.T) {
	legacyPriv := secp256k1.GenPrivKey()
	app := setupAppWithLegacyAccountForMempool(t, legacyPriv)

	msg := validMigrationMsgForMempoolWithLegacy(t, testChainID, legacyPriv)
	tx := newUnsignedMigrationTxForMempool(t, app, msg)

	txBytes, err := app.TxConfig().TxEncoder()(tx)
	require.NoError(t, err)

	resp, err := app.CheckTx(&abci.RequestCheckTx{
		Tx:   txBytes,
		Type: abci.CheckTxType_New,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Hard assertion against the symptom: the CheckTx log must NEVER contain
	// the mempool's "at least one signer" rejection. If this string appears,
	// the EVM mempool fix has regressed.
	require.NotContains(t, resp.Log, "at least one signer",
		"CheckTx must not surface the mempool's zero-signer rejection on a valid migration tx")

	// Acceptance: the migration tx must reach CheckTx success (code 0).
	// If the ante or mempool rejects for any other reason, fail loudly so the
	// failure mode is visible — this test is the canary for the full CheckTx
	// path on submit-proof.
	require.Zero(t, resp.Code,
		"CheckTx must accept a valid zero-signer migration tx (code=0); got code=%d log=%q",
		resp.Code, resp.Log)
}

func TestEVMMempool_CheckTxRejectsProofValidNonexistentLegacyAccount(t *testing.T) {
	app := lumeraapp.Setup(t)

	msg := validMigrationMsgForMempool(t, testChainID)
	tx := newUnsignedMigrationTxForMempool(t, app, msg)

	txBytes, err := app.TxConfig().TxEncoder()(tx)
	require.NoError(t, err)

	resp, err := app.CheckTx(&abci.RequestCheckTx{
		Tx:   txBytes,
		Type: abci.CheckTxType_New,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotZero(t, resp.Code)
	require.Contains(t, resp.Log, "legacy account not found",
		"proof-valid migration txs from nonexistent legacy accounts must fail at ante admission")
	require.NotContains(t, resp.Log, "at least one signer")
}

// TestEVMMempool_CheckTxRejectsZeroSignerNonMigrationTx is the end-to-end
// defense-in-depth pin: a zero-signer banktypes.MsgSend submitted through the
// same CheckTx entry point an operator hits must still be rejected.
//
// LAYERING NOTE: this rejection comes from the SDK signature-verification
// decorator in the ante chain ("no signatures supplied", codespace "sdk",
// code ErrNoSignatures=15), which runs BEFORE mempool admission. It therefore
// does NOT exercise the signer-extraction adapter at all — the ante stops the
// tx first. This test only proves the live path still rejects a malicious
// zero-signer non-migration tx; it cannot, on its own, detect an adapter that
// widened the hole, because the ante would mask such a regression here.
//
// The adapter-layer security guarantee — that a non-migration tx gets NO
// synthetic signer and is rejected at mempool admission — is pinned directly,
// bypassing the ante, by TestEVMMempool_InsertRejectsZeroSignerNonMigrationTx
// below, and at the unit level by
// TestEVMigrationSignerExtractionAdapter_NonMigrationTx_DelegatesToFallback.
func TestEVMMempool_CheckTxRejectsZeroSignerNonMigrationTx(t *testing.T) {
	app := lumeraapp.Setup(t)

	from := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	to := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	msg := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)))

	txBuilder := app.TxConfig().NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msg))
	txBuilder.SetGasLimit(200_000)
	// Deliberately NO signatures set: this mirrors a malicious or buggy
	// operator submitting a zero-signer tx for a non-migration message.
	tx := txBuilder.GetTx()

	txBytes, err := app.TxConfig().TxEncoder()(tx)
	require.NoError(t, err)

	resp, err := app.CheckTx(&abci.RequestCheckTx{
		Tx:   txBytes,
		Type: abci.CheckTxType_New,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotZero(t, resp.Code,
		"zero-signer NON-migration tx must be rejected by CheckTx; got code=0 log=%q", resp.Log)
	// Pin the rejecting layer: the ante's signature verification, not the
	// mempool. If this assertion starts failing, the rejection moved layers
	// and the comment above (and the division of coverage with the Insert
	// test) needs revisiting.
	require.Contains(t, resp.Log, "no signatures supplied",
		"expected ante signature-verification rejection; a different layer/message means the security coverage split has shifted")
}

// TestEVMMempool_InsertRejectsZeroSignerNonMigrationTx is the true adapter-layer
// security pin. It drives app.GetMempool().Insert directly — bypassing the ante
// — so the SignerExtractionAdapter is actually exercised. A zero-signer
// non-migration tx must NOT receive a synthetic signer: IsEVMigrationOnlyTx is
// false for a bank message, the adapter delegates to the SDK default extractor
// (which yields zero signers), and PriorityNonceMempool.Insert then rejects
// with "tx must have at least one signer".
//
// If this test ever turns green, the adapter HAS widened the hole — a
// non-migration message type would be admitted to the mempool without envelope
// signatures, which is exactly the security regression we promised would not
// happen. This is the assertion the CheckTx test above cannot make, because the
// ante masks the mempool layer there.
func TestEVMMempool_InsertRejectsZeroSignerNonMigrationTx(t *testing.T) {
	app := lumeraapp.Setup(t)

	from := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	to := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	bankMsg := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)))
	tx := newUnsignedMigrationTxForMempool(t, app, bankMsg)

	ctx := sdk.Context{}.WithBlockHeight(1)
	before := app.GetMempool().CountTx()
	err := app.GetMempool().Insert(ctx, tx)
	require.Error(t, err, "zero-signer non-migration tx must not be admitted to the mempool")
	require.Contains(t, err.Error(), "tx must have at least one signer",
		"adapter must delegate non-migration txs to the default extractor, not synthesize a signer")
	require.Equal(t, before, app.GetMempool().CountTx())
}

// TestEVMigrationSignerAdapter_DefaultExtractor_PinsFailureMode pins the
// SDK-side behavior that necessitates the custom adapter. The default
// extractor returns an empty signer slice for a zero-signer migration tx,
// which is what PriorityNonceMempool.Insert turns into the
// "tx must have at least one signer" error. The migration-aware adapter
// returns exactly one synthetic signer for the same tx. If the default
// extractor ever learns to handle this shape upstream, the workaround can
// be reconsidered.
func TestEVMigrationSignerAdapter_DefaultExtractor_PinsFailureMode(t *testing.T) {
	app := lumeraapp.Setup(t)

	msg := validMigrationMsgForMempool(t, testChainID)
	tx := newUnsignedMigrationTxForMempool(t, app, msg)

	defaultAdapter := sdkmempool.NewDefaultSignerExtractionAdapter()
	sigs, err := defaultAdapter.GetSigners(tx)
	require.NoError(t, err, "default adapter returns no error on zero-sig tx — it just returns an empty slice")
	require.Empty(t, sigs, "default adapter yields zero signers for migration tx — this is what makes PriorityNonceMempool.Insert reject with 'tx must have at least one signer'")
}

func TestEVMMempool_SDKPriorityNonceMempoolRejectsZeroSignerMigrationTx(t *testing.T) {
	app := lumeraapp.Setup(t)

	msg := validMigrationMsgForMempool(t, testChainID)
	tx := newUnsignedMigrationTxForMempool(t, app, msg)

	pool := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{})
	err := pool.Insert(sdk.Context{}.WithBlockHeight(1), tx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tx must have at least one signer")
	require.Zero(t, pool.CountTx())
}

func TestEVMMempool_InsertAcceptsZeroSignerValidatorMigrationTx(t *testing.T) {
	app := lumeraapp.Setup(t)

	msg := &evmigrationtypes.MsgMigrateValidator{
		NewAddress:    "lumera1ttwdmmlqf8xu5mkufrh5zcck8v8yn42a5m0xpg",
		LegacyAddress: testLegacyBech32,
	}
	tx := newUnsignedMigrationTxForMempool(t, app, msg)

	ctx := sdk.Context{}.WithBlockHeight(1)
	before := app.GetMempool().CountTx()
	err := app.GetMempool().Insert(ctx, tx)
	require.NoError(t, err)
	require.Equal(t, before+1, app.GetMempool().CountTx())
}

func TestEVMMempool_InsertRejectsMalformedMigrationLegacyAddress(t *testing.T) {
	app := lumeraapp.Setup(t)

	msg := &evmigrationtypes.MsgClaimLegacyAccount{
		NewAddress:    "lumera1ttwdmmlqf8xu5mkufrh5zcck8v8yn42a5m0xpg",
		LegacyAddress: "not-a-bech32",
	}
	tx := newUnsignedMigrationTxForMempool(t, app, msg)

	ctx := sdk.Context{}.WithBlockHeight(1)
	before := app.GetMempool().CountTx()
	err := app.GetMempool().Insert(ctx, tx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a valid bech32")
	require.Equal(t, before, app.GetMempool().CountTx())
}

func TestEVMMempool_InsertRejectsZeroSignerMixedMigrationTx(t *testing.T) {
	app := lumeraapp.Setup(t)

	from := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	to := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	bankMsg := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)))
	migrationMsg := &evmigrationtypes.MsgClaimLegacyAccount{
		NewAddress:    "lumera1ttwdmmlqf8xu5mkufrh5zcck8v8yn42a5m0xpg",
		LegacyAddress: testLegacyBech32,
	}
	tx := newUnsignedMigrationTxForMempool(t, app, migrationMsg, bankMsg)

	ctx := sdk.Context{}.WithBlockHeight(1)
	before := app.GetMempool().CountTx()
	err := app.GetMempool().Insert(ctx, tx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tx must have at least one signer")
	require.Equal(t, before, app.GetMempool().CountTx())
}

func TestEVMMempool_DuplicateLegacyMigrationTxDoesNotGrowMempool(t *testing.T) {
	app := lumeraapp.Setup(t)

	msg := &evmigrationtypes.MsgClaimLegacyAccount{
		NewAddress:    "lumera1ttwdmmlqf8xu5mkufrh5zcck8v8yn42a5m0xpg",
		LegacyAddress: testLegacyBech32,
	}
	tx := newUnsignedMigrationTxForMempool(t, app, msg)

	ctx := sdk.Context{}.WithBlockHeight(1)
	require.NoError(t, app.GetMempool().Insert(ctx, tx))
	require.Equal(t, 1, app.GetMempool().CountTx())

	require.NoError(t, app.GetMempool().Insert(ctx, tx))
	require.Equal(t, 1, app.GetMempool().CountTx(), "same legacy_address + sequence must remain one mempool entry")
}

func TestEVMMempool_PrepareProposalIncludesZeroSignerMigrationTx(t *testing.T) {
	legacyPriv := secp256k1.GenPrivKey()
	app := setupAppWithLegacyAccountForMempool(t, legacyPriv)

	msg := validMigrationMsgForMempoolWithLegacy(t, testChainID, legacyPriv)
	tx := newUnsignedMigrationTxForMempool(t, app, msg)
	txBytes, err := app.TxConfig().TxEncoder()(tx)
	require.NoError(t, err)

	require.NoError(t, app.GetMempool().Insert(sdk.Context{}.WithBlockHeight(1), tx))

	resp, err := app.PrepareProposal(&abci.RequestPrepareProposal{
		Height:     app.LastBlockHeight() + 1,
		MaxTxBytes: int64(len(txBytes) + 1024),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Txs, 1)
	require.Equal(t, txBytes, resp.Txs[0])
}

// validMigrationMsgForMempool builds a MsgClaimLegacyAccount whose embedded
// proofs pass ante-level cryptographic verification. Tests that expect CheckTx
// acceptance must also seed the legacy account so state admission passes.
//
// This mirrors validMigrationMsg in app/evm/ante_evmigration_fee_test.go but
// lives here to avoid a cross-package test-only export.
func validMigrationMsgForMempool(t *testing.T, chainID string) *evmigrationtypes.MsgClaimLegacyAccount {
	t.Helper()

	legacyPriv := secp256k1.GenPrivKey()
	return validMigrationMsgForMempoolWithLegacy(t, chainID, legacyPriv)
}

func validMigrationMsgForMempoolWithLegacy(
	t *testing.T,
	chainID string,
	legacyPriv *secp256k1.PrivKey,
) *evmigrationtypes.MsgClaimLegacyAccount {
	t.Helper()

	newPriv, err := evmcryptotypes.GenerateKey()
	require.NoError(t, err)

	legacy := sdk.AccAddress(legacyPriv.PubKey().Address().Bytes())
	newAddr := sdk.AccAddress(newPriv.PubKey().Address().Bytes())
	require.False(t, legacy.Equals(newAddr))

	payload := []byte(fmt.Sprintf(
		"lumera-evm-migration:%s:%d:claim:%s:%s",
		chainID,
		lcfg.EVMChainID,
		legacy.String(),
		newAddr.String(),
	))
	legacyHash := sha256.Sum256(payload)
	legacySig, err := legacyPriv.Sign(legacyHash[:])
	require.NoError(t, err)

	newSig, err := newPriv.Sign(payload)
	require.NoError(t, err)

	return &evmigrationtypes.MsgClaimLegacyAccount{
		LegacyAddress: legacy.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: evmigrationtypes.MigrationProof{Proof: &evmigrationtypes.MigrationProof_Single{Single: &evmigrationtypes.SingleKeyProof{
			PubKey:    legacyPriv.PubKey().Bytes(),
			Signature: legacySig,
			SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: evmigrationtypes.MigrationProof{Proof: &evmigrationtypes.MigrationProof_Single{Single: &evmigrationtypes.SingleKeyProof{
			PubKey:    newPriv.PubKey().Bytes(),
			Signature: newSig,
			SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
}

func newUnsignedMigrationTxForMempool(t *testing.T, app *lumeraapp.App, msgs ...sdk.Msg) sdk.Tx {
	t.Helper()

	txBuilder := app.TxConfig().NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msgs...))
	txBuilder.SetGasLimit(200_000)
	return txBuilder.GetTx()
}

func setupAppWithLegacyAccountForMempool(t *testing.T, legacyPriv *secp256k1.PrivKey) *lumeraapp.App {
	t.Helper()

	privVal := cmttypes.NewMockPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)

	validator := cmttypes.NewValidator(pubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{validator})

	legacyAddr := sdk.AccAddress(legacyPriv.PubKey().Address().Bytes())
	legacyAcc := authtypes.NewBaseAccount(legacyAddr, legacyPriv.PubKey(), 0, 0)
	genBals := []banktypes.Balance{
		{
			Address: legacyAddr.String(),
			Coins:   sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 100_000_000_000_000)),
		},
	}

	return lumeraapp.SetupWithGenesisValSet(
		t,
		valSet,
		[]authtypes.GenesisAccount{legacyAcc},
		testChainID,
		sdk.DefaultPowerReduction,
		genBals,
	)
}
