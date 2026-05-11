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

- **Path A — Supernode daemon migrates for you (recommended default).** You recover a new EVM key into the supernode keyring, add`evm_key_name` to`config.yml`, and restart. The daemon detects the legacy key, dual-signs with both keys, and broadcasts`MsgClaimLegacyAccount` itself. This is the flow the rest of this guide documents in steps 1–4.
- **Path B — Migrate via Keplr + Portal first, then let the supernode finalize.** You use the Portal's standard[end-user migration](migration.md#method-1-portal--keplr-recommended) (browser + Keplr) to submit the migration transaction yourself. Then on the supernode host, you recover the same EVM key into the supernode's keyring, update`config.yml`, and restart. On startup the daemon sees the on-chain migration record, matches it against your configured`evm_key_name`, skips the broadcast, and performs only local cleanup.

Path B is useful when you want to use Keplr's UX to see each step (the Portal shows balances, delegations, and a pre-migration checklist), when you need to migrate the account's balance urgently for non-supernode reasons, or when your node ops team and your wallet custody team are different people.

> **Terminal alternative for Path B.** If you prefer to stay in a shell rather than use Keplr, you can drive the account-level migration with the bundled shell helper scripts instead of the Portal — the end state (on-chain migration record + matching local key) is identical, and the supernode daemon's `alreadyMigrated` branch activates the same way on restart. See [migration-scripts.md](migration-scripts.md) for the full walkthrough, including multisig rejection, pre-flight estimates, and exit codes. In short:
>
> ```bash
> ./scripts/migrate-account.sh legacy-key new-key \
>   --chain-id lumera-mainnet-1 --node tcp://rpc.lumera:26657
> ```
>
> Then continue with Step B3 (recover the new EVM key into the supernode keyring) onward.

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

If you chose Path B and already submitted the migration via the Portal + Keplr flow, the restart supernode logs look like this instead:

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

## Path B — Migrating via Portal + Keplr first

Use this section if you chose Path B from the ["Two ways to migrate"](#two-ways-to-migrate-pick-one) choice above. Follow the steps in order — don't interleave with Path A steps.

### Before you start

- You need the **same mnemonic** in Keplr (for the Portal) and on the supernode host (for `supernode keys recover`). The deterministic address match between the Portal-submitted migration record and the key you'll import into the supernode keyring depends on this.
- Decide *when* you'll run each step. A safe order is: stop the supernode → migrate in Keplr → recover the EVM key → edit config → restart. Leaving the supernode running between the Portal migration and the final restart is not harmful (the legacy account no longer exists on-chain, so the supernode's outgoing txs will fail fast), but it produces alarming-looking errors in the logs until you restart.

### Step B1 — Stop the supernode

```bash
systemctl stop supernode   # or whatever init system you use
```

Stopping avoids log noise and ensures no inflight txs from the legacy key race with the migration.

### Step B2 — Migrate the account via the Portal (Keplr)

Follow the standard end-user migration flow in [migration.md → Method 1: Portal + Keplr](migration.md#method-1-portal--keplr-recommended). The supernode account behaves like any other Keplr account in this flow — there's nothing supernode-specific to do in the browser.

Quick summary of what you'll do:

1. Open the Lumera Portal's **Claim** page.
2. Connect Keplr with the mnemonic that currently controls the legacy supernode account.
3. The portal auto-detects the legacy account, shows your balance/delegations/supernode status, and offers a "Ready to Migrate" wizard.
4. Click through Review → Sign & Confirm → Submit. Keplr will pop up twice to sign the legacy proof (ADR-036) and the new proof (Ethereum `personal_sign`).
5. The portal confirms the transaction and shows the migration record with `new_address`.

After the Portal shows success, verify the on-chain record on the host (or on the Portal's success screen):

```bash
lumerad query evmigration migration-record <legacy-address>
```

Note the `new_address` — you'll verify that it matches what the supernode derives locally in Step B5.

### Step B3 — Recover the new EVM key into the supernode keyring

Exactly the same operation as Path A's Step 1. **Use the same mnemonic you used in Keplr** — this is the critical piece that makes Path B work:

```bash
supernode keys recover <evm-key-name> --mnemonic "twelve or twenty four mnemonic words ..."
supernode keys list
```

Confirm the printed EVM address matches the `new_address` you saw in the Portal and in the migration record. If they don't match, stop — you're using a different mnemonic than Keplr did, and the supernode will refuse to finalize.

### Step B4 — Add `evm_key_name` to `config.yml`

Exactly the same operation as Path A's Step 2:

```yaml
supernode:
  key_name: supernode-legacy       # existing legacy key (unchanged)
  evm_key_name: supernode-evm      # must match the name you chose in Step B3
  identity: "lumera1...legacyaddr" # existing legacy address — daemon will rewrite on restart
  # ...
```

### Step B5 — Restart the supernode (local cleanup only)

```bash
systemctl start supernode
```

On startup the daemon:

1. Detects the legacy key in the keyring (`is_legacy_key=true`).
2. Queries `MigrationRecord(legacyAddr)` — finds the record you submitted via Keplr.
3. Compares the record's `new_address` to the address derived from your locally-imported `evm_key_name` — they match (same mnemonic, same HD path, same algorithm).
4. Sets `alreadyMigrated=true` and **skips the broadcast step entirely**.
5. Performs only local cleanup: rewrites `config.yml` (`key_name` → evm key name, `identity` → new address, removes `evm_key_name`), deletes the old legacy key from the keyring.

Expected logs — see the [Path B log variant](#path-b-log-variant--already-migrated-via-keplr) callout in Step 3 for the exact sequence. The key line is `INFO  Account already migrated on-chain, skipping broadcast`.

### Step B6 — Verify

Same as Path A's [Step 4 — Verify](#step-4--verify). Three queries — migration record, supernode registration at the new address, and `config.yml` state — all should reflect the new EVM address.

### Path B gotchas

- **Different mnemonic on supernode host**: if the mnemonic you recover with `supernode keys recover` is *not* the one you used in Keplr, the derived bech32 addresses differ, and the daemon logs `migration record exists on-chain but new address mismatch` and exits. Recover with the Keplr mnemonic and retry.
- **Picked the wrong Keplr account**: if Keplr held multiple accounts and you migrated the wrong one, the on-chain migration record points to the wrong legacy address. Check the Portal's success page for the legacy address it migrated from — it must match your supernode's current `identity`.
- **Supernode never stopped**: if the supernode kept running between Step B2 and Step B5, its outbound txs will have been erroring with "account not found" for the duration. This is cosmetic — the final restart clears the state. But stop-first is cleaner.
- **Multisig legacy account**: Path B does not apply to multisig supernode accounts — Keplr can't drive a K-of-N ceremony. See the [Multisig supernode accounts](#multisig-supernode-accounts) section.

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

The new operator account is **also** a K-of-N multisig, constructed from `eth_secp256k1` sub-keys (see the [mirror-source rule](../evmigration/main.md#multisig-account-migration) in `evmigration/main.md`). The ceremony described below produces that new multisig, builds a dual-side proof, and broadcasts it.

> **Consensus invariants (multisig).** The chain rejects a multisig supernode-operator migration tx at `ValidateBasic` if any of these is violated:
>
> - **Shape + K/N mirror.** K-of-N legacy → K-of-N new, same K and same N (`ErrMirrorSourceMismatch`).
> - **Matching `signer_indices`.** The same K signer positions approve both halves.
> - **Sub-key uniqueness.** No duplicate entries in either side's `sub_pub_keys` list.
> - **Zero-signer submit.** `submit-proof` takes no `--from`, no fee flags, no envelope signature.
>
> Full reference: [legacy-migration.md § Consensus invariants](../evmigration/legacy-migration.md#consensus-invariants).

### Why automatic migration is refused

The supernode daemon holds a single signing key and cannot run the K-of-N ceremony required for multisig migration. When it detects `is_multisig=true` from `MigrationEstimate`, it fatals with:

```text
legacy supernode account lumera1... is a 2-of-3 multisig; automatic migration is not supported.

The daemon holds a single key and cannot run the multi-party signing ceremony.
Please complete migration offline using the lumerad CLI, then restart supernode —
the existing on-chain record will trigger local cleanup automatically.

Four-step offline ceremony:

  # 1) Each co-signer generates a fresh eth_secp256k1 sub-key; coordinator
  #    derives the new multisig:
  lumerad keys add <op>-eth-<N> --key-type eth_secp256k1 --keyring-backend <backend>
  lumerad keys add <op>-msig-new --multisig <op>-eth-1,<op>-eth-2,<op>-eth-3 \
    --multisig-threshold K --keyring-backend <backend>

  # 2) Coordinator builds the proof template:
  lumerad tx evmigration generate-proof-payload \
    --legacy <multisig-legacy-address> \
    --new <new-multisig-address> \
    --new-sub-pub-keys <op>-eth-1,<op>-eth-2,<op>-eth-3 \
    --new-threshold K \
    --kind claim --chain-id <chain-id> --out proof.json

  # 3) Each of K co-signers signs both sides in one call:
  lumerad tx evmigration sign-proof proof.json \
    --from <my-legacy-sub-key> --new-key <my-eth-sub-key> \
    --keyring-backend <backend> --chain-id <chain-id> \
    --out <signer>-partial.json

  # 4) Combine and submit (no --from on submit-proof):
  lumerad tx evmigration combine-proof *-partial.json --out tx.json
  lumerad tx evmigration submit-proof tx.json --chain-id <chain-id>
```

### Multisig flow overview

You complete the 4-step offline ceremony with `lumerad`, then restart the supernode — the daemon detects the on-chain migration record and finishes local cleanup through its idempotent path.

#### Step 1 — Generate N fresh `eth_secp256k1` sub-keys and derive the new multisig

Each co-signer generates their own destination-side eth sub-key on their own host (or wherever they hold the legacy sub-key). The coordinator collects the resulting pubkeys and derives the new multisig address:

```bash
# Each co-signer, on their own machine:
lumerad keys add <op-name>-eth-<N> --key-type eth_secp256k1 \
  --keyring-backend <backend>

# Coordinator, once all N eth sub-keys are available:
lumerad keys add <op-name>-msig-new \
  --multisig <op-name>-eth-1,<op-name>-eth-2,<op-name>-eth-3 \
  --multisig-threshold 2 \
  --keyring-backend <backend>

lumerad keys show <op-name>-msig-new --address
# lumera1...   <-- the new multisig bech32; record this as new_address
```

This replaces the old single-EOA "recover the new EVM key" step: the destination is a multisig derived from fresh eth sub-keys, not an EOA recovered from a mnemonic.

Set `evm_key_name` in the supernode's `config.yml` to the name of the new multisig key (`<op-name>-msig-new` in the example above) — the daemon will detect this during the post-migration restart and run cleanup.

#### Step 2 — Ensure the multisig's pubkey is on-chain

If the multisig has received funds but never signed a transaction, its `LegacyAminoPubKey` is nil on-chain and `generate-proof-payload` will fail. Submit any transaction from the multisig first (a 1-`ulume` self-send is sufficient), then confirm:

```bash
lumerad query auth account <multisig-legacy-address>
```

The response must show a `multisig` pubkey structure listing all N legacy sub-keys.

#### Step 3 — Coordinator generates the proof payload template

```bash
lumerad tx evmigration generate-proof-payload \
  --legacy <multisig-legacy-address> \
  --new <new-multisig-address-from-step-1> \
  --new-sub-pub-keys <op-name>-eth-1,<op-name>-eth-2,<op-name>-eth-3 \
  --new-threshold 2 \
  --kind claim \
  --chain-id <chain-id> \
  --keyring-backend <backend> \
  --out proof.json
```

- `--new-sub-pub-keys` accepts either keyring key names or base64 compressed 33-byte `eth_secp256k1` pubkeys. Mix freely.
- `--new-threshold` is **required** whenever `--new-sub-pub-keys` is set.
- `--kind claim` targets `MsgClaimLegacyAccount`; use `--kind validator` if the multisig is also a validator operator.
- `--chain-id` is **required** — it is embedded in the signed payload, so an empty or wrong value makes every sub-signature fail verification on-chain.
- `generate-proof-payload` does not broadcast anything, but it **does** need keyring access (to resolve `--new-sub-pub-keys` / `--legacy-key` entries that are local key names). Pass `--keyring-backend` (and `--keyring-dir` / `--home` when applicable).

Distribute `proof.json` to all co-signers.

#### Step 4 — Each co-signer signs both sides in one invocation

Every participating co-signer must hold **both** their legacy Cosmos sub-key (`--from`) **and** their destination-side eth sub-key (`--new-key`) in the same keyring. `sign-proof` signs both sides and writes a single partial file:

```bash
lumerad tx evmigration sign-proof proof.json \
  --from <my-legacy-sub-key-name> \
  --new-key <my-eth-sub-key-name> \
  --keyring-backend <backend> \
  --chain-id <chain-id> \
  --out <signer>-partial.json
```

`sign-proof` is idempotent on both sides — re-running it replaces this signer's prior entries in both `partial_legacy_signatures` and `partial_new_signatures`, never duplicates. Each co-signer sends their partial file back to the coordinator.

#### Step 5 — Coordinator combines partials

```bash
lumerad tx evmigration combine-proof \
  alice-partial.json bob-partial.json carol-partial.json \
  --out tx.json
```

`combine-proof` rejects the set if any two partials disagree on `chain_id`, `evm_chain_id`, `legacy_address`, `new_address`, `payload_hex`, proof kind, or either side's `sub_pub_keys` list. It verifies every merged partial on both legacy and new sides, drops invalid entries with a stderr warning, then **intersects** the valid signer-index sets across the two sides and selects the first K indices present on BOTH. This is what guarantees `legacy_proof.signer_indices == new_proof.signer_indices`, the consensus-level mirror-source rule. A one-sided partial (e.g. a co-signer who signed only the legacy half because they lost their eth sub-key) does not contribute toward quorum unless another co-signer supplied the new-side signature at the same index. If the intersection has fewer than K entries, combine-proof errors with `need <K> valid partial signatures signed on BOTH sides at matching indices, have <N>` and writes nothing.

#### Step 6 — Coordinator submits the pre-assembled tx

```bash
lumerad tx evmigration submit-proof tx.json \
  --chain-id <chain-id>
```

`submit-proof` broadcasts the pre-assembled tx **without signing at the Cosmos layer**. Migration messages declare zero signers (authorization is fully embedded in `legacy_proof` and `new_proof`), fees are waived by the evmigration ante handler, and replay is prevented by the keeper's `MigrationRecords.Has(legacyAddr)` check. There is no `--from` broadcaster key, no fee-payer, and no envelope signature — `submit-proof` loads `tx.json`, runs `ValidateBasic`, simulates gas via the migration-specific estimator, builds an unsigned tx, and broadcasts.

Verify the migration record:

```bash
lumerad query evmigration migration-record <multisig-legacy-address>
```

#### Step 7 — Restart the supernode (local cleanup only)

The daemon detects the on-chain migration record, confirms its `new_address` matches the multisig bech32 derived from the `evm_key_name` you configured in Step 1, skips the broadcast step (idempotent), rewrites `config.yml` (`key_name` → new multisig key name, `identity` → new multisig bech32, clears `evm_key_name`), and deletes the old legacy multisig composite from the keyring.

Expected logs on the cleanup restart:

```text
INFO  EVM module detected on chain
WARN  Legacy secp256k1 key detected — EVM account migration required
INFO  Account already migrated on-chain, skipping broadcast
INFO  New address confirmed as registered supernode
INFO  EVM migration complete — legacy key removed, config updated
```

### Why the new operator is not EVM-addressable

The new operator account is a Cosmos SDK multisig bech32 derived from `kmultisig.NewLegacyAminoPubKey` over N `eth_secp256k1` sub-keys. It is **not** an Ethereum 20-byte address. This is a non-goal, not a limitation:

- The new operator can perform **all** Cosmos-side operations required for supernode life-cycle: `MsgEditSupernode`, validator edits (if applicable), `x/staking` delegations, `x/distribution` withdrawals, `x/authz` grants, and IBC transfers. Every supernode-relevant workflow continues to work.
- The new operator **cannot** originate `MsgEthereumTx` — multisig bech32 addresses are not valid senders for EVM transactions, and there is no way to produce a single ECDSA signature that authenticates K-of-N.

Operators who want EVM DeFi access for their supernode rewards should configure a separate **single-EOA withdraw address** via:

```bash
lumerad tx distribution set-withdraw-addr <single-eth-eoa> \
  --from <new-multisig-key> \
  --multisig <new-multisig-key> \
  --chain-id <chain-id> \
  # ... plus the usual multisig sign/sign-batch/multi-sign/broadcast steps
```

Rewards then accrue to the single-EOA withdraw address, which **is** EVM-addressable and can originate `MsgEthereumTx` to interact with any EVM contract.

### Post-migration cleanup

The daemon's idempotent cleanup path detects the on-chain multisig `BaseAccount.PubKey` (set by `MigrateAuth`) and treats it as the canonical record of "the operator has migrated". No workflow change is required from the operator beyond the restart in Step 7 — the daemon does not need to "know" that the new operator is a multisig; it simply confirms that the on-chain `new_address` matches the address derived locally from `evm_key_name` and runs cleanup.

### Migration order relative to sub-signer personal migrations

Supernode operators whose operator key is a multisig often ask whether they need to coordinate their personal account migrations with the multisig's migration ceremony. They do not: sub-signer and multisig migrations are mutually independent. See the "Migration order — FAQ" in [evmigration/main.md](../evmigration/main.md#migration-order--faq) for the full explanation; the short version is that any order works, including interleaved, and a sub-signer's personal migration never affects the multisig's ability to migrate later.

### Multisig troubleshooting

**`sub-sig 0 (signer lumera1…) invalid: legacy signature verification failed`** — one of the partial signatures didn't verify under its declared sub-pub-key. Most common causes:

- `--chain-id` differed between `generate-proof-payload` and what the chain uses (the chain-id is embedded in the signed payload).
- A co-signer edited `proof.json` between `generate-proof-payload` and `sign-proof`.
- Wrong legacy sub-key used by a signer (`--from` pointed at a key that isn't one of the legacy multisig members), or wrong destination sub-key (`--new-key` pointed at a key not in `--new-sub-pub-keys`).

Regenerate `proof.json` with the correct `--chain-id`, have the affected signer re-run `sign-proof`, then re-combine.

**`sub-sig N (signer lumera1…) invalid: new signature verification failed`** — symmetric failure on the destination side. Typically the signer used the wrong `--new-key` (not the eth sub-key they claimed during `generate-proof-payload`) or their eth sub-key isn't actually one of the entries in `--new-sub-pub-keys`. Fix the `--new-key` value and re-run `sign-proof` for that signer.

**The multisig account was migrated but the supernode still starts the automatic flow** — check that the on-chain record's `new_address` exactly matches the multisig bech32 of the `evm_key_name` configured in the supernode keyring. If they differ, the daemon won't detect the already-migrated state and will try to broadcast fresh. Align `evm_key_name` with the multisig key that was actually used during the offline ceremony.

**What if I only have K−1 of the sub-keys available on the legacy side?** — you can't complete migration. The K-of-N threshold is enforced by the keeper (`need <K> valid partial signatures, have <N>`). Recover the missing legacy sub-key(s) from their mnemonics, or coordinate with the actual holders.

**What if only K−1 co-signers have provided eth sub-keys for the destination side?** — same situation, symmetric: you need K valid new-side partials. Have the missing co-signer(s) generate their eth sub-key (`lumerad keys add ... --key-type eth_secp256k1`), rebuild `proof.json` via `generate-proof-payload` with the full `--new-sub-pub-keys` list, and re-sign.

**The supernode's embedded error message says `assemble-proof` but the CLI has `combine-proof`. Which is correct?** — the CLI command is `combine-proof`. Any older embedded error message in the supernode binary is stale; use this guide's commands.

---

## Related documentation

- [migration.md](migration.md) — chain-level end-user migration guide (Portal + Keplr, shell scripts, raw CLI)
- [migration-scripts.md](migration-scripts.md) — reference for the bundled `migrate-account.sh` / `migrate-validator.sh` shell helpers (flags, exit codes, troubleshooting)
- [validator-migration.md](validator-migration.md) — validator operator migration guide (maintenance window,`max_validator_delegations` check, consensus key handling)
- [legacy-migration.md](../evmigration/legacy-migration.md) —`x/evmigration` module architecture, proto shapes, keeper logic, and the full reference for the offline proof flow
- [node-evm-config-guide.md](node-evm-config-guide.md) — post-upgrade`app.toml` / RPC configuration for full nodes and validators
