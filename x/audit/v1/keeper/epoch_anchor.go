package keeper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"lukechampine.com/blake3"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const epochSeedDomain = "lumera:epoch-seed"

func (k Keeper) GetEpochAnchor(ctx sdk.Context, epochID uint64) (types.EpochAnchor, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.EpochAnchorKey(epochID))
	if bz == nil {
		return types.EpochAnchor{}, false
	}
	var anchor types.EpochAnchor
	k.cdc.MustUnmarshal(bz, &anchor)
	return anchor, true
}

func (k Keeper) SetEpochAnchor(ctx sdk.Context, anchor types.EpochAnchor) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&anchor)
	if err != nil {
		return err
	}
	store.Set(types.EpochAnchorKey(anchor.EpochId), bz)
	return nil
}

// CreateEpochAnchorIfNeeded persists a per-epoch anchor exactly at the epoch start height.
//
// This freezes:
// - the deterministic seed derived from the header hash at epoch start
// - the eligible supernode sets (active + targets) at epoch start
func (k Keeper) CreateEpochAnchorIfNeeded(ctx sdk.Context, epochID uint64, epochStartHeight, epochEndHeight int64, params types.Params) error {
	if _, found := k.GetEpochAnchor(ctx, epochID); found {
		return nil
	}
	if ctx.BlockHeight() != epochStartHeight {
		return fmt.Errorf("epoch anchor must be created at epoch start height: want=%d got=%d", epochStartHeight, ctx.BlockHeight())
	}

	active, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStateActive)
	if err != nil {
		return err
	}
	targetsSN, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStateActive, sntypes.SuperNodeStatePostponed)
	if err != nil {
		return err
	}

	activeAccounts, err := supernodeAccountsFromSet(active)
	if err != nil {
		return err
	}
	targetAccounts, err := supernodeAccountsFromSet(targetsSN)
	if err != nil {
		return err
	}

	sort.Strings(activeAccounts)
	sort.Strings(targetAccounts)

	seed, err := deriveEpochSeed(ctx, epochID, epochStartHeight)
	if err != nil {
		return err
	}

	params = params.WithDefaults()
	paramsBz := k.cdc.MustMarshal(&params)
	paramsCommit := blake3.Sum256(paramsBz)

	activeCommit := commitStringList(activeAccounts)
	targetCommit := commitStringList(targetAccounts)

	anchor := types.EpochAnchor{
		EpochId:                 epochID,
		EpochStartHeight:        epochStartHeight,
		EpochEndHeight:          epochEndHeight,
		EpochLengthBlocks:       params.EpochLengthBlocks,
		Seed:                    seed,
		ActiveSupernodeAccounts: activeAccounts,
		TargetSupernodeAccounts: targetAccounts,
		ParamsCommitment:        paramsCommit[:],
		ActiveSetCommitment:     activeCommit,
		TargetsSetCommitment:    targetCommit,
	}

	return k.SetEpochAnchor(ctx, anchor)
}

func deriveEpochSeed(ctx sdk.Context, epochID uint64, epochStartHeight int64) ([]byte, error) {
	raw := ctx.HeaderHash()
	if len(raw) == 0 {
		raw = make([]byte, 8)
		binary.BigEndian.PutUint64(raw, uint64(epochStartHeight))
	}
	epochBz := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBz, epochID)

	var msg bytes.Buffer
	msg.WriteString(epochSeedDomain)
	msg.Write(raw)
	msg.Write(epochBz)

	sum := blake3.Sum256(msg.Bytes())
	out := make([]byte, 32)
	copy(out, sum[:])
	return out, nil
}

func supernodeAccountsFromSet(set []sntypes.SuperNode) ([]string, error) {
	out := make([]string, 0, len(set))
	seen := make(map[string]struct{}, len(set))
	for _, sn := range set {
		if sn.SupernodeAccount == "" {
			return nil, fmt.Errorf("supernode %q has empty supernode_account", sn.ValidatorAddress)
		}
		if _, ok := seen[sn.SupernodeAccount]; ok {
			continue
		}
		seen[sn.SupernodeAccount] = struct{}{}
		out = append(out, sn.SupernodeAccount)
	}
	return out, nil
}

func commitStringList(list []string) []byte {
	h := blake3.New(32, nil)
	for _, s := range list {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return sum
}
