package types

import (
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	proto "github.com/cosmos/gogoproto/proto"
	// this line is used by starport scaffolding # 1
)

type customTypeURLRegistry interface {
	RegisterCustomTypeURL(iface interface{}, typeURL string, impl proto.Message)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgRequestAction{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.action.MsgRequestAction", &MsgRequestAction{})
	}
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgFinalizeAction{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.action.MsgFinalizeAction", &MsgFinalizeAction{})
	}
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgApproveAction{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.action.MsgApproveAction", &MsgApproveAction{})
	}
	// this line is used by starport scaffolding # 3

	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.action.MsgUpdateParams", &MsgUpdateParams{})
	}
	msgservice.RegisterMsgServiceDesc(registry, &Msg_serviceDesc)
}
