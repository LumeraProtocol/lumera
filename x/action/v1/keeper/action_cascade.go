package keeper

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/action/v1/common"

	"cosmossdk.io/errors"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/jsonpb"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

const (
	cascadeCommitmentType         = "lep5/chunk-merkle/v1"
	cascadeCommitmentMaxChunkSize = uint32(262144) // 256 KiB — default / ceiling
	cascadeCommitmentMinChunkSize = uint32(1)      // 1 byte — floor
	cascadeCommitmentRootSize     = 32
	cascadeCommitmentMinTotalSize = uint64(4)      // reject trivially tiny files (< 4 bytes)
)

var cascadeCommitmentHashAlgo = actiontypes.HashAlgo_HASH_ALGO_BLAKE3

// isPowerOf2 returns true if v is a positive power of two.
func isPowerOf2(v uint32) bool {
	return v > 0 && (v&(v-1)) == 0
}

func validateAvailabilityCommitment(commitment *actiontypes.AvailabilityCommitment, challengeCount, minChunks uint32) error {
	if commitment == nil {
		return nil
	}

	if commitment.CommitmentType != cascadeCommitmentType {
		return fmt.Errorf("availability_commitment.commitment_type must be %q", cascadeCommitmentType)
	}
	if commitment.HashAlgo != cascadeCommitmentHashAlgo {
		return fmt.Errorf("availability_commitment.hash_algo must be %q", cascadeCommitmentHashAlgo)
	}

	// Reject trivially tiny files.
	if commitment.TotalSize < cascadeCommitmentMinTotalSize {
		return fmt.Errorf("availability_commitment.total_size must be >= %d bytes, got %d",
			cascadeCommitmentMinTotalSize, commitment.TotalSize)
	}

	// chunk_size must be a power of 2 in [minChunkSize, maxChunkSize].
	if !isPowerOf2(commitment.ChunkSize) {
		return fmt.Errorf("availability_commitment.chunk_size must be a power of 2, got %d", commitment.ChunkSize)
	}
	if commitment.ChunkSize < cascadeCommitmentMinChunkSize || commitment.ChunkSize > cascadeCommitmentMaxChunkSize {
		return fmt.Errorf("availability_commitment.chunk_size must be in [%d, %d], got %d",
			cascadeCommitmentMinChunkSize, cascadeCommitmentMaxChunkSize, commitment.ChunkSize)
	}

	expectedNumChunks := uint32((commitment.TotalSize + uint64(commitment.ChunkSize) - 1) / uint64(commitment.ChunkSize))
	if commitment.NumChunks != expectedNumChunks {
		return fmt.Errorf("availability_commitment.num_chunks must be %d for total_size %d and chunk_size %d", expectedNumChunks, commitment.TotalSize, commitment.ChunkSize)
	}

	// Unconditionally enforce minimum chunk count. The client MUST pick a
	// chunk_size that yields >= minChunks chunks (default 4).
	if commitment.NumChunks < minChunks {
		return fmt.Errorf("availability_commitment.num_chunks %d is below minimum %d: file of %d bytes must produce at least %d chunks (reduce chunk_size)",
			commitment.NumChunks, minChunks, commitment.TotalSize, minChunks)
	}

	if len(commitment.Root) != cascadeCommitmentRootSize {
		return fmt.Errorf("availability_commitment.root must be %d bytes", cascadeCommitmentRootSize)
	}

	// Validate challenge indices when the file is large enough for SVC.
	if commitment.NumChunks >= minChunks {
		expectedIndices := challengeCount
		if expectedIndices > commitment.NumChunks {
			expectedIndices = commitment.NumChunks
		}
		if uint32(len(commitment.ChallengeIndices)) != expectedIndices {
			return fmt.Errorf("availability_commitment.challenge_indices must have %d entries, got %d",
				expectedIndices, len(commitment.ChallengeIndices))
		}
		seen := make(map[uint32]bool, len(commitment.ChallengeIndices))
		for i, idx := range commitment.ChallengeIndices {
			if idx >= commitment.NumChunks {
				return fmt.Errorf("availability_commitment.challenge_indices[%d] = %d is out of range [0, %d)",
					i, idx, commitment.NumChunks)
			}
			if seen[idx] {
				return fmt.Errorf("availability_commitment.challenge_indices[%d] = %d is a duplicate", i, idx)
			}
			seen[idx] = true
		}
	}

	return nil
}

// CascadeActionHandler implements the ActionHandler interface for Cascade actions
type CascadeActionHandler struct {
	keeper *Keeper // Reference to the keeper for logger and other services
}

// NewCascadeActionHandler creates a new CascadeActionHandler
func NewCascadeActionHandler(k *Keeper) *CascadeActionHandler {
	return &CascadeActionHandler{
		keeper: k,
	}
}

// Process validates and handles any necessary transformations for CascadeMetadata
func (h CascadeActionHandler) Process(metadataBytes []byte, msgType common.MessageType, params *actiontypes.Params) ([]byte, error) {
	var metadata actiontypes.CascadeMetadata
	unmarshaler := &jsonpb.Unmarshaler{}
	if err := unmarshaler.Unmarshal(strings.NewReader(string(metadataBytes)), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cascade metadata from JSON: %w", err)
	}

	// Process based on message type (currently just validating)
	switch msgType {
	case common.MsgRequestAction:
		if metadata.DataHash == "" {
			return nil, fmt.Errorf("data_hash field is required for cascade metadata")
		}
		if metadata.FileName == "" {
			return nil, fmt.Errorf("file_name field is required for cascade metadata")
		}
		if metadata.RqIdsIc == 0 {
			return nil, fmt.Errorf("rq_ids_ic fieldis required for cascade metadata")
		}
		if metadata.Signatures == "" {
			return nil, fmt.Errorf("signatures field is required for cascade metadata")
		}
		if params == nil {
			return nil, fmt.Errorf("params field is required for cascade metadata")
		}
		if err := validateAvailabilityCommitment(metadata.AvailabilityCommitment, SVCChallengeCount, SVCMinChunksForChallenge); err != nil {
			return nil, err
		}
		metadata.RqIdsMax = params.MaxRaptorQSymbols
	case common.MsgFinalizeAction:
		if len(metadata.RqIdsIds) == 0 {
			return nil, fmt.Errorf("rq_ids_ids field is required for cascade metadata")
		}
	default:
		return nil, fmt.Errorf("unsupported message type: %s", msgType)
	}

	// Convert to protobuf binary format for more efficient storage
	return gogoproto.Marshal(&metadata)
}

// GetProtoMessageType returns the reflect.Type for CascadeMetadata
func (h CascadeActionHandler) GetProtoMessageType() reflect.Type {
	return reflect.TypeOf(actiontypes.CascadeMetadata{})
}

// ConvertJSONToProtobuf converts JSON metadata to gogo protobuf binary format
func (h CascadeActionHandler) ConvertJSONToProtobuf(jsonData []byte) ([]byte, error) {
	var metadata actiontypes.CascadeMetadata
	// Unmarshal JSON to CascadeMetadata struct
	unmarshaler := &jsonpb.Unmarshaler{}
	if err := unmarshaler.Unmarshal(strings.NewReader(string(jsonData)), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cascade metadata from JSON: %w", err)
	}

	// Marshal to protobuf binary format
	return gogoproto.Marshal(&metadata)
}

// ConvertProtobufToJSON converts gogo protobuf binary metadata to JSON format
func (h CascadeActionHandler) ConvertProtobufToJSON(protobufData []byte) ([]byte, error) {
	var metadata actiontypes.CascadeMetadata
	if err := gogoproto.Unmarshal(protobufData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cascade metadata from protobuf: %w", err)
	}

	// Marshal to JSON format
	marshaler := &jsonpb.Marshaler{
		EmitDefaults: true,
		EnumsAsInts:  true,
	}
	var buf bytes.Buffer
	if err := marshaler.Marshal(&buf, &metadata); err != nil {
		return nil, fmt.Errorf("failed to marshal cascade metadata to JSON: %w", err)
	}
	return buf.Bytes(), nil
}

// RegisterAction handles action-specific validation and processing during action registration
func (h CascadeActionHandler) RegisterAction(ctx sdk.Context, action *actiontypes.Action) error {
	if action.Metadata == nil {
		return errors.Wrap(actiontypes.ErrInvalidMetadata, "metadata is required for cascade actions")
	}
	var cascadeMeta actiontypes.CascadeMetadata
	if err := gogoproto.Unmarshal(action.Metadata, &cascadeMeta); err != nil {
		return errors.Wrap(actiontypes.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal cascade metadata: %v", err))
	}
	if err := validateAvailabilityCommitment(cascadeMeta.AvailabilityCommitment, SVCChallengeCount, SVCMinChunksForChallenge); err != nil {
		return errors.Wrap(actiontypes.ErrInvalidMetadata, err.Error())
	}

	// Validate Signature. Signature field contains: `Base64(rq_ids).creators_signature`
	// Where `creators_signature` is the signature of the creator over `Base64(rq_ids)`
	signatureParts := strings.Split(cascadeMeta.Signatures, ".")
	if len(signatureParts) != 2 {
		return errors.Wrap(actiontypes.ErrInvalidMetadata, "invalid signature format")
	}

	// Use VerifySignature from crypto.go to validate the signature in signatureParts[1] over the data in signatureParts[0].
	dataToVerify := signatureParts[0]
	creatorSignature := signatureParts[1] // the signature is Base64 encoded, VerifySignature will decode it

	// Reuse already-fetched creator account when possible to avoid duplicate lookups.
	if err := h.keeper.VerifySignature(ctx, dataToVerify, creatorSignature, action.Creator); err != nil {
		return errors.Wrap(actiontypes.ErrInvalidMetadata, fmt.Sprintf("failed to verify creator's signature: %v", err))
	}
	return nil
}

// FinalizeAction validates finalization data for Cascade actions
// Returns the recommended action state or ActionStateUnspecified if no change
func (h CascadeActionHandler) FinalizeAction(ctx sdk.Context, action *actiontypes.Action, superNodeAccount string, metadataBytes []byte) (actiontypes.ActionState, error) {
	h.keeper.Logger().Info("Validating Cascade action finalization",
		"action_id", action.ActionID,
		"supernode", superNodeAccount)

	// Verify Registration Metadata exists
	var existingCascadeMeta actiontypes.CascadeMetadata
	if len(action.Metadata) > 0 {
		if err := gogoproto.Unmarshal(action.Metadata, &existingCascadeMeta); err != nil {
			return actiontypes.ActionStateUnspecified,
				errors.Wrap(sdkerrors.ErrInvalidRequest, fmt.Sprintf("failed to unmarshal existing cascade metadata: %v", err))
		}
	}
	if existingCascadeMeta.DataHash == "" {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "data_hash is required in existing metadata")
	}
	if existingCascadeMeta.FileName == "" {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "file_name is required in existing metadata")
	}
	if existingCascadeMeta.RqIdsIc == 0 {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "rq_ids_ic is required in existing metadata")
	}
	if existingCascadeMeta.RqIdsMax == 0 {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "rq_ids_max is required in existing metadata")
	}
	if len(existingCascadeMeta.Signatures) == 0 {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "signatures is required in existing metadata")
	}

	var newCascadeMeta actiontypes.CascadeMetadata
	if err := gogoproto.Unmarshal(metadataBytes, &newCascadeMeta); err != nil {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal cascade metadata: %v", err))
	}

	// 1. Verify RqIdsIds
	if err := VerifyKademliaIDs(newCascadeMeta.RqIdsIds, existingCascadeMeta.Signatures, existingCascadeMeta.RqIdsIc, existingCascadeMeta.RqIdsMax); err != nil {
		h.keeper.RecordFinalizationSignatureFailure(
			ctx,
			action,
			superNodeAccount,
			fmt.Sprintf("cascade finalization rejected: rq_ids_ids verification failed: %v", err),
		)
		return actiontypes.ActionStateUnspecified, nil
	}

	if err := h.keeper.VerifyChunkProofs(ctx, action, superNodeAccount, newCascadeMeta.GetChunkProofs()); err != nil {
		return actiontypes.ActionStateUnspecified, err
	}

	// Cascade actions are finalized with a single supernode
	// Return DONE state since all validations passed
	return actiontypes.ActionStateDone, nil
}

// ValidateApproval validates approval data for Cascade actions
func (h CascadeActionHandler) ValidateApproval(ctx sdk.Context, action *actiontypes.Action) error {
	// Empty implementation - will be filled in later
	return nil
}

func (h CascadeActionHandler) GetUpdatedMetadata(ctx sdk.Context, existingMetadataBytes, newMetadataBytes []byte) ([]byte, error) {
	var (
		existingMetadata, newMetadata actiontypes.CascadeMetadata
	)

	err := gogoproto.Unmarshal(existingMetadataBytes, &existingMetadata)
	if err != nil {
		return nil, errors.Wrap(actiontypes.ErrInternalError, fmt.Sprintf("failed to unmarshal existing metadata: %v", err))
	}

	err = gogoproto.Unmarshal(newMetadataBytes, &newMetadata)
	if err != nil {
		return nil, errors.Wrap(actiontypes.ErrInternalError, fmt.Sprintf("failed to unmarshal new metadata: %v", err))
	}

	updatedMetadata := &actiontypes.CascadeMetadata{
		RqIdsIc:    existingMetadata.GetRqIdsIc(),
		RqIdsMax:   existingMetadata.GetRqIdsMax(),
		DataHash:   existingMetadata.GetDataHash(),
		FileName:   existingMetadata.GetFileName(),
		Signatures: existingMetadata.GetSignatures(),
		RqIdsIds:   newMetadata.GetRqIdsIds(),
		Public:     existingMetadata.GetPublic(),
		AvailabilityCommitment: existingMetadata.GetAvailabilityCommitment(),
		ChunkProofs:            newMetadata.GetChunkProofs(),
	}

	return gogoproto.Marshal(updatedMetadata)
}
