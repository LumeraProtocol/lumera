package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (k Keeper) PastelidEntryAll(ctx context.Context, req *types.QueryAllPastelidEntryRequest) (*types.QueryAllPastelidEntryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	var pastelidEntrys []types.PastelidEntry

	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	pastelidEntryStore := prefix.NewStore(store, types.KeyPrefix(types.PastelidEntryKeyPrefix))

	pageRes, err := query.Paginate(pastelidEntryStore, req.Pagination, func(key []byte, value []byte) error {
		var pastelidEntry types.PastelidEntry
		if err := k.cdc.Unmarshal(value, &pastelidEntry); err != nil {
			return err
		}

		pastelidEntrys = append(pastelidEntrys, pastelidEntry)
		return nil
	})

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryAllPastelidEntryResponse{PastelidEntry: pastelidEntrys, Pagination: pageRes}, nil
}

func (k Keeper) PastelidEntry(ctx context.Context, req *types.QueryGetPastelidEntryRequest) (*types.QueryGetPastelidEntryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	val, found := k.GetPastelidEntry(
		ctx,
		req.Address,
	)
	if !found {
		return nil, status.Error(codes.NotFound, "not found")
	}

	return &types.QueryGetPastelidEntryResponse{PastelidEntry: val}, nil
}
