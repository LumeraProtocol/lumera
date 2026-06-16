# gen-activity: migrate mode for generated live-chain accounts

**Date:** 2026-06-16
**Status:** Approved (design)
**Tools:** `devnet/tests/gen-activity`, `devnet/tests/evmigration`

## 1. Goal

Add an explicit migration mode to `tests-gen-activity` so accounts created by
the live-chain activity generator can be migrated in parallel after the EVM
cutover.

The design follows option 1 from the discussion: `tests-gen-activity` owns the
live-chain user experience and its registry, while reusable migration primitives
are extracted from `devnet/tests/evmigration` into shared Go code. The existing
`tests_evmigration` binary keeps its devnet scenario modes and reuses the same
shared migration package.

## 2. Background

`tests_evmigration` is scenario-oriented. It assumes the devnet docker layout,
predefined accounts, validator-local execution, and a migration-specific
accounts file.

`tests-gen-activity` is live-chain-oriented. It already has chain config,
registry management, a wizard-style UI, funding/activity generation, and can be
run against devnet, testnet, or mainnet-like environments. Its registry records
the generated keys, mnemonics, key style, multisig metadata, vesting metadata,
and activity log.

The two tools already share identity/activity structs and some helpers. Migration
logic should be shared too, but the tool-level orchestration should stay
separate because the operating assumptions differ.

## 3. Non-Goals

- Do not fold `tests_evmigration` into `tests-gen-activity`.
- Do not make normal account/activity generation implicitly migrate accounts.
- Do not expose devnet validator migration through the gen-activity wizard.
- Do not require a funder key for migration mode; ordinary account migration
  signs with the legacy account and uses evmigration fee handling.
- Do not support unsafe concurrent writes to the gen-activity registry.

## 4. User Interface

Add a first-class mode flag:

```bash
tests-gen-activity -mode=migrate \
  -chain devnet \
  -accounts devnet/tests/gen-activity/accounts.json \
  -parallelism 10 \
  -dry-run=false
```

The existing default wizard should include `migrate` in the run-mode selector.
When `migrate` is selected, hide fields that are not relevant to migration:

- funding key
- number of accounts
- account prefix
- max account amount
- multisig creation counts
- vesting creation settings
- add-accounts / activity-existing
- action settings
- funding batch size

The migrate-mode wizard should keep only:

- chain/config selection
- RPC / gRPC / chain ID / home / keyring backend
- accounts registry path
- parallelism
- dry-run

`parallelism` controls concurrent migration workers. `dry-run=true` prints the
planned migration set, already-migrated skips, and estimated tx types without
creating keys, submitting txs, or mutating the registry.

## 5. Shared Migration Package

Create a reusable migration package under `devnet/tests`, for example
`devnet/tests/migration`. It should depend on `devnet/tests/common` for
`ChainCLI`, key style, account identity, activity structs, and JSON helpers.

Shared package responsibilities:

- query `evmigration params`
- query `migration-record`
- query `migration-estimate`
- derive/import EVM destination keys from legacy mnemonics
- create destination multisig keys and run the proof flow
- submit single-sig claim migrations
- submit multisig proof migrations
- wait for tx inclusion
- verify migration record and selected post-migration invariants
- classify outcomes as migrated, already migrated, skipped, or failed

`tests_evmigration` should call this package from its existing modes instead of
owning all migration helpers locally. Devnet-specific behavior remains in
`tests_evmigration`: prepare mode, validator-local discovery, validator
migration, scenario reports, and devnet-only assertions.

## 6. gen-activity Registry Changes

Extend `AccountRecord` with migration metadata:

```go
type MigrationInfo struct {
    NewName     string `json:"new_name,omitempty"`
    NewAddress  string `json:"new_address,omitempty"`
    TxHash      string `json:"tx_hash,omitempty"`
    Height      int64  `json:"height,omitempty"`
    MigratedAt  string `json:"migrated_at,omitempty"`
    Status      string `json:"status,omitempty"` // migrated, already_migrated, skipped, failed
    Error       string `json:"error,omitempty"`
}
```

The existing top-level record can either embed these fields directly or place
them under `migration`. Prefer `migration` to avoid mixing generation metadata
with migration state:

```go
Migration *MigrationInfo `json:"migration,omitempty"`
```

A record is eligible when:

- `KeyStyle == "legacy"` or the address resolves to a legacy account on-chain
- a mnemonic or multisig signer metadata exists
- there is no successful migration metadata, or the chain does not yet show the
  matching migration record

The mode must be rerunnable. If an account is already migrated on-chain, update
the registry with `Status: "already_migrated"` and the observed new address.

## 7. Account Types

Migration mode should target all account types created by `tests-gen-activity`:

- regular single-sig accounts
- continuous/delayed vesting accounts
- permanent-locked accounts
- generated multisig accounts

Single-sig, vesting, and permanent-locked accounts use the same mnemonic-based
destination-key flow. The post-migration verifier should preserve auth account
type expectations where the migration module exposes them.

Multisig accounts use the existing four-step proof flow: generate payload, sign
legacy and new sides with enough signers, combine, submit proof. The shared
package should reuse the multisig extraction already planned for gen-activity
and evmigration.

## 8. Parallel Execution

Use a worker pool:

1. Coordinator loads the registry and builds an immutable work queue.
2. Workers execute migrations and return result structs.
3. A single coordinator goroutine applies results to the registry and writes the
   file atomically.

Workers must not write `accounts.json` directly.

The migration module has `max_migrations_per_block`. The coordinator should
query it and throttle submissions so `parallelism` does not exceed the chain's
per-block migration capacity. A simple initial design:

- allow up to `min(parallelism, max_migrations_per_block)` in-flight submissions
- wait for tx inclusion before releasing a slot
- keep the registry save cadence result-driven, not batch-driven

This keeps parallel migration useful without turning the mempool into a source
of noisy sequence or per-block-cap failures.

## 9. Error Handling

Each account migration result should be independent:

- one failed account does not stop the run
- the final process exits nonzero if any account failed
- successful and already-migrated accounts remain recorded
- failed records include a short error string and can be retried on the next run

Fatal preflight errors stop before workers start:

- chain ID missing
- registry cannot be parsed
- migration disabled
- migration window closed
- current runtime is pre-EVM

## 10. Validation

Post-migration validation should check:

- migration record exists and maps old address to expected new address
- new key address matches the derived/imported destination
- old account is rejected by post-migration estimate as already migrated
- balance transfer is consistent with migration module behavior
- recorded delegations, authz, feegrant, claim, and action references are
  re-keyed where those checks are available from shared activity records

The shared package should expose validation helpers granularly so gen-activity
can use live-chain-safe checks and tests_evmigration can keep deeper devnet
scenario checks.

## 11. Tests

Add unit tests for:

- config validation: `mode=migrate` does not require funding/generation fields
- wizard visibility: migrate mode hides creation/activity prompts
- dry-run migration planning does not mutate keyring or registry
- registry eligibility selection
- result application and atomic save path
- worker pool uses a single writer
- already-migrated chain state updates the registry and skips tx submission
- throttling respects `max_migrations_per_block`
- shared migration package derives deterministic EVM destinations
- multisig migration work item construction

Keep existing `tests_evmigration` tests green after extracting shared code.

## 12. Rollout

Implementation should land in small steps:

1. Add `mode` to gen-activity config and wizard, with `fresh` as the default
   existing behavior.
2. Extract read-only migration queries and destination-key derivation.
3. Add migrate dry-run planning.
4. Add single-sig migration execution and registry updates.
5. Add multisig migration support.
6. Refactor `tests_evmigration` to use the shared package.
7. Add deeper post-migration validation shared by both tools.

This sequence gives a usable migrate mode early while keeping the risky
multisig/proof and evmigration refactor steps isolated.
