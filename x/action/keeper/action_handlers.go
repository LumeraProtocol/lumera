package keeper

import (
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
)

// InitializeActionRegistry sets up the action registry with default handlers
func (k *Keeper) InitializeActionRegistry() *ActionRegistry {
	registry := NewActionRegistry(k)

	// Register handlers for existing action types
	registry.RegisterHandler(actionapi.ActionType_ACTION_TYPE_SENSE, NewSenseActionHandler(k))
	registry.RegisterHandler(actionapi.ActionType_ACTION_TYPE_CASCADE, NewCascadeActionHandler(k))

	return registry
}
