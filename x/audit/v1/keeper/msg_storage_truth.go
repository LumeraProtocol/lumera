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

	return nil, errorsmod.Wrap(
		types.ErrNotImplemented,
		"storage recheck submission is not active in the LEP-6 heal-op lifecycle milestone",
	)
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

	// Single-node networks may not have verifier assignments; finalize immediately.
	if len(healOp.VerifierSupernodeAccounts) == 0 {
		if err := m.finalizeHealOp(sdkCtx, healOp, true, req.HealManifestHash, req.Details); err != nil {
			return nil, err
		}
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeHealOpVerified,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyHealOpID, strconv.FormatUint(healOp.HealOpId, 10)),
				sdk.NewAttribute(types.AttributeKeyTicketID, healOp.TicketId),
				sdk.NewAttribute(types.AttributeKeyHealerSupernodeAccount, req.Creator),
			),
		)
		return &types.MsgClaimHealCompleteResponse{}, nil
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

	if negative > 0 {
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

	if positive == len(healOp.VerifierSupernodeAccounts) {
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
	if verified {
		currentEpoch, err := deriveEpochAtHeight(ctx.BlockHeight(), m.GetParams(ctx).WithDefaults())
		if err != nil {
			return err
		}
		ticketState.LastHealEpoch = currentEpoch.EpochID
		ticketState.ProbationUntilEpoch = currentEpoch.EpochID + uint64(m.GetParams(ctx).WithDefaults().StorageTruthProbationEpochs)
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
