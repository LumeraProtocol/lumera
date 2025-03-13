package types

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/sample"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestMsgApproveAction_ValidateBasic(t *testing.T) {
	validAddress := sample.AccAddress()
	validActionID := "action-123"

	tests := []struct {
		name string
		msg  MsgApproveAction
		err  error
	}{
		// Valid test case
		{
			name: "valid approve action message",
			msg: MsgApproveAction{
				Creator:  validAddress,
				ActionId: validActionID,
			},
			err: nil,
		},

		// Test cases for creator address validation
		{
			name: "invalid creator address",
			msg: MsgApproveAction{
				Creator:  "invalid_address",
				ActionId: validActionID,
			},
			err: sdkerrors.ErrInvalidAddress,
		},

		// Test cases for action ID validation
		{
			name: "empty action ID",
			msg: MsgApproveAction{
				Creator:  validAddress,
				ActionId: "",
			},
			err: ErrInvalidID,
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
