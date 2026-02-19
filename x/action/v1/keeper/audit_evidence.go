package keeper

import (
	"encoding/json"
	"fmt"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ctxKeyTop10ValidatorAddresses struct{}

func top10ValidatorAddressesFromContext(ctx sdk.Context) []string {
	v := ctx.Value(ctxKeyTop10ValidatorAddresses{})
	if v == nil {
		return nil
	}
	addrs, ok := v.([]string)
	if !ok {
		return nil
	}
	return addrs
}

func (k *Keeper) recordFinalizationEvidence(
	ctx sdk.Context,
	actionID string,
	subjectAddress string,
	top10ValidatorAddresses []string,
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

	var metadataJSON string
	switch evidenceType {
	case audittypes.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE:
		metaJSON, err := json.Marshal(audittypes.ActionFinalizationSignatureFailureEvidenceMetadata{
			Top_10ValidatorAddresses: top10ValidatorAddresses,
		})
		if err != nil {
			return 0, fmt.Errorf("marshal evidence metadata: %w", err)
		}
		metadataJSON = string(metaJSON)
	case audittypes.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10:
		metaJSON, err := json.Marshal(audittypes.ActionFinalizationNotInTop10EvidenceMetadata{
			Top_10ValidatorAddresses: top10ValidatorAddresses,
		})
		if err != nil {
			return 0, fmt.Errorf("marshal evidence metadata: %w", err)
		}
		metadataJSON = string(metaJSON)
	default:
		return 0, fmt.Errorf("unsupported finalization evidence type: %s", evidenceType.String())
	}

	return k.auditKeeper.CreateEvidence(
		ctx,
		reporterAddress,
		subjectAddress,
		actionID,
		evidenceType,
		metadataJSON,
	)
}

func (k *Keeper) recordFinalizationRejection(
	ctx sdk.Context,
	action *actiontypes.Action,
	attemptedFinalizerAddress string,
	reason string,
	top10ValidatorAddresses []string,
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
		top10ValidatorAddresses,
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
	top10ValidatorAddresses := top10ValidatorAddressesFromContext(ctx)
	return k.recordFinalizationRejection(
		ctx,
		action,
		attemptedFinalizerAddress,
		reason,
		top10ValidatorAddresses,
		audittypes.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE,
	)
}

func (k *Keeper) RecordFinalizationNotInTop10(
	ctx sdk.Context,
	action *actiontypes.Action,
	attemptedFinalizerAddress string,
	top10ValidatorAddresses []string,
	reason string,
) uint64 {
	return k.recordFinalizationRejection(
		ctx,
		action,
		attemptedFinalizerAddress,
		reason,
		top10ValidatorAddresses,
		audittypes.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10,
	)
}

func (k *Keeper) RecordActionExpired(ctx sdk.Context, action *actiontypes.Action) {
	if k.auditKeeper == nil || action == nil {
		return
	}
	if action.ActionID == "" {
		k.Logger().Error("failed to record action expiration evidence: action_id is required")
		return
	}

	reporterAddress, err := k.addressCodec.BytesToString(actiontypes.ModuleAccountAddress)
	if err != nil {
		k.Logger().Error("failed to record action expiration evidence: module reporter address", "err", err)
		return
	}

	topSuperNodesReq := &sntypes.QueryGetTopSuperNodesForBlockRequest{
		BlockHeight: int32(action.BlockHeight),
		Limit:       10,
		State:       sntypes.SuperNodeStateActive.String(),
	}
	topSuperNodesResp, err := k.supernodeQueryServer.GetTopSuperNodesForBlock(ctx, topSuperNodesReq)
	if err != nil {
		k.Logger().Error("failed to record action expiration evidence: query top supernodes", "action_id", action.ActionID, "err", err)
		return
	}

	top10ValidatorAddresses := make([]string, 0, len(topSuperNodesResp.Supernodes))
	for _, sn := range topSuperNodesResp.Supernodes {
		top10ValidatorAddresses = append(top10ValidatorAddresses, sn.ValidatorAddress)
	}

	metaJSON, err := json.Marshal(audittypes.ActionExpiredEvidenceMetadata{
		Top_10ValidatorAddresses: top10ValidatorAddresses,
	})
	if err != nil {
		k.Logger().Error("failed to record action expiration evidence: marshal evidence metadata", "action_id", action.ActionID, "err", err)
		return
	}

	metadataJSON := string(metaJSON)
	for _, sn := range topSuperNodesResp.Supernodes {
		if sn.SupernodeAccount == "" {
			continue
		}
		if _, err := k.auditKeeper.CreateEvidence(
			ctx,
			reporterAddress,
			sn.SupernodeAccount,
			action.ActionID,
			audittypes.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
			metadataJSON,
		); err != nil {
			k.Logger().Error(
				"failed to record action expiration evidence",
				"action_id", action.ActionID,
				"subject", sn.SupernodeAccount,
				"err", err,
			)
		}
	}
}
