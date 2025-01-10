package keeper

import (
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) *msgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}
