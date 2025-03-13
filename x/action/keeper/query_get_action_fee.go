package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/action/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k *Keeper) GetActionFee(goCtx context.Context, req *types.QueryGetActionFeeRequest) (*types.QueryGetActionFeeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO: Process the query
	_ = ctx
	// FOR NOW: Return the data size as the fee
	return &types.QueryGetActionFeeResponse{Amount: req.DataSize}, nil
}
