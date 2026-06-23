# `migrate-batch.sh` — Batch driver for EVM migration

A single-process driver for migrating many legacy accounts to their EVM-compatible
counterparts in one run.

This script does **not** reimplement migration logic. It is a thin lifecycle
manager around the existing, audited `migrate-multisig.sh` and
`migrate-account.sh` scripts. Its job is to:

1. Parse an operator-supplied mnemonics file describing many legacy accounts.
2. Per target, query the chain for current state.
3. Per target, set up an **ephemeral keyring** (mode 0700, wiped on exit),
   import the required mnemonics, reconstruct the multisig, top up + self-send
   to publish the pubkey if needed, then delegate the migration ceremony itself
   to the existing scripts.
4. Verify the migration via `evmigration migration-record` and tear down.

The mnemonics never enter the operator's main keyring. The ephemeral keyring
is created in `$TMPDIR` and removed on exit (including on SIGINT and trap-
triggered failures).

## When to use

- You have N legacy accounts to migrate (N ≥ 5 makes this script worth it).
- You hold all signer mnemonics for the multisigs (this script is **not** a
  coordinator-style flow where co-signers sit on separate hosts; for that,
  call `migrate-multisig.sh sign` per signer host directly).
- You want idempotency: re-running the script skips already-migrated targets
  and re-classifies the rest from chain state, so partial runs are safe.

## Subcommands

```bash
migrate-batch.sh report   --mnemonics <file> [--plan-out <file>]
migrate-batch.sh status   --mnemonics <file> [--node <url>] [--chain-id <id>] [--target <name>]
migrate-batch.sh execute  --mnemonics <file> [...]
```

### `report`  (offline, no chain calls)

Classifies every entry in the mnemonics file as either:

- a **multisig migration target** (with its signer order derived from
  `public_keys` order in the file, NOT from name suffix),
- a **standalone single-sig migration target** (a `local` entry not referenced
  by any multisig),
- or a **signer** that participates in some multisig.

Fails (exit 9) if any multisig has a signer pubkey not present in the file.

### `status`  (read-only chain probe)

For each target, runs:

- `evmigration migration-record` (already migrated?)
- `auth account` (does the pubkey exist on chain?)
- `bank balances` (does the account hold ulume to pay fees?)

and prints one of: `migrated`, `ready`, `needs-pubkey`, `needs-funding`, `unknown`.

### `execute`  (the real thing)

For each target, runs the full lifecycle. Read the in-file docs for the exact
state machine.

Common flags:

| flag | meaning |
|---|---|
| `--target <name>` | Process only the named target. Use this for the first run. |
| `--funder <key>` | Operator-keyring key that pays fees for zero-balance targets. |
| `--top-up-amount <coins>` | How much to send to a zero-balance target. Default `200000ulume` (covers the 100000ulume self-send + 5000ulume fee + headroom). |
| `--funder-keyring-{backend,dir,home}` | How to reach the funder key. Defaults: `test` backend, lumerad's default home / keyring dir. |
| `--log-file <path>` | Append one JSONL audit record per lifecycle milestone (batch_start, target_start, classify, keyring_setup, reconstructed, funding_*, self_send_*, ceremony_start, target_done, batch_done). Mode 0600 on create, append-only, correlated by per-run `batch_id`. Operator handles rotation. |
| `--dry-run` | Stop after read-only steps + address reconstruction; no broadcasts. |
| `--yes` | Skip the interactive confirmation. |
| `--continue-on-error` | Don't stop the batch on the first failed target. |

## Mnemonics file format

A single JSON object. Keys are arbitrary local names. Values are entries of
type `local` (single key) or `multi` (multisig). See the long comment at the
top of `migrate-batch.sh` for the exact schema.

**Signer order matters.** For each `multi` entry, the `public_keys[]` array
defines the canonical signer order used to derive the multisig address.
This driver matches signer mnemonics to slots by exact **pubkey equality**,
never by name suffix.

## Suggested workflow for a fresh deployment

```bash
# 1. Offline: confirm the file parses and classifies cleanly.
./scripts/migrate-batch.sh report \
  --mnemonics output.json \
  --plan-out plan.json

# 2. Read-only: see what each target's on-chain state is.
./scripts/migrate-batch.sh status \
  --mnemonics output.json \
  --chain-id lumera-testnet-2 \
  --node tcp://localhost:26657

# 3. Dry-run one target end-to-end (no broadcasts).
./scripts/migrate-batch.sh execute \
  --mnemonics output.json \
  --target seed_sale_1 \
  --chain-id lumera-testnet-2 \
  --funder ops-funder --top-up-amount 200000ulume \
  --dry-run

# 4. Execute one target for real, then expand.
./scripts/migrate-batch.sh execute \
  --mnemonics output.json \
  --target seed_sale_1 \
  --chain-id lumera-testnet-2 \
  --funder ops-funder --top-up-amount 200000ulume

# 5. Full batch (with confirmation prompt).
./scripts/migrate-batch.sh execute \
  --mnemonics output.json \
  --chain-id lumera-testnet-2 \
  --funder ops-funder --top-up-amount 200000ulume
```

## Safety properties

- Mnemonics are written to mode-0600 temp files inside a mode-0700 ephemeral
  keyring directory, consumed by `import_from_mnemonic`, and deleted.
- The entire ephemeral keyring dir is removed on EXIT (success, failure, or
  SIGINT), via a `trap`.
- The reconstructed legacy multisig address is asserted equal to the address
  in the mnemonics file. If signer order or threshold is wrong, the script
  aborts the target instead of broadcasting nonsense.
- The funder key uses the operator's main keyring (separate `--funder-*`
  flags). It is never imported into the ephemeral keyring.
- All on-chain queries are routed through the existing `lumerad_q` /
  `auth_pubkey_type` / `wait_for_tx` / `assert_broadcast_accepted` helpers
  from `evmigration-common.sh`.
- **Mainnet guard.** `execute` refuses chain-ids matching `lumera-mainnet*` or
  `lumera-1` unless the operator opts in via `LUMERA_BATCH_MAINNET_OK=i-understand`.
  This driver is currently scoped to testnet/devnet; remove this guard only
  after a dedicated mainnet-hardening pass (durable run log, per-target
  confirmation, etc.).

## Exit codes

| code | meaning |
|---|---|
| 0  | all targets succeeded (or were already migrated) |
| 1  | at least one target failed |
| 2  | fatal RPC / config error before processing any target |
| 9  | mnemonics file is structurally invalid OR references unknown signer pubkeys |
