package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func (k msgServer) RegisterSupernode(goCtx context.Context, msg *types.MsgRegisterSupernode) (*types.MsgRegisterSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO: Handling the message
	_ = ctx

	return &types.MsgRegisterSupernodeResponse{}, nil
}
