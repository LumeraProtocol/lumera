# Deterministic Assignment (Window Snapshot)

This document explains how `x/audit/v1` computes and persists the per-window **prober → targets** mapping (`WindowSnapshot.assignments`).

## Where it happens

- Snapshot creation is triggered in `BeginBlocker` exactly at `window_start_height` for a window.
  - File: `x/audit/v1/keeper/abci.go`
- Snapshot computation + persistence is implemented in:
  - File: `x/audit/v1/keeper/window.go`
- The deterministic assignment algorithm itself is implemented as pure helpers in:
  - File: `x/audit/v1/keeper/assignment/assignment.go`

## Inputs to the mapping

At `window_start_height` for a given `window_id`, the module builds:

- `probers` (called `senders` in code): supernodes in state `ACTIVE`
- `targets` (called `receivers` in code): supernodes in state `ACTIVE` + `POSTPONED`
- `params`: module params as of that height:
  - `peer_quorum_reports`
  - `min_probe_targets_per_window`
  - `max_probe_targets_per_window`
- `seedBytes`: `ctx.HeaderHash()` at `window_start_height` (the first 8 bytes are used)

Note:
- Snapshot creation requires `len(ctx.HeaderHash()) >= 8` (otherwise snapshot creation fails).

## Ordering (determinism invariant)

To ensure all nodes compute the same mapping:
- `probers` (`senders`) are sorted lexicographically by `supernode_account` (bech32 string).
- `targets` (`receivers`) are sorted lexicographically by `supernode_account`.

This sorting step is part of the deterministic assignment helpers.

## Step 1: Compute `k_window`

`k_window` is the number of targets assigned to each sender for a window.

Let:
- `a = len(probers)`
- `n = len(targets)`
- `q = peer_quorum_reports`

We compute the minimum `k` to get average per-receiver coverage of at least `q`:

```
sendersCount * k_window >= peer_quorum_reports * receiversCount
k_needed = ceil(q*n / a)
```

Then clamp:
- `k_needed = max(k_needed, min_probe_targets_per_window)`
- `k_needed = min(k_needed, max_probe_targets_per_window)`

And finally:
- `k_needed <= n - 1` (no self-targeting and no duplicates is only possible up to `n-1`)

Edge cases:
- If `a <= 0` or `n <= 1`, `k_window = 0`.

## Step 2: Compute a deterministic ring offset from the seed

Only the first 8 bytes are used:

- `offsetU64 = bigEndianUint64(seedBytes[0:8])`
- `offset = offsetU64 % len(targets)`

This rotates the receiver list per window so assignments don’t always start at receiver index 0.

## Step 3: Assign targets to each sender

For each prober at index `senderIndex` in sorted `probers`:

For each `j` from `0` to `k_window - 1`:

1) Compute a deterministic slot:
   - `slot = senderIndex * k_window + j`
2) Initial candidate:
   - `candidateIndex = (offset + slot) % len(targets)`
3) Resolve collisions deterministically by walking forward (wrap-around):
   - Reject candidate if:
     - `targets[candidateIndex] == sender` (self), or
     - candidate already chosen for this sender (no duplicates)
   - If rejected, set `candidateIndex = (candidateIndex + 1) % len(targets)` and retry
   - Stop once a valid candidate is found, or after `len(targets)` tries (in which case assignment stops early)

The resulting ordered `targets[]` are stored as one `ProberTargets` record:

```protobuf
message ProberTargets {
  string prober_supernode_account = 1;
  repeated string target_supernode_accounts = 2;
}
```

## What is persisted

The module persists a `WindowSnapshot` in state under `ws/<window_id>`:

```protobuf
message WindowSnapshot {
  uint64 window_id = 1;
  int64 window_start_height = 2;
  repeated ProberTargets assignments = 3;
}
```

Notes:
- The seed is used during snapshot creation, but is **not persisted** in `WindowSnapshot`.
- The persisted mapping is the minimal per-window source-of-truth for prober → targets.

## Determinism invariants (summary)

All nodes will compute the same `assignments` for a window if:
- They evaluate at the same `window_start_height`
- They observe the same ACTIVE / ACTIVE+POSTPONED sets at that height
- They sort senders/receivers identically
- They use the same params at that height (`peer_quorum_reports`, min/max probes)
- They use the same `seedBytes` (`HeaderHash()` at that height)
- They run the same collision-resolution rules (self + duplicate avoidance)
