# Validator Operator EVM Migration Guide

**Last updated**: 2026-04-21
**Applies to**: validator operators running a Lumera validator against an EVM-enabled chain (post-EVM upgrade)
**Prerequisite reading**: [migration.md](migration.md) for the chain-level mechanics of legacy → EVM account migration

---

## Overview

When Lumera upgraded to an EVM-compatible chain, every validator's legacy `secp256k1` **operator key** (coin-type 118) stopped matching the chain's new address derivation (`eth_secp256k1`, coin-type 60). This guide walks you through migrating that operator key.

> **The validator consensus key (`priv_validator_key.json`) is not affected by this migration.** It stays on the ed25519 algorithm and uses a separate HD path. Do not touch `priv_validator_key.json`; only the operator key (the one that signs `MsgCreateValidator`, withdraws commission, etc.) needs migration.

Validators **must** use `MsgMigrateValidator` (not `MsgClaimLegacyAccount`). The chain explicitly rejects `claim-legacy-account` for validator operator addresses. `MsgMigrateValidator` is a superset — it re-keys the validator record, every delegation pointing to the validator, distribution state, supernode registration (if any), and action references in a single atomic transaction.

**This guide's main flow covers the common single-sig validator operator key case.** If your validator operator key is a K-of-N multisig (rare), see the [Multisig validator operator keys](#multisig-validator-operator-keys) section at the end.

---

## What gets re-keyed

In addition to everything covered by a regular account migration (balances, authz, feegrants, claims, vesting), `MsgMigrateValidator` atomically handles:

- **Validator record** — operator address updated in both the primary record and power indices.
- **All delegations** — every delegator's active delegation to this validator is re-keyed to the new valoper.
- **Unbonding delegations** — all pending unbonds from this validator.
- **Redelegations** — where the validator is source or destination.
- **Distribution state** — current rewards, accumulated commission, outstanding rewards, historical rewards, slash events.
- **Supernode record** — if the validator runs a supernode on the same account, both the validator address and the supernode's `SupernodeAccount` field are updated. See [If you also run a supernode](#if-you-also-run-a-supernode) below.
- **Action records** — any `x/action` module records referencing this validator.
- **Pending rewards** — all delegator rewards and validator commission are withdrawn before re-keying.

The consensus key, voting power at block height, and validator jailing/slashing status are untouched.

---

## Pre-migration checklist

1. **Plan a maintenance window.** Your validator will miss blocks between stopping the node and restarting it after migration. Target a low-activity window and pre-arrange with delegators if needed.
2. **Verify eligibility.** Run the pre-flight estimate:

   ```bash
   lumerad query evmigration migration-estimate <legacy-validator-address>
   ```

   Check for:
   - `would_succeed: true` — the migration can proceed.
   - `is_validator: true` — the chain recognizes this address as a validator operator.
   - `val_delegation_count` at or below `max_validator_delegations` (default `2000`). If exceeded, governance must raise the limit or delegators must redelegate out before migration.
   - `rejection_reason` empty. Common non-empty values: validator is in `Unbonding` or `Unbonded` state (must be `Bonded`), migration is disabled by param, deadline has passed.

3. **Prepare both keys.** You need the legacy `secp256k1` key (coin-type 118) and a new `eth_secp256k1` key (coin-type 60) derived from the **same mnemonic**. See step 2 below.
4. **Pick a trusted external RPC.** Your own node will be stopped during the migration broadcast, so route the migration tx through a trusted peer.

---

## Step 1 — Check migration parameters

```bash
lumerad query evmigration params
```

```json
{
  "params": {
    "enable_migration": true,
    "max_migrations_per_block": "50",
    "max_validator_delegations": "2000"
  }
}
```

If `enable_migration: false`, migration is disabled chain-wide and you must wait for governance to enable it.

## Step 2 — Import both keys from the same mnemonic

```bash
# Legacy key (coin-type 118 / secp256k1) — the one currently registered on-chain
lumerad keys add val-legacy --recover --coin-type 118 --algo secp256k1 --keyring-backend file

# New EVM key (coin-type 60 / eth_secp256k1) — same mnemonic, different HD path
lumerad keys add val-new --recover --coin-type 60 --algo eth_secp256k1 --keyring-backend file
```

Both are recovered from the same BIP-39 mnemonic. The resulting bech32 addresses differ because the HD paths and address derivation differ — this is expected and is precisely what the migration fixes on-chain.

Verify both are present:

```bash
lumerad keys list --keyring-backend file
```

You should see `val-legacy` with pubkey type `/cosmos.crypto.secp256k1.PubKey` and `val-new` with pubkey type `/cosmos.evm.crypto.v1.ethsecp256k1.PubKey`.

## Step 3 — Run the pre-flight estimate

Before stopping the node, confirm the migration will succeed:

```bash
lumerad query evmigration migration-estimate <legacy-validator-address>
```

```json
{
  "is_validator": true,
  "delegation_count": "1",
  "total_touched": "2",
  "would_succeed": true,
  "val_delegation_count": "1",
  "balance_summary": "1000000ulume",
  "has_supernode": true,
  "is_multisig": false
}
```

`would_succeed: true` with `is_validator: true` and `val_delegation_count <= max_validator_delegations` means you're clear to proceed.

## Step 4 — Stop the validator

```bash
systemctl stop lumerad
```

Stopping before broadcast avoids double-signing risk and prevents the node from producing blocks with the legacy key while migration is in flight.

## Step 5 — Broadcast the validator migration

Route the transaction through a trusted external RPC since your own node is down:

```bash
lumerad tx evmigration migrate-validator val-legacy val-new \
  --keyring-backend file \
  --chain-id <chain-id> \
  --node tcp://<trusted-rpc>:26657
```

> **Shell helper alternative.** The bundled `scripts/migrate-validator.sh` wraps this command (plus the pre-flight estimate in Step 3, the delegation-cap check, post-migration verification against a pre-broadcast balance snapshot, and the post-migration checklist) into a single safer invocation. Use it when you want one command that enforces the guards rather than running each check by hand:
>
> ```bash
> ./scripts/migrate-validator.sh val-legacy val-new \
>   --keyring-backend file \
>   --chain-id <chain-id> \
>   --node tcp://<trusted-rpc>:26657 \
>   --i-have-stopped-the-node
> ```
>
> `--i-have-stopped-the-node` acknowledges the jailing risk non-interactively (required for systemd / CI / non-TTY runs; omitting it makes the script prompt for the literal word `yes`). `--yes` alone does **not** satisfy this check. See [migration-scripts.md](migration-scripts.md) for full flag reference, exit codes, and troubleshooting.

The CLI:

1. Reads both keys from the keyring.
2. Derives both addresses and builds the migration payload `lumera-evm-migration:<chain-id>:<evm-chain-id>:validator:<legacy>:<new>`.
3. Signs the legacy proof with `val-legacy` (secp256k1).
4. Signs the new proof with `val-new` (eth_secp256k1).
5. Simulates gas, asks for confirmation, and broadcasts.

On success you'll see `"code": 0` and the `migrate_validator` event in the response:

```json
{
  "height": "8121",
  "txhash": "A4C1416FF0DF6E93A7A9E9A5116BA433BFD65C2170678B5010CFF1894A75B76C",
  "code": 0,
  "gas_used": "383726"
}
```

## Step 6 — Verify the migration record

```bash
lumerad query evmigration migration-record <legacy-validator-address> \
  --node tcp://<trusted-rpc>:26657
```

```json
{
  "record": {
    "legacy_address": "lumera1...legacy",
    "new_address": "lumera1...new",
    "migration_time": "1775174579",
    "migration_height": "8121"
  }
}
```

Confirm the validator's new operator address under the new valoper prefix:

```bash
lumerad query staking validator <new-valoper-address> --node tcp://<trusted-rpc>:26657
```

## Step 7 — Restart the validator immediately

```bash
systemctl start lumerad
```

> **Warning:** restart promptly after migration. Extended downtime leads to missed blocks and eventual jailing. Use a trusted external RPC for the migration broadcast so you're not blocked on your own node being up.

Verify the validator is signing blocks:

```bash
lumerad query staking validator <new-valoper-address>
# Expect status "BOND_STATUS_BONDED"

# After a few blocks:
lumerad query slashing signing-info <new-consensus-address>
# Confirm missed_blocks_counter isn't growing unboundedly
```

---

## If you also run a supernode

If your validator account and your supernode account are the **same entity** (the common setup), `MsgMigrateValidator` handles the supernode side as a side-effect:

- The supernode's `SupernodeAccount` field is updated to the new address.
- Supernode evidence records and metrics state are migrated.
- Migration history is appended to the supernode record.

After the validator migration and restart, also restart the supernode so it picks up the new key state:

```bash
systemctl restart supernode
```

See [supernode-migration.md](supernode-migration.md) for the supernode daemon's config-update behavior — it detects the on-chain migration record on the next startup and rewrites `config.yml` automatically.

If your validator and supernode are **different entities** (separate addresses), migrate them independently — the supernode uses `MsgClaimLegacyAccount` via its own flow (or the supernode daemon's automatic startup migration).

---

## Verification

After the migration and restart:

```bash
# 1. Migration record exists and maps legacy → new
lumerad query evmigration migration-record <legacy-validator-address>

# 2. New validator is bonded under the new valoper
lumerad query staking validator <new-valoper-address>

# 3. Delegations point at the new valoper (pick any delegator to spot-check)
lumerad query staking delegations <delegator-address>

# 4. Commission and accumulated rewards are intact at the new address
lumerad query distribution commission <new-valoper-address>
lumerad query distribution rewards <delegator-address> <new-valoper-address>

# 5. If running a supernode, confirm record points at the new address
lumerad query supernode get-supernode <new-address>
```

---

## Troubleshooting

### `would_succeed: false`, `rejection_reason: validator is not in bonded status`

Your validator is `Unbonding` or `Unbonded`. Migration only runs on `Bonded` validators to avoid breaking state machines mid-transition. Either wait for the validator to re-bond or complete unbonding first, then migrate as a regular account with `MsgClaimLegacyAccount`.

### `would_succeed: false`, `rejection_reason: validator exceeds max_validator_delegations`

Total of (active delegations + unbonding delegations + redelegations) exceeds the `max_validator_delegations` param. Options:

- Governance proposal to raise `max_validator_delegations`.
- Delegators redelegate out before validator migration, then back in after.

### `post failed: Post "http://localhost:26657": dial tcp [::1]:26657: connect: connection refused`

You're targeting your own node, which is stopped. Pass `--node tcp://<trusted-rpc>:26657` to use an external RPC.

### Validator missing blocks after restart

Expected: a short window of missed blocks between stop and restart. Prolonged misses indicate the new key is not signing. Check:

- `priv_validator_key.json` is unchanged (ed25519 consensus key; migration should not have touched it).
- The restarted `lumerad` is using the same home directory as before.
- `config.toml` `consensus.create_empty_blocks` and peer settings are unchanged.

### `migration record exists on-chain but new address mismatch`

Someone completed migration with a different EVM key. Either use the actual key that signed (recover from the mnemonic that was used), or investigate the on-chain `new_address` — it's authoritative.

---

## Multisig validator operator keys

This section only applies if your validator's **operator key** is a K-of-N multisig. Normal validator operator keys are single-sig; multisig validator operators are rare and require a governance- or infrastructure-level decision to set up.

### Why the single-command path doesn't work

`lumerad tx evmigration migrate-validator` signs with a single `--from` key. A multisig composite can't single-sign, so the command can't drive the migration. Instead, use the four-step offline proof flow with `--kind validator`.

### Flow overview

1. Verify the multisig pubkey is registered on-chain (if never signed a tx, submit a 1-ulume self-send first):

   ```bash
   lumerad query auth account <multisig-valoper-as-acc-address>
   ```

   The response must show a `multisig` pubkey listing all N sub-keys.

2. **Recover the new EVM operator key** from the same mnemonic used to derive the multisig sub-keys' bundle seed (if applicable) or create a fresh key for the new validator operator:

   ```bash
   lumerad keys add val-new --recover --coin-type 60 --algo eth_secp256k1 --keyring-backend file
   ```

3. **Coordinator generates the proof payload** with `--kind validator`:

   ```bash
   lumerad tx evmigration generate-proof-payload \
     --legacy <multisig-legacy-address> \
     --new <new-evm-address> \
     --kind validator \
     --chain-id <chain-id> \
     --out proof.json
   ```

   `--chain-id` is **required** — the signed payload embeds it. `generate-proof-payload` is a query-style command and does **not** accept `--keyring-backend`.

4. **Each of the K sub-signers signs**:

   ```bash
   lumerad tx evmigration sign-proof proof.json \
     --from <my-sub-key-name> \
     --keyring-backend <backend> \
     --chain-id <chain-id> \
     --out my-partial.json
   ```

5. **Stop the validator** before broadcasting:

   ```bash
   systemctl stop lumerad
   ```

6. **Coordinator combines partials and broadcasts**:

   ```bash
   lumerad tx evmigration combine-proof \
     alice-partial.json bob-partial.json \
     --out tx.json

   lumerad tx evmigration submit-proof tx.json \
     --from <new-evm-key-name> \
     --chain-id <chain-id> \
     --keyring-backend <backend> \
     --node tcp://<trusted-rpc>:26657 -y
   ```

7. **Restart the validator** and verify as in steps 6–7 of the single-sig flow.

`combine-proof` verifies each partial under its sub-pub-key, skips invalid entries, and selects the first K valid partials in signer-index order. If fewer than K verify, it errors with `need <K> valid partial signatures, have <N>` and writes nothing.

### Multisig-specific notes

- The multisig **operator** migration re-keys all the same state as single-sig validator migration (delegations, distribution, supernode record, etc.).
- The resulting validator has a **single-sig EVM operator key**, not a multisig. Migration does not preserve the multisig topology on the new side — the `submit-proof` signer becomes the new authoritative operator.
- If you want the new operator to also be a multisig, set up a new K-of-N eth_secp256k1 multisig **after** migration using standard cosmos-sdk `keys add --multisig` plus a `MsgEditValidator` to point at it. This is outside the evmigration flow.
- See [legacy-migration.md](../evmigration/legacy-migration.md) for the wire-format and keeper-side verification logic.

---

## FAQ

**Q: Will delegators need to do anything?**
No. `MsgMigrateValidator` re-keys every delegation, unbonding, and redelegation record pointing at your validator atomically. Delegators see their delegation show up under the new valoper automatically.

**Q: Will my validator be jailed for downtime during migration?**
Short maintenance windows (single-digit minutes) are typically well within the `SignedBlocksWindow` × `MinSignedPerWindow` tolerance on mainnet-class chains. Plan for a normal-day restart and you're fine. Prolonged outages (hours) risk jailing — the migration itself only takes one block; most of the window is your own node restart latency.

**Q: Does my consensus key change?**
No. `priv_validator_key.json` (ed25519) is untouched. Only the operator key (`secp256k1` → `eth_secp256k1`) changes.

**Q: Can I change my validator's moniker / commission / description as part of migration?**
No — `MsgMigrateValidator` is purely a re-keying operation. Use `MsgEditValidator` before or after migration for any description/commission changes.

**Q: My validator is in the active set but my migration estimate still says `would_succeed: false`. Why?**
Check `rejection_reason` in the estimate response. The most common causes are validator status (must be `Bonded`, not `Unbonding`/`Unbonded`), exceeded `max_validator_delegations`, or migration being globally disabled via the `enable_migration` param.

**Q: I also run a supernode on this validator. What order do I migrate in?**
Migrate the validator first; `MsgMigrateValidator` handles the supernode side as a side-effect. Then restart both `lumerad` and `supernode`. See [supernode-migration.md](supernode-migration.md) for the daemon's self-healing on startup.

---

## Related documentation

- [migration.md](migration.md) — chain-level end-user migration guide (Portal + Keplr, shell scripts, raw CLI)
- [migration-scripts.md](migration-scripts.md) — reference for `migrate-validator.sh` and `migrate-account.sh` (flags, exit codes, troubleshooting, non-interactive / CI usage)
- [supernode-migration.md](supernode-migration.md) — supernode operator migration (automatic single-sig path, manual multisig path)
- [legacy-migration.md](../evmigration/legacy-migration.md) — `x/evmigration` module architecture, proto shapes, keeper logic, and the full reference for the offline proof flow
- [node-evm-config-guide.md](node-evm-config-guide.md) — post-upgrade `app.toml` / RPC configuration for full nodes and validators
