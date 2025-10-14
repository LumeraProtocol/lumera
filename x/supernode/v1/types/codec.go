package types

import (
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	// this line is used by starport scaffolding # 1
)

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
	// this line is used by starport scaffolding # 3

	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
	)
	msgservice.RegisterMsgServiceDesc(registry, &Msg_serviceDesc)
}
