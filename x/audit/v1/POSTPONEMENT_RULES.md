# Postponement and Recovery Rules (audit/v1)

This document describes the on-chain rules implemented by `x/audit/v1` for switching a supernode between `ACTIVE` and `POSTPONED`, and for recovering back to `ACTIVE`.

## Definitions

- **Epoch**: a contiguous block-height interval `[epoch_start_height, epoch_end_height]` derived from `epoch_zero_height` and `epoch_length_blocks`.
- **Probers**: supernodes that are `ACTIVE` at epoch start (i.e., present in `EpochAnchor.active_supernode_accounts`).
- **Targets**: supernodes that are `ACTIVE` or `POSTPONED` at epoch start (i.e., present in `EpochAnchor.target_supernode_accounts`).
- **Report**: `MsgSubmitAuditReport` stored under `(epoch_id, supernode_account)`.

## Enforcement timing

All postpone/recovery decisions are evaluated only at **epoch end** (`epoch_end_height`) in `EndBlocker`.

## Submission-time gating and completeness

Peer observation gating is enforced only when `MsgSubmitAuditReport` is accepted (enforcement later assumes stored observations already passed these checks):

- If the reporter is a **prober** for the epoch (it is in `EpochAnchor.active_supernode_accounts`), then:
  - `peer_observations` must include **exactly one entry per assigned target** (no missing targets, no extra targets, no duplicates).
  - For each `peer_observation`, `port_states` length must equal the configured `required_open_ports` length.
- If the reporter is **not** a prober for the epoch (e.g. `POSTPONED`), then:
  - `peer_observations` must be empty (self-report only).

## Postpone rules

### 1) Missing reports

If an `ACTIVE` supernode fails to submit any report for `consecutive_epochs_to_postpone` consecutive epochs, it is set to `POSTPONED`.

This is evaluated by checking for a stored report in each of the last `N` epochs.

### 2) Host requirements (self report)

If a submitted self report violates any enabled minimum free% threshold, the supernode is set to `POSTPONED`.

- Params: `min_cpu_free_percent`, `min_mem_free_percent`, `min_disk_free_percent` (`free% = 100 - usage%`).
- Special case: if `*_usage_percent == 0`, that metric is treated as **unknown** and does not trigger postponement.

The following self-report fields are currently ignored by postponement logic:
- `failed_actions_count`
- `inbound_port_states`

### 3) Peer ports (peer observations)

For any required port index `i`, a target is postponed if peers report that port as `CLOSED` at or above `peer_port_postpone_threshold_percent` for `consecutive_epochs_to_postpone` consecutive epochs.

An epoch counts toward the consecutive requirement only if:
- there is at least **1** peer reporter about the target in that epoch, and
- the share of peer reporters about the target in that epoch that report `PORT_STATE_CLOSED` for port index `i` meets or exceeds `peer_port_postpone_threshold_percent`.

## Recovery rule (POSTPONED → ACTIVE)

In a single epoch, a `POSTPONED` supernode becomes `ACTIVE` if:
- it submits one compliant self report (host requirements), and
- there exists at least **1** peer report about that supernode in the same epoch where **all** required ports are `PORT_STATE_OPEN`.
