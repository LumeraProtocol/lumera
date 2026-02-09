package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (k Keeper) HasReport(ctx sdk.Context, epochID uint64, reporterSupernodeAccount string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.ReportKey(epochID, reporterSupernodeAccount))
}

func (k Keeper) GetReport(ctx sdk.Context, epochID uint64, reporterSupernodeAccount string) (types.EpochReport, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.ReportKey(epochID, reporterSupernodeAccount))
	if bz == nil {
		return types.EpochReport{}, false
	}
	var r types.EpochReport
	k.cdc.MustUnmarshal(bz, &r)
	return r, true
}

func (k Keeper) SetReport(ctx sdk.Context, r types.EpochReport) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&r)
	if err != nil {
		return err
	}
	store.Set(types.ReportKey(r.EpochId, r.SupernodeAccount), bz)
	return nil
}

func (k Keeper) SetReportIndex(ctx sdk.Context, epochID uint64, reporterSupernodeAccount string) {
	store := k.kvStore(ctx)
	store.Set(types.ReportIndexKey(reporterSupernodeAccount, epochID), []byte{1})
}

func (k Keeper) SetStorageChallengeReportIndex(ctx sdk.Context, supernodeAccount string, epochID uint64, reporterSupernodeAccount string) {
	store := k.kvStore(ctx)
	store.Set(types.StorageChallengeReportIndexKey(supernodeAccount, epochID, reporterSupernodeAccount), []byte{1})
}

func (k Keeper) SetHostReportIndex(ctx sdk.Context, epochID uint64, reporterSupernodeAccount string) {
	store := k.kvStore(ctx)
	store.Set(types.HostReportIndexKey(reporterSupernodeAccount, epochID), []byte{1})
}
