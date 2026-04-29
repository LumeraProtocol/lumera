package keeper

import (
	"context"
	"errors"
	"fmt"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
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

	// Per 119-F8 / 119-F12 — hard-error on malformed score states at genesis.
	currentEpoch := uint64(0)
	if epochInfo, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params); err == nil {
		currentEpoch = epochInfo.EpochID
	}
	if err := types.ValidateScoreStatesGenesis(genState, currentEpoch); err != nil {
		return err
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
	for _, state := range genState.TicketArtifactCountStates {
		if err := k.SetTicketArtifactCountState(sdkCtx, state); err != nil {
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

	// Per 121-F7 — restore storage-truth postponement markers on chain restart.
	// Per NEW-B-6 / NEW-B-9 — cross-validate against supernode state so genesis
	// cannot encode a phantom postponement (audit marker but supernode not in
	// SuperNodeStatePostponed). Supernode genesis runs before audit
	// (app/app_config.go) so the supernode state is loaded by this point.
	for _, p := range genState.StorageTruthPostponements {
		sn, found, err := k.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, p.SupernodeAccount)
		if err != nil {
			return fmt.Errorf("audit genesis: failed to look up supernode %q for storage-truth postponement: %w", p.SupernodeAccount, err)
		}
		if !found {
			return fmt.Errorf("audit genesis: storage-truth postponement %q references unknown supernode", p.SupernodeAccount)
		}
		if len(sn.States) == 0 || sn.States[len(sn.States)-1].State != sntypes.SuperNodeStatePostponed {
			return fmt.Errorf("audit genesis: storage-truth postponement %q lacks corresponding supernode-postponed state", p.SupernodeAccount)
		}
		k.setStorageTruthPostponedAtEpochID(sdkCtx, p.SupernodeAccount, p.PostponedAtEpochId)
		if p.StrongPostpone {
			k.setStorageTruthStrongPostponeMarker(sdkCtx, p.SupernodeAccount)
		}
	}

	// Per final-gate F-B2/F-B3/F-B4 — restore genesis-covered
	// action-finalization markers, evidence aggregates, and heal-op votes.
	for _, p := range genState.ActionFinalizationPostponements {
		k.setActionFinalizationPostponedAtEpochID(sdkCtx, p.SupernodeAccount, p.PostponedAtEpochId)
	}
	for _, c := range genState.EvidenceEpochCounts {
		k.setEvidenceEpochCount(sdkCtx, c.EpochId, c.SubjectAddress, c.EvidenceType, c.Count)
	}
	for _, v := range genState.HealOpVerifications {
		k.SetHealOpVerification(sdkCtx, v.HealOpId, v.VerifierSupernodeAccount, v.Verified)
	}

	// Per NEW-C-1 — restore epoch-scoped audit prefix families.
	for _, e := range genState.RecheckEvidence {
		k.SetRecheckEvidence(sdkCtx, e.EpochId, e.TicketId, e.CreatorAccount)
	}
	for _, t := range genState.StorageProofTranscripts {
		if err := k.importStorageProofTranscriptForGenesis(sdkCtx, t.TranscriptHash, t.RecordJson); err != nil {
			return err
		}
	}
	for _, f := range genState.NodeFailureFacts {
		k.importNodeFailureFactForGenesis(sdkCtx, f)
	}
	for _, f := range genState.ReporterResultFacts {
		k.importReporterResultFactForGenesis(sdkCtx, f)
	}
	for _, m := range genState.FailedHealMarkers {
		if err := k.setStorageTruthFailedHeal(sdkCtx, m.SupernodeAccount, m.EpochId, m.TicketId); err != nil {
			return err
		}
	}
	for _, r := range genState.EpochReports {
		if err := k.SetReportRaw(sdkCtx, r); err != nil {
			return err
		}
	}
	for _, idx := range genState.ReportIndices {
		k.SetReportIndex(sdkCtx, idx.EpochId, idx.ReporterSupernodeAccount)
	}
	for _, idx := range genState.HostReportIndices {
		k.SetHostReportIndex(sdkCtx, idx.EpochId, idx.ReporterSupernodeAccount)
	}
	for _, idx := range genState.StorageChallengeIndices {
		k.SetStorageChallengeReportIndex(sdkCtx, idx.SupernodeAccount, idx.EpochId, idx.ReporterSupernodeAccount)
	}

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

	ticketArtifactCountStates, err := k.GetAllTicketArtifactCountStates(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.TicketArtifactCountStates = ticketArtifactCountStates

	healOps, err := k.GetAllHealOps(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.HealOps = healOps
	genesis.NextHealOpId = k.GetNextHealOpID(sdkCtx)
	if genesis.NextHealOpId == 0 {
		return nil, errors.New("invalid next heal op id")
	}

	// Per 121-F7 — export storage-truth postponement markers.
	genesis.StorageTruthPostponements = k.GetAllStorageTruthPostponements(sdkCtx)

	// Per final-gate F-B2/F-B3/F-B4 — export additional genesis-covered
	// action-finalization markers, evidence aggregates, and heal-op votes.
	genesis.ActionFinalizationPostponements = k.GetAllActionFinalizationPostponements(sdkCtx)
	genesis.EvidenceEpochCounts = k.GetAllEvidenceEpochCountsForGenesis(sdkCtx)
	genesis.HealOpVerifications = k.GetAllHealOpVerificationsForGenesis(sdkCtx)

	// Per NEW-C-1 — export every epoch-scoped audit prefix family.
	genesis.RecheckEvidence = k.GetAllRecheckEvidenceForGenesis(sdkCtx)
	genesis.StorageProofTranscripts = k.GetAllStorageProofTranscriptsForGenesis(sdkCtx)
	genesis.NodeFailureFacts = k.GetAllNodeFailureFactsForGenesis(sdkCtx)
	genesis.ReporterResultFacts = k.GetAllReporterResultFactsForGenesis(sdkCtx)
	genesis.FailedHealMarkers = k.GetAllFailedHealMarkersForGenesis(sdkCtx)
	reports, err := k.GetAllReportsForGenesis(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.EpochReports = reports
	genesis.ReportIndices = k.GetAllReportIndicesForGenesis(sdkCtx)
	genesis.HostReportIndices = k.GetAllHostReportIndicesForGenesis(sdkCtx)
	genesis.StorageChallengeIndices = k.GetAllStorageChallengeIndicesForGenesis(sdkCtx)

	return genesis, nil
}
