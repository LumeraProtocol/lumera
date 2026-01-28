package assignment

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// Deterministic assignment helpers.
//
// These helpers are intentionally pure (no keeper/store access) so the same logic can be reused by:
// - snapshot creation at window start
// - any future validation that wants to recompute the expected mapping from inputs
//
// Determinism relies on:
// - identical sender/receiver sets and their ordering (we sort lexicographically)
// - identical params at snapshot time (peer_quorum_reports, min/max probes)
// - identical seed bytes (we use the first 8 bytes to compute a ring offset)

func ComputeKWindowFromParams(params types.Params, sendersCount, receiversCount int) uint32 {
	// k_window is the number of targets each sender is assigned for a window.
	//
	// We derive it so that, on average, each receiver gets at least peer_quorum_reports observations:
	//   sendersCount * k_window >= peer_quorum_reports * receiversCount
	// Then clamp into [min_probe_targets_per_window, max_probe_targets_per_window] and ensure k_window <= receiversCount-1
	// (no self-targeting, no duplicates).
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

func computeAssignmentsFromInputs(senders []string, receivers []string, kWindow uint32, seedBytes []byte) ([]types.ProberTargets, error) {
	assignments := make([]types.ProberTargets, 0, len(senders))
	if kWindow == 0 || len(receivers) == 0 {
		for _, sender := range senders {
			assignments = append(assignments, types.ProberTargets{
				ProberSupernodeAccount:  sender,
				TargetSupernodeAccounts: []string{},
			})
		}
		return assignments, nil
	}
	if len(seedBytes) < 8 {
		return nil, fmt.Errorf("seed must be at least 8 bytes")
	}

	offsetU64 := binary.BigEndian.Uint64(seedBytes[:8])
	n := len(receivers)
	offset := int(offsetU64 % uint64(n))

	for senderIndex, sender := range senders {
		seen := make(map[int]struct{}, int(kWindow))
		targets := make([]string, 0, int(kWindow))

		for j := 0; j < int(kWindow); j++ {
			slot := senderIndex*int(kWindow) + j
			candidate := (offset + slot) % n

			tries := 0
			for tries < n {
				if receivers[candidate] != sender {
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
			targets = append(targets, receivers[candidate])
		}

		assignments = append(assignments, types.ProberTargets{
			ProberSupernodeAccount:  sender,
			TargetSupernodeAccounts: targets,
		})
	}

	return assignments, nil
}

func computeSnapshotAssignments(params types.Params, senders []string, receivers []string, seedBytes []byte) ([]types.ProberTargets, error) {
	// Sort to guarantee deterministic ordering across nodes.
	// Copy first to avoid mutating the caller's slices.
	sendersSorted := append([]string(nil), senders...)
	receiversSorted := append([]string(nil), receivers...)
	sort.Strings(sendersSorted)
	sort.Strings(receiversSorted)

	kWindow := ComputeKWindowFromParams(params, len(sendersSorted), len(receiversSorted))
	return computeAssignmentsFromInputs(sendersSorted, receiversSorted, kWindow, seedBytes)
}

// ComputeSnapshotAssignments is the canonical helper used by the module to derive the per-window prober -> targets mapping.
func ComputeSnapshotAssignments(params types.Params, senders []string, receivers []string, seedBytes []byte) ([]types.ProberTargets, error) {
	return computeSnapshotAssignments(params, senders, receivers, seedBytes)
}
