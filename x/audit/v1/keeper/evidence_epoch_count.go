package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) getEvidenceEpochCount(ctx sdk.Context, epochID uint64, subjectAddress string, evidenceType types.EvidenceType) uint64 {
	store := k.kvStore(ctx)
	bz := store.Get(types.EvidenceEpochCountKey(epochID, subjectAddress, evidenceType))
	if len(bz) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) incrementEvidenceEpochCount(ctx sdk.Context, epochID uint64, subjectAddress string, evidenceType types.EvidenceType) {
	store := k.kvStore(ctx)
	key := types.EvidenceEpochCountKey(epochID, subjectAddress, evidenceType)

	current := uint64(0)
	if bz := store.Get(key); len(bz) == 8 {
		current = binary.BigEndian.Uint64(bz)
	}

	next := current + 1
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, next)
	store.Set(key, out)
}

func (k Keeper) setEvidenceEpochCount(ctx sdk.Context, epochID uint64, subjectAddress string, evidenceType types.EvidenceType, count uint64) {
	store := k.kvStore(ctx)
	key := types.EvidenceEpochCountKey(epochID, subjectAddress, evidenceType)
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, count)
	store.Set(key, out)
}

// GetAllEvidenceEpochCountsForGenesis returns all eve/ aggregate counters for
// genesis export. Per final-gate F-B3.
func (k Keeper) GetAllEvidenceEpochCountsForGenesis(ctx sdk.Context) []types.GenesisEvidenceEpochCount {
	store := k.kvStore(ctx)
	prefix := types.EvidenceEpochCountPrefix()
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()

	var out []types.GenesisEvidenceEpochCount
	for ; it.Valid(); it.Next() {
		key := it.Key()
		bz := it.Value()
		if len(key) <= len(prefix)+8+1+1+4 || len(bz) != 8 {
			continue
		}
		rest := key[len(prefix):]
		epochID := binary.BigEndian.Uint64(rest[:8])
		if rest[8] != '/' {
			continue
		}
		rest = rest[9:]
		sep := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				sep = i
				break
			}
		}
		if sep <= 0 || len(rest[sep+1:]) != 4 {
			continue
		}
		out = append(out, types.GenesisEvidenceEpochCount{
			EpochId:        epochID,
			SubjectAddress: string(rest[:sep]),
			EvidenceType:   types.EvidenceType(binary.BigEndian.Uint32(rest[sep+1:])),
			Count:          binary.BigEndian.Uint64(bz),
		})
	}
	return out
}
