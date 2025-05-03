package keeper

import (
	"context"
	"strconv"

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

	dataSize, err := strconv.ParseInt(req.DataSize, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid data_size: %v", err)
	}

	params := k.GetParams(ctx)

	// Calculate: FeePerByte * DataSize + BaseActionFee
	perByteCost := params.FeePerByte.Amount.MulRaw(dataSize)
	totalAmount := perByteCost.Add(params.BaseActionFee.Amount)

	return &types.QueryGetActionFeeResponse{Amount: totalAmount.String()}, nil
}
