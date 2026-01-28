# Postponement Rules (Draft)

Minimal rules for switching a supernode between `ACTIVE` and `POSTPONED`, based on `MsgSubmitAuditReport` + `WindowSnapshot`.

1) **Enforcement timing**: rules are evaluated only at **window end** (`window_end_height`).

2) **Gating location**: peer assignment/target gating is enforced only when `MsgSubmitAuditReport` is accepted. Enforcement assumes stored peer observations already passed gating.

3) **Self host metrics → POSTPONE**: if a self report violates any enabled minimum free% threshold, postpone.
   - Params: `min_cpu_free_percent`, `min_mem_free_percent`, `min_disk_free_percent` (free% = `100 - usage%`).
   - Special case: if `*_usage_percent == 0`, treat as **unknown** and skip any action for that metric.

4) **Self non-rules (for now)**: ignore `failed_actions_count` and ignore `inbound_port_states` (self ports reflect inbound traffic, not reachability).

5) **Peer ports → POSTPONE**: for any required port index `i`, if peers unanimously report that port as `CLOSED` for `consecutive_windows_to_postpone` consecutive windows, postpone.
   - Param: `consecutive_windows_to_postpone` (default `1`).
   - A window counts only if there are **at least 2** distinct peer reporters about the target in that window, and **all** of them report `port_states[i] == PORT_STATE_CLOSED`.

6) **Recovery (POSTPONED → ACTIVE)**: in a single window, require:
   - one compliant self report (per rule 3), and
   - one compliant peer report: **at least 2** distinct peer reporters about the target in that window, and **all** required ports unanimously `OPEN`.
