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
				Params:    types.NewParams(true, 0, 0, 100),
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

	tests := []struct {
		name    string
		msg     types.MsgClaimLegacyAccount
		wantErr error
	}{
		{
			name: "valid",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:      newAddr,
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
				NewSignature:    []byte("new-sig"),
			},
		},
		{
			name: "invalid new_address",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:      "bad",
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
				NewSignature:    []byte("new-sig"),
			},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid legacy_address",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:      newAddr,
				LegacyAddress:   "bad",
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
				NewSignature:    []byte("new-sig"),
			},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "same address",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:      legacyAddr,
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
				NewSignature:    []byte("new-sig"),
			},
			wantErr: types.ErrSameAddress,
		},
		{
			name: "invalid pubkey size",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:      newAddr,
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    []byte{0x01, 0x02},
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
				NewSignature:    []byte("new-sig"),
			},
			wantErr: types.ErrInvalidLegacyPubKey,
		},
		{
			name: "empty signature",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:    newAddr,
				LegacyAddress: legacyAddr,
				LegacyPubKey:  legacyPub.Key,
				NewPubKey:     make([]byte, 33),
				NewSignature:  []byte("new-sig"),
			},
			wantErr: types.ErrInvalidLegacySignature,
		},
		{
			name: "invalid new pubkey size",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:      newAddr,
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       []byte{0x01},
				NewSignature:    []byte("new-sig"),
			},
			wantErr: types.ErrInvalidNewPubKey,
		},
		{
			name: "empty new signature",
			msg: types.MsgClaimLegacyAccount{
				NewAddress:      newAddr,
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
			},
			wantErr: types.ErrInvalidNewSignature,
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

	tests := []struct {
		name    string
		msg     types.MsgMigrateValidator
		wantErr error
	}{
		{
			name: "valid",
			msg: types.MsgMigrateValidator{
				NewAddress:      newAddr,
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
				NewSignature:    []byte("new-sig"),
			},
		},
		{
			name: "invalid new_address",
			msg: types.MsgMigrateValidator{
				NewAddress:      "bad",
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
				NewSignature:    []byte("new-sig"),
			},
			wantErr: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "same address",
			msg: types.MsgMigrateValidator{
				NewAddress:      legacyAddr,
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
				NewSignature:    []byte("new-sig"),
			},
			wantErr: types.ErrSameAddress,
		},
		{
			name: "missing new signature",
			msg: types.MsgMigrateValidator{
				NewAddress:      newAddr,
				LegacyAddress:   legacyAddr,
				LegacyPubKey:    legacyPub.Key,
				LegacySignature: []byte("sig"),
				NewPubKey:       make([]byte, 33),
			},
			wantErr: types.ErrInvalidNewSignature,
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
