package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (k Keeper) HasReport(ctx sdk.Context, windowID uint64, reporterSupernodeAccount string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.ReportKey(windowID, reporterSupernodeAccount))
}

func (k Keeper) GetReport(ctx sdk.Context, windowID uint64, reporterSupernodeAccount string) (types.AuditReport, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.ReportKey(windowID, reporterSupernodeAccount))
	if bz == nil {
		return types.AuditReport{}, false
	}
	var r types.AuditReport
	k.cdc.MustUnmarshal(bz, &r)
	return r, true
}

func (k Keeper) SetReport(ctx sdk.Context, r types.AuditReport) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&r)
	if err != nil {
		return err
	}
	store.Set(types.ReportKey(r.WindowId, r.SupernodeAccount), bz)
	return nil
}

func (k Keeper) SetReportIndex(ctx sdk.Context, windowID uint64, reporterSupernodeAccount string) {
	store := k.kvStore(ctx)
	store.Set(types.ReportIndexKey(reporterSupernodeAccount, windowID), []byte{1})
}

func (k Keeper) SetSupernodeReportIndex(ctx sdk.Context, supernodeAccount string, windowID uint64, reporterSupernodeAccount string) {
	store := k.kvStore(ctx)
	store.Set(types.SupernodeReportIndexKey(supernodeAccount, windowID, reporterSupernodeAccount), []byte{1})
}

func (k Keeper) SetSelfReportIndex(ctx sdk.Context, windowID uint64, reporterSupernodeAccount string) {
	store := k.kvStore(ctx)
	store.Set(types.SelfReportIndexKey(reporterSupernodeAccount, windowID), []byte{1})
}
