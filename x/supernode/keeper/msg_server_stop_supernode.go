package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

func (k msgServer) StopSupernode(goCtx context.Context, msg *types.MsgStopSupernode) (*types.MsgStopSupernodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO: Handling the message
	_ = ctx

	return &types.MsgStopSupernodeResponse{}, nil
}
