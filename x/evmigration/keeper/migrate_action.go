package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// MigrateActions updates action records where legacyAddr is the creator or
// is listed in the SuperNodes field (which stores AccAddress, not ValAddress).
func (k Keeper) MigrateActions(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	legacyStr := legacyAddr.String()
	newStr := newAddr.String()

	var toUpdate []*actiontypes.Action

	err := k.actionKeeper.IterateActions(ctx, func(action *actiontypes.Action) bool {
		modified := false

		if action.Creator == legacyStr {
			action.Creator = newStr
			modified = true
		}

		for i, sn := range action.SuperNodes {
			if sn == legacyStr {
				action.SuperNodes[i] = newStr
				modified = true
			}
		}

		if modified {
			toUpdate = append(toUpdate, action)
		}
		return false
	})
	if err != nil {
		return err
	}

	for _, action := range toUpdate {
		if err := k.actionKeeper.SetAction(ctx, action); err != nil {
			return err
		}
	}

	return nil
}
