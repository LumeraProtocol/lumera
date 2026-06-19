package app

import (
	"errors"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// stubMsgsTx is a minimal sdk.Tx that just carries a message slice — enough
// for the signer-extraction adapter to inspect.
type stubMsgsTx struct {
	msgs []sdk.Msg
}

func (m stubMsgsTx) GetMsgs() []sdk.Msg                  { return m.msgs }
func (m stubMsgsTx) GetMsgsV2() ([]proto.Message, error) { return nil, nil }
func (m stubMsgsTx) ValidateBasic() error                { return nil }

// recordingFallback lets us assert that the adapter delegates correctly
// for non-migration txs and does NOT delegate for migration-only txs.
type recordingFallback struct {
	called    int
	returnErr error
	returnSig []sdkmempool.SignerData
}

func (r *recordingFallback) GetSigners(_ sdk.Tx) ([]sdkmempool.SignerData, error) {
	r.called++
	return r.returnSig, r.returnErr
}

// A well-formed Lumera bech32 from a known foundation legacy address shape.
// The exact value does not matter — only that it parses as a bech32 and
// round-trips through AccAddressFromBech32.
const testLegacyBech32 = "lumera1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc58av9gw"

func TestEVMigrationSignerExtractionAdapter_MigrationOnlyTx_SyntheticSigner(t *testing.T) {
	fb := &recordingFallback{}
	adapter := newEVMigrationSignerExtractionAdapter(fb)

	tx := stubMsgsTx{
		msgs: []sdk.Msg{
			&evmigrationtypes.MsgClaimLegacyAccount{
				NewAddress:    "lumera1newaddressxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				LegacyAddress: testLegacyBech32,
			},
		},
	}

	sigs, err := adapter.GetSigners(tx)
	require.NoError(t, err)
	require.Len(t, sigs, 1, "migration-only tx must yield exactly one synthetic signer")
	expectedAcc, err := sdk.AccAddressFromBech32(testLegacyBech32)
	require.NoError(t, err)
	require.Equal(t, expectedAcc, sigs[0].Signer, "synthetic signer must equal AccAddress(legacy_address)")
	require.Equal(t, uint64(0), sigs[0].Sequence, "migration tx sequence must be 0")
	require.Zero(t, fb.called, "fallback must NOT be called for migration-only txs")
}

func TestEVMigrationSignerExtractionAdapter_MigrationOnlyTx_MigrateValidator(t *testing.T) {
	fb := &recordingFallback{}
	adapter := newEVMigrationSignerExtractionAdapter(fb)

	tx := stubMsgsTx{
		msgs: []sdk.Msg{
			&evmigrationtypes.MsgMigrateValidator{
				NewAddress:    "lumeravaloper1newvaloperxxxxxxxxxxxxxxxxxxxxxxxxx",
				LegacyAddress: testLegacyBech32,
			},
		},
	}

	sigs, err := adapter.GetSigners(tx)
	require.NoError(t, err)
	require.Len(t, sigs, 1)
	require.Equal(t, uint64(0), sigs[0].Sequence)
	require.Zero(t, fb.called)
}

func TestEVMigrationSignerExtractionAdapter_NonMigrationTx_DelegatesToFallback(t *testing.T) {
	expected := []sdkmempool.SignerData{
		sdkmempool.NewSignerData(sdk.AccAddress("dummy-signer-bytes"), 42),
	}
	fb := &recordingFallback{returnSig: expected}
	adapter := newEVMigrationSignerExtractionAdapter(fb)

	tx := stubMsgsTx{msgs: []sdk.Msg{&banktypes.MsgSend{}}}

	sigs, err := adapter.GetSigners(tx)
	require.NoError(t, err)
	require.Equal(t, 1, fb.called, "non-migration tx must delegate to fallback")
	require.Equal(t, expected, sigs, "fallback result must be returned verbatim")
}

func TestEVMigrationSignerExtractionAdapter_MixedTx_DelegatesToFallback(t *testing.T) {
	// Mixed tx (migration + non-migration message) is rejected by
	// IsEVMigrationOnlyTx, so the adapter must delegate. The mempool will
	// then see the real envelope signers — which the operator must have
	// provided — and rejection at the ante chain happens through the normal
	// fee/sig decorators rather than this adapter.
	fb := &recordingFallback{returnErr: errors.New("fallback ran")}
	adapter := newEVMigrationSignerExtractionAdapter(fb)

	tx := stubMsgsTx{
		msgs: []sdk.Msg{
			&evmigrationtypes.MsgClaimLegacyAccount{LegacyAddress: testLegacyBech32},
			&banktypes.MsgSend{},
		},
	}

	_, err := adapter.GetSigners(tx)
	require.Error(t, err)
	require.EqualError(t, err, "fallback ran")
	require.Equal(t, 1, fb.called)
}

func TestEVMigrationSignerExtractionAdapter_MultipleMigrationMessages_Rejected(t *testing.T) {
	fb := &recordingFallback{}
	adapter := newEVMigrationSignerExtractionAdapter(fb)

	tx := stubMsgsTx{
		msgs: []sdk.Msg{
			&evmigrationtypes.MsgClaimLegacyAccount{LegacyAddress: testLegacyBech32},
			&evmigrationtypes.MsgClaimLegacyAccount{LegacyAddress: testLegacyBech32},
		},
	}

	_, err := adapter.GetSigners(tx)
	require.Error(t, err, "migration txs must stay single-message so mempool identity is unambiguous")
	require.Contains(t, err.Error(), "exactly one migration message")
	require.Zero(t, fb.called)
}

func TestEVMigrationSignerExtractionAdapter_EmptyLegacyAddress_Rejected(t *testing.T) {
	fb := &recordingFallback{}
	adapter := newEVMigrationSignerExtractionAdapter(fb)

	tx := stubMsgsTx{
		msgs: []sdk.Msg{
			&evmigrationtypes.MsgClaimLegacyAccount{LegacyAddress: ""},
		},
	}

	_, err := adapter.GetSigners(tx)
	require.Error(t, err, "empty legacy_address must produce a clear adapter error")
	require.Contains(t, err.Error(), "empty legacy_address")
	require.Zero(t, fb.called)
}

func TestEVMigrationSignerExtractionAdapter_InvalidBech32_Rejected(t *testing.T) {
	fb := &recordingFallback{}
	adapter := newEVMigrationSignerExtractionAdapter(fb)

	tx := stubMsgsTx{
		msgs: []sdk.Msg{
			&evmigrationtypes.MsgClaimLegacyAccount{LegacyAddress: "not-a-bech32"},
		},
	}

	_, err := adapter.GetSigners(tx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a valid bech32")
}

func TestEVMigrationSignerExtractionAdapter_NilFallback_FallsBackToDefault(t *testing.T) {
	// Sanity check: passing nil fallback must NOT panic; a default adapter
	// is substituted. A non-migration tx using the default adapter against
	// a tx that doesn't implement SigVerifiableTx returns an error, which
	// is fine here — we just want to prove no nil-deref.
	adapter := newEVMigrationSignerExtractionAdapter(nil)
	tx := stubMsgsTx{msgs: []sdk.Msg{&banktypes.MsgSend{}}}
	_, _ = adapter.GetSigners(tx) // must not panic
}
