package keeper

import (
	"bytes"
	"encoding/hex"
	"sort"
	"strconv"

	"lukechampine.com/blake3"
)

type xorCandidate struct {
	id   string
	dist [32]byte
}

func storageChallengeComparisonTarget(seed []byte, epochID uint64) string {
	return "sc:challengers:" + hex.EncodeToString(seed) + ":" + strconv.FormatUint(epochID, 10)
}

func storageChallengeChallengerCount(nActive int, requested uint32) int {
	if nActive <= 0 {
		return 0
	}
	if requested == 0 {
		// auto = ceil(N/3), minimum 1
		return maxInt(1, (nActive+2)/3)
	}
	if int(requested) > nActive {
		return nActive
	}
	return int(requested)
}

func selectTopByXORDistance(ids []string, target string, k int) []string {
	if k <= 0 || len(ids) == 0 {
		return nil
	}

	targetHash := ensureHashedTargetBytes([]byte(target))

	candidates := make([]xorCandidate, 0, len(ids))
	for _, id := range ids {
		idHash := blake3.Sum256([]byte(id))
		var dist [32]byte
		for i := 0; i < 32; i++ {
			dist[i] = idHash[i] ^ targetHash[i]
		}
		candidates = append(candidates, xorCandidate{id: id, dist: dist})
	}

	sort.Slice(candidates, func(i, j int) bool {
		cmp := bytes.Compare(candidates[i].dist[:], candidates[j].dist[:])
		if cmp != 0 {
			return cmp < 0
		}
		return candidates[i].id < candidates[j].id
	})

	if k > len(candidates) {
		k = len(candidates)
	}
	out := make([]string, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, candidates[i].id)
	}
	return out
}

func ensureHashedTargetBytes(target []byte) [32]byte {
	if len(target) == 32 {
		var out [32]byte
		copy(out[:], target)
		return out
	}
	return blake3.Sum256(target)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
