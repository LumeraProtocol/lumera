package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// GetEpochParamsSnapshot returns the per-epoch params snapshot for the given epoch ID.
// Snapshots are created at epoch start alongside the EpochAnchor and are intended to keep
// assignment/gating stable within an epoch even if governance changes params mid-epoch.
func (k Keeper) GetEpochParamsSnapshot(ctx sdk.Context, epochID uint64) (types.Params, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.EpochParamsSnapshotKey(epochID))
	if bz == nil {
		return types.Params{}, false
	}
	var p types.Params
	k.cdc.MustUnmarshal(bz, &p)
	return p, true
}

func (k Keeper) SetEpochParamsSnapshot(ctx sdk.Context, epochID uint64, params types.Params) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&params)
	if err != nil {
		return err
	}
	store.Set(types.EpochParamsSnapshotKey(epochID), bz)
	return nil
}

func (k Keeper) CreateEpochParamsSnapshotIfNeeded(ctx sdk.Context, epochID uint64, params types.Params) error {
	store := k.kvStore(ctx)
	key := types.EpochParamsSnapshotKey(epochID)
	if bz := store.Get(key); bz != nil {
		return nil
	}
	return k.SetEpochParamsSnapshot(ctx, epochID, params.WithDefaults())
}

