# Supernode Operator EVM Migration Guide

**Last updated**: 2026-04-21
**Applies to**: operators running a Lumera supernode against an EVM-enabled chain (post-EVM upgrade)
**Prerequisite reading**: [migration.md](migration.md) for the chain-level mechanics of legacy → EVM account migration

---

## Overview

When Lumera upgraded to an EVM-compatible chain, every supernode's legacy `secp256k1` key (coin-type 118) stopped matching the chain's new address derivation (`eth_secp256k1`, coin-type 60). The supernode daemon ships with automatic migration for the common single-sig case and a guarded-refusal path for multisig accounts that directs you to the offline lumerad CLI flow.

This guide walks you through both paths.

```text
                    ┌──────────────────────────────────────────────┐
                    │   Your supernode's legacy key on-chain       │
                    │   — is it single-sig or multisig?            │
                    └──────────────────────────────────────────────┘
                                      │
                 ┌────────────────────┴────────────────────┐
                 ▼                                         ▼
    ┌──────────────────────────┐              ┌──────────────────────────┐
    │  Single-sig (secp256k1)  │              │  Multisig (K-of-N)       │
    │                          │              │                          │
    │  Path A — automatic:     │              │  Path B — manual:        │
    │  the supernode daemon    │              │  lumerad CLI performs    │
    │  runs migration itself   │              │  the four-step offline   │
    │  on the next restart.    │              │  signing ceremony.       │
    └──────────────────────────┘              └──────────────────────────┘
```

Migration is idempotent end-to-end: if anything fails mid-flight, restart and the daemon resumes from the on-chain migration record.

---

## Prerequisites

Before starting either path, confirm:

- Lumera chain is **EVM-enabled**. The supernode daemon verifies this at boot via `x/upgrade.ModuleVersions(evm)`. If the chain hasn't upgraded yet, the daemon fatals with `connected Lumera chain does not have EVM support`. There is nothing to migrate on a pre-EVM chain — wait for the upgrade.
- You hold the **mnemonic (seed phrase)** for the legacy key. For multisig accounts, you need the mnemonics for at least K of the N sub-signers.
- You have access to the host running the supernode daemon (and, for the multisig path, a host with `lumerad` installed and the sub-signers' keyrings).

If the chain hasn't upgraded yet: stop here. The supernode binary will refuse to boot and will print an actionable error.

---

## Path A — Single-sig supernode (automatic)

Use this when your on-chain supernode operator account is a regular single-sig `secp256k1` account.

### Step 1 — Recover the new EVM key from the same mnemonic

`supernode keys recover` always produces `eth_secp256k1` keys (coin-type 60). Run it with a **new key name** distinct from your legacy one:

```bash
supernode keys recover <evm-key-name> --mnemonic "twelve or twenty four mnemonic words ..."
```

Example:

```bash
supernode keys recover supernode-evm --mnemonic "abandon abandon ... about"
```

The output prints the new EVM address (derived at coin-type 60 from the same mnemonic). Verify:

```bash
supernode keys list
```

You should see both your legacy key and the newly recovered EVM key. Both derive from the same mnemonic; only their HD paths differ.

### Step 2 — Add `evm_key_name` to `config.yml`

Edit `config.yml` (inside your supernode base directory) and add the `evm_key_name` field under `supernode:` alongside the existing `key_name`:

```yaml
supernode:
  key_name: supernode-legacy       # existing legacy key (unchanged)
  evm_key_name: supernode-evm      # new — must match the name you chose in step 1
  identity: "lumera1...legacyaddr" # existing — daemon will rewrite on migration
  # ...
```

Keep `key_name` and `identity` as-is — the daemon rewrites both after migration succeeds.

### Step 3 — Restart the supernode

The daemon detects the legacy key + `evm_key_name` on boot and runs the migration automatically. Watch the logs:

```text
INFO  EVM module detected on chain
WARN  Legacy secp256k1 key detected — EVM account migration required
INFO  Migration estimate  {"would_succeed": true, "is_validator": false, "is_multisig": false, ...}
INFO  Migration tx passed CheckTx, waiting for block confirmation  {"tx_hash": "..."}
INFO  Migration tx confirmed in block
INFO  New address confirmed as registered supernode
INFO  EVM migration complete — legacy key removed, config updated
```

On success, the daemon has:

- Broadcast `MsgClaimLegacyAccount` (or `MsgMigrateValidator` if you're also a validator operator) with both signatures embedded.
- Waited for block inclusion.
- Deleted the old legacy key from the keyring.
- Rewritten `config.yml`: `key_name: supernode-evm`, `identity: lumera1...newEVMaddr`, `evm_key_name` removed.

From here on, the supernode runs on the EVM key with no further intervention.

### Step 4 — Verify

Query the on-chain migration record:

```bash
lumerad query evmigration migration-record <legacy-address>
```

The response should show `new_address` matching your EVM key's address. Also confirm the supernode's on-chain registration points at the new address:

```bash
lumerad query supernode get-supernode <new-address>
```

---

## Path B — Multisig supernode (manual, lumerad CLI)

Use this when your on-chain supernode operator account is a flat K-of-N multisig (`LegacyAminoPubKey`). The supernode daemon **refuses** to auto-migrate multisig accounts with an error like:

```text
legacy supernode account lumera1... is a 2-of-3 multisig; automatic migration is not supported.

The daemon holds a single key and cannot run the multi-party signing ceremony.
Please complete migration offline using the lumerad CLI, then restart supernode —
the existing on-chain record will trigger local cleanup automatically.
```

This is by design: the daemon has one signing key and can't participate in the K-of-N ceremony. You complete migration offline with `lumerad`, then restart the supernode — it detects the on-chain migration record and finishes local cleanup through the idempotent path.

> The command skeleton embedded in the daemon's error message uses placeholder flags (`--legacy-address`, `--new-address`, `assemble-proof`, stdout redirects) that don't match the current `lumerad` CLI. Use the exact commands in the steps below.

### Step 1 — Recover the new EVM key in the supernode keyring

Same as single-sig path:

```bash
supernode keys recover <evm-key-name> --mnemonic "twelve or twenty four mnemonic words ..."
```

Note the new EVM address — you'll pass it to every offline step.

### Step 2 — Precondition: ensure the multisig pubkey is on-chain

If the multisig account has never signed a transaction, its pubkey is nil on-chain and `generate-proof-payload` will fail. Submit any tx from the multisig account first (for example a 1-ulume self-send), then confirm:

```bash
lumerad query auth account <multisig-legacy-address>
```

The response must show a `multisig` pubkey structure listing all N sub-keys.

### Step 3 — Coordinator generates the proof payload template

On any host with `lumerad` installed (the "coordinator"):

```bash
lumerad tx evmigration generate-proof-payload \
  --legacy <multisig-legacy-address> \
  --new <new-evm-address-from-step-1> \
  --kind claim \
  --chain-id <chain-id> \
  --out proof.json
```

- `--kind claim` targets `MsgClaimLegacyAccount`. Use `--kind validator` only if the supernode operator account is also a validator operator (running both roles from the same multisig composite).
- `--chain-id` is **required**. The signed payload embeds the chain id; an empty or wrong chain-id makes every sub-signature fail verification on-chain.
- `generate-proof-payload` is a query-style command and **does not accept `--keyring-backend`** — it reads the multisig pubkey from the chain.

Distribute the resulting `proof.json` to all co-signers.

### Step 4 — Each co-signer signs on their own machine

Each of the K threshold sub-signers imports their sub-key and runs:

```bash
lumerad tx evmigration sign-proof proof.json \
  --from <my-sub-key-name> \
  --keyring-backend <backend> \
  --chain-id <chain-id> \
  --out my-partial.json
```

`sign-proof` is idempotent — re-running it replaces this signer's prior entry with a fresh signature. Each co-signer produces their own `<name>-partial.json`. Send all partial files back to the coordinator.

### Step 5 — Coordinator combines the partials

```bash
lumerad tx evmigration combine-proof \
  alice-partial.json bob-partial.json \
  --out tx.json
```

`combine-proof` rejects the set if any two partials disagree on `chain_id`, `evm_chain_id`, `legacy_address`, `new_address`, `payload_hex`, proof kind, or the `sub_pub_keys` list. It verifies each partial signature against its declared sub-pub-key, skips invalid entries, and selects the first K valid partials in signer-index order. If fewer than K verify, it errors with `need <K> valid partial signatures, have <N>` and writes nothing.

### Step 6 — Broadcast the assembled transaction

The coordinator broadcasts using **the new EVM key** (recovered into the supernode keyring in step 1) as the transaction signer:

```bash
lumerad tx evmigration submit-proof tx.json \
  --from <new-evm-key-name> \
  --chain-id <chain-id> \
  --keyring-backend <backend> -y
```

`submit-proof` signs the `new_signature` field with the EVM key, wraps the message in an unsigned Cosmos tx (no fee), and broadcasts. On success, verify the migration record lands on-chain:

```bash
lumerad query evmigration migration-record <multisig-legacy-address>
```

### Step 7 — Restart the supernode for local cleanup

With the migration record in place on-chain, restart the supernode daemon. It:

1. Detects the on-chain migration record at `<multisig-legacy-address>`.
2. Confirms the record's `new_address` matches the `evm_key_name` in `config.yml`.
3. Skips the broadcast step (idempotent — no duplicate tx).
4. Rewrites `config.yml`: `key_name` → the EVM key name, `identity` → new EVM address, clears `evm_key_name`.
5. Deletes the old multisig composite from the keyring.

Logs to expect:

```text
INFO  EVM module detected on chain
WARN  Legacy secp256k1 key detected — EVM account migration required
INFO  Account already migrated on-chain, skipping broadcast
INFO  New address confirmed as registered supernode
INFO  EVM migration complete — legacy key removed, config updated
```

---

## Verification

After either path completes:

```bash
# 1. Chain view — migration record exists and maps legacy → new
lumerad query evmigration migration-record <legacy-address>

# 2. New account on-chain has the expected eth_secp256k1 pubkey
lumerad query auth account <new-evm-address>

# 3. Supernode registered at the new address
lumerad query supernode get-supernode <new-evm-address>

# 4. Supernode config reflects the switch
grep -E "key_name|identity|evm_key_name" ~/.supernode/config.yml
```

The `config.yml` should show `key_name: <evm-key-name>`, `identity: <new-evm-address>`, and no `evm_key_name` line.

---

## Troubleshooting

### `evm_key_name "<name>" is not an eth_secp256k1 key`

You created or recovered the EVM-named key with the wrong algorithm. Delete it and re-run `supernode keys recover` (which always produces `eth_secp256k1`).

### `simulation failed: rpc error: ... invalid length: tx parse error`

The supernode binary is older than the chain's `x/evmigration` proto schema. Upgrade to a supernode build that includes the `LegacyProof` refactor (single-sig sends `LegacyProof{Single: SingleKeyProof{…}}` instead of the retired flat `legacy_pub_key`/`legacy_signature` fields).

### `connected Lumera chain does not have EVM support`

The chain hasn't run the EVM upgrade yet. This supernode binary is post-EVM-only — run the older pre-EVM binary, or wait for the chain upgrade.

### `migration record exists on-chain but new address mismatch`

Someone completed migration with a different EVM key than the one now in your `evm_key_name` config. Either:

- Use the EVM key that actually signed the original migration (re-recover it with the mnemonic that was used), or
- Investigate whether the on-chain `new_address` is correct — it's the authoritative record.

### `sub-sig 0 (signer lumera1…) invalid: legacy signature verification failed`

On the multisig path, this means one of the partial signatures didn't verify under its declared sub-pub-key. Most common causes:

- `--chain-id` differed between `generate-proof-payload` and what the chain uses (the chain-id is embedded in the signed payload).
- A co-signer edited `proof.json` between `generate-proof-payload` and `sign-proof`. The `payload_hex` field is canonicalized and validated; mismatches should fail earlier, but defensive handling exists.
- Wrong sub-key used by a signer (`--from` pointed at a key that isn't one of the multisig members).

Regenerate `proof.json` with the correct `--chain-id`, have the affected signer re-run `sign-proof`, then re-combine.

### The multisig account was migrated but the supernode still starts the automatic flow

Check that the on-chain record's `new_address` exactly matches the EVM key address you recovered into the supernode keyring. If they differ, the daemon won't detect the already-migrated state and will try to broadcast fresh — which fails because the legacy account no longer exists. Align `evm_key_name` with the EVM key that was actually used during the offline ceremony.

---

## FAQ

**Q: Do I have to migrate on day 1 of the EVM upgrade?**
No. There is no hard deadline unless governance sets one via the `migration_end_time` param. Until you migrate, your supernode binary refuses to run (it's post-EVM-only) — so in practice you migrate when you upgrade the binary.

**Q: Will my supernode lose its ranking / history across the migration?**
No. The migration re-keys the on-chain record: your supernode registration, evidence history, and metrics carry over under the new address. `x/evmigration` transfers all referenced state atomically.

**Q: My supernode runs as both a validator operator and a supernode. Do I migrate twice?**
No. Use `--kind validator` on `generate-proof-payload`. The validator migration also re-keys the supernode record bound to that validator.

**Q: Can I switch between single-sig and multisig as part of this migration?**
No. Migration preserves account topology: a single-sig legacy account migrates to a single-sig EVM account; a multisig legacy account migrates to the same K-of-N layout at the new address. Topology changes (e.g. moving from multisig to single-sig custody) are a separate on-chain operation, not part of evmigration.

**Q: What if I only have K−1 of the sub-keys available?**
You can't complete migration. The K-of-N threshold is enforced by the keeper (`need <K> valid partial signatures, have <N>`). Recover the missing sub-key(s) from their mnemonics, or coordinate with the actual holders.

**Q: The supernode's embedded error message says `assemble-proof` but the CLI has `combine-proof`. Which is correct?**
The CLI command is `combine-proof`. The embedded error message in the supernode binary is stale — use this guide's commands.

---

## Related documentation

- [migration.md](migration.md) — chain-level end-user migration guide (Portal + Keplr, single-sig CLI, multisig CLI walkthrough with more implementation detail)
- [legacy-migration.md](../evmigration/legacy-migration.md) — `x/evmigration` module architecture, proto shapes, keeper logic, and the full reference for the offline proof flow
- [node-evm-config-guide.md](node-evm-config-guide.md) — post-upgrade `app.toml` / RPC configuration for full nodes and validators
