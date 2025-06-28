package keeper

import (
	"context"
	"cosmossdk.io/store/prefix"
	"github.com/LumeraProtocol/lumera/x/claim/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k Keeper) ListClaimed(goCtx context.Context, req *types.QueryListClaimedRequest) (*types.QueryListClaimedResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Ensure we have a pagination request with a high limit to process all records
	if req.Pagination == nil {
		req.Pagination = &query.PageRequest{
			Limit: 20_000, // 20K can be hardcoded here as the mainnet claims size is ~17K
		}
	} else if req.Pagination.Limit == 0 {
		req.Pagination.Limit = 20_000
	}

	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, []byte(types.ClaimRecordKey))

	var claims []*types.ClaimRecord
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		var claimRecord types.ClaimRecord
		if err := k.cdc.Unmarshal(value, &claimRecord); err != nil {
			return err
		}

		if claimRecord.Claimed && claimRecord.VestedTier == req.VestedTerm {
			claims = append(claims, &claimRecord)
		}
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryListClaimedResponse{
		Claims:     claims,
		Pagination: pageRes,
	}, nil
}
