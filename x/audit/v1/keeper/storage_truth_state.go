package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (k Keeper) HasNodeSuspicionState(ctx sdk.Context, supernodeAccount string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.NodeSuspicionStateKey(supernodeAccount))
}

func (k Keeper) GetNodeSuspicionState(ctx sdk.Context, supernodeAccount string) (types.NodeSuspicionState, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.NodeSuspicionStateKey(supernodeAccount))
	if bz == nil {
		return types.NodeSuspicionState{}, false
	}
	var state types.NodeSuspicionState
	k.cdc.MustUnmarshal(bz, &state)
	return state, true
}

func (k Keeper) SetNodeSuspicionState(ctx sdk.Context, state types.NodeSuspicionState) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&state)
	if err != nil {
		return err
	}
	store.Set(types.NodeSuspicionStateKey(state.SupernodeAccount), bz)
	return nil
}

func (k Keeper) GetAllNodeSuspicionStates(ctx sdk.Context) ([]types.NodeSuspicionState, error) {
	store := k.kvStore(ctx)
	it := store.Iterator(types.NodeSuspicionStatePrefix(), storetypes.PrefixEndBytes(types.NodeSuspicionStatePrefix()))
	defer func() { _ = it.Close() }()

	states := make([]types.NodeSuspicionState, 0)
	for ; it.Valid(); it.Next() {
		var state types.NodeSuspicionState
		k.cdc.MustUnmarshal(it.Value(), &state)
		states = append(states, state)
	}
	return states, nil
}

func (k Keeper) HasReporterReliabilityState(ctx sdk.Context, reporterSupernodeAccount string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.ReporterReliabilityStateKey(reporterSupernodeAccount))
}

func (k Keeper) GetReporterReliabilityState(ctx sdk.Context, reporterSupernodeAccount string) (types.ReporterReliabilityState, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.ReporterReliabilityStateKey(reporterSupernodeAccount))
	if bz == nil {
		return types.ReporterReliabilityState{}, false
	}
	var state types.ReporterReliabilityState
	k.cdc.MustUnmarshal(bz, &state)
	return state, true
}

func (k Keeper) SetReporterReliabilityState(ctx sdk.Context, state types.ReporterReliabilityState) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&state)
	if err != nil {
		return err
	}
	store.Set(types.ReporterReliabilityStateKey(state.ReporterSupernodeAccount), bz)
	return nil
}

func (k Keeper) GetAllReporterReliabilityStates(ctx sdk.Context) ([]types.ReporterReliabilityState, error) {
	store := k.kvStore(ctx)
	it := store.Iterator(types.ReporterReliabilityStatePrefix(), storetypes.PrefixEndBytes(types.ReporterReliabilityStatePrefix()))
	defer func() { _ = it.Close() }()

	states := make([]types.ReporterReliabilityState, 0)
	for ; it.Valid(); it.Next() {
		var state types.ReporterReliabilityState
		k.cdc.MustUnmarshal(it.Value(), &state)
		states = append(states, state)
	}
	return states, nil
}

func (k Keeper) HasTicketDeteriorationState(ctx sdk.Context, ticketID string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.TicketDeteriorationStateKey(ticketID))
}

func (k Keeper) GetTicketDeteriorationState(ctx sdk.Context, ticketID string) (types.TicketDeteriorationState, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.TicketDeteriorationStateKey(ticketID))
	if bz == nil {
		return types.TicketDeteriorationState{}, false
	}
	var state types.TicketDeteriorationState
	k.cdc.MustUnmarshal(bz, &state)
	return state, true
}

func (k Keeper) SetTicketDeteriorationState(ctx sdk.Context, state types.TicketDeteriorationState) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&state)
	if err != nil {
		return err
	}
	store.Set(types.TicketDeteriorationStateKey(state.TicketId), bz)
	return nil
}

func (k Keeper) GetAllTicketDeteriorationStates(ctx sdk.Context) ([]types.TicketDeteriorationState, error) {
	store := k.kvStore(ctx)
	it := store.Iterator(types.TicketDeteriorationStatePrefix(), storetypes.PrefixEndBytes(types.TicketDeteriorationStatePrefix()))
	defer func() { _ = it.Close() }()

	states := make([]types.TicketDeteriorationState, 0)
	for ; it.Valid(); it.Next() {
		var state types.TicketDeteriorationState
		k.cdc.MustUnmarshal(it.Value(), &state)
		states = append(states, state)
	}
	return states, nil
}

func (k Keeper) HasTicketArtifactCountState(ctx sdk.Context, ticketID string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.TicketArtifactCountStateKey(ticketID))
}

func (k Keeper) GetTicketArtifactCountState(ctx sdk.Context, ticketID string) (types.TicketArtifactCountState, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.TicketArtifactCountStateKey(ticketID))
	if bz == nil {
		return types.TicketArtifactCountState{}, false
	}
	var state types.TicketArtifactCountState
	k.cdc.MustUnmarshal(bz, &state)
	return state, true
}

func (k Keeper) SetTicketArtifactCountState(ctx sdk.Context, state types.TicketArtifactCountState) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&state)
	if err != nil {
		return err
	}
	store.Set(types.TicketArtifactCountStateKey(state.TicketId), bz)
	return nil
}

func (k Keeper) GetAllTicketArtifactCountStates(ctx sdk.Context) ([]types.TicketArtifactCountState, error) {
	store := k.kvStore(ctx)
	it := store.Iterator(types.TicketArtifactCountStatePrefix(), storetypes.PrefixEndBytes(types.TicketArtifactCountStatePrefix()))
	defer func() { _ = it.Close() }()

	states := make([]types.TicketArtifactCountState, 0)
	for ; it.Valid(); it.Next() {
		var state types.TicketArtifactCountState
		k.cdc.MustUnmarshal(it.Value(), &state)
		states = append(states, state)
	}
	return states, nil
}

func (k Keeper) GetNextHealOpID(ctx sdk.Context) uint64 {
	store := k.kvStore(ctx)
	bz := store.Get(types.NextHealOpIDKey())
	if bz == nil {
		// Match GetNextEvidenceID: missing counter must not risk ID reuse.
		return k.deriveNextHealOpID(ctx)
	}
	if len(bz) != 8 {
		// Per CP-R3 A-F1 — recover from malformed state the same way evidence IDs do.
		return k.deriveNextHealOpID(ctx)
	}
	id := binary.BigEndian.Uint64(bz)
	if id == 0 {
		// Heal-op IDs start at 1; treat 0 as corrupt and derive a safe next ID.
		return k.deriveNextHealOpID(ctx)
	}
	return id
}

func (k Keeper) deriveNextHealOpID(ctx sdk.Context) uint64 {
	prefix := types.HealOpPrefix()
	it := k.kvStore(ctx).Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()

	var maxID uint64
	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) != len(prefix)+8 {
			continue
		}
		id := binary.BigEndian.Uint64(key[len(prefix):])
		if id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}

func (k Keeper) SetNextHealOpID(ctx sdk.Context, id uint64) {
	store := k.kvStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	store.Set(types.NextHealOpIDKey(), bz)
}

func (k Keeper) HasHealOp(ctx sdk.Context, healOpID uint64) bool {
	store := k.kvStore(ctx)
	return store.Has(types.HealOpKey(healOpID))
}

func (k Keeper) GetHealOp(ctx sdk.Context, healOpID uint64) (types.HealOp, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.HealOpKey(healOpID))
	if bz == nil {
		return types.HealOp{}, false
	}
	var healOp types.HealOp
	k.cdc.MustUnmarshal(bz, &healOp)
	return healOp, true
}

func (k Keeper) SetHealOp(ctx sdk.Context, healOp types.HealOp) error {
	store := k.kvStore(ctx)

	if existing, found := k.GetHealOp(ctx, healOp.HealOpId); found {
		store.Delete(types.HealOpByTicketIndexKey(existing.TicketId, existing.HealOpId))
		store.Delete(types.HealOpByStatusIndexKey(existing.Status, existing.HealOpId))
	}

	bz, err := k.cdc.Marshal(&healOp)
	if err != nil {
		return err
	}
	store.Set(types.HealOpKey(healOp.HealOpId), bz)
	store.Set(types.HealOpByTicketIndexKey(healOp.TicketId, healOp.HealOpId), []byte{1})
	store.Set(types.HealOpByStatusIndexKey(healOp.Status, healOp.HealOpId), []byte{1})
	return nil
}

func (k Keeper) GetAllHealOps(ctx sdk.Context) ([]types.HealOp, error) {
	store := k.kvStore(ctx)
	it := store.Iterator(types.HealOpPrefix(), storetypes.PrefixEndBytes(types.HealOpPrefix()))
	defer func() { _ = it.Close() }()

	healOps := make([]types.HealOp, 0)
	for ; it.Valid(); it.Next() {
		var healOp types.HealOp
		k.cdc.MustUnmarshal(it.Value(), &healOp)
		healOps = append(healOps, healOp)
	}
	return healOps, nil
}

func (k Keeper) HasHealOpVerification(ctx sdk.Context, healOpID uint64, verifierSupernodeAccount string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.HealOpVerificationKey(healOpID, verifierSupernodeAccount))
}

func (k Keeper) SetHealOpVerification(ctx sdk.Context, healOpID uint64, verifierSupernodeAccount string, verified bool) {
	store := k.kvStore(ctx)
	value := byte(0)
	if verified {
		value = 1
	}
	store.Set(types.HealOpVerificationKey(healOpID, verifierSupernodeAccount), []byte{value})
}

func (k Keeper) GetHealOpVerification(ctx sdk.Context, healOpID uint64, verifierSupernodeAccount string) (bool, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.HealOpVerificationKey(healOpID, verifierSupernodeAccount))
	if len(bz) == 0 {
		return false, false
	}
	return bz[0] == 1, true
}

func (k Keeper) GetAllHealOpVerifications(ctx sdk.Context, healOpID uint64) (map[string]bool, error) {
	store := k.kvStore(ctx)
	prefix := types.HealOpVerificationPrefix(healOpID)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()

	verifications := make(map[string]bool)
	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) <= len(prefix) {
			continue
		}
		verifier := string(key[len(prefix):])
		value := len(it.Value()) != 0 && it.Value()[0] == 1
		verifications[verifier] = value
	}
	return verifications, nil
}

// GetAllHealOpVerificationsForGenesis returns all st/hov/ per-verifier heal-op
// votes for genesis export. Per final-gate F-B4.
func (k Keeper) GetAllHealOpVerificationsForGenesis(ctx sdk.Context) []types.GenesisHealOpVerification {
	store := k.kvStore(ctx)
	prefix := types.HealOpVerificationRootPrefix()
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()

	var out []types.GenesisHealOpVerification
	for ; it.Valid(); it.Next() {
		key := it.Key()
		bz := it.Value()
		if len(key) <= len(prefix)+8+1 || len(bz) == 0 {
			continue
		}
		rest := key[len(prefix):]
		healOpID := binary.BigEndian.Uint64(rest[:8])
		if rest[8] != '/' || len(rest[9:]) == 0 {
			continue
		}
		out = append(out, types.GenesisHealOpVerification{
			HealOpId:                 healOpID,
			VerifierSupernodeAccount: string(rest[9:]),
			Verified:                 bz[0] == 1,
		})
	}
	return out
}
