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

func TestParams_MaxMultisigSubKeys(t *testing.T) {
	p := types.DefaultParams()
	require.Equal(t, uint32(20), p.MaxMultisigSubKeys)
	require.NoError(t, p.Validate())

	p.MaxMultisigSubKeys = 0
	require.ErrorContains(t, p.Validate(), "max_multisig_sub_keys must be positive")
}
