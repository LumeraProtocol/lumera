package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (q queryServer) PayoutHistory(goCtx context.Context, req *types.QueryPayoutHistoryRequest) (*types.QueryPayoutHistoryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.ValidatorAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "validator_address is required")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	keeperImpl, ok := q.k.(Keeper)
	if !ok {
		return nil, status.Error(codes.Internal, "unexpected keeper implementation")
	}
	store := prefix.NewStore(runtime.KVStoreAdapter(keeperImpl.storeService.OpenKVStore(ctx)), types.PayoutHistoryPrefixForValidator(req.ValidatorAddress))

	entries := make([]types.PayoutHistoryEntry, 0)
	pageRes, err := query.Paginate(store, req.Pagination, func(_, value []byte) error {
		var row types.PayoutHistoryEntry
		if err := keeperImpl.cdc.Unmarshal(value, &row); err != nil {
			return err
		}
		entries = append(entries, row)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryPayoutHistoryResponse{Entries: entries, Pagination: pageRes}, nil
}
