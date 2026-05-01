# S10–S15 Eval — Script Coverage Analysis

Analyzing `docs/gates-evals/S10-S15-eval-scenarios.md` against
`devnet/tests/everlight/everlight_test.sh`.

**Original purpose** (pre-approval): discussion document for which gaps to
close in the script. Two edits were planned and **landed**:

1. Annotated `S10-S15-eval-scenarios.md` with inline tags per step + checklist
   markers per item.
2. Extended `everlight_test.sh` (1595 → 1989 LOC) with all approved AUTO+ /
   AUTO-NEW assertions.

**Subsequent purpose** (this revision): durable reference describing what the
script does cover, what it does not, and **why** every uncovered eval step is
left out.

---

## 0. Status (2026-04-26)

### Script changes landed

All AUTO+ and AUTO-NEW additions from §4 have been merged into
`devnet/tests/everlight/everlight_test.sh`:

| ID | Status |
|---|---|
| S1.2a–d (pool-state subfields) | ✅ scripted |
| S1.3c (embedded supernode module-account permissions) | ✅ scripted |
| S1.5 / S1.5b / S1.5c (no x/everlight, export check) | ✅ scripted |
| S3.6c (pool drain) | ✅ scripted |
| S4.4 (mixed eligible/ineligible filter) | ✅ scripted |
| S7.8 (invalid value rejected — `registration_fee_share_bps=20000`) | ✅ scripted |
| S7.9 (bogus authority rejected) | ✅ scripted |
| S8.2 / S8.2a (SUPERNODE_STATE_* enum literals) | ✅ scripted |
| S8.3 (runtime no-x/everlight check) | ✅ scripted |
| S11.0–S11.2 (F13 mixed-violation invariant — best-effort devnet dual) | ✅ scripted; may skip as inconclusive |

> **Note on eval Scenario 2 step 5** (mixed → POSTPONED): not directly
> exercisable on devnet because (a) host-metric thresholds
> (`min_*_free_percent`) are 0 at devnet genesis, and (b) audit ignores
> `failed_actions_count`. S11.2 attempts a best-effort live dual, but may
> skip as inconclusive under epoch/recovery timing. The original direction is
> covered by Go tests.

> **Note on eval Scenario 7 step 5** (`payment_period_blocks=0`): cannot be
> tested via gov proposal because `WithDefaults()` rewrites a literal `0` to
> the default before `Validate()` runs. The script uses
> `registration_fee_share_bps=20000` (>10000 max) instead, which reaches
> `Validate()` and fails as required.

### Go-test coverage — all PASS

Per current gate evidence (2026-04-28 gate report top of eval doc), every
COVERED-ELSEWHERE item in this analysis is verified by a green Go test run:

| Eval scenario step | Go test path | Status |
|---|---|---|
| S2 step 5 (mixed-violation bifurcation) | `x/audit/v1/keeper/enforcement_storagefull_transition_test.go`, `msg_submit_epoch_report_storagefull_test.go` | ✅ PASS |
| S4 step 1 (zero pool, no panic) | `x/supernode/v1/keeper/distribution_test.go` | ✅ PASS |
| S4 step 2 (no eligible SNs, no panic) | `x/supernode/v1/keeper/distribution_test.go`, `tests/integration/everlight` | ✅ PASS |
| S5 step 1 (ramp-up reduces early payouts) | `x/supernode/v1/keeper/distribution_test.go`, `tests/e2e/everlight` | ✅ PASS |
| S5 step 3 (smoothing oscillation) | `x/supernode/v1/keeper/distribution_test.go` | ✅ PASS |
| S5 step 4 (cross-check vs keeper) | `x/supernode/v1/keeper/distribution_test.go` | ✅ PASS |
| S6 steps 3–6 (Cascade fee routing chain-side) | `tests/integration/action`, `x/supernode/v1/keeper/fee_routing_test.go` | ✅ PASS |
| S9 (upgrade idempotency) | `app/upgrades/v1_12_0/upgrade_test.go` | ✅ PASS |

Devnet smoke / E2E:
- `make devnet-tests-everlight` last recorded: 35 PASS / 0 FAIL / 5 SKIP; a later eval run found the old S1.3c permissionless-account assertion stale because Phase 1 uses the existing `supernode` module account
- `tests/e2e/everlight/everlight_e2e_test.go` — ✅ PASS

The eval-doc checklists have been updated so that items with exclusive
Go-test coverage are pre-marked as DONE; items with script coverage stay
unchecked until the operator runs `make devnet-tests-everlight`.

---

## 1. Coverage classification (per scenario step)

**Legend**

| Tag | Meaning |
|---|---|
| **AUTO** | Already covered by `everlight_test.sh` — annotate inline in eval doc |
| **AUTO+** | Step is partly covered; small targeted addition to script gives full coverage |
| **AUTO-NEW** | Not covered today, but cheap to add — only chain CLI |
| **NEEDS-FIXTURE** | Could be scripted, but only if devnet provides a missing piece (cascade flow, fresh SN registration, pre-upgrade genesis) |
| **MANUAL** | Should remain in operator manual checklist (UX, ergonomics, observability) |
| **COVERED-ELSEWHERE** | Already validated by Go tests — script should not duplicate; eval doc should reference |

### Scenario 1 — Module Bootstrap (F14, F18)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. supernode params shows reward_distribution | `S1.1`, `S1.1a`, `S1.1b`, `S1.4` | AUTO |
| 2. pool-state returns balance/last_distribution_height/total_distributed/eligible_sn_count | `S1.2` (asserts non-empty; doesn't assert each subfield) | AUTO+ — tighten S1.2 to `jq` each subfield |
| 3. auth module-account for embedded pool | `S1.3` queries `module-account supernode` because the embedded pool lives in the existing supernode module account | AUTO — assert account name/permissions shape matches the Phase 1 embedded-pool design; do not require a permissionless account |
| 4. send funds → pool balance increases | `S1.3a`, `S1.3b` | AUTO |
| 5. export genesis → state under `app_state.supernode`, not `app_state.everlight` | none | AUTO-NEW (`lumerad export` + jq `app_state | has("everlight")` == false) |

### Scenario 2 — STORAGE_FULL transitions (F12, F13)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. params shows max_storage_usage_percent | `S2.2` (also S1.4) | AUTO |
| 2. high-disk audit report → STORAGE_FULL | `S2.3`, `S2.4` | AUTO |
| 3. query node → STORAGE_FULL | implicit in `wait_for_supernode_state` for S2.4 | AUTO |
| 4. healthy report → recovers to ACTIVE | `S2.5`, `S2.6` | AUTO |
| 5. high-disk + other compliance violation → POSTPONED | none — script never injects a non-storage violation | AUTO-NEW (submit report with both `disk_usage_percent > max` AND `failed_actions_count > 0`; assert state == `SUPERNODE_STATE_POSTPONED`) |
| 6. legacy report-supernode-metrics with high disk does not mutate state | `S2.7`, `S2.8` | AUTO |

### Scenario 3 — Periodic Distribution Happy Path (F15)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. ≥2 SNs with different cascade_kademlia_db_bytes | `S3.0`, `S3.1`, `S3.2` (2 GiB / 4 GiB) | AUTO |
| 2. set short payment_period_blocks via gov | done by Scenario 7 prior to S3 (proposal sets `payment_period_blocks=2`) | AUTO (cross-scenario dependency) — annotate this in eval doc |
| 3. fund pool | `S3.3` | AUTO |
| 4. last_distribution_height advances | `S3.4` (`wait_for_distribution_height_change`) | AUTO |
| 5. payouts proportional to weight | `S3.5`, `S3.6`, `S3.7` | AUTO |
| 6. pool balance decreases by distributed amount | not asserted directly | AUTO+ — capture `pool-state.pool_balance` before/after S3.4, assert `before > after` (modulo new fee-share inflows) |

### Scenario 4 — Distribution Edge Cases (F15)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. zero pool, period triggers, no payout/panic | none | AUTO-NEW — Tricky: pool is auto-funded by Scenario 1 + 3. Either run Scenario 4 first on a fresh devnet, or wait for distribution to drain pool, then advance N blocks and assert no panic + `last_distribution_height` either stays put or advances cleanly. NEEDS-FIXTURE if drain is unreliable |
| 2. funded pool, no eligible SNs | none — all 5 devnet SNs are kept eligible to support S3 | NEEDS-FIXTURE — would require briefly forcing all SNs to POSTPONED (e.g., skipping every report for one full epoch). High flake risk; recommend leaving as MANUAL or asserting via Go integration tests in `tests/integration/everlight` |
| 3. node below `min_cascade_bytes_for_payment` excluded | `S4.3` | AUTO |
| 4. mixed eligible/ineligible | partial: S3 (eligible payout) + S4.3 (ineligible exclusion) — not in same run | AUTO+ — at end of S3, after distribution, assert below-threshold SN's balance unchanged |
| 5. STORAGE_FULL remains payout-eligible | `S4.1` | AUTO |

### Scenario 5 — Anti-Gaming Guardrails (F15)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. ramp-up reduces early payouts | none — SNs are registered at devnet start, ramp window already past; `new_sn_ramp_up_periods=1` after S7 anyway | NEEDS-FIXTURE — would need to register a new SN mid-script, force `new_sn_ramp_up_periods≥3`, and observe partial weight over ≥2 distribution periods. Doable but adds 1–2 minutes; risky on flake. Recommend: cover via Go E2E (`tests/e2e/everlight`) and keep manual eval pointer |
| 2. growth cap clamps sudden weight jump | `S5.4` (smoothed < raw) | AUTO |
| 3. smoothing dampens oscillation across 4-period window | partial: 2-point bump only (S5.2 / S5.3); doesn't oscillate | AUTO+ if we keep `measurement_smoothing_periods=1` we cannot show smoothing dampening — the param is set to 1 in S7. To show real smoothing we'd need to gov-set it to ≥3, then submit 3+ alternating reports. Adds ~30s per epoch × 3 epochs. NEEDS-FIXTURE if we don't want to extend devnet runtime |
| 4. cross-check vs keeper/E2E | n/a | COVERED-ELSEWHERE — `go test ./x/supernode/v1/keeper/distribution_test.go` |

### Scenario 6 — Registration Fee Share (F16)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. registration_fee_share_bps configured | `S6.1`, `S6.2` | AUTO |
| 2. record pool balance | trivially scriptable | AUTO-NEW |
| 3. submit + finalize Cascade action with known fee | none | NEEDS-FIXTURE — requires the cascade flow which depends on supernode containers serving Cascade requests. `make devnet-new-no-hermes` does start `supernova_supernode_*` services but exercising the full action flow needs the supernode binary to be present (operator concern: missing supernode binaries in devnet/bin would skip this) |
| 4. pool increases by configured share | depends on step 3 | NEEDS-FIXTURE |
| 5. set bps=0, repeat | depends on step 3 | NEEDS-FIXTURE |
| 6. high bps, repeat | depends on step 3 | NEEDS-FIXTURE |

> **Recommendation for S6:** add a NEW S6.3 stub that *attempts* to submit a Cascade action and `skip` cleanly if supernode service isn't available. When the cascade flow runs, S6.3+ asserts pool delta == fee × bps / 10000.

### Scenario 7 — Governance (F11, F14)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. submit gov proposal updating reward_distribution | `S7.2` | AUTO |
| 2. validators vote, proposal passes | `S7.4`, `S7.5` | AUTO |
| 3. updated value visible | `S7.6` | AUTO |
| 4. unauthorized direct update rejected | none | AUTO-NEW — `lumerad tx supernode update-params --from <non-gov-key>` and assert tx fails with "unauthorized" |
| 5. invalid value (e.g. payment_period_blocks=0) rejected | none | AUTO-NEW — submit proposal with invalid value, vote yes, assert either submission fails at validate-basic OR proposal status ends as FAILED with execution error |

### Scenario 8 — Proto / API Compatibility (F10, F11)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. SUPERNODE_STATE_STORAGE_FULL appears for storage-full SN | implicitly covered by S2.4 (`wait_for_supernode_state ... SUPERNODE_STATE_STORAGE_FULL`) | AUTO |
| 2. cascade_kademlia_db_bytes in reward eligibility query | `S8.1c` | AUTO |
| 3. cascade_kademlia_db_bytes in metrics query | `S8.1c` legacy fallback path | AUTO |
| 4. SUPERNODE_STATE_* enum names preserved (no rename) | none — the script never asserts the canonical literals are present in any output | AUTO-NEW — `lumerad query supernode list-supernodes -o json` and assert at least one state literal matches `^SUPERNODE_STATE_[A-Z_]+$` (cheap regex check); also explicitly grep for `SUPERNODE_STATE_ACTIVE` / `SUPERNODE_STATE_STORAGE_FULL` |
| 5. embedded Everlight state under `app_state.supernode` on export | none | AUTO-NEW — same `lumerad export` we add for S1.5 |

### Scenario 9 — Upgrade Handler Idempotency (F18)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1–5. pre-Everlight chain → upgrade → state preserved | `scenario_stubs` skip | COVERED-ELSEWHERE — `go test ./app/upgrades/v1_12_0 -count=1` already in gate evidence. Live devnet upgrade testing is supported by the existing `make devnet-upgrade-1101` chain (v1.7.2→…→v1.10.1→v1.12.0); see Makefile.devnet:303-330. **Recommendation:** keep `everlight_test.sh` skip for Scenario 9, and instead add a **separate** companion script (out of scope for this task) or a manual playbook step that documents `devnet-upgrade-1101 && devnet-upgrade-<v1_12_0_target>` |

### Scenario 10 — Full Lifecycle (Cross-Feature)

| Eval step | Script function/labels | Tag |
|---|---|---|
| 1. multiple SNs different bytes | covered by S3 | AUTO |
| 2. foundation funding | covered by S1.3a | AUTO |
| 3. registration fee share via Cascade action | NEEDS-FIXTURE (same as S6.3) | NEEDS-FIXTURE |
| 4. pool reflects both funding sources | depends on step 3 | NEEDS-FIXTURE |
| 5. force one node to STORAGE_FULL | covered by S2 | AUTO |
| 6. distribution occurs | covered by S3.4 | AUTO |
| 7. STORAGE_FULL still receives payout | covered by S4.1 + S3 | AUTO |
| 8. drop below threshold → no payout next period | partially covered by S4.3 | AUTO+ — assert the same SN's balance does NOT change across one further distribution period |

> **Recommendation for S10:** keep stub. Most of its substeps are already exercised by Scenarios 1–5, 8 in this run. Document this in eval doc as "Scenario 10 = end-to-end composition; covered by sequential execution of Scenarios 1, 3, 4, 5".

---

## 2. Out-of-script — by category

Every eval-doc step that the script does NOT execute, broken down by
**why**. Ordered by category to make audit easy.

### 2A. NEEDS-FIXTURE — root: missing devnet capability

These are the *root* gaps. Anything tagged "depends on …" in §2B inherits
its NEEDS-FIXTURE status from one of these.

| Eval step | Missing capability | Where it IS verified |
|---|---|---|
| **S6 step 3** — submit + finalize a Cascade action with a known fee | Requires the supernode service to actually serve Cascade requests. `devnet/bin/supernode` is operator-supplied; script cannot assume its presence. | `tests/integration/action/lep5_integration_test.go`, `x/supernode/v1/keeper/fee_routing_test.go` ✅ |
| **S10 step 3** — finalize Cascade action so registration fee share routes to the pool | Same as S6 step 3. | Same as above ✅ |
| **S9 steps 1–4** — pre-Everlight chain → upgrade → state preserved | Requires multi-binary upgrade rehearsal (v1.7.2 → … → v1.10.1 → v1.12.0). Existing Make targets (`devnet-upgrade-1101`) drive this externally; composing them inside `everlight_test.sh` is brittle. | `app/upgrades/v1_12_0/upgrade_test.go` ✅ |
| **S4 step 1** — zero-pool distribution, no panic | Devnet pool is auto-funded by Scenarios 1 + 3 + 4. A deterministic zero-pool window is not stable to script. | `x/supernode/v1/keeper/distribution_test.go` ✅ |
| **S4 step 2** — funded pool with no eligible SNs | Forcing all 5 devnet SNs into POSTPONED simultaneously is a flake source. | `x/supernode/v1/keeper/distribution_test.go`, `tests/integration/everlight` ✅ |
| **S5 step 1** — new-SN ramp-up reduces early payouts | Requires mid-script SN registration and ≥2 distribution periods of timing-sensitive observation. High flake risk. | `x/supernode/v1/keeper/distribution_test.go`, `tests/e2e/everlight` ✅ |
| **S5 step 3** — smoothing dampens oscillation across 4-period window | Would require a second gov proposal to set `measurement_smoothing_periods≥3` and 3+ alternating reports across 3 epochs. Operator deferred to keep script runtime bounded. | `x/supernode/v1/keeper/distribution_test.go` ✅ |
| **S2 step 5** — single audit report with mixed violations → POSTPONED | Devnet genesis has all `min_*_free_percent = 0` (host-metric thresholds disabled), and audit ignores `failed_actions_count`. No single report can produce a non-storage compliance violation. | `x/audit/v1/keeper/enforcement_storagefull_transition_test.go`, `msg_submit_epoch_report_storagefull_test.go` ✅ — script attempts a best-effort dual via S11.2 and skips if live epoch timing makes it inconclusive |

### 2B. Cascaded — out-of-script *because* a precondition is NEEDS-FIXTURE

These steps could each be cheaply scripted in isolation, but the assertion
they make is only meaningful after a NEEDS-FIXTURE step (§2A) has run.
Therefore they are deferred to the same Go-test coverage that satisfies
their parent.

| Eval step | Precondition (from §2A) | Where it IS verified |
|---|---|---|
| **S6 step 2** — record pool balance | depends on S6 step 3 (Cascade action) being meaningful | n/a — pure observation, only meaningful in concert |
| **S6 step 4** — pool increases by configured fee share | depends on S6 step 3 | `x/supernode/v1/keeper/fee_routing_test.go` ✅ |
| **S6 step 5** — `bps=0` disables routing | depends on S6 step 3 | `x/supernode/v1/keeper/fee_routing_test.go` ✅ |
| **S6 step 6** — high `bps` increases routed share | depends on S6 step 3 | `x/supernode/v1/keeper/fee_routing_test.go` ✅ |
| **S10 step 4** (partial — fee-share component) — pool reflects both funding sources | foundation portion is scripted (S1.3a/b); fee-share portion depends on S10 step 3 | Foundation portion: scripted. Fee-share portion: `tests/integration/action`, `fee_routing_test.go` ✅ |
| **S9 step 2** — existing SuperNodes preserved across upgrade | depends on S9 step 1 (upgrade rehearsal) | `app/upgrades/v1_12_0/upgrade_test.go` ✅ |
| **S9 step 3** — existing actions preserved across upgrade | depends on S9 step 1 | `app/upgrades/v1_12_0/upgrade_test.go` ✅ |
| **S9 step 4** — embedded Everlight defaults initialized post-upgrade | depends on S9 step 1; the *post-upgrade* surface IS scripted via S1.1/S1.2 against today's running chain | `app/upgrades/v1_12_0/upgrade_test.go` ✅ + S1.x (post-upgrade) |

### 2C. MANUAL — pure human-judgment items

| Item | Reason it stays manual |
|---|---|
| Operator CLI ergonomics (helpful error messages, output readability) | Subjective; pass/fail criterion is human judgment |
| Multi-period stability observation in Scenario 5 ("Behavior is stable across multiple periods") | Long-window observation across many distribution periods; only valuable when an operator is watching for unexpected drift, not as a binary assertion |
| Cross-check vs keeper/E2E test outputs (S5 step 4) | Subjective comparison narrative; the underlying invariants ARE covered by Go tests ✅ |
| `buf lint SUPERNODE_STATE_*` waiver verification | Build-time concern, not runtime; gate report owns this — already documented as a warning in the eval-doc gate context |

### 2D. Summary — what coverage gaps actually remain after this round

After landing the AUTO+ / AUTO-NEW set and accepting Go-test coverage as
authoritative for the categories above:

- **Zero true gaps in chain-logic verification.** Every F-feature behavior has
  either script coverage on devnet or Go-test coverage in PASS state.
- **The only operator-attended verifications** are the S6/S10 Cascade-fee
  routing steps (manual when the supernode service is available) and the S9
  pre-upgrade rehearsal (already supported by `make devnet-upgrade-1101`).
- **All MANUAL items** are either subjective UX judgments or stability
  observations — not invariants that can be expressed as a binary
  assertion.

---

## 3. Proposed eval-doc annotation style

For each step in `S10-S15-eval-scenarios.md`, append an inline tag right after
the step text. Example for Scenario 1, step 4:

```
4. Send funds to the module account -> Expected: transaction succeeds and pool
   balance increases. **[AUTO: scenario_1_module_bootstrap S1.3a/S1.3b]**
```

And for additions:

```
5. Export genesis -> Expected: Everlight state lives under `app_state.supernode`,
   not `app_state.everlight`. **[AUTO-NEW: planned addition S1.5]**
```

Manual-only items get **[MANUAL]** with a one-line reason.

The "Scripted Coverage" section at the top of the eval doc will be rewritten to
list the assertions covered today + the assertions added in this round.

---

## 4. Concrete script additions proposed (cheap set)

In priority order. All use lumerad CLI only, no new binaries.

| New | Where | What | LOC est. |
|---|---|---|---|
| **S1.5** | `scenario_1_module_bootstrap` end | `lumerad export | jq` — assert `.app_state | has("everlight") == false` and `.app_state.supernode.everlight_state` (or wherever the embedded state sits) is non-null | ~25 |
| **S1.3 embedded-pool permissions** | inside S1.3 | assert the queried account is `supernode` and exposes a permissions array matching the existing-module-account design | ~5 |
| **S1.2 tighten** | inside S1.2 | assert each of `pool_balance`, `last_distribution_height`, `total_distributed`, `eligible_sn_count` exists | ~10 |
| **S2.5b** | `scenario_2_storage_full_transition` after S2.6 | submit audit report with disk_usage>max AND failed_actions_count>0; assert state → POSTPONED | ~30 |
| **S3.6c** | end of `scenario_3_periodic_distribution_happy_path` | capture pool_balance before S3.3 funding, after S3.4; assert distribution drained the funded amount minus dust (and minus any concurrent fee-share inflow) | ~20 |
| **S4.4** | `scenario_4_distribution_edge_cases` | assert below-threshold SN's bank balance unchanged after one full distribution period | ~20 |
| **S7.8** | `scenario_7_governance` after S7.6 | submit a proposal with `payment_period_blocks=0`; assert it either fails validation or executes with FAILED status | ~40 |
| **S7.9** | `scenario_7_governance` end | direct `tx supernode update-params --from <non-gov-key>`; assert tx code != 0 with "unauthorized" in raw_log | ~20 |
| **S8.2** | `scenario_8_proto_compatibility` end | assert at least one `SUPERNODE_STATE_*` literal present in `list-supernodes` JSON; explicitly grep for `SUPERNODE_STATE_ACTIVE`/`STORAGE_FULL` | ~15 |
| **S8.3** | inside S8 (reuse S1.5 export) | call `lumerad export` once at the end; assert no `app_state.everlight` and yes `app_state.supernode` | ~10 (shares logic with S1.5) |

**Total: ~9 new assertions, ~195 LOC.** No new external dependencies. Should
not measurably slow the script (>4 of these are pure-query, the others reuse
existing helpers).

### Deferred (NEEDS-FIXTURE — not part of this round)

- **S5.5b smoothing oscillation:** add only if we can extend governance step S7
  to also set `measurement_smoothing_periods=3`. Trade-off: extends script
  runtime by ~1–2 minutes for 3 oscillating reports across 3 epochs.
- **S5.6 ramp-up effect:** mid-script SN registration. Not recommended — covered
  in Go.
- **S6.3+ Cascade fee routing:** add a `cascade_action_or_skip` helper that
  attempts a Cascade action and `skip`s cleanly if the supernode service is not
  serving requests. Real assertion only fires when devnet supernode binaries
  are present.
- **S4.0 zero-pool:** would need the test to either run before S1.3 funding (changes ordering) or wait for natural drain. Not stable enough to add without more thought.
- **S9 / S10 stubs:** keep skip; document the alternate test paths in eval doc.

---

## 5. Eval-doc edits planned (after approval)

1. **Header** (line 7+): refresh "Latest recorded devnet run" once we know the new pass/skip counts after script extensions land.
2. **"Scripted Coverage" block** (lines 41–57): rewrite to itemize each assertion the script makes by ID (S1.1 … S8.3), grouped by scenario.
3. **Each scenario's Steps list**: add inline `**[AUTO: …]**`, `**[AUTO-NEW: …]**`, `**[MANUAL: …]**`, `**[COVERED-ELSEWHERE: …]**` tag at the end of each numbered step.
4. **Each scenario's Checklist**: add a leading `(scripted)` or `(manual)` marker per item so the operator can skip checked-by-script items confidently.
5. **Scenario 1 step 3** wording: change "module-account everlight" → "supernode pool module account (embedded Everlight)" since the standalone `everlight` module account no longer exists.
6. **Scenario 9 / Scenario 10**: add a "Scripted Coverage Note" explaining why these are skipped in `everlight_test.sh` and where they're covered (`app/upgrades/v1_12_0/upgrade_test.go`, plus sequential execution of S1+S3+S4+S5).

---

## 6. Open questions for the operator

1. **OK to drop the `everlight` module-account references** in Scenario 1 (eval doc step 3)? Code reality is the embedded `supernode` module account holds the pool funds — see `x/supernode/v1/types/keys.go` and `recent_decisions[2026-04-08]`. The eval doc currently says "auth module-account everlight" which would fail today.
2. **Do we want the smoothing-window oscillation test (S5.5b)?** It costs ~1–2 minutes runtime but is the only way to demonstrate `measurement_smoothing_periods > 1` behavior live.
3. **Do we want a Cascade-action gated test (S6.3)?** Even gated behind a clean `skip` when supernode service is absent, it adds maintenance. Alternative: keep S6 as a params-only check and rely on `tests/integration/action` for the fee-routing path.
4. **Confirm Scenario 9 stays a stub.** The Go upgrade test is in gate evidence; running a live devnet upgrade chain inside `everlight_test.sh` is out of scope.

After your answers I'll make the two edits (eval doc + script) in that order.
