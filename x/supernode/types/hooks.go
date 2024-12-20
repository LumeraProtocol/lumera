package types

import (
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// StakingHooksWrapper wrapper type for staking hooks
type StakingHooksWrapper struct {
	hooks stakingtypes.StakingHooks
}

// NewStakingHooksWrapper creates a new wrapper
func NewStakingHooksWrapper(hooks stakingtypes.StakingHooks) StakingHooksWrapper {
	return StakingHooksWrapper{
		hooks: hooks,
	}
}

// IsNil checks if the hooks are nil
func (w StakingHooksWrapper) IsNil() bool {
	return w.hooks == nil
}

// Hooks returns the underlying hooks
func (w StakingHooksWrapper) Hooks() stakingtypes.StakingHooks {
	return w.hooks
}
