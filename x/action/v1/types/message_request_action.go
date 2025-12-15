package types

import (
	"github.com/LumeraProtocol/lumera/x/action/v1/common"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgRequestAction{}

func NewMsgRequestAction(creator string, actionType string, metadata string, price string, expirationTime string, fileSizeKbs string) *MsgRequestAction {
	return &MsgRequestAction{
		Creator:        creator,
		ActionType:     actionType,
		Metadata:       metadata,
		Price:          price,
		ExpirationTime: expirationTime,
		FileSizeKbs:    fileSizeKbs,
	}
}

func (msg *MsgRequestAction) Type() string {
	return "RequestAction"
}

func (msg *MsgRequestAction) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgRequestAction) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	// Check if action type is valid - case insensitive
	_, err = ParseActionType(msg.ActionType)
	if err != nil {
		return errorsmod.Wrap(ErrInvalidActionType, err.Error())
	}

	// Check if price is valid
	if msg.Price == "" {
		return errorsmod.Wrap(ErrInvalidPrice, "price cannot be empty")
	}

	_, err = sdk.ParseCoinNormalized(msg.Price)
	if err != nil {
		return errorsmod.Wrapf(ErrInvalidPrice, "invalid price format, %s", err)
	}

	// Check if expiration_time is valid
	if msg.ExpirationTime == "" {
		return errorsmod.Wrap(ErrInvalidExpiration, "expiration_time cannot be empty")
	}

	expirationTime, err := strconv.ParseInt(msg.ExpirationTime, 10, 64)
	if err != nil {
		return errorsmod.Wrapf(ErrInvalidExpiration, "invalid expiration_time format, %s", err)
	}

	if expirationTime < 0 {
		return errorsmod.Wrap(ErrInvalidExpiration, "expiration_time must be positive")
	}

	// Basic metadata validation (non-empty)
	if msg.Metadata == "" {
		return errorsmod.Wrap(ErrInvalidMetadata, "metadata cannot be empty")
	}

	if err = DoActionValidation(msg.Metadata, msg.ActionType, common.MsgRequestAction); err != nil {
		return errorsmod.Wrapf(ErrInvalidMetadata, "metadata validation failed, %s", err)
	}

	// Validate fileSizeKbs: allow empty, otherwise must be int64 >= 0
	if msg.FileSizeKbs != "" {
		fileSizeKbs, err := strconv.ParseInt(msg.FileSizeKbs, 10, 64)
		if err != nil {
			return errorsmod.Wrapf(ErrInvalidFileSize, "invalid fileSizeKbs format, %s", err)
		}
		if fileSizeKbs < 0 {
			return errorsmod.Wrap(ErrInvalidFileSize, "fileSizeKbs must be >= 0")
		}
	}

	return nil
}
