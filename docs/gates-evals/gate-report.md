# Gate Report
Generated: 2026-04-06T12:00:00Z
Features Audited: F10, F11, F12, F13, F14, F15, F16, F17, F18
Slices Audited: S10, S11, S12, S13, S14, S15

## Summary
**OVERALL: FAIL** (1 blocking issue, 2 warnings)

## Test Results
- Unit (x/everlight): 34 passed, 0 failed - Coverage: 67.8% (threshold: 80%) - **WARN** (below target, non-blocking)
- Unit (x/supernode): all passed, 0 failed - PASS
- Unit (x/action): all passed, 0 failed - PASS
- Unit (app/upgrades/v1_15_0): 8 passed, 0 failed - PASS
- Integration (tests/integration/everlight): 11 passed, 0 failed - PASS
- Integration (tests/integration overall): all passed, 0 failed - PASS

## Build
- `go build ./...`: PASS (exit 0)
- `make build`: FAIL (missing `ignite` CLI tool in local environment; not a code issue)

## Code Quality
- Lint (x/everlight): 0 errors - PASS
- Lint (x/supernode): 3 pre-existing warnings (deprecated SDK APIs, test-only) - PASS
- Lint (x/action): 6 pre-existing warnings (test-only errcheck, deprecated rand.Seed) - PASS
- Typecheck (`go build ./...`): 0 errors - PASS

## Security
- Module Account Permissions: Everlight has only `Burner` (no Minter, no Staking) - PASS
- No governance voting rights on module account - PASS (Burner-only)
- Anti-gaming guardrails (growth cap, smoothing, ramp-up): Implemented and tested - PASS
- NaN/Inf handling in float conversion: `legacyDecFromFloat64` validates against NaN/Inf - PASS
- No secrets in code: Verified. `unsinged.json` is an unsigned tx template, not a credential - PASS
- Determinism: Distribution uses `LegacyDec` arithmetic with truncation (dust stays in pool) - PASS

## Acceptance Test Evidence

| Feature | AT ID | Criterion | Evidence | Status |
|---------|-------|-----------|----------|--------|
| F10 | AT30 | SN with only cascade_kademlia_db_bytes violation transitions to STORAGE_FULL | `TestEvaluateComplianceStorageFullOnly` (metrics_validation_test.go), `TestReportSupernodeMetrics_StorageFullFromPostponedEmitsStorageFullEvent` (msg_server_report_supernode_metrics_test.go) | PASS |
| F11 | AT41 | All Everlight params governable via MsgUpdateParams | `TestMsgUpdateParams` (msg_server_test.go), `TestEverlightParams` (integration) | PASS |
| F12 | AT30 | (shared with F10 above) | Same evidence | PASS |
| F12 | AT31 | STORAGE_FULL SN excluded from Cascade selection, included in Sense/Agents | `TestAT31_StorageFullExclusionFromCascadeInclusionInSense` (query_get_top_super_nodes_for_block_test.go) | PASS |
| F12 | AT32 | STORAGE_FULL SN recovers to ACTIVE when storage drops below threshold | Code path in `msg_server_report_supernode_metrics.go:103-108` calls `recoverFromStorageFull()`. **NO dedicated test.** | **FAIL** |
| F12 | AT33 | SN with storage + other violation transitions to POSTPONED | `TestEvaluateComplianceStorageFullPlusOtherIssue` (metrics_validation_test.go) + POSTPONED transition code path in msg_server_report_supernode_metrics.go:131-139 | PASS |
| F13 | AT30 | (shared) | Same as F10/F12 AT30 | PASS |
| F13 | AT33 | (shared) | Same as F12 AT33 | PASS |
| F14 | AT34 | Everlight module account accepts MsgSend transfers | `TestEverlightModuleAccount` (integration: funds via MsgSend, verifies balance) + `TestSendCoinsFromAccountToModule` (unit) | PASS |
| F14 | AT41 | (shared with F11 above) | Same evidence | PASS |
| F14 | AT44 | Pool with zero balance: no distribution, no panic | `TestZeroPoolBalance` (distribution_test.go) + `TestEverlightEndBlockerEmptyPool` (integration) | PASS |
| F14 | AT45 | No eligible SNs: no distribution, no panic | `TestNoEligibleSNs` (distribution_test.go) + `TestEverlightEndBlockerNoEligibleSNs` (integration) | PASS |
| F15 | AT35 | Pool distributes proportionally by cascade_kademlia_db_bytes | `TestDistributePoolProportionally` (distribution_test.go) + `TestEverlightEndBlockerDistribution` (integration) | PASS |
| F15 | AT36 | SNs below min_cascade_bytes_for_payment excluded | `TestMinCascadeBytesThreshold` (distribution_test.go) | PASS |
| F15 | AT37 | New SN receives ramped-up (partial) payout weight | `TestNewSNRampUp` (distribution_test.go) | PASS |
| F15 | AT38 | Usage growth cap limits reported cascade bytes increase | `TestUsageGrowthCap` (distribution_test.go) | PASS |
| F15 | AT44 | (shared) | Same as F14 AT44 | PASS |
| F15 | AT45 | (shared) | Same as F14 AT45 | PASS |
| F16 | AT39 | Registration fee share flows to Everlight pool | `TestGetRegistrationFeeShareBps` + `TestRegistrationFeeShareCalculation` (fee_routing_test.go) + action.go DistributeFees code path (line 618-632) | PASS |
| F17 | AT40 | Block reward share flows to Everlight pool | `TestBeginBlockerBlockRewardShare` + edge cases (fee_routing_test.go) + `TestEverlightBeginBlockerFeeRouting` (integration) | PASS |
| F18 | AT42 | Upgrade handler initializes Everlight store and params | `TestStoreUpgradesAddsEverlight`, `TestStoreUpgradesAddedContainsOnlyEverlight`, `TestCreateUpgradeHandlerReturnsNonNil` (upgrade_test.go) | PASS |
| F18 | AT43 | Existing SN states and actions unaffected by upgrade | `TestStoreUpgradesDoesNotDeleteExistingModules`, `TestStoreUpgradesDoesNotRenameExistingModules` (upgrade_test.go) | PASS |

## Blocking Issues
1. **AT32 missing executable test evidence** -- `STORAGE_FULL -> ACTIVE recovery` has no dedicated test. The code path exists (`recoverFromStorageFull` in `x/supernode/v1/keeper/metrics_state.go:79`) and is wired in `msg_server_report_supernode_metrics.go:103-108`, but there is no test that: (a) starts a SN in STORAGE_FULL state, (b) submits compliant metrics with `cascade_kademlia_db_bytes` below the threshold, and (c) verifies the SN transitions back to ACTIVE with a `EventTypeSupernodeStorageRecovered` event. **File**: `x/supernode/v1/keeper/msg_server_report_supernode_metrics_test.go`. **Action**: Add a test named `TestReportSupernodeMetrics_StorageFullRecoversToActive` that exercises this flow end-to-end.

## Warnings
1. **Coverage below target**: x/everlight/v1/keeper at 67.8% vs 80% target. Key gap: BeginBlocker/EndBlocker integration paths are tested in integration tests but not counted in unit coverage. Non-blocking because integration tests cover the paths.
2. **`make build` fails**: `ignite` CLI is not installed locally. This is an environment issue, not a code defect. `go build ./...` passes cleanly.

## Scope Compliance
- All implementations are within the boundaries defined in `requirements.json` `scope.in_scope`.
- No scope creep detected. Module uses block-height epoch (not x/epochs) per constraints.
- Module account has minimal permissions (Burner only) per R13 mitigation.
- BeginBlocker ordering correct: everlight runs before x/distribution to skim fee collector.

## Recommended Actions
1. **[BLOCKING]** Add test `TestReportSupernodeMetrics_StorageFullRecoversToActive` in `x/supernode/v1/keeper/msg_server_report_supernode_metrics_test.go`. Test should: create SN in STORAGE_FULL state, set params with `cascade_kademlia_db_max_bytes > 0`, submit metrics with `cascade_kademlia_db_bytes` below threshold and all other metrics compliant, verify SN state transitions to ACTIVE, verify `EventTypeSupernodeStorageRecovered` event emitted.
2. **[OPTIONAL]** Increase unit test coverage for x/everlight/v1/keeper to meet the 80% target. Priority areas: error paths in `distributePool`, edge cases in `legacyDecFromFloat64`.
