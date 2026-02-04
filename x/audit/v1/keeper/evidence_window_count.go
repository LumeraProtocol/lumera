package keeper

import (
	"encoding/binary"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) getEvidenceWindowCount(ctx sdk.Context, windowID uint64, subjectAddress string, evidenceType types.EvidenceType) uint64 {
	store := k.kvStore(ctx)
	bz := store.Get(types.EvidenceWindowCountKey(windowID, subjectAddress, evidenceType))
	if len(bz) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) incrementEvidenceWindowCount(ctx sdk.Context, windowID uint64, subjectAddress string, evidenceType types.EvidenceType) {
	store := k.kvStore(ctx)
	key := types.EvidenceWindowCountKey(windowID, subjectAddress, evidenceType)

	current := uint64(0)
	if bz := store.Get(key); len(bz) == 8 {
		current = binary.BigEndian.Uint64(bz)
	}

	next := current + 1
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, next)
	store.Set(key, out)
}
