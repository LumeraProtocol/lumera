# gen-activity: migrate mode for generated live-chain accounts

**Date:** 2026-06-16
**Status:** Approved (design); revised 2026-06-16 after codebase review
**Tools:** `devnet/tests/gen-activity`, `devnet/tests/evmigration`, `devnet/tests/common`

## 1. Goal

Add an explicit migration mode to `tests-gen-activity` so accounts created by
the live-chain activity generator can be migrated in parallel after the EVM
cutover.

The design follows option 1 from the discussion: `tests-gen-activity` owns the
live-chain user experience and its registry, while reusable migration primitives
live in shared Go code. The existing `tests_evmigration` binary keeps its devnet
scenario modes and reuses the same shared migration code.

**Revision note (shared-code location):** rather than create a brand-new
`devnet/tests/migration` package, the shared migration primitives are added to
the existing `devnet/tests/common` package, which both tools already import
(`ChainCLI`, `Multisig`, key style, `AccountIdentity`, activity structs, JSON
helpers). This avoids a new package boundary and a large up-front import churn.
`tests_evmigration` is then refactored to call these `common` helpers instead of
its local copies.

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

`-chain`, `-accounts`, `-parallelism`, and `-dry-run` already exist. The new
piece is `-mode`. Today the "mode" concept is implicit: two mutually-exclusive
booleans (`-add-accounts`, `-activity-existing`) with the unset state meaning
"fresh". This design promotes mode to an explicit `-mode` flag with values
`fresh | add-accounts | activity-existing | migrate`, backed by a `Mode` config
field. For backward compatibility the existing booleans continue to set the
equivalent mode and must not be combined with a conflicting `-mode` value;
`Validate()` rejects contradictions. `migrate` is mutually exclusive with all
generation/activity modes.

The existing default wizard already has a run-mode selector
(`wizard.go` `settingMode`, options `fresh`/`add-accounts`/`activity-existing`);
add `migrate` to it.
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

## 5. Shared Migration Code (in `devnet/tests/common`)

Add reusable migration primitives to the existing `devnet/tests/common`
package (new files, e.g. `migration.go`, `migration_keys.go`,
`migration_multisig.go`, `migration_verify.go`). They build on the package's
existing `ChainCLI`, `Multisig`, `KeyStyle`, `AccountIdentity`, and JSON
helpers.

Shared responsibilities:

- query `evmigration params` (`enable_migration`, `migration_end_time`,
  `max_migrations_per_block`, `max_validator_delegations`, `max_multisig_sub_keys`)
- query `migration-record` (by legacy address and by new address)
- query `migration-estimate`
- derive/import EVM destination keys from legacy mnemonics (coin-type 60,
  `eth_secp256k1`) — deterministic from the same mnemonic
- create destination multisig keys and run the four-step proof flow
- submit single-sig claim migrations (`tx evmigration claim-legacy-account`)
- submit multisig proof migrations (generate-proof-payload → sign-proof ×K →
  combine-proof → submit-proof)
- wait for tx inclusion
- verify migration record and selected post-migration invariants
- classify outcomes as migrated, already migrated, skipped, or failed

`tests_evmigration` is refactored to call these `common` helpers instead of its
local copies (see §12 step 6). Devnet-specific behavior remains in
`tests_evmigration`: prepare mode, validator-local discovery, validator
migration, scenario reports, and devnet-only assertions.

The migration helpers must take their chain seams through interfaces/`ChainCLI`
so gen-activity can unit-test planning and result application with fakes, the
same way it already fakes `activityChain`, `fundingChain`, and `multisigExerciser`.

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

The fields go under a nested `migration` object on `AccountRecord` to avoid
mixing generation metadata with migration state:

```go
Migration *MigrationInfo `json:"migration,omitempty"`
```

Because the field is optional (`omitempty`) and additive, the registry
`schemaVersion` stays at **2**; old registries load unchanged. Some live devnet
registries carry ad-hoc top-level `migrated` / `new_address` keys that were added
by hand during manual migration testing; these are not part of the struct and
are ignored on load. Migrate mode supersedes them by writing the structured
`migration` object, and may optionally backfill from them when present.

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
destination-key flow. The keeper (`x/evmigration/keeper/migrate_auth.go`)
detects the legacy auth type (continuous/delayed/periodic vesting,
permanent-locked), captures the original schedule, removes the legacy account,
transfers the balance, and recreates the vesting account at the new address.
The destination must be a fresh address or a plain `BaseAccount`
(`ErrInvalidMigrationDestination` otherwise); the mnemonic-derived coin-type-60
key is fresh, so this holds. The post-migration verifier should preserve auth
account type expectations where the migration module exposes them.

Multisig accounts use the existing four-step proof flow: generate payload, sign
legacy and new sides with enough signers, combine, submit proof. The shared
package should reuse the multisig extraction already planned for gen-activity
and evmigration.

**Member-key portability.** The proof flow signs with the legacy multisig member
sub-keys, which (unlike a single-sig account's key) cannot be derived on the fly.
Generation therefore persists each member's mnemonic in
`MultisigInfo.Members[]` (alongside the existing `member_names`), so migrate mode
can re-import any missing member key into a fresh keyring before the ceremony —
giving multisig the same portability single-sig already has. Before creating any
destination keys, migrate mode pre-flights member-key presence: it imports
missing keys from their stored mnemonics, and if a key is missing with no
recoverable mnemonic it fails that account fast with an actionable message
(rather than the keeper's opaque "key not found"/signature errors) and creates no
orphan destination keys. Registries generated before this change carry only
`member_names`; those accounts must be migrated from the keyring that generated
them.

**Chain-id in the proof.** `generate-proof-payload` MUST be invoked with
`--chain-id`: the chain id is part of the signed proof payload, and omitting it
makes the CLI fall back to the bech32 prefix, so every signature fails on-chain
verification.

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

### 8.1 Logging & observability (concurrent troubleshooting)

Concurrency makes interleaved logs hard to read, so every migration log line must
be self-identifying and correlatable:

- Each work item gets a stable **correlation id** (the account name, plus a
  monotonic work index) included in every line for that item, e.g.
  `[migrate gen-0007 #12]`.
- Log the **lifecycle** of each item at a consistent set of points: queued,
  preflight/estimate result, destination key derived (with new address),
  submit (with tx hash), inclusion (with height + wait duration), verify result,
  and final outcome (migrated / already_migrated / skipped / failed + reason).
- Worker pool events are logged: worker start/stop, in-flight slot
  acquisition/release, and throttle waits caused by `max_migrations_per_block`
  (so a stall is visibly attributed to throttling, not a hang).
- The coordinator (single writer) logs each registry apply with the item id and
  resulting status, plus periodic progress (`done N/total, in-flight M`).
- Each line carries a wall-clock timestamp and is written through one
  synchronized logger so concurrent lines never interleave mid-line.
- A run summary at the end tallies counts per outcome and lists failed items with
  their short error strings, so a run can be triaged without grepping.
- `dry-run` uses the same correlation ids and lifecycle phrasing (minus the tx
  lines) so dry-run output maps 1:1 onto a real run.

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
2. Add read-only migration queries and destination-key derivation to
   `devnet/tests/common`.
3. Add migrate dry-run planning and the `MigrationInfo` registry field.
4. Add single-sig migration execution (incl. vesting/permanent-locked) and
   registry updates.
5. Add multisig migration support.
6. Refactor `tests_evmigration` to use the shared `common` migration code.
7. Add deeper post-migration validation shared by both tools.

This sequence gives a usable migrate mode early while keeping the risky
multisig/proof and evmigration refactor steps isolated.
