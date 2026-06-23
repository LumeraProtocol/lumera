# EVM Migration Helper Scripts - User Guide

**Applies to**: Lumera chains with the `x/evmigration` module enabled after the EVM upgrade.
**Audience**: Terminal users running `lumerad`: regular account holders, validator operators, supernode operators, and multisig coordinators.

---

## Start Here

Use the script that matches the account you are migrating:

| Situation                                       | Script                         | What it does                                                                                                                      |
| ----------------------------------------------- | ------------------------------ | --------------------------------------------------------------------------------------------------------------------------------- |
| Regular single-key account                      | `scripts/migrate-account.sh`   | Migrates a legacy coin-type 118 `secp256k1` account to a coin-type 60 `eth_secp256k1` account.                                   |
| Single-key validator operator                   | `scripts/migrate-validator.sh` | Migrates the validator operator account and re-keys validator-related state. The validator node must be stopped before broadcast. |
| Multisig account or multisig validator operator | `scripts/migrate-multisig.sh`  | Runs a 4-step coordinator/co-signer ceremony: `generate`, `sign`, `combine`, `submit`.                                           |

Most users should do this first:

```bash
./scripts/migrate-account.sh <legacy-key> <new-evm-key> --dry-run
```

If the dry-run succeeds, remove `--dry-run` and run the same command to broadcast.

Important rules:

- The **legacy key** is the old Lumera key: coin type 118, `secp256k1`.
- The **new key** is the EVM-compatible key: coin type 60, `eth_secp256k1`.
- For mnemonic-based migrations, both keys normally come from the **same mnemonic** with different coin types.
- The destination address must be **fresh**. It must not already exist on-chain, must not have bank balance, and must not appear in any migration record.
- Run `--dry-run` first. It performs the same safety checks and stops before broadcast.
- Do not use `migrate-account.sh` for validators. Use `migrate-validator.sh`.
- Do not use the single-key scripts for multisig. They will detect multisig accounts and point you to `migrate-multisig.sh`.

During pre-flight, the scripts now print the important successful checks explicitly:

```text
INFO  check OK: no migration record found for legacy address lumera1...
INFO  check OK: destination address lumera1... has no migration record as a legacy address
INFO  check OK: no migration record found by new address lumera1...
INFO  check OK: destination address lumera1... does not exist on-chain
```

If any of these checks fails, stop and read the error. Reusing an already-used destination address is unsafe and the chain will reject it.

---

## What The Scripts Add

Compared with raw `lumerad tx evmigration ...`, the scripts add:

- **Migration record checks**: the legacy address must not already be migrated.
- **Destination checks**: the new address must not be an old legacy address, must not already be a migration destination, and must not already exist as an auth account.
- **Pre-flight estimate**: the script queries `migration-estimate`, prints what would move, and aborts if the keeper says the migration would fail.
- **Wrong-script guards**: account script rejects validators; validator script rejects non-validators; single-key scripts reject multisig.
- **Validator cap checks**: validator migration checks `max_validator_delegations`.
- **Validator downtime acknowledgement**: validator migration requires explicit acknowledgement that the node is stopped.
- **Broadcast validation**: the scripts reject CheckTx failures immediately.
- **Post-migration verification**: after broadcast, the scripts verify the migration record and balances.
- **Dry-run mode**: runs safety checks and preview without broadcasting.
- **Automatic gas sizing**: broadcasts with `--gas auto` (record-count fallback); no manual `--gas` needed.

---

## Prerequisites

On the machine where you run the scripts:

- `lumerad` built from a post-EVM-upgrade commit.
- `bash` 4.4 or newer.
- `jq` on `PATH`.
- Access to a Lumera RPC endpoint. By default, the scripts use local CometBFT RPC at `tcp://localhost:26657`.
- The legacy key and new EVM key in the keyring, or a mnemonic file for one-shot import.

Verify your binary supports the migration module:

```bash
lumerad query evmigration --help
```

The help output should include commands such as `migration-estimate`, `migration-record`, and `migration-record-by-new-address`.

---

## Getting The Scripts

Release tarballs include:

```text
lumerad
scripts/
  evmigration-common.sh
  migrate-account.sh
  migrate-validator.sh
  migrate-multisig.sh
```

Keep the scripts together in the same `scripts/` directory. They source `evmigration-common.sh` relative to their own path.

From a source checkout:

```bash
git clone https://github.com/LumeraProtocol/lumera.git
cd lumera
./scripts/migrate-account.sh --help
```

---

## Single-Key Common Flags

`migrate-account.sh` and `migrate-validator.sh` take positional arguments:

```text
<legacy-key-name> <new-key-name>
```

They accept these common flags:

| Flag                       | Default                                               | Description                                                          |
| -------------------------- | ----------------------------------------------------- | -------------------------------------------------------------------- |
| `--node <url>`             | `$LUMERA_NODE` or `tcp://localhost:26657`             | RPC endpoint.                                                        |
| `--chain-id <id>`          | `$LUMERA_CHAIN_ID`, `$CHAIN_ID`, or auto-detected     | Chain ID used for tx generation and broadcast.                       |
| `--keyring-backend <b>`    | `test`                                                | `test`, `file`, or `os`.                                             |
| `--keyring-dir <dir>`      | unset                                                 | Keyring directory independent of `--home`.                           |
| `--home <dir>`             | `lumerad` default                                     | Passed through to `lumerad`.                                         |
| `--mnemonic-file <path>`   | unset                                                 | One-shot import from a mnemonic file with mode `0600` or stricter.   |
| `--yes`, `-y`              | off                                                   | Skip the normal broadcast confirmation prompt.                       |
| `--dry-run`                | off                                                   | Run checks and preview, then stop before broadcast.                  |
| `--binary <path>`          | `lumerad` from `PATH`                                 | Use a specific `lumerad` binary.                                     |

### Chain ID Resolution

For the migration scripts, `--chain-id` is optional. The scripts resolve the chain ID in this order:

1. `--chain-id <id>`
2. `$LUMERA_CHAIN_ID`
3. `$CHAIN_ID`
4. Auto-detection from `lumerad status --node <node>` using `.node_info.network`

The resolved chain ID is logged at the top of the run:

```text
INFO  chain ID: lumera-mainnet-1
```

or:

```text
INFO  auto-detected chain ID from tcp://localhost:26657: lumera-mainnet-1
```

`migrate-multisig.sh generate`, `sign`, and `submit` use the same chain ID resolution. `generate` and `submit` can auto-detect it from the RPC endpoint. `sign` can auto-detect it too when `--node` points at a reachable RPC endpoint.

### RPC Endpoint Setup

Most examples below omit `--node` for readability. They work when `lumerad` can reach a local node at the default `tcp://localhost:26657`.

If you are not running a local node, or your local CLI/RPC setup points at the wrong network, set a mainnet RPC endpoint explicitly before running the scripts:

```bash
export LUMERA_NODE=https://rpc.lumera.io:443
```

or pass it per command:

```bash
./scripts/migrate-account.sh <legacy-key> <new-evm-key> --node https://rpc.lumera.io:443
```

Lumera mainnet CometBFT RPC endpoint:

| Provider       | RPC endpoint                  |
| -------------- | ----------------------------- |
| Lumera mainnet | `https://rpc.lumera.io:443` |

Public endpoints can be rate-limited or temporarily unavailable. For production operations, prefer your own node or a provider endpoint with an SLA/API key.

### Environment Variables

- `LUMERA_NODE`: default RPC endpoint.
- `LUMERA_CHAIN_ID`: preferred chain ID default.
- `CHAIN_ID`: secondary chain ID default.
- `LUMERA_TX_WAIT_TIMEOUT`: tx inclusion wait timeout in seconds. Default is `90`.

Example for slow networks:

```bash
LUMERA_TX_WAIT_TIMEOUT=300 ./scripts/migrate-account.sh legacy new
```

---

## Gas

The scripts no longer use the CLI default (200000) gas for the broadcast. `migrate-account.sh` and `migrate-validator.sh` broadcast with `--gas auto --gas-adjustment 1.5` — the chain simulates the exact gas, and since migration fees are waived this costs nothing. If the simulate fails (e.g. an RPC timeout on a validator with a very large delegation set), they fall back to a record-count formula, `6,000,000 + 1,500,000 × records`. Note: `--gas auto` runs the full handler during the simulate, so for large validators it can take tens of seconds to ~2 minutes — if your node's `timeout_broadcast_tx_commit` is at the default 10s you may see an `EOF` error; raise it (e.g. to `600s`) or use the fixed-gas fallback (which skips the simulate). `migrate-multisig.sh combine` likewise simulates gas with `--gas auto` when it builds the unsigned tx, so the combine step needs connectivity to a node. Override the constants via `MIGRATION_GAS_BASE`, `MIGRATION_GAS_PER_RECORD`, `MIGRATION_GAS_ADJUSTMENT`.

---

## Account Migration

Use this for regular single-key accounts and non-validator supernode operator accounts.

### 1. Import Or Create Both Keys

Import the legacy key:

```bash
lumerad keys add <legacy-key> --recover --coin-type 118 --algo secp256k1 --keyring-backend test
```

> **Already have the legacy key?** If this key is already in the keyring (e.g. the operator key you've been running the node with), this command fails with **`duplicated address created`** — the recovered key resolves to an address the keyring already holds. Skip the import and pass the *existing* key name as `<legacy-key>` to the script. Only the new EVM key needs to be recovered fresh.

Import the new EVM-compatible key:

```bash
lumerad keys add <new-evm-key> --recover --coin-type 60 --algo eth_secp256k1 --keyring-backend test
```

Enter the same mnemonic for both if you are migrating a normal mnemonic-derived account.

Check addresses before proceeding:

```bash
lumerad keys show <legacy-key> -a --keyring-backend test
lumerad keys show <new-evm-key> -a --keyring-backend test
```

The legacy address must match your pre-EVM Lumera address. The new address must be a fresh destination.

### 2. Dry-Run

```bash
./scripts/migrate-account.sh <legacy-key> <new-evm-key> --dry-run
```

Dry-run performs:

- legacy key type check
- new key type check
- legacy migration record lookup
- destination migration record lookup by legacy address
- destination migration record lookup by new address
- destination auth-account existence check
- `migration-estimate`
- multisig and validator rejection
- balance snapshot

It exits before the confirmation prompt and before broadcast.

### 3. Broadcast

```bash
./scripts/migrate-account.sh <legacy-key> <new-evm-key>
```

Use `--yes` only if you want to skip the final confirmation prompt:

```bash
./scripts/migrate-account.sh <legacy-key> <new-evm-key> \
  --yes
```

Before broadcast, the script prints a tx-body preview. After broadcast, it waits for inclusion and verifies chain state.

### 4. Example Successful Output

This example uses sample key names:

- legacy key: `alice-legacy`
- new EVM key: `alice-evm`

Your addresses, balance, height, time, gas estimate, and tx hash will be different.

```text
$ ./scripts/migrate-account.sh alice-legacy alice-evm
INFO  chain ID: lumera-devnet-1
INFO  legacy key alice-legacy -> address lumera1pz9mzf725dx62yatk8dtaqu44746t5j63qc7v2
INFO  new EVM key alice-evm -> address lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v
INFO  check OK: no migration record found for legacy address lumera1pz9mzf725dx62yatk8dtaqu44746t5j63qc7v2
INFO  check OK: destination address lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v has no migration record as a legacy address
INFO  check OK: no migration record found by new address lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v
INFO  check OK: destination address lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v does not exist on-chain
Migration preview for legacy account lumera1pz9mzf725dx62yatk8dtaqu44746t5j63qc7v2 (coin-type 118, secp256k1):
  Validator:         no
  Multisig:          no
  Balance:           10000ulume
  Delegations:       none
  Unbonding:         none
  Redelegations:     none
  Authz grants:      none
  Feegrants:         none
  Actions:           none
  Supernode:         no
  Would succeed:     yes
INFO  migrating legacy account lumera1pz9mzf725dx62yatk8dtaqu44746t5j63qc7v2 -> EVM-compatible lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v

Tx body to broadcast:
  Type:           /lumera.evmigration.MsgClaimLegacyAccount
  Legacy address: lumera1pz9mzf725dx62yatk8dtaqu44746t5j63qc7v2
  New address:    lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v
  Gas limit:      200000

Proceed with migration? [y/N] y
gas estimate: 672811
INFO  broadcast tx FFD7FEB173B8C0D5493F6F2A2EA1894BA0AD4D909EA2A09448E92DDEBF7E68AC; waiting for inclusion...
INFO  tx included at height 985 (waited 0s)

Migration record (chain state):
  legacy address: lumera1pz9mzf725dx62yatk8dtaqu44746t5j63qc7v2
  new address:    lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v
  height:         985
  unix time:      1777582779

New account balance (lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v):
  10000ulume

INFO  migration complete
INFO    legacy: lumera1pz9mzf725dx62yatk8dtaqu44746t5j63qc7v2
INFO    new:    lumera1ck5p50xqgtstastxlxfvzejr6q03xapqmk3x0v
INFO    tx:     FFD7FEB173B8C0D5493F6F2A2EA1894BA0AD4D909EA2A09448E92DDEBF7E68AC
```

The important things to confirm in this output are:

- `Would succeed: yes`
- all four `check OK` lines are present
- the tx is included in a block
- the migration record maps the legacy address to the expected new address
- the new account balance contains the migrated funds

After the sample run succeeds, the final summary is:

```text
INFO  migration complete
INFO    legacy: lumera1...
INFO    new:    lumera1...
INFO    tx:     ...
```

It also prints the final migration record and new account balance.

### 5. Optional Cleanup

After you verify the migration, delete the old key if your operational policy allows it:

```bash
lumerad keys delete <legacy-key> --keyring-backend test
```

---

## One-Shot Mnemonic File Flow

Use this when you do not want to manually import keys first.

Create a file containing the mnemonic and lock down permissions:

```bash
chmod 0600 /secure/tmp/mnemonic.txt
```

Run:

```bash
./scripts/migrate-account.sh <legacy-key-name> <new-key-name> \
  --mnemonic-file /secure/tmp/mnemonic.txt \
  --yes
```

The script imports missing keys, runs the migration, and deletes only the keyring entries it created for this run. The mnemonic file itself is not modified.

If a key name already exists, the script derives the same role from the mnemonic and compares addresses:

- if the existing key matches the mnemonic-derived address, the script reuses it
- if the existing key points to a different address, the script stops before migration

Example with the legacy key already present and the new EVM key imported from the mnemonic:

```text
$ ./scripts/migrate-account.sh bob-legacy bob-evm --mnemonic-file /secure/tmp/mnemonic.txt
INFO  chain ID: lumera-devnet-1
INFO  legacy key bob-legacy already exists in keyring and matches mnemonic; reusing it
INFO  imported new EVM key bob-evm from mnemonic for this run
INFO  legacy key bob-legacy -> address lumera1e82483sre0qcm2x2ajqgyzj4evxzy3cz8xsrq0
INFO  new EVM key bob-evm -> address lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0
INFO  check OK: no migration record found for legacy address lumera1e82483sre0qcm2x2ajqgyzj4evxzy3cz8xsrq0
INFO  check OK: destination address lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0 has no migration record as a legacy address
INFO  check OK: no migration record found by new address lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0
INFO  check OK: destination address lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0 does not exist on-chain
Migration preview for legacy account lumera1e82483sre0qcm2x2ajqgyzj4evxzy3cz8xsrq0 (coin-type 118, secp256k1):
  Validator:         no
  Multisig:          no
  Balance:           25000ulume
  Delegations:       none
  Unbonding:         none
  Redelegations:     none
  Authz grants:      none
  Feegrants:         none
  Actions:           none
  Supernode:         no
  Would succeed:     yes
INFO  migrating legacy account lumera1e82483sre0qcm2x2ajqgyzj4evxzy3cz8xsrq0 -> EVM-compatible lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0

Tx body to broadcast:
  Type:           /lumera.evmigration.MsgClaimLegacyAccount
  Legacy address: lumera1e82483sre0qcm2x2ajqgyzj4evxzy3cz8xsrq0
  New address:    lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0
  Gas limit:      200000

Proceed with migration? [y/N] y
gas estimate: 668329
INFO  broadcast tx 7F6CB7EF6DB1BAD888FA8D1371D6794A96171875A47AAE7579565A17BE7E07CF; waiting for inclusion...
INFO  tx included at height 13496 (waited 1s)

Migration record (chain state):
  legacy address: lumera1e82483sre0qcm2x2ajqgyzj4evxzy3cz8xsrq0
  new address:    lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0
  height:         13496
  unix time:      1777910386

New account balance (lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0):
  25000ulume

INFO  migration complete
INFO    legacy: lumera1e82483sre0qcm2x2ajqgyzj4evxzy3cz8xsrq0
INFO    new:    lumera1hlauuqfmnhdn8m9x0p9g3hjfrzlsg92a6u8cd0
INFO    tx:     7F6CB7EF6DB1BAD888FA8D1371D6794A96171875A47AAE7579565A17BE7E07CF
```

---

## Validator Migration

Use this for a single-key validator operator account.

The validator node must be stopped before broadcasting. The migration re-keys validator operator state and related staking references. Your consensus key (`priv_validator_key.json`) is not changed.

### 1. Plan Downtime

Most migrations complete quickly, but the validator can miss blocks while stopped. Plan a maintenance window using your chain's slashing parameters (`signed_blocks_window`, `min_signed_per_window`) and leave margin for restart.

### 2. Validator Dry-Run

```bash
./scripts/migrate-validator.sh <legacy-op-key> <new-evm-op-key> \
  --i-have-stopped-the-node \
  --dry-run
```

`--i-have-stopped-the-node` is still required in dry-run. It is an explicit acknowledgement gate for validator migration. `--yes` does not satisfy this gate.

Dry-run checks the same destination safety rules as account migration, then checks:

- the legacy account is not multisig
- the legacy account is a validator operator
- validator delegation, unbonding, and redelegation counts are within `max_validator_delegations`
- `migration-estimate.would_succeed` is true

### 3. Stop The Validator Node

Examples:

```bash
systemctl stop lumerad
```

or:

```bash
docker compose stop lumerad
```

The scripts do not manage your service process. You must stop and restart it yourself.

### 4. Broadcast

```bash
./scripts/migrate-validator.sh <legacy-op-key> <new-evm-op-key> \
  --i-have-stopped-the-node
```

For non-interactive automation:

```bash
./scripts/migrate-validator.sh <legacy-op-key> <new-evm-op-key> \
  --yes \
  --i-have-stopped-the-node
```

The script prints a warning banner, previews the tx body, broadcasts, waits for inclusion, and verifies the migration.

On success, it prints a checklist:

```text
INFO  validator migration complete - post-migration checklist:
INFO    1. Import <new-key> into the production keyring (correct --keyring-backend)
INFO    2. Restart lumerad
INFO    3. Verify new operator via: lumerad query staking validator <new-valoper>
INFO    4. Monitor missed-block counters for the next few blocks
```

### 5. Restart The Validator

Make sure the new operator key is available in the production keyring, then restart:

```bash
lumerad keys add <new-evm-op-key> \
  --recover \
  --coin-type 60 \
  --algo eth_secp256k1 \
  --keyring-backend file

systemctl start lumerad
```

Verify:

```bash
lumerad query staking validator <new-valoper>
lumerad query evmigration migration-record <legacy-op-address>
```

---

## Multisig Migration

Use `migrate-multisig.sh` for multisig accounts and multisig validator operators.

A multisig migration is a K-of-N signing ceremony:

1. Coordinator creates `proof.json`.
2. Co-signers sign the proof and return `partial-*.json`.
3. Coordinator combines partials into `tx.json`.
4. Coordinator submits `tx.json`.

The coordinator does not need signing keys. Co-signers sign locally.

### Multisig Requirements

- The legacy multisig pubkey must already be seeded on-chain. If it has never signed a transaction, submit any K-of-N multisig-signed tx first.
- The destination is also a multisig built from `eth_secp256k1` sub-keys.
- Legacy and new multisigs must mirror each other: same K, same N, and matching signer indices.
- Signer order matters. The signer index is part of the multisig address, so use `--nosort` when you create the destination key and keep the EVM sub-keys in the same signer order as the legacy multisig.
- The new multisig destination address must be fresh.
- For same-mnemonic migrations, recover each EVM sub-key from the matching legacy mnemonic, then compose the destination multisig in the legacy signer order.
- For multisig validators, stop the validator node before `submit`.

### 0. Build The Destination Multisig Key

Create or import each EVM sub-key first:

```bash
lumerad keys add <signer-1-evm> --recover \
  --coin-type 60 \
  --algo eth_secp256k1 \
  --keyring-backend test

lumerad keys add <signer-2-evm> --recover \
  --coin-type 60 \
  --algo eth_secp256k1 \
  --keyring-backend test

lumerad keys add <signer-3-evm> --recover \
  --coin-type 60 \
  --algo eth_secp256k1 \
  --keyring-backend test
```

Then create the destination multisig key with `--nosort`:

```bash
lumerad keys add <new-evm-multisig-key> \
  --multisig <signer-1-evm>,<signer-2-evm>,<signer-3-evm> \
  --multisig-threshold 2 \
  --nosort \
  --keyring-backend test

lumerad keys show <new-evm-multisig-key> -a --keyring-backend test
```

The order must mirror the legacy on-chain signer order. If the `generate` step prints a different signer mapping than you intended, stop, rebuild the destination multisig key in the correct order, and generate a new proof.

> **⚠️ Critical: build the destination multisig with `--nosort`.** Cosmos SDK's `keys add --multisig` sorts sub-pubkeys by bytes by default. Because legacy `secp256k1` and new `eth_secp256k1` pubkey bytes sort differently, the default sort breaks the mirror-source invariant — co-signers will fail at `sign` with a "signer index mismatch" error and you'll have to rebuild the destination key and regenerate `proof.json`. Always pass `--nosort` and list members in the same order as the legacy multisig's on-chain `public_keys`. See [Step 0](#0-build-the-destination-evm-multisig) below.

### 0. Build the destination EVM multisig

Run this before `generate`. Each member who holds a legacy sub-key must already have created an `eth_secp256k1` sub-key in their local keyring; collect the N member key names (or base64 pubkeys) and assemble the composite **in the same order as the legacy multisig** and **with `--nosort`**:

```bash
# 1. Read legacy member order from chain (this is the canonical reference order).
lumerad query auth account <legacy-multisig-address> -o json \
  | jq -r '.account.value.pub_key.public_keys[].key'

# 2. For each legacy member at index i, identify their eth_secp256k1 sub-key.
#    Convention: name them <member>_evm to keep the mapping obvious.

# 3. Build the destination multisig in matching order, WITH --nosort.
lumerad keys add <new-multisig-key> \
  --multisig=<member-1>_evm,<member-2>_evm,...,<member-N>_evm \
  --multisig-threshold=<K> \
  --nosort
```

`--nosort` preserves the order you list, so each member ends up at the same signer index on both sides — which is what the on-chain mirror-source rule requires and what `combine` will check.

If you skip `--nosort` (or list members in a different order than the legacy multisig), `migrate-multisig.sh sign` will reject the partial with a clear "signer-index mismatch" error and the exact rebuild command — fix it, then re-run `generate`.

### 1. Coordinator: Generate

```bash
./scripts/migrate-multisig.sh generate \
  --legacy <legacy-multisig-key-or-address> \
  --new-key <new-evm-multisig-key> \
  --out proof.json
```

If you already created the destination EVM multisig key locally, use `--new-key`. The script expects that keyring entry to already exist; it does not create the destination multisig for you. It reads the existing keyring entry, extracts its `eth_secp256k1` signer pubkeys, derives the destination address, and infers `--new-sub-pub-keys` for you.

If you do not have a local destination multisig key, pass the signer pubkeys explicitly:

```bash
./scripts/migrate-multisig.sh generate \
  --legacy <legacy-multisig-key-or-address> \
  --new lumera1<new-multisig-address> \
  --new-sub-pub-keys <eth-pubkey-or-key-name-1>,<eth-pubkey-or-key-name-2>,<eth-pubkey-or-key-name-3> \
  --out proof.json
```

The script infers whether this is a regular claim or validator migration from chain state. You do not pass a kind.

`--legacy` can be either the legacy multisig account address or a local multisig key name. If you pass a key name, the script resolves it to the account address before querying chain state and generating the proof.

`--out` defaults to `proof.json`. `--chain-id` is optional when `LUMERA_CHAIN_ID` or `CHAIN_ID` is set, or when the script can auto-detect the chain ID from the RPC endpoint. If your local `lumerad` is not configured to use the correct RPC endpoint, pass `--node https://rpc.lumera.io:443` for Lumera mainnet.

`--new` is optional when using `--new-sub-pub-keys`, but strongly recommended. When supplied, the script can perform all destination safety checks before co-signers spend time signing. When using `--new-key`, the script resolves `--new` from the local key automatically.

`--new-sub-pub-keys` entries may be local keyring key names or base64-encoded compressed 33-byte `eth_secp256k1` pubkeys. `--new-threshold` is optional; if omitted, it defaults to the on-chain legacy multisig threshold.

The generate step checks:

- legacy account has an on-chain multisig pubkey
- legacy account is multisig
- migration kind can be inferred from chain state
- legacy address has no migration record
- if `--new` is supplied, destination has no migration records and does not exist on-chain
- `migration-estimate.would_succeed` is true

The generate step also prints both signer orders and the signing instruction:

```text
INFO  On-chain legacy signer order (2-of-3)
INFO    signer index is part of the multisig address; keep this order unchanged
INFO    index 0: <legacy-signer-0-pubkey>
INFO    index 1: <legacy-signer-1-pubkey>
INFO    index 2: <legacy-signer-2-pubkey>
INFO  Destination signer order (2-of-3)
INFO    signer index is part of the multisig address; keep this order unchanged
INFO    index 0: <evm-signer-0-pubkey>
INFO    index 1: <evm-signer-1-pubkey>
INFO    index 2: <evm-signer-2-pubkey>
INFO  Signing instruction: co-signers should sign the same signer index on both legacy and new sides
INFO  For same-mnemonic migrations: recover each EVM sub-key from the matching legacy mnemonic, then build the destination multisig with --nosort in the legacy order above
INFO  done — distribute proof.json to the K co-signers
```

Distribute `proof.json` to co-signers only after the legacy and destination signer orders are correct.

### 2. Co-Signers: Sign

Signer with both legacy and new sub-keys:

```bash
./scripts/migrate-multisig.sh sign proof.json \
  --from <my-legacy-sub-key> \
  --new-key <my-new-eth-sub-key> \
  --out partial-alice.json
```

Signer with only the legacy sub-key:

```bash
./scripts/migrate-multisig.sh sign proof.json \
  --from <my-legacy-sub-key> \
  --out partial-legacy-alice.json
```

Signer with only the new sub-key:

```bash
./scripts/migrate-multisig.sh sign proof.json \
  --new-key <my-new-eth-sub-key> \
  --out partial-new-alice.json
```

At least one of `--from` or `--new-key` is required. A signer who has both should pass both. For multisig-to-multisig migrations, the script rejects a both-sides signature when the legacy key and new EVM key resolve to different signer indices; recreate or rederive the destination multisig so each participant's new key occupies the same signer position as their legacy key.

Successful signing prints the matched index:

```text
INFO  legacy signer index 0: <my-legacy-sub-key>
INFO  new signer index 0: <my-new-eth-sub-key>
INFO  Signing instruction: sign the same signer index on both legacy and new sides; combine only counts matching indices
INFO  signing proof.json: legacy(<my-legacy-sub-key>)
new(<my-new-eth-sub-key>)
INFO  partial written to partial-alice.json
```

One-sided partials are allowed, but they do not satisfy quorum by themselves. The final combined proof must have the same K signer indices on both legacy and new sides.

Return the `partial-*.json` files to the coordinator.

### 3. Coordinator: Combine

```bash
./scripts/migrate-multisig.sh combine \
  partial-alice.json partial-bob.json partial-carol.json \
  --node <rpc-endpoint> \
  --out tx.json
```

The combine step runs `combine-proof --gas auto` to simulate gas when building the unsigned tx, so it requires a reachable node (default `tcp://localhost:26657`, overridable with `--node`).

The combine step verifies:

- all partial files agree on chain ID, legacy address, new address, kind, payload, thresholds, signature format, and sub-pub-key lists
- each side has at least K signatures
- the matching-index intersection also has at least K signatures
- `lumerad tx evmigration combine-proof` accepts the partial signatures

If per-side quorum is met but matching-index quorum is not, the script exits 4. Example: legacy signed by indices `{0,1}` and new signed by `{0,2}` is not enough for 2-of-3, because only index `0` signed both sides.

### 4. Coordinator: Submit

```bash
./scripts/migrate-multisig.sh submit tx.json
```

For multisig validator migration:

```bash
./scripts/migrate-multisig.sh submit tx.json \
  --i-have-stopped-the-node
```

`submit-proof` does not take `--from`, fee flags, or gas-price flags. Authorization is in the proof bytes, and fees are waived by the migration ante handler.

The submit step checks:

- `tx.json` is a multisig-to-multisig migration tx
- legacy address has no migration record
- new address has no migration records
- new address does not exist on-chain
- fresh `migration-estimate` still succeeds
- after broadcast, migration record and balances verify

`--dry-run` works on `submit`: it performs checks and stops before broadcast. `--yes` skips the ordinary confirmation prompt, but it does not replace `--i-have-stopped-the-node` for validator migrations.

### Worked Example: Regular 2-of-3 Multisig Account

This example follows a real devnet transcript. It migrates a regular, non-validator 2-of-3 multisig account:

- Legacy multisig key: `seed_sale_4_legacy_nosort`
- New EVM multisig key: `seed_sale_4_evm_nosort`
- Legacy address: `lumera135a3ccpu96acwatcv3vu87hrdft3vygnn90y9g`
- New address: `lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc`

Generate the proof template:

```bash
./scripts/migrate-multisig.sh generate \
  --legacy lumera135a3ccpu96acwatcv3vu87hrdft3vygnn90y9g \
  --new-key seed_sale_4_evm_nosort \
  --chain-id lumera-devnet-1 \
  --node https://rpc.pastel.network:443 \
  --out files/proof.json
```

The script resolves the destination key, previews the migration, infers this is a regular account claim, checks that the destination is fresh, prints both signer orders, and writes `files/proof.json`:

```text
INFO  chain ID: lumera-devnet-1
INFO  using destination EVM multisig key seed_sale_4_evm_nosort -> address lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc
Migration preview for legacy account lumera135a3ccpu96acwatcv3vu87hrdft3vygnn90y9g (coin-type 118, secp256k1):
  Validator:         no
  Multisig:          yes (2-of-3)
  Balance:           5000000000900ulume
  Delegations:       none
  Unbonding:         none
  Redelegations:     none
  Authz grants:      none
  Feegrants:         none
  Actions:           none
  Supernode:         no
  Would succeed:     yes
INFO  auto-detected multisig migration kind: claim
INFO  check OK: no migration record found for legacy address lumera135a3ccpu96acwatcv3vu87hrdft3vygnn90y9g
INFO  check OK: destination address lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc has no migration record as a legacy address
INFO  check OK: no migration record found by new address lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc
INFO  check OK: destination address lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc does not exist on-chain
INFO  generating proof template at files/proof.json
INFO    legacy: lumera135a3ccpu96acwatcv3vu87hrdft3vygnn90y9g
INFO    new:    lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc
INFO  On-chain legacy signer order (2-of-3)
INFO    signer index is part of the multisig address; keep this order unchanged
INFO    index 0: Api82SIbjLdzvT/O+zvOZVIOiK7x8U37PPYlgzHwKiff
INFO    index 1: AxtYULXuxXtEgxLI5ZZYAZOXPmZR4bVulmU1YlvCVxH9
INFO    index 2: A7w3rlJ5Wc9A2MJVJGZyLuBIzVztH1uOM3eJLq0iCIhY
INFO  Destination signer order (2-of-3)
INFO    signer index is part of the multisig address; keep this order unchanged
INFO    index 0: AmnBRY8jp09QX5lCLbT13aBSR+M8MAmKyNT1YsSKbhGZ
INFO    index 1: A/L5Q/ReLsFVsvFLvInF+JakHnmAMz0+7XXtMeaUyab9
INFO    index 2: Au+UkGZBduvzKNDIj0Zouk3LXNeShkMq76aYrolEIjCF
INFO  Signing instruction: co-signers should sign the same signer index on both legacy and new sides
INFO  For same-mnemonic migrations: recover each EVM sub-key from the matching legacy mnemonic, then build the destination multisig with --nosort in the legacy order above
INFO  done — distribute files/proof.json to the K co-signers
```

The generated proof uses `version: 2`, `kind: "claim"`, and matching 2-of-3 side specs. The legacy side contains Cosmos `secp256k1` sub-pubkeys; the new side contains `eth_secp256k1` sub-pubkeys.

Each signer signs locally. In the sample, signer 0 signs both sides into `files/partial-1.json`; signer 1 repeats the same command with their own keys to create `files/partial-2.json`:

```bash
./scripts/migrate-multisig.sh sign files/proof.json \
  --from seed_sale_4_1 \
  --new-key seed_sale_4_1_evm \
  --chain-id lumera-devnet-1 \
  --out files/partial-1.json
```

Successful output shows that both keys are signer index 0:

```text
INFO  chain ID: lumera-devnet-1
INFO  legacy signer index 0: seed_sale_4_1
INFO  new signer index 0: seed_sale_4_1_evm
INFO  Signing instruction: sign the same signer index on both legacy and new sides; combine only counts matching indices
INFO  signing files/proof.json: legacy(seed_sale_4_1)
new(seed_sale_4_1_evm)
INFO  partial written to files/partial-1.json
```

The partial file contains one legacy signature and one new-side signature at the same signer index:

```json
{
  "partial_legacy_signatures": [
    {
      "index": 0,
      "signature": "..."
    }
  ],
  "partial_new_signatures": [
    {
      "index": 0,
      "signature": "..."
    }
  ]
}
```

After collecting two matching signer partials, combine them:

```bash
./scripts/migrate-multisig.sh combine files/partial-1.json files/partial-2.json \
  --node https://rpc.pastel.network:443 \
  --out files/tx.json
```

The combine step shows which signer indices are present and confirms all quorum checks:

```text
INFO  chain ID: lumera-devnet-1
Legacy-side partials (2-of-3 required):
  [X] signer 0  files/partial-1.json
  [X] signer 1  files/partial-2.json
  [ ] signer 2  (missing)
Legacy threshold satisfied: yes (2 >= 2)
New-side partials (2-of-3 required):
  [X] signer 0  files/partial-1.json
  [X] signer 1  files/partial-2.json
  [ ] signer 2  (missing)
New threshold satisfied: yes (2 >= 2)
Matching-index threshold satisfied: yes (2 >= 2)

INFO  combined tx written to files/tx.json
```

Dry-run the submit before broadcasting:

```bash
./scripts/migrate-multisig.sh submit files/tx.json \
  --chain-id lumera-devnet-1 \
  --node https://rpc.pastel.network:443 \
  --dry-run
```

For a regular multisig account, the submit summary has `Kind: claim` and does not require the validator downtime flag:

```text
==== Multisig migration submit ====
  Kind:        claim
  Legacy msig: 2-of-3
  New msig:    2-of-3 (eth sub-keys)
  Legacy:      lumera135a3ccpu96acwatcv3vu87hrdft3vygnn90y9g
  New:         lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc
===================================

INFO  --dry-run: stopping before broadcast
```

Broadcast when the dry-run is clean:

```bash
LUMERA_TX_WAIT_TIMEOUT=180 ./scripts/migrate-multisig.sh submit files/tx.json \
  --chain-id lumera-devnet-1 \
  --node https://rpc.pastel.network:443 \
  --yes
```

The sample broadcast was accepted immediately, but the public RPC did not index the tx before the timeout. The script kept printing wait progress, then proceeded to on-chain verification and completed successfully because the migration record and balances were present:

```text
INFO  broadcast tx 9106FC4E4E2BFA795D73EE3929777FC4BB33CAC46295E33DA90218361D5775E8; waiting for inclusion...
INFO  tx 9106FC4E4E2BFA795D73EE3929777FC4BB33CAC46295E33DA90218361D5775E8 not indexed yet; polling query tx for up to 179s
INFO  still waiting for tx 9106FC4E4E2BFA795D73EE3929777FC4BB33CAC46295E33DA90218361D5775E8 to be indexed (10s elapsed, timeout 180s)
...
WARN  tx 9106FC4E4E2BFA795D73EE3929777FC4BB33CAC46295E33DA90218361D5775E8 not indexed after 180s (timeout was 180s); it may still land on chain
WARN    check status manually: lumerad q tx 9106FC4E4E2BFA795D73EE3929777FC4BB33CAC46295E33DA90218361D5775E8
WARN    to wait longer on slow networks, re-run with: LUMERA_TX_WAIT_TIMEOUT=300 migrate-multisig.sh ...
WARN    proceeding to on-chain verification — if the migration record/balances are present, the migration succeeded

Migration record (chain state):
  legacy address: lumera135a3ccpu96acwatcv3vu87hrdft3vygnn90y9g
  new address:    lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc
  height:         17545
  unix time:      1782238473

New account balance (lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc):
  5000000000900ulume

INFO  migration complete
INFO    legacy: lumera135a3ccpu96acwatcv3vu87hrdft3vygnn90y9g
INFO    new:    lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc
INFO    tx:     9106FC4E4E2BFA795D73EE3929777FC4BB33CAC46295E33DA90218361D5775E8
```

If your output reaches `INFO migration complete`, the migration succeeded even if tx indexing timed out earlier.

---

## Safety Checks You Will See

The scripts perform these checks before broadcast:

| Check                                                          | Query or source                                       | Failure meaning                                                            |
| -------------------------------------------------------------- | ----------------------------------------------------- | -------------------------------------------------------------------------- |
| Legacy address has no migration record                         | `evmigration migration-record <legacy>`              | The source was already migrated. Do not broadcast again.                   |
| Destination was not previously a legacy migration source        | `evmigration migration-record <new>`                 | The destination address was already migrated from; choose another new key. |
| Destination was not previously used as a migration destination | `evmigration migration-record-by-new-address <new>` | Another migration already points to this new address.                      |
| Destination does not exist on-chain                            | `auth account <new>`                                 | The new address already has account state. Choose a fresh key.             |
| Migration would succeed                                        | `evmigration migration-estimate <legacy>`            | Keeper says the migration will fail; read `rejection_reason`.              |

Successful checks are logged as `INFO check OK: ...` so users can see exactly what was verified.

After broadcast, the scripts verify:

- migration record exists for the legacy address
- migration record points to the expected new address
- legacy bank balances are zero
- new bank balances are at least the pre-broadcast legacy balance snapshot

---

## Exit Codes

| Code   | Meaning                                      | Typical cause                                                                                                      |
| ------ | -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `0`  | Success or clean dry-run                     | No broadcast in dry-run; migration verified in normal mode.                                                        |
| `1`  | Usage error                                  | Bad arguments, missing required flag, bad mnemonic-file permissions, key name collision.                           |
| `2`  | Environment or query error                   | Missing binary, old binary, missing `jq`, RPC/query failure.                                                       |
| `3`  | Single-key vs multisig mismatch              | Single-key script saw multisig, or multisig script saw single-sig.                                                 |
| `4`  | Pre-flight or quorum failure                 | `migration-estimate.would_succeed=false`, or multisig combine lacks valid quorum.                                  |
| `5`  | Already migrated or destination already used | Migration record exists, destination used, or destination account exists.                                          |
| `6`  | Wrong script or validator cap error          | Account script used for validator, validator script used for non-validator, or validator record count exceeds cap. |
| `7`  | Post-verification failed                     | Broadcast happened, but record or balance checks did not pass. Investigate chain state manually.                   |
| `8`  | Multisig pubkey not seeded                   | Legacy multisig has no on-chain pubkey. Submit any multisig-signed tx first.                                       |
| `9`  | Multisig file integrity error                | Bad JSON, unsupported proof version, payload mismatch, or cross-file disagreement.                                 |
| `10` | User aborted or downtime not acknowledged    | Prompt declined, no TTY for required prompt, or missing `--i-have-stopped-the-node`.                             |

---

## Troubleshooting

### `new address ... already exists on-chain`

The destination address is not fresh. Do not use it for migration. Create or derive another coin-type 60 `eth_secp256k1` key and retry dry-run.

Check manually:

```bash
lumerad query auth account <new-address> --node <node>
lumerad query bank balances <new-address> --node <node>
```

### `legacy address ... is already migrated`

The migration record already exists. Check it:

```bash
lumerad query evmigration migration-record <legacy-address> --node <node>
```

If it points to the expected new address, the migration already completed. Use the new address going forward.

If it points to a different address, stop and investigate which key/mnemonic produced that destination.

### `new address ... is already a migration destination`

Another migration already used this destination address. Check:

```bash
lumerad query evmigration migration-record-by-new-address <new-address> --node <node>
```

Use a fresh destination key.

### `pre-flight: migration would fail: ...`

The chain's `migration-estimate` rejected the migration. Common reasons:

- legacy account not found
- migration disabled by governance
- migration window ended
- validator is not bonded
- validator migration exceeds `max_validator_delegations`
- account state is not supported for migration

Read the printed `rejection_reason` and fix that condition before retrying.

### `legacy account is a K-of-N multisig`

Use `migrate-multisig.sh`. The single-key scripts cannot migrate multisig accounts.

### `account ... is a validator`

Use `migrate-validator.sh` for single-key validators. Use `migrate-multisig.sh generate` for multisig validators; the script infers validator migration from chain state.

### `validator downtime not acknowledged`

Pass the explicit flag after stopping the node:

```bash
--i-have-stopped-the-node
```

`--yes` does not satisfy this check.

### Multisig `pubkey is not seeded on-chain`

The legacy multisig has never published its `LegacyAminoPubKey` on-chain, so the chain has no key for `generate` to read. This is common for genesis-funded multisigs that have only ever *received* funds. Confirm it with `lumerad query auth account <multisig-address>`: an unseeded account shows `pub_key: null` and `sequence: "0"`.

The script fails before creating a proof:

```text
INFO  chain ID: lumera-devnet-1
ERROR multisig pubkey is not seeded on-chain for lumera1...
ERROR submit any transaction from the multisig account first, then retry
```

Submit any multisig-signed transaction first, then retry `generate`. The simplest operation is a tiny self-send or delegation, but note it is **itself a K-of-N multisig tx**: for a 2-of-3 it needs at least two member signatures (`tx sign --multisig` x K -> `tx multisign` -> `tx broadcast`), not a single signature.

Unlike the fee-waived migration, this seed transaction pays normal gas. If the multisig has no spendable `ulume`, fund it first or broadcast with `--fee-granter <funded-account>`. Also include enough fees for the chain minimum. A raw `lumerad tx broadcast` can exit `0` while the returned JSON contains a CheckTx failure:

```text
{"height":"0","txhash":"...","codespace":"sdk","code":13,"raw_log":"fee not provided... insufficient fee"}
```

Always inspect the JSON `code`; `code: 0` means accepted, nonzero means rejected. See [migration.md -> Precondition: ensure the multisig pubkey is on-chain](migration.md#precondition-ensure-the-multisig-pubkey-is-on-chain) for the full command sequence.

### Multisig destination address is not what I expected

Changing the destination signer order changes the destination address, even when all EVM sub-keys came from the expected mnemonics. For example, these two destination multisigs contain the same three EVM sub-keys but resolve to different addresses:

```text
seed_sale_4_1_evm,seed_sale_4_2_evm,seed_sale_4_3_evm -> lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc
seed_sale_4_2_evm,seed_sale_4_3_evm,seed_sale_4_1_evm -> lumera14lldtdzf43zweznx5ptdvpyq3djpqe70la6nf9
```

Use `--nosort` and list EVM sub-keys in the same signer order as the legacy multisig. The `generate` step prints both orders. For a same-mnemonic migration, signer index 0's legacy mnemonic should produce signer index 0's EVM sub-key, signer index 1's legacy mnemonic should produce signer index 1's EVM sub-key, and so on.

Do not wait until `submit` to choose the destination. The destination address is determined when the destination multisig key is created. The operator should first build the destination multisig key, read the address from `lumerad keys show`, then let `generate` print and verify the final signer order.

### Multisig `--new ... does not match destination key ...`

This protects against copying the wrong destination address:

```text
ERROR --new lumera14lldtdzf43zweznx5ptdvpyq3djpqe70la6nf9 does not match destination key seed_sale_4_evm_nosort address lumera1knldeyjs0m6mfl29hdv0flpuhadcrf2p9ccjlc
```

Fix the command so `--new` matches the local `--new-key`, or omit `--new` when using `--new-key`; the script will derive the address from the keyring entry.

### Multisig `payload_hex mismatch`

The proof file was edited or came from incompatible inputs. Regenerate `proof.json` and redistribute it to co-signers.

### Multisig `... is signer index N, but new key ... is signer index M`

`sign` aborts before writing a partial file with:

```text
legacy key "alice-legacy" is signer index 0, but new key "alice-evm" is signer index 3; multisig migration requires the same signer position to approve both halves
```

**Why:** you passed both `--from` and `--new-key` in one `sign` call, but the two keys sit at *different positions* in their respective multisigs. The consensus mirror-source rule requires `legacy_proof.signer_indices == new_proof.signer_indices`, so a co-signer must occupy the **same signer index** on the legacy and new sides. This almost always means the destination multisig was built with a different member order than the legacy one — typically because `keys add --multisig` sorted the sub-keys (its default) instead of preserving caller order.

**Fix:** recreate the destination multisig with `--nosort`, listing the eth sub-keys in the **same member order** as the legacy multisig's `public_keys` (`lumerad query auth account <multisig-legacy-address>`), then regenerate `proof.json` with `--new-sub-pub-keys` in that same order. The early abort is intentional — without it the mismatch would surface much later, at `combine`, as `need K valid partial signatures signed on BOTH sides at matching indices`.

### Multisig combine reports `threshold satisfied: no`

`combine` prints exactly which signer indices are present and missing:

```text
Legacy-side partials (2-of-3 required):
  [X] signer 0  files/partial-1.json
  [ ] signer 1  (missing)
  [ ] signer 2  (missing)
Legacy threshold satisfied: no (1 < 2)
New-side partials (2-of-3 required):
  [X] signer 0  files/partial-1.json
  [ ] signer 1  (missing)
  [ ] signer 2  (missing)
New threshold satisfied: no (1 < 2)
Matching-index threshold satisfied: no (1 < 2) — one-sided partials do not count
```

Collect enough partial files to satisfy the threshold on both sides at the same signer indices. A legacy-only partial and a new-only partial from different indices do not combine into one valid signer approval.

### Tx wait timed out, then the script continued

This means the tx was broadcast, but `wait-tx` / `query tx` did not return an indexed tx before `LUMERA_TX_WAIT_TIMEOUT`:

```text
WARN  tx <hash> not indexed after 180s (timeout was 180s); it may still land on chain
WARN    proceeding to on-chain verification — if the migration record/balances are present, the migration succeeded
```

Do not assume this is a failed migration. The script then queries chain state directly. If it prints the migration record, destination balance, and `INFO migration complete`, the migration succeeded. To wait longer on slow RPC/indexer nodes, rerun with a larger timeout before broadcasting a new tx:

```bash
LUMERA_TX_WAIT_TIMEOUT=300 ./scripts/migrate-multisig.sh submit tx.json --yes
```

If the script exits with post-verification failure, check the record manually before retrying.

### Post-verification failed

The tx may already be on-chain. Verify manually:

```bash
lumerad query evmigration migration-record <legacy-address> --node <node>
lumerad query bank balances <legacy-address> --node <node>
lumerad query bank balances <new-address> --node <node>
```

If the record exists and balances are correct, the failure may have been transient RPC/indexer lag. If not, keep the tx hash and contact release maintainers.

---

## Non-Interactive Usage

For account migration:

```bash
./scripts/migrate-account.sh <legacy-key> <new-evm-key> \
  --yes
```

For validator migration:

```bash
./scripts/migrate-validator.sh <legacy-op-key> <new-evm-op-key> \
  --yes \
  --i-have-stopped-the-node
```

For multisig submit:

```bash
./scripts/migrate-multisig.sh submit tx.json \
  --yes
```

For multisig validator submit, also pass `--i-have-stopped-the-node`.

The scripts never handle keyring passwords directly. Password prompts depend on `--keyring-backend`.

---

## Related Documentation

- [migration.md](migration.md) - top-level migration methods.
- [validator-migration.md](validator-migration.md) - validator-specific operational guide.
- [supernode-migration.md](supernode-migration.md) - supernode migration and daemon-driven cleanup.
- [legacy-migration.md](../evmigration/legacy-migration.md) - architecture and keeper behavior.
- [evmigration-scripts-design.md](../../design/evmigration-scripts-design.md) - script design notes.
