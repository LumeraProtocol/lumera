package ante

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	// Blank import to seal SDK bech32 prefixes to "lumera"; required for
	// MsgClaimLegacyAccount.ValidateBasic, which decodes the legacy/new
	// addresses via sdk.AccAddressFromBech32.
	_ "github.com/LumeraProtocol/lumera/config"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

type mockValidateBasicTx struct {
	msgs []sdk.Msg
	err  error
}

func (m mockValidateBasicTx) GetMsgs() []sdk.Msg { return m.msgs }

func (m mockValidateBasicTx) GetMsgsV2() ([]proto.Message, error) { return nil, nil }

func (m mockValidateBasicTx) ValidateBasic() error { return m.err }

func noopAnteHandler(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
	return ctx, nil
}

// TestEVMigrationValidateBasicDecorator_AllowsMissingTxSignatures verifies that
// migration-only txs can omit Cosmos tx signatures while still using tx-level
// basic validation for all other errors.
func TestEVMigrationValidateBasicDecorator_AllowsMissingTxSignatures(t *testing.T) {
	t.Parallel()

	dec := EVMigrationValidateBasicDecorator{}
	tx := mockValidateBasicTx{
		msgs: []sdk.Msg{&evmigrationtypes.MsgClaimLegacyAccount{}},
		err:  sdkerrors.ErrNoSignatures,
	}

	called := false
	_, err := dec.AnteHandle(sdk.Context{}, tx, false, func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		called = true
		return ctx, nil
	})
	require.NoError(t, err)
	require.True(t, called)
}

// TestEVMigrationValidateBasicDecorator_RejectsOtherErrors verifies that the
// decorator only suppresses ErrNoSignatures for migration-only txs.
func TestEVMigrationValidateBasicDecorator_RejectsOtherErrors(t *testing.T) {
	t.Parallel()

	dec := EVMigrationValidateBasicDecorator{}
	tx := mockValidateBasicTx{
		msgs: []sdk.Msg{&evmigrationtypes.MsgClaimLegacyAccount{}},
		err:  sdkerrors.ErrInvalidAddress,
	}

	_, err := dec.AnteHandle(sdk.Context{}, tx, false, noopAnteHandler)
	require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
}

// TestEVMigrationValidateBasicDecorator_NonMigrationStillRequiresSigs verifies
// that regular txs keep the SDK's no-signature rejection.
func TestEVMigrationValidateBasicDecorator_NonMigrationStillRequiresSigs(t *testing.T) {
	t.Parallel()

	dec := EVMigrationValidateBasicDecorator{}
	tx := mockValidateBasicTx{
		msgs: []sdk.Msg{&banktypes.MsgSend{}},
		err:  sdkerrors.ErrNoSignatures,
	}

	_, err := dec.AnteHandle(sdk.Context{}, tx, false, noopAnteHandler)
	require.ErrorIs(t, err, sdkerrors.ErrNoSignatures)
}

// realValidateBasicTx invokes each message's actual ValidateBasic instead of
// returning a synthetic error. Used by the mirror-source rejection test below
// to exercise the full Msg*.ValidateBasic chain through the ante decorator.
type realValidateBasicTx struct {
	msgs []sdk.Msg
}

func (t realValidateBasicTx) GetMsgs() []sdk.Msg { return t.msgs }

func (t realValidateBasicTx) GetMsgsV2() ([]proto.Message, error) { return nil, nil }

func (t realValidateBasicTx) ValidateBasic() error {
	for _, msg := range t.msgs {
		if vb, ok := msg.(sdk.HasValidateBasic); ok {
			if err := vb.ValidateBasic(); err != nil {
				return err
			}
		}
	}
	return nil
}

// TestEVMigrationValidateBasicDecorator_RejectsRealMirrorSourceMismatch builds
// a real MsgClaimLegacyAccount with a multisig legacy proof paired against a
// single-key new proof — a shape mismatch — and submits it to the decorator.
// The per-side proof structures are individually well-formed, so per-side
// ValidateBasic passes; the cross-side ValidateProofPair is what rejects the
// pair with ErrMirrorSourceMismatch. This complements the synthetic-error
// tests above by proving the real chain MsgClaimLegacyAccount.ValidateBasic
// → ValidateProofPair → ErrMirrorSourceMismatch propagates through the ante
// decorator end to end.
func TestEVMigrationValidateBasicDecorator_RejectsRealMirrorSourceMismatch(t *testing.T) {
	t.Parallel()

	// Two distinct 20-byte addresses; .String() honors the lumera prefix
	// established by the blank config import at package load.
	legacyAddr := sdk.AccAddress(make([]byte, 20))
	newAddrBytes := make([]byte, 20)
	newAddrBytes[19] = 1
	newAddr := sdk.AccAddress(newAddrBytes)
	require.NotEqual(t, legacyAddr.String(), newAddr.String())

	// Helper: 33-byte placeholder pubkey filled with a single byte. Distinct
	// fill bytes guarantee MultisigProof's duplicate-sub-key check passes.
	pk := func(b byte) []byte {
		out := make([]byte, secp256k1.PubKeySize)
		for i := range out {
			out[i] = b
		}
		return out
	}

	legacyProof := evmigrationtypes.MigrationProof{
		Proof: &evmigrationtypes.MigrationProof_Multisig{
			Multisig: &evmigrationtypes.MultisigProof{
				SubPubKeys:    [][]byte{pk(0x01), pk(0x02), pk(0x03)},
				Threshold:     2,
				SignerIndices: []uint32{0, 2},
				// 64-byte sub-sigs are the legacy-side requirement.
				SubSignatures: [][]byte{make([]byte, 64), make([]byte, 64)},
				SigFormat:     evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
			},
		},
	}
	newProof := evmigrationtypes.MigrationProof{
		Proof: &evmigrationtypes.MigrationProof_Single{
			Single: &evmigrationtypes.SingleKeyProof{
				PubKey:    pk(0x04),
				Signature: make([]byte, 65), // new-side eth_secp256k1: 65 bytes R||S||V
				SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
			},
		},
	}

	msg := &evmigrationtypes.MsgClaimLegacyAccount{
		LegacyAddress: legacyAddr.String(),
		NewAddress:    newAddr.String(),
		LegacyProof:   legacyProof,
		NewProof:      newProof,
	}

	dec := EVMigrationValidateBasicDecorator{}
	tx := realValidateBasicTx{msgs: []sdk.Msg{msg}}

	_, err := dec.AnteHandle(sdk.Context{}, tx, false, noopAnteHandler)
	require.Error(t, err)
	require.ErrorIs(t, err, evmigrationtypes.ErrMirrorSourceMismatch)
	require.ErrorContains(t, err, "shape")
}
