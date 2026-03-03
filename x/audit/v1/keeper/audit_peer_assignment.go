package keeper

import (
	"encoding/binary"
	"fmt"
)

// computeAuditPeerTargetsForReporter deterministically computes the set of targets a reporter must observe
// for the given epoch, using only the epoch anchor + current params.
//
// Deterministic assignments are derived from the epoch anchor; we do not store a per-epoch assignment table on-chain.
func computeAuditPeerTargetsForReporter(params paramsLike, activeSorted []string, targetsSorted []string, seed []byte, reporter string) ([]string, bool, error) {
	if reporter == "" {
		return nil, false, fmt.Errorf("empty reporter")
	}
	if len(seed) < 8 {
		return nil, false, fmt.Errorf("seed must be at least 8 bytes")
	}

	reporterIndex := -1
	for i, s := range activeSorted {
		if s == reporter {
			reporterIndex = i
			break
		}
	}
	if reporterIndex < 0 {
		return []string{}, false, nil
	}

	sendersCount := len(activeSorted)
	receiversCount := len(targetsSorted)
	kEpoch := computeKFromParams(params, sendersCount, receiversCount)
	if kEpoch == 0 || receiversCount == 0 {
		return []string{}, true, nil
	}

	offsetU64 := binary.BigEndian.Uint64(seed[:8])
	offset := int(offsetU64 % uint64(receiversCount))

	seen := make(map[int]struct{}, int(kEpoch))
	out := make([]string, 0, int(kEpoch))

	for j := 0; j < int(kEpoch); j++ {
		slot := reporterIndex*int(kEpoch) + j
		candidate := (offset + slot) % receiversCount

		tries := 0
		for tries < receiversCount {
			if targetsSorted[candidate] != reporter {
				if _, ok := seen[candidate]; !ok {
					break
				}
			}
			candidate = (candidate + 1) % receiversCount
			tries++
		}

		if tries >= receiversCount {
			break
		}

		seen[candidate] = struct{}{}
		out = append(out, targetsSorted[candidate])
	}

	return out, true, nil
}

type paramsLike interface {
	GetPeerQuorumReports() uint32
	GetMinProbeTargetsPerEpoch() uint32
	GetMaxProbeTargetsPerEpoch() uint32
}

func computeKFromParams(params paramsLike, sendersCount, receiversCount int) uint32 {
	if sendersCount <= 0 || receiversCount <= 1 {
		return 0
	}

	a := uint64(sendersCount)
	n := uint64(receiversCount)
	q := uint64(params.GetPeerQuorumReports())

	kNeeded := (q*n + a - 1) / a

	kMin := uint64(params.GetMinProbeTargetsPerEpoch())
	kMax := uint64(params.GetMaxProbeTargetsPerEpoch())
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
