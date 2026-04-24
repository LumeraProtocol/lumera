# EVM Integration — Test Inventory

Complete test catalog for Lumera's Cosmos EVM integration.
See [main.md](main.md) for architecture, app changes, and operational details.

---

## Executive Summary

Lumera ships **~470 EVM-related tests** spanning unit, integration, and devnet levels — the most comprehensive pre-mainnet EVM test suite in the Cosmos ecosystem. For context:

- **Evmos** — the first Cosmos EVM chain — launched mainnet with primarily unit tests and a handful of end-to-end scripts; their integration test suite was built incrementally *after* mainnet issues surfaced (e.g., the zero-base-fee spam incident).
- **Kava** — relied heavily on simulation tests and manual QA for their EVM launch; structured integration tests came later.
- **Cronos** — forked Ethermint and inherited its test base but added few chain-specific integration tests before launch.

Lumera's suite goes beyond any of these baselines **before** mainnet:

| Capability                                                                       | Lumera                                  | Typical Cosmos EVM chain at launch        |
| -------------------------------------------------------------------------------- | --------------------------------------- | ----------------------------------------- |
| Dual-route ante handler tests (EVM + Cosmos path)                                | 28 unit + 3 integration                 | Rarely tested separately                  |
| App-side mempool (ordering, nonce gaps, replacement, capacity, WS subscriptions, metrics) | 16 integration + 10 unit (metrics)      | None (relies on CometBFT mempool)         |
| Async broadcast queue (deadlock prevention)                                      | 4 unit                                  | Not applicable (novel to Lumera)          |
| JSON-RPC batching, persistence across restart                                    | 23 integration                          | Basic RPC smoke tests                     |
| ERC20/IBC middleware (v1 + v2 stacks)                                            | 7 integration + 14 unit (policy)        | Partial or post-launch                    |
| Precisebank (6↔18 decimal bridge)                                               | 39 unit + 6 integration                 | Not applicable (novel to Lumera)          |
| Feemarket (EIP-1559)                                                             | 9 unit + 8 integration                  | Inherited from upstream, rarely augmented |
| Precompile coverage (11 precompiles + gas metering + action + supernode + wasm)   | 42+ integration                         | Smoke-level                               |
| Account migration (coin-type 118→60)                                            | 116 unit + 14 integration + devnet tool | Not applicable (novel to Lumera)          |
| OpenRPC discovery + spec sync                                                    | 15 unit + 2 integration                 | No chain has this                         |
| WebSocket subscriptions (newHeads, logs, pending)                                | 4 integration                           | Untested or manual                        |
| Cross-runtime bridge (CosmWasm ↔ EVM)                                           | 12 integration + 31 unit + 15 crossruntime unit | No chain has this              |
| Devnet multi-validator E2E                                                       | 12+ devnet tests                        | Manual or ad-hoc scripts                  |

Three areas are **unique to Lumera** with no equivalent in any other Cosmos EVM chain: the async broadcast queue (solving the CometBFT/EVM mempool deadlock), the precisebank 6↔18 decimal bridge, and the full account migration module. Each has dedicated test coverage. The cross-runtime bridge (CosmWasm ↔ EVM) is also unique — no other chain has both runtimes, let alone a tested bridge between them.

All three previously identified critical test gaps (mempool capacity pressure, batch JSON-RPC, WebSocket subscriptions) have been closed.

---

## Test Coverage Assessment

### Coverage by area

| Category        | Area                                 | Tests | Coverage quality |
| --------------- | ------------------------------------ | ----- | ---------------- |
| **Unit**        | App wiring/config/genesis/commands   | 72    | Excellent — [details](tests/unit-app-wiring.md) |
| **Unit**        | EVM ante decorators                  | 28    | Excellent — [details](tests/unit-ante.md) |
| **Unit**        | EVM module/config guard/genesis      | 7     | High — [details](tests/unit-evm-config.md) |
| **Unit**        | Fee market                           | 9     | Excellent — [details](tests/unit-feemarket.md) |
| **Unit**        | Precisebank                          | 39    | Excellent — [details](tests/unit-precisebank.md) |
| **Unit**        | OpenRPC / generator                  | 15    | High — [details](tests/unit-openrpc.md) |
| **Unit**        | JSON-RPC rate limiting               | 25    | High — right-to-left XFF parsing, trusted-hop skipping, CIDR parsing |
| **Unit**        | ERC20 policy                         | 14    | High — 3 modes, base denom + exact ibc/ allowlist CRUD |
| **Unit**        | EVMigration keeper                   | 116+  | Excellent — [details](tests/unit-evmigration.md) |
| **Unit**        | EVMigration types (proof)            | 6     | High — `TestMultisigProof_ValidateBasic`, `TestMultisigProof_ValidateParams_SizeCap`, `TestLegacyProof_ValidateBasic_Dispatch`, `TestSingleKeyProof_ValidateBasic` and variants |
| **Unit**        | EVMigration CLI                      | 26    | High — [details](tests/unit-evmigration-cli.md) |
| **Unit**        | Cross-runtime bridge (plugin helpers + crossruntime) | 46 | High — [details](tests/integration-precompiles.md#cosmwasm---evm-plugin-unit-tests) |
|                 |                                      |       |                  |
| **Integration** | Ante                                 | 3     | Medium — [details](tests/integration-ante.md) |
| **Integration** | Contracts                            | 15    | High — [details](tests/integration-contracts.md) |
| **Integration** | Fee market                           | 8     | Excellent — [details](tests/integration-feemarket.md) |
| **Integration** | IBC ERC20                            | 7     | High — [details](tests/integration-ibc-erc20.md) |
| **Integration** | JSON-RPC / indexer                   | 23    | Very high — [details](tests/integration-jsonrpc.md) |
| **Integration** | Mempool                              | 16    | High — [details](tests/integration-mempool.md) |
| **Integration** | Precisebank                          | 6     | High — [details](tests/integration-precisebank.md) |
| **Integration** | Precompiles (standard + custom + wasm) | 42   | High — [details](tests/integration-precompiles.md) |
| **Integration** | VM queries / state                   | 12    | High — [details](tests/integration-vm.md) |
| **Integration** | EVMigration                          | 14+   | High — [details](tests/integration-evmigration.md) |
|                 |                                      |       |                  |
| **Devnet**      | EVM / fee market / cross-peer / IBC  | 12+   | High — [details](tests/devnet.md) |
| **Devnet**      | EVMigration tool                     | 7 modes | High — [details](tests/devnet.md#evm-migration-devnet-tests) |
|                 |                                      |       |                  |
|                 | **Totals**                           | **Unit: ~397 · Integration: ~146 · Devnet: 12+ · Total: ~555** | |

### Gaps and next steps

**Moderate test gaps** — all previously moderate gaps have been addressed:

- ~~Precompile gas metering accuracy validation~~ — Covered by `PrecompileGasMeteringAccuracy` and `PrecompileGasEstimateMatchesActual`
- ~~Multi-validator EVM consensus scenarios~~ — Single-node integration framework validates cross-block state consistency; multi-validator coverage deferred to devnet systemtests
- ~~Chain upgrade with EVM state preservation~~ — Covered by `TestEVMStatePreservationAcrossRestart`
- ~~Concurrent operation race condition detection~~ — Covered by `TestConcurrentMixedEVMOperations`
- ~~ERC20 allowance/transferFrom/approve flows~~ — Covered by `TestERC20ApproveAllowanceTransferFrom`

**Recommended next steps** — see [Recommended Next Steps](#recommended-next-steps) below.

### Key architectural strengths

1. **Async broadcast queue** — Novel solution to the cosmos/evm mempool deadlock. Decouples txpool promotion from CometBFT `CheckTx` via bounded channel + single background worker.
2. **Min gas price floor** — Prevents base fee decay to zero on quiet chains (Evmos experienced spam attacks from this).
3. **Tracing + rate limiting already implemented** — Runtime-configurable EVM tracing and app-layer JSON-RPC per-IP rate limiting are integrated now, not deferred.
4. **Governance-controlled IBC voucher ERC20 policy** — Three-mode policy (`all`/`allowlist`/`none`) for auto-registration risk control.
5. **Dual CosmWasm + EVM runtime with cross-runtime bridge** — Unique among Cosmos EVM chains. Bidirectional bridge (Wasm Precompile + custom handlers) enables Solidity↔CosmWasm contract interaction.
6. **IBC v1 + v2 ERC20 middleware** — Both transfer stack versions have ERC20 token registration middleware.
7. **OpenRPC discovery** — Machine-readable API spec with build-time synchronization. Unique across all Cosmos EVM chains.
8. **Account migration module** — Purpose-built `x/evmigration` for coin-type-118-to-60 transition with dual-signature verification and atomic state migration across 9 SDK modules.

### Bottom line

Lumera's EVM integration is **architecturally excellent and feature-complete** for its current scope, and it is already ahead in several operator-facing areas (tracing, rate limiting, governance-controlled ERC20 voucher policy, mempool hardening, and cross-runtime bridge). Security audit, CORS origin lockdown, and JSON-RPC namespace exposure profiles are all complete. The main remaining gap versus mature production Cosmos EVM chains is **ecosystem surface**: monitoring runbook and external block explorer.

---

## Detailed Test Tables

Each area has its own detailed file with per-test descriptions:

### Unit Tests

| Area | File | Tests |
| ---- | ---- | ----- |
| App wiring, config, genesis, commands | [unit-app-wiring.md](tests/unit-app-wiring.md) | 72 |
| EVM ante decorators | [unit-ante.md](tests/unit-ante.md) | 28 |
| EVM module/config guard/genesis | [unit-evm-config.md](tests/unit-evm-config.md) | 7 |
| Fee market (EIP-1559) | [unit-feemarket.md](tests/unit-feemarket.md) | 9 |
| Precisebank (6↔18 bridge) | [unit-precisebank.md](tests/unit-precisebank.md) | 39 |
| OpenRPC & generator | [unit-openrpc.md](tests/unit-openrpc.md) | 15 |
| EVMigration keeper | [unit-evmigration.md](tests/unit-evmigration.md) | 116+ |
| EVMigration types (proof) | `x/evmigration/types/proof_test.go` | 6 |
| EVMigration CLI | [unit-evmigration-cli.md](tests/unit-evmigration-cli.md) | 26 |

### Integration Tests

| Area | File | Tests |
| ---- | ---- | ----- |
| Ante handler | [integration-ante.md](tests/integration-ante.md) | 3 |
| Contract lifecycle | [integration-contracts.md](tests/integration-contracts.md) | 15 |
| Fee market (EIP-1559) | [integration-feemarket.md](tests/integration-feemarket.md) | 8 |
| IBC ERC20 middleware | [integration-ibc-erc20.md](tests/integration-ibc-erc20.md) | 7 |
| JSON-RPC & indexer | [integration-jsonrpc.md](tests/integration-jsonrpc.md) | 23 |
| Mempool | [integration-mempool.md](tests/integration-mempool.md) | 16 |
| Precisebank | [integration-precisebank.md](tests/integration-precisebank.md) | 6 |
| Precompiles (standard + custom + wasm + crossruntime) | [integration-precompiles.md](tests/integration-precompiles.md) | 42 |
| VM queries / state | [integration-vm.md](tests/integration-vm.md) | 12 |
| EVMigration | [integration-evmigration.md](tests/integration-evmigration.md) | 14+ |

### Devnet Tests

| Area | File | Tests |
| ---- | ---- | ----- |
| EVM, fee market, cross-peer, IBC, migration | [devnet.md](tests/devnet.md) | 12+ |
| EVMigration multisig CLI flow | `devnet/tests/evmigration/multisig.go` | 1 mode |

### Multisig support tests (added with multisig feature)

The tables below list the individual tests added for multisig proof support. They supplement the counts in the rows above.

#### Unit — verifier (`x/evmigration/keeper/verify_test.go`)

Names were renamed in the v2/MigrationProof refactor; legacy `TestVerifyLegacyProof_Multisig_*` entries were merged into the dual-side `TestVerifyMigrationProof_*` suite.

| Test | Description |
| ---- | ----------- |
| `TestVerifyMigrationProof_NewSide_Multisig_Valid2of3` | 2-of-3 new-side multisig over eth sub-keys passes the verifier. |
| `TestVerifyMigrationProof_NewSide_Multisig_AminoAddressMismatch_OnKeyTypeSwap` | Proof whose `LegacyAminoPubKey` address does not match `new_address` is rejected (catches key-type confusion between Cosmos and eth sub-keys). |
| `TestVerifyMigrationProof_NewSide_Multisig_SubSigInvalid_UnderCosmosKeyBytes` | Sub-signature over Cosmos-key-bytes fails verification on eth-side (format-boundary test). |
| Legacy-side multisig verifier paths | Exercised in integration via `TestClaimLegacyAccount_Multisig_*`; unit-level coverage replaced by `TestVerifyMigrationProof_*`. |

#### Unit — type validation (`x/evmigration/types/proof_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestSingleKeyProof_ValidateBasic` | Valid and invalid `SingleKeyProof` shapes (nil pub_key, nil sig, unspecified format). |
| `TestMultisigProof_ValidateBasic` | Valid and invalid `MultisigProof` shapes (zero threshold, mismatched indices/sigs length, non-ascending indices, wrong sub-key size, unspecified format). |
| `TestMultisigProof_ValidateParams_SizeCap` | `ValidateParams` rejects when `len(sub_pub_keys) > MaxMultisigSubKeys`. |
| `TestMultisigProof_ValidateBasic_RejectsDuplicateSubKeys` | Rejects a `sub_pub_keys` list containing pairwise-duplicate entries with `ErrInvalidMigrationPubKey`. Prevents one keyholder from being counted as two distinct signers toward K-of-N. |
| `TestMigrationProof_ValidateBasic_Dispatch` | `MigrationProof.ValidateBasic` dispatches to the correct sub-validator and rejects a nil oneof. Side-aware (SideLegacy / SideNew). |
| `TestValidateProofPair_MirrorSourceRule` | 6-case matrix enforcing the cross-side mirror-source invariant: single↔single ok, multi↔multi with matching K/N ok, shape mismatch (single↔multi or multi↔single) rejected, K mismatch rejected, N mismatch rejected — all failures return `ErrMirrorSourceMismatch` (code 1121). |
| `TestValidateProofPair_SignerIndicesMustMatch` | Multi-multi pair where `legacy_proof.signer_indices != new_proof.signer_indices` is rejected — catches two disjoint K-subsets each authorizing one side. |
| `TestValidateProofPair_NilInputsReturnErrorNotPanic` | Defensive nil-guards on nil proofs, typed-nil oneof wrappers, and nil inner `MultisigProof` — direct callers outside ValidateBasic can't panic the helper. |
| `TestMsgClaimLegacyAccount_ValidateBasic_MirrorSource` | Shape-mismatch (multi↔single) routes through full `Msg*.ValidateBasic` path and rejects with `ErrMirrorSourceMismatch`. Validator-message equivalent exists as `TestMsgMigrateValidator_ValidateBasic_MirrorSource`. |
| `TestMsgClaimLegacyAccount_ValidateBasic_SignerIndicesMismatch` | Cross-side `signer_indices` mismatch routes through full `Msg*.ValidateBasic`. Validator equivalent: `TestMsgMigrateValidator_ValidateBasic_SignerIndicesMismatch`. |
| `TestMsgClaimLegacyAccount_ValidateBasic_DuplicateSubKeys` | Duplicate legacy-side sub-keys routes through full `Msg*.ValidateBasic`. Validator equivalent: `TestMsgMigrateValidator_ValidateBasic_DuplicateSubKeys` (exercises new-side duplicate). |

#### Unit — query server (`x/evmigration/keeper/query_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestLegacyAccounts_Multisig` | `LegacyAccounts` response includes `is_multisig=true`, correct `threshold` and `num_signers` for a multisig account. |
| `TestMigrationEstimate_Multisig_Supported` | Estimate returns `would_succeed=true` for a valid K-of-N secp256k1 multisig. |
| `TestMigrationEstimate_Multisig_TooManySubKeys` | Estimate returns `would_succeed=false` when `num_signers > MaxMultisigSubKeys`. |
| `TestMigrationEstimate_Multisig_NonSecp256k1` | Estimate returns `would_succeed=false` when any sub-key is not secp256k1. |
| `TestMigrationEstimate_Multisig_DuplicateSubKey` | Estimate returns `would_succeed=false` when any two sub-key entries are byte-equal — preflight mirror of `MultisigProof.validateBasic`'s consensus-level duplicate rejection, so operators don't run a K-of-N ceremony that would fail at submit. |

#### Integration (`tests/integration/evmigration/migration_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestClaimLegacyAccount_Multisig_Success` | End-to-end 2-of-3 multisig migration: balances move, migration record stored. |
| `TestClaimLegacyAccount_Multisig_ADR036` | ADR-036 sig format path for multisig. |
| `TestClaimLegacyAccount_Multisig_ReplayRejected` | Second migration attempt on same multisig address is rejected. |
| `TestClaimLegacyAccount_Multisig_CorruptedSubSig` | Corrupted sub-signature causes rejection with appropriate error. |
| `TestClaimLegacyAccount_MultisigToMultisig` | End-to-end 2-of-3 Cosmos-multisig → 2-of-3 eth-multisig migration: destination derived from `kmultisig.NewLegacyAminoPubKey` over eth sub-keys; balances move; migration record stored; `MigrateAuth` sets the new multisig `LegacyAminoPubKey` on-chain. |
| `TestMigrateValidator_MultisigToMultisig` | `MsgMigrateValidator` variant of the multisig→multisig flow: validator operator re-keys to the eth multisig; `x/staking`, `x/distribution`, `x/supernode` state re-keyed to the new operator bech32. |
| `TestClaimLegacyAccount_MultisigVesting_ToMultisig` | PermanentLocked vesting account owned by a legacy multisig migrates to an eth multisig while preserving the vesting schedule and locked amount. |
| `TestClaimLegacyAccount_Multisig_WrongThreshold_LegacySide` | Truncated `signer_indices` on the legacy side (K=2 claimed but only 1 entry supplied) rejected via `MultisigProof.validateStructure` with `expected exactly K=... signer_indices`. |
| `TestClaimLegacyAccount_Multisig_WrongThreshold_NewSide` | Truncated `signer_indices` on the new side (K=2 claimed but only 1 entry supplied) rejected via `MultisigProof.validateStructure`. |
| `TestClaimLegacyAccount_Multisig_ADR036_BothSides` | ADR-036 sig format accepted on both legacy and new sides for a multisig→multisig migration. |
| `TestClaimLegacyAccount_Multisig_MirrorSourceMismatch_Shape` | Cross-side shape mismatch (multisig legacy + single-key new) rejected with `ErrMirrorSourceMismatch` via the full `Msg*.ValidateBasic` path — exercises the pair check that production's msg-service-router auto-invokes before dispatch. |
| `TestClaimLegacyAccount_Multisig_MirrorSourceMismatch_KN` | 2-of-3 legacy → 3-of-5 new — same shape, mismatched K and N — rejected with `ErrMirrorSourceMismatch`. Distinct from `WrongThreshold_*` tests which exercise single-side `signer_indices` truncation. |
| `TestClaimLegacyAccount_Multisig_SignerIndicesMismatch` | Cross-side disjoint K-subsets (legacy signed at `[0,1]`, new at `[0,2]`) rejected with `ErrMirrorSourceMismatch` carrying `"signer_indices"` in the message. |
| `TestClaimLegacyAccount_Multisig_DuplicateSubKey_Submit` | Duplicate sub-key (position 0 repeated at position 2 on the legacy side) rejected with `ErrInvalidMigrationPubKey` + `"duplicates sub_pub_keys[0]"` — complements preflight coverage in `TestMigrationEstimate_Multisig_DuplicateSubKey`. |
| `TestQueryMigrationEstimate_Multisig_Success` | `MigrationEstimate` returns `would_succeed=true` and estimated gas for a supported multisig source. |
| `TestQueryMigrationEstimate_Multisig_SizeCapped` | `MigrationEstimate` returns `would_succeed=false` when `num_signers > MaxMultisigSubKeys`. |
| `TestQueryMigrationEstimate_Multisig_NonSecp256k1SubKey` | `MigrationEstimate` returns `would_succeed=false` when any legacy sub-key is not secp256k1. |

#### Unit — CLI multisig (`x/evmigration/client/cli/tx_multisig_internal_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestCLI_MultisigToMultisig_EndToEnd` | End-to-end in-process CLI walkthrough: `generate-proof-payload` → `sign-proof` (per co-signer, both sides) → `combine-proof` → `submit-proof` produces a well-formed tx with zero envelope signatures. |
| `TestBuildProofFromPartial_*` | Per-side test-helper suite (independent K selection). Covers valid 2-of-3, drops invalid with warnings, below-threshold-after-drops errors with `need <K> valid partial signatures, have <N>`, out-of-range index dropped, bad base64 dropped, duplicate-index deduped, wrong pubkey length rejected. |
| `TestBuildMigrationProofs_IntersectsIndicesAcrossSides` | Production `combine-proof` dispatcher picks **intersected** indices: legacy signed at `[0,1,2]`, new signed at `[1,2]` → assembled proofs share `signer_indices=[1,2]` on BOTH sides (not legacy=`[0,1]` vs new=`[1,2]`). Locks in the cross-side intersection that makes tx output pass the mirror-source rule. |
| `TestBuildMigrationProofs_IntersectionBelowThreshold` | Empty intersection (legacy at `[0,1]`, new at `[2]`, K=2) — dispatcher errors with `signed on BOTH sides at matching indices`. |
| `TestBuildMigrationProofs_IntersectionHasOneButNeedsK` | Non-empty-but-below-K intersection (legacy at `[0,1]`, new at `[0,2]`, K=2, shared=`{0}`) — rejects "have 1". Pins the off-by-one where `len(intersection) > 0` might be mistakenly treated as "enough." |
| `TestBuildMigrationProofs_RejectsMixedShape` | Single-key legacy + multisig new (or vice versa) is caught at combine time, before writing a tx.json that would fail `ValidateBasic.ValidateProofPair`. |
| `TestValidateSideSpec_RejectsDuplicateSubKeys` | A 2-of-3 SideSpec where positions 0 and 2 carry the same base64 sub-key is rejected by `validateSideSpec` — catches duplicates on both `generate-proof-payload` authoring and `LoadPartialProof`. |
| `TestCmdSubmitProof_DoesNotExposeSigningFlags` | Flag-surface lock: `--from`, `--fees`, `--fee-payer`, `--fee-granter`, `--gas`, `--gas-adjustment`, `--gas-prices`, `--sign-mode`, `--offline`, `--generate-only` are NOT advertised on `submit-proof` (zero-signer command); `--node`, `--chain-id`, `--keyring-backend`, `--keyring-dir`, `--broadcast-mode`, `--yes`, `--tx-timeout` ARE. |
| `TestVerifyMigrationProof_NewSide_Multisig_*` | Keeper-side verification of the new-side multisig proof: valid K-of-N passes, wrong threshold / duplicate indices / non-ascending indices / non-eth-secp256k1 sub-key all rejected with specific errors. |

#### Devnet (`devnet/tests/evmigration/`)

| Mode | Description |
| ---- | ----------- |
| `tests_evmigration -mode=multisig` | Exercises the full four-step offline CLI flow: `generate-proof-payload` → `sign-proof` (per co-signer) → `combine-proof` → `submit-proof`. Verifies the migration record on-chain after broadcast. |
| `tests_evmigration -mode=multisig` (multisig→multisig) | 2-of-3 Cosmos-multisig → 2-of-3 eth-multisig balance migration; verifies `MigrationRecord` on-chain and asserts `MigrateAuth` set the new multisig `LegacyAminoPubKey` on the destination `BaseAccount`. |
| `tests_evmigration -mode=multisig-vesting` | PermanentLocked vesting multisig source → eth multisig destination; verifies vesting schedule, original vesting amount, and locked balance are preserved after `MigrateAuth` rewrites the account. |
| `tests_evmigration -mode=multisig-validator` | Multisig validator operator → eth multisig re-key via `MsgMigrateValidator`; post-migration sanity check submits `MsgEditValidator` from the new multisig to assert all Cosmos-side operator ops work end-to-end. |

#### BATS — wrapper scripts (`tests/scripts/`)

| Suite | Coverage |
| ----- | -------- |
| `tests/scripts/common.bats` | 61 tests: logging, flag parsing, keyring flag passthrough, multisig auth-type routing, `auth_pubkey_type`, `assert_secp256k1_key` / `assert_eth_key`, `read_proof_file` happy-path + missing-field + payload-hex-mismatch + single-key-rejection + out-of-range partial, `read_migration_tx_file` for multisig + single-key rejection, `summarize_partials` per-side + **matching-index intersection gate** (fails when legacy `[0,1]` and new `[0,2]` are presented — per-side thresholds both "yes" but shared count below K), cross-file chain-id mismatch. |
| `tests/scripts/migrate-multisig.bats` | 38 tests: subcommand dispatch; `submit` dry-run / happy path / rejects `--from` as unknown flag / validator-downtime ack / exit 3 on non-multisig tx / exit 4 on estimate flip; `generate` happy path (claim + validator) / exit 8 on nil pubkey / exit 3 on single-sig / exit 6 on kind-validator-on-non-validator / exit 4 on estimate / exit 5 on already-migrated / missing required flags; `sign` happy path / tampered payload / v1-proof-file rejection / key-not-in-sub-key-set / eth-key-as-legacy / exit-1 on no `--from`/`--new-key`; `combine` matching-index matrix output / sub-threshold exit 4 / cross-file consistency exit 9 / lumerad below-threshold mapping / flag wiring. |

---

### Known multisig test-coverage gaps

Coverage at unit, preflight, and integration layers is complete for all consensus invariants. One remaining gap, low priority:

| # | Gap | Why it matters |
| --- | --- | --- |
| 1 | `tests_evmigration -mode=multisig-large-kn` | Devnet mode exercising a larger K/N combination (e.g. 5-of-7). All current devnet modes are 2-of-3; a larger case would stress governance-param interaction and sub-key fixture generation at scale. **Low** priority — unit `MigrationEstimate_Multisig_TooManySubKeys` already exercises the cap boundary, and `TestQueryMigrationEstimate_Multisig_SizeCapped` at integration covers the reject-at-21 case. |

---

## Recommended Next Steps

### High priority (before mainnet)

1. ~~**Security audit of EVM integration layer**~~ — DONE. See [security-audit.md](security-audit.md).
2. ~~**Production JSON-RPC hardening profile**~~ — DONE. CORS origin lockdown (`app/openrpc/http.go`), namespace exposure lockdown (`cmd/lumera/cmd/jsonrpc_policy.go`), rate limiter fixed to front public port (Bug #20).
3. **External block explorer integration** — Blockscout or Etherscan-compatible explorer. All comparable chains have this at mainnet.

### Medium priority

1. ~~**CosmWasm + EVM interaction design**~~ — DONE. Bidirectional cross-runtime bridge implemented: WasmPrecompile (0x0903) for EVM->CosmWasm, custom message/query handlers for CosmWasm->EVM. Phase 1 is non-payable with depth-1 reentrancy guard. See `precompiles/wasm/` and `app/wasm_evm_plugin.go`.
2. **Ops monitoring runbook** — Document fee market monitoring (base fee tracking, gas utilization trends), alerting thresholds, and common failure mode diagnosis.
3. **EVM governance proposals** — Mechanism to toggle precompiles and adjust EVM params via on-chain governance (Evmos has dedicated governance proposals for this).

### Low priority

1. **Multi-validator EVM consensus scenarios** — Expand devnet tests beyond single-validator assertions.
2. **ERC20 provenance policy tests** — Add tests for "same base denom, different IBC trace" to validate admission policy (security audit Finding #3).
