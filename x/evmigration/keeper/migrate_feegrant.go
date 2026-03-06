package keeper

import (
	"cosmossdk.io/x/feegrant"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateFeegrant re-keys fee allowances where legacyAddr is granter or grantee.
// Since feegrant.Keeper.revokeAllowance is unexported, we re-create the allowance
// at the new address via GrantAllowance. The old allowance entry becomes orphaned
// (referencing a removed legacy account) but is harmless.
func (k Keeper) MigrateFeegrant(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	type allowanceToMigrate struct {
		granter sdk.AccAddress
		grantee sdk.AccAddress
		grant   feegrant.Grant
	}

	var toMigrate []allowanceToMigrate

	err := k.feegrantKeeper.IterateAllFeeAllowances(ctx, func(grant feegrant.Grant) bool {
		granterAddr, err := sdk.AccAddressFromBech32(grant.Granter)
		if err != nil {
			return false
		}
		granteeAddr, err := sdk.AccAddressFromBech32(grant.Grantee)
		if err != nil {
			return false
		}

		if granterAddr.Equals(legacyAddr) || granteeAddr.Equals(legacyAddr) {
			toMigrate = append(toMigrate, allowanceToMigrate{
				granter: granterAddr,
				grantee: granteeAddr,
				grant:   grant,
			})
		}
		return false
	})
	if err != nil {
		return err
	}

	for _, a := range toMigrate {
		allowance, err := a.grant.GetGrant()
		if err != nil {
			return err
		}

		// Compute new granter/grantee.
		newGranter := a.granter
		if newGranter.Equals(legacyAddr) {
			newGranter = newAddr
		}
		newGrantee := a.grantee
		if newGrantee.Equals(legacyAddr) {
			newGrantee = newAddr
		}

		// Re-create the allowance at new addresses.
		if err := k.feegrantKeeper.GrantAllowance(ctx, newGranter, newGrantee, allowance); err != nil {
			return err
		}
	}

	return nil
}
