# Supernode Operator EVM Migration Guide

**Last updated**: 2026-04-21
**Applies to**: operators running a Lumera supernode against an EVM-enabled chain (post-EVM upgrade)
**Prerequisite reading**: [migration.md](migration.md) for the chain-level mechanics of legacy → EVM account migration

---

## Overview

When Lumera upgraded to an EVM-compatible chain, every supernode's legacy `secp256k1` key (coin-type 118) stopped matching the chain's new address derivation (`eth_secp256k1`, coin-type 60). The supernode daemon performs the migration automatically on startup once you add a new EVM key to its config.

**This is the common case for virtually every supernode operator.** The rest of this document is the main walkthrough for that case.

> If your supernode's on-chain operator account is a K-of-N multisig (rare, and only possible if you explicitly set one up), the daemon refuses to migrate and directs you to a manual `lumerad` CLI ceremony. See the [Multisig supernode accounts](#multisig-supernode-accounts) section at the end.

Migration is idempotent end-to-end: if anything fails mid-flight, restart the daemon and it resumes from whatever state the chain already has.

---

## Two ways to migrate (pick one)

Both paths land in the same final state (new EVM key registered as supernode, legacy key deleted, `config.yml` updated). The operator steps are identical — what differs is whether the daemon initiates the on-chain migration or just finalizes one you already submitted.

- **Path A — Supernode daemon migrates for you (recommended default).** You recover a new EVM key into the supernode keyring, add `evm_key_name` to `config.yml`, and restart. The daemon detects the legacy key, dual-signs with both keys, and broadcasts `MsgClaimLegacyAccount` itself. This is the flow the rest of this guide documents in steps 1–4.
- **Path B — Migrate via Keplr + Portal first, then let the supernode finalize.** You use the Portal's standard [end-user migration](migration.md#method-1-portal--keplr-recommended) (browser + Keplr) to submit the migration transaction yourself. Then on the supernode host, you recover the same EVM key into the supernode's keyring, update `config.yml`, and restart. On startup the daemon sees the on-chain migration record, matches it against your configured `evm_key_name`, skips the broadcast, and performs only local cleanup.

Path B is useful when you want to use Keplr's UX to see each step (the Portal shows balances, delegations, and a pre-migration checklist), when you need to migrate the account's balance urgently for non-supernode reasons, or when your node ops team and your wallet custody team are different people.

**Why both paths work deterministically**: `supernode keys recover` derives keys at HD path `m/44'/60'/0'/0/0` using `eth_secp256k1`. Keplr uses the identical derivation for Lumera's EVM chain definition. Given the same mnemonic, both produce the exact same bech32 address — so the new address in the on-chain migration record matches what the supernode derives locally, and the `alreadyMigrated` branch activates cleanly.

If you chose Path B, the steps below are the same but in Step 3 the logs will show a *skipped* broadcast (see the **Path B log variant** callout in that section).

---

## Prerequisites

Before starting:

- Lumera chain is**EVM-enabled**. The supernode daemon verifies this at boot via`x/upgrade.ModuleVersions(evm)`. If the chain hasn't upgraded yet the daemon fatals with`connected Lumera chain does not have EVM support` — wait for the chain upgrade.
- You hold the**mnemonic (seed phrase)** for the legacy supernode key.
- You have access to the host running the supernode daemon and can edit`config.yml`.

---

## Step 1 — Recover the new EVM key from the same mnemonic

`supernode keys recover` always produces `eth_secp256k1` keys (coin-type 60). Run it with a **new key name** distinct from your legacy one:

```bash
supernode keys recover <evm-key-name> --mnemonic "twelve or twenty four mnemonic words ..."
```

Example:

```bash
`supernode keys recover supernode-evm --mnemonic "inspire words ... about"
```

The output prints the new EVM address (derived at coin-type 60 from the same mnemonic). Verify:

```bash
supernode keys list
```

You should see both your legacy key and the newly recovered EVM key. Both derive from the same mnemonic; only their HD paths differ.

## Step 2 — Add `evm_key_name` to `config.yml`

Edit `config.yml` (inside your supernode base directory) and add the `evm_key_name` field under `supernode:` alongside the existing `key_name`:

```yaml
supernode:
  key_name: supernode-legacy       # existing legacy key (unchanged)
  evm_key_name: supernode-evm      # new — must match the name you chose in step 1
  identity: "lumera1...legacyaddr" # existing legacy address — daemon will rewrite on migration
  # ...
```

Keep `key_name` and `identity` as-is — the daemon rewrites both after migration succeeds.

## Step 3 — Restart the supernode

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

- Broadcast`MsgClaimLegacyAccount` (or`MsgMigrateValidator` if you're also a validator operator) with both signatures embedded.
- Waited for block inclusion.
- Deleted the old legacy key from the keyring.
- Rewritten`config.yml`:`key_name: supernode-evm`,`identity: lumera1...newEVMaddr`,`evm_key_name` removed.

From here on, the supernode runs on the EVM key with no further intervention.

### Path B log variant — already migrated via Keplr

If you chose Path B and already submitted the migration via the Portal + Keplr flow, the restart logs look like this instead:

```text
INFO  EVM module detected on chain
WARN  Legacy secp256k1 key detected — EVM account migration required
INFO  Account already migrated on-chain, skipping broadcast
INFO  New address confirmed as registered supernode
INFO  EVM migration complete — legacy key removed, config updated
```

The daemon queries `MigrationRecord(legacyAddr)`, sees that the on-chain record's `new_address` matches the address derived from your local `evm_key_name`, sets the internal `alreadyMigrated=true` flag, and skips the broadcast branch. The rest of the cleanup (delete legacy key, rewrite `config.yml`) runs identically to Path A.

If the logs show `migration record exists on-chain but new address mismatch`, the EVM key you recovered into the supernode keyring isn't the one Keplr used during the Portal flow — either use the same mnemonic (the one that signed in the Portal), or investigate whether two different mnemonics got mixed up.

## Step 4 — Verify

Query the on-chain migration record:

```bash
lumerad query evmigration migration-record <legacy-address>
```

The response should show `new_address` matching your EVM key's address. Also confirm the supernode's on-chain registration points at the new address:

```bash
lumerad query supernode get-supernode <new-address>
```

Finally, confirm `config.yml` reflects the switch:

```bash
grep -E "key_name|identity|evm_key_name" ~/.supernode/config.yml
```

You should see `key_name: <evm-key-name>`, `identity: <new-evm-address>`, and no `evm_key_name` line.

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
- Investigate whether the on-chain`new_address` is correct — it's the authoritative record.

---

## FAQ

**Q: Do I have to migrate on day 1 of the EVM upgrade?**
No — unless governance sets a deadline via the `migration_end_time` param. In practice you migrate when you upgrade the binary, since the new binary is EVM-only.

**Q: Will my supernode lose its ranking / history across the migration?**
No. The migration re-keys the on-chain record: your supernode registration, evidence history, and metrics carry over under the new address. `x/evmigration` transfers all referenced state atomically.

**Q: My supernode runs as both a validator operator and a supernode. Do I migrate twice?**
No — a single `MsgMigrateValidator` re-keys both the validator operator record and the supernode record bound to it. See [validator-migration.md](validator-migration.md) for the validator-specific walkthrough (including the maintenance window and the `max_validator_delegations` check); the supernode side happens as a side-effect of that tx.

**Q: Can I roll back if the migration fails mid-flight?**
No rollback is needed — the daemon is idempotent. If the broadcast fails, restart; if the broadcast succeeded but local cleanup failed, restart. Each restart resumes from the current chain state.

---

## Multisig supernode accounts

This section only applies if your on-chain supernode operator account is a flat K-of-N multisig (`LegacyAminoPubKey`). If your supernode was set up normally with a single-sig key, **you don't need this section** — follow steps 1–4 above.

### Why automatic migration is refused

The supernode daemon holds a single signing key and cannot run the K-of-N ceremony required for multisig migration. When it detects `is_multisig=true` from `MigrationEstimate`, it fatals with:

```text
legacy supernode account lumera1... is a 2-of-3 multisig; automatic migration is not supported.

The daemon holds a single key and cannot run the multi-party signing ceremony.
Please complete migration offline using the lumerad CLI, then restart supernode —
the existing on-chain record will trigger local cleanup automatically.
```

> The command skeleton in the daemon's error message uses placeholder flags (`--legacy-address`, `--new-address`, `assemble-proof`, stdout redirects) that don't match the current `lumerad` CLI. Use the exact commands in the steps below, not the ones in the error text.

### Multisig flow overview

You complete the 4-step offline ceremony with `lumerad`, then restart the supernode — the daemon detects the on-chain migration record and finishes local cleanup through its idempotent path.

1. Recover the new EVM key in the supernode keyring (same as step 1 above).
2. Ensure the multisig pubkey is on-chain. If the multisig account has never signed a transaction, its pubkey is nil on-chain and `generate-proof-payload` will fail. Submit any tx from the multisig account first (e.g. a 1-ulume self-send), then confirm:

   ```bash
   lumerad query auth account <multisig-legacy-address>
   ```

   The response must show a `multisig` pubkey structure listing all N sub-keys.
3. Coordinator generates the proof payload template (on any host with `lumerad`):

   ```bash
   lumerad tx evmigration generate-proof-payload \
     --legacy <multisig-legacy-address> \
     --new <new-evm-address-from-step-1> \
     --kind claim \
     --chain-id <chain-id> \
     --out proof.json
   ```

   - `--kind claim` targets`MsgClaimLegacyAccount`. Use`--kind validator` only if the supernode operator account is also a validator operator.
   - `--chain-id` is**required**: the signed payload embeds the chain id; an empty or wrong chain-id makes every sub-signature fail verification on-chain.
   - `generate-proof-payload` is a query-style command and**does not accept `--keyring-backend`**.

   Distribute `proof.json` to all co-signers.
4. Each of the K threshold sub-signers runs:

   ```bash
   lumerad tx evmigration sign-proof proof.json \
     --from <my-sub-key-name> \
     --keyring-backend <backend> \
     --chain-id <chain-id> \
     --out my-partial.json
   ```

   `sign-proof` is idempotent — re-running it replaces this signer's prior entry. Each co-signer produces their own `<name>-partial.json` file. Send all partial files back to the coordinator.
5. Coordinator combines the partials:

   ```bash
   lumerad tx evmigration combine-proof \
     alice-partial.json bob-partial.json \
     --out tx.json
   ```

   `combine-proof` rejects the set if any two partials disagree on `chain_id`, `evm_chain_id`, `legacy_address`, `new_address`, `payload_hex`, proof kind, or the `sub_pub_keys` list. It verifies each partial signature, skips invalid entries, and selects the first K valid partials in signer-index order. If fewer than K verify, it errors with `need <K> valid partial signatures, have <N>` and writes nothing.
6. Coordinator broadcasts using **the new EVM key** (recovered into the supernode keyring) as the transaction signer:

   ```bash
   lumerad tx evmigration submit-proof tx.json \
     --from <new-evm-key-name> \
     --chain-id <chain-id> \
     --keyring-backend <backend> -y
   ```

   Verify the migration record:

   ```bash
   lumerad query evmigration migration-record <multisig-legacy-address>
   ```
7. **Restart the supernode.** The daemon detects the on-chain migration record, confirms its `new_address` matches `evm_key_name` in `config.yml`, skips the broadcast step (idempotent), rewrites `config.yml` (`key_name` → EVM key, `identity` → new EVM address, clears `evm_key_name`), and deletes the old multisig composite from the keyring.

Expected logs on the cleanup restart:

```text
INFO  EVM module detected on chain
WARN  Legacy secp256k1 key detected — EVM account migration required
INFO  Account already migrated on-chain, skipping broadcast
INFO  New address confirmed as registered supernode
INFO  EVM migration complete — legacy key removed, config updated
```

### Multisig troubleshooting

**`sub-sig 0 (signer lumera1…) invalid: legacy signature verification failed`** — one of the partial signatures didn't verify under its declared sub-pub-key. Most common causes:

- `--chain-id` differed between`generate-proof-payload` and what the chain uses (the chain-id is embedded in the signed payload).
- A co-signer edited`proof.json` between`generate-proof-payload` and`sign-proof`.
- Wrong sub-key used by a signer (`--from` pointed at a key that isn't one of the multisig members).

Regenerate `proof.json` with the correct `--chain-id`, have the affected signer re-run `sign-proof`, then re-combine.

**The multisig account was migrated but the supernode still starts the automatic flow** — check that the on-chain record's `new_address` exactly matches the EVM key address you recovered into the supernode keyring. If they differ, the daemon won't detect the already-migrated state and will try to broadcast fresh. Align `evm_key_name` with the EVM key that was actually used during the offline ceremony.

**What if I only have K−1 of the sub-keys available?** — you can't complete migration. The K-of-N threshold is enforced by the keeper (`need <K> valid partial signatures, have <N>`). Recover the missing sub-key(s) from their mnemonics, or coordinate with the actual holders.

**The supernode's embedded error message says `assemble-proof` but the CLI has `combine-proof`. Which is correct?** — the CLI command is `combine-proof`. The embedded error message in the supernode binary is stale; use this guide's commands.

---

## Related documentation

- [migration.md](migration.md) — chain-level end-user migration guide (Portal + Keplr, single-sig CLI)
- [validator-migration.md](validator-migration.md) — validator operator migration guide (maintenance window,`max_validator_delegations` check, consensus key handling)
- [legacy-migration.md](../evmigration/legacy-migration.md) —`x/evmigration` module architecture, proto shapes, keeper logic, and the full reference for the offline proof flow
- [node-evm-config-guide.md](node-evm-config-guide.md) — post-upgrade`app.toml` / RPC configuration for full nodes and validators
