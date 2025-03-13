package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgApproveAction{}

func NewMsgApproveAction(creator string, actionId string, signature string) *MsgApproveAction {
	return &MsgApproveAction{
		Creator:  creator,
		ActionId: actionId,
	}
}

func (msg *MsgApproveAction) ValidateBasic() error {
	// Validate creator address
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	// Validate action ID
	if msg.ActionId == "" {
		return errorsmod.Wrap(ErrInvalidID, "action ID cannot be empty")
	}

	return nil
}
