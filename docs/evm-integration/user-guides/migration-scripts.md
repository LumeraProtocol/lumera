# EVM Migration Helper Scripts — User Guide

**Applies to**: Lumera chain with the `x/evmigration` module enabled (post-EVM upgrade).
**Audience**: Terminal users — regular account holders, validator operators, and supernode operators running `lumerad` from a shell.

---

## What these scripts do

The Lumera release ships two bash wrappers that automate the end-to-end migration of a **single-signature** legacy (coin-type 118) account or validator operator to its EVM-compatible (coin-type 60) counterpart:

| Script | Wraps | Use for |
|---|---|---|
| `scripts/migrate-account.sh` | `lumerad tx evmigration claim-legacy-account` | Regular accounts, including supernode operator accounts that are not also validators |
| `scripts/migrate-validator.sh` | `lumerad tx evmigration migrate-validator` | Validator operator accounts |

Compared to calling `lumerad` directly (see [migration.md Method 3](migration.md#method-3-lumera-cli)), each script adds:

- **Multisig detection** — classifies the legacy account via `migration-estimate.is_multisig` and refuses to proceed, pointing you at the offline 4-step flow instead.
- **Pre-flight preview** — runs `migration-estimate` before broadcast and prints a summary (balance, delegations, unbondings, authz grants, fee grants, supernode, whether the account is a validator). Aborts if the keeper says the migration would not succeed.
- **Wrong-script guard** — `migrate-account.sh` refuses validator operators; `migrate-validator.sh` refuses non-validators. Both use `migration-estimate.is_validator` to decide.
- **Delegation-cap check** — `migrate-validator.sh` computes `val_delegation_count + val_unbonding_count + val_redelegation_count` and compares against the chain's `max_validator_delegations` parameter, aborting if the cap would be exceeded.
- **Downtime acknowledgement** — `migrate-validator.sh` prints a warning banner and requires an explicit confirmation (either `--i-have-stopped-the-node` or typing the full word `yes`) before broadcasting. `--yes` does not satisfy this check.
- **Atomic pre/post balance snapshot** — captures the legacy address's per-denom balances *before* broadcast, and verifies after inclusion that the new address holds at least the snapshotted amounts per denom while the legacy address is zero.
- **Already-migrated guard** — aborts if the legacy address already has a migration record, or if the new address was previously used as a migration destination.
- **Dry-run mode** — `--dry-run` runs every pre-flight check and prints the preview, then exits 0 before broadcasting.

## When not to use these scripts

- **Multisig accounts** — the scripts refuse them by design. Use the offline 4-step flow documented at [legacy-migration.md](../evmigration/legacy-migration.md#multisig-account-migration).
- **Supernode daemon auto-migration** — if you run a supernode with `evm_key_name` set in `config.yml`, the supernode daemon performs migration for you on restart. See [supernode-migration.md](supernode-migration.md). The scripts are an alternative for operators who prefer to handle migration via `lumerad` directly before starting the supernode.
- **Keplr/Portal users** — use [migration.md Method 1](migration.md#method-1-portal--keplr-recommended). The scripts are terminal-only.

---

## Prerequisites

On the machine where you run the scripts:

- `lumerad` binary built from a post-EVM-upgrade commit (required). Pre-EVM binaries don't have the `evmigration` subcommands — the scripts detect this and abort with exit 2.
- `bash` 4.4 or newer (uses `mapfile` and `${var,,}` semantics).
- `jq` on `$PATH`.
- Access to a running Lumera RPC endpoint (`--node tcp://host:26657`).
- The **same mnemonic** (recovery phrase) that controls the legacy address. Both the coin-type 118 and coin-type 60 keys must be derivable from it.

Verify the binary version:

```bash
lumerad query evmigration --help
# should list: legacy-accounts, migrated-accounts, migration-estimate,
# migration-record, migration-record-by-new-address, migration-records,
# migration-stats, params
```

## Getting the scripts

Two sources — pick whichever matches how you installed `lumerad`:

### From a release tarball

Every `lumerad_${GOOS}_${GOARCH}.tar.gz` on the [releases page](https://github.com/LumeraProtocol/lumera/releases) ships the three scripts alongside the binary:

```text
lumerad
scripts/
  evmigration-common.sh
  migrate-account.sh
  migrate-validator.sh
```

Extract and invoke with `./scripts/migrate-account.sh …` from the extraction directory. The scripts resolve their shared library via `$(dirname "${BASH_SOURCE[0]}")`, so they work as long as all three files stay in the same directory.

### From a source checkout

```bash
git clone https://github.com/LumeraProtocol/lumera.git
cd lumera
./scripts/migrate-account.sh --help
```

---

## Common flags

Both scripts accept the same flags. Positional arguments are always `<legacy-key-name> <new-key-name>`.

| Flag | Default | Description |
|---|---|---|
| `--node <url>` | `$LUMERA_NODE` or `tcp://localhost:26657` | RPC endpoint |
| `--chain-id <id>` | `$LUMERA_CHAIN_ID` | Chain ID (required; aborts with exit 1 if unset) |
| `--keyring-backend <b>` | `test` | `test`, `file`, or `os` — same values as `lumerad keys` |
| `--keyring-dir <dir>` | *(unset)* | Point keyring at a directory independent of `--home` |
| `--home <dir>` | `lumerad`'s default (`~/.lumera`) | Passed through to `lumerad` |
| `--mnemonic-file <path>` | *(unset)* | One-shot import: read mnemonic from file (mode `0600`), derive both keys, clean up after |
| `--yes`, `-y` | off | Skip the standard "Proceed with migration?" prompt |
| `--dry-run` | off | Run preview and pre-flight checks, do not broadcast |
| `--binary <path>` | `lumerad` from `$PATH` | Override which `lumerad` executable to invoke |

Environment variable fallbacks: `LUMERA_NODE`, `LUMERA_CHAIN_ID`.

---

## Account migration walkthrough

### 1. Import both keys

```bash
# Legacy key (coin-type 118)
lumerad keys add legacy-key \
  --recover --coin-type 118 --algo secp256k1 \
  --keyring-backend test

# New EVM key (coin-type 60)
lumerad keys add new-key \
  --recover --coin-type 60 --algo eth_secp256k1 \
  --keyring-backend test
```

Enter the **same mnemonic** for both. Confirm the legacy address matches your pre-EVM address on chain.

### 2. Preview with `--dry-run`

```bash
./scripts/migrate-account.sh legacy-key new-key \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --dry-run --yes
```

The preview block shows what will move (balance, delegations, unbondings, authz grants, fee grants, supernode, validator/multisig flags, whether migration would succeed). Exits 0 if pre-flight is clean.

### 3. Run the migration

Remove `--dry-run` to broadcast. Keep `--yes` to skip the interactive confirmation, or drop it for a y/N prompt:

```bash
./scripts/migrate-account.sh legacy-key new-key \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657
```

The script broadcasts, waits up to 30 seconds for inclusion, verifies the migration record points at the new address, confirms the legacy balance is zero, and confirms the new address holds at least the pre-broadcast snapshot per denom. On success you'll see:

```text
INFO  migration complete
INFO    legacy: lumera1…
INFO    new:    lumera1…
INFO    tx:     DEADBEEF…
```

### 4. (Optional) Clean up the legacy key

```bash
lumerad keys delete legacy-key --keyring-backend test
```

### One-shot mnemonic flow

If you'd rather not touch the keyring yourself:

```bash
chmod 0600 /secure/tmp/mnemonic.txt   # must be mode 0600 or stricter
./scripts/migrate-account.sh legacy-key new-key \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --mnemonic-file /secure/tmp/mnemonic.txt --yes
```

The script imports both keys into your chosen keyring under the given names, runs the migration, and deletes both keys from the keyring on exit (success or failure) via a cleanup trap. The mnemonic file itself is never modified. Both key names must not already exist in the keyring — the script refuses to overwrite.

---

## Validator migration walkthrough

Validator migration has an additional risk: the node is stopped throughout the migration window, so it misses blocks and can be jailed if you don't restart promptly.

### 1. Plan the maintenance window

The re-keying operation touches every delegation, unbonding, and redelegation that references your validator. Most migrations complete in one block; allocate a downtime budget that accounts for your `signed_blocks_window` / `min_signed_per_window` parameters plus a margin for the restart.

### 2. Preview

```bash
./scripts/migrate-validator.sh legacy-op-key new-evm-key \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --i-have-stopped-the-node --yes --dry-run
```

The preview adds **Validator delegations / unbondings / redelegations (to validator)** counts alongside the per-operator figures. If the sum is within 10% of `max_validator_delegations`, the script logs a warning; if it exceeds the cap, it aborts with exit 6 (governance needs to raise the cap, or some delegators need to redelegate first).

### 3. Stop the validator node

Stop the node however your setup requires (`systemctl stop lumerad`, `docker compose stop lumerad`, etc.). The scripts do not touch service managers.

### 4. Run the migration

```bash
./scripts/migrate-validator.sh legacy-op-key new-evm-key \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --i-have-stopped-the-node
```

`--i-have-stopped-the-node` satisfies the downtime acknowledgement non-interactively. Omit it to get an interactive prompt that requires the full word `yes` (not `y`). `--yes` does **not** satisfy this check — that's deliberate. Non-interactive runs without the flag abort with exit 10 instead of hanging.

On success the script prints:

```text
INFO  validator migration complete — post-migration checklist:
INFO    1. Import <new-key> into the production keyring (correct --keyring-backend)
INFO    2. Restart lumerad
INFO    3. Verify new operator via: lumerad query staking validator <new-valoper>
INFO    4. Monitor missed-block counters for the next few blocks
```

### 5. Restart the validator

Import the new EVM key into the node's production keyring if it isn't already there, then start the binary:

```bash
lumerad keys add new-operator-key \
  --recover --coin-type 60 --algo eth_secp256k1 \
  --keyring-backend file

systemctl start lumerad   # or your platform's equivalent
```

The consensus key (`priv_validator_key.json`, ed25519) is **not affected** by this migration — only the operator key.

---

## Exit codes

The scripts use a stable exit-code scheme so you can wrap them in higher-level automation.

| Code | Meaning | Typical cause |
|---|---|---|
| `0` | Success, or dry-run completed cleanly | — |
| `1` | Usage error | Wrong number of positional args; unknown flag; flag-shaped value (e.g. `--node --chain-id …`); mnemonic file missing or mode > `0600`; key name collision in the keyring |
| `2` | Environment error | `lumerad` missing, wrong version, or missing required subcommands; `jq` missing; RPC endpoint unreachable; migration-record / bank-balances query failed |
| `3` | Multisig rejected | Legacy account's `is_multisig` flag is true — use the offline 4-step flow |
| `4` | Pre-flight estimate failed | Keeper returned `would_succeed=false`; `rejection_reason` is printed |
| `5` | Already migrated | Legacy address already has a migration record, or the new address was previously used as a destination |
| `6` | Wrong-script or cap error | `migrate-account.sh` invoked on a validator, `migrate-validator.sh` invoked on a non-validator, or validator's `val_*_count` total exceeds `max_validator_delegations` |
| `7` | Post-verification failed | Broadcast succeeded but the post-migration checks didn't pass: record missing / wrong new address / non-zero legacy balance / new balance below pre-broadcast snapshot. The transaction is already on-chain; investigate manually |
| `10` | User aborted | User declined a confirmation prompt, or the validator downtime acknowledgement was not satisfied |

## Troubleshooting

Keyed by symptom / exit code.

### Exit 1: `expected exactly two positional arguments`

You're missing one of the key names. Both are required even with `--mnemonic-file`: the script uses them as the names for the imported keys.

### Exit 1: `--foo requires a value`

A flag was followed by another flag instead of a value. Common example:

```bash
# Wrong — --chain-id is consumed as the value of --node
./scripts/migrate-account.sh legacy new --node --chain-id lumera-mainnet-1
```

Fix: put flags and values in order, or quote if the value genuinely starts with `--` (rare).

### Exit 1: `mnemonic file … must be mode 0600`

The mnemonic file is group- or world-readable. Fix permissions:

```bash
chmod 0600 /path/to/mnemonic
```

The scripts also refuse to run if either key name already exists in the keyring — pick unused names or delete the existing keys first.

### Exit 2: `lumerad binary not found` or `does not support 'tx evmigration …'`

Either `$BIN` (default `lumerad`) isn't on `$PATH`, or the binary is pre-EVM-upgrade. Build from master or the upgrade tag and point `--binary /path/to/lumerad` at the new build.

### Exit 2: `could not query migration-record … verify manually`

The `migration-record` query failed — node is unreachable, the RPC endpoint is wrong, or the node's tx indexer is still catching up. Confirm with:

```bash
lumerad status --node <your-endpoint>
lumerad query evmigration params --node <your-endpoint>
```

Then re-run.

### Exit 3: `legacy account is a K-of-N multisig`

The scripts only handle single-sig accounts. Multisig migration uses an offline coordinator-driven ceremony — see [legacy-migration.md](../evmigration/legacy-migration.md#multisig-account-migration).

### Exit 4: `pre-flight: migration would fail: …`

The keeper's `migration-estimate` returned `would_succeed=false`. The `rejection_reason` printed alongside explains why. Common reasons:

- `legacy account not found` — the address has never held any state on-chain.
- `validator is not in bonded status` — used for validator migration of an unbonding/unbonded validator.
- Migration is disabled via governance (`enable_migration=false`) or past `migration_end_time`.

### Exit 5: `… has already been migrated` or `new address … is already a migration destination`

Idempotency guard. The target address pair has already been used. Check the existing record:

```bash
lumerad query evmigration migration-record <legacy-addr>
lumerad query evmigration migration-record-by-new-address <new-addr>
```

### Exit 6: `account is a validator — use scripts/migrate-validator.sh instead`

You invoked `migrate-account.sh` on a validator operator address. Validators require the `migrate-validator` tx variant (which re-keys all delegations pointing to the validator, not just the operator's own state). Run:

```bash
./scripts/migrate-validator.sh <legacy-op-key> <new-evm-key> …
```

### Exit 6: `exceeds max_validator_delegations=<cap>`

Your validator has more delegation/unbonding/redelegation records referencing it than the per-tx safety cap allows. Options: governance raises `max_validator_delegations`, or delegators redelegate to other validators until the count drops below the cap.

### Exit 7: post-migration verification failed

The broadcast succeeded — the on-chain state has already been moved — but one of the three post-checks failed. The script prints the specific check that failed. Verify manually:

```bash
lumerad query evmigration migration-record <legacy-addr>
lumerad query bank balances <legacy-addr>
lumerad query bank balances <new-addr>
```

If the record exists and the new balances look correct, the verification failed due to transient RPC issues and your migration is actually fine. If not, contact the release maintainers with the transaction hash.

### Exit 10: `validator downtime not acknowledged`

You invoked `migrate-validator.sh` without `--i-have-stopped-the-node` in a non-interactive context (systemd, CI, SSH without `-t`). The script requires either the flag or an interactive `yes` prompt. `--yes` alone does not satisfy this check — that's intentional to force explicit acknowledgement.

### Script hangs at `Type "yes" to confirm the node is stopped`

You're running interactively and need to type the literal word `yes` (not `y`). For non-interactive automation use `--i-have-stopped-the-node` instead.

---

## Non-interactive usage

All confirmation prompts except the validator downtime prompt are skipped with `-y` / `--yes`. The validator downtime prompt is controlled separately by `--i-have-stopped-the-node`. Non-interactive runs with `--yes` but without `--i-have-stopped-the-node` fail fast with exit 10 instead of hanging.

For scripted automation (CI jobs, runbooks, etc.):

```bash
./scripts/migrate-validator.sh legacy-op new-evm \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --yes --i-have-stopped-the-node
```

The scripts never prompt for a keyring password — that's governed entirely by `--keyring-backend`. Use `test` for automation (no password), `file` if you want password-protected keyrings (interactive once at load), or `os` for the OS keychain.

---

## Multisig migration

Multisig legacy accounts use a four-step offline ceremony rather than a single command — one coordinator and K co-signers across different machines. The destination is **also** a K-of-N multisig, built from `eth_secp256k1` sub-keys (the "mirror-source rule" — see [evmigration/main.md → Multisig account migration](../evmigration/main.md#multisig-account-migration)). The `scripts/migrate-multisig.sh` wrapper layers the same pre-flight and verification rails onto each step.

> **Consensus invariants (multisig).** The chain rejects a multisig migration tx at `ValidateBasic` if any of these are violated:
>
> - **Shape + K/N mirror.** K-of-N legacy → K-of-N new, same K and same N (`ErrMirrorSourceMismatch`).
> - **Matching `signer_indices`.** The same K signer positions approve both halves — a co-signer who signs only one side doesn't count on the other.
> - **Sub-key uniqueness.** No duplicate entries in either side's `sub_pub_keys` list.
> - **Zero-signer submit.** `submit` takes no `--from`, no fee flags, no envelope signature.
>
> The wrapper's `combine` step mirrors these rules before writing `tx.json`, so what it produces will pass `ValidateBasic`. Full reference: [legacy-migration.md § Consensus invariants](../evmigration/legacy-migration.md#consensus-invariants).

Before you begin:

- Every co-signer and the coordinator need `lumerad` (post-EVM-upgrade) and `jq` on their machine.
- The multisig's on-chain pubkey must already be seeded (any prior multisig-signed transaction registers it). If it's nil, submit any multisig-signed tx first — e.g. a 1-`ulume` self-send via `lumerad tx bank send`.
- Each co-signer generates a fresh `eth_secp256k1` sub-key in their own keyring (`lumerad keys add <name>-eth --key-type eth_secp256k1`). The coordinator collects the N eth pubkeys (or local key-names, if signers share a keyring) and derives the new multisig address with `lumerad keys add --multisig ... --multisig-threshold K`.

### Step 1 — Coordinator: generate the proof template

```bash
./scripts/migrate-multisig.sh generate \
  --legacy lumera1<multisig-bech32> \
  --new-sub-pub-keys <k1>,<k2>,<k3> \
  --new-threshold    2 \
  --kind   claim \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657 \
  --keyring-backend file \
  --out  proof.json
```

`--new-sub-pub-keys` entries may be local keyring key names (if the coordinator imported each co-signer's eth pubkey) **or** base64-encoded 33-byte compressed `eth_secp256k1` pubkeys — mix freely. `--new` is optional; if supplied, the CLI cross-checks it against the address derived from `--new-sub-pub-keys` and `--new-threshold`. `--sig-format` applies to the legacy side (default `SIG_FORMAT_CLI`); the new side defaults to the same for multisig.

Use `--kind validator` if the multisig holds a validator operator. The wrapper checks `is_multisig` and `is_validator` against the pre-flight estimate and aborts with exit 3 (not multisig) or exit 6 (validator flag on non-validator) before calling `lumerad`. If the on-chain pubkey is nil, it exits 8 with the remediation printed. If the migration-estimate already says `would_succeed=false` (governance disabled migration, deadline passed, validator over cap), it exits 4 before wasting co-signer time on a doomed ceremony.

Distribute `proof.json` to all co-signers.

### Step 2 — Each co-signer: append partial signatures (both sides)

Each co-signer holds their legacy Cosmos sub-key **and** their destination-side eth sub-key in the same keyring, and signs both sides in one invocation:

```bash
./scripts/migrate-multisig.sh sign proof.json \
  --from    alice-legacy-sub \
  --new-key alice-eth-sub \
  --chain-id lumera-mainnet-1 \
  --keyring-backend file \
  --out alice-partial.json
```

`--from` signs the legacy half; `--new-key` signs the new half. At least one is required. A co-signer who holds only one sub-key may pass just that flag, but **one-sided partials do not count toward quorum by themselves** — the mirror-source rule requires the same K signer positions to approve both halves, so the wrapper's `combine` step only counts an index that has a valid signature on *both* sides. One-sided partials contribute only when another co-signer supplies the other-side signature at the same index. The wrapper validates the proof file's `payload_hex` against a canonical reconstruction (catches tampering; exit 9), confirms `--from`'s pubkey is in `legacy.sub_pub_keys`, and `--new-key`'s pubkey is in `new.sub_pub_keys` (catches "wrong signer" mistakes; exit 1). Re-running overwrites the signer's prior entries on both sides (idempotent). Each signer sends their `*-partial.json` back to the coordinator.

### Step 3 — Coordinator: combine partials

```bash
./scripts/migrate-multisig.sh combine \
  alice-partial.json bob-partial.json \
  --out tx.json
```

The wrapper cross-checks that every partial agrees on `chain_id`, `legacy_address`, `new_address`, `payload_hex`, `kind`, and the per-side `threshold`, `sig_format`, and `sub_pub_keys` lists (exit 9 on disagreement). It prints **two** K-of-N entry-presence matrices — one per side:

```text
Legacy-side partials (2-of-3 required):
  [X] signer 0  alice-partial.json
  [X] signer 1  bob-partial.json
  [ ] signer 2  (missing)
Legacy threshold satisfied: yes (2 >= 2)
New-side partials (2-of-3 required):
  [X] signer 0  alice-partial.json
  [X] signer 1  bob-partial.json
  [ ] signer 2  (missing)
New threshold satisfied: yes (2 >= 2)
```

The wrapper also computes the **matching-index** count — the size of the intersection between legacy and new signer-index sets — and gates its return on that count meeting K. Per-side thresholds can BOTH report "yes" while the intersection is below K (e.g. legacy signed at `{0,1}`, new signed at `{0,2}` — each side has 2 entries, but only index 0 is signed on both). When the intersection is short, the wrapper prints:

```text
Matching-index threshold satisfied: no (1 < 2) — one-sided partials do not count
```

and aborts with exit 4 before calling `lumerad`. If `lumerad combine-proof` itself reports fewer than K *cryptographically valid* signatures signed on both sides at matching indices (wrong key, tampered payload), the wrapper maps that to exit 4 as well. This mirrors the consensus invariant: `legacy_proof.signer_indices == new_proof.signer_indices`.

### Step 4 — Coordinator: submit

```bash
./scripts/migrate-multisig.sh submit tx.json \
  --chain-id lumera-mainnet-1 \
  --node tcp://rpc.lumera:26657
```

`submit-proof` does **not** sign at the Cosmos tx layer — migration messages declare zero signers (authorization is embedded in the proof bytes), fees are waived by the evmigration ante handler, and replay is prevented by the keeper's migration record. There is no `--from`; `--keyring-backend` / `--keyring-dir` / `--home` are accepted for SDK-context plumbing only.

Pre-flight checks (in order): reject non-multisig→multisig tx JSON (exit 3); the legacy address has no migration record yet; the new address isn't already a migration destination; a fresh `migration-estimate` still reports `would_succeed: true` (catches state drift during a multi-hour or multi-day ceremony — governance could have disabled migration, a validator could have exceeded `max_validator_delegations`). After broadcast it waits for inclusion and verifies the migration record matches.

For `--kind validator` tx files, the submit step prints the same downtime banner as `migrate-validator.sh` and requires either `--i-have-stopped-the-node` or a typed `yes` response. `--yes` does not satisfy this check.

### Multisig-specific exit codes

In addition to the codes shared with single-sig scripts ([Exit codes](#exit-codes) above):

| Code | Meaning |
|---|---|
| `8` | Multisig pubkey not seeded on-chain; submit any multisig-signed tx first |
| `9` | Input file integrity check failed (JSON parse, missing field, payload_hex mismatch, cross-file disagreement, unsupported version) |

### Multisig troubleshooting

- **Exit 8 on `generate`**: the multisig has never signed a tx. Run any transaction from the multisig account first (smallest: `lumerad tx bank send <multisig-addr> <multisig-addr> 1ulume --from <sub-key>` in the usual multisig coordinator flow). Then retry.
- **Exit 9 on `sign` with "payload_hex mismatch"**: someone edited a field in the proof after generation. Regenerate from the coordinator and redistribute.
- **Exit 9 on `sign` with "unsupported version"**: the proof file was produced by an older `lumerad` with the v1 schema. Regenerate with a post-upgrade binary; v1 files are not migratable in place.
- **Exit 1 on `sign` with "not among legacy.sub_pub_keys"**: the `--from` key's pubkey isn't listed in the template's `legacy.sub_pub_keys`. Confirm you imported the correct Cosmos sub-key (wrong key name, wrong mnemonic, wrong HD path — coin-type must be 118 for legacy sub-keys).
- **Exit 1 on `sign` with "not among new.sub_pub_keys"**: the `--new-key` pubkey isn't in the template's new-side sub-keys — most commonly because the coordinator built `--new-sub-pub-keys` from a different set of eth keys than the co-signers now hold.
- **Exit 4 on `combine`**: either you passed partials short of K on either side, or one or more partials had invalid signatures. The wrapper prints per-side entry-presence summaries before invoking `lumerad`; if entries look fine but `lumerad` reports below-threshold valid sigs, the bad signer needs to re-sign.
- **Exit 4 on `submit`**: chain state changed during the ceremony (governance disabled migration, deadline passed, validator over cap). The `rejection_reason` from the fresh estimate is printed.

---

## Related documentation

- [migration.md](migration.md) — top-level migration methods (Portal, scripts, raw CLI)
- [validator-migration.md](validator-migration.md) — full validator-specific walkthrough with maintenance-window planning
- [supernode-migration.md](supernode-migration.md) — supernode operators and daemon-driven migration
- [legacy-migration.md](../evmigration/legacy-migration.md) — architectural reference, including the multisig offline flow
- [evmigration-scripts-design.md](../../design/evmigration-scripts-design.md) — design doc for the scripts themselves
