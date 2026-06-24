package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// Per PR #118 / Zee F1 — artifactKey intentionally excluded so duplicate fail
// reports across artifacts of the same target+ticket are caught.
type storageProofDescriptorKey struct {
	target        string
	bucket        types.StorageProofBucketType
	ticketID      string
	artifactClass types.StorageProofArtifactClass
	artifactOrd   uint32
}

func validateStorageProofResults(
	reporterAccount string,
	allowedTargets map[string]struct{},
	isProber bool,
	enforceCompoundCoverage bool,
	results []*types.StorageProofResult,
) error {
	if len(results) == 0 {
		if enforceCompoundCoverage && isProber && len(allowedTargets) > 0 {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, "storage_proof_results must include RECENT and OLD entries for every assigned target")
		}
		return nil
	}
	if !isProber {
		return errorsmod.Wrap(types.ErrInvalidReporterState, "reporter not eligible for storage proof results in this epoch")
	}

	seen := make(map[storageProofDescriptorKey]struct{}, len(results))
	for i, result := range results {
		fieldName := fmt.Sprintf("storage_proof_results[%d]", i)
		if result == nil {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+" must not be nil")
		}
		if result.TargetSupernodeAccount == "" {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".target_supernode_account is required")
		}
		if result.ChallengerSupernodeAccount == "" {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".challenger_supernode_account is required")
		}
		if result.ChallengerSupernodeAccount != reporterAccount {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".challenger_supernode_account must match report creator")
		}
		if result.TargetSupernodeAccount == reporterAccount {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+" must not target the reporter")
		}
		if _, ok := allowedTargets[result.TargetSupernodeAccount]; !ok {
			return errorsmod.Wrapf(types.ErrInvalidStorageProofs, "%s.target_supernode_account %q is not assigned to reporter in this epoch", fieldName, result.TargetSupernodeAccount)
		}
		if result.TranscriptHash == "" {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".transcript_hash is required")
		}

		switch result.BucketType {
		case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD,
			types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_PROBATION,
			types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK:
		default:
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".bucket_type is invalid")
		}

		switch result.ResultClass {
		case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL,
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET,
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT,
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL:
		default:
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".result_class is invalid")
		}

		if result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET {
			if result.BucketType != types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT &&
				result.BucketType != types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".result_class NO_ELIGIBLE_TICKET is only valid for RECENT or OLD buckets")
			}
			if result.TicketId != "" {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".ticket_id must be empty for NO_ELIGIBLE_TICKET")
			}
			if result.ArtifactClass != types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_UNSPECIFIED {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".artifact_class must be UNSPECIFIED for NO_ELIGIBLE_TICKET")
			}
			if result.ArtifactOrdinal != 0 {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".artifact_ordinal must be 0 for NO_ELIGIBLE_TICKET")
			}
			if result.ArtifactKey != "" {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".artifact_key must be empty for NO_ELIGIBLE_TICKET")
			}
		} else {
			if result.TicketId == "" {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".ticket_id is required")
			}
			if result.ArtifactCount == 0 {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".artifact_count must be > 0")
			}
			if result.ArtifactOrdinal >= result.ArtifactCount {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".artifact_ordinal must be < artifact_count")
			}
			if result.DerivationInputHash == "" {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".derivation_input_hash is required")
			}
			if result.ChallengerSignature == "" {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".challenger_signature is required")
			}
			switch result.ArtifactClass {
			case types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX,
				types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_SYMBOL:
			default:
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".artifact_class is invalid")
			}
			if result.ArtifactKey == "" {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".artifact_key is required")
			}
		}

		if result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL &&
			result.BucketType != types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".result_class RECHECK_CONFIRMED_FAIL requires RECHECK bucket")
		}

		key := storageProofDescriptorKey{
			target:        result.TargetSupernodeAccount,
			bucket:        result.BucketType,
			ticketID:      result.TicketId,
			artifactClass: result.ArtifactClass,
			artifactOrd:   result.ArtifactOrdinal,
		}
		if _, dup := seen[key]; dup {
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+" duplicates another storage proof result descriptor")
		}
		seen[key] = struct{}{}
	}

	if enforceCompoundCoverage {
		if err := validateCompoundStorageProofCoverage(allowedTargets, results); err != nil {
			return err
		}
	}

	return nil
}

func (k Keeper) validateStorageProofArtifactCounts(
	ctx sdk.Context,
	epochID uint64,
	params types.Params,
	results []*types.StorageProofResult,
) error {
	if len(results) == 0 {
		return nil
	}

	for i, result := range results {
		if result == nil {
			continue
		}
		fieldName := fmt.Sprintf("storage_proof_results[%d]", i)
		if result.ResultClass == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET {
			if err := k.validateNoEligibleTicketConsistency(ctx, epochID, params, result, fieldName); err != nil {
				return err
			}
			continue
		}

		state, found := k.GetTicketArtifactCountState(ctx, result.TicketId)

		// Per 122-F2 — legacy 0-count tickets fall back to cascadeMeta length to avoid finalization brick.
		var canonicalCount uint32
		if found {
			switch result.ArtifactClass {
			case types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX:
				canonicalCount = state.IndexArtifactCount
			case types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_SYMBOL:
				canonicalCount = state.SymbolArtifactCount
			}
		}
		if !found || canonicalCount == 0 {
			// Legacy ticket with no anchored count — accept reporter's count as fallback.
			ctx.EventManager().EmitEvent(sdk.NewEvent(
				types.EventTypeArtifactCountUnanchored,
				sdk.NewAttribute(types.AttributeKeyTicketID, result.TicketId),
			))
			// Do NOT block finalization on the fallback; ticket finalizes with legacy count.
			continue
		}

		switch result.ArtifactClass {
		case types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX:
			if state.IndexArtifactCount != result.ArtifactCount {
				return errorsmod.Wrapf(
					types.ErrTicketArtifactMismatch,
					"%s index artifact_count %d does not match canonical count %d for ticket %q",
					fieldName,
					result.ArtifactCount,
					state.IndexArtifactCount,
					result.TicketId,
				)
			}
			if result.ArtifactOrdinal >= state.IndexArtifactCount {
				return errorsmod.Wrapf(
					types.ErrInvalidStorageProofs,
					"%s artifact_ordinal %d out of range for canonical index artifact_count %d",
					fieldName,
					result.ArtifactOrdinal,
					state.IndexArtifactCount,
				)
			}
		case types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_SYMBOL:
			if state.SymbolArtifactCount != result.ArtifactCount {
				return errorsmod.Wrapf(
					types.ErrTicketArtifactMismatch,
					"%s symbol artifact_count %d does not match canonical count %d for ticket %q",
					fieldName,
					result.ArtifactCount,
					state.SymbolArtifactCount,
					result.TicketId,
				)
			}
			if result.ArtifactOrdinal >= state.SymbolArtifactCount {
				return errorsmod.Wrapf(
					types.ErrInvalidStorageProofs,
					"%s artifact_ordinal %d out of range for canonical symbol artifact_count %d",
					fieldName,
					result.ArtifactOrdinal,
					state.SymbolArtifactCount,
				)
			}
		default:
			return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+".artifact_class is invalid")
		}
	}
	return nil
}

func (k Keeper) validateNoEligibleTicketConsistency(
	ctx sdk.Context,
	epochID uint64,
	params types.Params,
	result *types.StorageProofResult,
	fieldName string,
) error {
	if result == nil ||
		result.ResultClass != types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET ||
		result.TargetSupernodeAccount == "" {
		return nil
	}

	window := storageTruthNoEligibleConsistencyWindow(result.BucketType, params)
	startEpoch := storageTruthWindowStart(epochID, window)
	seenEligible, err := k.hasObservedEligibleTicketForTargetBucketInWindow(
		ctx,
		result.TargetSupernodeAccount,
		result.BucketType,
		startEpoch,
		epochID,
	)
	if err != nil {
		return err
	}
	if seenEligible {
		return errorsmod.Wrapf(
			types.ErrInvalidStorageProofs,
			"%s NO_ELIGIBLE_TICKET conflicts with recently observed eligible ticket history for target %q bucket %s",
			fieldName,
			result.TargetSupernodeAccount,
			result.BucketType.String(),
		)
	}
	return nil
}

func storageTruthNoEligibleConsistencyWindow(bucket types.StorageProofBucketType, params types.Params) uint64 {
	switch bucket {
	case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT:
		if params.EpochLengthBlocks == 0 || params.StorageTruthRecentBucketMaxBlocks == 0 {
			return 3
		}
		window := params.StorageTruthRecentBucketMaxBlocks / params.EpochLengthBlocks
		if params.StorageTruthRecentBucketMaxBlocks%params.EpochLengthBlocks != 0 {
			window++
		}
		if window == 0 {
			window = 1
		}
		return window
	case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD:
		// Use the old-Class-A lookback baseline so recent known OLD eligibility cannot
		// be suppressed by NO_ELIGIBLE submissions.
		window := uint64(params.StorageTruthOldClassAFaultWindow)
		if window == 0 {
			window = 21
		}
		return window
	default:
		window := uint64(params.StorageTruthPatternEscalationWindow)
		if window == 0 {
			window = 14
		}
		return window
	}
}

func (k Keeper) hasObservedEligibleTicketForTargetBucketInWindow(
	ctx sdk.Context,
	target string,
	bucket types.StorageProofBucketType,
	startEpoch uint64,
	endEpoch uint64,
) (bool, error) {
	if target == "" {
		return false, nil
	}
	// Per 122-Copilot-5 + 122-F1 — indexed lookup avoids DeliverTx full-table scan.
	// Scan secondary index: "st/spt-tbe/" + target + "/" + u32be(bucket) + "/" epoch range.
	bucketPfx := types.TranscriptByTargetBucketEpochScanPrefix(target, uint32(bucket))
	startKey := binary.BigEndian.AppendUint64(append([]byte(nil), bucketPfx...), startEpoch)
	endKey := binary.BigEndian.AppendUint64(append([]byte(nil), bucketPfx...), endEpoch+1)
	it := k.kvStore(ctx).Iterator(startKey, endKey)
	defer func() { _ = it.Close() }()

	for ; it.Valid(); it.Next() {
		var record storageProofTranscriptRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return false, err
		}
		if types.StorageProofResultClass(record.ResultClass) == types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET {
			continue
		}
		if record.TicketID == "" {
			continue
		}
		return true, nil
	}
	return false, nil
}

func validateCompoundStorageProofCoverage(allowedTargets map[string]struct{}, results []*types.StorageProofResult) error {
	type bucketCoverage struct {
		recent bool
		old    bool
	}

	coverage := make(map[string]bucketCoverage, len(allowedTargets))
	for target := range allowedTargets {
		coverage[target] = bucketCoverage{}
	}

	for i, result := range results {
		if result == nil {
			continue
		}
		cov, ok := coverage[result.TargetSupernodeAccount]
		if !ok {
			continue
		}
		fieldName := fmt.Sprintf("storage_proof_results[%d]", i)
		switch result.BucketType {
		case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT:
			if cov.recent {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+" duplicates RECENT storage proof for assigned target")
			}
			cov.recent = true
		case types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_OLD:
			if cov.old {
				return errorsmod.Wrap(types.ErrInvalidStorageProofs, fieldName+" duplicates OLD storage proof for assigned target")
			}
			cov.old = true
		default:
			continue
		}
		coverage[result.TargetSupernodeAccount] = cov
	}

	for target, cov := range coverage {
		if !cov.recent || !cov.old {
			return errorsmod.Wrapf(types.ErrInvalidStorageProofs, "assigned target %q must have exactly one RECENT and one OLD storage proof result", target)
		}
	}
	return nil
}
