package keeper

import (
	"context"
)

// BeginBlocker contains logic that runs at the beginning of each block.
// Currently the supernode module has no begin-block behavior.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	return nil
}

// EndBlocker contains logic that runs at the end of each block.
// It delegates to HandleMetricsStaleness, which may transition ACTIVE
// supernodes into POSTPONED when they fail to report metrics on time.
func (k Keeper) EndBlocker(ctx context.Context) error {
	// Metrics staleness enforcement is handled by the audit module.
	return nil
}
