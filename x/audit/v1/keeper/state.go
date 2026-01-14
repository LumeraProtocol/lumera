package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (k Keeper) HasReport(ctx sdk.Context, windowID uint64, reporterValidatorAddress string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.ReportKey(windowID, reporterValidatorAddress))
}

func (k Keeper) GetReport(ctx sdk.Context, windowID uint64, reporterValidatorAddress string) (types.AuditReport, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.ReportKey(windowID, reporterValidatorAddress))
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
	store.Set(types.ReportKey(r.WindowId, r.ReporterValidatorAddress), bz)
	return nil
}

func (k Keeper) GetEvidenceAggregate(ctx sdk.Context, windowID uint64, targetValidatorAddress string, portIndex uint32) (types.PortEvidenceAggregate, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.EvidenceKey(windowID, targetValidatorAddress, portIndex))
	if bz == nil {
		return types.PortEvidenceAggregate{}, false
	}
	var a types.PortEvidenceAggregate
	k.cdc.MustUnmarshal(bz, &a)
	return a, true
}

func (k Keeper) SetEvidenceAggregate(ctx sdk.Context, windowID uint64, targetValidatorAddress string, portIndex uint32, a types.PortEvidenceAggregate) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&a)
	if err != nil {
		return err
	}
	store.Set(types.EvidenceKey(windowID, targetValidatorAddress, portIndex), bz)
	return nil
}

func (k Keeper) GetAuditStatus(ctx sdk.Context, validatorAddress string) (types.AuditStatus, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.AuditStatusKey(validatorAddress))
	if bz == nil {
		return types.AuditStatus{}, false
	}
	var s types.AuditStatus
	k.cdc.MustUnmarshal(bz, &s)
	return s, true
}

func (k Keeper) SetAuditStatus(ctx sdk.Context, s types.AuditStatus) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&s)
	if err != nil {
		return err
	}
	store.Set(types.AuditStatusKey(s.ValidatorAddress), bz)
	return nil
}

func consensusFromAggregate(a types.PortEvidenceAggregate, quorum uint32) types.PortState {
	if a.Count < quorum {
		return types.PortState_PORT_STATE_UNKNOWN
	}
	if a.Conflict {
		return types.PortState_PORT_STATE_UNKNOWN
	}
	return a.FirstState
}

func (k Keeper) setRequiredPortsState(ctx sdk.Context, validatorAddress string, requiredPortsCount int, portIndex uint32, state types.PortState) error {
	status, found := k.GetAuditStatus(ctx, validatorAddress)
	if !found {
		status = types.AuditStatus{
			ValidatorAddress: validatorAddress,
			Compliant:        true,
		}
	}

	if len(status.RequiredPortsState) != requiredPortsCount {
		status.RequiredPortsState = make([]types.PortState, requiredPortsCount)
	}

	if int(portIndex) < len(status.RequiredPortsState) {
		status.RequiredPortsState[portIndex] = state
	}

	return k.SetAuditStatus(ctx, status)
}
