# Comprehensive EVM Support Audit

Date: 2026-05-29
Branch: `evm-audit`
Baseline: `origin/master` at `e4b16194b2a646bf9f51a80c134bfce2d78e95ff`

## Executive Summary

This audit reviewed Lumera's merged EVM support across app wiring, ante handling, mempool, JSON-RPC/OpenRPC, fee market, precisebank, ERC20/IBC, precompiles, CosmWasm/EVM bridge, evmigration, migration scripts, user docs, and devnet coverage.

Confirmed high-signal findings:

| ID | Severity | Area | Status |
| --- | --- | --- | --- |
| SEC-01 | High | EVMigration ante/mempool | Fixed on `evm-audit` |
| BUG-01 | Medium | Precisebank tests/accounting coverage | Fixed on `evm-audit` |
| DOC-01 | Medium | Security audit/config docs | Fixed on `evm-audit` |
| DOC-02 | Low | Architecture docs | Fixed on `evm-audit` |
| TEST-01 | Medium | Devnet/test inventory docs | Fixed on `evm-audit` |

No direct fund-theft path was found in the reviewed EVM entry points. The most important security issue found during this pass was operational DoS: structurally valid but cryptographically invalid migration txs were fee-free, unsigned at the Cosmos envelope layer, and could pass ante/mempool admission because proof verification happened later in message execution. This branch fixes that by verifying migration proofs in the reduced ante path.

## Security Audit Report

### SEC-01: Invalid embedded-proof migration txs can be admitted fee-free before cryptographic verification

Severity: High

Affected code:

- `app/evm/ante.go:189` builds a migration-only ante chain.
- `app/evm/ante.go:205` documents that migration txs skip the standard fee/signature/sequence subchain.
- `ante/evmigration_validate_basic_decorator.go:34` only runs tx `ValidateBasic`, allowing `ErrNoSignatures` for migration-only txs.
- `x/evmigration/types/proof.go:21` confirms `ValidateBasic` checks structure, while param-dependent limits are deferred.
- `x/evmigration/keeper/msg_server_claim_legacy.go:50` and `:70` perform param and cryptographic proof verification only in msg execution.

Impact:

- Attackers can construct many `MsgClaimLegacyAccount` or `MsgMigrateValidator` txs with correctly shaped public keys/signature lengths but invalid signatures.
- These txs require no Cosmos tx signer, no sequence, and no fee.
- CheckTx/ante can admit them because it does not cryptographically verify embedded proofs.
- They fail only during DeliverTx, consuming mempool capacity and proposal/block execution bandwidth.
- `MaxMigrationsPerBlock` does not mitigate failed invalid-proof attempts because the counter is incremented in finalization after successful migration.

Exploitability:

- Requires only public tx broadcast access.
- Does not require funds on either legacy or destination account if the attacker targets any existing legacy address and creates structurally valid proof payloads bound to that address.
- Variants can be produced by changing signature bytes, memo, gas, or destination material.

Recommended fix:

- Add a migration-specific ante decorator before mempool admission that performs stateless cryptographic proof verification using `ctx.ChainID()`, `config.EVMChainID`, message kind, legacy address, and new address.
- Enforce `MaxMultisigSubKeys` before expensive crypto if params can be made available to the decorator; otherwise add a conservative stateless hard cap equal to the default and keep the keeper param check as defense in depth.
- Add tests proving malformed signatures are rejected in CheckTx/ante, not only in DeliverTx.
- Consider adding a low fixed gas/fee requirement for invalid migration attempts after destination funding becomes possible, but do not rely on fees alone for this admission bug.

Suggested tests:

- Unit: `EVMigrationValidateBasicDecorator` or new decorator rejects invalid legacy proof signature.
- Unit: rejects invalid new-side proof signature.
- Integration: broadcasting invalid-proof migration tx returns CheckTx error and does not enter mempool.
- Negative: valid proof remains fee-free and envelope-signature-free.

Status on `evm-audit`:

- `app/evm/ante.go` now requires an `EVMigrationKeeper` proof verifier and runs it immediately after migration `ValidateBasic`.
- `x/evmigration/keeper/ante.go` reuses the existing migration proof verification helpers for `MsgClaimLegacyAccount` and `MsgMigrateValidator`.
- `app/evm/ante_evmigration_fee_test.go` now constructs real legacy and EVM keys and asserts a corrupted embedded proof is rejected in ante.
- `x/evmigration/keeper/ante_test.go` now covers valid claim/validator proofs, invalid legacy proof rejection, invalid new proof rejection, and unsupported message rejection.
- `app/evm/ante_evmigration_fee_test.go` now also encodes an unsigned migration tx and submits it through `app.CheckTx`, proving invalid embedded proofs are rejected at admission.

### Existing Security Posture

Positive controls confirmed in code/docs:

- EVM and Cosmos ante paths are explicitly separated.
- Mainnet JSON-RPC namespace policy rejects `admin`, `debug`, and `personal`.
- JSON-RPC rate limiting is now injected into the public alias proxy when aliasing is active.
- EVMigration proof payload includes Cosmos chain ID and EVM chain ID.
- Multisig migration enforces shape, K/N, duplicate-subkey rejection, and matching signer indices.
- Precompile tx paths reviewed bind caller authority from `contract.Caller()` rather than calldata-provided actor fields.

Residual risks:

- Migration proofs still have no per-proof expiry; operational replay remains possible until migration is disabled or deadline expires.
- Public JSON-RPC safety depends on correct operator config and docs being clear about alias vs standalone rate-limit topology.

## Implementation Bug Report

### BUG-01: Fast EVM baseline test fails in precisebank extended burn state matrix

Severity: Medium

Command:

```bash
go test -tags=test ./app ./app/evm ./config ./precompiles/... ./x/evmigration/... ./x/erc20policy/...
```

Observed result:

```text
FAIL: TestPreciseBankBurnExtendedCoinStateTransitions/borrow_from_integer_to_cover_fractional_burn
panic: module account mint does not have permissions to burn tokens: unauthorized
```

Affected code:

- `app/precisebank_mint_burn_behavior_test.go:56` correctly documents that `mint` has no burner permission.
- `app/precisebank_mint_burn_behavior_test.go:236` uses `minttypes.ModuleName` as the module under test for extended-denom burn transitions.
- `app/precisebank_mint_burn_behavior_test.go:261` expects `PreciseBankKeeper.BurnCoins(ctx, minttypes.ModuleName, ...)` to succeed.
- `app/app_config.go:219` grants `mint` only `Minter`, while `gov` has `Burner` at `:222`.

Impact:

- The documented baseline test command is red on clean `origin/master`.
- The failing test contradicts the permission matrix in the same file.
- This blocks using the current app test suite as a release gate for EVM precisebank behavior.

Likely fix:

- If the scenario is testing generic extended-denom burn accounting, use a module account with burner permission (`gov` or another burner) and fund it accordingly.
- If production expects `mint` to burn precisebank extended denom, update module permissions and add a focused permission test explaining why `mint` is both minter and burner.

Status on `evm-audit`:

- The test now uses `gov`, which has burner permission, for the extended-denom burn state transition matrix.

## Test Coverage Matrix

| Feature / invariant | Unit | Integration | Devnet/e2e | Script | Status |
| --- | --- | --- | --- | --- | --- |
| EVM app wiring, genesis modules, store keys | Covered | Partial | Upgrade flow | N/A | Covered |
| Dual ante routing | Covered | Covered | Indirect | N/A | Covered |
| Migration tx fee-free envelope | Covered | Covered | Covered | Covered | Covered |
| Invalid migration embedded signatures rejected before mempool admission | Covered | App CheckTx covered | Missing | N/A | Partially covered |
| EVM mempool nonce gaps/replacement/ordering/capacity | Covered | Covered | Cross-peer tx | N/A | Covered |
| Async EVM broadcast worker re-gossip | Covered | Covered | Covered | N/A | Covered |
| JSON-RPC basic methods, tx lookup, receipts | Covered | Covered | Covered | N/A | Covered |
| JSON-RPC rate limiting public alias path | Unit covered | Missing/unclear | Missing | N/A | Partially covered |
| WebSocket subscriptions | Covered | Covered | Missing | N/A | Partially covered |
| Fee market base fee and min floor | Covered | Covered | Covered | N/A | Covered |
| Precisebank send/query/fractional accounting | Covered | Covered | Missing | N/A | Partially covered |
| ERC20/IBC exact and provenance-bound allowlist | Covered | Covered | IBC transfer only | N/A | Partially covered |
| Contract deploy/call/logs/storage persistence | N/A | Covered | Missing | N/A | Partially covered |
| Standard precompiles | N/A | Covered | Missing | N/A | Partially covered |
| Action/Supernode/Wasm precompiles | Covered/partial | Covered | Missing | N/A | Partially covered |
| CosmWasm -> EVM bridge | Covered | Covered | Missing | N/A | Partially covered |
| EVM -> CosmWasm precompile | Covered/partial | Covered | Missing | N/A | Partially covered |
| EVMigration single-key account | Covered | Covered | Covered | Covered | Covered |
| EVMigration multisig account | Covered | Covered | Covered | Covered | Covered |
| EVMigration vesting account | Covered | Covered | Covered | Covered | Covered |
| Validator migration delegation/redelegation/distribution/supernode | Covered | Covered | Covered | Covered | Covered |
| Invalid/replay migration negative paths | Covered | Covered | Partial | Covered | Partially covered |
| Devnet pre-EVM -> EVM upgrade | N/A | N/A | Covered by target | N/A | Covered |
| Devnet JSON-RPC across validators | N/A | N/A | Covered | N/A | Covered |
| Devnet EVM contract deploy/call/log/precompile | N/A | Covered single-node | Missing | N/A | Missing |

## Migration Guide Review

Reviewed docs:

- `docs/evm-integration/user-guides/migration.md`
- `docs/evm-integration/user-guides/validator-migration.md`
- `docs/evm-integration/user-guides/supernode-migration.md`
- `docs/evm-integration/user-guides/migration-scripts.md`
- `docs/evm-integration/evmigration/**`

Strengths:

- Guides explain the coin type 118 -> 60 migration clearly.
- Single-key and multisig script guides include chain ID resolution, dry-run flow, destination freshness, wrong-script guards, and failure guidance.
- User guide warns that migration is irreversible and fee-free.
- Multisig docs correctly describe the mirror-source rule and signer-index intersection.

Gaps:

- Migration docs should warn operators that fee-free migration txs increase public mempool abuse sensitivity until invalid embedded proofs are rejected in ante.
- The Portal guide is heavily devnet-profile oriented in examples; add a concise mainnet quick path with exact chain/profile names, RPC expectations, and support escalation.
- Script docs correctly state `--chain-id` auto-detection, but should warn that disconnected/offline signing ceremonies should pin `--chain-id` explicitly and record it in the ceremony transcript.
- Add a recovery table for partial multisig ceremonies: stale proof after governance chain-id/config change, missing co-signer, wrong new-sub-pub-key order, and destination no longer fresh between generate and submit.

## Docs Consistency Report

### DOC-01: Security audit and config comments describe stale JSON-RPC rate-limit topology

Severity: Medium

Evidence:

- `docs/evm-integration/audits/security-audit-2026-03-20.md:17` said one finding remained open before this branch refreshed it.
- `docs/evm-integration/audits/security-audit-2026-03-20.md:21` called ERC20 provenance still open before this branch refreshed it.
- `docs/evm-integration/audits/security-audit-2026-03-20.md:49` described the old separate-port rate-limit bypass before this branch refreshed it.
- `docs/evm-integration/audits/security-audit-2026-03-20.md:210` later said the ERC20 finding was fixed, contradicting the old executive summary.
- `cmd/lumera/cmd/config.go:40` says rate limiting is a reverse proxy listening on `proxy-address`, but `app/app.go:422` and `app/evm_jsonrpc_ratelimit.go` route alias-active traffic through the public alias proxy and use `proxy-address` only as standalone fallback.

Recommended fix:

- Rewrite the security audit executive summary to reflect all previously tracked findings accurately.
- Update the `app.toml` template comments: when alias proxy is active, `enable=true` rate-limits the public JSON-RPC address; `proxy-address` is only used when the alias proxy is inactive.
- Add a short operator note in the node EVM config guide.

### DOC-02: Coin-type architecture doc uses obsolete migration proof format

Severity: Low

Evidence:

- `docs/evm-integration/architecture/coin-type-change.md:57` describes `MsgClaimLegacyAccount` with only `legacy_addr`, `new_addr`, and `legacy_signature`.
- `docs/evm-integration/architecture/coin-type-change.md:58` uses old payload `lumera-evm-migration:<legacy_addr>:<new_addr>`.
- Code now signs `lumera-evm-migration:<chainID>:<evmChainID>:<kind>:<legacyAddr>:<newAddr>` in `x/evmigration/keeper/verify.go:21`.

Recommended fix:

- Update the architecture doc to mention both legacy and new proofs.
- Replace the payload example with the current v2 domain-separated payload.

## Devnet Scenario Matrix

| Required devnet scenario | Existing coverage | Status |
| --- | --- | --- |
| Fresh EVM-enabled devnet startup | `devnet-new` plus validator EVM tests | Covered |
| Pre-EVM to EVM upgrade | `make devnet-evm-upgrade` | Covered |
| Single-key account migration | `tests_evmigration migrate-all` | Covered |
| Multisig account migration | prepared fixture and `multisig` mode | Covered |
| Vesting migration | prepared PermanentLocked fixture and `multisig-vesting` mode | Covered |
| Replay rejection | Integration tests; not explicit devnet | Partially covered |
| Invalid proof rejection | Integration/script tests; not explicit devnet | Partially covered |
| Validator migration with delegations/redelegations/unbonding | `migrate-validator`, `migrate-all`, verify scan | Covered |
| Validator distribution/commission and supernode linkage | verify scan | Covered |
| IBC transfer in EVM mode | Hermes `TestIBCTransferWithEVMModeStillRelays` | Covered |
| Wrong-provenance ERC20 rejection | Integration only | Missing |
| JSON-RPC from multiple validators | `TestEVMTransactionVisibleAcrossPeerValidator` | Covered |
| JSON-RPC restart persistence | Integration only | Missing |
| WebSocket subscriptions | Integration only | Missing |
| Public JSON-RPC rate-limit profile | Unit only | Missing |
| Contract deploy/call/logs | Integration only | Missing |
| Precompile tx/query paths | Integration only | Missing |

### TEST-01: Devnet inventory doc is stale and understates/omits scenarios

Severity: Medium

Evidence:

- `docs/evm-integration/testing/tests/devnet.md` lists only three EVM devnet tests.
- `devnet/tests/validator/evm_test.go` actually contains eight EVM JSON-RPC/fee/tx/cross-peer tests.
- `devnet/tests/evmigration/main.go` lists additional modes: `migrate-all`, `verify`, `multisig-vesting`, and `multisig-validator`, but `devnet/tests/evmigration/README.md` names a non-existent relative doc path.

Recommended fix:

- Regenerate `docs/evm-integration/testing/tests/devnet.md` from current devnet test names and modes.
- Add explicit rows for missing devnet scenarios instead of implying devnet coverage is complete.
- Fix `devnet/tests/evmigration/README.md` to link to `docs/evm-integration/evmigration/devnet-tests.md`.

## Command Results

| Command | Result |
| --- | --- |
| `go version` | `go version go1.26.2 linux/amd64` |
| `go test -tags=test ./app ./app/evm ./config ./precompiles/... ./x/evmigration/... ./x/erc20policy/...` | Passed after the `evm-audit` precisebank test fixes |
| `go test -tags=test ./app/evm ./x/evmigration/keeper` | Passed with focused ante proof and CheckTx coverage |
| `make lint-scripts` | Passed |
| `make test-scripts` | Passed, 119 Bats tests |
| `go test -tags='integration test' ./tests/integration/evm/... ./tests/integration/evmigration/... -run 'TestNonExistent'` | Passed compile/no-test smoke |
| `make integration-tests NOCACHE=1` | Interrupted after one full-suite attempt failed in `tests/integration/evm/contracts`: `TestContractCodePersistsAcrossRestart` hit `send legacy tx ... exceeds block gas limit`; follow-up focused and package-level contracts runs passed |
| `go test -tags='integration test' ./tests/integration/evm/contracts -run TestContractCodePersistsAcrossRestart -count=1 -v` | Passed |
| `go test -tags='integration test' ./tests/integration/evm/contracts -count=1` | Passed in 262.693s |
| `make unit-tests NOCACHE=1` | Not run separately because the narrower EVM baseline already failed in `app` |
| `make simulation-tests` | Not run in this pass; should run after baseline unit failure is resolved |
| `make devnet-evm-upgrade` | Not run in this pass; Docker devnet run is long and should be scheduled as release-gate validation |

## Recommended Backlog

1. Re-run full `make integration-tests NOCACHE=1`; the earlier contracts failure did not reproduce in focused or package-level runs.
2. Add devnet scenarios for rate-limit profile, WebSocket, restart persistence, contracts, precompiles, and ERC20 wrong-provenance rejection.
3. Run `make simulation-tests` and `make devnet-evm-upgrade` as release-gate validation after the focused fixes land.
