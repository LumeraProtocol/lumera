# Integration Tests: EVM Migration

Purpose: end-to-end integration tests for the `x/evmigration` module using real keepers wired via `app.Setup(t)`.
File: `tests/integration/evmigration/migration_test.go`
Run: `go test -tags=test ./tests/integration/evmigration/... -v`

Additional real-node broadcast coverage for zero-signer `submit-proof` txs lives in the EVM mempool suite:
`tests/integration/evm/mempool/evmigration_zero_signer_test.go`. Those tests start a `lumerad` node, wait for height 1, and submit encoded tx bytes through CometBFT `broadcast_tx_sync`.

| Test | Description |
| --- | --- |
| `TestClaimLegacyAccount_Success` | End-to-end migration: balances move, migration record stored, counter incremented. |
| `TestClaimLegacyAccount_MigrationDisabled` | Rejection when enable_migration is false with real params. |
| `TestClaimLegacyAccount_AlreadyMigrated` | Double migration and NewAddressWasMigrated with real state. |
| `TestClaimLegacyAccount_SameAddress` | Rejection when legacy and new addresses are identical. |
| `TestClaimLegacyAccount_InvalidSignature` | Rejection with a bad legacy signature against real auth state. |
| `TestClaimLegacyAccount_ValidatorMustUseMigrateValidator` | Validator operators rejected from ClaimLegacyAccount with real staking state. |
| `TestClaimLegacyAccount_MultiDenom` | Multi-denomination balance transfer verified with real bank module. |
| `TestClaimLegacyAccount_LegacyAccountRemoved` | Legacy auth account removed and new account exists after migration. |
| `TestClaimLegacyAccount_LeavesClaimRecordUntouched` | Migration does not re-key claim records: a seeded claim record whose DestAddress points at the legacy address is left unchanged (claim DB is reference-only). |
| `TestClaimLegacyAccount_AfterValidatorMigration` | Fresh-state validator-first flow: migrate validator first, then migrate a legacy delegator account. |
| `TestMigrateValidator_Success` | End-to-end validator migration: bonded validator with self-delegation + external delegator. |
| `TestMigrateValidator_NotValidator` | Rejection when legacy address is not a validator operator with real staking state. |
| `TestMigrateValidator_JailedValidator` | Rejection when validator is jailed with real staking/auth state; asserts no migration record or destination validator is created. |
| `TestMigrateValidator_UnbondedNotJailedSucceeds` | Recovery path: an Unbonded, non-jailed validator (fell out of the active set on stake weight) migrates successfully with full re-keying of the validator record, delegations, distribution, and counters. |
| `TestMigrateValidatorAtScale` | Scale correctness gate (`migration_scaling_test.go`): migrates a validator with ~6000 records in the realistic testnet val1 mix (~0.42 deleg / 0.19 unbond / 0.39 redel) and asserts every delegation, unbonding delegation, and source-role redelegation is re-keyed. Guards the PR #184 hot-path optimizations against scale-only regressions; deterministic, no wall-clock assertion. See the scaling benchmark below for on-demand measurement. |
| `TestQueryMigrationRecord_Integration` | Query server returns record after real migration, nil before. |
| `TestQueryMigrationEstimate_Integration` | Estimate query with real staking state reports correct values. |
| `TestEVMigrationZeroSignerTxBroadcastSyncWithMempoolEnabled` | Mempool-suite regression: valid zero-signer migration tx passes real-node CheckTx with app-side mempool enabled. |
| `TestEVMigrationProofValidNonexistentLegacyAccountRejectedByAnte` | Mempool-suite negative test: proof-valid zero-signer migration tx is rejected by ante state admission when the legacy account does not exist. |
| `TestEVMigrationMalformedLegacyAddressRejectedByValidateBasic` | Mempool-suite negative test: malformed `legacy_address` is rejected by `ValidateBasic` in the ante chain on the real-node broadcast path (before mempool admission). |
| `TestZeroSignerNonMigrationBroadcastSyncStillRejected` | Mempool-suite negative control: zero-signer non-migration tx remains rejected. |

## Scaling test + benchmark

File: `tests/integration/evmigration/migration_scaling_test.go`

This file carries two things that share one seeding harness:

- **`TestMigrateValidatorAtScale`** (runs in the pipeline) — a plain `Test` that migrates a ~6000-record validator and asserts full re-keying. This is the CI regression gate for scale correctness (listed in the table above). It makes no wall-clock/timing assertion, so it is deterministic and does not flake.
- **`BenchmarkMigrateValidatorScaling`** (on demand) — measures how the cost *scales* across sizes.

`BenchmarkMigrateValidatorScaling` measures how `MsgMigrateValidator` scales with the migrating validator's own footprint — the number of delegation / unbonding-delegation / redelegation records the hot path re-keys. It reproduces the live testnet scenario (a validator with thousands of records) that motivated the scoped-iteration / double-fetch / O(1)-refcount optimizations in PR #184, seeding a realistic mix (~0.42 deleg / 0.19 unbond / 0.39 redel, the observed testnet val1 ratio) via the real `StakingKeeper` and timing only the migration call.

The **benchmark** is **excluded from the normal pipeline by construction**: `go test` runs no `Benchmark*` without an explicit `-bench`, and `make integration-tests` passes none. Run it on demand (pin one measured iteration per size, since seeding thousands of records is slow):

```bash
go test -tags='integration test' ./tests/integration/evmigration/ \
    -run='^$' -bench='^BenchmarkMigrateValidatorScaling$' -benchtime=1x -timeout=30m -v
```

### Representative results

Single-iteration run (`-benchtime=1x`, linux/amd64, AMD Ryzen 9 5900X):

| records | ns/op | ms/op | µs/record |
| ------: | ----------: | ----: | --------: |
| 1000 | 40,987,018 | 41.0 | 41.0 |
| 2000 | 75,297,933 | 75.3 | 37.6 |
| 4000 | 147,209,620 | 147.2 | 36.8 |
| 6000 | 240,509,053 | 240.5 | 40.1 |

**Analysis:** cost is **linear** in the validator's own record count — per-record cost is flat at ~38 µs/record across a 6× range and each doubling of N roughly doubles the wall time (slope ≈ 1.0). This is `ns/op` in a gas-meterless, consensus-free harness, so it reflects *algorithmic shape* rather than on-chain gas or block time; it corroborates the live devnet finding of linear ~688k gas/record (see [`docs/design/2026-06-22-validator-migration-gas-design.md`](../../../design/2026-06-22-validator-migration-gas-design.md)). The `TestMigrateValidatorAtScale` pipeline gate runs the 6000-record row (~1.8 s wall clock including seeding).

Notes:
- Sizes swept: 1000 / 2000 / 4000 / 6000 records. The reported `records/op` custom metric is the deterministic re-keyed total; wall-clock `ns/op` is indicative only (the in-process harness has no gas meter or consensus).
- Seeding advances block time per unbonding/redelegation record so their completion times spread across distinct staking-queue buckets, mirroring a real chain. Without this, all records share one completion-time bucket and the migration's `InsertUBDQueue` / `InsertRedelegationQueue` re-serialize the whole bucket per entry (O(N²)) — a seeding artifact that masks the true linear per-record cost.
- Each iteration asserts full re-keying (`assertMigrated`): every delegation, unbonding delegation, and source-role redelegation moves off the old valoper onto the new one, and the migration record is stored.
