package keeper

import (
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// InitializeActionRegistry sets up the action registry with default handlers
func (k *Keeper) InitializeActionRegistry() *ActionRegistry {
	registry := NewActionRegistry(k)

	// Register handlers for existing action types
	registry.RegisterHandler(actiontypes.ActionTypeSense, NewSenseActionHandler(k))
	registry.RegisterHandler(actiontypes.ActionTypeCascade, NewCascadeActionHandler(k))

	return registry
}
