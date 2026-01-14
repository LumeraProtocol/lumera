package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (q queryServer) AuditStatus(ctx context.Context, req *types.QueryAuditStatusRequest) (*types.QueryAuditStatusResponse, error) {
	if req == nil {
		return &types.QueryAuditStatusResponse{Status: types.AuditStatus{}}, nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	status, found := q.k.GetAuditStatus(sdkCtx, req.ValidatorAddress)
	if !found {
		status = types.AuditStatus{
			ValidatorAddress: req.ValidatorAddress,
			Compliant:        true,
		}
	}

	return &types.QueryAuditStatusResponse{Status: status}, nil
}

