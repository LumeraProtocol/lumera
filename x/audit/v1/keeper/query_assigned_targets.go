package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (q queryServer) AssignedTargets(ctx context.Context, req *types.QueryAssignedTargetsRequest) (*types.QueryAssignedTargetsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.SupernodeAccount == "" {
		return nil, status.Error(codes.InvalidArgument, "supernode_account is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate prober is a registered supernode.
	_, found, err := q.k.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.SupernodeAccount)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return nil, status.Error(codes.NotFound, "supernode not found")
	}

	params := q.k.GetParams(ctx).WithDefaults()

	epochID := req.EpochId
	var epochStart int64
	if !req.FilterByEpochId {
		epoch, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		epochID = epoch.EpochID
		epochStart = epoch.StartHeight
	} else {
		epoch, err := deriveEpochByID(epochID, params)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		epochStart = epoch.StartHeight
	}

	anchor, found := q.k.GetEpochAnchor(sdkCtx, epochID)
	if !found {
		return nil, status.Error(codes.NotFound, "epoch anchor not found")
	}

	targets, _, err := computeAuditPeerTargetsForReporter(&params, anchor.ActiveSupernodeAccounts, anchor.TargetSupernodeAccounts, anchor.Seed, req.SupernodeAccount)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryAssignedTargetsResponse{
		EpochId:                 epochID,
		EpochStartHeight:        epochStart,
		RequiredOpenPorts:       append([]uint32(nil), params.RequiredOpenPorts...),
		TargetSupernodeAccounts: targets,
	}, nil
}
