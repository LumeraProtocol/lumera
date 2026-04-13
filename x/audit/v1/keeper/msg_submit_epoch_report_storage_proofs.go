package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// MaxStorageProofResultsPerReport bounds the number of StorageProofResult
// entries a single epoch report may carry. Per LEP-6 §6–§7 a prober submits
// roughly 1 target × 2 buckets (RECENT + OLD) per epoch, plus a small
// recheck/probation budget. 16 leaves ~8× headroom while preventing both
// unbounded validation work and unbounded permanent on-chain growth via
// EpochReport persistence. If real traffic ever exceeds this, the cap can
// be promoted to a govvable chain param (see LEP-6 pre-master checklist).
const MaxStorageProofResultsPerReport = 16

type storageProofDescriptorKey struct {
	target        string
	bucket        types.StorageProofBucketType
	ticketID      string
	artifactClass types.StorageProofArtifactClass
	artifactOrd   uint32
}

func validateStorageProofResults(reporterAccount string, allowedTargets map[string]struct{}, isProber bool, results []*types.StorageProofResult) error {
	if len(results) == 0 {
		return nil
	}
	if !isProber {
		return errorsmod.Wrap(types.ErrInvalidReporterState, "reporter not eligible for storage proof results in this epoch")
	}
	if len(results) > MaxStorageProofResultsPerReport {
		return errorsmod.Wrapf(
			types.ErrInvalidStorageProofs,
			"storage_proof_results length %d exceeds per-report cap %d",
			len(results), MaxStorageProofResultsPerReport,
		)
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

		// Per LEP-6 §10, descriptor identity is (target, bucket, ticket_id, artifact_class, artifact_ordinal).
		// artifact_key is a deterministic function of that tuple and intentionally excluded
		// to prevent dedup bypass via attacker-supplied alternate keys for the same logical descriptor.
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

	return nil
}
