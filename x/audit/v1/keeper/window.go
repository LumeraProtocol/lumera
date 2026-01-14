package keeper

import (
	"encoding/binary"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

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

func computeKWindow(params types.Params, sendersCount, receiversCount int) uint32 {
	if sendersCount <= 0 || receiversCount <= 1 {
		return 0
	}

	a := uint64(sendersCount)
	n := uint64(receiversCount)
	q := uint64(params.PeerQuorumReports)

	kNeeded := (q*n + a - 1) / a

	kMin := uint64(params.MinProbeTargetsPerWindow)
	kMax := uint64(params.MaxProbeTargetsPerWindow)
	if kNeeded < kMin {
		kNeeded = kMin
	}
	if kNeeded > kMax {
		kNeeded = kMax
	}

	// Avoid self + no duplicates.
	if kNeeded > n-1 {
		kNeeded = n - 1
	}

	return uint32(kNeeded)
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
		senders = append(senders, sn.ValidatorAddress)
	}
	receivers := make([]string, 0, len(receiversSN))
	for _, sn := range receiversSN {
		receivers = append(receivers, sn.ValidatorAddress)
	}

	sort.Strings(senders)
	sort.Strings(receivers)

	snap := types.WindowSnapshot{
		WindowId:           windowID,
		WindowStartHeight:  ctx.BlockHeight(),
		SeedBytes:          ctx.HeaderHash(),
		Senders:            senders,
		Receivers:          receivers,
		KWindow:            computeKWindow(params, len(senders), len(receivers)),
	}

	return k.SetWindowSnapshot(ctx, snap)
}

func assignedTargets(snapshot types.WindowSnapshot, senderValidatorAddress string) ([]string, bool) {
	kWindow := int(snapshot.KWindow)
	if kWindow == 0 || len(snapshot.Receivers) == 0 {
		return []string{}, true
	}

	senderIndex := -1
	for i, s := range snapshot.Senders {
		if s == senderValidatorAddress {
			senderIndex = i
			break
		}
	}
	if senderIndex < 0 {
		return nil, false
	}

	seed := snapshot.SeedBytes
	if len(seed) < 8 {
		return nil, false
	}
	offsetU64 := binary.BigEndian.Uint64(seed[:8])
	n := len(snapshot.Receivers)
	offset := int(offsetU64 % uint64(n))

	seen := make(map[int]struct{}, kWindow)
	out := make([]string, 0, kWindow)

	for j := 0; j < kWindow; j++ {
		slot := senderIndex*kWindow + j
		candidate := (offset + slot) % n

		tries := 0
		for tries < n {
			if snapshot.Receivers[candidate] != senderValidatorAddress {
				if _, ok := seen[candidate]; !ok {
					break
				}
			}
			candidate = (candidate + 1) % n
			tries++
		}

		if tries >= n {
			break
		}

		seen[candidate] = struct{}{}
		out = append(out, snapshot.Receivers[candidate])
	}

	return out, true
}

