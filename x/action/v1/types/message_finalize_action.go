package types

import (
	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/action/v1/common"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgFinalizeAction{}

func NewMsgFinalizeAction(creator string, actionId string, actionType string, metadata string) *MsgFinalizeAction {
	return &MsgFinalizeAction{
		Creator:    creator,
		ActionId:   actionId,
		ActionType: actionType,
		Metadata:   metadata,
	}
}

func (msg *MsgFinalizeAction) Type() string {
	return "FinalizeAction"
}

func (msg *MsgFinalizeAction) ValidateBasic() error {
	// Validate finalizer address
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	// Validate action ID
	if msg.ActionId == "" {
		return errorsmod.Wrap(ErrInvalidID, "action ID cannot be empty")
	}

	// Check if action type is valid - case insensitive
	_, err = ParseActionType(msg.ActionType)
	if err != nil {
		return errorsmod.Wrap(ErrInvalidActionType, err.Error())
	}

	// Basic metadata validation (non-empty)
	if msg.Metadata == "" {
		return errorsmod.Wrap(ErrInvalidMetadata, "metadata cannot be empty")
	}

	if err = DoActionValidation(msg.Metadata, msg.ActionType, common.MsgFinalizeAction); err != nil {
		return errorsmod.Wrapf(ErrInvalidMetadata, "metadata validation failed, %s", err)
	}

	return nil
}
