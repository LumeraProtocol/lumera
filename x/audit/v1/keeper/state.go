package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
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
	case supernodetypes.SuperNodeStateDisabled, supernodetypes.SuperNodeStateStopped, supernodetypes.SuperNodeStatePenalized, supernodetypes.SuperNodeStatePostponed:
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

// GetAllReportsForGenesis exports all r/ epoch reports.
// Per NEW-C-1.
func (k Keeper) GetAllReportsForGenesis(ctx sdk.Context) ([]types.EpochReport, error) {
	prefix := types.ReportPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	out := make([]types.EpochReport, 0)
	for ; it.Valid(); it.Next() {
		var r types.EpochReport
		if err := k.cdc.Unmarshal(it.Value(), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// SetReportRaw writes an epoch report to its primary key without triggering
// supernode-state transitions. Used by InitGenesis to restore exported reports.
func (k Keeper) SetReportRaw(ctx sdk.Context, r types.EpochReport) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&r)
	if err != nil {
		return err
	}
	store.Set(types.ReportKey(r.EpochId, r.SupernodeAccount), bz)
	return nil
}

// GetAllReportIndicesForGenesis exports all ri/ index entries.
func (k Keeper) GetAllReportIndicesForGenesis(ctx sdk.Context) []types.GenesisReportIndex {
	prefix := types.ReportIndexRootPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	out := make([]types.GenesisReportIndex, 0)
	for ; it.Valid(); it.Next() {
		// "ri/" + reporter + "/" + u64be(epoch_id)
		key := it.Key()
		body := key[len(prefix):]
		if len(body) < 1+8 {
			continue
		}
		// Find the last "/" before 8-byte epoch suffix.
		// Reporter cannot contain '/' (bech32). Search forward for '/' followed by 8 bytes ending key.
		split := len(body) - 8 - 1
		if split < 0 || body[split] != '/' {
			continue
		}
		reporter := string(body[:split])
		epochID := binary.BigEndian.Uint64(body[split+1:])
		out = append(out, types.GenesisReportIndex{
			ReporterSupernodeAccount: reporter,
			EpochId:                  epochID,
		})
	}
	return out
}

// GetAllHostReportIndicesForGenesis exports all hr/ index entries.
func (k Keeper) GetAllHostReportIndicesForGenesis(ctx sdk.Context) []types.GenesisHostReportIndex {
	prefix := types.HostReportIndexRootPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	out := make([]types.GenesisHostReportIndex, 0)
	for ; it.Valid(); it.Next() {
		key := it.Key()
		body := key[len(prefix):]
		split := len(body) - 8 - 1
		if split < 0 || body[split] != '/' {
			continue
		}
		reporter := string(body[:split])
		epochID := binary.BigEndian.Uint64(body[split+1:])
		out = append(out, types.GenesisHostReportIndex{
			ReporterSupernodeAccount: reporter,
			EpochId:                  epochID,
		})
	}
	return out
}

// GetAllStorageChallengeIndicesForGenesis exports all sc/ index entries.
func (k Keeper) GetAllStorageChallengeIndicesForGenesis(ctx sdk.Context) []types.GenesisStorageChallengeIndex {
	prefix := types.StorageChallengeReportIndexRootPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	out := make([]types.GenesisStorageChallengeIndex, 0)
	for ; it.Valid(); it.Next() {
		// "sc/" + supernode + "/" + u64be(epoch_id) + "/" + reporter
		key := it.Key()
		body := key[len(prefix):]
		// Find first '/'
		slash1 := -1
		for i := 0; i < len(body); i++ {
			if body[i] == '/' {
				slash1 = i
				break
			}
		}
		if slash1 < 0 || len(body) < slash1+1+8+1 {
			continue
		}
		supernode := string(body[:slash1])
		epochID := binary.BigEndian.Uint64(body[slash1+1 : slash1+1+8])
		// next byte is '/'
		reporter := string(body[slash1+1+8+1:])
		out = append(out, types.GenesisStorageChallengeIndex{
			SupernodeAccount:         supernode,
			EpochId:                  epochID,
			ReporterSupernodeAccount: reporter,
		})
	}
	return out
}
