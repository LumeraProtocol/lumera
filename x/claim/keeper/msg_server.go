package keeper

import (
	"github.com/LumeraProtocol/lumera/x/claim/types"
)

type msgServer struct {
	types.UnimplementedMsgServer


	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{
		UnimplementedMsgServer: types.UnimplementedMsgServer{},
		Keeper: keeper,
	}
}

var _ types.MsgServer = msgServer{}
