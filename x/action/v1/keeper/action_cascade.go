package keeper

import (
	"encoding/json"
	"fmt"
	"github.com/LumeraProtocol/lumera/x/action/v1/common"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"reflect"
	"strings"

	"cosmossdk.io/errors"
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"
)

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
func (h CascadeActionHandler) Process(metadataBytes []byte, msgType common.MessageType, params *types2.Params) ([]byte, error) {
	var metadata actionapi.CascadeMetadata
	if err := protojson.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cascade metadata: %w", err)
	}

	// Process based on message type (currently just validating)
	switch msgType {
	case common.MsgRequestAction:
		if metadata.DataHash == "" {
			return nil, fmt.Errorf("data_hash is required for cascade metadata")
		}
		if metadata.FileName == "" {
			return nil, fmt.Errorf("file_name is required for cascade metadata")
		}
		if metadata.RqIdsIc == 0 {
			return nil, fmt.Errorf("rq_ids_ic is required for cascade metadata")
		}
		if metadata.Signatures == "" {
			return nil, fmt.Errorf("signatures is required for cascade metadata")
		}
		if params == nil {
			return nil, fmt.Errorf("params is required for cascade metadata")
		}
		metadata.RqIdsMax = params.MaxRaptorQSymbols
	case common.MsgFinalizeAction:
		if len(metadata.RqIdsIds) == 0 {
			return nil, fmt.Errorf("rq_ids_ids is required for cascade metadata")
		}
	default:
		return nil, fmt.Errorf("unsupported message type: %s", msgType)
	}

	// Convert to protobuf binary format for more efficient storage
	return proto.Marshal(&metadata)
}

// GetProtoMessageType returns the reflect.Type for CascadeMetadata
func (h CascadeActionHandler) GetProtoMessageType() reflect.Type {
	return reflect.TypeOf(actionapi.CascadeMetadata{})
}

// ConvertJSONToProtobuf converts JSON metadata to protobuf binary format
func (h CascadeActionHandler) ConvertJSONToProtobuf(jsonData []byte) ([]byte, error) {
	var metadata actionapi.CascadeMetadata
	if err := protojson.Unmarshal(jsonData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cascade metadata from JSON: %w", err)
	}

	// Marshal to protobuf binary format
	return proto.Marshal(&metadata)
}

// ConvertProtobufToJSON converts protobuf binary metadata to JSON format
func (h CascadeActionHandler) ConvertProtobufToJSON(protobufData []byte) ([]byte, error) {
	var metadata actionapi.CascadeMetadata
	if err := proto.Unmarshal(protobufData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cascade metadata from protobuf: %w", err)
	}

	return json.Marshal(&metadata)
}

// RegisterAction handles action-specific validation and processing during action registration
func (h CascadeActionHandler) RegisterAction(ctx sdk.Context, action *actionapi.Action) error {
	if action.Metadata == nil {
		return errors.Wrap(types2.ErrInvalidMetadata, "metadata is required for cascade actions")
	}
	var cascadeMeta actionapi.CascadeMetadata
	if err := proto.Unmarshal(action.Metadata, &cascadeMeta); err != nil {
		return errors.Wrap(types2.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal cascade metadata: %v", err))
	}

	// Validate Signature. Signature field contains: `Base64(rq_ids).creators_signature`
	// Where `creators_signature` is the signature of the creator over `Base64(rq_ids)`
	signatureParts := strings.Split(cascadeMeta.Signatures, ".")
	if len(signatureParts) != 2 {
		return errors.Wrap(types2.ErrInvalidMetadata, "invalid signature format")
	}

	// Use VerifySignature from crypto.go to validate the signature in signatureParts[1] over the data in signatureParts[0].
	dataToVerify := signatureParts[0]
	creatorSignature := signatureParts[1] // the signature is Base64 encoded, VerifySignature will decode it

	if err := h.keeper.VerifySignature(ctx, dataToVerify, creatorSignature, action.Creator); err != nil {
		return errors.Wrap(types2.ErrInvalidMetadata, fmt.Sprintf("failed to verify creator's signature: %v", err))
	}
	return nil
}

// FinalizeAction validates finalization data for Cascade actions
// Returns the recommended action state or ActionState_ACTION_STATE_UNSPECIFIED if no change
func (h CascadeActionHandler) FinalizeAction(ctx sdk.Context, action *actionapi.Action, superNodeAccount string, metadataBytes []byte) (actionapi.ActionState, error) {
	h.keeper.Logger().Info("Validating Cascade action finalization",
		"action_id", action.ActionID,
		"supernode", superNodeAccount)

	// Verify Registration Metadata exists
	var existingCascadeMeta actionapi.CascadeMetadata
	if len(action.Metadata) > 0 {
		if err := proto.Unmarshal(action.Metadata, &existingCascadeMeta); err != nil {
			return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
				errors.Wrap(sdkerrors.ErrInvalidRequest, fmt.Sprintf("failed to unmarshal existing cascade metadata: %v", err))
		}
	}
	if existingCascadeMeta.DataHash == "" {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "data_hash is required in existing metadata")
	}
	if existingCascadeMeta.FileName == "" {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "file_name is required in existing metadata")
	}
	if existingCascadeMeta.RqIdsIc == 0 {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "rq_ids_ic is required in existing metadata")
	}
	if existingCascadeMeta.RqIdsMax == 0 {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "rq_ids_max is required in existing metadata")
	}
	if len(existingCascadeMeta.Signatures) == 0 {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "signatures is required in existing metadata")
	}

	var newCascadeMeta actionapi.CascadeMetadata
	if err := proto.Unmarshal(metadataBytes, &newCascadeMeta); err != nil {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal cascade metadata: %v", err))
	}

	// 1. Verify RqIdsIds
	if err := VerifyKademliaIDs(newCascadeMeta.RqIdsIds, existingCascadeMeta.Signatures, existingCascadeMeta.RqIdsIc, existingCascadeMeta.RqIdsMax); err != nil {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, fmt.Sprintf("failed to verify rq_ids_ids: %v", err))
	}

	// Cascade actions are finalized with a single supernode
	// Return DONE state since all validations passed
	return actionapi.ActionState_ACTION_STATE_DONE, nil
}

// ValidateApproval validates approval data for Cascade actions
func (h CascadeActionHandler) ValidateApproval(ctx sdk.Context, action *actionapi.Action) error {
	// Empty implementation - will be filled in later
	return nil
}
