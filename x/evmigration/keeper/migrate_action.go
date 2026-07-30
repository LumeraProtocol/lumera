package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

type actionIndexRefs struct {
	creator   bool
	supernode bool
}

// MigrateActions preserves action identity continuity without rewriting historical
// participants. Pending and processing actions remain fully actionable by the new
// account, while done actions only move their creator (the party that can still
// need creator-scoped access). Terminal actions are immutable history.
func (k Keeper) MigrateActions(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	legacyStr := legacyAddr.String()
	newStr := newAddr.String()

	byCreator, err := k.actionKeeper.GetActionsByCreator(ctx, legacyStr)
	if err != nil {
		return fmt.Errorf("get actions by creator %s: %w", legacyStr, err)
	}
	bySuperNode, err := k.actionKeeper.GetActionsBySuperNode(ctx, legacyStr)
	if err != nil {
		return fmt.Errorf("get actions by supernode %s: %w", legacyStr, err)
	}

	// Keep the index encounter order deterministic, but use index values only to
	// discover IDs. The primary action store is the sole canonical data source.
	refs := make(map[string]actionIndexRefs, len(byCreator)+len(bySuperNode))
	order := make([]string, 0, len(byCreator)+len(bySuperNode))
	addIndexRows := func(rows []*actiontypes.Action, creatorIndex bool) error {
		for i, indexed := range rows {
			if indexed == nil {
				return fmt.Errorf("malformed action index row %d: nil action", i)
			}
			if indexed.ActionID == "" {
				return fmt.Errorf("malformed action index row %d: empty action ID", i)
			}
			if creatorIndex {
				if indexed.Creator != legacyStr {
					return fmt.Errorf("stale creator index for action %s", indexed.ActionID)
				}
			} else if countAddress(indexed.SuperNodes, legacyStr) == 0 {
				return fmt.Errorf("stale supernode index for action %s", indexed.ActionID)
			}

			ref, exists := refs[indexed.ActionID]
			if !exists {
				order = append(order, indexed.ActionID)
			}
			if creatorIndex {
				ref.creator = true
			} else {
				ref.supernode = true
			}
			refs[indexed.ActionID] = ref
		}
		return nil
	}
	if err := addIndexRows(byCreator, true); err != nil {
		return err
	}
	if err := addIndexRows(bySuperNode, false); err != nil {
		return err
	}

	// Resolve and validate every canonical row before writing any of them. This
	// makes index corruption and destination collisions fail closed.
	updates := make([]*actiontypes.Action, 0, len(order))
	for _, id := range order {
		canonical, found := k.actionKeeper.GetActionByID(ctx, id)
		if !found || canonical == nil {
			return fmt.Errorf("canonical action %s referenced by index not found", id)
		}
		if canonical.ActionID == "" || canonical.ActionID != id {
			return fmt.Errorf("malformed canonical action for index ID %s: got %q", id, canonical.ActionID)
		}

		ref := refs[id]
		legacySNCount := countAddress(canonical.SuperNodes, legacyStr)
		if ref.creator && canonical.Creator != legacyStr {
			return fmt.Errorf("stale creator index conflicts with canonical action %s", id)
		}
		if ref.supernode && legacySNCount == 0 {
			return fmt.Errorf("stale supernode index conflicts with canonical action %s", id)
		}

		switch canonical.State {
		case actiontypes.ActionStatePending, actiontypes.ActionStateProcessing:
			if legacySNCount > 1 {
				return fmt.Errorf("action %s has duplicate legacy supernode entries", id)
			}
			if legacySNCount == 1 && countAddress(canonical.SuperNodes, newStr) != 0 {
				return fmt.Errorf("action %s already contains destination supernode", id)
			}
			updated := cloneAction(canonical)
			changed := false
			if updated.Creator == legacyStr {
				updated.Creator = newStr
				changed = true
			}
			for i, supernode := range updated.SuperNodes {
				if supernode == legacyStr {
					updated.SuperNodes[i] = newStr
					changed = true
				}
			}
			if !changed {
				return fmt.Errorf("live action %s index does not reference legacy address", id)
			}
			updates = append(updates, updated)

		case actiontypes.ActionStateDone:
			if canonical.Creator == legacyStr {
				updated := cloneAction(canonical)
				updated.Creator = newStr
				updates = append(updates, updated)
			}

		case actiontypes.ActionStateApproved, actiontypes.ActionStateRejected,
			actiontypes.ActionStateFailed, actiontypes.ActionStateExpired,
			actiontypes.ActionStateUnspecified:
			// Immutable historical record.

		default:
			return fmt.Errorf("action %s has malformed state %d", id, canonical.State)
		}
	}

	// A direct helper call must be as atomic as a message execution. SetAction
	// updates several indexes, so discard the cache if any later write fails.
	cacheCtx, commit := ctx.CacheContext()
	for _, action := range updates {
		if err := k.actionKeeper.SetAction(cacheCtx, action); err != nil {
			return fmt.Errorf("set action %s: %w", action.ActionID, err)
		}
	}
	commit()
	return nil
}

func countAddress(addresses []string, target string) int {
	count := 0
	for _, address := range addresses {
		if address == target {
			count++
		}
	}
	return count
}

func cloneAction(action *actiontypes.Action) *actiontypes.Action {
	clone := *action
	clone.Metadata = append([]byte(nil), action.Metadata...)
	clone.AppPubkey = append([]byte(nil), action.AppPubkey...)
	clone.SuperNodes = append([]string(nil), action.SuperNodes...)
	return &clone
}
