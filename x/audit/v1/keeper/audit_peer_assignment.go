package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
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
	if params.GetStorageTruthEnforcementMode() != types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED {
		return computeStorageTruthTargetsForReporter(params, activeSorted, targetsSorted, seed, reporter), containsString(activeSorted, reporter), nil
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
	GetStorageTruthEnforcementMode() types.StorageTruthEnforcementMode
	GetStorageTruthChallengeTargetDivisor() uint32
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

func computeStorageTruthTargetsForReporter(params paramsLike, activeSorted []string, targetsSorted []string, seed []byte, reporter string) []string {
	if !containsString(activeSorted, reporter) {
		return []string{}
	}

	active := sortedUniqueStrings(activeSorted)
	targetCandidates := intersectionInOrder(sortedUniqueStrings(targetsSorted), active)
	if len(targetCandidates) == 0 {
		targetCandidates = active
	}
	if len(active) <= 1 || len(targetCandidates) == 0 {
		return []string{}
	}

	targetCount := storageTruthChallengeTargetCount(params, len(active))
	if targetCount > len(targetCandidates) {
		targetCount = len(targetCandidates)
	}

	rankedTargets := rankStorageTruthAccounts(seed, targetCandidates, "challenge_target")
	selectedTargets := make([]string, 0, targetCount)
	for _, ranked := range rankedTargets {
		if len(selectedTargets) >= targetCount {
			break
		}
		selectedTargets = append(selectedTargets, ranked.account)
	}

	unassignedTargets := make(map[string]struct{}, len(selectedTargets))
	assignedTargets := make(map[string]struct{}, len(selectedTargets))
	for _, target := range selectedTargets {
		unassignedTargets[target] = struct{}{}
	}

	for _, challenger := range active {
		if len(assignedTargets) >= targetCount {
			break
		}
		bestTarget := ""
		var bestRank []byte
		for target := range unassignedTargets {
			if target == challenger {
				continue
			}
			rank := storageTruthAssignmentHash(seed, challenger, target, "pair")
			if bestTarget == "" || bytes.Compare(rank, bestRank) < 0 || (bytes.Equal(rank, bestRank) && target < bestTarget) {
				bestTarget = target
				bestRank = rank
			}
		}
		if bestTarget == "" {
			for _, target := range rankedTargets {
				if _, alreadyAssigned := assignedTargets[target.account]; alreadyAssigned || target.account == challenger {
					continue
				}
				rank := storageTruthAssignmentHash(seed, challenger, target.account, "pair")
				if bestTarget == "" || bytes.Compare(rank, bestRank) < 0 || (bytes.Equal(rank, bestRank) && target.account < bestTarget) {
					bestTarget = target.account
					bestRank = rank
				}
			}
			if bestTarget == "" {
				continue
			}
		}
		delete(unassignedTargets, bestTarget)
		assignedTargets[bestTarget] = struct{}{}
		if challenger == reporter {
			return []string{bestTarget}
		}
	}

	return []string{}
}

func storageTruthChallengeTargetCount(params paramsLike, activeCount int) int {
	if activeCount <= 0 {
		return 0
	}
	divisor := int(params.GetStorageTruthChallengeTargetDivisor())
	if divisor <= 0 {
		divisor = int(types.DefaultStorageTruthChallengeTargetDivisor)
	}
	count := (activeCount + divisor - 1) / divisor
	if count < 1 {
		count = 1
	}
	if count > activeCount {
		count = activeCount
	}
	return count
}

type rankedStorageTruthAccount struct {
	account string
	rank    []byte
}

func rankStorageTruthAccounts(seed []byte, accounts []string, label string) []rankedStorageTruthAccount {
	ranked := make([]rankedStorageTruthAccount, 0, len(accounts))
	for _, account := range accounts {
		ranked = append(ranked, rankedStorageTruthAccount{
			account: account,
			rank:    storageTruthAssignmentHash(seed, account, label),
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		cmp := bytes.Compare(ranked[i].rank, ranked[j].rank)
		if cmp != 0 {
			return cmp < 0
		}
		return ranked[i].account < ranked[j].account
	})
	return ranked
}

func storageTruthAssignmentHash(seed []byte, parts ...string) []byte {
	h := sha256.New()
	_, _ = h.Write(seed)
	for _, part := range parts {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(part))
	}
	return h.Sum(nil)
}

func sortedUniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, value := range in {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func intersectionInOrder(values []string, allowed []string) []string {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := allowedSet[value]; ok {
			out = append(out, value)
		}
	}
	return out
}

func (k Keeper) storageTruthEligibleChallengers(ctx sdk.Context, activeSorted []string, epochID uint64, params types.Params) []string {
	if params.StorageTruthEnforcementMode == types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED {
		return append([]string(nil), activeSorted...)
	}

	threshold := params.StorageTruthReporterReliabilityIneligibleThreshold
	if threshold <= 0 {
		threshold = types.DefaultStorageTruthReporterReliabilityIneligibleThreshold
	}

	eligible := make([]string, 0, len(activeSorted))
	for _, account := range activeSorted {
		state, found := k.GetReporterReliabilityState(ctx, account)
		if !found {
			eligible = append(eligible, account)
			continue
		}
		score := decayTowardZero(state.ReliabilityScore, params.StorageTruthReporterReliabilityDecayPerEpoch, epochDelta(epochID, state.LastUpdatedEpoch))
		if score >= threshold || (state.IneligibleUntilEpoch != 0 && state.IneligibleUntilEpoch >= epochID) {
			continue
		}
		eligible = append(eligible, account)
	}
	return eligible
}
