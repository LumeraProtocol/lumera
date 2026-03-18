# Devnet EVM Migration Tests

## Overview

The `tests_evmigration` tool is a standalone binary for end-to-end testing of the `x/evmigration` module on the Lumera devnet. It validates the chain's ability to atomically migrate account state when upgrading from legacy Cosmos key derivation (coin-type 118, `secp256k1`) to EVM-compatible key derivation (coin-type 60, `eth_secp256k1`).

When Lumera upgrades to support EVM (v1.12.0), the same mnemonic produces a **different on-chain address** under coin-type 60. The evmigration module provides `MsgClaimLegacyAccount` and `MsgMigrateValidator` transactions that atomically transfer all state from the old address to the new one. This tool creates realistic pre-migration state, then exercises and verifies those migration paths.

Source code: `devnet/tests/evmigration/`

## Modules Tested

The migration touches many modules. The test tool verifies correct re-keying across all of them:

| Module | What's Migrated |
|---|---|
| **x/auth** | Account removal + re-creation (preserves vesting params) |
| **x/bank** | Balance transfer from legacy to new address via `SendCoins` |
| **x/staking** | Delegations, unbonding entries, redelegations (with queue and `UnbondingId` indexes) |
| **x/distribution** | Reward withdrawal, delegator starting info |
| **x/authz** | Grant re-keying (both grantor and grantee roles) |
| **x/feegrant** | Fee allowance re-creation (both granter and grantee) |
| **x/supernode** | `ValidatorAddress`, `SupernodeAccount`, `Evidence`, `PrevSupernodeAccounts`, `MetricsState` |
| **x/action** | `Creator` and `SuperNodes` fields in action records |
| **x/claim** | `DestAddress` in claim records |
| **x/evmigration** | Core migration logic, dual-signature verification, rate limiting, params |

Two custom ante decorators support the migration:

- **EVMigrationFeeDecorator** (`ante/evmigration_fee_decorator.go`) — allows zero-fee migration transactions (the new address has no balance before migration completes).
- **EVMigrationValidateBasicDecorator** (`ante/evmigration_validate_basic_decorator.go`) — lets migration-only transactions skip the normal Cosmos signature check (auth is via the legacy signature in the message payload).

## Modes

The tool has six operating modes, designed to be run sequentially during a devnet upgrade cycle.

### 1. `prepare` — Create Legacy State (Pre-EVM)

Run **before** the EVM upgrade (on v1.11.0) to populate the chain with legacy accounts and on-chain activity.

Creates **N legacy accounts** (coin-type 118, marked `IsLegacy=true` with full mnemonic stored) and **N extra accounts** for background noise. Default: 5 + 5.

Activity generated per account (deterministic pattern based on account index):

| Activity | Which Accounts | Amount / Details |
|---|---|---|
| **Delegations** | Every account | 100k–500k ulume |
| **Unbonding** | Every 4th legacy account | 20k ulume |
| **Redelegations** | Every 6th legacy account | 1–3 entries of 15k ulume each |
| **Withdraw address** | Every 7th legacy account | Set to a third-party address |
| **Authz grants** | Every 3rd legacy account | Grants to 3 random peers |
| **Authz received** | Every 4th legacy (offset 1) | Receives grants from 3 random peers |
| **Feegrants** | Every 5th legacy account | 500k spend-limit to 3 peers |
| **Feegrants received** | Every 6th legacy (offset 1) | Receives feegrants from 3 peers |
| **Actions (CASCADE)** | Every 4th legacy (offset 2) | Submitted via `sdk-go` with supernode involvement |
| **Claims** | Progressive distribution | Pre-seeded Pastel keys; ~70% instant, ~30% delayed (tiers 1/2/3) |

Execution strategy:
- **Phase 1** — Own-account operations (delegations, unbonding, redelegations, withdrawal addr, authz grants out, feegrants out) are **parallelized** in 5-worker batches.
- **Phase 2** — Cross-account operations (authz receives, feegrant receives) run **sequentially** to avoid nonce conflicts.
- **Phase 3** — Extra-account random activity, parallelized.
- **Phase 4** — Claim activity using 100 pre-seeded Pastel keypairs from `claim_keys.go`.

Output: `accounts.json` file containing the complete `AccountRecord` for each account (name, mnemonic, address, activity flags and details). This file is consumed by all subsequent modes.

### 2. `estimate` — Query Migration Readiness (Post-EVM)

Run **after** the EVM upgrade (on v1.12.0). Queries the `migration-estimate` RPC endpoint for every legacy account.

Returns per account:
- `WouldSucceed` — whether migration can proceed
- `RejectionReason` — why blocked (e.g. "already migrated", "migration disabled")
- Counts of: delegations, unbondings, redelegations, authz grants, feegrants, actions, validator delegations

Classifies each account as:
- **ready_to_migrate** — `WouldSucceed=true`
- **already_migrated** — rejection says "already migrated"
- **blocked** — `WouldSucceed=false`, logs reason

Prints a summary:

```
legacy_accounts: 5
estimates_fetched: 5
ready_to_migrate: 5
already_migrated: 0
blocked: 0
estimate_query_errors: 0
```

### 3. `migrate` — Migrate Regular Accounts (Post-EVM)

Migrates all legacy accounts using `MsgClaimLegacyAccount`. Per-account flow:

1. Check for rerun: query `migration-record` — if it already exists, skip to validation.
2. Query `migration-estimate` — verify `WouldSucceed=true`.
3. Derive the new EVM-compatible address from the same mnemonic using coin-type 60.
4. Create a new keyring entry for the destination address.
5. Sign the migration payload: `sign("claim", legacy_privkey, legacy_addr, new_addr)` → base64 signature.
6. Submit `MsgClaimLegacyAccount(new_address, legacy_address, legacy_pubkey_b64, signature_b64)`, signed by the new address key (zero-fee via the EVMigrationFeeDecorator).
7. Verify on-chain `migration-record` exists with the correct new address.

Execution strategy:
- Accounts are shuffled randomly.
- Processed in random batches of 1–5 accounts.
- Progress saved to `accounts.json` after each batch.
- Migration stats queried after each batch.

The migration is **atomic** — a single transaction migrates the entire account state across all modules. If any step fails, the whole transaction rolls back and no record is stored.

### 4. `migrate-validator` — Migrate Validator Operator (Post-EVM)

Specialized mode for validator operators. Uses `MsgMigrateValidator` instead of `MsgClaimLegacyAccount`.

**Detection:** Iterates the local keyring, identifies keys matching active validators via staking queries, and filters for legacy `secp256k1` keys. Must match exactly one candidate (override with `-validator-keys=<name>`).

Steps:
1. Create a unique destination key (`eth_secp256k1`, coin-type 60).
2. Export the legacy validator private key.
3. Sign a validator migration proof: `sign("validator", legacy_addr, new_addr)` — note the different message prefix vs regular migration.
4. Submit `MsgMigrateValidator(new_address, legacy_address, pubkey, signature)`.
5. Verify `migration-record`.

Extensive post-migration validation:
- Estimate query post-migration must return "already migrated".
- New validator exists at the new valoper address.
- Delegator count matches pre/post migration.
- All actions referencing the old creator/supernode now reference the new address.
- Supernode fields verified: `ValidatorAddress`, `SupernodeAccount`, `Evidence` entries, `PrevSupernodeAccounts` history (new entry appended with current block height), `MetricsState` re-keyed.
- If the validator's supernode account was already migrated independently before validator migration, it must be preserved and reattached under the new valoper without tripping the stale supernode-account index collision.

### 5. `verify` — Verify No Leftover Legacy State (Post-Migration)

Run **after** all migrations complete. Queries every chain module (except `x/evmigration` itself) via RPC to confirm that no legacy address references remain in on-chain state.

For each migrated legacy address, the tool checks:

| Module | Check |
|---|---|
| **bank** | No remaining balance on legacy address |
| **staking** | No delegations, unbonding delegations, or redelegations |
| **distribution** | No pending rewards; withdraw address not pointing to legacy |
| **authz** | No grants as granter or grantee |
| **feegrant** | No allowances as granter or grantee |
| **action** | No actions referencing legacy as creator or supernode |
| **claim** | No unclaimed records; `dest_address` not pointing to legacy |
| **supernode** | No `supernode_account` or `evidence.reporter_address` fields referencing legacy (note: `prev_supernode_accounts` entries are excluded — legacy addresses there are legitimate historical records) |
| **evmigration** | Migration record must exist; estimate must return "already migrated" |

Results are reported as either `PASS` (all addresses clean) or `FAIL` with per-address details grouped by module. The tool exits with a non-zero status on failure, which halts the pipeline.

### 6. `cleanup` — Remove Test Keys

Loads `accounts.json` and deletes all test keys from the local keyring (`~/.lumera/keyring-test/` or the path from `-home`).

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `-mode` | (required) | `prepare`, `estimate`, `migrate`, `migrate-validator`, `verify`, or `cleanup` |
| `-bin` | `lumerad` | Path to `lumerad` binary |
| `-rpc` | `tcp://localhost:26657` | Tendermint RPC endpoint |
| `-grpc` | (derived from RPC) | gRPC endpoint (default: RPC host + port 9090) |
| `-chain-id` | `lumera-devnet-1` | Chain ID |
| `-accounts` | `accounts.json` | Path to the accounts JSON file |
| `-home` | (lumerad default) | `lumerad` home directory |
| `-funder` | (auto-detect) | Key name to fund accounts in prepare mode |
| `-gas` | `500000` | Gas limit (fixed value avoids simulation sequence races) |
| `-gas-adjustment` | `1.5` | Gas adjustment (only with `--gas=auto`) |
| `-gas-prices` | `0.025ulume` | Gas prices |
| `-evm-cutover-version` | `v1.12.0` | Version where coin-type switches to 60 |
| `-num-accounts` | `5` | Number of legacy accounts to generate |
| `-num-extra` | `5` | Number of extra (non-migration) accounts |
| `-account-tag` | (auto-detect) | Account name prefix tag (e.g. `val1` → `pre-evm-val1-000`) |
| `-validator-keys` | (auto-detect) | Validator key name for migrate-validator mode |

## Makefile Targets

All targets are defined in `Makefile.devnet` and run the tool inside devnet Docker containers via `docker compose exec`.

### Sequential targets

These run the tool on each validator container **one at a time**, in order:

| Target | Description |
|---|---|
| `make devnet-evmigration-sync-bin` | Copy the `tests_evmigration` binary into the devnet shared volume |
| `make devnet-evmigration-prepare` | Run prepare mode on all validator containers |
| `make devnet-evmigration-estimate` | Run estimate mode on all validator containers |
| `make devnet-evmigration-migrate` | Run migrate mode on all validator containers |
| `make devnet-evmigration-migrate-validator` | Run migrate-validator mode on all validator containers |
| `make devnet-evmigration-verify` | Run verify mode on all validator containers |
| `make devnet-evmigration-cleanup` | Run cleanup mode on all validator containers |

### Parallel targets (`devnet-evmigrationp-*`)

These run the tool on **all validator containers simultaneously** using background processes, with per-container output captured and printed after completion. Each container gets its own accounts file, so there are no cross-validator conflicts. If any container fails, the target fails after all containers finish.

| Target | Description |
|---|---|
| `make devnet-evmigrationp-prepare` | Run prepare mode on all validators in parallel |
| `make devnet-evmigrationp-estimate` | Run estimate mode on all validators in parallel |
| `make devnet-evmigrationp-migrate` | Run migrate mode on all validators in parallel |
| `make devnet-evmigrationp-migrate-validator` | Run migrate-validator mode on all validators in parallel |
| `make devnet-evmigrationp-verify` | Run verify mode on all validators in parallel |
| `make devnet-evmigrationp-cleanup` | Run cleanup mode on all validators in parallel |

The parallel targets use the `_run_evmigration_in_containers_parallel` macro, which spawns one `docker compose exec` per validator service as a background process, collects exit codes, and prints output prefixed by service name. This is significantly faster for modes like `prepare` and `migrate` where each validator's work is independent.

### Full upgrade pipeline (`devnet-evm-upgrade`)

The `make devnet-evm-upgrade` target runs the **complete end-to-end EVM upgrade cycle** as a single automated pipeline. It orchestrates all stages from a clean v1.11.0 devnet through to a fully migrated v1.12.0 chain, using the parallel targets for speed:

| Stage | What it does |
|---|---|
| 1. Install v1.11.0 devnet | `devnet-down` → `devnet-clean` → `devnet-build-1110` → `devnet-up-detach` |
| 2. Wait for height 40 | Waits for the chain to produce blocks (confirms v1.11.0 is healthy) |
| 3. Prepare legacy state | `devnet-evmigrationp-prepare` (parallel across all validators) |
| 4. Wait for +5 blocks | Lets prepared state settle into committed blocks |
| 5. Upgrade to v1.12.0 | `devnet-upgrade-1120` (governance proposal → vote → halt → binary swap → restart) |
| 6. Check estimates | `devnet-evmigrationp-estimate` (verify all accounts are `ready_to_migrate`) |
| 7. Migrate validators | `devnet-evmigrationp-migrate-validator` (validator operators first) |
| 8. Migrate accounts | `devnet-evmigrationp-migrate` (regular accounts second) |
| 9. Verify clean state | `devnet-evmigrationp-verify` (confirms no legacy address leftovers in any module) |

Each stage has error handling — if any stage fails, the pipeline aborts with a clear error message identifying which stage failed. Validators are migrated before regular accounts because `MsgMigrateValidator` atomically re-keys the validator record and all its delegations, which must happen before delegators attempt their own migration.

Usage:

```bash
# Run the full upgrade pipeline (takes ~10-15 minutes)
make devnet-evm-upgrade
```

### Configurable variables

| Variable | Default | Description |
|---|---|---|
| `EVMIGRATION_CHAIN_ID` | `lumera-devnet-1` | Chain ID passed to the tool |
| `EVMIGRATION_NUM_ACCOUNTS` | `5` | Number of legacy accounts per validator |
| `EVMIGRATION_NUM_EXTRA` | `5` | Number of extra accounts per validator |

Each validator gets its own accounts file (`/shared/status/<moniker>/evmigration-accounts.json`) to avoid cross-validator key/account collisions. Account name tags are auto-derived from the local validator/funder key name.

## Building the Test Binary

```bash
make devnet-tests-build
```

This builds `tests_evmigration` (along with `tests_validator` and `tests_hermes`) and places it in `devnet/bin/`.

## Full Upgrade Test Walkthrough

> **Quick path:** `make devnet-evm-upgrade` runs all steps below automatically as a single pipeline. See [Full upgrade pipeline](#full-upgrade-pipeline-devnet-evm-upgrade) above. The manual steps below are useful for debugging or running individual stages.

### Step 1: Start devnet on v1.11.0

The `devnet/bin-v1.11.0/` directory must contain the pre-EVM binaries:

| File | Description |
|---|---|
| `lumerad` | v1.11.0 chain binary |
| `libwasmvm.x86_64.so` | CosmWasm runtime library |
| `supernode-linux-amd64` | Supernode binary |
| `tests_validator` | Validator devnet tests |
| `tests_hermes` | Hermes IBC relayer tests |
| `tests_evmigration` | EVM migration test binary (built from `devnet/tests/evmigration/`) |

```bash
# Clean any existing devnet, build from v1.11.0 binaries, and start
make devnet-new-1110
```

This runs `devnet-down` → `devnet-clean` → `devnet-build-1110` → (10s sleep) → `devnet-up`. The build uses `DEVNET_BUILD_LUMERA=0` (skips compiling lumerad, uses the pre-built binary from `devnet/bin-v1.11.0/`).

### Step 2: Prepare legacy state

Once the devnet is running on v1.11.0:

```bash
make devnet-evmigration-prepare
```

This creates legacy accounts and activity on each validator node. Accounts JSON files are written to `/shared/status/<moniker>/evmigration-accounts.json` inside the containers.

### Step 3: Upgrade to v1.12.0 (EVM)

```bash
make devnet-upgrade-1120
```

This calls `devnet/scripts/upgrade.sh v1.12.0 auto-height ../bin`, which:

1. **Submits a software-upgrade governance proposal** for `v1.12.0` at `current_height + 100`.
2. **Retrieves the proposal ID** and verifies it.
3. **Votes yes with all validators** (if in voting period).
4. **Waits for the chain to reach the upgrade height** (chain halts automatically).
5. **Swaps binaries**: stops containers, copies all files from `devnet/bin/` (the current build) to the shared release directory, restarts containers.

The `devnet/bin/` directory must contain the v1.12.0 `lumerad` binary (built by `make build`).

### Step 4: Check migration estimates

```bash
make devnet-evmigration-estimate
```

Verifies all legacy accounts are in the `ready_to_migrate` state.

### Step 5: Migrate regular accounts

```bash
make devnet-evmigration-migrate
```

Migrates all legacy (non-validator) accounts in randomized batches.

### Step 6: Migrate validators

```bash
make devnet-evmigration-migrate-validator
```

Migrates the validator operator account on each node with full post-migration validation.

### Step 7: Verify clean state

```bash
make devnet-evmigration-verify
```

Queries all modules via RPC to confirm no legacy address references remain (except legitimate `prev_supernode_accounts` entries). Exits non-zero if any leftover state is found.

### Step 8: Clean up

```bash
make devnet-evmigration-cleanup
```

Removes test keys from the keyring on each validator node.

## Rerun Support

All modes are **idempotent**:

- **prepare** — reloads `accounts.json` if it exists and skips already-created accounts.
- **estimate** — can be run any number of times; purely read-only.
- **migrate** — checks `migration-record` on-chain before submitting; skips already-migrated accounts and saves progress after each batch.
- **migrate-validator** — checks migration record before submitting.
- **verify** — purely read-only; can be run any number of times.
- **cleanup** — silently skips keys that don't exist.

## Runtime Version Checks

The tool validates the running `lumerad` version:

- **prepare** mode enforces `lumerad version < v1.12.0` (coin-type 118 environment).
- **estimate / migrate / migrate-validator** modes enforce `lumerad version >= v1.12.0` (coin-type 60 environment).
