package keeper

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/LumeraProtocol/lumera/x/action/v1/common"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"google.golang.org/protobuf/encoding/protojson"

	"cosmossdk.io/errors"
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"google.golang.org/protobuf/proto"
)

// SenseActionHandler implements the ActionHandler interface for Sense actions
type SenseActionHandler struct {
	keeper *Keeper // Reference to the keeper for logger and other services
}

// NewSenseActionHandler creates a new SenseActionHandler
func NewSenseActionHandler(k *Keeper) *SenseActionHandler {
	return &SenseActionHandler{
		keeper: k,
	}
}

// Process handles any necessary transformations for SenseMetadata
func (h SenseActionHandler) Process(metadataBytes []byte, msgType common.MessageType, params *types2.Params) ([]byte, error) {
	var metadata actionapi.SenseMetadata
	if err := protojson.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sense metadata: %w", err)
	}

	// Validate fields based on message type
	switch msgType {
	case common.MsgRequestAction:
		if metadata.DataHash == "" {
			return nil, fmt.Errorf("data_hash is required for sense metadata")
		}
		if metadata.DdAndFingerprintsIc == 0 {
			return nil, fmt.Errorf("dd_and_fingerprints_ic is required for sense metadata")
		}
		if params == nil {
			return nil, fmt.Errorf("params is required for cascade metadata")
		}
		metadata.DdAndFingerprintsMax = params.MaxDdAndFingerprints
	case common.MsgFinalizeAction:
		if len(metadata.DdAndFingerprintsIds) == 0 {
			return nil, fmt.Errorf("dd_and_fingerprints_ids is required for sense metadata")
		}
		if metadata.Signatures == "" {
			return nil, fmt.Errorf("signatures is required for sense metadata")
		}
	default:
		return nil, fmt.Errorf("unsupported message type: %s", msgType)
	}

	// Convert to protobuf binary format for more efficient storage
	return proto.Marshal(&metadata)
}

// GetProtoMessageType returns the reflect.Type for SenseMetadata
func (h SenseActionHandler) GetProtoMessageType() reflect.Type {
	return reflect.TypeOf(actionapi.SenseMetadata{})
}

// ConvertJSONToProtobuf converts JSON metadata to protobuf binary format
func (h SenseActionHandler) ConvertJSONToProtobuf(jsonData []byte) ([]byte, error) {
	var metadata actionapi.SenseMetadata
	if err := protojson.Unmarshal(jsonData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sense metadata from JSON: %w", err)
	}

	// Marshal to protobuf binary format
	return proto.Marshal(&metadata)
}

// ConvertProtobufToJSON converts protobuf binary metadata to JSON format
func (h SenseActionHandler) ConvertProtobufToJSON(protobufData []byte) ([]byte, error) {
	var metadata actionapi.SenseMetadata
	if err := proto.Unmarshal(protobufData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sense metadata from protobuf: %w", err)
	}

	return json.Marshal(&metadata)
}

// RegisterAction handles action-specific validation and processing during action registration
func (h SenseActionHandler) RegisterAction(_ sdk.Context, action *actionapi.Action) error {
	if action.Metadata == nil {
		return errors.Wrap(types2.ErrInvalidMetadata, "metadata is required for cascade actions")
	}
	// There is nothing else to do for Sense actions during registration
	//var newSenseMeta actionapi.SenseMetadata
	//if err := proto.Unmarshal(action.Metadata, &newSenseMeta); err != nil {
	//	return errors.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal sense metadata: %v", err))
	//}

	return nil
}

// FinalizeAction validates finalization data for Sense actions
// Returns the recommended action state or ActionState_ACTION_STATE_UNSPECIFIED if no change
func (h SenseActionHandler) FinalizeAction(ctx sdk.Context, action *actionapi.Action, superNodeAccount string, metadataBytes []byte) (actionapi.ActionState, error) {
	h.keeper.Logger().Info("Validating Sense action finalization",
		"action_id", action.ActionID,
		"supernode", superNodeAccount,
		"current_state", action.State.String(),
		"previous supernodes_count", len(action.SuperNodes))

	if action.GetState() == actionapi.ActionState_ACTION_STATE_PENDING {
		action.State = actionapi.ActionState_ACTION_STATE_PROCESSING
	}

	// Verify Registration Metadata exists
	var existingSenseMeta actionapi.SenseMetadata
	if len(action.Metadata) > 0 {
		if err := proto.Unmarshal(action.Metadata, &existingSenseMeta); err != nil {
			return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
				errors.Wrap(sdkerrors.ErrInvalidRequest, fmt.Sprintf("failed to unmarshal existing sense metadata: %v", err))
		}
	}
	if existingSenseMeta.DataHash == "" {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "data_hash is required in existing metadata")
	}
	if existingSenseMeta.DdAndFingerprintsIc == 0 {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "dd_and_fingerprints_ic is required in existing metadata")
	}
	if existingSenseMeta.DdAndFingerprintsMax == 0 {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "dd_and_fingerprints_max is required in existing metadata")
	}

	// Parse the incoming metadata
	var newSenseMeta actionapi.SenseMetadata
	if err := proto.Unmarshal(metadataBytes, &newSenseMeta); err != nil {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal sense metadata: %v", err))
	}

	// 1. Verify supernode signature is included in the signatures list
	h.keeper.Logger().Debug("Verifying supernode signature for Sense action",
		"action_id", action.ActionID,
		"signature_present", len(newSenseMeta.Signatures) > 0)

	// Validate Signature. Signature field contains: `Base64(dd_and_fp__ids).sn1_signature.sn2_signature.sn3_signature`
	// Where `creators_signature` is the signature of the creator over `Base64(dd_and_fp__ids)`
	signatureParts := strings.Split(newSenseMeta.Signatures, ".")
	if len(signatureParts) != 4 {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, "invalid signature format")
	}

	// Use VerifySignature from crypto.go to validate the signature in either
	// of signatureParts[1,2,3] over the data in signatureParts[0].
	dataToVerify := signatureParts[0]
	var verifyErr error
	for i := 1; i < 4; i++ {
		superNodeSignature := signatureParts[i] // the signature is Base64 encoded, VerifySignature will decode it

		// Verify that superNode's signature is valid for the fingerprint data
		verifyErr = h.keeper.VerifySignature(ctx, dataToVerify, superNodeSignature, superNodeAccount)
		if verifyErr == nil {
			break
		}
	}
	if verifyErr != nil {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED, verifyErr
	}

	// 2. Verify DdAndFingerprintsIds size
	if err := VerifyKademliaIDs(newSenseMeta.DdAndFingerprintsIds,
		newSenseMeta.Signatures,
		existingSenseMeta.DdAndFingerprintsIc,
		existingSenseMeta.DdAndFingerprintsMax); err != nil {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, fmt.Sprintf("failed to verify dd_and_pf_ids: %v", err))
	}

	existingSenseMeta.Signatures = newSenseMeta.Signatures
	updatedMetadataBytes, err := proto.Marshal(&existingSenseMeta)
	if err != nil {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types2.ErrInvalidMetadata, fmt.Sprintf("failed to marshal updated sense metadata: %v", err))
	}

	action.Metadata = updatedMetadataBytes
	h.keeper.Logger().Info("Finalized Sense action with single supernode",
		"action_id", action.ActionID,
		"supernode", superNodeAccount)

	return actionapi.ActionState_ACTION_STATE_DONE, nil
}

// ValidateApproval validates approval data for Sense actions
func (h SenseActionHandler) ValidateApproval(ctx sdk.Context, action *actionapi.Action) error {
	// Empty implementation - will be filled in later
	return nil
}

func (h SenseActionHandler) GetUpdatedMetadata(ctx sdk.Context, existingMetadata, newMetadata []byte) ([]byte, error) {
	// Empty implementation - will be filled in later
	return nil, nil
}
