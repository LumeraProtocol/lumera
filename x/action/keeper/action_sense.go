package keeper

import (
	"encoding/json"
	"fmt"
	"github.com/LumeraProtocol/lumera/x/action/common"
	"google.golang.org/protobuf/encoding/protojson"
	"reflect"
	"slices"
	"strings"

	"cosmossdk.io/errors"
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/types"
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
func (h SenseActionHandler) Process(metadataBytes []byte, msgType common.MessageType, params *types.Params) ([]byte, error) {
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
		return errors.Wrap(types.ErrInvalidMetadata, "metadata is required for cascade actions")
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
			errors.Wrap(types.ErrInvalidMetadata, "data_hash is required in existing metadata")
	}
	if existingSenseMeta.DdAndFingerprintsIc == 0 {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types.ErrInvalidMetadata, "dd_and_fingerprints_ic is required in existing metadata")
	}
	if existingSenseMeta.DdAndFingerprintsMax == 0 {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types.ErrInvalidMetadata, "dd_and_fingerprints_max is required in existing metadata")
	}

	// Parse the incoming metadata
	var newSenseMeta actionapi.SenseMetadata
	if err := proto.Unmarshal(metadataBytes, &newSenseMeta); err != nil {
		return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
			errors.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("failed to unmarshal sense metadata: %v", err))
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
			errors.Wrap(types.ErrInvalidMetadata, "invalid signature format")
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
			errors.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("failed to verify dd_and_pf_ids: %v", err))
	}

	// 3. State management based on number of supernodes
	// We need to track fingerprint submissions from each supernode
	// Get existing metadata from the action's embedded metadata
	// TODO: MAYBE for Sense too we can just require single SN Finalize request - it will include all 3 signatures anyway

	// First, determine state transitions
	if action.State == actionapi.ActionState_ACTION_STATE_PENDING {
		// First supernode - transition to PROCESSING
		// Initialize the map if not already present
		if existingSenseMeta.SupernodeFingerprints == nil {
			existingSenseMeta.SupernodeFingerprints = make(map[string]string)
		}

		// Update the map with the current values (this will be stored by the keeper)
		existingSenseMeta.SupernodeFingerprints[superNodeAccount] = strings.Join(newSenseMeta.DdAndFingerprintsIds, "")

		// Marshal the updated metadata so the keeper can store it
		updatedMetadataBytes, err := proto.Marshal(&existingSenseMeta)
		if err != nil {
			return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
				errors.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("failed to marshal updated sense metadata: %v", err))
		}

		// Set the updated metadata (will be stored by the keeper)
		action.Metadata = updatedMetadataBytes

		// Return PROCESSING state to indicate state change needed
		return actionapi.ActionState_ACTION_STATE_PROCESSING, nil
	} else {
		// Already processing, update the map with current supernode
		if existingSenseMeta.SupernodeFingerprints == nil {
			return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
				errors.Wrap(types.ErrInvalidMetadata, "supernode_fingerprints is required in existing metadata")
		}

		// Update the map
		existingSenseMeta.SupernodeFingerprints[superNodeAccount] = strings.Join(newSenseMeta.DdAndFingerprintsIds, "")

		// Check if we have submissions from 3 different supernodes
		if len(existingSenseMeta.SupernodeFingerprints) >= 3 {
			// Validate that all sets match
			if ok, badSNs := compareFingerprints(existingSenseMeta.SupernodeFingerprints); ok {
				h.keeper.Logger().Info("All 3 supernodes have submitted matching data - finalizing Sense action",
					"action_id", action.ActionID)

				// Update signature with the latest submission
				existingSenseMeta.Signatures = newSenseMeta.Signatures

				// Clear up the map
				for f := range existingSenseMeta.SupernodeFingerprints {
					delete(existingSenseMeta.SupernodeFingerprints, f)
				}

				// Marshal the updated metadata
				updatedMetadataBytes, err := proto.Marshal(&existingSenseMeta)
				if err != nil {
					return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
						errors.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("failed to marshal updated sense metadata: %v", err))
				}

				// Set the updated metadata (will be stored by the keeper)
				action.Metadata = updatedMetadataBytes

				// Return DONE state to indicate completion
				return actionapi.ActionState_ACTION_STATE_DONE, nil
			} else {
				// TODO: add evidence processing for mismatched fingerprints

				h.keeper.Logger().Error("Supernode submissions do not match",
					"action_id", action.ActionID)

				// There can be only 2 bad choices:
				// 1. all 3 supernodes are different
				var actionState actionapi.ActionState
				if len(badSNs) == 3 {
					// all is really bad
					h.keeper.Logger().Error("All supernodes have provided different fingerprints",
						"action_id", action.ActionID,
						"bad_supernodes", badSNs)
					actionState = actionapi.ActionState_ACTION_STATE_FAILED
				} else {
					// We still can wait for one more good SN
					h.keeper.Logger().Info("Waiting for more supernodes to submit matching fingerprints",
						"action_id", action.ActionID,
						"bad_supernodes", badSNs,
						"remaining_supernodes", len(existingSenseMeta.SupernodeFingerprints)-len(badSNs))
					actionState = actionapi.ActionState_ACTION_STATE_PROCESSING

					for i, sn := range action.SuperNodes {
						if slices.Contains(badSNs, sn) {
							action.SuperNodes[i] = fmt.Sprintf("%s (bad)", sn)
						}
					}
				}

				// Remove the bad supernodes from the map
				for _, badSN := range badSNs {
					delete(existingSenseMeta.SupernodeFingerprints, badSN)
				}

				// Update the metadata
				updatedMetadataBytes, err := proto.Marshal(&existingSenseMeta)
				if err != nil {
					return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
						errors.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("failed to marshal updated sense metadata: %v", err))
				}

				// Set the updated metadata (will be stored by the keeper)
				action.Metadata = updatedMetadataBytes

				return actionState, nil
			}
		} else {
			// Not enough supernodes yet, just update the metadata
			updatedMetadataBytes, err := proto.Marshal(&existingSenseMeta)
			if err != nil {
				return actionapi.ActionState_ACTION_STATE_UNSPECIFIED,
					errors.Wrap(types.ErrInvalidMetadata, fmt.Sprintf("failed to marshal updated sense metadata: %v", err))
			}

			// Set the updated metadata (will be stored by the keeper)
			action.Metadata = updatedMetadataBytes

			// No state change
			return actionapi.ActionState_ACTION_STATE_PROCESSING, nil
		}
	}
}

// ValidateApproval validates approval data for Sense actions
func (h SenseActionHandler) ValidateApproval(ctx sdk.Context, action *actionapi.Action) error {
	// Empty implementation - will be filled in later
	return nil
}

func compareFingerprints(m map[string]string) (bool, []string) {
	if len(m) < 2 {
		return false, nil // Not enough data to compare
	}

	// Count appearances of each fingerprint
	counts := make(map[string]int)
	for _, fingerprint := range m {
		counts[fingerprint]++
	}

	// Determine the maximum count
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	// Find all fingerprints sharing that maximum count
	var mostFrequent []string
	for fingerprint, c := range counts {
		if c == maxCount {
			mostFrequent = append(mostFrequent, fingerprint)
		}
	}

	var badSuperNodes []string

	// If all are different, maxCount would be 1 (each fingerprint appears once)
	// => all must be marked as bad
	if maxCount == 1 {
		for node := range m {
			badSuperNodes = append(badSuperNodes, node)
		}
		return false, badSuperNodes
	}

	// If there is a tie among multiple fingerprints for maxCount,
	// we do not have a single majority => mark everyone as bad
	if len(mostFrequent) > 1 {
		for node := range m {
			badSuperNodes = append(badSuperNodes, node)
		}
		return false, badSuperNodes
	}

	// Otherwise, we have a single majority fingerprint
	majority := mostFrequent[0]
	for node, fingerprint := range m {
		if fingerprint != majority {
			badSuperNodes = append(badSuperNodes, node)
		}
	}

	// Return whether all have the same fingerprint (badSuperNodes empty) and the slice of bad nodes
	return len(badSuperNodes) == 0, badSuperNodes
}
