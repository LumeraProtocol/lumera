package types

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/sample"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestMsgDelayedClaim_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDelayedClaim
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgDelayedClaim{
				Creator: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: MsgDelayedClaim{
				Creator: sample.AccAddress(),
				Tier:    1,
			},
		}, {
			name: "invalid tier",
			msg: MsgDelayedClaim{
				Creator: sample.AccAddress(),
				Tier:    7,
			},
			err: sdkerrors.ErrInvalidRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}
