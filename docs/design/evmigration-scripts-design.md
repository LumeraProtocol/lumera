# EVM Migration Helper Scripts — Design

**Status**: Draft
**Owner**: evmigration team
**Scope**: User-facing bash helpers for single-signature account and validator migration to the post-EVM-upgrade Lumera chain.

---

## 1. Purpose

Provide two shell scripts in `/scripts` that wrap the `lumerad tx evmigration claim-legacy-account` and `lumerad tx evmigration migrate-validator` commands with safety rails, pre-flight checks, and post-migration verification. The scripts target **single-signature accounts only**; multisig accounts are detected and explicitly rejected with a pointer to the offline 4-step flow documented in [evmigration-multisig-design.md](evmigration-multisig-design.md).

Audience: chain operators, validator operators, and power users running migration from a terminal. Keplr/Portal users are covered by the web wizard and are out of scope.

## 2. Non-goals

- Multisig migration — rejected with guidance, not supported.
- Supernode daemon automatic migration — covered by the supernode daemon itself; this design only covers the `lumerad` CLI path.
- Systemd / service-manager lifecycle (no `systemctl` calls). The scripts print a checklist instead.
- Mnemonic generation, recovery, or any key *creation* beyond importing an existing mnemonic on the optional path.
- Cross-node state backfill, devnet bootstrapping, or anything unrelated to a single account's migration.

## 3. File layout

```text
scripts/
├── evmigration-common.sh      # Sourced library, not directly runnable
├── migrate-account.sh         # Wraps `lumerad tx evmigration claim-legacy-account`
└── migrate-validator.sh       # Wraps `lumerad tx evmigration migrate-validator`
```

All three files use `#!/usr/bin/env bash` and `set -euo pipefail`, matching the convention from [scripts/cli-help-smoke.sh](../../scripts/cli-help-smoke.sh). `jq` is a hard dependency — absence aborts early with a clear error.

`evmigration-common.sh` is sourced, not executed. Both entry-point scripts `source` it relative to their own location:

```bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh
source "${SCRIPT_DIR}/evmigration-common.sh"
```

## 4. Shared library (`evmigration-common.sh`)

### 4.1 Exported functions

| Function | Purpose |
|---|---|
| `parse_common_flags "$@"` | Populates globals: `NODE`, `CHAIN_ID`, `KEYRING_BACKEND`, `KEYRING_DIR`, `HOME_DIR`, `MNEMONIC_FILE`, `YES`, `DRY_RUN`, `BIN`, `LEGACY_KEY`, `NEW_KEY` |
| `require_binary` | Verifies `$BIN` is runnable and the build supports `evmigration` subcommands |
| `require_jq` | Aborts if `jq` is not on `$PATH` |
| `lumerad_q <args...>` | Thin wrapper that runs `"$BIN" query "$@" --node "$NODE" --output json` |
| `lumerad_tx <args...>` | Wrapper that runs `"$BIN" tx "$@" --node "$NODE" --chain-id "$CHAIN_ID" --keyring-backend "$KEYRING_BACKEND" --output json`, injecting `--keyring-dir` and `--home` when set. Auto-sizes gas: broadcasts with `--gas auto --gas-adjustment 1.5`, falling back to a record-count formula `6,000,000 + 1,500,000 × records` (clamped to block `max_gas`) when the simulate fails. Migration fees are waived, so the gas limit is free. See 4.7 |
| `lumerad_keys <args...>` | Thin wrapper for `lumerad keys` with the same keyring flags |
| `resolve_address <key-name>` | Returns bech32 via `lumerad keys show <k> -a` |
| `assert_single_sig <estimate-json>` | Reads `is_multisig` from a captured `migration-estimate` response (see 4.4); errors with exit code 3 if true |
| `assert_not_migrated <bech32>` | Queries `migration-record <addr>`; errors with exit code 5 if one exists |
| `assert_new_address_unused <bech32>` | Queries `migration-record <new-addr>` and `migration-record-by-new-address <new-addr>`; errors with exit code 5 if either returns a record |
| `preflight_estimate <bech32>` | Queries `migration-estimate`, parses JSON, prints human summary to stderr, and **emits the raw JSON on stdout** so callers can classify multisig, validator, and cap cases before applying the generic `would_succeed` failure |
| `assert_estimate_succeeds <estimate-json>` | Reads `would_succeed`; exits 4 with `rejection_reason` when false. Callers run this only after more specific checks that intentionally use exit 3 or 6. |
| `snapshot_bank_balances <bech32>` | Captures `bank balances <addr>` as structured JSON (used as a pre-broadcast snapshot; see 4.6) |
| `import_from_mnemonic <file> <legacy-name> <new-name>` | Reads mnemonic from file (after permission check), runs two `lumerad keys add --recover` invocations with correct coin-type/algo into the user's specified keyring, registers a `trap` that deletes the two keys on exit |
| `wait_for_tx <hash>` | Polls `lumerad query tx <hash>` up to 30s; returns non-zero on timeout, missing tx, or non-zero execution code. This is intentionally idempotent because the underlying evmigration CLI already calls SDK `wait-tx` after sync broadcast. |
| `verify_migration <legacy-bech32> <new-bech32> <legacy-balance-snapshot-json>` | After broadcast: migration record must exist with matching `new_address`, legacy bank balance must be zero, new bank balance must be ≥ snapshot per-denom (see 4.6). Exits 7 on failure |
| `confirm <prompt>` | Interactive y/N that respects `--yes` (returns 0 without prompting); returns 10 on user abort |
| `log_info` / `log_warn` / `log_error` | Prefixed output to stderr; color if `stderr` is a TTY; no color otherwise |

### 4.2 Flag schema

Parsed by `parse_common_flags`. Positional arguments (two key names) are stored in `LEGACY_KEY` and `NEW_KEY`. Script-specific flags (e.g. `--i-have-stopped-the-node` in `migrate-validator.sh`) are stripped out by the entry-point script **before** calling `parse_common_flags`, so they don't trigger the unknown-flag abort.

| Flag | Default | Notes |
|---|---|---|
| `--node <url>` | `$LUMERA_NODE` or `tcp://localhost:26657` | RPC endpoint |
| `--chain-id <id>` | `$LUMERA_CHAIN_ID` | Required; abort if unset |
| `--keyring-backend <b>` | `test` | Passed through to `lumerad` |
| `--keyring-dir <dir>` | unset | Passed through when set |
| `--home <dir>` | unset (lumerad default) | Passed through when set |
| `--mnemonic-file <path>` | unset | Triggers mnemonic import flow |
| `--yes` / `-y` | off | Skip standard confirmation prompts |
| `--dry-run` | off | Exit after pre-flight; never broadcast |
| `--binary <path>` | `lumerad` on `$PATH` | Override binary location |

Unknown flags abort with exit code 1 and a usage message.

### 4.3 Mnemonic handling

Triggered only by `--mnemonic-file`. The file must be mode `0600` or stricter; a group/world-readable file aborts with exit code 1.

Flow:

1. Check both `$LEGACY_KEY` and `$NEW_KEY` do **not** already exist in the user's keyring. Abort with exit 1 if either does (no silent overwrite).
2. Read the mnemonic from the file into a shell variable.
3. Run `lumerad keys add "$LEGACY_KEY" --recover --coin-type 118 --algo secp256k1 <keyring-flags>` piping the mnemonic via `printf '%s\n'`.
4. Run `lumerad keys add "$NEW_KEY" --recover --coin-type 60 --algo eth_secp256k1 <keyring-flags>` piping the same mnemonic.
5. `unset` the mnemonic variable.
6. Register a cleanup trap that runs `lumerad keys delete "$LEGACY_KEY" --yes` and `lumerad keys delete "$NEW_KEY" --yes` on script exit, preserving the original exit code:

   ```bash
   trap 'rc=$?; cleanup_mnemonic_keys; exit "$rc"' EXIT
   ```

The script never writes the mnemonic to disk, never logs it, and never passes it on an argv. `stderr` from `lumerad keys add` is not muted so import errors surface verbatim.

### 4.4 Multisig detection

`assert_single_sig` reads the `is_multisig` field from the `migration-estimate` response (see 4.5 — both scripts capture that JSON once and pass it to the assertion). If `is_multisig == true`, abort with exit code 3 and print guidance pointing at [evmigration-multisig-design.md](evmigration-multisig-design.md) and the offline 4-step CLI flow.

Rationale: the `migration-estimate` endpoint already performs the multisig classification server-side ([x/evmigration/keeper/query.go](../../x/evmigration/keeper/query.go) feeds `is_multisig`, `threshold`, `num_signers`). Using it avoids reimplementing pubkey-type parsing in bash and stays correct across any future auth/account JSON shape changes.

If the legacy account's pubkey is *unset* on-chain (cold wallet / nil pubkey), `is_multisig` is `false` and migration is **allowed** to proceed; the `lumerad tx evmigration claim-legacy-account` command handles pubkey seeding from the local keyring.

### 4.5 Pre-flight estimate

`preflight_estimate` runs `lumerad query evmigration migration-estimate <addr>`, writes the raw JSON response to stdout, and extracts fields for a human summary written to stderr. It does **not** exit on `would_succeed=false` by itself; callers first classify conditions with more specific exit codes:

- `would_succeed` (bool)
- `rejection_reason` (string; only meaningful when `would_succeed=false`)
- `balance_summary`, account-level delegation/unbonding/redelegation counts, `authz_grant_count`, `feegrant_count`, `action_count`, `is_validator`, validator-level `val_*_count` fields, `has_supernode`, and multisig metadata

Example summary block:

```text
Migration preview for lumera1...:
  Balance:        1234567890ulume
  Delegations:    3
  Unbonding:      1 entry
  Redelegations:  0
  Authz grants:   2
  Feegrants:      0
  Actions:        4
  Validator:      no
  Supernode:      no
  Multisig:       no
  Would succeed:  yes
```

For validator addresses, include `val_delegation_count`, `val_unbonding_count`, and `val_redelegation_count` in the summary because those are the fields used for the validator cap check. After the caller has handled multisig rejection, wrong-script checks, and the validator cap check, `assert_estimate_succeeds "$estimate_json"` prints `rejection_reason` to stderr and exits 4 if `would_succeed=false`.

### 4.6 Post-migration verification

Before broadcasting, both scripts call `snapshot_bank_balances "$legacy_addr"` to capture the legacy address's per-denom balances as JSON (via `lumerad_q bank balances <addr>`). The `migration-estimate` endpoint only exposes a `balance_summary` string (not structured coins), so the structured snapshot has to come from the `bank` module directly.

After `wait_for_tx` returns success, `verify_migration` runs three checks:

1. `lumerad_q evmigration migration-record <legacy>` must return a record whose `new_address` equals `$new_bech32`.
2. `lumerad_q bank balances <legacy>` must show all balances at zero.
3. `lumerad_q bank balances <new>` compared against the **pre-broadcast snapshot**: for every `{denom, amount}` in the snapshot, the new address's balance of that denom must be ≥ `amount`. The new balance can be strictly greater because staking rewards and validator commission are withdrawn during migration and flow into the new bank balance. Accounts with an empty snapshot (fully-staked, no liquid balance) pass trivially.

Any failure exits 7 with a loud message instructing the user to query the tx hash and investigate manually. The tx was already on-chain at this point, so rollback is not possible.

### 4.7 Gas sizing

`migrate-account.sh` and `migrate-validator.sh` broadcast through `lumerad_tx`, which no longer uses the CLI default (200000) gas. It appends `--gas auto --gas-adjustment 1.5` so the chain simulates the exact gas for the migration tx; because the migration ante handler waives fees, the resulting gas limit is free regardless of size.

If the simulate fails — for example an RPC timeout simulating a validator with a very large delegation set — `lumerad_tx` falls back to a record-count formula, `MIGRATION_GAS_BASE + MIGRATION_GAS_PER_RECORD × records` (defaults `6,000,000 + 1,500,000 × records`). The record count comes from the `MIGRATION_RECORD_COUNT` global, which the entry-point scripts set from the `migration-estimate` counts before broadcasting. The fallback limit is clamped to the chain's block `max_gas`; if it would exceed that, the script aborts rather than broadcast an over-limit tx. Note: `--gas auto`'s simulate runs the full migration handler, so for validators with many records it can take tens of seconds to ~2 minutes — this can exceed CometBFT's default `timeout_broadcast_tx_commit = 10s` and return an `EOF` error. Operators should raise that timeout (e.g. to `600s`) on the node they broadcast through, or use the fixed-gas fallback path (which skips the simulate entirely).

The constants are env-overridable: `MIGRATION_GAS_BASE`, `MIGRATION_GAS_PER_RECORD`, and `MIGRATION_GAS_ADJUSTMENT`.

`migrate-multisig.sh combine` does **not** go through `lumerad_tx`; it sets gas separately by running `combine-proof --gas auto`, which simulates gas while building the unsigned tx. As a result the combine step requires connectivity to a reachable node (default `tcp://localhost:26657`, overridable with `--node`).

## 5. `migrate-account.sh`

**Usage:**

```text
./scripts/migrate-account.sh <legacy-key-name> <new-key-name> [flags]
```

**Flow:**

1. `parse_common_flags "$@"`, `require_binary`, `require_jq`.
2. If `$MNEMONIC_FILE` set: `import_from_mnemonic` (installs cleanup trap).
3. `legacy_addr=$(resolve_address "$LEGACY_KEY")`, `new_addr=$(resolve_address "$NEW_KEY")`.
4. `assert_not_migrated "$legacy_addr"` and `assert_new_address_unused "$new_addr"`.
5. `estimate_json=$(preflight_estimate "$legacy_addr")` — captures full JSON from stdout for later reuse; prints the human summary to stderr.
6. `assert_single_sig "$estimate_json"` (reads `is_multisig`).
7. **Validator check**: if `estimate_json`'s `is_validator` field is true (or, equivalently, `lumerad_q staking validator "$(lumera_to_valoper "$legacy_addr")"` returns a record), abort with exit 6:
   *"This account is a validator. Use scripts/migrate-validator.sh instead."*
   The valoper conversion shells out to `lumerad debug addr` (see Appendix A).
8. `assert_estimate_succeeds "$estimate_json"`; exits 4 for generic estimate failures such as disabled migration or closed migration window.
9. If `estimate_json.has_supernode == true`, log a warning that the supernode registration will move with the account.
10. `legacy_balance_snapshot=$(snapshot_bank_balances "$legacy_addr")`.
11. `confirm "Proceed with migration from $legacy_addr to $new_addr?"`.
12. If `$DRY_RUN`: exit 0 here.
13. Broadcast:

    ```bash
    tx_json=$(lumerad_tx evmigration claim-legacy-account "$LEGACY_KEY" "$NEW_KEY" --yes)
    tx_hash=$(jq -r '.txhash // empty' <<<"$tx_json")
    ```

14. Require `tx_hash` to be non-empty, then run `wait_for_tx "$tx_hash"`. The command's custom CLI path already waits for sync broadcasts internally, but the script still polls/query-checks the hash so post-migration verification starts from an execution-confirmed tx response.
15. `verify_migration "$legacy_addr" "$new_addr" "$legacy_balance_snapshot"`.
16. Print a success summary with the tx hash and the new bech32 / hex addresses.

## 6. `migrate-validator.sh`

**Usage:**

```text
./scripts/migrate-validator.sh <legacy-validator-key> <new-evm-key> [flags]
```

**Flow:**

1. Entry-point script pre-parses and strips `--i-have-stopped-the-node` (setting a local `NODE_STOPPED=1`) before calling `parse_common_flags` with the remaining args.
2. `parse_common_flags "$@"`, `require_binary`, `require_jq`.
3. If `$MNEMONIC_FILE` set: `import_from_mnemonic` (installs cleanup trap).
4. `legacy_addr=$(resolve_address "$LEGACY_KEY")`, `new_addr=$(resolve_address "$NEW_KEY")`.
5. `assert_not_migrated "$legacy_addr"` and `assert_new_address_unused "$new_addr"`.
6. `estimate_json=$(preflight_estimate "$legacy_addr")`.
7. `assert_single_sig "$estimate_json"`.
8. **Reverse validator check**: if `estimate_json.is_validator == false`, abort with exit 6:
   *"This account is not a validator. Use scripts/migrate-account.sh instead."*
9. **Delegation cap check** (uses `val_*_count` fields that count records *referencing the validator*, matching what the keeper enforces in [msg_server_migrate_validator.go](../../x/evmigration/keeper/msg_server_migrate_validator.go) via `GetValidatorDelegations` / `GetUnbondingDelegationsFromValidator` / redelegations by src-or-dst):

   ```bash
   cap=$(lumerad_q evmigration params | jq -r '.params.max_validator_delegations')
   total=$(jq -r '.val_delegation_count + .val_unbonding_count + .val_redelegation_count' <<<"$estimate_json")
   ```

   Abort with exit 6 if `total > cap`. Log a warning if `total > cap * 9 / 10`.

10. `assert_estimate_succeeds "$estimate_json"`; exits 4 for non-cap estimate failures such as unbonding/unbonded validator status, disabled migration, or closed migration window.
11. `legacy_balance_snapshot=$(snapshot_bank_balances "$legacy_addr")`.
12. **Validator downtime banner**:

    ```text
    ================================================================
    WARNING — VALIDATOR MIGRATION
    Your validator will miss blocks and may be jailed during
    migration. The node MUST be stopped before broadcasting this tx.
    ================================================================
    ```

    Require a separate confirmation, satisfied by EITHER the pre-stripped `--i-have-stopped-the-node` flag (step 1, `NODE_STOPPED=1`) OR an interactive typed `yes` / `no` response (must be the full word "yes"). `--yes` does NOT satisfy this check — this is the one place the script is deliberately more interactive than the account path.

13. `confirm "Proceed with validator migration from $legacy_addr to $new_addr?"`.
14. If `$DRY_RUN`: exit 0.
15. Broadcast `lumerad tx evmigration migrate-validator "$LEGACY_KEY" "$NEW_KEY" --yes`, capture `.txhash`, and require it to be non-empty.
16. `wait_for_tx "$tx_hash"`, then `verify_migration "$legacy_addr" "$new_addr" "$legacy_balance_snapshot"`.
17. Print post-migration checklist:
    - Import `$NEW_KEY` into the production keyring at the correct `--keyring-backend`.
    - Restart the validator binary.
    - Verify the validator's new operator address via `lumerad_q staking validator <new-valoper>`.
    - Monitor missed-block counters for the next few blocks.

## 7. Exit codes

| Code | Meaning |
|---|---|
| `0` | Success, or dry-run completed cleanly |
| `1` | Usage error / bad flags / bad input file permissions / key name collision |
| `2` | Environment error: binary missing, jq missing, node unreachable, unsupported binary version |
| `3` | Multisig rejected; user directed to offline flow |
| `4` | Pre-flight estimate returned `would_succeed=false` |
| `5` | Legacy account is already migrated, or the requested new address is already unavailable because it was migrated or used as a migration destination |
| `6` | Wrong-script or delegation-cap error (validator check failures) |
| `7` | Broadcast completed but post-migration verification failed |
| `10` | User aborted at a confirmation prompt |

## 8. Safety rails

- `set -euo pipefail` in all three files.
- `IFS=$'\n\t'` at top of each entry-point script.
- No `rm -rf` anywhere. The only destructive call is `lumerad keys delete <name> --yes`, scoped to the two names the script itself created.
- Mnemonic files are permission-checked (`0600` or stricter) and never written elsewhere.
- All `lumerad` stderr surfaces verbatim; no swallowed errors.
- `trap` cleanup preserves `$?` so the caller sees the real exit code.
- The validator downtime confirmation is independent from `--yes` — explicit acknowledgement is mandatory.

## 9. Testing strategy

- **Shellcheck**: all three files pass `shellcheck -x` as part of CI (new lint target or addition to existing Makefile `lint` rule).
- **Devnet smoke test**: a new script-level test (optional build tag) that brings up the existing `make devnet-new` topology, runs `migrate-account.sh` against a pre-funded legacy account and asserts the expected exit code and on-chain state.
- **Unit-style bash tests with `bats`** (nice-to-have, not required for first cut): tests that stub `lumerad` with a small shim and assert flag parsing, multisig rejection, exit codes, and mnemonic cleanup.
- **Manual validation matrix**: single-sig account, single-sig validator, already-migrated account (expect exit 5), multisig account (expect exit 3), validator-using-account-script (exit 6), account-using-validator-script (exit 6), over-cap validator (exit 6), `--dry-run` on all of the above.

## 10. Documentation updates

- Add a "Method 3: Shell helper scripts" section to [docs/evm-integration/user-guides/migration.md](../evm-integration/user-guides/migration.md).
- Cross-link from [docs/evm-integration/user-guides/validator-migration.md](../evm-integration/user-guides/validator-migration.md) and [docs/evm-integration/user-guides/supernode-migration.md](../evm-integration/user-guides/supernode-migration.md).
- No changes to the multisig design doc (scripts deliberately punt to it).

## 11. Appendix A — `lumera_to_valoper` helper

Reuses the chain's own `lumerad debug addr` subcommand to avoid implementing bech32 re-encoding in bash. Verified output format (from a current `lumerad` binary):

```text
Address: [...]
Address (hex): F63DD6CD01A8B06A381F09354AAC8F945387BD16
Bech32 Acc: lumera17c7adngp4zcx5wqlpy654ty0j3fc00gk23wz59
Bech32 Val: lumeravaloper17c7adngp4zcx5wqlpy654ty0j3fc00gk9ta7jx
Bech32 Con: lumeravalcons17c7adngp4zcx5wqlpy654ty0j3fc00gk3cwz78
```

Implementation:

```bash
lumera_to_valoper() {
  local addr=$1
  local valoper
  valoper=$("$BIN" debug addr "$addr" | awk -F': ' '/^Bech32 Val: /{print $2; exit}')
  if [[ -z "$valoper" ]]; then
    log_error "cannot derive valoper for $addr"
    return 2
  fi
  printf '%s\n' "$valoper"
}
```
