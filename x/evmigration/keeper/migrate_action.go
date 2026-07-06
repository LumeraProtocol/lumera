package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// MigrateActions updates action records where legacyAddr is the creator or is
// listed in the SuperNodes field (which stores AccAddress, not ValAddress).
//
// Rather than scanning the entire action store, it resolves the affected
// actions through the creator and supernode secondary indexes, so cost scales
// with the number of actions this address touches, not the global action count.
func (k Keeper) MigrateActions(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	legacyStr := legacyAddr.String()
	newStr := newAddr.String()

	byCreator, err := k.actionKeeper.GetActionsByCreator(ctx, legacyStr)
	if err != nil {
		return err
	}
	bySuperNode, err := k.actionKeeper.GetActionsBySuperNode(ctx, legacyStr)
	if err != nil {
		return err
	}

	// An action may reference the legacy address as both creator and supernode,
	// and each index lookup returns an independent copy. Dedupe by action ID so
	// the record is written exactly once, preserving encounter order.
	seen := make(map[string]*actiontypes.Action, len(byCreator)+len(bySuperNode))
	order := make([]string, 0, len(byCreator)+len(bySuperNode))
	for _, action := range byCreator {
		if _, ok := seen[action.ActionID]; !ok {
			seen[action.ActionID] = action
			order = append(order, action.ActionID)
		}
	}
	for _, action := range bySuperNode {
		if _, ok := seen[action.ActionID]; !ok {
			seen[action.ActionID] = action
			order = append(order, action.ActionID)
		}
	}

	for _, id := range order {
		action := seen[id]
		if action.Creator == legacyStr {
			action.Creator = newStr
		}
		for i, sn := range action.SuperNodes {
			if sn == legacyStr {
				action.SuperNodes[i] = newStr
			}
		}
		if err := k.actionKeeper.SetAction(ctx, action); err != nil {
			return err
		}
	}

	return nil
}
