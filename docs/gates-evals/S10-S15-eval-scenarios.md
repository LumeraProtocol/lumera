# Evaluation Scenarios -- Everlight Phase 1
Generated: 2026-04-10T02:12:01Z
Project: lumera
Slices: S10-S15
Features: F10, F11, F12, F13, F14, F15, F16, F18
Gate Result: PASS (2026-04-09)

## How to Use

1. Build the chain: `make build`
2. Start the local environment: `make devnet-clean && make devnet-new-no-hermes && make devnet-up`
3. Run the scripted smoke coverage first: `make devnet-tests-everlight`
4. Execute each scenario step-by-step on the running devnet.
5. Record results in the checklists.
6. Fill the feedback form at the bottom.
7. Feed results back with `$bridge-feedback`.

## Scripted Coverage

`devnet/tests/everlight/everlight_test.sh` currently automates:
- Scenario 1 end-to-end bootstrap checks, including module-account funding and pool-balance verification
- Scenario 2 STORAGE_FULL transition checks on a live registered SuperNode, including recovery preparation and disk-usage-triggered transition
- Scenario 3 distribution happy path on devnet with two SuperNodes at 2 GiB and 4 GiB, funded pool, and payout ordering verification
- Scenario 4 distribution edge coverage for STORAGE_FULL payout eligibility and below-threshold exclusion
- Scenario 6 parameter presence checks for registration-fee routing
- Scenario 7 governance parameter update flow, including proposal submission, voting, and final param verification
- Scenario 8 embedded params/genesis checks, plus live supernode/metrics checks when a registered SuperNode is present

`devnet/tests/everlight/everlight_test.sh` still leaves as manual or environment-dependent:
- Scenario 5, which needs multi-period metrics history to validate ramp-up, smoothing, and growth-cap behavior credibly
- Scenario 9, which needs a pre-Everlight upgrade path
- Scenario 10, which needs the full funding + action + multi-period lifecycle

## Latest Devnet Run

Source: `everlight-devnet-tests.log`

- Executed on connected devnet at block height 1814
- Results: 34 PASS, 0 FAIL, 3 SKIP
- Executed coverage: Scenarios 1, 2, 3, 4, 6, 7, 8
- Skipped coverage: Scenario 5 (anti-gaming guardrails), Scenario 9 (upgrade idempotency), Scenario 10 (full lifecycle)

This confirms live devnet validation for the core embedded-supernode Everlight flows. The remaining skipped scenarios are follow-up coverage items, not evidence of regression in the scenarios that were executed.

---

## Scenario 1: [F14, F18] Embedded Everlight Surface Bootstraps Cleanly

**Goal:** Verify the v1.15.0 upgrade exposes Everlight functionality through the embedded `supernode` API surface rather than a standalone module.
**Preconditions:** Fresh devnet or upgraded chain with the current binary.
**Linked:** F11, F14, F18, AT34, AT41, AT42, AT43

### Steps:
1. Query supernode params:
   `lumerad query supernode params`
   -> Expected: `reward_distribution` is present with non-zero defaults and `max_storage_usage_percent` is present.

2. Query pool state through supernode:
   `lumerad query supernode pool-state`
   -> Expected: Returns current pool balance, `last_distribution_height`, `total_distributed`, and `eligible_sn_count`.

3. Check the module account:
   `lumerad query auth module-account everlight`
   -> Expected: The `everlight` account exists and has no Minter/Burner/Staking permissions.

4. Send funds to the module account:
   `lumerad tx bank send <from> <everlight-module-addr> 1000000ulume --yes`
   -> Expected: Transaction succeeds and pool balance increases.

5. Re-query pool state:
   `lumerad query supernode pool-state`
   -> Expected: Balance reflects the transfer.

### Checklist:
- [ ] `reward_distribution` is exposed through `supernode params`
- [ ] `supernode pool-state` returns valid values
- [ ] `everlight` module account exists with restricted permissions
- [ ] Pool funding via `MsgSend` succeeds
- [ ] Pool balance updates are visible through `supernode pool-state`

---

## Scenario 2: [F12, F13] STORAGE_FULL State Transitions

**Goal:** Verify storage-only violations transition a SuperNode to `STORAGE_FULL`, while mixed violations still result in `POSTPONED`.
**Preconditions:** Running devnet with at least one registered SuperNode. `max_storage_usage_percent` set to a testable value.
**Linked:** F12, F13, AT30, AT31, AT32, AT33

### Steps:
1. Verify `max_storage_usage_percent` is set to a testable threshold (default 90).
   -> Expected: Param is queryable via `lumerad query supernode params`.

2. Register a SuperNode and confirm it starts as `ACTIVE`.
   -> Expected: `lumerad query supernode get-super-node <validator-address>` shows `ACTIVE`.

3. Report metrics with `cascade_kademlia_db_bytes` above the threshold and all other checks healthy.
   -> Expected: Node transitions to `STORAGE_FULL`.

4. Query node state again.
   -> Expected: State is `STORAGE_FULL`, not `POSTPONED`.

5. Lower reported storage usage below the threshold.
   -> Expected: Node recovers to `ACTIVE`.

6. Report storage violation plus one additional compliance problem.
   -> Expected: Node transitions to `POSTPONED`.

### Checklist:
- [ ] Storage-only violation produces `STORAGE_FULL`
- [ ] `STORAGE_FULL` is distinct from `POSTPONED`
- [ ] Recovery to `ACTIVE` works
- [ ] Mixed violations still produce `POSTPONED`
- [ ] State transitions are queryable and observable

---

## Scenario 3: [F15] Periodic Distribution Happy Path

**Goal:** Verify EndBlocker distributes the pool proportionally to eligible SuperNodes by reported Cascade bytes.
**Preconditions:** Devnet with at least two eligible SuperNodes, funded pool, small `payment_period_blocks`.
**Linked:** F15, AT35, AT36, AT44, AT45

### Steps:
1. Prepare SN-A with 2 GB and SN-B with 4 GB of `cascade_kademlia_db_bytes`.
2. Set:
   - `payment_period_blocks = 10`
   - `min_cascade_bytes_for_payment = 1073741824`
   - `new_sn_ramp_up_periods = 1`
3. Fund the pool with `10000000ulume`.
4. Advance blocks until the distribution period triggers.
5. Query both account balances.
   -> Expected: SN-A receives about 1/3 of the payout, SN-B about 2/3.
6. Query pool state:
   `lumerad query supernode pool-state`
   -> Expected: Pool is near zero except for truncation dust and `last_distribution_height` is updated.

### Checklist:
- [ ] Distribution triggers at the configured period
- [ ] Payout ratios follow reported storage weight
- [ ] Pool drains as expected
- [ ] `last_distribution_height` updates
- [ ] No unexpected errors or panics occur

---

## Scenario 4: [F15] Distribution Edge Cases

**Goal:** Verify zero-balance, no-eligible-node, and below-threshold cases are safe and deterministic.
**Preconditions:** Devnet with configurable SuperNode set.
**Linked:** F15, AT36, AT44, AT45

### Steps:
1. Trigger a distribution period with zero pool balance.
   -> Expected: No panic, no payout.

2. Fund the pool with no eligible SuperNodes.
   -> Expected: Pool remains intact, no payout, no panic.

3. Register one node below `min_cascade_bytes_for_payment`.
   -> Expected: Node is excluded from payout.

4. Mix one eligible and one ineligible node.
   -> Expected: Only the eligible node receives payment.

5. Mix one `ACTIVE` and one `STORAGE_FULL` eligible node.
   -> Expected: Both remain payout-eligible.

### Checklist:
- [ ] Zero pool behaves safely
- [ ] No eligible nodes behaves safely
- [ ] Below-threshold nodes are excluded
- [ ] `STORAGE_FULL` nodes still receive payouts
- [ ] Observed behavior matches keeper tests

---

## Scenario 5: [F15] Anti-Gaming Guardrails

**Goal:** Verify ramp-up, smoothing, and growth-cap logic affect payout weight as intended.
**Preconditions:** Devnet with multiple distribution periods and controllable metrics reports.
**Linked:** F15, AT37, AT38

### Steps:
1. Register a new SuperNode with large storage and set `new_sn_ramp_up_periods = 4`.
   -> Expected: Weight ramps over successive periods instead of immediately reaching full value.

2. Set `usage_growth_cap_bps_per_period = 5000` and increase a node from 2 GB to 20 GB in one period.
   -> Expected: Effective weight increases only by the configured cap.

3. Set `measurement_smoothing_periods = 4` and report oscillating storage values.
   -> Expected: Effective distribution weight reflects smoothing, not raw spikes.

### Checklist:
- [ ] Ramp-up reduces early payouts for new nodes
- [ ] Growth cap limits sudden weight jumps
- [ ] Smoothing dampens noisy reports
- [ ] Behavior is stable across multiple periods

---

## Scenario 6: [F16] Registration Fee Share Routing

**Goal:** Verify action registration fees contribute the configured share to the Everlight pool.
**Preconditions:** Devnet with action flow available and at least one registered SuperNode.
**Linked:** F16, AT39

### Steps:
1. Set `registration_fee_share_bps = 500`.
2. Record the pool balance:
   `lumerad query supernode pool-state`
3. Submit and finalize a Cascade action with a known fee.
4. Query the pool balance again.
   -> Expected: Pool increases by 5% of the registration fee.

5. Repeat with `registration_fee_share_bps = 0`.
   -> Expected: Pool does not increase from the fee path.

6. Repeat with `registration_fee_share_bps = 10000`.
   -> Expected: Full fee share routes to the pool.

### Checklist:
- [ ] Configured fee share reaches the pool
- [ ] `0` bps disables routing
- [ ] `10000` bps routes the full configured share
- [ ] Routing occurs on finalized action flow, not just on config change

---

## Scenario 7: [F11, F14] Governance and Param Controls

**Goal:** Verify the embedded Everlight parameters are governed through `supernode` param updates.
**Preconditions:** Running devnet with governance authority available.
**Linked:** F11, F14, AT41

### Steps:
1. Update each `reward_distribution` field through the authorized path.
   -> Expected: Updates are accepted and visible in `lumerad query supernode params`.

2. Attempt the same update from a non-authority address.
   -> Expected: Request is rejected.

3. Submit invalid values such as `payment_period_blocks = 0`.
   -> Expected: Validation rejects the update.

### Checklist:
- [ ] All embedded Everlight params are governable
- [ ] Unauthorized senders are rejected
- [ ] Invalid values are rejected

---

## Scenario 8: [F10, F11] Embedded Proto and Genesis Compatibility

**Goal:** Verify the proto surface and exported genesis reflect the embedded `supernode` design.
**Preconditions:** Built binary and running chain.
**Linked:** F10, F11, AT30, AT41

### Steps:
1. Query a SuperNode and inspect state values.
   -> Expected: `SUPERNODE_STATE_STORAGE_FULL` appears in query responses when applicable.

2. Query metrics for a registered node:
   `lumerad query supernode get-metrics <validator-address>`
   -> Expected: `cascade_kademlia_db_bytes` is present.

3. Export genesis:
   `lumerad genesis export | jq '.app_state.supernode.params.reward_distribution, .app_state.supernode.last_distribution_height'`
   -> Expected: Embedded Everlight distribution state exists under `app_state.supernode`, not `app_state.everlight`.

### Checklist:
- [ ] `STORAGE_FULL` is visible through the supernode API
- [ ] `cascade_kademlia_db_bytes` is visible in metrics output
- [ ] Embedded Everlight state exports under `app_state.supernode`

---

## Scenario 9: [F18] Upgrade Handler Idempotency

**Goal:** Verify the upgrade path initializes embedded Everlight state without disturbing pre-existing chain data.
**Preconditions:** Pre-upgrade chain state with SuperNodes and actions.
**Linked:** F18, AT42, AT43

### Steps:
1. Start from pre-Everlight state and perform the v1.15.0 upgrade.
2. Query existing SuperNodes.
   -> Expected: Existing states are preserved.

3. Query existing actions.
   -> Expected: Existing action state and metadata are preserved.

4. Query `supernode params` and `supernode pool-state`.
   -> Expected: Embedded Everlight defaults are initialized and queryable.

### Checklist:
- [ ] Existing SuperNode states survive upgrade
- [ ] Existing actions survive upgrade
- [ ] Embedded Everlight defaults are initialized
- [ ] Upgraded chain exposes the new `supernode` query surface cleanly

---

## Scenario 10: [Cross-Feature] Full Lifecycle from Funding to Payout

**Goal:** Exercise the full user-visible lifecycle from foundation funding through fee routing and payout distribution.
**Preconditions:** Clean devnet with multiple SuperNodes.
**Linked:** F12, F14, F15, F16

### Steps:
1. Register three SuperNodes with different `cascade_kademlia_db_bytes`.
2. Send foundation funds to the `everlight` module account.
3. Finalize a Cascade action so fee share is routed to the pool.
4. Confirm pool balance reflects both funding sources:
   `lumerad query supernode pool-state`
5. Force one eligible node into `STORAGE_FULL`.
6. Advance until distribution occurs.
7. Verify all eligible nodes, including the `STORAGE_FULL` node, receive payout.
8. Lower one node below `min_cascade_bytes_for_payment` and trigger the next period.
   -> Expected: That node stops receiving payouts.

### Checklist:
- [ ] Foundation funding works
- [ ] Registration fee share augments the pool
- [ ] `STORAGE_FULL` nodes stay payout-eligible
- [ ] Below-threshold nodes become payout-ineligible
- [ ] Multi-period lifecycle behaves consistently

---

# Feedback Form

## Overall Assessment
- [ ] Ready for merge/launch
- [ ] Minor fixes needed
- [ ] Major fixes needed

## Ratings (1-5)
| Area | Rating | Notes |
|------|--------|-------|
| Correctness | ___ | |
| Query UX | ___ | |
| Upgrade Confidence | ___ | |
| Event / Error Clarity | ___ | |
| Operator Experience | ___ | |
| Performance | ___ | |

## Issues Found
| # | Severity | Feature | Scenario | Description | Steps to Reproduce |
|---|----------|---------|----------|-------------|-------------------|
| 1 | | | | | |
| 2 | | | | | |
| 3 | | | | | |

## DX Friction Points
(CLI friction, confusing output, missing flags, unclear setup, etc.)

## Security Observations
(Module-account handling, payout correctness, anti-gaming confidence, etc.)

## Suggestions for Improvement
(Free form)

## Would you approve this for merge? Why or why not?
(Free form)
