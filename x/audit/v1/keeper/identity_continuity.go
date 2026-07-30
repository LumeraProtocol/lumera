package keeper

import (
	"bytes"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

type accountSingletonMove struct {
	sourceKey      []byte
	destinationKey []byte
	sourceVal      []byte
	destinationVal []byte
}

// AccountTransitionPlan is an immutable snapshot of every Audit write needed
// for one identity transition. Its fields are private so callers can pass a
// plan to Apply but cannot alter discovered fragments.
type AccountTransitionPlan struct {
	transition      types.AccountTransition
	transitionBytes []byte
	singletons      []accountSingletonMove
	transitionReads []accountSingletonMove
	transitionCount int
}

// BuildCurrentAccountTransitionPlan derives the transition boundary from
// Audit's own epoch configuration. Cross-module migration callers must not
// duplicate or expose Audit's height-to-epoch rules.
func (k Keeper) BuildCurrentAccountTransitionPlan(ctx sdk.Context, source, destination string) (AccountTransitionPlan, error) {
	params := k.GetParams(ctx).WithDefaults()
	currentEpoch, err := deriveEpochAtHeight(ctx.BlockHeight(), params)
	if err != nil {
		return AccountTransitionPlan{}, err
	}
	return k.BuildAccountTransitionPlan(ctx, types.AccountTransition{
		SourceAccount:      source,
		DestinationAccount: destination,
		EffectiveEpoch:     currentEpoch.EpochID + 1,
	})
}

// RecordAccountTransition builds and applies one durable lineage edge. It is a
// convenience for callers that do not need to aggregate this plan with plans
// from other modules.
func (k Keeper) RecordAccountTransition(ctx sdk.Context, transition types.AccountTransition) error {
	plan, err := k.BuildAccountTransitionPlan(ctx, transition)
	if err != nil {
		return err
	}
	return k.ApplyAccountTransitionPlan(ctx, plan)
}

// BuildAccountTransitionPlan performs all graph, collision, corruption and
// active-workflow checks without writing state.
func (k Keeper) BuildAccountTransitionPlan(ctx sdk.Context, transition types.AccountTransition) (AccountTransitionPlan, error) {
	if err := k.validateTransitionEndpoints(transition); err != nil {
		return AccountTransitionPlan{}, err
	}
	params := k.GetParams(ctx).WithDefaults()
	currentEpoch, err := deriveEpochAtHeight(ctx.BlockHeight(), params)
	if err != nil {
		return AccountTransitionPlan{}, err
	}
	if transition.EffectiveEpoch != currentEpoch.EpochID+1 {
		return AccountTransitionPlan{}, fmt.Errorf("account transition effective epoch must be current epoch + 1: got %d, want %d", transition.EffectiveEpoch, currentEpoch.EpochID+1)
	}
	store := k.kvStore(ctx)
	transitionCount, err := k.accountTransitionCount(ctx)
	if err != nil {
		return AccountTransitionPlan{}, err
	}
	if transitionCount >= types.MaxAccountTransitions {
		return AccountTransitionPlan{}, fmt.Errorf("account transitions exceed limit %d", types.MaxAccountTransitions)
	}
	if store.Has(types.AccountTransitionForwardKey(transition.SourceAccount)) {
		return AccountTransitionPlan{}, fmt.Errorf("account transition fork at %q", transition.SourceAccount)
	}
	if store.Has(types.AccountTransitionReverseKey(transition.DestinationAccount)) || store.Has(types.AccountTransitionForwardKey(transition.DestinationAccount)) {
		return AccountTransitionPlan{}, fmt.Errorf("account transition destination collision at %q", transition.DestinationAccount)
	}
	if previous, ok, err := k.reverseTransition(ctx, transition.SourceAccount); err != nil {
		return AccountTransitionPlan{}, err
	} else if ok && previous.EffectiveEpoch >= transition.EffectiveEpoch {
		return AccountTransitionPlan{}, fmt.Errorf("account transition epochs must strictly increase")
	}
	root, err := k.lineageRoot(ctx, transition.SourceAccount)
	if err != nil {
		return AccountTransitionPlan{}, err
	}
	if root == transition.DestinationAccount {
		return AccountTransitionPlan{}, fmt.Errorf("account transition cycle")
	}
	if err := k.accountTransitionWorkflowBlocker(ctx, transition.SourceAccount); err != nil {
		return AccountTransitionPlan{}, err
	}
	singletons, err := k.buildSingletonMoves(ctx, transition.SourceAccount, transition.DestinationAccount)
	if err != nil {
		return AccountTransitionPlan{}, err
	}
	bz, err := k.cdc.Marshal(&transition)
	if err != nil {
		return AccountTransitionPlan{}, err
	}
	transitionReads, err := k.snapshotAccountTransitionIndexes(ctx)
	if err != nil {
		return AccountTransitionPlan{}, err
	}
	return AccountTransitionPlan{
		transition:      transition,
		transitionBytes: append([]byte(nil), bz...),
		singletons:      singletons,
		transitionReads: transitionReads,
		transitionCount: transitionCount,
	}, nil
}

// ApplyAccountTransitionPlan consumes only frozen plan fragments and performs
// no discovery decisions after mutation begins.
func (k Keeper) ApplyAccountTransitionPlan(ctx sdk.Context, plan AccountTransitionPlan) error {
	if len(plan.transitionBytes) == 0 || plan.transition.SourceAccount == "" {
		return fmt.Errorf("invalid account transition plan")
	}
	store := k.kvStore(ctx)
	count, err := k.accountTransitionCount(ctx)
	if err != nil {
		return err
	}
	if count != plan.transitionCount ||
		store.Has(types.AccountTransitionForwardKey(plan.transition.SourceAccount)) ||
		store.Has(types.AccountTransitionReverseKey(plan.transition.DestinationAccount)) ||
		store.Has(types.AccountTransitionForwardKey(plan.transition.DestinationAccount)) {
		return fmt.Errorf("stale or already applied account transition plan")
	}
	if !accountTransitionIndexesMatchSnapshot(store, plan.transitionReads) {
		return fmt.Errorf("stale account transition index precondition")
	}
	for _, move := range plan.singletons {
		if !bytes.Equal(store.Get(move.sourceKey), move.sourceVal) || store.Has(move.destinationKey) {
			return fmt.Errorf("stale account transition singleton precondition")
		}
	}
	for _, move := range plan.singletons {
		if move.sourceVal == nil {
			continue
		}
		store.Set(move.destinationKey, move.destinationVal)
		store.Delete(move.sourceKey)
	}
	store.Set(types.AccountTransitionForwardKey(plan.transition.SourceAccount), plan.transitionBytes)
	store.Set(types.AccountTransitionReverseKey(plan.transition.DestinationAccount), plan.transitionBytes)
	return nil
}

// snapshotAccountTransitionIndexes freezes and validates both copies of every
// lineage edge. A migration fails closed if either index is orphaned, malformed,
// mismatched, or exceeds the consensus bound.
func (k Keeper) snapshotAccountTransitionIndexes(ctx sdk.Context) ([]accountSingletonMove, error) {
	store := k.kvStore(ctx)
	reads := make([]accountSingletonMove, 0)
	for _, spec := range []struct {
		prefix  []byte
		forward bool
	}{
		{types.AccountTransitionForwardPrefix(), true},
		{types.AccountTransitionReversePrefix(), false},
	} {
		it := store.Iterator(spec.prefix, storetypes.PrefixEndBytes(spec.prefix))
		count := 0
		for ; it.Valid(); it.Next() {
			count++
			if count > types.MaxAccountTransitions {
				_ = it.Close()
				return nil, fmt.Errorf("account transition indexes exceed limit %d", types.MaxAccountTransitions)
			}
			var transition types.AccountTransition
			if err := k.cdc.Unmarshal(it.Value(), &transition); err != nil {
				_ = it.Close()
				return nil, fmt.Errorf("malformed account transition index: %w", err)
			}
			if err := k.validateTransitionEndpoints(transition); err != nil {
				_ = it.Close()
				return nil, fmt.Errorf("malformed account transition index: %w", err)
			}
			suffix := string(it.Key()[len(spec.prefix):])
			mirrorKey := types.AccountTransitionReverseKey(transition.DestinationAccount)
			if spec.forward {
				if suffix != transition.SourceAccount {
					_ = it.Close()
					return nil, fmt.Errorf("malformed account transition: forward index key does not match source account")
				}
			} else {
				if suffix != transition.DestinationAccount {
					_ = it.Close()
					return nil, fmt.Errorf("malformed account transition: reverse index key does not match destination account")
				}
				mirrorKey = types.AccountTransitionForwardKey(transition.SourceAccount)
			}
			if !bytes.Equal(store.Get(mirrorKey), it.Value()) {
				_ = it.Close()
				return nil, fmt.Errorf("malformed account transition: forward/reverse indexes disagree")
			}
			reads = append(reads, accountSingletonMove{
				sourceKey: append([]byte(nil), it.Key()...),
				sourceVal: append([]byte(nil), it.Value()...),
			})
		}
		_ = it.Close()
	}
	return reads, nil
}

func accountTransitionIndexesMatchSnapshot(store storetypes.KVStore, reads []accountSingletonMove) bool {
	readIndex := 0
	for _, prefix := range [][]byte{types.AccountTransitionForwardPrefix(), types.AccountTransitionReversePrefix()} {
		it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
		for ; it.Valid(); it.Next() {
			if readIndex >= len(reads) ||
				!bytes.Equal(it.Key(), reads[readIndex].sourceKey) ||
				!bytes.Equal(it.Value(), reads[readIndex].sourceVal) {
				_ = it.Close()
				return false
			}
			readIndex++
		}
		_ = it.Close()
	}
	return readIndex == len(reads)
}

func (k Keeper) validateTransitionEndpoints(transition types.AccountTransition) error {
	if transition.SourceAccount == "" || transition.DestinationAccount == "" || transition.SourceAccount == transition.DestinationAccount {
		return fmt.Errorf("invalid account transition endpoints")
	}
	if transition.EffectiveEpoch == 0 {
		return fmt.Errorf("effective epoch must be non-zero")
	}
	sourceBytes, err := k.addressCodec.StringToBytes(transition.SourceAccount)
	if err != nil {
		return fmt.Errorf("malformed source account: %w", err)
	}
	canonicalSource, err := k.addressCodec.BytesToString(sourceBytes)
	if err != nil || canonicalSource != transition.SourceAccount {
		return fmt.Errorf("noncanonical source account")
	}
	destinationBytes, err := k.addressCodec.StringToBytes(transition.DestinationAccount)
	if err != nil {
		return fmt.Errorf("malformed destination account: %w", err)
	}
	canonicalDestination, err := k.addressCodec.BytesToString(destinationBytes)
	if err != nil || canonicalDestination != transition.DestinationAccount {
		return fmt.Errorf("noncanonical destination account")
	}
	return nil
}

// AccountForEpoch resolves the member of account's lineage that was logical at
// epoch. Boundary semantics are destination-at-effective_epoch.
func (k Keeper) AccountForEpoch(ctx sdk.Context, account string, epoch uint64) (string, error) {
	current, err := k.lineageRoot(ctx, account)
	if err != nil {
		return "", err
	}
	for hops := 0; ; hops++ {
		next, ok, err := k.forwardTransition(ctx, current)
		if err != nil {
			return "", err
		}
		if !ok || next.EffectiveEpoch > epoch {
			return current, nil
		}
		if hops >= types.MaxAccountTransitions {
			return "", fmt.Errorf("account lineage exceeds transition limit %d", types.MaxAccountTransitions)
		}
		current = next.DestinationAccount
	}
}

// CurrentAccount resolves the live endpoint of an account lineage.
func (k Keeper) CurrentAccount(ctx sdk.Context, account string) (string, error) {
	current, err := k.lineageRoot(ctx, account)
	if err != nil {
		return "", err
	}
	for hops := 0; ; hops++ {
		next, ok, err := k.forwardTransition(ctx, current)
		if err != nil {
			return "", err
		}
		if !ok {
			return current, nil
		}
		if hops >= types.MaxAccountTransitions {
			return "", fmt.Errorf("account lineage exceeds transition limit %d", types.MaxAccountTransitions)
		}
		current = next.DestinationAccount
	}
}

func (k Keeper) forwardTransition(ctx sdk.Context, source string) (types.AccountTransition, bool, error) {
	transition, found, err := k.transitionAtKey(ctx, types.AccountTransitionForwardKey(source))
	if err == nil && found && transition.SourceAccount != source {
		return types.AccountTransition{}, false, fmt.Errorf("malformed account transition: forward index key does not match source account")
	}
	return transition, found, err
}

func (k Keeper) reverseTransition(ctx sdk.Context, destination string) (types.AccountTransition, bool, error) {
	transition, found, err := k.transitionAtKey(ctx, types.AccountTransitionReverseKey(destination))
	if err == nil && found && transition.DestinationAccount != destination {
		return types.AccountTransition{}, false, fmt.Errorf("malformed account transition: reverse index key does not match destination account")
	}
	return transition, found, err
}

func (k Keeper) transitionAtKey(ctx sdk.Context, key []byte) (types.AccountTransition, bool, error) {
	bz := k.kvStore(ctx).Get(key)
	if bz == nil {
		return types.AccountTransition{}, false, nil
	}
	var transition types.AccountTransition
	if err := k.cdc.Unmarshal(bz, &transition); err != nil {
		return types.AccountTransition{}, false, fmt.Errorf("malformed account transition: %w", err)
	}
	if err := k.validateTransitionEndpoints(transition); err != nil {
		return types.AccountTransition{}, false, fmt.Errorf("malformed account transition: %w", err)
	}
	return transition, true, nil
}

// ImportAccountTransition restores already genesis-validated indexes without
// replaying singleton movement.
func (k Keeper) ImportAccountTransition(ctx sdk.Context, transition types.AccountTransition) error {
	if err := k.validateTransitionEndpoints(transition); err != nil {
		return err
	}
	bz, err := k.cdc.Marshal(&transition)
	if err != nil {
		return err
	}
	store := k.kvStore(ctx)
	store.Set(types.AccountTransitionForwardKey(transition.SourceAccount), bz)
	store.Set(types.AccountTransitionReverseKey(transition.DestinationAccount), bz)
	return nil
}

func (k Keeper) GetAllAccountTransitions(ctx sdk.Context) ([]types.AccountTransition, error) {
	prefix := types.AccountTransitionForwardPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()
	out := make([]types.AccountTransition, 0)
	for ; it.Valid(); it.Next() {
		if len(out) >= types.MaxAccountTransitions {
			return nil, fmt.Errorf("account transitions exceed limit %d", types.MaxAccountTransitions)
		}
		var transition types.AccountTransition
		if err := k.cdc.Unmarshal(it.Value(), &transition); err != nil {
			return nil, fmt.Errorf("malformed account transition: %w", err)
		}
		if err := k.validateTransitionEndpoints(transition); err != nil {
			return nil, fmt.Errorf("malformed account transition: %w", err)
		}
		if transition.SourceAccount != string(it.Key()[len(prefix):]) {
			return nil, fmt.Errorf("malformed account transition: forward index key does not match source account")
		}
		out = append(out, transition)
	}
	return out, nil
}

func (k Keeper) lineageRoot(ctx sdk.Context, account string) (string, error) {
	seen := make(map[string]struct{}, types.MaxAccountTransitions)
	for hops := 0; ; hops++ {
		if _, duplicate := seen[account]; duplicate {
			return "", fmt.Errorf("account transition cycle")
		}
		seen[account] = struct{}{}
		previous, ok, err := k.reverseTransition(ctx, account)
		if err != nil {
			return "", err
		}
		if !ok {
			return account, nil
		}
		if hops >= types.MaxAccountTransitions {
			return "", fmt.Errorf("account lineage exceeds transition limit %d", types.MaxAccountTransitions)
		}
		account = previous.SourceAccount
	}
}

func (k Keeper) accountTransitionCount(ctx sdk.Context) (int, error) {
	prefix := types.AccountTransitionForwardPrefix()
	it := k.kvStore(ctx).Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()
	count := 0
	for ; it.Valid(); it.Next() {
		count++
		if count > types.MaxAccountTransitions {
			return 0, fmt.Errorf("account transitions exceed limit %d", types.MaxAccountTransitions)
		}
	}
	return count, nil
}

func (k Keeper) buildSingletonMoves(ctx sdk.Context, source, destination string) ([]accountSingletonMove, error) {
	store := k.kvStore(ctx)
	pairs := [][2][]byte{
		{types.ActionFinalizationPostponementKey(source), types.ActionFinalizationPostponementKey(destination)},
		{types.StorageTruthPostponementKey(source), types.StorageTruthPostponementKey(destination)},
		{types.StorageTruthPostponementStrongKey(source), types.StorageTruthPostponementStrongKey(destination)},
		{types.NodeSuspicionStateKey(source), types.NodeSuspicionStateKey(destination)},
		{types.ReporterReliabilityStateKey(source), types.ReporterReliabilityStateKey(destination)},
	}
	moves := make([]accountSingletonMove, 0, len(pairs))
	for i, pair := range pairs {
		if store.Has(pair[1]) {
			return nil, fmt.Errorf("account transition singleton destination collision")
		}
		value := store.Get(pair[0])
		moves = append(moves, accountSingletonMove{
			sourceKey: append([]byte(nil), pair[0]...), destinationKey: append([]byte(nil), pair[1]...), sourceVal: append([]byte(nil), value...),
		})
		if value == nil {
			continue
		}
		destinationValue := append([]byte(nil), value...)
		switch i {
		case 0, 1:
			if len(value) != 8 {
				return nil, fmt.Errorf("malformed account transition epoch marker")
			}
		case 2:
			if !bytes.Equal(value, []byte{1}) {
				return nil, fmt.Errorf("malformed account transition strong-postpone marker")
			}
		case 3:
			var state types.NodeSuspicionState
			if err := k.cdc.Unmarshal(value, &state); err != nil || state.SupernodeAccount != source {
				return nil, fmt.Errorf("malformed account transition node-suspicion state")
			}
			state.SupernodeAccount = destination
			var err error
			destinationValue, err = k.cdc.Marshal(&state)
			if err != nil {
				return nil, err
			}
		case 4:
			var state types.ReporterReliabilityState
			if err := k.cdc.Unmarshal(value, &state); err != nil || state.ReporterSupernodeAccount != source {
				return nil, fmt.Errorf("malformed account transition reporter-reliability state")
			}
			state.ReporterSupernodeAccount = destination
			var err error
			destinationValue, err = k.cdc.Marshal(&state)
			if err != nil {
				return nil, err
			}
		}
		moves[len(moves)-1].destinationVal = destinationValue
	}
	return moves, nil
}

// accountTransitionWorkflowBlocker rejects moving an account participating in
// a non-final heal operation. The prefix scan is deterministic and capped.
func (k Keeper) accountTransitionWorkflowBlocker(ctx sdk.Context, source string) error {
	prefix := types.HealOpPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()
	count := 0
	for ; it.Valid(); it.Next() {
		count++
		if count > types.MaxIdentityTransitionHealOps {
			return fmt.Errorf("heal operations exceed identity transition limit %d", types.MaxIdentityTransitionHealOps)
		}
		var op types.HealOp
		if err := k.cdc.Unmarshal(it.Value(), &op); err != nil {
			return fmt.Errorf("malformed heal operation: %w", err)
		}
		if isHealOpFinalStatus(op.Status) {
			continue
		}
		if op.HealerSupernodeAccount == source {
			return fmt.Errorf("account transition blocked: source is healer in non-final heal operation %d", op.HealOpId)
		}
		for _, verifier := range op.VerifierSupernodeAccounts {
			if verifier == source {
				return fmt.Errorf("account transition blocked: source is verifier in non-final heal operation %d", op.HealOpId)
			}
		}
	}
	return nil
}
