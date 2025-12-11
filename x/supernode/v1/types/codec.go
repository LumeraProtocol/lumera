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
		&MsgRegisterSupernode{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgDeregisterSupernode{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgStartSupernode{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgStopSupernode{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateSupernode{},
	)
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgReportSupernodeMetrics{},
	)
	// this line is used by starport scaffolding # 3

	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
	)
	registerLegacyTypeURLAliases(registry)
	msgservice.RegisterMsgServiceDesc(registry, &Msg_serviceDesc)
}

// registerLegacyTypeURLAliases wires the pre-versioned Msg type URLs into the
// interface registry so legacy governance items continue to decode.
func registerLegacyTypeURLAliases(registry cdctypes.InterfaceRegistry) {
	registrar, ok := registry.(customTypeURLRegistry)
	if !ok {
		panic(fmt.Sprintf("supernode module: interface registry %T does not support RegisterCustomTypeURL; skipping legacy type URL aliases\n", registry))
	}

	for _, alias := range legacySupernodeAliases {
		registrar.RegisterCustomTypeURL((*sdk.Msg)(nil), "/"+alias.Legacy, alias.Factory())
	}
}
