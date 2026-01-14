package keeper

import "github.com/LumeraProtocol/lumera/x/audit/v1/types"

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{
		UnimplementedMsgServer: types.UnimplementedMsgServer{},
		Keeper:                 keeper,
	}
}

var _ types.MsgServer = msgServer{}

