package keeper

import (
	"encoding/json"
	"fmt"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k *Keeper) recordFinalizationEvidence(
	ctx sdk.Context,
	actionID string,
	subjectAddress string,
	attemptedFinalizerAddress string,
	expectedFinalizerAddresses []string,
	reason string,
	evidenceType audittypes.EvidenceType,
) (uint64, error) {
	if k.auditKeeper == nil {
		return 0, fmt.Errorf("audit keeper is not configured")
	}
	if actionID == "" {
		return 0, fmt.Errorf("actionID is required")
	}

	reporterAddress, err := k.addressCodec.BytesToString(actiontypes.ModuleAccountAddress)
	if err != nil {
		return 0, fmt.Errorf("module reporter address: %w", err)
	}

	metaJSON, err := json.Marshal(audittypes.FinalizationEvidenceMetadata{
		AttemptedFinalizerAddress:  attemptedFinalizerAddress,
		ExpectedFinalizerAddresses: expectedFinalizerAddresses,
		Reason:                     reason,
	})
	if err != nil {
		return 0, fmt.Errorf("marshal evidence metadata: %w", err)
	}

	return k.auditKeeper.CreateEvidence(
		ctx,
		reporterAddress,
		subjectAddress,
		actionID,
		evidenceType,
		string(metaJSON),
	)
}

func (k *Keeper) recordFinalizationRejection(
	ctx sdk.Context,
	action *actiontypes.Action,
	attemptedFinalizerAddress string,
	reason string,
	expectedFinalizerAddresses []string,
	evidenceType audittypes.EvidenceType,
) uint64 {
	if action == nil {
		return 0
	}

	// Use the attempted finalizer as the default audited subject.
	subjectAddress := attemptedFinalizerAddress

	var evidenceID uint64
	if id, err := k.recordFinalizationEvidence(
		ctx,
		action.ActionID,
		subjectAddress,
		attemptedFinalizerAddress,
		expectedFinalizerAddresses,
		reason,
		evidenceType,
	); err != nil {
		k.Logger().Error(
			"failed to record finalization evidence",
			"action_id", action.ActionID,
			"finalizer", attemptedFinalizerAddress,
			"err", err,
		)
	} else {
		evidenceID = id
	}

	attrs := []sdk.Attribute{
		sdk.NewAttribute(actiontypes.AttributeKeyActionID, action.ActionID),
		sdk.NewAttribute(actiontypes.AttributeKeyCreator, action.Creator),
		sdk.NewAttribute(actiontypes.AttributeKeyFinalizer, attemptedFinalizerAddress),
		sdk.NewAttribute(actiontypes.AttributeKeyActionType, action.ActionType.String()),
		sdk.NewAttribute(actiontypes.AttributeKeyError, reason),
	}
	if evidenceID != 0 {
		attrs = append(attrs, sdk.NewAttribute(actiontypes.AttributeKeyEvidenceID, fmt.Sprintf("%d", evidenceID)))
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(actiontypes.EventTypeActionFinalizationRejected, attrs...),
	)

	return evidenceID
}

func (k *Keeper) RecordFinalizationSignatureFailure(
	ctx sdk.Context,
	action *actiontypes.Action,
	attemptedFinalizerAddress string,
	reason string,
) uint64 {
	return k.recordFinalizationRejection(
		ctx,
		action,
		attemptedFinalizerAddress,
		reason,
		nil,
		audittypes.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE,
	)
}

func (k *Keeper) RecordFinalizationNotInTop10(
	ctx sdk.Context,
	action *actiontypes.Action,
	attemptedFinalizerAddress string,
	expectedFinalizerAddresses []string,
	reason string,
) uint64 {
	return k.recordFinalizationRejection(
		ctx,
		action,
		attemptedFinalizerAddress,
		reason,
		expectedFinalizerAddresses,
		audittypes.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10,
	)
}
