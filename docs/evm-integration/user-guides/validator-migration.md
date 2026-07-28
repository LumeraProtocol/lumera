# Validator Operator EVM Migration Guide

**Last updated**: 2026-06-16
**Applies to**: validator operators running a Lumera validator against an EVM-enabled chain (post-EVM upgrade)
**Prerequisite reading**: [migration.md](migration.md) for the chain-level mechanics of legacy → EVM account migration

> **Mandatory operations gate:** Run the [EVM Migration Operator Runbook](operator-migration-runbook.md) before this validator procedure. It requires supervisor/user/base-dir/config/keyring discovery, a mode-0600 config backup, verified destination identity, proof that every instance using the consensus key is stopped, one broadcast, and public-address/gRPC/log verification.
>
> **Command context used below:** before any CLI example, populate these values from the approved manifest and runbook discovery. `LUMERAD` must be the exact absolute `artifacts.chain_executable.release_path` whose version/tag/commit/hash/source were verified. Each command below passes only flags supported by that command family: tx/query use the explicit home, keyring, chain, and node context; keys use home and keyring context; status uses only the trusted node; and debug address conversion takes no context flags. Do not replace these commands with bare `lumerad` or a universal wrapper.
>
> ```bash
> LUMERAD=/absolute/path/from/approved/artifacts.chain_executable.release_path
> LUMERA_HOME=/absolute/discovered/lumera/home
> KEYRING_BACKEND=<test|file|os>
> KEYRING_DIR=/absolute/discovered/keyring/location
> CHAIN_ID=<approved-chain-id>
> TRUSTED_RPC=<trusted-rpc>
> ```

---

## Overview

When Lumera upgraded to an EVM-compatible chain, every validator's legacy `secp256k1` **operator key** (coin-type 118) stopped matching the chain's new address derivation (`eth_secp256k1`, coin-type 60). This guide walks you through migrating that operator key.

> **The validator consensus key (`priv_validator_key.json`) is not affected by this migration.** It stays on the ed25519 algorithm and uses a separate HD path. Do not touch `priv_validator_key.json`; only the operator key (the one that signs `MsgCreateValidator`, withdraws commission, etc.) needs migration.

Validators **must** use `MsgMigrateValidator` (not `MsgClaimLegacyAccount`). The chain explicitly rejects `claim-legacy-account` for validator operator addresses. `MsgMigrateValidator` is a superset — it re-keys the validator record, every delegation pointing to the validator, distribution state, supernode registration (if any), and action references in a single atomic transaction.

**This guide's main flow covers the common single-sig validator operator key case.** If your validator operator key is a K-of-N multisig (rare), see the [Multisig validator operator keys](#multisig-validator-operator-keys) section at the end.

> **Supervisor stop is mandatory.** Discover the actual supervisor and service/workload identity, then stop and restart only through that supervisor (systemd, Docker, Kubernetes, cosmovisor, runit/s6, or equivalent). Never use an ad hoc process-signal shortcut: a restart policy can immediately respawn the process. Before broadcasting, prove that the supervisor is stopped and that no process, container, pod, or duplicate instance capable of accessing this validator's `priv_validator_key.json` remains. Follow the canonical [Operator Runbook §5](operator-migration-runbook.md#5-stop-and-prove-stopped); if the supervisor cannot be identified or stopped, do not migrate.

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
   sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migration-estimate <legacy-validator-address>
   ```

   Check for:
   - `would_succeed: true` — the migration can proceed.
   - `is_validator: true` — the chain recognizes this address as a validator operator.
   - `validator_jailed: false` **and** `validator_status` is not `BOND_STATUS_UNBONDING` — migration requires the validator to be un-jailed and not mid-unbonding. Both `BOND_STATUS_BONDED` and `BOND_STATUS_UNBONDED` (with `validator_jailed: false`) are migratable — a validator that fell out of the active set purely on stake weight does **not** need to re-enter it. If the validator is jailed or still `BOND_STATUS_UNBONDING`, see [Step 3a](#step-3a--recovering-from-a-jailed-or-unbonding-validator) before proceeding.
   - `val_delegation_count + val_unbonding_count + val_redelegation_count` at or below `max_validator_delegations` (default `2500`). If exceeded, governance must raise the limit or delegators must redelegate out before migration.
   - `rejection_reason` empty. Common non-empty values: validator is jailed (recoverable via `unjail`), validator is unbonding (wait for the unbonding period to complete, then migrate), migration is disabled by param, deadline has passed.

3. **Prepare both keys.** Keep the legacy `secp256k1` key (coin type 118) and prepare an `eth_secp256k1` destination key (coin type 60). It may use the same mnemonic or a different mnemonic. See step 2 below.
4. **Pick a trusted external RPC.** Your own node will be stopped during the migration broadcast, so route the migration tx through a trusted peer.
5. **Confirm the validator is healthy *now*.** Sample the active validator set (`sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" staking validators --output json | jq '.validators[] | select(.operator_address == "<your-valoper>") | {status, jailed, tokens}'`) and confirm `jailed: false` (and that status is not `BOND_STATUS_UNBONDING`) immediately before the maintenance window. A jail event between checklist completion and migration start is the most common preventable cause of a failed migration window — keep the gap short, and re-run pre-flight just before Step 4.

---

## Step 1 — Check migration parameters

```bash
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration params
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

## Step 2 — Pre-stage the EVM destination key

Use the compatibility manifest's verified, named, and hashed PR-2 no-echo destination-prestage implementation to place `val-new` in the exact validator-operator keyring discovered by the operator runbook. It must create/recover coin type 60 / `eth_secp256k1` under a new key name, accept the mnemonic only through hidden TTY or protected input descriptor, and never emit it or place it in argv. If the PR-2 dependency is blocked, stop.

The legacy key is normally already present. Do not recover it again under a second name: a keyring rejects the duplicate address. Pass the existing legacy key name to the manifest-pinned helper.

The destination requirements are coin type `60`, key type `eth_secp256k1`, a fresh on-chain address, and proven custody. **Do not fund or use it before migrating.** The chain and helper reject a destination with existing account state.

Verify both public key records using the exact approved binary/home/backend/location:

```bash
sudo -u <validator-service-user> "$LUMERAD" keys --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" list
```

You should see the existing legacy key with pubkey type `/cosmos.crypto.secp256k1.PubKey` and `val-new` with pubkey type `/cosmos.evm.crypto.v1.ethsecp256k1.PubKey`.

## Step 3 — Run the pre-flight estimate

Before stopping the node, confirm the migration will succeed:

```bash
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migration-estimate <legacy-validator-address>
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

> **A terminal rejection may return *only* `rejection_reason`.** When the account can never be migrated as-is — most commonly because it was **already migrated** — the estimate collapses to a single field, e.g. `{ "rejection_reason": "already migrated" }`, with none of the `is_validator` / `would_succeed` / count fields shown above. The query isn't broken; the condition is terminal. For `already migrated`, look up where it went with `sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migration-record <legacy-address>` and use the new address going forward. See [Troubleshooting](#troubleshooting).

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
# 1. Start through the supervisor identity discovered by the operator runbook.
<discovered-supervisor-command> start <validator-service-or-workload>

# 2. Wait for it to catch up to the tip. Repeat until catching_up = false.
sudo -u <validator-service-user> "$LUMERAD" status --node "$TRUSTED_RPC" | jq '.sync_info | {catching_up, latest_block_height}'

# 3. Submit the unjail transaction (signed with the validator's operator key).
sudo -u <validator-service-user> "$LUMERAD" tx --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" slashing unjail \
  --from <validator-key> \
  --gas auto --gas-adjustment 1.3 --fees <fee>ulume \
  --yes

# 4. Wait one block, then verify status.
VALOPER=$(sudo -u <validator-service-user> "$LUMERAD" debug addr <legacy-validator-address> | awk -F': ' '/^Bech32 Val: /{print $2; exit}')
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" staking validator "$VALOPER" --output json \
  | jq '.validator | {status, jailed, tokens, delegator_shares}'
# Expect: jailed = false. Status BOND_STATUS_BONDED (rebonded) or
# BOND_STATUS_UNBONDED (not enough stake to rebond) — both are migratable.

# 5. Re-run the pre-flight estimate to confirm migration is now unblocked.
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migration-estimate <legacy-validator-address>
# Expect: would_succeed = true.

# 6. Stop through that same discovered supervisor, then repeat its stopped-state
# and consensus-key process/container/pod proofs before broadcasting (Step 4).
<discovered-supervisor-command> stop <validator-service-or-workload>
```

### Common failure modes when unjailing

- **`validator still jailed; cannot be unjailed`** — the slashing window hasn't fully elapsed. Wait ~30 s and retry. (The window is `signed_blocks_window` blocks, which on Lumera is parameterized via `slashing` module params; query `sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" slashing params` to see the current value.)
- **`validator missing self-delegation`** — your validator's self-stake fell below `min_self_delegation`. Self-delegate first (`sudo -u <validator-service-user> "$LUMERAD" tx --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" staking delegate <valoper> <amount>`), then retry unjail.
- **`unauthorized: account does not exist`** — the operator key you're signing with isn't the validator's operator. Confirm `sudo -u <validator-service-user> "$LUMERAD" keys --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" show <validator-key> -a` matches the legacy address you're migrating.

### What if the validator is `BOND_STATUS_UNBONDED` with `jailed: false`?

**No recovery needed — this validator is directly migratable.** An unbonded, un-jailed validator (voluntary exit, or pushed out of the top `max_validators` slots on stake weight) has already been removed from the unbonding-validator queue, so migration can safely re-key it without orphaning any queue entry. You do **not** need to re-stake or re-enter the active set — doing so is neither required nor helpful. Skip straight to Step 4 and run the migration.

If instead the validator is still `BOND_STATUS_UNBONDING` (not yet `UNBONDED`), it still holds a live queue entry; wait for the unbonding period to complete so it transitions to `BOND_STATUS_UNBONDED`, then migrate.

## Step 4 — Stop the validator

Use the operator runbook's exact systemd, Docker, or Kubernetes branch to stop **and prove stopped** every instance that can access the consensus key. Record the evidence and keep container orchestrator reconciliation paused. A bare `systemctl stop` is not a valid substitute for containerized deployments.

**Stopping the node before you broadcast is mandatory, not a formality.** Migration re-keys a live, bonded validator in a single transaction: it rewrites the **operator (staking) key** and the on-chain consensus-address→operator mapping. The **consensus key itself is unchanged** — and that is precisely what makes a running node dangerous here.

Because the consensus key is shared and unchanged across the migration, if the old node instance is still running while you bring up the node under the migrated config, two processes can briefly both hold that same consensus key and sign the same block height. That is **equivocation**, and the chain punishes it with a **tombstone**: the validator is permanently removed from ever validating again, plus a slash of bonded stake. Unlike downtime jailing, a tombstone is **irreversible** — you cannot `unjail` out of it; you must build a brand-new validator with a new consensus key.

This is not a theoretical two-instance mistake. It is a very real risk whenever `lumerad` runs under a process supervisor — systemd (`Restart=always`), Docker (`--restart`), Kubernetes, or cosmovisor: **the supervisor may already have respawned an instance while your manual restart brings up a second one.** A hurried "migrate while it's still running, then restart quickly" is the exact pattern that produces this overlap.

Stopping cleanly first makes the overlap structurally impossible. And note there is no consensus reason to rush the restart at all: the consensus key is untouched, so block signing does not depend on the migration. The post-migration restart ([Step 7](#step-7--restart-the-validator-immediately)) only reloads the new **operator** key for future operator transactions — so do a **deliberate stop-then-start**, never a race.

The trade is deliberately asymmetric: keeping the node up risks an **unrecoverable** tombstone; stopping it costs only **recoverable** missed blocks (and at worst a downtime jail you can `unjail`). When one side of the gamble is permanent, you don't gamble — you stop the node.

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

Do not use a shortened raw-CLI or helper command from this account-specific guide. Return to [Operator Runbook §6](operator-migration-runbook.md#6-re-run-dry-run-verify-destination-broadcast-once) and execute its canonical manifest-pinned helper command for the discovered systemd, Docker, or Kubernetes environment. The post-stop dry-run and one live invocation must use the manifest-pinned absolute helper and `lumerad` paths, exact service identity, explicit home, keyring backend and location, chain ID, trusted node, and stopped-node acknowledgement. The live invocation must be identical to the approved dry-run except for removal of `--dry-run`.

The helper reads both keys from the selected keyring, derives both addresses, signs the legacy and destination proofs, simulates gas, and broadcasts exactly once. Preserve its transaction hash and public-address output, then apply the runbook's query-before-retry boundary.

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
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migration-record <legacy-validator-address>
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
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" staking validator <new-valoper-address>
```

## Step 7 — Restart the validator immediately

Restart only through the exact supervisor and workload identity discovered in the operator runbook, then prove that exactly one supervised instance is running. Use the runbook's concrete systemd, Docker, or Kubernetes restart-and-verification branch; do not substitute an ad hoc process launch.

> **Warning:** restart promptly after migration. Extended downtime leads to missed blocks and eventual jailing. Use a trusted external RPC for the migration broadcast so you're not blocked on your own node being up.

**Revert the RPC timeout.** If you raised `timeout_broadcast_tx_commit` on your broadcast node in [Step 4a](#step-4a--raise-the-rpc-timeout-on-your-broadcast-node-required), set it back to the default `"10s"` in that node's `config.toml` `[rpc]` section and restart it. The elevated value is only needed for the one-time migration simulate.

Verify the validator is signing blocks:

```bash
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" staking validator <new-valoper-address>
# Expect status "BOND_STATUS_BONDED"

# After a few blocks:
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" slashing signing-info <new-consensus-address>
# Confirm missed_blocks_counter isn't growing unboundedly
```

---

## If you also run a supernode

If your validator account and your supernode account are the **same entity** (the common setup), `MsgMigrateValidator` handles the supernode side as a side-effect:

- The supernode's `SupernodeAccount` field is updated to the new address.
- Supernode evidence records and metrics state are migrated.
- Migration history is appended to the supernode record.

After the validator migration and restart, also restart the supernode so it picks up the new key state:

Restart the SuperNode through its independently discovered supervisor/workload identity and prove exactly one supervised instance is running, following the operator runbook's matching systemd, Docker, or Kubernetes branch.

See [supernode-migration.md](supernode-migration.md) for the supernode daemon's config-update behavior — it detects the on-chain migration record on the next startup and rewrites `config.yml` automatically.

If your validator and supernode are **different entities** (separate addresses), migrate them independently — the supernode uses `MsgClaimLegacyAccount` via its own flow (or the supernode daemon's automatic startup migration).

---

## Verification

After the migration and restart:

```bash
# 1. Migration record exists and maps legacy → new
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migration-record <legacy-validator-address>

# 2. New validator is bonded under the new valoper
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" staking validator <new-valoper-address>

# 3. Delegations point at the new valoper (pick any delegator to spot-check)
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" staking delegations <delegator-address>

# 4. Commission and accumulated rewards are intact at the new address
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" distribution commission <new-valoper-address>
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" distribution rewards <delegator-address> <new-valoper-address>

# 5. If running a supernode, confirm record points at the new address.
#    NOTE: get-supernode takes the VALOPER address (lumeravaloper1…), not the
#    account address. Convert with: sudo -u <validator-service-user> "$LUMERAD" keys --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" show <new-key> -a --bech val
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" supernode get-supernode <new-valoper-address>
```

---

## Troubleshooting

### `would_succeed: false`, `rejection_reason: validator is jailed (status: ...)`

Your validator was kicked out of the active set for a slashable offense (almost always downtime). The pre-flight response will also show `validator_jailed: true` and `validator_status` ∈ {`BOND_STATUS_UNBONDING`, `BOND_STATUS_UNBONDED`}. The full recovery flow — restart node → wait for catch-up → `unjail` → confirm `jailed: false` → stop node → retry migration — is documented in [Step 3a](#step-3a--recovering-from-a-jailed-or-unbonding-validator). After `unjail` the validator is migratable whether it rebonds to `BOND_STATUS_BONDED` or stays `BOND_STATUS_UNBONDED`.

The minimum command, assuming the node is up and synced:

```bash
sudo -u <validator-service-user> "$LUMERAD" tx --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" slashing unjail \
  --from <validator-key> \
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
sudo -u <validator-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migration-record <legacy-validator-address>
# record.new_address is authoritative — your validator now operates under the
# valoper derived from that address.
```

If the recorded `new_address` is **not** the EVM key you expected, stop and investigate which mnemonic produced it before doing anything else (see [`migration record exists on-chain but new address mismatch`](#migration-record-exists-on-chain-but-new-address-mismatch) below). Re-broadcasting a migration for an already-migrated key is rejected by the chain; do not retry.

### `new address ... already exists on-chain`

The destination EVM key you derived in Step 2 is not fresh — it already has account state on-chain, and the migration refuses to overwrite it. Derive a different coin-type 60 / `eth_secp256k1` key (or, if you intentionally funded it, note that you must instead pick an unused address). See the destination-freshness warning under [Step 2](#step-2--pre-stage-the-evm-destination-key).

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

`sudo -u <validator-service-user> "$LUMERAD" tx --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migrate-validator` signs with a single `--from` key. A multisig composite can't single-sign, so the command can't drive the migration. Instead, use the four-step offline proof flow with `--kind validator`. The destination **must** also be a K-of-N multisig of `eth_secp256k1` sub-keys — the mirror-source rule (`types.ValidateProofPair`) is a consensus invariant, so migrating a 2-of-3 legacy operator to a single-EOA or 3-of-5 destination is rejected at `ValidateBasic` with `ErrMirrorSourceMismatch` (code 1121).

> **Consensus invariants (multisig validator).** The chain rejects a multisig validator migration tx at `ValidateBasic` if any of these is violated:
>
> - **Shape + K/N mirror.** K-of-N legacy → K-of-N new, same K and same N.
> - **Matching `signer_indices`.** The same K signer positions approve both halves — a co-signer who signs only one side doesn't count on the other.
> - **Sub-key uniqueness.** No duplicate entries in either side's `sub_pub_keys` list.
> - **Zero-signer submit.** `submit-proof` takes no `--from`, no fee flags, no envelope signature.
>
> Full reference with error codes and helper functions: [legacy-migration.md § Consensus invariants](../evmigration/legacy-migration.md#consensus-invariants).

### Flow overview

The multisig ceremony remains a seven-stage process: prove the legacy multisig pubkey is registered; pre-stage N fresh destination sub-keys; derive the same-K/same-N destination composite without sorting; generate the validator proof payload; collect matching-index signatures on both sides; stop and prove the validator stopped; combine, submit once, restart, and verify.

Do not execute a shortened `lumerad` command copied from this guide. Each participant and the coordinator must return to the [Operator Runbook](operator-migration-runbook.md), verify the manifest-pinned absolute `artifacts.chain_executable.release_path`, and invoke it with explicit `--home`, `--keyring-backend`, `--keyring-dir`, `--chain-id`, and `--node` as applicable to that stage. The final submit must occur only after the runbook's stopped-node proof and must use the same trusted node and chain ID as the approved ceremony. Treat every proof file and public key list as custody-sensitive evidence and apply the runbook's query-before-retry rule after submission.

The command semantics and proof-file fields are documented in [legacy-migration.md](../evmigration/legacy-migration.md), but its abbreviated examples are not an execution substitute for the manifest-pinned runbook context. In particular, preserve legacy member order with `--nosort`; use `--kind validator`; keep K, N, and signer indices identical across both proof halves; and never add `--from`, fee, or envelope-signature flags to `submit-proof`.

After successful submission, restart through the discovered supervisor and verify as in steps 6–7 of the single-sig flow. The queryable operator address is now the destination multisig bech32, not an EOA.

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
