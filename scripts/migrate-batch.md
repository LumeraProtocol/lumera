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

## Prerequisites

On the machine where you run the script:

- `lumerad` built from a **post-EVM-upgrade** commit. Verify with
  `lumerad query evmigration --help` — it must list `migration-record`,
  `migration-estimate`, etc.
- `bash` 4.4 or newer.
- `jq` on `PATH`.
- Access to a Lumera RPC endpoint. Default is `tcp://localhost:26657`.
  Pass `--node <url>` to point elsewhere.
- An **operator keyring** containing the `--funder` key (see below). This
  is your main `lumerad` keyring — `--keyring-backend test|file|os` are all
  supported via the `--funder-keyring-{backend,dir,home}` flags. Default
  backend is `test` and default location is `lumerad`'s default home.

You can run the script from any host that satisfies the above. There is no
requirement that it run on a validator node; running on a dedicated ops box
with read-only RPC access to a full node is the recommended pattern.

## Picking and preparing the funder

`--funder <key>` is the operator-keyring key that pays fees for every target
classified `needs-funding` (see § Subcommands → status below for what that
means). Three things to verify before kicking off `execute`:

1. **Key is in the operator keyring**, not the ephemeral one. Confirm with
   `lumerad keys show <funder-key> --keyring-backend <backend>`.
2. **Spendable balance is sufficient.** The funder needs roughly
   `N_needs_funding × top_up_amount + N_needs_funding × broadcast_fee`.
   For the mainnet foundation bundle (~24 vesting-locked multisigs) at the
   default `200000ulume` top-up that's < 0.01 LUME total, but check anyway.
   A vesting-locked key **cannot** be used as a funder — it has spendable=0.
3. **Key type is supported.** Both legacy `secp256k1` and EVM
   `eth_secp256k1` funder keys work; the script broadcasts a plain `MsgSend`
   from it.

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
- `bank balances` and `bank spendable-balances` (does the account hold ulume
  it can actually spend on the pubkey-publishing self-send amount plus fees?)

and prints one of:

- `migrated`      — already migrated, will be skipped by `execute`.
- `ready`         — pubkey on chain, ready to migrate. No self-send needed.
- `needs-pubkey`  — has **spendable** balance but no pubkey on chain. Will
                    self-send to publish the pubkey, then migrate.
- `needs-funding` — pubkey missing AND spendable balance is below the
                    self-send amount plus fee threshold. Covers **two** real cases:
                    (a) zero total balance (classic fresh foundation
                    account), and (b) non-zero total balance but
                    **vesting-locked** so spendable cannot cover the
                    self-send amount plus fee. Both require `--funder`
                    during `execute`.
- `unknown`       — RPC failure (re-check connectivity / endpoint).

`status` surfaces both `balance` and `spendable` per target so an operator
can verify the classification at a glance: a multisig holding 5T LUME but
classified `needs-funding` will show `spendable=0` next to it.

### `execute`  (the real thing)

For each target, runs the full lifecycle. Read the in-file docs for the exact
state machine.

Common flags:

| flag | meaning |
|---|---|
| `--target <name>` | Process only the named target. Use this for the first run. |
| `--funder <key>` | Operator-keyring key that pays fees for **any** target classified `needs-funding`, including vesting-locked accounts whose total balance is large but spendable is zero. Must have spendable balance ≥ `N_needs_funding × (--top-up-amount + broadcast-fee)`. Required whenever any target is `needs-funding`; the script aborts that target otherwise. |
| `--top-up-amount <coins>` | How much the funder sends to each `needs-funding` target. Default `200000ulume`. Sizing rule: must cover the downstream self-send (`100000ulume`) + its broadcast fee (`5000ulume`) with comfortable headroom (default leaves ~95000ulume slack). If you've overridden the self-send amount or fees in the script, set this to at least `2 × (self_send_amount + self_send_fee)` and never less than `200000ulume`. |
| `--funder-keyring-{backend,dir,home}` | How to reach the funder key. Defaults: `test` backend, `lumerad`'s default home / keyring dir. Example: `--funder-keyring-backend file --funder-keyring-home /etc/lumerad`. |
| `--log-file <path>` | Append one JSONL audit record per lifecycle milestone (batch_start, target_start, classify, keyring_setup, reconstructed, funding_*, self_send_*, ceremony_start, target_done, batch_done). `classify` events include both `balance` and `spendable`. Mode 0600 on create, append-only, correlated by per-run `batch_id`. Operator handles rotation. |
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

The first four steps are warm-ups; only step 5 broadcasts at scale. Pick
**two** dry-run targets in step 3 — one vested (likely `needs-pubkey`) and
one vesting-locked (likely `needs-funding`) — so you exercise both code
paths before the full batch.

```bash
# 1. Offline: confirm the file parses and classifies cleanly.
./scripts/migrate-batch.sh report \
  --mnemonics output.json \
  --plan-out plan.json

# 2. Read-only: see what each target's on-chain state is.
#    The output table shows balance AND spendable. Eyeball the summary
#    counts and confirm needs-funding count matches your expectation
#    (typically: every still-vesting multisig + every fresh zero-balance
#    account).
./scripts/migrate-batch.sh status \
  --mnemonics output.json \
  --chain-id lumera-mainnet-1 \
  --node tcp://localhost:26657

# 3a. Dry-run a VESTED target end-to-end (no broadcasts).
#     Exercises the no-funder self-send path.
./scripts/migrate-batch.sh execute \
  --mnemonics output.json \
  --target seed_sale_1 \
  --chain-id lumera-mainnet-1 \
  --funder ops-funder --top-up-amount 200000ulume \
  --dry-run

# 3b. Dry-run a VESTING-LOCKED target end-to-end (no broadcasts).
#     Exercises the --funder + self-send path. Pick any target the status
#     output showed as needs-funding with balance>0 (e.g. team_3, advisors_1).
./scripts/migrate-batch.sh execute \
  --mnemonics output.json \
  --target team_3 \
  --chain-id lumera-mainnet-1 \
  --funder ops-funder --top-up-amount 200000ulume \
  --dry-run

# 4. Execute one target for real, then expand.
./scripts/migrate-batch.sh execute \
  --mnemonics output.json \
  --target seed_sale_1 \
  --chain-id lumera-mainnet-1 \
  --funder ops-funder --top-up-amount 200000ulume

# 5. Full batch (with confirmation prompt; --log-file is strongly recommended).
./scripts/migrate-batch.sh execute \
  --mnemonics output.json \
  --chain-id lumera-mainnet-1 \
  --funder ops-funder --top-up-amount 200000ulume \
  --log-file ./batch-$(date -u +%Y%m%dT%H%M%SZ).jsonl \
  --continue-on-error
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
