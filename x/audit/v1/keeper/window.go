package keeper

import (
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper/assignment"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (k Keeper) getWindowOriginHeight(ctx sdk.Context) (int64, bool) {
	// The origin height is set once (on first use) and then kept stable forever.
	// All window boundaries are derived from it to avoid drifting schedules.
	store := k.kvStore(ctx)
	bz := store.Get(types.WindowOriginHeightKey())
	if len(bz) != 8 {
		return 0, false
	}
	return int64(binary.BigEndian.Uint64(bz)), true
}

func (k Keeper) getOrInitWindowOriginHeight(ctx sdk.Context) int64 {
	store := k.kvStore(ctx)
	bz := store.Get(types.WindowOriginHeightKey())
	if len(bz) == 8 {
		return int64(binary.BigEndian.Uint64(bz))
	}

	origin := ctx.BlockHeight()
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, uint64(origin))
	store.Set(types.WindowOriginHeightKey(), out)
	return origin
}

func (k Keeper) windowStartHeight(origin int64, params types.Params, windowID uint64) int64 {
	return origin + int64(windowID)*int64(params.ReportingWindowBlocks)
}

func (k Keeper) windowEndHeight(origin int64, params types.Params, windowID uint64) int64 {
	return k.windowStartHeight(origin, params, windowID) + int64(params.ReportingWindowBlocks) - 1
}

func (k Keeper) windowIDAtHeight(origin int64, params types.Params, height int64) uint64 {
	if height < origin {
		return 0
	}
	return uint64((height - origin) / int64(params.ReportingWindowBlocks))
}

func (k Keeper) GetWindowSnapshot(ctx sdk.Context, windowID uint64) (types.WindowSnapshot, bool) {
	store := k.kvStore(ctx)
	bz := store.Get(types.WindowSnapshotKey(windowID))
	if bz == nil {
		return types.WindowSnapshot{}, false
	}
	var snap types.WindowSnapshot
	k.cdc.MustUnmarshal(bz, &snap)
	return snap, true
}

func (k Keeper) SetWindowSnapshot(ctx sdk.Context, snap types.WindowSnapshot) error {
	store := k.kvStore(ctx)
	bz, err := k.cdc.Marshal(&snap)
	if err != nil {
		return err
	}
	store.Set(types.WindowSnapshotKey(snap.WindowId), bz)
	return nil
}

func (k Keeper) CreateWindowSnapshotIfNeeded(ctx sdk.Context, windowID uint64, params types.Params) error {
	if _, found := k.GetWindowSnapshot(ctx, windowID); found {
		return nil
	}

	active, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStateActive)
	if err != nil {
		return err
	}
	receiversSN, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStateActive, sntypes.SuperNodeStatePostponed)
	if err != nil {
		return err
	}

	senders := make([]string, 0, len(active))
	for _, sn := range active {
		if sn.SupernodeAccount == "" {
			return fmt.Errorf("supernode %q has empty supernode_account", sn.ValidatorAddress)
		}
		senders = append(senders, sn.SupernodeAccount)
	}
	receivers := make([]string, 0, len(receiversSN))
	for _, sn := range receiversSN {
		if sn.SupernodeAccount == "" {
			return fmt.Errorf("supernode %q has empty supernode_account", sn.ValidatorAddress)
		}
		receivers = append(receivers, sn.SupernodeAccount)
	}

	seedBytes := ctx.HeaderHash()
	assignments, err := assignment.ComputeSnapshotAssignments(params, senders, receivers, seedBytes)
	if err != nil {
		return err
	}

	snap := types.WindowSnapshot{
		WindowId:          windowID,
		WindowStartHeight: ctx.BlockHeight(),
		Assignments:       assignments,
	}

	return k.SetWindowSnapshot(ctx, snap)
}
