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

1. **Plan a maintenance window.** Your validator will miss blocks between stopping the node and restarting it after migration. Target a low-activity window and pre-arrange with delegators if needed. Mainnet genesis sets `app_state.slashing.params.downtime_jail_duration` to `3600s` (1 hour). Keep the full account migration downtime, from stopping `lumerad` through the post-migration restart and catch-up, comfortably below this limit. If the window approaches 1 hour, restart the node, catch up, and recover/unjail before retrying the migration.
2. **Verify eligibility.** Run the pre-flight estimate:

   ```bash
   lumerad query evmigration migration-estimate <legacy-validator-address>
   ```

   Check for:
   - `would_succeed: true` — the migration can proceed.
   - `is_validator: true` — the chain recognizes this address as a validator operator.
   - `validator_status: "BOND_STATUS_BONDED"` and `validator_jailed: false` — the validator is in the active set and not jailed. If either fails, see [Step 3a](#step-3a--recovering-from-a-jailed-or-non-bonded-validator) before proceeding.
   - `val_delegation_count + val_unbonding_count + val_redelegation_count` at or below `max_validator_delegations` (default `2000`). If exceeded, governance must raise the limit or delegators must redelegate out before migration.
   - `rejection_reason` empty. Common non-empty values: validator is jailed (recoverable via `unjail`), validator is voluntarily unbonded (recoverable by re-staking), migration is disabled by param, deadline has passed.

3. **Prepare both keys.** You need the legacy `secp256k1` key (coin-type 118) and a new `eth_secp256k1` key (coin-type 60) derived from the **same mnemonic**. See step 2 below.
4. **Pick a trusted external RPC.** Your own node will be stopped during the migration broadcast, so route the migration tx through a trusted peer.
5. **Confirm the validator is healthy *now*.** Sample the active validator set (`lumerad query staking validators --output json | jq '.validators[] | select(.operator_address == "<your-valoper>") | {status, jailed, tokens}'`) and confirm `BOND_STATUS_BONDED` + `jailed: false` immediately before the maintenance window. A jail event between checklist completion and migration start is the most common preventable cause of a failed migration window — keep the gap short, and re-run pre-flight just before Step 4.

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
  "is_multisig": false,
  "validator_status": "BOND_STATUS_BONDED",
  "validator_jailed": false
}
```

`would_succeed: true` with `is_validator: true`, `validator_status: BOND_STATUS_BONDED`, `validator_jailed: false`, and `val_delegation_count + val_unbonding_count + val_redelegation_count <= max_validator_delegations` means you're clear to proceed.

The chain checks the validator's bond status and jailed flag and rejects migration when either disqualifies the validator. Two failure shapes you may see:

- `validator_status` is `BOND_STATUS_UNBONDING` or `BOND_STATUS_UNBONDED` with `validator_jailed: true` → the validator was jailed (typically for downtime). Recoverable: see [Step 3a](#step-3a--recovering-from-a-jailed-or-non-bonded-validator).
- `validator_status` is `BOND_STATUS_UNBONDING` or `BOND_STATUS_UNBONDED` with `validator_jailed: false` → the validator voluntarily exited the active set (self-unbonded). Less common; recoverable by re-delegating self-stake until back in the active set, then re-running pre-flight.

> **Why both fields exist.** A jailed validator is *always* `Unbonding` or `Unbonded` (jailing transitions out of the active set). But the reverse isn't true — voluntary unbonding doesn't set the jailed flag. Surfacing both lets you distinguish "needs `unjail`" from "needs `delegate`".

## Step 3a — Recovering from a jailed or non-bonded validator

Skip this section if Step 3 returned `would_succeed: true`.

If the pre-flight reported `validator_jailed: true`, your validator was kicked out of the active set for a slashable offense (almost always downtime — your node was offline long enough to miss `min_signed_per_window` × `signed_blocks_window` blocks). Migration is gated until you bring the validator back to `BOND_STATUS_BONDED` with `jailed: false`.

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
# Expect: status = BOND_STATUS_BONDED, jailed = false.

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

### What if the validator is `Unbonded` with `jailed: false`?

This is the voluntary-exit case (no slashing event, just `MsgUndelegate`'d below the threshold). `unjail` doesn't apply — there's nothing to un-jail. Instead, re-stake until you're back in the active set:

```bash
lumerad tx staking delegate <valoper> <amount>ulume \
  --from <validator-key> --chain-id <chain-id>
# Wait for the next end-block; the validator will rebond if its
# new stake puts it in the top max_validators slots.
```

Then re-run pre-flight as in step 5 above, and proceed.

## Step 4 — Stop the validator

```bash
systemctl stop lumerad
```

Stopping before broadcast avoids double-signing risk and prevents the node from producing blocks with the legacy key while migration is in flight.

> **Downtime warning:** mainnet genesis sets `downtime_jail_duration` to `3600s` (1 hour). Do not let the stop-to-restart migration window exceed this time; if the window approaches 1 hour, restart and catch up before retrying the migration.

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

### `would_succeed: false`, `rejection_reason: validator is jailed (status: ...)`

Your validator was kicked out of the active set for a slashable offense (almost always downtime). The pre-flight response will also show `validator_jailed: true` and `validator_status` ∈ {`BOND_STATUS_UNBONDING`, `BOND_STATUS_UNBONDED`}. The full recovery flow — restart node → wait for catch-up → `unjail` → confirm `BOND_STATUS_BONDED` → stop node → retry migration — is documented in [Step 3a](#step-3a--recovering-from-a-jailed-or-non-bonded-validator).

The minimum command, assuming the node is up and synced:

```bash
lumerad tx slashing unjail \
  --from <validator-key> \
  --chain-id <chain-id> \
  --gas auto --gas-adjustment 1.3 --fees <fee>ulume --yes
```

If unjail itself fails with `validator still jailed; cannot be unjailed`, the slashing window hasn't fully elapsed. Wait, then retry.

### `would_succeed: false`, `rejection_reason: validator status is unbonded (not bonded)` (no jail)

The pre-flight response shows `validator_jailed: false` and `validator_status: BOND_STATUS_UNBONDING` (or `UNBONDED`). This is the voluntary-exit case: the validator self-unbonded (or fell out of the top `max_validators` slots) without ever being jailed, so `unjail` does nothing. Re-stake to re-enter the active set:

```bash
lumerad tx staking delegate <valoper> <amount>ulume \
  --from <validator-key> --chain-id <chain-id>
```

Then wait for the next end-block, re-run pre-flight, and proceed once `validator_status` is `BOND_STATUS_BONDED`. See [Step 3a — voluntary-exit case](#what-if-the-validator-is-unbonded-with-jailed-false) for the longer treatment.

> **Older versions of this doc / chain referenced `rejection_reason: validator is not in bonded status`.** The current chain produces the more specific messages above. If you see the old text, you're talking to a node running pre-jailed-field code; the underlying condition is the same.

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
     --keyring-backend file

   lumerad keys show val-msig-new --address
   # lumera1...  <-- this is the new operator address
   ```

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
