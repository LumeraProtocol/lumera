package keeper

import (
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type msgServer struct {
	types.UnimplementedMsgServer

	types.SupernodeKeeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper types.SupernodeKeeper) *msgServer {
	return &msgServer{
		UnimplementedMsgServer: types.UnimplementedMsgServer{},
		SupernodeKeeper: keeper,
	}
}

var _ types.MsgServer = msgServer{}
