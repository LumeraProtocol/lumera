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
		&MsgRegisterSupernode{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.supernode.MsgRegisterSupernode", &MsgRegisterSupernode{})
	}
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgDeregisterSupernode{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.supernode.MsgDeregisterSupernode", &MsgDeregisterSupernode{})
	}
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgStartSupernode{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.supernode.MsgStartSupernode", &MsgStartSupernode{})
	}
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgStopSupernode{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.supernode.MsgStopSupernode", &MsgStopSupernode{})
	}
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateSupernode{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.supernode.MsgUpdateSupernode", &MsgUpdateSupernode{})
	}
	// this line is used by starport scaffolding # 3

	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
	)
	if registrar, ok := registry.(customTypeURLRegistry); ok {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/lumera.supernode.MsgUpdateParams", &MsgUpdateParams{})
	}
	msgservice.RegisterMsgServiceDesc(registry, &Msg_serviceDesc)
}
