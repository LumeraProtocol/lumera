package keeper

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (m msgServer) SubmitStorageRecheckEvidence(ctx context.Context, req *types.MsgSubmitStorageRecheckEvidence) (*types.MsgSubmitStorageRecheckEvidenceResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}
	if req.Creator == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator is required")
	}
	if req.ChallengedSupernodeAccount == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "challenged_supernode_account is required")
	}
	if req.ChallengedSupernodeAccount == req.Creator {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "challenged_supernode_account must not equal creator")
	}
	if req.TicketId == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "ticket_id is required")
	}
	if req.ChallengedResultTranscriptHash == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "challenged_result_transcript_hash is required")
	}
	if req.RecheckTranscriptHash == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "recheck_transcript_hash is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if _, found := m.GetEpochAnchor(sdkCtx, req.EpochId); !found {
		return nil, errorsmod.Wrapf(types.ErrInvalidEpochID, "epoch anchor not found for epoch_id %d", req.EpochId)
	}

	if _, found, err := m.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.Creator); err != nil {
		return nil, err
	} else if !found {
		return nil, errorsmod.Wrap(types.ErrReporterNotFound, "creator is not a registered supernode")
	}
	if _, found, err := m.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.ChallengedSupernodeAccount); err != nil {
		return nil, err
	} else if !found {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "challenged_supernode_account is not a registered supernode")
	}

	switch req.RecheckResultClass {
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL:
	default:
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "recheck_result_class is invalid")
	}

	challengedRecord, found, err := m.getStorageProofTranscriptRecord(sdkCtx, req.ChallengedResultTranscriptHash)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "challenged_result_transcript_hash does not reference a submitted storage proof result")
	}
	if challengedRecord.EpochID != req.EpochId {
		return nil, errorsmod.Wrapf(types.ErrInvalidRecheckEvidence, "challenged result epoch %d does not match request epoch %d", challengedRecord.EpochID, req.EpochId)
	}
	if challengedRecord.TicketID != req.TicketId {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "challenged result ticket_id does not match request ticket_id")
	}
	if challengedRecord.TargetAccount != req.ChallengedSupernodeAccount {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "challenged result target does not match challenged_supernode_account")
	}
	if challengedRecord.ReporterAccount == req.Creator {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "creator must be independent from the challenged result reporter")
	}
	if !challengedRecord.RecheckEligible {
		return nil, errorsmod.Wrap(types.ErrInvalidRecheckEvidence, "challenged result class is not recheck-eligible")
	}

	// Replay protection: one recheck per (epoch, ticket, creator).
	if m.HasRecheckEvidence(sdkCtx, req.EpochId, req.TicketId, req.Creator) {
		return nil, errorsmod.Wrapf(types.ErrInvalidRecheckEvidence, "recheck evidence already submitted for epoch %d ticket %q by %q", req.EpochId, req.TicketId, req.Creator)
	}
	m.SetRecheckEvidence(sdkCtx, req.EpochId, req.TicketId, req.Creator)
	if err := m.linkStorageTruthRecheckTranscript(
		sdkCtx,
		req.ChallengedResultTranscriptHash,
		req.RecheckTranscriptHash,
		req.Creator,
		req.RecheckResultClass,
	); err != nil {
		return nil, err
	}

	// Derive current epoch for scoring context.
	params := m.GetParams(sdkCtx).WithDefaults()
	currentEpoch, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params)
	if err != nil {
		return nil, err
	}

	// Capture the original reporter BEFORE applyStorageTruthScores updates the ticket state.
	// If the recheck result is PASS (overturn), we apply a +25 penalty to the original reporter.
	// If the recheck result is RECHECK_CONFIRMED_FAIL (confirms), we apply a -3 reward.
	var overturnOriginalReporter, confirmOriginalReporter string
	if ticketState, found := m.GetTicketDeteriorationState(sdkCtx, req.TicketId); found {
		origReporter := ticketState.LastReporterSupernodeAccount
		if origReporter != "" && origReporter != req.Creator {
			switch req.RecheckResultClass {
			case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS:
				overturnOriginalReporter = origReporter
			case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL:
				confirmOriginalReporter = origReporter
			}
		}
	}

	// Synthesise a StorageProofResult carrying the recheck outcome and apply scores.
	recheckResult := &types.StorageProofResult{
		TicketId:               req.TicketId,
		TargetSupernodeAccount: req.ChallengedSupernodeAccount,
		ResultClass:            req.RecheckResultClass,
		BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK,
	}
	if err := m.applyStorageTruthScores(sdkCtx, currentEpoch.EpochID, req.Creator, []*types.StorageProofResult{recheckResult}); err != nil {
		return nil, err
	}

	// Recheck overturn penalty: if recheck result is PASS, it overturns the original fail.
	// Penalize the original reporter by +25.
	if overturnOriginalReporter != "" {
		if _, _, err := m.applyReporterReliabilityDelta(
			sdkCtx,
			currentEpoch.EpochID,
			overturnOriginalReporter,
			25, // +25 overturn penalty
			params.StorageTruthReporterReliabilityDecayPerEpoch,
			1,
			params,
		); err != nil {
			return nil, err
		}
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeStorageTruthScoreUpdated,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(currentEpoch.EpochID, 10)),
			sdk.NewAttribute(types.AttributeKeyContradictedReporter, overturnOriginalReporter),
			sdk.NewAttribute(types.AttributeKeyRecheckResultClass, req.RecheckResultClass.String()),
		))
	}

	// §15.3: recheck confirms original fail — reward the correct original reporter with -3.
	if confirmOriginalReporter != "" {
		if _, _, err := m.applyReporterReliabilityDelta(
			sdkCtx,
			currentEpoch.EpochID,
			confirmOriginalReporter,
			-3, // recovery credit for confirmed correct fail
			params.StorageTruthReporterReliabilityDecayPerEpoch,
			0,
			params,
		); err != nil {
			return nil, err
		}
	}
	if overturnOriginalReporter != "" {
		if err := m.markStorageTruthReporterResultRecheck(sdkCtx, overturnOriginalReporter, req.ChallengedResultTranscriptHash, false); err != nil {
			return nil, err
		}
	}
	if confirmOriginalReporter != "" {
		if err := m.markStorageTruthReporterResultRecheck(sdkCtx, confirmOriginalReporter, req.ChallengedResultTranscriptHash, true); err != nil {
			return nil, err
		}
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeStorageRecheckEvidence,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(req.EpochId, 10)),
		sdk.NewAttribute(types.AttributeKeyReporterSupernodeAccount, req.Creator),
		sdk.NewAttribute(types.AttributeKeyTargetSupernodeAccount, req.ChallengedSupernodeAccount),
		sdk.NewAttribute(types.AttributeKeyTicketID, req.TicketId),
		sdk.NewAttribute(types.AttributeKeyRecheckResultClass, req.RecheckResultClass.String()),
	))

	return &types.MsgSubmitStorageRecheckEvidenceResponse{}, nil
}

func (m msgServer) ClaimHealComplete(ctx context.Context, req *types.MsgClaimHealComplete) (*types.MsgClaimHealCompleteResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}
	if req.Creator == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator is required")
	}
	if req.HealOpId == 0 {
		return nil, errorsmod.Wrap(types.ErrHealOpNotFound, "heal_op_id is required")
	}
	if req.TicketId == "" {
		return nil, errorsmod.Wrap(types.ErrHealOpTicketMismatch, "ticket_id is required")
	}
	if req.HealManifestHash == "" {
		return nil, errorsmod.Wrap(types.ErrHealOpInvalidState, "heal_manifest_hash is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	healOp, found := m.GetHealOp(sdkCtx, req.HealOpId)
	if !found {
		return nil, errorsmod.Wrapf(types.ErrHealOpNotFound, "heal op %d not found", req.HealOpId)
	}
	if healOp.TicketId != req.TicketId {
		return nil, errorsmod.Wrapf(types.ErrHealOpTicketMismatch, "ticket_id %q does not match heal op ticket_id %q", req.TicketId, healOp.TicketId)
	}
	if healOp.HealerSupernodeAccount != req.Creator {
		return nil, errorsmod.Wrap(types.ErrHealOpUnauthorized, "creator is not assigned healer for this heal op")
	}
	if healOp.Status != types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED && healOp.Status != types.HealOpStatus_HEAL_OP_STATUS_IN_PROGRESS {
		return nil, errorsmod.Wrapf(types.ErrHealOpInvalidState, "heal op status %s does not accept healer completion claim", healOp.Status.String())
	}

	healOp.Status = types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED
	healOp.UpdatedHeight = uint64(sdkCtx.BlockHeight())
	healOp.ResultHash = req.HealManifestHash
	healOp.Notes = appendStorageTruthNote(healOp.Notes, req.Details)

	if len(healOp.VerifierSupernodeAccounts) == 0 {
		return nil, errorsmod.Wrap(types.ErrHealOpInvalidState, "heal op has no independent verifier assignments")
	}

	if err := m.SetHealOp(sdkCtx, healOp); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeHealOpHealerReported,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyHealOpID, strconv.FormatUint(healOp.HealOpId, 10)),
			sdk.NewAttribute(types.AttributeKeyTicketID, healOp.TicketId),
			sdk.NewAttribute(types.AttributeKeyHealerSupernodeAccount, req.Creator),
			sdk.NewAttribute(types.AttributeKeyTranscriptHash, req.HealManifestHash),
		),
	)

	return &types.MsgClaimHealCompleteResponse{}, nil
}

func (m msgServer) SubmitHealVerification(ctx context.Context, req *types.MsgSubmitHealVerification) (*types.MsgSubmitHealVerificationResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}
	if req.Creator == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator is required")
	}
	if req.HealOpId == 0 {
		return nil, errorsmod.Wrap(types.ErrHealOpNotFound, "heal_op_id is required")
	}
	if req.VerificationHash == "" {
		return nil, errorsmod.Wrap(types.ErrHealOpInvalidState, "verification_hash is required")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	healOp, found := m.GetHealOp(sdkCtx, req.HealOpId)
	if !found {
		return nil, errorsmod.Wrapf(types.ErrHealOpNotFound, "heal op %d not found", req.HealOpId)
	}
	if healOp.Status != types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED {
		return nil, errorsmod.Wrapf(types.ErrHealOpInvalidState, "heal op status %s does not accept verification", healOp.Status.String())
	}
	if !containsString(healOp.VerifierSupernodeAccounts, req.Creator) {
		return nil, errorsmod.Wrap(types.ErrHealOpUnauthorized, "creator is not assigned verifier for this heal op")
	}
	if m.HasHealOpVerification(sdkCtx, req.HealOpId, req.Creator) {
		return nil, errorsmod.Wrap(types.ErrHealVerificationExists, "verification already submitted by creator")
	}

	m.SetHealOpVerification(sdkCtx, req.HealOpId, req.Creator, req.Verified)

	verifications, err := m.GetAllHealOpVerifications(sdkCtx, req.HealOpId)
	if err != nil {
		return nil, err
	}

	positive := 0
	negative := 0
	for _, verifier := range healOp.VerifierSupernodeAccounts {
		v, ok := verifications[verifier]
		if !ok {
			continue
		}
		if v {
			positive++
		} else {
			negative++
		}
	}

	// Majority quorum: need majority of verifiers to agree (positive or negative).
	n := len(healOp.VerifierSupernodeAccounts)
	majority := n/2 + 1

	if negative >= majority {
		if err := m.finalizeHealOp(sdkCtx, healOp, false, req.VerificationHash, req.Details); err != nil {
			return nil, err
		}
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeHealOpFailed,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyHealOpID, strconv.FormatUint(healOp.HealOpId, 10)),
				sdk.NewAttribute(types.AttributeKeyTicketID, healOp.TicketId),
				sdk.NewAttribute(types.AttributeKeyVerifierSupernodeAccount, req.Creator),
				sdk.NewAttribute(types.AttributeKeyVerified, strconv.FormatBool(req.Verified)),
			),
		)
		return &types.MsgSubmitHealVerificationResponse{}, nil
	}

	if positive >= majority {
		if err := m.finalizeHealOp(sdkCtx, healOp, true, req.VerificationHash, req.Details); err != nil {
			return nil, err
		}
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeHealOpVerified,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyHealOpID, strconv.FormatUint(healOp.HealOpId, 10)),
				sdk.NewAttribute(types.AttributeKeyTicketID, healOp.TicketId),
				sdk.NewAttribute(types.AttributeKeyVerifierSupernodeAccount, req.Creator),
				sdk.NewAttribute(types.AttributeKeyVerificationHash, req.VerificationHash),
			),
		)
		return &types.MsgSubmitHealVerificationResponse{}, nil
	}

	// Not enough votes yet — accumulate and wait.
	return &types.MsgSubmitHealVerificationResponse{}, nil
}

func (m msgServer) finalizeHealOp(
	ctx sdk.Context,
	healOp types.HealOp,
	verified bool,
	verificationHash string,
	details string,
) error {
	if verified {
		healOp.Status = types.HealOpStatus_HEAL_OP_STATUS_VERIFIED
	} else {
		healOp.Status = types.HealOpStatus_HEAL_OP_STATUS_FAILED
	}
	healOp.UpdatedHeight = uint64(ctx.BlockHeight())
	if verificationHash != "" {
		healOp.ResultHash = verificationHash
	}
	healOp.Notes = appendStorageTruthNote(healOp.Notes, details)
	if err := m.SetHealOp(ctx, healOp); err != nil {
		return err
	}

	ticketState, found := m.GetTicketDeteriorationState(ctx, healOp.TicketId)
	if !found {
		return nil
	}
	if ticketState.ActiveHealOpId == healOp.HealOpId {
		ticketState.ActiveHealOpId = 0
	}

	params := m.GetParams(ctx).WithDefaults()
	currentEpoch, err := deriveEpochAtHeight(ctx.BlockHeight(), params)
	if err != nil {
		return err
	}

	if verified {
		// Post-heal score reset: D = max(8, floor(D_old * 0.25))
		oldScore := ticketState.DeteriorationScore
		resetScore := oldScore / 4
		if resetScore < 8 {
			resetScore = 8
		}
		ticketState.DeteriorationScore = resetScore
		ticketState.LastHealEpoch = currentEpoch.EpochID
		ticketState.ProbationUntilEpoch = currentEpoch.EpochID + uint64(params.StorageTruthProbationEpochs)
	} else {
		// Failed heal: D += 15
		ticketState.DeteriorationScore = addInt64Saturated(ticketState.DeteriorationScore, 15)
		// Failed heals enter a cooldown window before re-scheduling.
		cooldownUntil := currentEpoch.EpochID + uint64(params.StorageTruthProbationEpochs)
		if ticketState.ProbationUntilEpoch < cooldownUntil {
			ticketState.ProbationUntilEpoch = cooldownUntil
		}
		m.setStorageTruthFailedHeal(ctx, healOp.HealerSupernodeAccount, currentEpoch.EpochID, healOp.TicketId)
	}
	return m.SetTicketDeteriorationState(ctx, ticketState)
}

func containsString(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func appendStorageTruthNote(existing, note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return existing
	}
	if existing == "" {
		return note
	}
	return fmt.Sprintf("%s | %s", existing, note)
}
