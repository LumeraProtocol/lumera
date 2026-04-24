package types_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

func validAddr() string {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
}

// validSingleProof returns a MigrationProof with a well-formed SingleKeyProof
// using legacy-side rules (Cosmos secp256k1, 64-byte signature).
func validSingleProof(pub *secp256k1.PubKey) types.MigrationProof {
	return types.MigrationProof{
		Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    pub.Key,
			Signature: make([]byte, 64),
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}},
	}
}

// validNewProof returns a MigrationProof with a well-formed new-side SingleKeyProof
// (eth_secp256k1, 65-byte R||S||V signature, EIP-191 format).
func validNewProof() types.MigrationProof {
	return types.MigrationProof{
		Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    make([]byte, 33),
			Signature: make([]byte, 65),
			SigFormat: types.SigFormat_SIG_FORMAT_EIP191,
		}},
	}
}

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgUpdateParams
		wantErr error
	}{
		{
			name: "valid",
			msg: types.MsgUpdateParams{
				Authority: validAddr(),
				Params:    types.DefaultParams(),
			},
		},
		{
			name: "invalid authority",
			msg: types.MsgUpdateParams{
				Authority: "bad",
				Params:    types.DefaultParams(),
			},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid params",
			msg: types.MsgUpdateParams{
				Authority: validAddr(),
				Params:    types.NewParams(true, 0, 0, 100, 20),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				// "invalid params" case returns a non-nil error from Params.Validate()
				// but it's not a sentinel error, so just check it's returned
				if tc.name == "invalid params" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestMsgClaimLegacyAccount_ValidateBasic(t *testing.T) {
	legacyKey := secp256k1.GenPrivKey()
	legacyPub := legacyKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(legacyPub.Address()).String()
	newAddr := validAddr()

	goodProof := validSingleProof(legacyPub)

	goodNewProof := validNewProof()

	tests := []struct {
		name    string
		msg     types.MsgClaimLegacyAccount
		wantErr error
	}{
		{
			name: "valid",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:    newAddr,
				LegacyAddress: legacyAddr,
				LegacyProof:   goodProof,
				NewProof:      goodNewProof,
			},
		},
		{
			name: "invalid new_address",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:    "bad",
				LegacyAddress: legacyAddr,
				LegacyProof:   goodProof,
				NewProof:      goodNewProof,
			},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid legacy_address",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:    newAddr,
				LegacyAddress: "bad",
				LegacyProof:   goodProof,
				NewProof:      goodNewProof,
			},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "same address",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:    legacyAddr,
				LegacyAddress: legacyAddr,
				LegacyProof:   goodProof,
				NewProof:      goodNewProof,
			},
			wantErr: types.ErrSameAddress,
		},
		{
			name: "invalid pubkey size",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:    newAddr,
				LegacyAddress: legacyAddr,
				LegacyProof: types.MigrationProof{
					Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
						PubKey:    []byte{0x01, 0x02},
						Signature: make([]byte, 64),
						SigFormat: types.SigFormat_SIG_FORMAT_CLI,
					}},
				},
				NewProof: goodNewProof,
			},
			wantErr: types.ErrInvalidMigrationPubKey,
		},
		{
			name: "wrong legacy signature length",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:    newAddr,
				LegacyAddress: legacyAddr,
				LegacyProof: types.MigrationProof{
					Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
						PubKey:    legacyPub.Key,
						Signature: nil,
						SigFormat: types.SigFormat_SIG_FORMAT_CLI,
					}},
				},
				NewProof: goodNewProof,
			},
			wantErr: types.ErrInvalidMigrationSignature,
		},
		{
			name: "missing new proof",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:    newAddr,
				LegacyAddress: legacyAddr,
				LegacyProof:   goodProof,
			},
			wantErr: types.ErrInvalidMigrationProof,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgMigrateValidator_ValidateBasic(t *testing.T) {
	legacyKey := secp256k1.GenPrivKey()
	legacyPub := legacyKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(legacyPub.Address()).String()
	newAddr := validAddr()

	goodProof := validSingleProof(legacyPub)

	goodNewProof := validNewProof()

	tests := []struct {
		name    string
		msg     types.MsgMigrateValidator
		wantErr error
	}{
		{
			name: "valid",
			msg: types.MsgMigrateValidator{
				NewAddress:    newAddr,
				LegacyAddress: legacyAddr,
				LegacyProof:   goodProof,
				NewProof:      goodNewProof,
			},
		},
		{
			name: "invalid new_address",
			msg: types.MsgMigrateValidator{
				NewAddress:    "bad",
				LegacyAddress: legacyAddr,
				LegacyProof:   goodProof,
				NewProof:      goodNewProof,
			},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "same address",
			msg: types.MsgMigrateValidator{
				NewAddress:    legacyAddr,
				LegacyAddress: legacyAddr,
				LegacyProof:   goodProof,
				NewProof:      goodNewProof,
			},
			wantErr: types.ErrSameAddress,
		},
		{
			name: "missing new proof",
			msg: types.MsgMigrateValidator{
				NewAddress:    newAddr,
				LegacyAddress: legacyAddr,
				LegacyProof:   goodProof,
			},
			wantErr: types.ErrInvalidMigrationProof,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// validMultisigProof returns a MigrationProof carrying a well-formed K-of-N
// MultisigProof on the requested side. All sub-keys are random secp256k1
// pubkeys; signatures are zero-filled at the per-side expected length.
func validMultisigProof(threshold, n int, side types.Side) types.MigrationProof {
	subs := make([][]byte, n)
	for i := range n {
		subs[i] = secp256k1.GenPrivKey().PubKey().(*secp256k1.PubKey).Key
	}
	sigLen := 64
	if side == types.SideNew {
		sigLen = 65
	}
	indices := make([]uint32, threshold)
	sigs := make([][]byte, threshold)
	for i := range threshold {
		indices[i] = uint32(i)
		sigs[i] = make([]byte, sigLen)
	}
	return types.MigrationProof{
		Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
			Threshold:     uint32(threshold),
			SubPubKeys:    subs,
			SignerIndices: indices,
			SubSignatures: sigs,
			SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
		}},
	}
}

// TestValidateProofPair_MirrorSourceRule exercises the 6-case matrix that
// defines the consensus-level mirror-source invariant:
//   - single↔single and multi↔multi with matching K/N must pass;
//   - any shape mismatch (single↔multi, multi↔single) or K/N mismatch
//     between two multisig sides must fail with ErrMirrorSourceMismatch.
func TestValidateProofPair_MirrorSourceRule(t *testing.T) {
	legacySingle := validSingleProof(secp256k1.GenPrivKey().PubKey().(*secp256k1.PubKey))
	newSingle := validNewProof()
	legacyMulti2of3 := validMultisigProof(2, 3, types.SideLegacy)
	newMulti2of3 := validMultisigProof(2, 3, types.SideNew)
	newMulti1of1 := validMultisigProof(1, 1, types.SideNew)
	newMulti3of5 := validMultisigProof(3, 5, types.SideNew)

	tests := []struct {
		name    string
		legacy  types.MigrationProof
		newP    types.MigrationProof
		wantErr error
	}{
		{name: "single/single ok", legacy: legacySingle, newP: newSingle},
		{name: "multi2of3/multi2of3 ok", legacy: legacyMulti2of3, newP: newMulti2of3},
		{name: "shape: legacy single, new multi", legacy: legacySingle, newP: newMulti2of3, wantErr: types.ErrMirrorSourceMismatch},
		{name: "shape: legacy multi, new single", legacy: legacyMulti2of3, newP: newSingle, wantErr: types.ErrMirrorSourceMismatch},
		{name: "K mismatch: 2of3 -> 3of5", legacy: legacyMulti2of3, newP: newMulti3of5, wantErr: types.ErrMirrorSourceMismatch},
		{name: "N mismatch + K mismatch: 2of3 -> 1of1", legacy: legacyMulti2of3, newP: newMulti1of1, wantErr: types.ErrMirrorSourceMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			legacy := tc.legacy
			newP := tc.newP
			err := types.ValidateProofPair(&legacy, &newP)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateProofPair_NilInputsReturnErrorNotPanic guards the helper against
// nil proofs / typed-nil oneof wrappers / nil inner MultisigProof. Direct
// callers (tests, tooling, future refactors) shouldn't be able to panic it.
func TestValidateProofPair_NilInputsReturnErrorNotPanic(t *testing.T) {
	goodMulti := validMultisigProof(2, 3, types.SideLegacy)
	nilInnerMulti := types.MigrationProof{
		Proof: &types.MigrationProof_Multisig{Multisig: nil},
	}
	nilOneof := types.MigrationProof{}
	tests := []struct {
		name           string
		legacy, newP   *types.MigrationProof
		wantErrIs      error
		wantErrContain string
	}{
		{name: "both nil pointers", legacy: nil, newP: nil, wantErrIs: types.ErrInvalidMigrationProof},
		{name: "legacy nil pointer", legacy: nil, newP: &goodMulti, wantErrIs: types.ErrInvalidMigrationProof},
		{name: "new nil pointer", legacy: &goodMulti, newP: nil, wantErrIs: types.ErrInvalidMigrationProof},
		{name: "legacy multisig with nil inner", legacy: &nilInnerMulti, newP: &goodMulti, wantErrIs: types.ErrInvalidMigrationProof, wantErrContain: "legacy multisig"},
		{name: "new multisig with nil inner", legacy: &goodMulti, newP: &nilInnerMulti, wantErrIs: types.ErrInvalidMigrationProof, wantErrContain: "new multisig"},
		{name: "legacy oneof unset", legacy: &nilOneof, newP: &goodMulti, wantErrIs: types.ErrMirrorSourceMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateProofPair(tc.legacy, tc.newP)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.wantErrIs)
			if tc.wantErrContain != "" {
				require.Contains(t, err.Error(), tc.wantErrContain)
			}
		})
	}
}

// TestMsgClaimLegacyAccount_ValidateBasic_MirrorSource confirms the consensus
// check fires through the full ValidateBasic path (not just the helper).
func TestMsgClaimLegacyAccount_ValidateBasic_MirrorSource(t *testing.T) {
	legacyKey := secp256k1.GenPrivKey()
	legacyPub := legacyKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(legacyPub.Address()).String()
	newAddr := validAddr()

	// 2-of-3 legacy → 1-of-1 new: shape and K/N both diverge.
	legMulti := validMultisigProof(2, 3, types.SideLegacy)
	newMulti1of1 := validMultisigProof(1, 1, types.SideNew)
	msg := types.MsgClaimLegacyAccount{
		NewAddress:    newAddr,
		LegacyAddress: legacyAddr,
		LegacyProof:   legMulti,
		NewProof:      newMulti1of1,
	}
	err := msg.ValidateBasic()
	require.ErrorIs(t, err, types.ErrMirrorSourceMismatch)
}

// TestMsgMigrateValidator_ValidateBasic_MirrorSource mirrors the above for the
// validator migration message.
func TestMsgMigrateValidator_ValidateBasic_MirrorSource(t *testing.T) {
	legacyKey := secp256k1.GenPrivKey()
	legacyPub := legacyKey.PubKey().(*secp256k1.PubKey)
	legacyAddr := sdk.AccAddress(legacyPub.Address()).String()
	newAddr := validAddr()

	legSingle := validSingleProof(legacyPub)
	newMulti := validMultisigProof(2, 3, types.SideNew)
	msg := types.MsgMigrateValidator{
		NewAddress:    newAddr,
		LegacyAddress: legacyAddr,
		LegacyProof:   legSingle,
		NewProof:      newMulti,
	}
	err := msg.ValidateBasic()
	require.ErrorIs(t, err, types.ErrMirrorSourceMismatch)
}

func TestParams_MaxMultisigSubKeys(t *testing.T) {
	p := types.DefaultParams()
	require.Equal(t, uint32(20), p.MaxMultisigSubKeys)
	require.NoError(t, p.Validate())

	p.MaxMultisigSubKeys = 0
	require.ErrorContains(t, p.Validate(), "max_multisig_sub_keys must be positive")
}
