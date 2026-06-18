package app_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	lcfg "github.com/LumeraProtocol/lumera/config"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// testChainID matches the chain-id used by Setup(t) (see app/test_helpers.go).
const testChainID = "testing"

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
// CheckTx entry point an operator hits and asserts the response is non-zero.
//
// This is a stronger test than calling app.GetMempool().Insert(...) directly
// because it exercises the proposer-pool wiring as well, and because it
// drives the same code path the live binary uses on broadcast.
func TestEVMMempool_CheckTxAcceptsZeroSignerMigrationTx(t *testing.T) {
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

// TestEVMMempool_CheckTxRejectsZeroSignerNonMigrationTx is the security pin
// Andrey asked for: the SignerExtractionAdapter fix MUST NOT loosen mempool
// checks for any tx that isn't a payload-authenticated migration message.
// A zero-signer banktypes.MsgSend submitted through the same CheckTx entry
// point must still be rejected.
//
// If this test ever turns green, the adapter has widened the hole — every
// non-migration message type would then be able to bypass mempool signer
// extraction, which is exactly the security regression we promised would
// not happen.
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
		"zero-signer NON-migration tx must be rejected by CheckTx; got code=0 (security regression — adapter widened the hole)")
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

// validMigrationMsgForMempool builds a MsgClaimLegacyAccount whose embedded
// proofs pass ante-level cryptographic verification, so the only thing that
// can reject the tx in CheckTx is the mempool's signer-extraction step.
//
// This mirrors validMigrationMsg in app/evm/ante_evmigration_fee_test.go but
// lives here to avoid a cross-package test-only export.
func validMigrationMsgForMempool(t *testing.T, chainID string) *evmigrationtypes.MsgClaimLegacyAccount {
	t.Helper()

	legacyPriv := secp256k1.GenPrivKey()
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
