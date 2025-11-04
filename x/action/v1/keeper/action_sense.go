package keeper

import (
	"fmt"
	"reflect"
	"strings"
	"bytes"

	"github.com/LumeraProtocol/lumera/x/action/v1/common"

	"cosmossdk.io/errors"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/gogoproto/jsonpb"
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
func (h SenseActionHandler) Process(metadataBytes []byte, msgType common.MessageType, params *actiontypes.Params) ([]byte, error) {
	var metadata actiontypes.SenseMetadata
	// Unmarshal JSON to SenseMetadata struct
	unmarshaller := &jsonpb.Unmarshaler{}
	if err := unmarshaller.Unmarshal(strings.NewReader(string(metadataBytes)), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sense metadata from JSON: %w", err)
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
	return gogoproto.Marshal(&metadata)
}

// GetProtoMessageType returns the reflect.Type for SenseMetadata
func (h SenseActionHandler) GetProtoMessageType() reflect.Type {
	return reflect.TypeOf(actiontypes.SenseMetadata{})
}

// ConvertJSONToProtobuf converts JSON metadata to protobuf binary format
func (h SenseActionHandler) ConvertJSONToProtobuf(jsonData []byte) ([]byte, error) {
	var metadata actiontypes.SenseMetadata
	// Unmarshal JSON to SenseMetadata struct
	unmarshaller := &jsonpb.Unmarshaler{}
	if err := unmarshaller.Unmarshal(strings.NewReader(string(jsonData)), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sense metadata from JSON: %w", err)
	}

	// Marshal to protobuf binary format
	return gogoproto.Marshal(&metadata)
}

// ConvertProtobufToJSON converts protobuf binary metadata to JSON format
func (h SenseActionHandler) ConvertProtobufToJSON(protobufData []byte) ([]byte, error) {
	var metadata actiontypes.SenseMetadata
	if err := gogoproto.Unmarshal(protobufData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sense metadata from protobuf: %w", err)
	}

	// Marshal to JSON format
	marshaler := &jsonpb.Marshaler{
		EmitDefaults: true,
		EnumsAsInts: true,
	}
	var buf bytes.Buffer
	if err := marshaler.Marshal(&buf, &metadata); err != nil {
		return nil, fmt.Errorf("failed to marshal cascade metadata to JSON: %w", err)
	}
	return buf.Bytes(), nil
}

// RegisterAction handles action-specific validation and processing during action registration
func (h SenseActionHandler) RegisterAction(_ sdk.Context, action *actiontypes.Action) error {
	if action.Metadata == nil {
		return errors.Wrap(actiontypes.ErrInvalidMetadata, "metadata is required for cascade actions")
	}
	// There is nothing else to do for Sense actions during registration
	//var newSenseMeta actiontypes.SenseMetadata
	//if err := proto.Unmarshal(action.Metadata, &newSenseMeta); err != nil {
	//	return errors.Wrap(actiontypes.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal sense metadata: %v", err))
	//}

	return nil
}

// FinalizeAction validates finalization data for Sense actions
// Returns the recommended action state or ActionStateUnspecified if no change
func (h SenseActionHandler) FinalizeAction(ctx sdk.Context, action *actiontypes.Action, superNodeAccount string, metadataBytes []byte) (actiontypes.ActionState, error) {
	h.keeper.Logger().Info("Validating Sense action finalization",
		"action_id", action.ActionID,
		"supernode", superNodeAccount,
		"current_state", action.State.String(),
		"previous supernodes_count", len(action.SuperNodes))

	if action.GetState() == actiontypes.ActionStatePending {
		action.State = actiontypes.ActionStateProcessing
	}

	// Verify Registration Metadata exists
	var existingSenseMeta actiontypes.SenseMetadata
	if len(action.Metadata) > 0 {
		if err := gogoproto.Unmarshal(action.Metadata, &existingSenseMeta); err != nil {
			return actiontypes.ActionStateUnspecified,
				errors.Wrap(sdkerrors.ErrInvalidRequest, fmt.Sprintf("failed to unmarshal existing sense metadata: %v", err))
		}
	}
	if existingSenseMeta.DataHash == "" {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "data_hash is required in existing metadata")
	}
	if existingSenseMeta.DdAndFingerprintsIc == 0 {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "dd_and_fingerprints_ic is required in existing metadata")
	}
	if existingSenseMeta.DdAndFingerprintsMax == 0 {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "dd_and_fingerprints_max is required in existing metadata")
	}

	// Parse the incoming metadata
	var newSenseMeta actiontypes.SenseMetadata
	if err := gogoproto.Unmarshal(metadataBytes, &newSenseMeta); err != nil {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal sense metadata: %v", err))
	}

	// 1. Verify supernode signature is included in the signatures list
	h.keeper.Logger().Debug("Verifying supernode signature for Sense action",
		"action_id", action.ActionID,
		"signature_present", len(newSenseMeta.Signatures) > 0)

	// Validate Signature. Signature field contains: `Base64(dd_and_fp__ids).sn1_signature.sn2_signature.sn3_signature`
	// Where `creators_signature` is the signature of the creator over `Base64(dd_and_fp__ids)`
	signatureParts := strings.Split(newSenseMeta.Signatures, ".")
	if len(signatureParts) != 4 {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, "invalid signature format")
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
		return actiontypes.ActionStateUnspecified, verifyErr
	}

	// 2. Verify DdAndFingerprintsIds size
	if err := VerifyKademliaIDs(newSenseMeta.DdAndFingerprintsIds,
		newSenseMeta.Signatures,
		existingSenseMeta.DdAndFingerprintsIc,
		existingSenseMeta.DdAndFingerprintsMax); err != nil {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, fmt.Sprintf("failed to verify dd_and_pf_ids: %v", err))
	}

	existingSenseMeta.Signatures = newSenseMeta.Signatures
	updatedMetadataBytes, err := gogoproto.Marshal(&existingSenseMeta)
	if err != nil {
		return actiontypes.ActionStateUnspecified,
			errors.Wrap(actiontypes.ErrInvalidMetadata, fmt.Sprintf("failed to marshal updated sense metadata: %v", err))
	}

	action.Metadata = updatedMetadataBytes
	h.keeper.Logger().Info("Finalized Sense action with single supernode",
		"action_id", action.ActionID,
		"supernode", superNodeAccount)

	return actiontypes.ActionStateDone, nil
}

// ValidateApproval validates approval data for Sense actions
func (h SenseActionHandler) ValidateApproval(ctx sdk.Context, action *actiontypes.Action) error {
	// Empty implementation - will be filled in later
	return nil
}

func (h SenseActionHandler) GetUpdatedMetadata(ctx sdk.Context, existingMetadata, newMetadata []byte) ([]byte, error) {
	// Empty implementation - will be filled in later
	return nil, nil
}
