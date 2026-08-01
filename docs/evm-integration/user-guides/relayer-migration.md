# Hermes IBC Relayer EVM Migration Guide

**Last updated**: 2026-06-16
**Applies to**: operators running a Hermes IBC relayer whose Lumera signing account is a legacy (coin-type 118 `secp256k1`) account, against an EVM-enabled Lumera chain (post-EVM upgrade)
**Prerequisite reading**: [migration.md](migration.md) for the chain-level mechanics of legacy → EVM account migration

> **Mandatory operations gate:** Use the [EVM Migration Operator Runbook](operator-migration-runbook.md) before this relayer procedure. The runbook adds release/binary/keyring provenance, mode-0600 config backup, no-echo destination staging, verified stop/restart alternatives, a one-broadcast rule, and secret-redacted evidence; this guide remains authoritative for Hermes' pinned HD path and key replacement.
>
> Discover the relayer service user's absolute Hermes base directory and use it for every read, backup, and mutation. Do not allow the invoking user's `~` or `$HOME` to select a relayer file. Populate and verify the exact manifest artifact before continuing:
>
> ```bash
> MANIFEST=/absolute/path/to/approved/compatibility-manifest.json
> HERMES=$(jq -er '.artifacts.relayer_executable.release_path' "$MANIFEST")
> HERMES_SHA256=$(jq -er '.artifacts.relayer_executable.sha256' "$MANIFEST")
> LUMERAD=$(jq -er '.artifacts.chain_executable.release_path' "$MANIFEST")
> LUMERAD_SHA256=$(jq -er '.artifacts.chain_executable.sha256' "$MANIFEST")
> MIGRATE_ACCOUNT=$(jq -er '.artifacts.bound_files["scripts/migrate-account.sh"].release_path' "$MANIFEST")
> MIGRATE_ACCOUNT_SHA256=$(jq -er '.artifacts.bound_files["scripts/migrate-account.sh"].sha256' "$MANIFEST")
> EVMIGRATION_COMMON=$(jq -er '.artifacts.bound_files["scripts/evmigration-common.sh"].release_path' "$MANIFEST")
> EVMIGRATION_COMMON_SHA256=$(jq -er '.artifacts.bound_files["scripts/evmigration-common.sh"].sha256' "$MANIFEST")
> HERMES_HOME=/absolute/discovered/hermes/home
> HERMES_CONFIG="$HERMES_HOME/config.toml"
> test "${HERMES#/}" != "$HERMES" && test "${LUMERAD#/}" != "$LUMERAD"
> test "${MIGRATE_ACCOUNT#/}" != "$MIGRATE_ACCOUNT" && test "${EVMIGRATION_COMMON#/}" != "$EVMIGRATION_COMMON"
> test "${HERMES_HOME#/}" != "$HERMES_HOME"
> printf '%s  %s\n%s  %s\n%s  %s\n%s  %s\n' \
>   "$HERMES_SHA256" "$HERMES" \
>   "$LUMERAD_SHA256" "$LUMERAD" \
>   "$MIGRATE_ACCOUNT_SHA256" "$MIGRATE_ACCOUNT" \
>   "$EVMIGRATION_COMMON_SHA256" "$EVMIGRATION_COMMON" | sha256sum -c -
> test -f "$HERMES_CONFIG"
> ```

---

## Overview

A Hermes relayer signs IBC packet/ack/timeout transactions on Lumera using a key under its discovered absolute `$HERMES_HOME/keys/<chain-id>/keyring-test/<key_name>.json`. If that account was created before the EVM upgrade it is a legacy `secp256k1` (coin-type 118) account.

Migrating the relayer account is different from migrating a normal user account in one important way: **two independent tools must agree on the key.** `lumerad` performs the migration (it needs the destination key in its keyring to sign `MsgClaimLegacyAccount`), and **Hermes** must independently re-derive the *same* account from the *same* mnemonic so it can keep signing. Those two derivations only line up if you pin the HD path — see the gotcha below.

> **This is a service-affecting, irreversible change.** The legacy relayer address is blocked after migration. Plan a short relaying pause, and do not migrate until the derivation gate (Step 3) passes.

---

## The HD-path gotcha (read this first)

Lumera's EVM keys use `eth_secp256k1` at HD path `m/44'/60'/0'/0/0`. `lumerad keys add --coin-type 60 --algo eth_secp256k1` uses exactly that path.

**Hermes' default `ethermint` derivation does NOT use `m/44'/60'/0'/0/0`.** Given the same mnemonic, Hermes' default and lumerad produce *different* Lumera addresses:

| derivation | address (example) |
| --- | --- |
| `lumerad --coin-type 60 --algo eth_secp256k1` (path `m/44'/60'/0'/0/0`) | `lumera1ccvqdk…` |
| `hermes keys add` (ethermint **default** path) | `lumera1addyff…` ❌ different |
| `hermes keys add --hd-path "m/44'/60'/0'/0/0"` | `lumera1ccvqdk…` ✅ matches lumerad |

**Always pass `--hd-path "m/44'/60'/0'/0/0"` to `hermes keys add`** for a Lumera EVM relayer key. Without it Hermes derives a key for a different address and will not control the migrated account.

You must also tell Hermes the chain uses Ethereum-style keys, via `address_type` in `config.toml` (Step 1).

---

## Prerequisites

- The manifest-pinned Hermes executable is ≥ 1.10 (verified on 1.13.2) with `ethermint` address-type support.
- The relayer mnemonic, or the legacy relayer key present in a `lumerad` keyring (to sign the migration).
- The manifest-pinned chain executable supports `eth_secp256k1`, and its pinned query confirms the migration window is open.

Confirm the current relayer account is legacy using `$HERMES --config "$HERMES_CONFIG"` as the relayer service user and the runbook's manifest-pinned chain-query form. Do not use PATH lookup or a default Hermes config.

---

## Procedure

Throughout, `lumera-mainnet-1` is the chain id and `relayer` is the Hermes `key_name` for the Lumera chain — adjust to your config.

### Step 1 — Tell Hermes the Lumera chain uses Ethereum-style keys

Edit the discovered absolute `$HERMES_CONFIG` and add `address_type` to the **Lumera** `[[chains]]` block:

```toml
[[chains]]
id = 'lumera-mainnet-1'
# … existing fields …
key_name = 'relayer'
address_type = { derivation = 'ethermint', proto_type = { pk_type = '/cosmos.evm.crypto.v1.ethsecp256k1.PubKey' } }
```

> The `proto_type.pk_type` is `'/cosmos.evm.crypto.v1.ethsecp256k1.PubKey'` for Cosmos EVM (v0.6.0). It is **not** `pid`, and it is **not** the ethermint `'/ethermint.crypto.v1.ethsecp256k1.PubKey'` URL.

Validate:

```bash
sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" config validate
```

### Step 2 — Pre-stage the EVM destination key in lumerad

Use only the compatibility manifest's **verified, named, and hashed PR-2 no-echo implementation** to create/recover `relayer-evm` as coin type 60 / `eth_secp256k1` in the exact lumerad keyring discovered by the operator runbook. The implementation must accept a hidden TTY or protected file descriptor and must not place the mnemonic in argv, a shell variable, shell history, or logs. If that manifest dependency is blocked, stop.

Record only the public destination address:

```bash
NEWADDR=$(sudo -u <relayer-service-user> "$LUMERAD" keys show relayer-evm -a \
  --home /absolute/lumera/home \
  --keyring-backend <backend> \
  --keyring-dir /absolute/keyring/location)
printf '%s\n' "$NEWADDR"
```

The destination mnemonic is transferred to Hermes only through the same protected custody channel used by PR-2. The following steps refer to a temporary operator-owned mode-`0600` file at `/run/secrets/relayer-evm.mnemonic`; never place its contents in a shell variable or shared path, and securely remove it after Step 6.

### Step 3 — GATE: prove Hermes derives the same address

Before the irreversible migration, confirm Hermes derives **exactly** `$NEWADDR` from the relayer mnemonic with the pinned path:

```bash
umask 077
test "$(stat -c '%a' /run/secrets/relayer-evm.mnemonic)" = 600
sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" keys add \
  --chain lumera-mainnet-1 --key-name relayer-evm-gate \
  --mnemonic-file /run/secrets/relayer-evm.mnemonic \
  --hd-path "m/44'/60'/0'/0/0"

if ! HERMES_KEYS_OUTPUT=$(sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" keys list --chain lumera-mainnet-1); then
  echo 'Hermes keys list failed; do not migrate' >&2
  exit 1
fi
if ! HERMES_GATE_ADDR=$(printf '%s\n' "$HERMES_KEYS_OUTPUT" |
  EVMIGRATION_COMMON="$EVMIGRATION_COMMON" EXPECTED_ADDR="$NEWADDR" bash -c \
    'source "$EVMIGRATION_COMMON"; require_hermes_key_address relayer-evm-gate "$EXPECTED_ADDR"'); then
  echo 'Hermes destination-address gate failed; do not migrate' >&2
  exit 1
fi
test "$HERMES_GATE_ADDR" = "$NEWADDR"
sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" keys delete --chain lumera-mainnet-1 --key-name relayer-evm-gate
```

If the addresses differ, re-check Step 1 (`address_type`) and the `--hd-path`. **Do not proceed past this gate on a mismatch** — you would migrate funds to an account Hermes cannot sign for.

### Step 4 — Pause relaying and migrate the account

Stop and prove stopped using the relayer's actual supervisor branch in the operator runbook. Then run the manifest-pinned helper as the discovered service user, with the exact approved binary/home/backend/keyring and trusted external RPC. The successful post-stop dry-run and destination-address match are mandatory; the following live command is run once only:

```bash
sudo -u <relayer-service-user> "$MIGRATE_ACCOUNT" \
  relayer-legacy relayer-evm \
  --binary "$LUMERAD" \
  --home /absolute/lumera/home \
  --keyring-backend <backend> \
  --keyring-dir /absolute/keyring/location \
  --chain-id lumera-mainnet-1 \
  --node <trusted-rpc>
```

For Docker/Kubernetes, use the runbook's one-shot container/Pod form with these account-helper arguments; do not run this host/systemd form against container storage. `relayer-legacy` is the legacy relayer key in the same lumerad keyring. Migration is fee-free; the full balance moves to `$NEWADDR`. On a tx hash or ambiguous transport failure, apply the runbook's query-before-retry rule. Verify:

```bash
"$LUMERAD" query evmigration migration-record <legacy-addr> --node <trusted-rpc> --output json | jq '.record.new_address'
"$LUMERAD" query bank balances "$NEWADDR" --node <trusted-rpc> --output json | jq '.balances'
```

### Step 5 — Replace the Hermes relayer key with the EVM key

Back up and remove the old key, then import the EVM key under the relayer's `key_name`, **with the pinned path**. Keep the backup on an encrypted operator-controlled volume, never in a shared `/tmp` path:

```bash
umask 077
RELAYER_BACKUP_DIR=/absolute/encrypted/operator-backup/relayer-$(date -u +%Y%m%dT%H%M%SZ)
install -d -m 0700 "$RELAYER_BACKUP_DIR"
install -m 0600 "$HERMES_HOME/keys/lumera-mainnet-1/keyring-test/relayer.json" \
  "$RELAYER_BACKUP_DIR/relayer.json.bak"
stat -c '%a %n' "$RELAYER_BACKUP_DIR/relayer.json.bak"  # must be 600

sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" keys delete --chain lumera-mainnet-1 --key-name relayer
sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" keys add \
  --chain lumera-mainnet-1 --key-name relayer \
  --mnemonic-file /run/secrets/relayer-evm.mnemonic \
  --hd-path "m/44'/60'/0'/0/0"

if ! HERMES_KEYS_OUTPUT=$(sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" keys list --chain lumera-mainnet-1); then
  echo 'Hermes keys list failed; keep relayer stopped' >&2
  exit 1
fi
if ! HERMES_ACTIVE_ADDR=$(printf '%s\n' "$HERMES_KEYS_OUTPUT" |
  EVMIGRATION_COMMON="$EVMIGRATION_COMMON" EXPECTED_ADDR="$NEWADDR" bash -c \
    'source "$EVMIGRATION_COMMON"; require_hermes_key_address relayer "$EXPECTED_ADDR"'); then
  echo 'Active Hermes key does not control the migrated destination; keep relayer stopped' >&2
  exit 1
fi
test "$HERMES_ACTIVE_ADDR" = "$NEWADDR"
```

### Step 6 — Restart and verify

```bash
# Restart through the discovered supervisor so Hermes loads the pinned config and new key.
sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" keys balance --chain lumera-mainnet-1
sudo -u <relayer-service-user> "$HERMES" --config "$HERMES_CONFIG" health-check
```

Watch the logs for a few packets to confirm it signs and broadcasts cleanly. IBC clients, connections, and channels are **not** owned by the relayer account, so they are unaffected by the key change — Hermes just needs a funded, signable account.

---

## Rollback / abort

- **Before Step 4 (migration):** fully reversible. Restore `address_type` removal in `config.toml`, restore the mode-`0600` `relayer.json` from the encrypted backup if you touched it, and delete `relayer-evm*` keys. Nothing on-chain changed.
- **After Step 4:** the legacy address is permanently blocked; there is no rollback. The only recovery is forward — ensure Hermes holds the EVM key (Step 5) and is funded.

Keep the encrypted `relayer.json.bak` only until Step 6 verifies; a stray `.json` left in the live `keyring-test/` directory can confuse Hermes' key loader. After relaying is confirmed, securely remove both the backup and `/run/secrets/relayer-evm.mnemonic` according to the custody platform's deletion procedure.

---

## Why this is fiddly (and the others aren't)

For a normal user/validator/supernode migration, `lumerad` (or the supernode daemon) is the *only* signer, so its derivation never has to agree with anything external. The relayer is the one account where a **second tool** (Hermes) must reconstruct the same key from the mnemonic. That makes the HD path a hard requirement, not a convenience:

- Same mnemonic + same path (`m/44'/60'/0'/0/0`) + same algo (`eth_secp256k1`) ⇒ same private key ⇒ same address.
- Hermes' default ethermint path differs, so omitting `--hd-path` silently produces a *different* account — which is why the Step 3 gate exists.

> **Runbook rule:** relayer/operator-key migration needs the key transferred (shared mnemonic with a pinned HD path, or an exported private key), not just "use the same seed." Always gate on a derived-address match before the irreversible migration step.
