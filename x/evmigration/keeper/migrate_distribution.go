package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateDistribution withdraws all pending delegation rewards for legacyAddr,
// materializing them into the legacy bank balance before balances are moved.
func (k Keeper) MigrateDistribution(ctx sdk.Context, legacyAddr sdk.AccAddress) error {
	// Get all delegations for the legacy address.
	delegations, err := k.stakingKeeper.GetDelegatorDelegations(ctx, legacyAddr, ^uint16(0))
	if err != nil {
		return err
	}

	// Withdraw rewards for each delegation.
	for _, del := range delegations {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			return err
		}
		// WithdrawDelegationRewards sends rewards to the legacy bank balance.
		// Ignoring the returned coins — they're now in the bank balance.
		if _, err := k.distributionKeeper.WithdrawDelegationRewards(ctx, legacyAddr, valAddr); err != nil {
			return err
		}
	}

	return nil
}
