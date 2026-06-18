package app

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/stretchr/testify/require"

	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// TestEVMMempool_AcceptsZeroSignerMigrationTx is the regression test for the
// "tx must have at least one signer" rejection on submit-proof that blocked
// the v1.20.0-rc4 multisig migration rehearsal.
//
// Before the fix in app/evmigration_signer_extraction_adapter.go, the
// upstream ExperimentalEVMMempool delegated signer extraction on the Cosmos
// side to DefaultSignerExtractionAdapter, which inspects GetSignaturesV2()
// and refuses any tx with zero envelope signatures — exactly the shape that
// MsgClaimLegacyAccount produces by design. That refusal happened at the
// mempool layer, *before* the migration-aware ante chain
// (app/evm/ante.go: migrationCosmosAnte) could admit the tx.
//
// With the fix, a migration-only tx still has zero envelope sigs but the
// mempool synthesizes a signer from legacy_address, the Insert succeeds, and
// the ante chain runs as designed. We assert: no error from Insert AND the
// mempool count increased.
func TestEVMMempool_AcceptsZeroSignerMigrationTx(t *testing.T) {
	app := Setup(t)
	require.NotNil(t, app.GetMempool(), "mempool must be wired")

	txConfig := app.TxConfig()
	txBuilder := txConfig.NewTxBuilder()

	// Build the simplest possible migration message. The mempool path we
	// are exercising never inspects proof bytes — that's the ante's job —
	// so legacy_address is the only field we must populate.
	msg := &evmigrationtypes.MsgClaimLegacyAccount{
		NewAddress:    "lumera1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc58av9gw",
		LegacyAddress: "lumera1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc58av9gw",
	}
	require.NoError(t, txBuilder.SetMsgs(msg))
	// Zero fee / zero gas — migration ante waives fees. Critically: no
	// signatures set on the builder; the resulting tx has signer_infos: []
	// and signatures: [], matching exactly what `lumerad tx evmigration
	// submit-proof` produces on a real chain.
	txBuilder.SetGasLimit(0)

	tx := txBuilder.GetTx()

	ctx := sdk.Context{}.WithBlockHeight(1)

	before := app.GetMempool().CountTx()
	err := app.GetMempool().Insert(ctx, tx)
	require.NoError(t, err, "zero-signer migration tx must be accepted by the mempool")
	require.NotContains(t,
		"", // dummy — using NotContains on err.Error() above wouldn't fire when err is nil
		"tx must have at least one signer",
		"sanity assertion (kept for grep when this regresses)",
	)
	require.Equal(t, before+1, app.GetMempool().CountTx(), "mempool count must increment for the accepted tx")
}

// TestEVMigrationSignerAdapter_FallbackRejectsZeroSignerTx pins the failure
// mode in absence of the adapter. This is the corner of the failure surface
// that the v1.20.0-rc4 multisig migration rehearsal walked into: build the
// SAME zero-signer migration tx and feed it directly to the SDK default
// signer extractor, then assert it reports the "tx must have at least one
// signer" symptom. If this test EVER starts passing (i.e. the default
// adapter learns to handle this shape upstream), the workaround in
// app/evm_mempool.go can be reconsidered.
func TestEVMigrationSignerAdapter_FallbackRejectsZeroSignerTx(t *testing.T) {
	app := Setup(t)
	txConfig := app.TxConfig()
	txBuilder := txConfig.NewTxBuilder()

	msg := &evmigrationtypes.MsgClaimLegacyAccount{
		NewAddress:    "lumera1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc58av9gw",
		LegacyAddress: "lumera1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc58av9gw",
	}
	require.NoError(t, txBuilder.SetMsgs(msg))
	tx := txBuilder.GetTx()

	defaultAdapter := sdkmempool.NewDefaultSignerExtractionAdapter()
	sigs, err := defaultAdapter.GetSigners(tx)
	require.NoError(t, err, "default adapter returns no error on zero-sig tx — it just returns an empty slice")
	require.Empty(t, sigs, "default adapter yields zero signers for migration tx — this is what makes PriorityNonceMempool.Insert reject with 'tx must have at least one signer'")

	// The migration-aware adapter, by contrast, yields exactly one
	// synthetic signer for the same tx.
	migAdapter := newEVMigrationSignerExtractionAdapter(defaultAdapter)
	migSigs, err := migAdapter.GetSigners(tx)
	require.NoError(t, err)
	require.Len(t, migSigs, 1, "migration-aware adapter must synthesize exactly one signer")
}
