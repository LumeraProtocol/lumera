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

	// Keep assignment stable within the epoch by using the params snapshot captured at epoch start
	// (when available). Fallback to current params for backward compatibility.
	assignParams := params
	if snap, ok := q.k.GetEpochParamsSnapshot(sdkCtx, epochID); ok {
		assignParams = snap.WithDefaults()
	}

	logicalAccount, err := q.k.AccountForEpoch(sdkCtx, req.SupernodeAccount, epochID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	eligibleChallengers, err := q.k.storageTruthEligibleChallengers(sdkCtx, anchor.ActiveSupernodeAccounts, epochID, assignParams)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	targets, _, err := computeAuditPeerTargetsForReporter(&assignParams, eligibleChallengers, anchor.TargetSupernodeAccounts, anchor.Seed, logicalAccount)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	targetMappings, err := q.k.ResolveAccountIdentityMappings(sdkCtx, targets)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryAssignedTargetsResponse{
		EpochId:                  epochID,
		EpochStartHeight:         epochStart,
		RequiredOpenPorts:        append([]uint32(nil), assignParams.RequiredOpenPorts...),
		TargetSupernodeAccounts:  targets,
		ReporterSupernodeAccount: logicalAccount,
		TargetAccountMappings:    targetMappings,
	}, nil
}

// ResolveAccountIdentityMappings resolves each logical account independently
// through the indexed lineage. It intentionally preserves input order and
// fails the whole operation if any lineage cannot be trusted.
func (k Keeper) ResolveAccountIdentityMappings(ctx sdk.Context, logicalAccounts []string) ([]types.AccountIdentityMapping, error) {
	mappings := make([]types.AccountIdentityMapping, len(logicalAccounts))
	for i, logicalAccount := range logicalAccounts {
		currentAccount, err := k.CurrentAccount(ctx, logicalAccount)
		if err != nil {
			return nil, err
		}
		mappings[i] = types.AccountIdentityMapping{
			LogicalAccount: logicalAccount,
			CurrentAccount: currentAccount,
		}
	}
	return mappings, nil
}
