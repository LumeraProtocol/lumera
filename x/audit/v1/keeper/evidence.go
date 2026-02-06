package keeper

import (
	"context"
	"fmt"
	"strings"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/gogoproto/jsonpb"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

const (
	// Evidence is currently unpruned and stored on-chain, so these must remain bounded to
	// avoid state-bloat/DoS. Keep them "roomy" enough to avoid accidental breakage.
	maxEvidenceMetadataJSONBytes = 2 * 1024 * 1024 // 2 MiB
	maxEvidenceActionIDBytes     = 4 * 1024        // 4 KiB

	// Applies to non-SC evidence types until/unless a dedicated param is introduced.
	maxEvidenceMetadataBytes = 256 * 1024 // 256 KiB

	maxScChallengeIDBytes    = 256      // 256 B
	maxScFileKeyBytes        = 4 * 1024 // 4 KiB
	maxScFailureTypeBytes    = 256      // 256 B
	maxScTranscriptHashBytes = 256      // 256 B
)

func (k Keeper) CreateEvidence(
	ctx context.Context,
	reporterAddress string,
	subjectAddress string,
	actionID string,
	evidenceType types.EvidenceType,
	metadataJSON string,
) (uint64, error) {
	if _, err := k.addressCodec.StringToBytes(reporterAddress); err != nil {
		return 0, errorsmod.Wrap(types.ErrInvalidReporter, err.Error())
	}
	if _, err := k.addressCodec.StringToBytes(subjectAddress); err != nil {
		return 0, errorsmod.Wrap(types.ErrInvalidSubject, err.Error())
	}

	if evidenceType == types.EvidenceType_EVIDENCE_TYPE_UNSPECIFIED {
		return 0, types.ErrInvalidEvidenceType
	}

	switch evidenceType {
	case types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
		types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE,
		types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10:
		expectedReporter, err := k.addressCodec.BytesToString(authtypes.NewModuleAddress("action"))
		if err != nil {
			return 0, errorsmod.Wrap(types.ErrInvalidReporter, err.Error())
		}
		if reporterAddress != expectedReporter {
			return 0, errorsmod.Wrap(types.ErrInvalidReporter, "reporter must be the action module account")
		}
	}

	metadataJSON = strings.TrimSpace(metadataJSON)
	if metadataJSON == "" {
		return 0, types.ErrInvalidMetadata
	}
	if len(metadataJSON) > maxEvidenceMetadataJSONBytes {
		return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "metadata is too large")
	}

	if actionID == "" {
		// For the initial supported evidence types (action expiration/finalization), action id is required.
		switch evidenceType {
		case types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
			types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE,
			types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10:
			return 0, types.ErrInvalidActionID
		}
	}
	if len(actionID) > maxEvidenceActionIDBytes {
		return 0, errorsmod.Wrap(types.ErrInvalidActionID, "action_id is too large")
	}

	metadataBytes, err := marshalEvidenceMetadataJSON(evidenceType, metadataJSON)
	if err != nil {
		return 0, errorsmod.Wrap(types.ErrInvalidMetadata, err.Error())
	}
	if evidenceType != types.EvidenceType_EVIDENCE_TYPE_STORAGE_CHALLENGE_FAILURE && len(metadataBytes) > maxEvidenceMetadataBytes {
		return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "metadata is too large")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if evidenceType == types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED ||
		evidenceType == types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE ||
		evidenceType == types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10 {
		params := k.GetParams(ctx).WithDefaults()
		epoch, err := deriveEpochAtHeight(sdkCtx.BlockHeight(), params)
		if err != nil {
			return 0, err
		}
		k.incrementEvidenceEpochCount(sdkCtx, epoch.EpochID, subjectAddress, evidenceType)
	}

	if evidenceType == types.EvidenceType_EVIDENCE_TYPE_STORAGE_CHALLENGE_FAILURE {
		params := k.GetParams(ctx).WithDefaults()
		if params.ScEvidenceMaxBytes > 0 && uint64(len(metadataBytes)) > params.ScEvidenceMaxBytes {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "metadata exceeds sc_evidence_max_bytes")
		}

		var m types.StorageChallengeFailureEvidenceMetadata
		if err := gogoproto.Unmarshal(metadataBytes, &m); err != nil {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("unmarshal StorageChallengeFailureEvidenceMetadata: %v", err))
		}
		if strings.TrimSpace(m.ChallengerSupernodeAccount) == "" {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "challenger_supernode_account is required")
		}
		if reporterAddress != m.ChallengerSupernodeAccount {
			return 0, errorsmod.Wrap(types.ErrInvalidReporter, "reporter must match challenger_supernode_account")
		}
		if strings.TrimSpace(m.ChallengedSupernodeAccount) == "" {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "challenged_supernode_account is required")
		}
		if subjectAddress != m.ChallengedSupernodeAccount {
			return 0, errorsmod.Wrap(types.ErrInvalidSubject, "subject_address must match challenged_supernode_account")
		}
		if len(strings.TrimSpace(m.ChallengeId)) > maxScChallengeIDBytes {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "challenge_id is too large")
		}
		if len(strings.TrimSpace(m.FileKey)) > maxScFileKeyBytes {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "file_key is too large")
		}
		if len(strings.TrimSpace(m.FailureType)) > maxScFailureTypeBytes {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "failure_type is too large")
		}
		if len(strings.TrimSpace(m.TranscriptHash)) > maxScTranscriptHashBytes {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, "transcript_hash is too large")
		}

		anchor, found := k.GetEpochAnchor(sdkCtx, m.EpochId)
		if !found {
			return 0, errorsmod.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("epoch anchor not found for epoch_id %d", m.EpochId))
		}

		if params.ScEnabled && params.ScEvidenceSubmitterMustBeChallenger {
			kc := storageChallengeChallengerCount(len(anchor.ActiveSupernodeAccounts), params.ScChallengersPerEpoch)
			target := storageChallengeComparisonTarget(anchor.Seed, m.EpochId)
			challengers := selectTopByXORDistance(anchor.ActiveSupernodeAccounts, target, kc)

			allowed := false
			for _, c := range challengers {
				if c == reporterAddress {
					allowed = true
					break
				}
			}
			if !allowed {
				return 0, errorsmod.Wrap(types.ErrInvalidReporter, "reporter is not an authorized challenger for epoch")
			}
		}

		// Optional consistency check: ensure subject was eligible as a target at epoch start.
		eligible := false
		for _, t := range anchor.TargetSupernodeAccounts {
			if t == subjectAddress {
				eligible = true
				break
			}
		}
		if !eligible {
			return 0, errorsmod.Wrap(types.ErrInvalidSubject, "subject is not an eligible target for epoch")
		}
	}
	reportedHeight := uint64(sdkCtx.BlockHeight())

	evidenceID := k.GetNextEvidenceID(sdkCtx)
	k.SetNextEvidenceID(sdkCtx, evidenceID+1)

	ev := types.Evidence{
		EvidenceId:      evidenceID,
		SubjectAddress:  subjectAddress,
		ReporterAddress: reporterAddress,
		ActionId:        actionID,
		EvidenceType:    evidenceType,
		Metadata:        metadataBytes,
		ReportedHeight:  reportedHeight,
	}

	if err := k.SetEvidence(sdkCtx, ev); err != nil {
		return 0, err
	}
	k.SetEvidenceBySubjectIndex(sdkCtx, subjectAddress, evidenceID)
	if actionID != "" {
		k.SetEvidenceByActionIndex(sdkCtx, actionID, evidenceID)
	}

	return evidenceID, nil
}

func marshalEvidenceMetadataJSON(evidenceType types.EvidenceType, metadataJSON string) ([]byte, error) {
	u := &jsonpb.Unmarshaler{}

	switch evidenceType {
	case types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED:
		var m types.ActionExpiredEvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal ActionExpiredEvidenceMetadata: %w", err)
		}
		return gogoproto.Marshal(&m)

	case types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE:
		var m types.ActionFinalizationSignatureFailureEvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal ActionFinalizationSignatureFailureEvidenceMetadata: %w", err)
		}
		return gogoproto.Marshal(&m)

	case types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10:
		var m types.ActionFinalizationNotInTop10EvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal ActionFinalizationNotInTop10EvidenceMetadata: %w", err)
		}
		return gogoproto.Marshal(&m)

	case types.EvidenceType_EVIDENCE_TYPE_STORAGE_CHALLENGE_FAILURE:
		var m types.StorageChallengeFailureEvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal StorageChallengeFailureEvidenceMetadata: %w", err)
		}
		return gogoproto.Marshal(&m)

	default:
		return nil, fmt.Errorf("unsupported evidence_type: %s", evidenceType.String())
	}
}
