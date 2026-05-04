# lumera — Project Status

Last updated: 2026-05-02
Version: BRIDGE installer v2.9.0 (chain release in flight: v1.12.0)
Source of truth: docs/context.json (feature status), docs/requirements.json (intent)

lumera is a Cosmos SDK + Tendermint blockchain (built originally with Ignite CLI) that runs verifiable Cascade and Sense action processing through SuperNodes. It is the **knowledge provider** in a four-repo workspace: `lumera` ships proto definitions and chain logic that `supernode`, `sdk-go`, and `sdk-js` consume. The current direction is closing out **Everlight Phase 1** (sustainable SuperNode retention compensation embedded in `x/supernode`) for the v1.12.0 chain upgrade, with LEP-6 (Phase 2 storage-truth) scaffolding starting to land in master ahead of any formal BRIDGE scope-in.

## In-Flight Branches

`master` is the active branch; all BRIDGE slices S01–S15 (LEP-5 + Everlight Phase 1) are merged. `context.json` does not declare any `parallel_tracks`, but two recently-touched non-master branches are still around the workspace:

- **chore/v1.12.0-upgrade-handler-completeness** (last commit 2026-05-02) — Merged into master as PR #131 (`3da2efe`); branch left intact locally. Goal: clarify required-keeper comment for the v1.12.0 upgrade handler. No further work expected.
- **evm** (last commit 2026-04-24) — Pre-beta EVM workstream: versioned `make` targets, test-accounts and IBC fixes for devnet. Outside the LEP-5 / Everlight scope; not currently tracked in `requirements.json`. Status: dormant on this checkout.
- **everlight** (last commit 2026-04-10) — Earlier Everlight Phase-1 doc/spec branch ("align phase-1 docs to disk-usage storage_full model"). Superseded by master after the embedded `x/supernode` consolidation; retained for history.

Working tree is dirty on master with regenerated artifacts (see Test & Quality Summary). No new in-flight feature branches beyond the merged v1.12.0 cleanup.

## Version Milestones

### v1.12.0 (LEP-5 + Everlight Phase 1) — In Progress (functionally complete, devnet re-gate pending)

Bundles the LEP-5 chunk-Merkle availability commitment work and Everlight Phase 1 retention-compensation pool into a single chain upgrade. Embedded-supernode design: no standalone `x/everlight` module; pool account is the existing `supernode` module account.

**LEP-5 (F01–F08)**

| Feature | Slice | Title | Key Deliverables | Status |
|---------|-------|-------|------------------|--------|
| F01 | S01 | Merkle Tree Library | `x/action/v1/merkle/` BLAKE3 domain-separated tree, proof gen + verify | done |
| F02 | S02 | Protobuf Schema Extensions | `AvailabilityCommitment` (HashAlgo enum, challenge_indices), `ChunkProof`, extended `CascadeMetadata` + `Params` | done |
| F03 | S03 | Challenge Index Generation | Client-provided indices stored on-chain at registration; keeper validates uniqueness/range/count | done |
| F04 | S04 | Registration Commitment Validation | `CascadeActionHandler.Process` validates and stores `AvailabilityCommitment` | done |
| F05 | S05 | Finalization Proof Verification | Keeper `VerifyChunkProofs`, BLAKE3 path verification, SVC skip for small files, evidence event on failure | done |
| F06 | S06 | Module Parameters and Governance | SVC params hardcoded as protocol constants via `getSVCParamsOrDefault()`; proto fields 12/13 reserved | done |
| F07 | S06 | Chain Upgrade and Migration (LEP-5) | Upgrade handler bundled with F18 into v1.12.0 (completed outside BRIDGE scope) | done |
| F08 | S07 | LEP-5 Multi-Category Tests | Integration / system / simulation / systemex / devnet docker E2E suites | done |

Slices DONE: S01–S07. All gates PASS through 2026-04-09 fresh re-gate.

**Everlight Phase 1 (F10–F18)**

| Feature | Slice | Title | Key Deliverables | Status |
|---------|-------|-------|------------------|--------|
| F10 | S10 | SuperNode Proto Extensions for Everlight | `SUPERNODE_STATE_STORAGE_FULL` (=6), `cascade_kademlia_db_bytes` (field 15), `RewardDistribution` (field 19) | done |
| F11 | S10 | Everlight Module Proto Schemas | Everlight surface embedded in `proto/lumera/supernode/v1` (`PoolState`, `SNEligibility`, params) | done |
| F12 | S11 | STORAGE_FULL SuperNode State | `markStorageFull` / `recoverFromStorageFull`, Cascade-excluded but compute-eligible | done |
| F13 | S11 | Compliance Bifurcation | Storage-only → STORAGE_FULL; storage + other → POSTPONED | done |
| F14 | S12 | Everlight Module Core | Embedded keeper, distribution state, genesis, params, queries inside `x/supernode` | done |
| F15 | S13 | Periodic Distribution Logic | EndBlocker distributes pool every `payment_period_blocks`; smoothing, growth cap, ramp-up, zero-balance / zero-eligible safety | done |
| F16 | S14 | Registration Fee Share Routing | `DistributeFees` routes configured bps to Everlight pool via supernode-owned interface | done |
| F17 | S14 | Block Reward Share Routing | Removed from this branch; explicitly deferred to optional Phase 4 | deferred |
| F18 | S15 | Chain Upgrade Handler (Everlight) | `app/upgrades/v1_12_0` initializes consolidated supernode params, anchors Everlight clock | done |

Slices DONE: S10–S15. Last operator approval recorded 2026-04-29.

### Future / out-of-band

- **LEP-6 storage-truth foundation (Phase 2)** — Scaffolding has begun landing on master (PR #117 audit `451f8a8`, plus `0e04831` "storage truth parameters, host report corrections, epoch boundary handling"). Currently **out of scope** in `requirements.json`. Needs a `bridge-feature` or `bridge-scope` pass before it should be tracked here.
- **Cross-repo integration (IAT01–IAT04)** — Pending until lumera publishes a tagged Everlight release and `supernode` / `sdk-go` / `sdk-js` consume it.
- **F17 / AT40 (block reward routing, optional Phase 4)** — Deferred. Would require x/distribution `AllocateTokens` modification.

### Architectural Decisions

`docs/decisions.md` is present with four entries; relevant items surfaced in Open Questions / Known Issues below. No `docs/contracts/` directory in the repo.

## Workspace Examples

No workspace examples in this repo.

## Test & Quality Summary

| Metric | Value | Notes |
|--------|-------|-------|
| Smoke tests | PASS | `go test ./x/action/v1/... && go test ./x/supernode/v1/... && go test ./tests/integration/action/...` last green at gate 2026-05-01 |
| Integration / E2E | PASS | `tests/integration/action`, `tests/integration/everlight`, `tests/e2e/everlight`, `app/upgrades/v1_12_0` all green at gate 2026-05-01 |
| Devnet E2E | unavailable | `make devnet-tests-everlight` blocked by absent local devnet — sole reason gate 2026-05-01 FAILed |
| Eval scenarios | 10 generated, 7 E2E (refreshed 2026-04-28) | `docs/gates-evals/S10-S15-eval-scenarios.md`, awaiting_feedback=false |
| Lint (`golangci-lint`) | scoped pass; 3 unused-symbol findings operator-waived 2026-05-01; whole-project warnings explicitly waived | `make lint` reports 111 broader diagnostics outside Everlight scope |
| Lint (`buf`) | `SUPERNODE_STATE_*` enum-prefix findings waived for API compatibility (decision 2026-04-28); other repository-wide buf findings out of Everlight scope | |
| Typecheck / build | PASS | `make build`, `make build-proto` clean at gate 2026-05-01 |
| `govulncheck` | 0 reachable | After Go 1.25.9 + CometBFT 0.38.21 update on 2026-04-08 |
| Coverage | n/a | Last numeric value 67.8% at the 2026-04-06 pre-resolution gate; recent gates report `n/a` |
| Last gate | **FAIL** on 2026-05-01 (commit 451f8a8) | 1 blocker (devnet unavailable), 5 warnings; previous run 2026-04-28 was PASS |

Worktree drift on master (uncommitted regenerated artifacts):

- `CHANGELOG.md`
- `buf.lock`
- `devnet/go.mod`
- `docs/static/openapi.yml`
- `x/action/v1/types/metadata.pb.go`
- `x/supernode/v1/types/params.pb.go`
- `x/supernode/v1/types/supernode_state.pb.go`

These look like outputs of `make build-proto` / `go mod tidy`. Confirm before committing — they should match the proto sources currently checked in.

## Known Open Issues

- **[blocker] Devnet smoke (`make devnet-tests-everlight`) was unavailable for the 2026-05-01 re-gate** — Only the local-devnet step blocked PASS; all other functional / integration / E2E suites were green. Bringing devnet up and re-running should clear the gate.
- **[warning] Whole-project lint debt** — `make lint` surfaces 111 diagnostics across the repo and `buf lint proto/` reports broader repo findings; both treated as non-blocking warnings outside the Everlight slice boundary (see decision 2026-04-28).
- **[warning] Three S10–S15 devnet evaluation checks intentionally skipped** — Operator deferred for later follow-up; non-blocking on closure.
- **[warning] `SUPERNODE_STATE_*` enum prefix waived in buf lint** — Compatibility-preserving decision (see `docs/decisions.md` 2026-04-28); revisit only if/when downstream clients can absorb a rename.
- **[note] LEP-6 scaffolding has begun landing on master** without being tracked in `requirements.json` — see commits `451f8a8`, `0e04831`. Should be brought into BRIDGE scope via `bridge-feature` or remain explicitly out-of-scope and documented.
- **[note] IAT01–IAT04 cross-repo integration tests** are pending a tagged lumera Everlight release plus downstream consumer updates (`supernode`, `sdk-go`, `sdk-js`). Not a code blocker, but blocks the cross-repo gate.

Resolved (kept here for traceability):

- ~~AT32 (STORAGE_FULL recovery test) missing dedicated test~~ — resolved 2026-04-08, gate passed.
- ~~22 golangci-lint findings (2026-04-08 first run)~~ — addressed / waived per operator on subsequent runs.
- ~~5 reachable vulnerabilities (2026-04-08 govulncheck)~~ — resolved by Go 1.25.9 + CometBFT 0.38.21 update; current `govulncheck` reports 0 called.

## Open Questions

`requirements.json.execution.open_questions` is empty. The items below are unresolved policy/scope questions surfaced from `context.json.discrepancies`, `docs/decisions.md`, and gate-history notes.

| ID | Topic | Status | Notes |
|----|-------|--------|-------|
| OQ-A | Pool-account model: requirements still describe a permissionless `everlight` account with no minter/burner/staking/voting permissions, but Phase 1 uses the existing `supernode` module account with its existing permissions. | open | Either reconcile `requirements.json` to the embedded design (decision 2026-04-28) or open a new feature for a dedicated permissionless pool account. Recorded in `context.json.discrepancies`. |
| OQ-B | Should F17 / AT40 (block reward routing, Phase 4) be revived? | open | Currently deferred; no implementation in this branch. Requires `x/distribution AllocateTokens` modification. |
| OQ-C | LEP-6 storage-truth scaffolding (PR #117, host-report / epoch-boundary work) is on master without BRIDGE scoping. | open | Decide: scope-in via `/bridge-feature`, leave out-of-scope and defer, or hold the line until Phase 1 is fully tagged. |
| OQ-D | Three S10–S15 devnet evaluation checks were skipped on 2026-04-10. | open | Document the specific checks and either re-run them after devnet is available or formally retire them. |
| OQ-E | Tagged release + downstream-consumer updates needed before IAT01–IAT04 can run. | open | Multi-repo coordination — needs `lumera` tag, then `supernode` / `sdk-go` / `sdk-js` bumps. |

## What's Next

Next slice: **none scheduled**. `context.json.next_slice` is null and `current_slice` is unset — Phase 1 closure is the working state.

Suggested next moves (per `context.json.handoff.next_immediate`, in operator preference order):

- **Re-gate v1.12.0 with devnet up.** Bring the local devnet online, run `make devnet-tests-everlight`, and re-run `/bridge-gate` to clear the 2026-05-01 FAIL → PASS.
- **Tag the Everlight Phase 1 release** once the gate clears, then drive IAT01–IAT04 across `supernode`, `sdk-go`, `sdk-js`.
- **Reconcile the pool-account discrepancy (OQ-A)** in `requirements.json` so docs match the implemented embedded-supernode design (or open a new feature for a dedicated account).
- **Decide on LEP-6 (OQ-C)** — bring the in-flight storage-truth scaffolding into BRIDGE via `/bridge-feature`, or pin it as out-of-scope until Phase 1 is tagged.
- **F17 / Phase 4 (OQ-B)** — open only on explicit operator decision to revive block-reward routing.
- **Stop** — declare Everlight Phase 1 closed and pause BRIDGE work for this repo.
