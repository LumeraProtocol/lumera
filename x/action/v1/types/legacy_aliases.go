package types

import (
	proto "github.com/cosmos/gogoproto/proto"

	"github.com/LumeraProtocol/lumera/internal/legacyalias"
)

var legacyActionAliases = []legacyalias.Alias{
	{
		Legacy:    "lumera.action.MsgRequestAction",
		Canonical: "lumera.action.v1.MsgRequestAction",
		Factory:   func() proto.Message { return &MsgRequestAction{} },
	},
	{
		Legacy:    "lumera.action.MsgFinalizeAction",
		Canonical: "lumera.action.v1.MsgFinalizeAction",
		Factory:   func() proto.Message { return &MsgFinalizeAction{} },
	},
	{
		Legacy:    "lumera.action.MsgApproveAction",
		Canonical: "lumera.action.v1.MsgApproveAction",
		Factory:   func() proto.Message { return &MsgApproveAction{} },
	},
	{
		Legacy:    "lumera.action.MsgUpdateParams",
		Canonical: "lumera.action.v1.MsgUpdateParams",
		Factory:   func() proto.Message { return &MsgUpdateParams{} },
	},
}

func init() {
	for _, alias := range legacyActionAliases {
		legacyalias.Register(alias)
	}
}
