package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgCreatePastelId{}

func NewMsgCreatePastelId(creator string, idType string, pastelId string, pqKey string, signature string, timeStamp string, version uint64) *MsgCreatePastelId {
	return &MsgCreatePastelId{
		Creator:   creator,
		IdType:    idType,
		PastelId:  pastelId,
		PqKey:     pqKey,
		Signature: signature,
		TimeStamp: timeStamp,
		Version:   version,
	}
}

func (msg *MsgCreatePastelId) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	return nil
}
