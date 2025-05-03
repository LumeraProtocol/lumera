package types

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestMsgStartSupernode_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgStartSupernode
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgStartSupernode{
				Creator: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: MsgStartSupernode{
				Creator: cryptotestutils.AccAddress(),
			},
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
