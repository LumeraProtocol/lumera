package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func (k msgServer) DeregisterSupernode(goCtx context.Context, msg *types.MsgDeregisterSupernode) (*types.MsgDeregisterSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO: Handling the message
	_ = ctx

	return &types.MsgDeregisterSupernodeResponse{}, nil
}
