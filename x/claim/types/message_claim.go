package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgClaim{}

func NewMsgClaim(creator string, oldAddress string, newAddress string, pubKey string, signature string) *MsgClaim {
	return &MsgClaim{

		OldAddress: oldAddress,
		NewAddress: newAddress,
		PubKey:     pubKey,
		Signature:  signature,
	}
}
