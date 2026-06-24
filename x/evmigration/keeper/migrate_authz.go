package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
)

// MigrateAuthz re-keys all authz grants where legacyAddr is granter or grantee.
func (k Keeper) MigrateAuthz(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	type grantToMigrate struct {
		granter sdk.AccAddress
		grantee sdk.AccAddress
		grant   authz.Grant
	}

	var toMigrate []grantToMigrate

	// Collect all grants involving legacyAddr.
	k.authzKeeper.IterateGrants(ctx, func(granterAddr, granteeAddr sdk.AccAddress, grant authz.Grant) bool {
		if granterAddr.Equals(legacyAddr) || granteeAddr.Equals(legacyAddr) {
			toMigrate = append(toMigrate, grantToMigrate{
				granter: granterAddr,
				grantee: granteeAddr,
				grant:   grant,
			})
		}
		return false
	})

	for _, g := range toMigrate {
		auth, err := g.grant.GetAuthorization()
		if err != nil {
			return err
		}
		msgType := auth.MsgTypeURL()

		// Delete old grant.
		if err := k.authzKeeper.DeleteGrant(ctx, g.grantee, g.granter, msgType); err != nil {
			return err
		}

		// Compute new granter/grantee.
		newGranter := g.granter
		if newGranter.Equals(legacyAddr) {
			newGranter = newAddr
		}
		newGrantee := g.grantee
		if newGrantee.Equals(legacyAddr) {
			newGrantee = newAddr
		}

		// Re-create grant with new addresses.
		if err := k.authzKeeper.SaveGrant(ctx, newGrantee, newGranter, auth, g.grant.Expiration); err != nil {
			return err
		}
	}

	return nil
}
