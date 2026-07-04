# Validator Operator EVM Migration Guide

**Last updated**: 2026-06-16
**Applies to**: validator operators running a Lumera validator against an EVM-enabled chain (post-EVM upgrade)
**Prerequisite reading**: [migration.md](migration.md) for the chain-level mechanics of legacy → EVM account migration

---

## Overview

When Lumera upgraded to an EVM-compatible chain, every validator's legacy `secp256k1` **operator key** (coin-type 118) stopped matching the chain's new address derivation (`eth_secp256k1`, coin-type 60). This guide walks you through migrating that operator key.

> **The validator consensus key (`priv_validator_key.json`) is not affected by this migration.** It stays on the ed25519 algorithm and uses a separate HD path. Do not touch `priv_validator_key.json`; only the operator key (the one that signs `MsgCreateValidator`, withdraws commission, etc.) needs migration.

Validators **must** use `MsgMigrateValidator` (not `MsgClaimLegacyAccount`). The chain explicitly rejects `claim-legacy-account` for validator operator addresses. `MsgMigrateValidator` is a superset — it re-keys the validator record, every delegation pointing to the validator, distribution state, supernode registration (if any), and action references in a single atomic transaction.

**This guide's main flow covers the common single-sig validator operator key case.** If your validator operator key is a K-of-N multisig (rare), see the [Multisig validator operator keys](#multisig-validator-operator-keys) section at the end.

> **Note on `systemctl` commands.** This guide uses `systemctl stop/start lumerad` (and `systemctl restart supernode`) as examples. Many validators don't run the node under systemd — Docker/Kubernetes, cosmovisor, runit/s6, or a bare `lumerad start` under a process supervisor are all common. Substitute whatever supervises your node (e.g. `docker stop <container>`, your cosmovisor service unit, or `pkill -f "lumerad start"`). The only invariant that matters: **`lumerad` must not be producing blocks when you broadcast the migration.**

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

1. **Plan a maintenance window.** Your validator will miss blocks between stopping the node and restarting it after migration. Target a low-activity window and pre-arrange with delegators if needed. Mainnet genesis sets `app_state.slashing.params.downtime_jail_duration` to `3600s` (1 hour). Keep the full account migration downtime, from stopping `lumerad` through the post-migration restart and catch-up, comfortably below this limit. If the window approaches 1 hour, restart the node, catch up, and recover/unjail before retrying the migration.
2. **Verify eligibility.** Run the pre-flight estimate:

   ```bash
   lumerad query evmigration migration-estimate <legacy-validator-address>
   ```

   Check for:
   - `would_succeed: true` — the migration can proceed.
   - `is_validator: true` — the chain recognizes this address as a validator operator.
   - `validator_jailed: false` **and** `validator_status` is not `BOND_STATUS_UNBONDING` — migration requires the validator to be un-jailed and not mid-unbonding. Both `BOND_STATUS_BONDED` and `BOND_STATUS_UNBONDED` (with `validator_jailed: false`) are migratable — a validator that fell out of the active set purely on stake weight does **not** need to re-enter it. If the validator is jailed or still `BOND_STATUS_UNBONDING`, see [Step 3a](#step-3a--recovering-from-a-jailed-or-unbonding-validator) before proceeding.
   - `val_delegation_count + val_unbonding_count + val_redelegation_count` at or below `max_validator_delegations` (default `2500`). If exceeded, governance must raise the limit or delegators must redelegate out before migration.
   - `rejection_reason` empty. Common non-empty values: validator is jailed (recoverable via `unjail`), validator is unbonding (wait for the unbonding period to complete, then migrate), migration is disabled by param, deadline has passed.

3. **Prepare both keys.** You need the legacy `secp256k1` key (coin-type 118) and a new `eth_secp256k1` key (coin-type 60) derived from the **same mnemonic**. See step 2 below.
4. **Pick a trusted external RPC.** Your own node will be stopped during the migration broadcast, so route the migration tx through a trusted peer.
5. **Confirm the validator is healthy *now*.** Sample the active validator set (`lumerad query staking validators --output json | jq '.validators[] | select(.operator_address == "<your-valoper>") | {status, jailed, tokens}'`) and confirm `jailed: false` (and that status is not `BOND_STATUS_UNBONDING`) immediately before the maintenance window. A jail event between checklist completion and migration start is the most common preventable cause of a failed migration window — keep the gap short, and re-run pre-flight just before Step 4.

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
    "max_validator_delegations": "2500"
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

> **Legacy key already in the keyring?** If the operator's legacy (coin-type 118) key is already present — which is the common case, since it's the key you've been running the validator with — `keys add val-legacy --recover` fails with **`duplicated address created`**. That's expected: the recovered key derives the *same* bech32 address as the one already stored, and a keyring can't hold the same address under two names. **Skip this `keys add` step and pass the existing key's name as the legacy argument** to the migration command (Step 4) or to `migrate-validator.sh`. You only need to recover the new EVM key (`val-new`).

Both are recovered from the same BIP-39 mnemonic. The resulting bech32 addresses differ because the HD paths and address derivation differ — this is expected and is precisely what the migration fixes on-chain.

> **The new EVM address must be fresh — do not fund or use it before migrating.** The migration moves all balances and state to the destination atomically and **refuses to run if the destination already exists on-chain** (the chain and `migrate-validator.sh` both check this). It's tempting to send "a little gas" to the new address first; don't. If the destination already has account state, derive a different coin-type 60 key, or complete the migration first and fund it afterward. The pre-flight will surface this as a destination-freshness failure (`new address ... already exists on-chain`).

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
  "is_multisig": false,
  "validator_status": "BOND_STATUS_BONDED",
  "validator_jailed": false
}
```

`would_succeed: true` with `is_validator: true`, `validator_jailed: false`, `validator_status` ∈ {`BOND_STATUS_BONDED`, `BOND_STATUS_UNBONDED`}, and `val_delegation_count + val_unbonding_count + val_redelegation_count <= max_validator_delegations` means you're clear to proceed.

> **A terminal rejection may return *only* `rejection_reason`.** When the account can never be migrated as-is — most commonly because it was **already migrated** — the estimate collapses to a single field, e.g. `{ "rejection_reason": "already migrated" }`, with none of the `is_validator` / `would_succeed` / count fields shown above. The query isn't broken; the condition is terminal. For `already migrated`, look up where it went with `lumerad query evmigration migration-record <legacy-address>` and use the new address going forward. See [Troubleshooting](#troubleshooting).

The chain rejects migration in exactly two cases: the validator is **jailed**, or it is still **`BOND_STATUS_UNBONDING`**. A `BOND_STATUS_UNBONDED` validator that is *not* jailed is fully migratable — this is the recovery path for an operator who fell out of the active set on stake weight. The failure shapes you may see:

- `validator_jailed: true` (status is then `BOND_STATUS_UNBONDING` or `BOND_STATUS_UNBONDED`) → the validator was jailed (typically for downtime). Recoverable: see [Step 3a](#step-3a--recovering-from-a-jailed-or-unbonding-validator).
- `validator_status: BOND_STATUS_UNBONDING` with `validator_jailed: false` → the validator is mid-unbonding (voluntarily exiting, or being pushed out of the active set). Wait for the unbonding period to complete — the validator then becomes `BOND_STATUS_UNBONDED` and is directly migratable. See [Step 3a](#step-3a--recovering-from-a-jailed-or-unbonding-validator).

> **Why `BOND_STATUS_UNBONDING` is blocked but `BOND_STATUS_UNBONDED` is not.** An unbonding validator still holds a live entry in the staking module's unbonding-validator queue, keyed by its operator address. Re-keying the operator during migration would orphan that queue entry and halt the chain when it matures. An *unbonded* validator has already been dequeued, so there is nothing to orphan — migration re-keys its record safely. A jailed validator is always `Unbonding` or `Unbonded`, but the reverse isn't true; surfacing both `validator_status` and `validator_jailed` lets you distinguish "needs `unjail`", "wait for unbonding", and "clear to migrate".

## Step 3a — Recovering from a jailed or unbonding validator

Skip this section if Step 3 returned `would_succeed: true`.

If the pre-flight reported `validator_jailed: true`, your validator was kicked out of the active set for a slashable offense (almost always downtime — your node was offline long enough to miss `min_signed_per_window` × `signed_blocks_window` blocks). Migration is gated until you clear the jailed flag with `unjail`. You do **not** need to return to the active set: once `jailed: false`, the validator is migratable whether it ends up `BOND_STATUS_BONDED` (enough stake to rebond) or `BOND_STATUS_UNBONDED` (not enough) — only a jailed or still-`BOND_STATUS_UNBONDING` validator is blocked.

### The timing trap

`unjail` is a **transaction signed by the validator's operator key** (the same key you're trying to migrate). It requires the node to be **running, synced, and able to broadcast**. But migrate-validator requires the node to be **stopped before broadcast** to avoid double-signing risk. So the recovery sequence intentionally restarts the node, runs `unjail`, waits for re-bonding, then stops again before the migration:

```bash
# 1. Make sure the validator is running.
systemctl start lumerad

# 2. Wait for it to catch up to the tip. Repeat until catching_up = false.
lumerad status | jq '.sync_info | {catching_up, latest_block_height}'

# 3. Submit the unjail transaction (signed with the validator's operator key).
lumerad tx slashing unjail \
  --from <validator-key> \
  --chain-id <chain-id> \
  --keyring-backend <backend> \
  --gas auto --gas-adjustment 1.3 --fees <fee>ulume \
  --yes

# 4. Wait one block, then verify status.
VALOPER=$(lumerad debug addr <legacy-validator-address> | awk -F': ' '/^Bech32 Val: /{print $2; exit}')
lumerad query staking validator "$VALOPER" --output json \
  | jq '.validator | {status, jailed, tokens, delegator_shares}'
# Expect: jailed = false. Status BOND_STATUS_BONDED (rebonded) or
# BOND_STATUS_UNBONDED (not enough stake to rebond) — both are migratable.

# 5. Re-run the pre-flight estimate to confirm migration is now unblocked.
lumerad query evmigration migration-estimate <legacy-validator-address>
# Expect: would_succeed = true.

# 6. Stop the node again before broadcasting the migration (Step 4).
systemctl stop lumerad
```

### Common failure modes when unjailing

- **`validator still jailed; cannot be unjailed`** — the slashing window hasn't fully elapsed. Wait ~30 s and retry. (The window is `signed_blocks_window` blocks, which on Lumera is parameterized via `slashing` module params; query `lumerad query slashing params` to see the current value.)
- **`validator missing self-delegation`** — your validator's self-stake fell below `min_self_delegation`. Self-delegate first (`lumerad tx staking delegate <valoper> <amount>`), then retry unjail.
- **`unauthorized: account does not exist`** — the operator key you're signing with isn't the validator's operator. Confirm `lumerad keys show <validator-key> -a` matches the legacy address you're migrating.

### What if the validator is `BOND_STATUS_UNBONDED` with `jailed: false`?

**No recovery needed — this validator is directly migratable.** An unbonded, un-jailed validator (voluntary exit, or pushed out of the top `max_validators` slots on stake weight) has already been removed from the unbonding-validator queue, so migration can safely re-key it without orphaning any queue entry. You do **not** need to re-stake or re-enter the active set — doing so is neither required nor helpful. Skip straight to Step 4 and run the migration.

If instead the validator is still `BOND_STATUS_UNBONDING` (not yet `UNBONDED`), it still holds a live queue entry; wait for the unbonding period to complete so it transitions to `BOND_STATUS_UNBONDED`, then migrate.

## Step 4 — Stop the validator

```bash
systemctl stop lumerad   # or however you supervise the node — see "Note on systemctl commands" in the Overview
```

Stopping before broadcast avoids double-signing risk and prevents the node from producing blocks with the legacy key while migration is in flight.

> **Downtime warning:** mainnet genesis sets `downtime_jail_duration` to `3600s` (1 hour). Do not let the stop-to-restart migration window exceed this time; if the window approaches 1 hour, restart and catch up before retrying the migration.

## Step 4a — Raise the RPC timeout on your broadcast node (required)

`migrate-validator` sizes gas with `--gas auto`, which runs the **full re-keying handler inside a simulate call** before broadcasting. For a validator with thousands of delegations / unbondings / redelegations that simulate takes tens of seconds to ~2 minutes — past CometBFT's default `timeout_broadcast_tx_commit = 10s`, which aborts the call with an `EOF` error and the migration never lands.

Your own validator node is stopped (Step 4) and you broadcast through a **trusted external RPC** (Step 5), so raise the timeout on **that** node — a full node you control, *not* the stopped validator. Edit its `~/.lumera/config/config.toml`:

```toml
[rpc]
timeout_broadcast_tx_commit = "600s"   # default "10s"; set ≥ your expected simulate time
```

Then **restart that node** so the change takes effect (CometBFT reads this only at startup).

> **No reconfigurable RPC?** If you must broadcast through an endpoint you cannot change (e.g. a public provider), skip the simulate instead: pass a high fixed `--gas` computed from the gas formula (`6,000,000 + 1,500,000 × records`) rather than `--gas auto`.

Revert this to `"10s"` and restart once the migration is done — see [Step 7](#step-7--restart-the-validator-immediately).

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
> `--i-have-stopped-the-node` acknowledges the jailing risk non-interactively (required for systemd / CI / non-TTY runs; omitting it makes the script prompt for the literal word `yes`). `--yes` alone does **not** satisfy this check. This gate applies to `--dry-run` too: a dry-run from a non-TTY context (a pipe, CI, `docker exec -i`) aborts with `validator downtime not acknowledged and no TTY available` unless you also pass `--i-have-stopped-the-node` — even though a dry-run never broadcasts. See [migration-scripts.md](migration-scripts.md) for full flag reference, exit codes, and troubleshooting.

Example interactive helper run:

```text
$ ./scripts/migrate-validator.sh validator-legacy validator-evm --node http://172.28.0.11:26657
INFO  chain ID: lumera-devnet-1
INFO  legacy key validator-legacy -> address lumera1k0aj0fp28trnnfsn7u2recq7yfnujk7wqj9j4y
INFO  new EVM key validator-evm -> address lumera1ay0lsu8uw0unswqakvx7ytmdelslkm4vt5nnht
INFO  check OK: no migration record found for legacy address lumera1k0aj0fp28trnnfsn7u2recq7yfnujk7wqj9j4y
INFO  check OK: destination address lumera1ay0lsu8uw0unswqakvx7ytmdelslkm4vt5nnht has no migration record as a legacy address
INFO  check OK: no migration record found by new address lumera1ay0lsu8uw0unswqakvx7ytmdelslkm4vt5nnht
INFO  check OK: destination address lumera1ay0lsu8uw0unswqakvx7ytmdelslkm4vt5nnht does not exist on-chain
Migration preview for legacy account lumera1k0aj0fp28trnnfsn7u2recq7yfnujk7wqj9j4y (coin-type 118, secp256k1):
  Validator:         yes
  Val delegations:   38 (to validator)
  Val unbondings:    8 (to validator)
  Val redelegations: 16 (src or dst)
  Validator status:  Bonded
  Jailed:            no
  Multisig:          no
  Balance:           999740294121ulume
  Delegations:       1
  Unbonding:         none
  Redelegations:     none
  Authz grants:      none
  Feegrants:         none
  Actions:           none
  Supernode:         yes
  Would succeed:     yes
================================================================
WARNING - VALIDATOR MIGRATION
Your validator will miss blocks and may be jailed during
migration. The node MUST be stopped before broadcasting this tx.
================================================================
Type "yes" to confirm the node is stopped: yes
INFO  migrating legacy validator lumera1k0aj0fp28trnnfsn7u2recq7yfnujk7wqj9j4y -> EVM-compatible lumera1ay0lsu8uw0unswqakvx7ytmdelslkm4vt5nnht
```

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
systemctl start lumerad   # or however you supervise the node — see "Note on systemctl commands" in the Overview
```

> **Warning:** restart promptly after migration. Extended downtime leads to missed blocks and eventual jailing. Use a trusted external RPC for the migration broadcast so you're not blocked on your own node being up.

**Revert the RPC timeout.** If you raised `timeout_broadcast_tx_commit` on your broadcast node in [Step 4a](#step-4a--raise-the-rpc-timeout-on-your-broadcast-node-required), set it back to the default `"10s"` in that node's `config.toml` `[rpc]` section and restart it. The elevated value is only needed for the one-time migration simulate.

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

# 5. If running a supernode, confirm record points at the new address.
#    NOTE: get-supernode takes the VALOPER address (lumeravaloper1…), not the
#    account address. Convert with: lumerad keys show <new-key> -a --bech val
lumerad query supernode get-supernode <new-valoper-address>
```

---

## Troubleshooting

### `would_succeed: false`, `rejection_reason: validator is jailed (status: ...)`

Your validator was kicked out of the active set for a slashable offense (almost always downtime). The pre-flight response will also show `validator_jailed: true` and `validator_status` ∈ {`BOND_STATUS_UNBONDING`, `BOND_STATUS_UNBONDED`}. The full recovery flow — restart node → wait for catch-up → `unjail` → confirm `jailed: false` → stop node → retry migration — is documented in [Step 3a](#step-3a--recovering-from-a-jailed-or-unbonding-validator). After `unjail` the validator is migratable whether it rebonds to `BOND_STATUS_BONDED` or stays `BOND_STATUS_UNBONDED`.

The minimum command, assuming the node is up and synced:

```bash
lumerad tx slashing unjail \
  --from <validator-key> \
  --chain-id <chain-id> \
  --gas auto --gas-adjustment 1.3 --fees <fee>ulume --yes
```

If unjail itself fails with `validator still jailed; cannot be unjailed`, the slashing window hasn't fully elapsed. Wait, then retry.

### `would_succeed: false`, `rejection_reason: validator is unbonding; wait for the unbonding period to complete, then migrate`

The pre-flight response shows `validator_jailed: false` and `validator_status: BOND_STATUS_UNBONDING`. The validator is mid-unbonding (voluntary exit, or pushed out of the top `max_validators` slots) and still holds a live unbonding-validator-queue entry keyed by the old operator address. Migrating now would orphan that entry and halt the chain at maturity, so the chain blocks it. **Do nothing but wait:** once the unbonding period elapses the validator transitions to `BOND_STATUS_UNBONDED`, at which point it is directly migratable — no re-staking required. Re-run pre-flight after the transition and proceed. See [Step 3a](#step-3a--recovering-from-a-jailed-or-unbonding-validator) for the longer treatment.

> **A `BOND_STATUS_UNBONDED` (not jailed) validator is *not* rejected.** Only `BOND_STATUS_UNBONDING` and jailed validators are blocked. If your validator fell out of the active set on stake weight and has finished unbonding, migration succeeds directly — you do not need to re-enter the active set.

> **Older versions of this doc / chain referenced `rejection_reason: validator is not in bonded status` or `validator is unbonding or unbonded; wait for completion`, and rejected `BOND_STATUS_UNBONDED` too.** The current chain migrates an unbonded, un-jailed validator directly and only blocks `BOND_STATUS_UNBONDING`. If you see the old text or an unbonded validator being rejected, you're talking to a node running older code.

### `would_succeed: false`, `rejection_reason: validator exceeds max_validator_delegations`

Total of (active delegations + unbonding delegations + redelegations) exceeds the `max_validator_delegations` param. Options:

- Governance proposal to raise `max_validator_delegations`.
- Delegators redelegate out before validator migration, then back in after.

### `rejection_reason: already migrated`

This operator key was already migrated (migration is one-shot). The estimate returns only this single field — no `is_validator`, `would_succeed`, or counts. Find where it went and use the new address from now on:

```bash
lumerad query evmigration migration-record <legacy-validator-address>
# record.new_address is authoritative — your validator now operates under the
# valoper derived from that address.
```

If the recorded `new_address` is **not** the EVM key you expected, stop and investigate which mnemonic produced it before doing anything else (see [`migration record exists on-chain but new address mismatch`](#migration-record-exists-on-chain-but-new-address-mismatch) below). Re-broadcasting a migration for an already-migrated key is rejected by the chain; do not retry.

### `new address ... already exists on-chain`

The destination EVM key you derived in Step 2 is not fresh — it already has account state on-chain, and the migration refuses to overwrite it. Derive a different coin-type 60 / `eth_secp256k1` key (or, if you intentionally funded it, note that you must instead pick an unused address). See the destination-freshness warning under [Step 2](#step-2--import-both-keys-from-the-same-mnemonic).

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

`lumerad tx evmigration migrate-validator` signs with a single `--from` key. A multisig composite can't single-sign, so the command can't drive the migration. Instead, use the four-step offline proof flow with `--kind validator`. The destination **must** also be a K-of-N multisig of `eth_secp256k1` sub-keys — the mirror-source rule (`types.ValidateProofPair`) is a consensus invariant, so migrating a 2-of-3 legacy operator to a single-EOA or 3-of-5 destination is rejected at `ValidateBasic` with `ErrMirrorSourceMismatch` (code 1121).

> **Consensus invariants (multisig validator).** The chain rejects a multisig validator migration tx at `ValidateBasic` if any of these is violated:
>
> - **Shape + K/N mirror.** K-of-N legacy → K-of-N new, same K and same N.
> - **Matching `signer_indices`.** The same K signer positions approve both halves — a co-signer who signs only one side doesn't count on the other.
> - **Sub-key uniqueness.** No duplicate entries in either side's `sub_pub_keys` list.
> - **Zero-signer submit.** `submit-proof` takes no `--from`, no fee flags, no envelope signature.
>
> Full reference with error codes and helper functions: [legacy-migration.md § Consensus invariants](../evmigration/legacy-migration.md#consensus-invariants).

### Flow overview

1. Verify the multisig pubkey is registered on-chain (if never signed a tx, submit a 1-ulume self-send first):

   ```bash
   lumerad query auth account <multisig-valoper-as-acc-address>
   ```

   The response must show a `multisig` pubkey listing all N sub-keys.

2. **Each co-signer generates a fresh `eth_secp256k1` sub-key** in their own keyring:

   ```bash
   lumerad keys add val-eth-<N> --key-type eth_secp256k1 --keyring-backend file
   ```

   The coordinator collects the N eth pubkeys (or local key-names, if sub-signers share a keyring), then derives the destination composite:

   ```bash
   lumerad keys add val-msig-new \
     --multisig val-eth-1,val-eth-2,val-eth-3 \
     --multisig-threshold 2 \
     --nosort \
     --keyring-backend file

   lumerad keys show val-msig-new --address
   # lumera1...  <-- this is the new operator address
   ```

   > **`--nosort` is required, and member order matters.** Without `--nosort`, `keys add` sorts the sub-keys by address, so the composite address here would not match the one `generate-proof-payload` derives from `--new-sub-pub-keys` (which preserves listed order) — and the signer indices would not line up with the legacy side. List the members in the **same order as the legacy multisig's `public_keys`** (see `lumerad query auth account <multisig-legacy-address>`), so each participant occupies the same signer index on both sides. The consensus mirror-source rule requires `legacy_proof.signer_indices == new_proof.signer_indices`.

3. **Coordinator generates the proof payload** with `--kind validator`:

   ```bash
   lumerad tx evmigration generate-proof-payload \
     --legacy <multisig-legacy-address> \
     --new-sub-pub-keys val-eth-1,val-eth-2,val-eth-3 \
     --new-threshold 2 \
     --kind validator \
     --chain-id <chain-id> \
     --keyring-backend file \
     --out proof.json
   ```

   `--chain-id` is **required** — the signed payload embeds it. `generate-proof-payload` needs keyring access to resolve `--new-sub-pub-keys` key names, so pass `--keyring-backend` (and `--keyring-dir` / `--home` when applicable). Mirror-source rule: `--new-threshold` must equal the legacy threshold K and the number of entries in `--new-sub-pub-keys` must equal the legacy N; the CLI rejects a mismatch before writing `proof.json`.

4. **Each co-signer signs both sides** in a single invocation (legacy sub-key + destination eth sub-key):

   ```bash
   lumerad tx evmigration sign-proof proof.json \
     --from    <my-legacy-sub-key> \
     --new-key <my-eth-sub-key> \
     --keyring-backend file \
     --chain-id <chain-id> \
     --out my-partial.json
   ```

   At least one of `--from` / `--new-key` is required; a co-signer who holds only one sub-key passes only that flag. Re-running is idempotent (replaces the signer's prior entries on the corresponding side).

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
     --chain-id <chain-id> \
     --node tcp://<trusted-rpc>:26657 -y
   ```

   `submit-proof` does **not** sign at the Cosmos layer — migration messages declare zero signers, fees are waived by the evmigration ante handler. There is no `--from`.

7. **Restart the validator** and verify as in steps 6–7 of the single-sig flow. Note: the queryable operator address is now the new multisig bech32 (`val-msig-new`), not an EOA.

`combine-proof` verifies each partial under its sub-pub-key on **both sides**, skips invalid entries, then **intersects** the valid signer-index sets across the two sides and selects the first K indices present on BOTH. This is what makes `legacy_proof.signer_indices == new_proof.signer_indices` (the consensus mirror-source rule). A co-signer who signs only one side (e.g. lost access to their eth sub-key) doesn't contribute toward quorum unless another co-signer supplies the other side's signature at the same index. If the intersection has fewer than K entries, combine-proof errors with `need <K> valid partial signatures signed on BOTH sides at matching indices, have <N>` and writes nothing.

### Multisig-specific notes

- The multisig **operator** migration re-keys all the same state as single-sig validator migration (delegations, distribution, supernode record, etc.).
- The new operator is a `LegacyAminoPubKey` multisig of `eth_secp256k1` sub-keys with the **same K and N** as the legacy operator (mirror-source rule, enforced at consensus by `types.ValidateProofPair`). The destination bech32 can perform all Cosmos-side operations (staking, supernode, governance, IBC, authz) but **cannot** originate `MsgEthereumTx` — it's not an EVM-addressable 20-byte address. Operators who want EVM DeFi access for commissions should configure a separate single-EOA withdraw address via `MsgSetWithdrawAddress` after migration.
- If you specifically want to collapse a K-of-N multisig into a single-EOA operator, do the K-of-N → K-of-N migration first, then in a follow-up transaction vote the multisig quorum to execute `MsgSend` + `MsgEditValidator` (re-keying via normal x/staking operations). There is no single-step "multisig → EOA" migration in evmigration.
- See [legacy-migration.md](../evmigration/legacy-migration.md) for the wire-format and keeper-side verification logic.

---

## FAQ

**Q: Will delegators need to do anything?**
No. `MsgMigrateValidator` re-keys every delegation, unbonding, and redelegation record pointing at your validator atomically. Delegators see their delegation show up under the new valoper automatically.

**Q: Will my validator be jailed for downtime during migration?**
Short maintenance windows (single-digit minutes) are typically well within the `SignedBlocksWindow` × `MinSignedPerWindow` tolerance on mainnet-class chains. Mainnet genesis sets `downtime_jail_duration` to `3600s` (1 hour), so do not let the account migration downtime exceed that time. The migration itself only takes one block; most of the window is your own node restart latency.

**Q: Does my consensus key change?**
No. `priv_validator_key.json` (ed25519) is untouched. Only the operator key (`secp256k1` → `eth_secp256k1`) changes.

**Q: Can I change my validator's moniker / commission / description as part of migration?**
No — `MsgMigrateValidator` is purely a re-keying operation. Use `MsgEditValidator` before or after migration for any description/commission changes.

**Q: My validator is in the active set but my migration estimate still says `would_succeed: false`. Why?**
Check `rejection_reason` in the estimate response. The most common causes are the validator being jailed (run `unjail`) or still `Unbonding` (wait for the unbonding period to complete), exceeded `max_validator_delegations`, or migration being globally disabled via the `enable_migration` param. Note that a `BOND_STATUS_UNBONDED` validator that is *not* jailed **is** migratable — being out of the active set on stake weight alone does not block migration.

**Q: I also run a supernode on this validator. What order do I migrate in?**
Migrate the validator first; `MsgMigrateValidator` handles the supernode side as a side-effect. Then restart both `lumerad` and `supernode`. See [supernode-migration.md](supernode-migration.md) for the daemon's self-healing on startup.

---

## Related documentation

- [migration.md](migration.md) — chain-level end-user migration guide (Portal + Keplr, shell scripts, raw CLI)
- [migration-scripts.md](migration-scripts.md) — reference for `migrate-validator.sh` and `migrate-account.sh` (flags, exit codes, troubleshooting, non-interactive / CI usage)
- [supernode-migration.md](supernode-migration.md) — supernode operator migration (automatic single-sig path, manual multisig path)
- [legacy-migration.md](../evmigration/legacy-migration.md) — `x/evmigration` module architecture, proto shapes, keeper logic, and the full reference for the offline proof flow
- [node-evm-config-guide.md](node-evm-config-guide.md) — post-upgrade `app.toml` / RPC configuration for full nodes and validators
