package keeper

import (
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ActionRegistrar defines the required methods for registering actions
type ActionRegistrar interface {
	// RegisterAction creates and configures a new action with default parameters
	RegisterAction(ctx sdk.Context, action *actiontypes.Action) (string, error)
}

// ActionFinalizer defines the required methods for finalizing actions
type ActionFinalizer interface {
	// FinalizeAction updates an action's state to DONE, validates metadata,
	// and handles fee distribution
	FinalizeAction(ctx sdk.Context, actionID string, superNode string, metadata []byte) error
}

// ActionApprover defines the required methods for approving actions
type ActionApprover interface {
	// ApproveAction updates an action's state to APPROVED and validates the creator
	ApproveAction(ctx sdk.Context, actionID string, creator string) error
}

// Ensure Keeper implements these interfaces by placing proper validation at compile time
var _ ActionRegistrar = (*Keeper)(nil)
var _ ActionFinalizer = (*Keeper)(nil)
var _ ActionApprover = (*Keeper)(nil)
