package types

import (
	proto "github.com/cosmos/gogoproto/proto"

	"github.com/LumeraProtocol/lumera/internal/legacyalias"
)

var legacySupernodeAliases = []legacyalias.Alias{
	{
		Legacy:    "lumera.supernode.MsgRegisterSupernode",
		Canonical: "lumera.supernode.v1.MsgRegisterSupernode",
		Factory:   func() proto.Message { return &MsgRegisterSupernode{} },
	},
	{
		Legacy:    "lumera.supernode.MsgDeregisterSupernode",
		Canonical: "lumera.supernode.v1.MsgDeregisterSupernode",
		Factory:   func() proto.Message { return &MsgDeregisterSupernode{} },
	},
	{
		Legacy:    "lumera.supernode.MsgStartSupernode",
		Canonical: "lumera.supernode.v1.MsgStartSupernode",
		Factory:   func() proto.Message { return &MsgStartSupernode{} },
	},
	{
		Legacy:    "lumera.supernode.MsgStopSupernode",
		Canonical: "lumera.supernode.v1.MsgStopSupernode",
		Factory:   func() proto.Message { return &MsgStopSupernode{} },
	},
	{
		Legacy:    "lumera.supernode.MsgUpdateSupernode",
		Canonical: "lumera.supernode.v1.MsgUpdateSupernode",
		Factory:   func() proto.Message { return &MsgUpdateSupernode{} },
	},
	{
		Legacy:    "lumera.supernode.MsgUpdateParams",
		Canonical: "lumera.supernode.v1.MsgUpdateParams",
		Factory:   func() proto.Message { return &MsgUpdateParams{} },
	},
}

func init() {
	for _, alias := range legacySupernodeAliases {
		legacyalias.Register(alias)
	}
}
