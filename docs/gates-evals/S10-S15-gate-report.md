# Gate Report
Generated: 2026-05-01T02:29:00Z
Features Audited: F10, F11, F12, F13, F14, F15, F16, F18
Commit: 451f8a8

## Summary
**OVERALL: FAIL**

This is a re-gate of the closed S10-S15 Everlight Phase 1 scope after later PR updates touched `x/action/v1`, `x/supernode/v1`, Everlight integration/E2E tests, and the Everlight devnet script. `docs/context.json` currently has no features in `review`; the operator explicitly requested this re-gate after PR feedback fixes.

Functional tests, code generation, scoped vulnerability scanning, and `make build` passed. The operator explicitly waived scoped Go lint findings on 2026-05-01. The gate still fails because the Everlight devnet target could not run: the local devnet was not running.

## Test Results
- Proto/codegen: `make build-proto` - PASS
- LEP-5 action module regression: `go test ./x/action/v1/... -count=1` - PASS
- Everlight supernode module: `go test ./x/supernode/v1/... -count=1` - PASS
- LEP-5 integration regression: `go test ./tests/integration/action/... -count=1` - PASS
- Everlight integration: `go test ./tests/integration/everlight -count=1` - PASS
- Everlight E2E: `go test ./tests/e2e/everlight -count=1` - PASS
- Everlight upgrade: `go test ./app/upgrades/v1_12_0 -count=1` - PASS
- Everlight devnet target: `make devnet-tests-everlight` - FAIL/BLOCKED; `supernova_validator_1` was unreachable and the command advised `make devnet-up-detach`
- Build: `make build` - PASS
- Typecheck: direct `go build ./...` was not run because `docs/context.json` says not to use direct `go build`; package tests plus `make build` provided compile evidence

## Code Quality
- Go lint: `golangci-lint run ./x/action/v1/... ./x/supernode/v1/...` - WAIVED by operator on 2026-05-01
  - `x/supernode/v1/keeper/audit_metrics.go:5`: unused `maxAuditEpochLookback`
  - `x/supernode/v1/keeper/audit_setter.go:5`: unused `globalAuditKeeper`
  - `x/supernode/v1/keeper/distribution_test.go:147`: unused `(*mockAuditKeeper).setReport`
- Proto lint: `buf lint proto/` - FAIL/WARN for this S10-S15 gate
  - Full-repo lint still reports broad legacy proto style findings.
  - New LEP-6/audit proto documentation findings are outside S10-S15 Everlight Phase 1 but should be resolved before gating LEP-6.
  - `SUPERNODE_STATE_*` enum-prefix findings remain intentionally waived for downstream API compatibility.

## Security
- Vulnerabilities: `govulncheck ./x/action/v1/... ./x/supernode/v1/... ./tests/integration/action/... ./tests/integration/everlight ./tests/e2e/everlight ./app/upgrades/v1_12_0` - PASS, 0 called vulnerabilities
- Informational: govulncheck also reported 2 vulnerabilities in imported packages and 4 in required modules that scoped code does not appear to call

## Acceptance Test Evidence
| Feature | AT ID | Criterion | Evidence | Status |
|---------|-------|-----------|----------|--------|
| F10,F12 | AT30 | SN with only storage violation transitions to STORAGE_FULL | `go test ./x/supernode/v1/... -count=1`; scoped code/tests compile after LEP-6 metric-source change | VERIFIED |
| F12 | AT31 | STORAGE_FULL excluded from Cascade, included in Sense/Agents | `go test ./x/supernode/v1/... -count=1`; `go test ./tests/e2e/everlight -count=1` | VERIFIED |
| F12 | AT32 | STORAGE_FULL recovers to ACTIVE | `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F13 | AT33 | Storage + other violation transitions to POSTPONED | `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F14 | AT34 | Everlight pool account accepts transfers | `go test ./tests/integration/everlight -count=1` | VERIFIED |
| F15 | AT35 | Pool distributes proportionally by cascade_kademlia_db_bytes | `go test ./x/supernode/v1/... -count=1`; `go test ./tests/integration/everlight -count=1`; `go test ./tests/e2e/everlight -count=1` | VERIFIED |
| F15 | AT36 | Below-min SNs excluded from distribution | `go test ./tests/e2e/everlight -count=1` | VERIFIED |
| F15 | AT37 | New SN receives ramped-up payout weight | `go test ./x/supernode/v1/... -count=1`; `go test ./tests/e2e/everlight -count=1` | VERIFIED |
| F15 | AT38 | Usage growth cap limits reported bytes increase | `go test ./x/supernode/v1/... -count=1`; `go test ./tests/e2e/everlight -count=1` | VERIFIED |
| F16 | AT39 | Registration fee share flows to Everlight pool | `go test ./x/action/v1/... -count=1`; `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F17 | AT40 | Phase 4 block reward share | Deferred by requirements and context | DEFERRED |
| F14 | AT41 | RewardDistribution governable via supernode MsgUpdateParams | `go test ./tests/integration/everlight -count=1`; `go test ./tests/e2e/everlight -count=1` | VERIFIED |
| F18 | AT42 | Upgrade handler initializes supernode RewardDistribution params | `go test ./app/upgrades/v1_12_0 -count=1` | VERIFIED |
| F18 | AT43 | Existing SN states/actions unaffected by upgrade | `go test ./app/upgrades/v1_12_0 -count=1` | VERIFIED |
| F15 | AT44 | Zero-balance pool produces no distribution and no panic | `go test ./x/supernode/v1/... -count=1`; `go test ./tests/integration/everlight -count=1` | VERIFIED |
| F15 | AT45 | No eligible SNs produces no distribution and no panic | `go test ./x/supernode/v1/... -count=1`; `go test ./tests/integration/everlight -count=1` | VERIFIED |

## Blocking Issues
1. `make devnet-tests-everlight` could not execute because local devnet was not running. This blocks live devnet evidence refresh until devnet is started or explicitly waived.

## Warnings
1. Worktree was dirty during the gate: `buf.lock`, `devnet/go.mod`, `docs/static/openapi.yml`, `x/action/v1/types/metadata.pb.go`, `x/supernode/v1/types/params.pb.go`, and `x/supernode/v1/types/supernode_state.pb.go`.
2. `make build-proto` reproduced generated-file drift after the PR proto comment updates; those generated changes should be committed or intentionally reverted before final approval.
3. `buf lint proto/` reports broad repository findings. For S10-S15, this is partly legacy/compatibility noise and partly LEP-6/audit scope, but it should not be ignored for future LEP-6 gating.
4. No coverage percentage was produced by the gate commands; the `80%` target in requirements was not measured.
5. Scoped Go lint still has three unused-symbol findings, but these are waived for this gate by operator instruction.

## Recommended Actions
1. Start devnet with `make devnet-up-detach` or the project-preferred devnet startup flow, then re-run `make devnet-tests-everlight`.
2. Decide whether to commit the generated proto/openapi/buf.lock drift from `make build-proto`.
3. Optionally clean up the waived unused-symbol lint findings before final merge.
4. Re-run `$bridge-gate` after devnet evidence is available or explicitly waived. Do not proceed to `$bridge-eval` from this failed gate.
