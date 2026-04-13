package keeper

import (
	"context"
	"errors"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	params := genState.Params.WithDefaults()
	if err := params.Validate(); err != nil {
		return err
	}

	// Genesis is the initial source of truth for module params. After genesis, params can
	// only be updated via governance (MsgUpdateParams).
	if err := k.SetParams(ctx, params); err != nil {
		return err
	}

	sdkCtx, ok := ctx.(sdk.Context)
	if !ok {
		sdkCtx = sdk.UnwrapSDKContext(ctx)
	}

	var nextEvidenceID uint64
	if genState.NextEvidenceId != 0 {
		nextEvidenceID = genState.NextEvidenceId
	}

	for _, ev := range genState.Evidence {
		if err := k.SetEvidence(sdkCtx, ev); err != nil {
			return err
		}
		k.SetEvidenceBySubjectIndex(sdkCtx, ev.SubjectAddress, ev.EvidenceId)
		if ev.ActionId != "" {
			k.SetEvidenceByActionIndex(sdkCtx, ev.ActionId, ev.EvidenceId)
		}
		if ev.EvidenceId >= nextEvidenceID {
			nextEvidenceID = ev.EvidenceId + 1
		}
	}

	if nextEvidenceID == 0 {
		nextEvidenceID = 1
	}
	k.SetNextEvidenceID(sdkCtx, nextEvidenceID)

	nextHealOpID := uint64(1)
	if genState.NextHealOpId != 0 {
		nextHealOpID = genState.NextHealOpId
	}

	for _, state := range genState.NodeSuspicionStates {
		if err := k.SetNodeSuspicionState(sdkCtx, state); err != nil {
			return err
		}
	}
	for _, state := range genState.ReporterReliabilityStates {
		if err := k.SetReporterReliabilityState(sdkCtx, state); err != nil {
			return err
		}
	}
	for _, state := range genState.TicketDeteriorationStates {
		if err := k.SetTicketDeteriorationState(sdkCtx, state); err != nil {
			return err
		}
	}
	for _, healOp := range genState.HealOps {
		if err := k.SetHealOp(sdkCtx, healOp); err != nil {
			return err
		}
		if healOp.HealOpId >= nextHealOpID {
			nextHealOpID = healOp.HealOpId + 1
		}
	}
	k.SetNextHealOpID(sdkCtx, nextHealOpID)

	return nil
}

// ExportGenesis returns the module's exported genesis.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	sdkCtx, ok := ctx.(sdk.Context)
	if !ok {
		sdkCtx = sdk.UnwrapSDKContext(ctx)
	}

	evidence, err := k.GetAllEvidence(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.Evidence = evidence
	genesis.NextEvidenceId = k.GetNextEvidenceID(sdkCtx)

	if genesis.NextEvidenceId == 0 {
		return nil, errors.New("invalid next evidence id")
	}

	nodeSuspicionStates, err := k.GetAllNodeSuspicionStates(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.NodeSuspicionStates = nodeSuspicionStates

	reporterReliabilityStates, err := k.GetAllReporterReliabilityStates(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.ReporterReliabilityStates = reporterReliabilityStates

	ticketDeteriorationStates, err := k.GetAllTicketDeteriorationStates(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.TicketDeteriorationStates = ticketDeteriorationStates

	healOps, err := k.GetAllHealOps(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.HealOps = healOps
	genesis.NextHealOpId = k.GetNextHealOpID(sdkCtx)
	if genesis.NextHealOpId == 0 {
		return nil, errors.New("invalid next heal op id")
	}

	return genesis, nil
}
