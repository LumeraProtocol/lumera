package keeper

import (
	"fmt"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/action/v1/merkle"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

// SVCChallengeCount is the number of chunks challenged during SVC verification.
const SVCChallengeCount uint32 = 8

// SVCMinChunksForChallenge is the minimum number of chunks a file must produce
// for SVC verification to apply. Files with fewer chunks skip SVC.
const SVCMinChunksForChallenge uint32 = 4

// VerifyChunkProofs validates LEP-5 chunk proofs for Cascade finalization.
//
// The expected challenge indices are read from the AvailabilityCommitment
// stored at registration time. Each proof must match the corresponding
// stored index and verify against the committed Merkle root.
func (k *Keeper) VerifyChunkProofs(
	ctx sdk.Context,
	action *actiontypes.Action,
	superNodeAccount string,
	proofs []*actiontypes.ChunkProof,
) error {
	if action == nil {
		return errorsmod.Wrap(actiontypes.ErrInvalidMetadata, "action is nil")
	}

	var metadata actiontypes.CascadeMetadata
	if err := gogoproto.Unmarshal(action.Metadata, &metadata); err != nil {
		return errorsmod.Wrapf(actiontypes.ErrInvalidMetadata, "failed to unmarshal cascade metadata: %v", err)
	}

	commitment := metadata.GetAvailabilityCommitment()
	if commitment == nil {
		// Backward compatibility: pre-LEP-5 actions do not include commitments.
		return nil
	}

	if commitment.NumChunks < SVCMinChunksForChallenge {
		// Small files are out of challenge scope.
		return nil
	}

	expectedCount := SVCChallengeCount
	if expectedCount > commitment.NumChunks {
		expectedCount = commitment.NumChunks
	}

	if uint32(len(proofs)) != expectedCount {
		err := errorsmod.Wrapf(actiontypes.ErrWrongProofCount, "expected %d proofs, got %d", expectedCount, len(proofs))
		emitSVCEvidenceEvent(ctx, action.ActionID, superNodeAccount, err.Error())
		return err
	}

	root, err := bytesToMerkleHash("availability_commitment.root", commitment.Root)
	if err != nil {
		wrapped := errorsmod.Wrap(actiontypes.ErrInvalidMerkleProof, err.Error())
		emitSVCEvidenceEvent(ctx, action.ActionID, superNodeAccount, wrapped.Error())
		return wrapped
	}

	// Read expected challenge indices from the stored commitment.
	expectedIndices := commitment.ChallengeIndices
	if uint32(len(expectedIndices)) != expectedCount {
		err = errorsmod.Wrapf(actiontypes.ErrInvalidMetadata,
			"commitment has %d challenge_indices, expected %d", len(expectedIndices), expectedCount)
		emitSVCEvidenceEvent(ctx, action.ActionID, superNodeAccount, err.Error())
		return err
	}

	for i, proof := range proofs {
		if proof == nil {
			err = errorsmod.Wrapf(actiontypes.ErrInvalidMerkleProof, "proof %d is nil", i)
			emitSVCEvidenceEvent(ctx, action.ActionID, superNodeAccount, err.Error(),
				sdk.NewAttribute(actiontypes.AttributeKeyProofIndex, strconv.Itoa(i)),
			)
			return err
		}

		if proof.ChunkIndex != expectedIndices[i] {
			err = errorsmod.Wrapf(
				actiontypes.ErrWrongChallengeIndex,
				"proof %d: expected index %d, got %d",
				i,
				expectedIndices[i],
				proof.ChunkIndex,
			)
			emitSVCEvidenceEvent(ctx, action.ActionID, superNodeAccount, err.Error(),
				sdk.NewAttribute(actiontypes.AttributeKeyProofIndex, strconv.Itoa(i)),
				sdk.NewAttribute(actiontypes.AttributeKeyExpectedChunkIndex, strconv.FormatUint(uint64(expectedIndices[i]), 10)),
				sdk.NewAttribute(actiontypes.AttributeKeyChunkIndex, strconv.FormatUint(uint64(proof.ChunkIndex), 10)),
			)
			return err
		}

		merkleProof, convErr := chunkProofToMerkleProof(proof)
		if convErr != nil {
			err = errorsmod.Wrap(actiontypes.ErrInvalidMerkleProof, convErr.Error())
			emitSVCEvidenceEvent(ctx, action.ActionID, superNodeAccount, err.Error(),
				sdk.NewAttribute(actiontypes.AttributeKeyProofIndex, strconv.Itoa(i)),
				sdk.NewAttribute(actiontypes.AttributeKeyChunkIndex, strconv.FormatUint(uint64(proof.ChunkIndex), 10)),
			)
			return err
		}

		if !merkleProof.Verify(root) {
			err = errorsmod.Wrapf(actiontypes.ErrInvalidMerkleProof, "proof for chunk %d failed verification", proof.ChunkIndex)
			emitSVCEvidenceEvent(ctx, action.ActionID, superNodeAccount, err.Error(),
				sdk.NewAttribute(actiontypes.AttributeKeyProofIndex, strconv.Itoa(i)),
				sdk.NewAttribute(actiontypes.AttributeKeyChunkIndex, strconv.FormatUint(uint64(proof.ChunkIndex), 10)),
			)
			return err
		}
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			actiontypes.EventTypeSVCVerificationPassed,
			sdk.NewAttribute(actiontypes.AttributeKeyActionID, action.ActionID),
			sdk.NewAttribute(actiontypes.AttributeKeySuperNodes, superNodeAccount),
		),
	)

	return nil
}


func chunkProofToMerkleProof(proof *actiontypes.ChunkProof) (*merkle.Proof, error) {
	if len(proof.PathHashes) != len(proof.PathDirections) {
		return nil, fmt.Errorf("path_hashes/path_directions length mismatch: %d/%d", len(proof.PathHashes), len(proof.PathDirections))
	}

	leafHash, err := bytesToMerkleHash("leaf_hash", proof.LeafHash)
	if err != nil {
		return nil, err
	}

	pathHashes := make([][merkle.HashSize]byte, 0, len(proof.PathHashes))
	for i, pathHash := range proof.PathHashes {
		decoded, decodeErr := bytesToMerkleHash(fmt.Sprintf("path_hashes[%d]", i), pathHash)
		if decodeErr != nil {
			return nil, decodeErr
		}
		pathHashes = append(pathHashes, decoded)
	}

	return &merkle.Proof{
		ChunkIndex:     proof.ChunkIndex,
		LeafHash:       leafHash,
		PathHashes:     pathHashes,
		PathDirections: proof.PathDirections,
	}, nil
}

func bytesToMerkleHash(field string, value []byte) ([merkle.HashSize]byte, error) {
	var out [merkle.HashSize]byte
	if len(value) != merkle.HashSize {
		return out, fmt.Errorf("%s must be %d bytes, got %d", field, merkle.HashSize, len(value))
	}
	copy(out[:], value)
	return out, nil
}

func emitSVCEvidenceEvent(ctx sdk.Context, actionID, superNodeAccount, reason string, attrs ...sdk.Attribute) {
	eventAttrs := []sdk.Attribute{
		sdk.NewAttribute(actiontypes.AttributeKeyActionID, actionID),
		sdk.NewAttribute(actiontypes.AttributeKeySuperNodes, superNodeAccount),
		sdk.NewAttribute(actiontypes.AttributeKeyError, reason),
	}
	eventAttrs = append(eventAttrs, attrs...)

	ctx.EventManager().EmitEvent(sdk.NewEvent(actiontypes.EventTypeSVCEvidence, eventAttrs...))
}
