# Integration Tests: EVM Migration

Purpose: end-to-end integration tests for the `x/evmigration` module using real keepers wired via `app.Setup(t)`.
File: `tests/integration/evmigration/migration_test.go`
Run: `go test -tags=test ./tests/integration/evmigration/... -v`

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
| `TestClaimLegacyAccount_AfterValidatorMigration` | Fresh-state validator-first flow: migrate validator first, then migrate a legacy delegator account. |
| `TestMigrateValidator_Success` | End-to-end validator migration: bonded validator with self-delegation + external delegator. |
| `TestMigrateValidator_NotValidator` | Rejection when legacy address is not a validator operator with real staking state. |
| `TestMigrateValidator_JailedValidator` | Rejection when validator is jailed with real staking/auth state; asserts no migration record or destination validator is created. |
| `TestQueryMigrationRecord_Integration` | Query server returns record after real migration, nil before. |
| `TestQueryMigrationEstimate_Integration` | Estimate query with real staking state reports correct values. |
