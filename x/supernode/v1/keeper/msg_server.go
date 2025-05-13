package keeper

import (
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type msgServer struct {
	types.UnimplementedMsgServer

	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) *msgServer {
	return &msgServer{
		UnimplementedMsgServer: types.UnimplementedMsgServer{},
		Keeper: keeper,
	}
}

var _ types.MsgServer = msgServer{}
