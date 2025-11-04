package types

import (
	"fmt"

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
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgFinalizeAction{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgApproveAction{},
	)
	// this line is used by starport scaffolding # 3

	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
	)
	registerLegacyTypeURLAliases(registry)
	msgservice.RegisterMsgServiceDesc(registry, &Msg_serviceDesc)
}

// registerLegacyTypeURLAliases wires the pre-versioned Msg type URLs into the
// interface registry so proposals stored with legacy URLs still decode.
func registerLegacyTypeURLAliases(registry cdctypes.InterfaceRegistry) {
	registrar, ok := registry.(customTypeURLRegistry)
	if !ok {
		panic(fmt.Sprintf("action module: interface registry %T does not support RegisterCustomTypeURL; cannot register legacy type URL aliases", registry))
	}

	for _, alias := range legacyActionAliases {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/"+alias.Legacy, alias.Factory())
	}
}
