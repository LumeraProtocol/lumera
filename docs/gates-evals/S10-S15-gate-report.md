# Gate Report
Generated: 2026-04-28T02:13:28Z
Features Audited: F10, F11, F12, F13, F14, F15, F16, F18

## Summary
**OVERALL: PASS**

Context was updated before this gate: S10-S15 Everlight features are now in `review` status, and F17/AT40 remains deferred to optional Phase 4. Per operator instruction, non-BRIDGE-tracked changes were ignored unless they touched LEP-5, LEP-6 planning/docs, or Everlight.

Functional Everlight evidence passed. Scoped Go lint issues in BRIDGE-tracked `x/action/v1` and `x/supernode/v1` are fixed. The remaining `buf lint` enum-value prefix findings for `SUPERNODE_STATE_*` are explicitly waived because enum names are intentionally preserved for API compatibility.

## Test Results
- Everlight supernode module: `go test ./x/supernode/v1/... -count=1` - PASS
- Everlight integration: `go test ./tests/integration/everlight -count=1` - PASS
- Everlight E2E: `go test ./tests/e2e/everlight -count=1` - PASS
- Everlight upgrade: `go test ./app/upgrades/v1_12_0 -count=1` - PASS
- LEP-5 action module regression: `go test ./x/action/v1/... -count=1` - PASS
- LEP-5 integration regression: `go test ./tests/integration/action/... -count=1` - PASS
- LEP-5 system regression: `go test ./tests/system/action/... -count=1` - PASS
- LEP-5 devnet scoped package run: `cd devnet && go test ./tests/validator -run 'TestLEP5' -count=1` - PASS
- Everlight devnet target: `make devnet-tests-everlight` - PASS with 35 pass, 0 fail, 5 skip
- Full devnet validator package: `cd devnet && go test ./tests/validator -count=1` - FAIL, ignored for this gate because the failing test is IBC fixture setup, not LEP-5/Everlight
- Build: `make build` - PASS (`Nothing to be done for 'build'`)
- Typecheck: not run as `go build ./...` because `docs/context.json` build instructions say not to use direct `go build`; package tests and `make build` provided compile evidence

## Code Quality
- Lint Errors: `golangci-lint run ./x/action/v1/... ./x/supernode/v1/...` - PASS
- Proto lint: BRIDGE comment/doc findings fixed; `SUPERNODE_STATE_*` enum-value prefix findings waived for API compatibility - WAIVED

## Security
- Vulnerabilities: `govulncheck ./x/action/v1/... ./x/supernode/v1/... ./tests/integration/action/... ./tests/integration/everlight ./tests/e2e/everlight ./app/upgrades/v1_12_0` found 0 called vulnerabilities - PASS
- Informational: govulncheck also reported 2 vulnerabilities in imported packages and 4 in required modules that scoped code does not appear to call

## Acceptance Test Evidence
| Feature | AT ID | Criterion | Evidence | Status |
|---------|-------|-----------|----------|--------|
| F10,F12 | AT30 | SN with only storage violation transitions to STORAGE_FULL | `go test ./x/supernode/v1/... -count=1`; proto/code inspection | VERIFIED |
| F12 | AT31 | STORAGE_FULL excluded from Cascade, included in Sense/Agents | `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F12 | AT32 | STORAGE_FULL recovers to ACTIVE | `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F13 | AT33 | Storage + other violation transitions to POSTPONED | `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F14 | AT34 | Everlight pool account accepts transfers | `go test ./tests/integration/everlight -count=1` | VERIFIED |
| F15 | AT35 | Pool distributes proportionally by cascade_kademlia_db_bytes | `go test ./tests/integration/everlight -count=1`; `go test ./tests/e2e/everlight -count=1`; devnet S3.1-S3.4 passed | VERIFIED |
| F15 | AT36 | Below-min SNs excluded from distribution | `go test ./tests/e2e/everlight -count=1` | VERIFIED |
| F15 | AT37 | New SN receives ramped-up payout weight | `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F15 | AT38 | Usage growth cap limits reported bytes increase | `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F16 | AT39 | Registration fee share flows to Everlight pool | `go test ./x/action/v1/... -count=1`; `go test ./x/supernode/v1/... -count=1` | VERIFIED |
| F17 | AT40 | Phase 4 block reward share | Deferred by requirements and context | DEFERRED |
| F14 | AT41 | RewardDistribution governable via supernode MsgUpdateParams | `go test ./tests/integration/everlight -count=1`; `go test ./tests/e2e/everlight -count=1` | VERIFIED |
| F18 | AT42 | Upgrade handler initializes supernode RewardDistribution params | `go test ./app/upgrades/v1_12_0 -count=1` | VERIFIED |
| F18 | AT43 | Existing SN states/actions unaffected by upgrade | `go test ./app/upgrades/v1_12_0 -count=1` | VERIFIED |
| F15 | AT44 | Zero-balance pool produces no distribution and no panic | `go test ./tests/integration/everlight -count=1` | VERIFIED |
| F15 | AT45 | No eligible SNs produces no distribution and no panic | `go test ./tests/integration/everlight -count=1` | VERIFIED |

## Devnet Evidence
- `make devnet-tests-everlight` connected to devnet at height 65 and completed with PASS: 35, FAIL: 0, SKIP: 5.
- Passed coverage includes module bootstrap (F14/F18), registration fee share (F16), governance parameter update (F11/F14), proto compatibility (F10/F11), STORAGE_FULL transition/recovery and legacy metrics non-mutation (F12/F13), distribution trigger setup (F15), and anti-gaming parameter presence (F15).
- Skips were: S3.5/S3.6/S3.7 payout assertions because candidates were not eligible at payout time; S4 edge cases because STORAGE_FULL precondition could not be established; S5.2 baseline audit report because no free reporter slot remained after epoch-safe retries; S9 upgrade idempotency requiring pre-Everlight genesis; S10 full lifecycle requiring full supernode lifecycle setup.

## Blocking Issues
None.

## Warnings
1. `devnet/go.mod` is modified from `go 1.25.5` to `go 1.25.9`; this is Everlight-adjacent because it affects devnet verification.
2. Full `devnet/tests/validator` failed on missing IBC channel fixture `/shared/status/hermes/channel_transfer.json`; scoped LEP-5 devnet tests and `make devnet-tests-everlight` passed.
3. Untracked local scaffolding/config files are present and ignored for gate scope: `.agents/`, `.bridge-version`, `.claude/`, `.codex/`, `AGENTS.md`, `CLAUDE.md`.
4. `buf lint` still reports `ENUM_VALUE_PREFIX` findings for `SUPERNODE_STATE_*`; waived because renaming would break the public API surface.

## Recommended Actions
1. Proceed to `$bridge-eval` for S10-S15.
2. If full devnet validator evidence is required, provision the IBC fixture. The Everlight-specific devnet target has passed with documented skips.
3. Keep the `SUPERNODE_STATE_*` enum prefix waiver documented for downstream API consumers.
