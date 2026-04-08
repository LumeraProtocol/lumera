# Gate Report
Generated: 2026-04-08T23:17:23Z
Features Audited: F10, F11, F12, F13, F14, F15, F16, F18
Slices Audited: S10-S15

## Summary
**OVERALL: PASS**

Functional Everlight evidence is green, the dependency security updates removed the previously reachable Everlight-scope `govulncheck` findings, and the build/test evidence remains clean. Whole-project `govulncheck ./...` and `make lint` still surface broader repo issues, but they are outside the Everlight slice boundary and are recorded here as non-blocking warnings per operator instruction.

## Test Results
- Unit and targeted integration: `go test ./x/action/v1/... -count=1 && go test ./x/supernode/v1/... -count=1 && go test ./tests/integration/action/... -count=1` - PASS
- Real Everlight integration/E2E/upgrade coverage: `go test ./tests/integration/everlight ./tests/e2e/everlight ./app/upgrades/v1_15_0 ./tests/integration/supernode -count=1` - PASS
- Coverage: n/a (threshold: 80%) - WARN

## Code Quality
- Lint Errors: not re-gated for this rerun (operator instruction: ignore current lint debt) - WARN
- Whole-project lint diagnostics: `make lint` reports 111 diagnostics across broader repo code and tests - WARN
- Type Errors: 0 observed via successful package test compilation and `make build` - PASS
- Build: `make build` - PASS

## Security
- Vulnerabilities affecting reachable code: 0 - PASS
- Whole-project reachable vulnerabilities: `govulncheck ./...` reports 8 vulnerabilities across 6 modules - WARN
- Additional imported/required-package vulnerabilities reported as non-reaching by `govulncheck` - WARN

## Acceptance Test Evidence
| Feature | AT ID | Criterion | Evidence | Status |
|---------|-------|-----------|----------|--------|
| F10 | AT30 | SN with only cascade_kademlia_db_bytes violation transitions to STORAGE_FULL (not POSTPONED) | `go test ./x/supernode/v1/keeper -count=1` includes `TestEvaluateComplianceStorageFullOnly` | PASS |
| F12 | AT31 | STORAGE_FULL SN excluded from Cascade selection, included in Sense/Agents selection | `go test ./x/supernode/v1/keeper -count=1` includes `TestAT31_StorageFullExclusionFromCascadeInclusionInSense` | PASS |
| F12 | AT32 | STORAGE_FULL SN recovers to ACTIVE when storage drops below threshold | `go test ./x/supernode/v1/keeper -count=1` includes `TestReportSupernodeMetrics_StorageFullRecoversToActive` | PASS |
| F13 | AT33 | SN with storage violation + other violation transitions to POSTPONED | `go test ./x/supernode/v1/keeper -count=1` includes `TestEvaluateComplianceStorageFullPlusOtherIssue` | PASS |
| F14 | AT34 | Everlight pool account (within x/supernode) accepts MsgSend transfers | `go test ./tests/integration/everlight -count=1` covers `TestEverlightModuleAccount` and `TestEverlightPoolState` | PASS |
| F15 | AT35 | Pool distributes proportionally by cascade_kademlia_db_bytes at period boundary | `go test ./x/supernode/v1/keeper -count=1` includes `TestDistributePoolProportionally`; `go test ./tests/e2e/everlight -count=1` covers `TestE2E_MultiSNProportionalDistribution` | PASS |
| F15 | AT36 | SNs below min_cascade_bytes_for_payment excluded from distribution | `go test ./x/supernode/v1/keeper -count=1` includes `TestMinCascadeBytesThreshold`; `go test ./tests/e2e/everlight -count=1` covers `TestE2E_BelowThresholdExclusion` | PASS |
| F15 | AT37 | New SN receives ramped-up (partial) payout weight during ramp-up period | `go test ./x/supernode/v1/keeper -count=1` includes `TestNewSNRampUp` | PASS |
| F15 | AT38 | Usage growth cap limits reported cascade bytes increase per period | `go test ./x/supernode/v1/keeper -count=1` includes `TestUsageGrowthCap` | PASS |
| F16 | AT39 | Registration fee share flows to Everlight pool on action finalization | `go test ./x/action/v1/... -count=1` and `go test ./x/supernode/v1/keeper -count=1` include `TestRegistrationFeeShareCalculation` and `TestGetRegistrationFeeShareBps` | PASS |
| F14 | AT41 | All Everlight params (RewardDistribution) governable via supernode MsgUpdateParams | `go test ./tests/integration/everlight -count=1` covers `TestEverlightParams`; `go test ./x/supernode/v1/keeper -count=1` includes `TestMsgUpdateParams` | PASS |
| F18 | AT42 | Upgrade handler initializes supernode RewardDistribution params with defaults | `go test ./app/upgrades/v1_15_0 -count=1` | PASS |
| F18 | AT43 | Existing SN states and actions unaffected by Everlight upgrade | `go test ./app/upgrades/v1_15_0 -count=1` covers `TestStoreUpgrades_NoStandaloneEverlightStoreAdded` and upgrade preservation behavior | PASS |
| F15 | AT44 | Pool with zero balance produces no distribution and no panic | `go test ./x/supernode/v1/keeper -count=1` includes `TestZeroPoolBalance`; `go test ./tests/integration/everlight -count=1` covers `TestEverlightEndBlockerEmptyPool` | PASS |
| F15 | AT45 | No eligible SNs produces no distribution and no panic | `go test ./x/supernode/v1/keeper -count=1` includes `TestNoEligibleSNs`; `go test ./tests/integration/everlight -count=1` covers `TestEverlightEndBlockerNoEligibleSNs` | PASS |

## Blocking Issues
None.

## Warnings
1. The gate scope was inferred from the completed Everlight migration (`F10-F16`, `F18`) because `docs/context.json` currently has no features in `review` status.
2. Lint was intentionally excluded from blocking status for this rerun per operator instruction. Existing lint debt remains unresolved and was not used to determine gate outcome.
3. Whole-project `govulncheck ./...` reports 8 reachable vulnerabilities outside the audited Everlight slice, including `google.golang.org/grpc@v1.77.0`, `github.com/shamaton/msgpack/v2@v2.2.3`, `github.com/ethereum/go-ethereum@v1.15.11`, `go.opentelemetry.io/otel/sdk@v1.38.0`, `github.com/ulikunitz/xz@v0.5.14`, and `github.com/cosmos/cosmos-sdk@v0.53.5`.
4. Whole-project `make lint` reports 111 diagnostics, heavily concentrated in integration/system tests and non-Everlight modules, including unchecked test helper errors, unused helpers, and SDK deprecation warnings.

## Recommended Actions
1. Proceed to `$bridge-eval` for S10-S15 while keeping the current lint waiver explicit in downstream artifacts.
2. Track the whole-project vulnerability set from `govulncheck ./...` as repo-wide follow-up work, separate from this Everlight feature gate.
3. Track the 111 `make lint` diagnostics as repo-wide cleanup debt, separate from this Everlight feature gate.
4. When you want lint and full-project security restored as blocking criteria, rerun `$bridge-gate` without the waiver.
