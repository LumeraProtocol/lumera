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

	windowID := req.WindowId
	if !req.FilterByWindowId {
		ws, err := q.k.getCurrentWindowState(sdkCtx, params)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		windowID = ws.WindowID
	}

	snap, found := q.k.GetWindowSnapshot(sdkCtx, windowID)
	if !found {
		return nil, status.Error(codes.NotFound, "window snapshot not found")
	}

	var targets []string
	for _, a := range snap.Assignments {
		if a.ProberSupernodeAccount != req.SupernodeAccount {
			continue
		}
		targets = append([]string(nil), a.TargetSupernodeAccounts...)
		break
	}

	return &types.QueryAssignedTargetsResponse{
		WindowId:                windowID,
		WindowStartHeight:       snap.WindowStartHeight,
		RequiredOpenPorts:       append([]uint32(nil), params.RequiredOpenPorts...),
		TargetSupernodeAccounts: targets,
	}, nil
}
