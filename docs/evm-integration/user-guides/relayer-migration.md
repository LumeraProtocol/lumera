# Hermes IBC Relayer EVM Migration Guide

**Last updated**: 2026-06-16
**Applies to**: operators running a Hermes IBC relayer whose Lumera signing account is a legacy (coin-type 118 `secp256k1`) account, against an EVM-enabled Lumera chain (post-EVM upgrade)
**Prerequisite reading**: [migration.md](migration.md) for the chain-level mechanics of legacy → EVM account migration

---

## Overview

A Hermes relayer signs IBC packet/ack/timeout transactions on Lumera using a key in its own keyring (`~/.hermes/keys/<chain-id>/keyring-test/<key_name>.json`). If that account was created before the EVM upgrade it is a legacy `secp256k1` (coin-type 118) account, and it appears in `lumerad query evmigration legacy-accounts` until migrated.

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

- Hermes ≥ 1.10 (verified on 1.13.2) with `ethermint` address-type support.
- The relayer mnemonic, or the legacy relayer key present in a `lumerad` keyring (to sign the migration).
- A `lumerad` binary that supports `eth_secp256k1` (the EVM-era Lumera binary).
- The migration window open on-chain (`lumerad query evmigration params` → `enable_migration: true`).

Confirm the current relayer account is legacy:

```bash
# from the Hermes host
ADDR=$(hermes keys list --chain lumera-mainnet-1 | grep ' relayer ' | grep -oE 'lumera1[0-9a-z]+' | head -1)
lumerad query evmigration migration-record "$ADDR" --output json   # record.new_address empty == not migrated
lumerad query auth account "$ADDR" --output json                   # pub_key type secp256k1 == true legacy
```

---

## Procedure

Throughout, `lumera-mainnet-1` is the chain id and `relayer` is the Hermes `key_name` for the Lumera chain — adjust to your config.

### Step 1 — Tell Hermes the Lumera chain uses Ethereum-style keys

Edit `~/.hermes/config.toml` and add `address_type` to the **Lumera** `[[chains]]` block:

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
hermes config validate    # must print: SUCCESS configuration is valid
```

### Step 2 — Create the EVM destination key in lumerad

The migration's destination key must live in the **lumerad** keyring (alongside the legacy relayer key, so `claim-legacy-account` can sign). Create it and note its address:

```bash
lumerad keys add relayer-evm --coin-type 60 --algo eth_secp256k1 --keyring-backend test
NEWADDR=$(lumerad keys show relayer-evm -a --keyring-backend test)
echo "$NEWADDR"
```

Capture the mnemonic printed at creation — you need it for Hermes in Step 5. (Alternatively, generate the key inside Hermes in Step 3 and recover it into lumerad; either way the same mnemonic must end up in both keyrings.)

### Step 3 — GATE: prove Hermes derives the same address

Before the irreversible migration, confirm Hermes derives **exactly** `$NEWADDR` from the relayer mnemonic with the pinned path:

```bash
# pipe the mnemonic in; never write it to a shared path
printf '%s\n' "$RELAYER_EVM_MNEMONIC" | \
  hermes keys add --chain lumera-mainnet-1 --key-name relayer-evm-gate \
    --mnemonic-file /dev/stdin --hd-path "m/44'/60'/0'/0/0"

hermes keys list --chain lumera-mainnet-1 | grep relayer-evm-gate
# the printed address MUST equal $NEWADDR. If it does not, STOP — do not migrate.
hermes keys delete --chain lumera-mainnet-1 --key-name relayer-evm-gate
```

If the addresses differ, re-check Step 1 (`address_type`) and the `--hd-path`. **Do not proceed past this gate on a mismatch** — you would migrate funds to an account Hermes cannot sign for.

### Step 4 — Pause relaying and migrate the account

Stop the relayer so it does not sign with an account that is about to be blocked, then migrate. (`migrate-account.sh` is the bundled helper — see [migration-scripts.md](migration-scripts.md). Both keys must be in the same lumerad keyring.)

```bash
# stop the hermes process/container (resumes after Step 6)
./scripts/migrate-account.sh relayer-legacy relayer-evm \
  --chain-id lumera-mainnet-1 --node tcp://localhost:26657 --yes
```

`relayer-legacy` is the legacy relayer key in your lumerad keyring. Migration is fee-free; the full balance moves to `$NEWADDR`. Verify:

```bash
lumerad query evmigration migration-record <legacy-addr> --output json | jq '.record.new_address'  # == $NEWADDR
lumerad query bank balances "$NEWADDR" --output json | jq '.balances'                               # funds present
```

### Step 5 — Replace the Hermes relayer key with the EVM key

Back up and remove the old key, then import the EVM key under the relayer's `key_name`, **with the pinned path**:

```bash
cp ~/.hermes/keys/lumera-mainnet-1/keyring-test/relayer.json /tmp/relayer.json.bak
hermes keys delete --chain lumera-mainnet-1 --key-name relayer

printf '%s\n' "$RELAYER_EVM_MNEMONIC" | \
  hermes keys add --chain lumera-mainnet-1 --key-name relayer \
    --mnemonic-file /dev/stdin --hd-path "m/44'/60'/0'/0/0"

hermes keys list --chain lumera-mainnet-1 | grep ' relayer '   # MUST show $NEWADDR
```

### Step 6 — Restart and verify

```bash
# restart the hermes process/container so it loads the ethermint config + new key
hermes keys balance --chain lumera-mainnet-1     # SUCCESS balance for key `relayer`: <amount> ulume
hermes health-check                              # chain lumera-mainnet-1 is healthy
```

Watch the logs for a few packets to confirm it signs and broadcasts cleanly. IBC clients, connections, and channels are **not** owned by the relayer account, so they are unaffected by the key change — Hermes just needs a funded, signable account.

---

## Rollback / abort

- **Before Step 4 (migration):** fully reversible. Restore `address_type` removal in `config.toml`, restore `relayer.json` from backup if you touched it, delete `relayer-evm*` keys. Nothing on-chain changed.
- **After Step 4:** the legacy address is permanently blocked; there is no rollback. The only recovery is forward — ensure Hermes holds the EVM key (Step 5) and is funded.

Keep `/tmp/relayer.json.bak` only until Step 6 verifies; it is the now-blocked legacy key and a stray `.json` left in the `keyring-test/` directory can confuse Hermes' key loader — delete it once relaying is confirmed.

---

## Why this is fiddly (and the others aren't)

For a normal user/validator/supernode migration, `lumerad` (or the supernode daemon) is the *only* signer, so its derivation never has to agree with anything external. The relayer is the one account where a **second tool** (Hermes) must reconstruct the same key from the mnemonic. That makes the HD path a hard requirement, not a convenience:

- Same mnemonic + same path (`m/44'/60'/0'/0/0`) + same algo (`eth_secp256k1`) ⇒ same private key ⇒ same address.
- Hermes' default ethermint path differs, so omitting `--hd-path` silently produces a *different* account — which is why the Step 3 gate exists.

> **Runbook rule:** relayer/operator-key migration needs the key transferred (shared mnemonic with a pinned HD path, or an exported private key), not just "use the same seed." Always gate on a derived-address match before the irreversible migration step.
