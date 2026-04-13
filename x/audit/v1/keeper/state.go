package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	supernodetypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
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

	// Canonical STORAGE_FULL transition source: audit epoch host reports.
	// If disk usage is omitted/zero in report, skip transition logic.
	if r.HostReport.DiskUsagePercent == 0 {
		ctx.EventManager().EmitEvent(sdk.NewEvent("audit_set_report_transition", sdk.NewAttribute("disk_usage_percent", "0"), sdk.NewAttribute("transition_skipped", "true")))
		return nil
	}
	reporterSN, found, err := k.supernodeKeeper.GetSuperNodeByAccount(ctx, r.SupernodeAccount)
	if err != nil {
		return err
	}
	if !found || len(reporterSN.States) == 0 {
		ctx.EventManager().EmitEvent(sdk.NewEvent("audit_set_report_transition", sdk.NewAttribute("supernode_found", "false")))
		return nil
	}
	latest := reporterSN.States[len(reporterSN.States)-1].State
	maxStorage := float64(k.supernodeKeeper.GetParams(ctx).MaxStorageUsagePercent)
	isStorageFull := r.HostReport.DiskUsagePercent > maxStorage

	switch latest {
	case supernodetypes.SuperNodeStateDisabled, supernodetypes.SuperNodeStateStopped, supernodetypes.SuperNodeStatePenalized:
		return nil
	}

	if isStorageFull && latest != supernodetypes.SuperNodeStateStorageFull {
		reporterSN.States = append(reporterSN.States, &supernodetypes.SuperNodeStateRecord{State: supernodetypes.SuperNodeStateStorageFull, Height: ctx.BlockHeight()})
		ctx.EventManager().EmitEvent(sdk.NewEvent("audit_set_report_transition", sdk.NewAttribute("to_state", "storage_full")))
		return k.supernodeKeeper.SetSuperNode(ctx, reporterSN)
	}
	if !isStorageFull && latest == supernodetypes.SuperNodeStateStorageFull {
		reporterSN.States = append(reporterSN.States, &supernodetypes.SuperNodeStateRecord{State: supernodetypes.SuperNodeStateActive, Height: ctx.BlockHeight()})
		ctx.EventManager().EmitEvent(sdk.NewEvent("audit_set_report_transition", sdk.NewAttribute("to_state", "active")))
		return k.supernodeKeeper.SetSuperNode(ctx, reporterSN)
	}
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
