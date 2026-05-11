# LEP-6 Devnet Test Implementation Plan (PR1 — chain-side e2e)

> **Status:** implemented in PR #134 on `LEP-6-devnet-tests`; this document is retained as delivered design/runbook context.
> **Author:** j-rafique (Bilal)
> **Branch (target):** `LEP-6-devnet-tests` off `origin/master @ 3da2efe3`

---

## Goal

Add the missing devnet-test coverage for LEP-6 storage-truth enforcement, exercising the chain-side lifecycle under the real 5-validator Docker devnet (real BFT consensus, real EndBlock, real fee distribution, real genesis JSON), without depending on the supernode runtime (SN role is played in-process by signing as registered SN identities).

This addresses Anton's directive: *"LEP-6 doesn't even have devnet tests, going forward let's always add new tests to devnet setup."*

## Scope (Medium per D1)

**In scope (this PR):**
1. Params + genesis sanity + epoch anchor query
2. `MsgSubmitEpochReport` happy-path (host report + peer observations + storage proof results) → readback via storage-challenge report query
3. `MsgSubmitStorageRecheckEvidence` with `RECHECK_CONFIRMED_FAIL` → assert `NodeSuspicionState` and `TicketDeteriorationState` updated on chain
4. Heal-op lifecycle: `MsgClaimHealComplete` → `MsgSubmitHealVerification` → `HealOp` query reflects terminal `HEAL_OP_STATUS_VERIFIED`
5. Negative-path consensus checks for reviewer-facing auth/state invariants:
   - original reporter and challenged target cannot submit recheck evidence
   - claim against nonexistent heal op is rejected
   - duplicate heal verification vote is rejected

**Out of scope (deferred):**
- Postponement triggers across multiple epochs (already covered in `tests/systemtests/audit_storage_truth_*`)
- Recovery clean-pass (already covered in systemtests; not consensus-path-sensitive)
- Full supernode↔lumera e2e (covered separately in supernode `make test-lep6` / runtime system tests)

## Architecture

### Why devnet tests beyond systemtests
`tests/systemtests/audit_*_test.go` already cover edge cases under an in-process 4-node testnet with mocked SN keys and `ModifyGenesisJSON` hooks. The **value-add** of devnet tests is exercising the **lifecycle happy-path** under:
- Real 5-validator BFT consensus (not 4-node test fixture)
- Real fee distribution under v1.12.0 audit fee-share routing
- Real EndBlock pruning / epoch advancement in containerised network timing
- Real `devnet-genesis.json` (catches genesis-validation regressions that systemtest-mutated genesis can't)

### How the test plays the SN role
Devnet bootstraps 5 supernodes via `supernode-setup.sh`, each writing its mnemonic to `/tmp/<chain-id>/shared/status/<moniker>/sn_mnemonic`. The bash test runs on the host but submits `lumerad` CLI tx/query commands inside validator containers. It uses the supernode keys registered by `supernode-setup.sh` when available; if supernodes are not pre-registered, it bootstraps validator-owned supernode registrations using the devnet Docker network IP convention (`172.28.0.(10+idx)`). We need ≥2 distinct SNs because:
- Recheck evidence must come from a different SN than the prober (chain-enforced)
- Heal-op verifier must be different from the healer

### Genesis impact (D3 — verified safe)
- Lower `epoch_length_blocks` from 400 → 20 and set devnet-only `storage_truth_ticket_deterioration_heal_threshold` to `8`: makes the recheck/heal lifecycle testable quickly while preserving the production scoring constant assertion (`+8`). Heal-op scheduling still requires the chain eligibility predicate (holder diversity, index failure, or repeated recent failures), so the delivered test drives both threshold and eligibility before expecting a heal op.
- Verified: LEP-5 devnet test is epoch-agnostic; IBC tests don't touch audit; Everlight S7.7 already attempts the same lower-to-20 (now via gov, which fails on master because `epoch_length_blocks` is genesis-immutable — Anton's fix). With our genesis change, S7.7 becomes a no-op, **not** a new break.
- Add LEP-6 storage-truth params explicitly (currently missing in `devnet-genesis.json`). `WithDefaults()` would auto-derive bucket windows, but we set them explicitly so the genesis is self-documenting and any auto-derivation regression is caught.

### Tech stack
- Bash/CLI test script modeled on `devnet/tests/everlight/everlight_test.sh` because `sdk-go@v1.0.8` does not expose audit tx helpers or a generic broadcaster for LEP-6 messages
- `lumerad` CLI inside validator containers for tx/query submission under real Docker devnet consensus
- Helpers lifted/adapted from `tests/systemtests/audit_test_helpers_test.go`: host-report JSON builder, observation JSON builder, proof-result JSON builder
- DeliverTx-aware `wait_for_tx` / expected-rejection helpers so CheckTx success cannot mask chain rejection

## File Map

### New files

**`devnet/tests/lep6/lep6_test.sh`** — main CLI-driven test script
- `TestLEP6_ParamsAndEpochAnchor`
- `TestLEP6_SubmitEpochReport_HappyPath`
- `TestLEP6_SubmitStorageRecheckEvidence_UpdatesSuspicionScore`
- `TestLEP6_RecheckEvidenceRejectsUnauthorizedSubmitter`
- `TestLEP6_ClaimHealCompleteRejectsNonexistentOp`
- `TestLEP6_HealOpLifecycle_ClaimVerifyFinalize`, including duplicate heal-verification rejection
- Test-local helpers (supernode signer discovery/bootstrap, query wrappers, JSON builders, DeliverTx-aware expected-rejection checks) — kept in same script for review locality; can split later if it grows

### Modified files

**`devnet/default-config/devnet-genesis.json`** — add LEP-6 params + lower epoch length
- `app_state.audit.params.epoch_length_blocks`: `"400"` → `"20"`
- `app_state.audit.params.keep_last_epoch_entries`: `"200"` → `"200"` (unchanged; ample for 20-block epochs)
- Add full `storage_truth_*` block with explicit values aligned to defaults but tuned for testability:
  - `storage_truth_enforcement_mode`: `"STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT"` (allow scoring + enforcement to be observed; not SHADOW which suppresses postponement)
  - `storage_truth_node_suspicion_threshold_postpone`: `"100"` (higher than recheck delta of 15, so single test recheck doesn't postpone)
  - `storage_truth_node_suspicion_decay_per_epoch`: `"1000"` (no decay per epoch, so short devnet windows keep score assertions deterministic)
  - `storage_truth_challenge_target_divisor`: `1` (every active SN is target — guarantees prober-target match without seed roulette)
  - All other LEP-6 fields: chain defaults explicitly written for self-documentation
- `app_state.audit.params.consecutive_epochs_to_postpone`: `1` → `100` (disable missing-report postponement during test windows)

**`Makefile.devnet`** — add `devnet-tests-lep6` target and `.PHONY` declaration

**`devnet/Readme.md`** — document the new target under §6.7 Devnet Makefile Commands

**`ACTIVE_WORK.md`** — record the new branch, PR target, and what was delivered

## Delivered Implementation Notes

This PR implements the bash/CLI approach rather than the earlier Go-test scaffold. The delivered artifacts are:

- `devnet/tests/lep6/lep6_test.sh` — CLI-driven chain-side lifecycle e2e runner.
- `Makefile.devnet` — `devnet-tests-lep6` target invokes the bash runner against a running Docker devnet.
- `devnet/default-config/devnet-genesis.json` — short devnet epochs plus explicit LEP-6 storage-truth params.
- `devnet/Readme.md` — documents the target and prerequisites.

### Runtime flow

1. Preflight: verify Docker devnet is running, query `audit params`, and derive `required_open_ports | length` dynamically so host/peer reports match the live genesis.
2. Bootstrap/discovery: use existing registered supernodes when present; otherwise register validator-owned supernodes with IPs matching `devnet/generators/docker-compose.go` (`172.28.0.(10+idx)`).
3. T1: verify LEP-6 params and current epoch anchor.
4. T2: submit a complete epoch report with host report, peer observations, and storage proof result.
5. T3: submit recheck evidence and assert exact scoring deltas: `NodeSuspicionState += 15`, `TicketDeteriorationState == 8`.
6. T5/T6/T7: negative-path checks assert the expected rejection reason, not just “some non-zero code”.
7. T4: drive ticket deterioration until both threshold and scheduling eligibility are true; then use the chain-assigned healer/verifier set and require terminal `HEAL_OP_STATUS_VERIFIED`.

### Run commands

```bash
make devnet-up-detach
make devnet-tests-lep6
```

Optional direct invocation:

```bash
COMPOSE_FILE=devnet/docker-compose.yml bash devnet/tests/lep6/lep6_test.sh
```

### Notes from implementation review

- `HEAL_OP_STATUS_VERIFIED` is the successful terminal status. `HEAL_OP_STATUS_HEALER_REPORTED` is only the post-claim intermediate state.
- Recheck evidence does not populate `artifact_class`, so it cannot satisfy the index-failure eligibility path by itself; the test waits for repeated recent failures or holder diversity instead.
- `storage_truth_node_suspicion_decay_per_epoch=1000` means no per-epoch decay in this branch’s thousandths-based decay model.
- The previous Go-test task list was intentionally removed from this document because it does not match the delivered bash implementation.

## Verification Strategy

Each task has a verification command. Final acceptance gates:
1. `make build` clean
2. `lumerad genesis validate-genesis devnet/default-config/devnet-genesis.json` ✓
3. `sg docker -c 'make devnet-build-default'` ✓
4. `sg docker -c 'make devnet-up-detach'` → 5 SNs registered within 60s
5. `make devnet-tests-lep6` → all bash-runner T1/T2/T3/T4/T5/T6/T7 checks PASS within 30 min total
6. `git diff --check` clean
7. Single commit on branch (`git log --oneline origin/master..HEAD | wc -l` == 1)

## Out-of-Scope (explicit list — do NOT add in this PR)

- Postponement trigger across multiple epochs (covered in `audit_storage_truth_activation_test.go`)
- Recovery clean-pass (covered in `audit_recovery_enforcement_test.go`)
- Storage-truth edge cases (covered in `audit_storage_truth_edge_cases_test.go`)
- v1.12.0 upgrade rehearsal target `devnet-upgrade-1120` (separate concern; track in `ACTIVE_WORK.md`)
- Full SN runtime testing (blocked on supernode `#286/#287/#288`)
- Genesis migration handler tweaks (chain landed; no devnet-test value-add)

## Rollback Plan

If the test cannot achieve PASS during validation:
1. Identify failing assertion (deterioration score? heal op transition timing?).
2. Determine root cause: genesis param mismatch, chain bug, or test bug.
3. **If chain bug:** open chain-side issue, mark this PR as draft, halt.
4. **If test bug:** iterate task. No production-side rollback needed.
5. **If timing-related** (heal op did not reach `HEAL_OP_STATUS_VERIFIED` within test timeout): seed heal_ops via genesis import path (proven safe — used in `audit_storage_truth_edge_cases_test.go`).

Genesis change is idempotent: reverting the JSON diff restores 400-block epochs without state corruption (devnet is always rebuilt clean via `devnet-build-default`).

## PR Body Draft

```markdown
## Summary

Adds the missing LEP-6 devnet test coverage. Closes the gap Anton flagged: *"LEP-6 doesn't even have devnet tests."*

## What's covered (chain-side e2e)

- LEP-6 params + current epoch anchor sanity
- `MsgSubmitEpochReport` happy-path (host report + peer observations + storage proof results) → `EpochReport` readback
- `MsgSubmitStorageRecheckEvidence(RECHECK_CONFIRMED_FAIL)` → `NodeSuspicionState` (+15) + `TicketDeteriorationState` (+8) updated on chain
- Heal-op lifecycle: claim → verify using chain-assigned verifiers → `HEAL_OP_STATUS_VERIFIED`

All under real 5-validator BFT consensus, real EndBlock, real fee distribution, real `devnet-genesis.json`.

## Genesis change

- `epoch_length_blocks`: 400 → 20 (testable in <2 min; safe — no other devnet test depends on 400)
- Added explicit `storage_truth_*` params block (was missing entirely; previously relied on `WithDefaults()` auto-fill)

⚠️ `everlight_test.sh` S7.7 attempts to set `epoch_length_blocks=20` via gov proposal — that path was broken upstream when the param became genesis-immutable. With this PR's genesis change, S7.7 is now a no-op. Heads-up for the Everlight fix.

## What's NOT covered here (follow-up)

- Full supernode↔lumera e2e — blocked on supernode `#286/#287/#288` merge
- Postponement / recovery / edge cases — already covered in `tests/systemtests/audit_storage_truth_*`
- `devnet-upgrade-1120` rehearsal target — separate concern, tracked in ACTIVE_WORK.md

## How to run

```
make devnet-up-detach
make devnet-tests-lep6
```

## Validation log

[link to /tmp/lep6-devnet-validation-<timestamp>.log uploaded as gist]

cc @anton @matee
```

---

**Delivered shape:** single commit, single PR, on branch `LEP-6-devnet-tests`; bash runner plus devnet genesis/Makefile/docs wiring.
