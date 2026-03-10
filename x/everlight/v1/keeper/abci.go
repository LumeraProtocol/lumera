package keeper

import "context"

// BeginBlocker contains logic that runs at the beginning of each block.
func (k Keeper) BeginBlocker(_ context.Context) error {
	return nil
}

// EndBlocker contains logic that runs at the end of each block.
// Distribution logic will be added in S13.
func (k Keeper) EndBlocker(_ context.Context) error {
	return nil
}
