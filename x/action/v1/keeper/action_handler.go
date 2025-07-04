package keeper

import (
	"reflect"

	"cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/action/v1/common"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ActionHandler defines the interface for processing action-specific operations
type ActionHandler interface {
	// Process validates and performs any necessary transformations on the action data
	Process(metadataBytes []byte, msgType common.MessageType, params *actiontypes.Params) ([]byte, error)

	// GetProtoMessageType returns the reflect.Type of the protobuf message for this action
	GetProtoMessageType() reflect.Type

	// ConvertJSONToProtobuf converts JSON data to protobuf binary format
	ConvertJSONToProtobuf(jsonData []byte) ([]byte, error)

	// ConvertProtobufToJSON converts protobuf binary data to JSON format
	ConvertProtobufToJSON(protobufData []byte) ([]byte, error)

	// RegisterAction handles action-specific validation during registration
	RegisterAction(ctx sdk.Context, action *actiontypes.Action) error

	// FinalizeAction validates action-specific finalization data
	// Returns action state updates (or ActionStateUnspecified if no change)
	FinalizeAction(ctx sdk.Context, existingAction *actiontypes.Action, superNodeAccount string, newMetadataBytes []byte) (actiontypes.ActionState, error)

	// ValidateApproval validates action-specific approval data
	ValidateApproval(ctx sdk.Context, action *actiontypes.Action) error

	// GetUpdatedMetadata returns the updated metadata on finalize action
	GetUpdatedMetadata(ctx sdk.Context, existingMetadata, newMetadata []byte) ([]byte, error) 
}

// ActionRegistry maintains a registry of handlers for different action types
type ActionRegistry struct {
	handlers map[actiontypes.ActionType]ActionHandler
	keeper   *Keeper // Reference to the keeper for logger and other services
}

// NewActionRegistry creates a new action registry
func NewActionRegistry(k *Keeper) *ActionRegistry {
	return &ActionRegistry{
		handlers: make(map[actiontypes.ActionType]ActionHandler),
		keeper:   k,
	}
}

// RegisterHandler registers a handler for a specific action type
func (r *ActionRegistry) RegisterHandler(actionType actiontypes.ActionType, handler ActionHandler) {
	r.handlers[actionType] = handler
}

// GetHandler retrieves the handler for a specific action type
func (r *ActionRegistry) GetHandler(actionType actiontypes.ActionType) (ActionHandler, error) {
	handler, ok := r.handlers[actionType]
	if !ok {
		return nil, ErrNoHandlerForActionType(actionType)
	}

	return handler, nil
}

// ErrNoHandlerForActionType returns a formatted error for missing handlers
func ErrNoHandlerForActionType(actionType actiontypes.ActionType) error {
	return errors.Wrapf(
		actiontypes.ErrInvalidActionType,
		"no handler registered for action type %s",
		actionType.String(),
	)
}
