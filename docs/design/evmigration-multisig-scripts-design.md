# EVM Multisig Migration Helper Script — Design

**Status**: Draft
**Owner**: evmigration team
**Scope**: A single bash helper, `scripts/migrate-multisig.sh`, with four subcommands that wrap the `lumerad tx evmigration {generate-proof-payload, sign-proof, combine-proof, submit-proof}` flow with safety rails.

---

## 1. Purpose

Multisig legacy account migration on Lumera is an offline, coordinator-driven ceremony spanning at least K+1 machines (one coordinator plus K co-signers). The existing `lumerad` subcommands already implement the cryptography; this script layers the same kind of operator rails the single-sig `migrate-account.sh` / `migrate-validator.sh` scripts provide — pre-flight classification, file-integrity checks, post-broadcast verification — onto each of the four steps.

Prerequisite reading:

- [evmigration-multisig-design.md](evmigration-multisig-design.md) — architectural reference for the proof format, partial files, and keeper-side verification.
- [evmigration-scripts-design.md](evmigration-scripts-design.md) — the single-sig script pair this design extends.
- [migration-scripts.md](../evm-integration/user-guides/migration-scripts.md) — current user guide this design will update.

Audience: multisig coordinators and co-signers operating from a terminal; power-user holders of K-of-N accounts that migrated their funds into a multisig at some point during the legacy chain's lifetime.

## 2. Non-goals

- Orchestrating the multi-machine ceremony itself — transport of partial files between signers is still the operator's problem (email, shared drive, ticketing system, whatever).
- Replacing the raw `lumerad tx evmigration` subcommands — the script is a wrapper, not a reimplementation. `lumerad` remains the canonical entry point; the scripts only add pre/post checks.
- Single-sig migration — that flow stays in `migrate-account.sh` / `migrate-validator.sh`. Multisig runs the dual-key signature scheme ([evmigration-multisig-design.md](evmigration-multisig-design.md) §3.2) which is fundamentally different from the single-sig ADR-036 + EIP-191 dual proof.
- Key generation / import for the sub-keys — operators bring keys into their keyring via `lumerad keys add …` themselves.
- Generating the new EVM destination key from a mnemonic — the `--mnemonic-file` flow from the single-sig scripts does not extend to multisig because the destination is a simple eth_secp256k1 key, which the operator imports once.

## 3. CLI surface

Top-level invocation:

```text
./scripts/migrate-multisig.sh <subcommand> [subcommand-args...]
```

With `<subcommand>` one of:

| Subcommand | Who runs it | Wraps |
|---|---|---|
| `generate` | Coordinator (once) | `lumerad tx evmigration generate-proof-payload` |
| `sign` | Each of K co-signers (once each) | `lumerad tx evmigration sign-proof` |
| `combine` | Coordinator | `lumerad tx evmigration combine-proof` |
| `submit` | Coordinator | `lumerad tx evmigration submit-proof` |

Unknown subcommands, missing subcommand, or `-h`/`--help` print a subcommand index and exit 0 (help) or 1 (usage error).

### 3.1 `generate`

```text
./scripts/migrate-multisig.sh generate \
  --legacy <multisig-bech32> \
  --new    <new-eth-bech32> \
  --kind   claim|validator \
  --chain-id <id> \
  --node   <url> \
  --out    proof.json \
  [--sig-format SIG_FORMAT_CLI|SIG_FORMAT_ADR036]  # default CLI
  [--binary <path>]
```

`generate` is query-style and does not touch the local keyring. The wrapper should not accept `--keyring-backend`, `--keyring-dir`, or `--home` for this subcommand; those flags belong to `sign` and `submit`.

**Pre-flight:**

- `--chain-id` is required (empty chain-id produces silently non-verifying sub-signatures — documented footgun in [evmigration-multisig-design.md](evmigration-multisig-design.md)).
- Query `auth account <legacy>` first and inspect the pubkey. If the pubkey is nil, abort with exit code 8 and the remediation: "multisig pubkey is not seeded on-chain; submit any transaction from the multisig account first, then retry." There is no `--legacy-key` recovery path for multisig because a local key cannot provide the trusted threshold and full sub-key set.
- If the pubkey is non-nil, query `migration-estimate <legacy>` and abort with exit code 3 if `is_multisig == false` (pointing at `migrate-account.sh` / `migrate-validator.sh` instead).
- `--kind` must be `claim` or `validator`. `validator` is additionally gated: if it's selected and the estimate reports `is_validator == false`, abort with exit code 6.

**Success output:** the specified `--out` file exists and contains a valid `PartialProof` template: `kind`, `legacy_address`, `new_address`, `chain_id`, `evm_chain_id`, `payload_hex`, `multisig.threshold`, `multisig.sub_pub_keys_b64`, `multisig.sig_format`, and an empty `partial_signatures` array.

### 3.2 `sign`

```text
./scripts/migrate-multisig.sh sign <proof-or-partial.json> \
  --from <my-sub-key> \
  --chain-id <id> \
  --out my-partial.json \
  [--keyring-backend <b>] [--keyring-dir <dir>] [--home <dir>] [--binary <path>]
```

`<proof-or-partial.json>` can be either the coordinator-produced template OR another co-signer's partial file (the sign operation is idempotent per the spec).

**Pre-flight:**

- Input file exists and parses as JSON.
- Validate `payload_hex` against a canonical reconstruction from the other fields. Abort exit 9 on mismatch (catches tampering or mistakenly-edited fields).
- Extract `multisig.sub_pub_keys_b64` from the file, then resolve `--from`'s pubkey from the keyring and confirm it matches one of the listed sub-key pubkeys. Abort exit 1 if the signer isn't in the set (catches "wrong key" mistakes early, which otherwise only surface at `combine` time when the keeper-side pubkey verification fails).

**Success output:** `--out` contains a partial with the signer's `{index, signature_b64}` entry appended. The `partial_signatures` array is idempotent — re-running `sign` with the same `--from` replaces the existing entry for that signer index.

### 3.3 `combine`

```text
./scripts/migrate-multisig.sh combine <partial1.json> <partial2.json> [...] \
  --out tx.json \
  [--binary <path>]
```

No `--node` or `--chain-id` — `combine` is pure local file assembly.

**Pre-flight:**

- Each input file exists and parses.
- Cross-file consistency: every partial must agree on `chain_id`, `evm_chain_id`, `legacy_address`, `new_address`, `payload_hex`, kind, `multisig.threshold`, `multisig.sig_format`, and the `multisig.sub_pub_keys_b64` list.
- The underlying `lumerad combine-proof` verifies signatures and only assembles the first K valid signatures in ascending signer-index order. The wrapper's before-invocation summary is intentionally only an **entry-presence** summary, not a cryptographic validity verdict:

  ```text
  Partial signature entries (3-of-5 required):
    [X] signer 0  lumera1sub0...  (alice-partial.json)
    [X] signer 1  lumera1sub1...  (bob-partial.json)
    [ ] signer 2  lumera1sub2...  (missing)
    [X] signer 3  lumera1sub3...  (carol-partial.json)
    [ ] signer 4  lumera1sub4...  (missing)
  Entry threshold satisfied: yes (3 >= 3)
  ```

- If fewer than K partial-signature entries are present, abort exit 4 before invoking `lumerad`. If K entries are present but fewer than K signatures are cryptographically valid, invoke `lumerad`, surface its `need <K> valid partial signatures, have <N>` error, and exit 4.

**Success output:** `--out` contains the assembled unsigned tx with the first K valid partial signatures in ascending signer order.

### 3.4 `submit`

```text
./scripts/migrate-multisig.sh submit <tx.json> \
  --from <new-eth-key> \
  --chain-id <id> \
  --node <url> \
  [--keyring-backend <b>] [--keyring-dir <dir>] [--home <dir>] [--binary <path>]
  [--yes] [--dry-run] [--i-have-stopped-the-node]
```

**Pre-flight (identical in spirit to `migrate-account.sh`'s happy path):**

- `<tx.json>` exists and parses. Extract `legacy_address` and `new_address` from its embedded proof.
- Resolve `--from`'s address and verify it matches `new_address` (operators running submit with the wrong destination key is a common failure mode).
- Verify `--from` is `eth_secp256k1` — reject other algorithms with exit 1 and a clear message.
- Run `assert_not_migrated <legacy>` and `assert_new_address_unused <new>` (shared with single-sig flow).
- **Re-run `assert_estimate_succeeds` against a fresh `migration-estimate <legacy>` query.** The ceremony between `generate` and `submit` may span hours or days; chain state can shift under it (governance disables migration via `enable_migration=false`, `migration_end_time` passes, validator accumulates delegations past `max_validator_delegations`). Re-checking catches those cases before burning a broadcast attempt. Exits 4 with the current `rejection_reason` if the estimate no longer succeeds.
- `snapshot_bank_balances <legacy>`.
- Print a confirmation banner listing: legacy address, new address, kind (claim or validator), and K-of-N multisig info. If kind is `validator`, include the same downtime warning + typed-`yes` acknowledgement as `migrate-validator.sh` (and the same `--i-have-stopped-the-node` escape hatch).

**Post-broadcast:**

- `wait_for_tx`, then `verify_migration <legacy> <new> <snapshot>` — same semantics as the single-sig scripts (migration record must exist with matching `new_address`, legacy balances must be zero, new balances must meet-or-exceed the pre-broadcast snapshot per denom).

`--dry-run` exits 0 after pre-flight, before broadcast.

## 4. Shared library extensions

New functions added to `scripts/evmigration-common.sh`:

| Function | Purpose |
|---|---|
| `assert_multisig <estimate-json>` | Inverse of `assert_single_sig`. If `is_multisig == false`, abort exit 3 with a pointer to `migrate-account.sh` / `migrate-validator.sh` |
| `auth_account_json <addr>` | Cached `lumerad_q auth account <addr>` wrapper returning JSON |
| `auth_pubkey_type <addr>` | Returns one of `none` (nil), `single-sig`, `multisig`, or `unknown` based on the `.account.pub_key."@type"` field |
| `read_proof_file <path>` | Reads and validates a proof or partial JSON file. Validates required fields (`kind`, `legacy_address`, `new_address`, `chain_id`, `evm_chain_id`, `payload_hex`, `multisig.threshold`, `multisig.sub_pub_keys_b64`, `multisig.sig_format`, `partial_signatures`), verifies `payload_hex` matches canonical reconstruction from the other fields, confirms `multisig.threshold` and the `sub_pub_keys_b64` list length are internally consistent. Emits the JSON on stdout, human summary on stderr. Fails exit 9 on any violation |
| `summarize_partials <files...>` | Parses all inputs, prints the K-of-N signed matrix shown in §3.3, returns 0 if threshold satisfied, non-zero otherwise |
| `assert_eth_key <key-name>` | Confirms the given key in the keyring is `eth_secp256k1` (matching the algorithm the EVM path uses) |

All new functions follow the existing style (short, composable, fail-closed, one responsibility each).

## 5. Updates to existing scripts

`migrate-account.sh` and `migrate-validator.sh` currently exit 3 on multisig with a message pointing at `legacy-migration.md`. Change the message to also point at `./scripts/migrate-multisig.sh`:

```text
ERROR legacy account is a K-of-N multisig; use scripts/migrate-multisig.sh for the offline 4-step flow
ERROR see docs/evm-integration/user-guides/migration-scripts.md#multisig
```

## 6. Exit codes

Preserves the stable scheme from the single-sig scripts, plus two new codes:

| Code | Meaning |
|---|---|
| `0` | Success, or dry-run completed |
| `1` | Usage error: wrong subcommand, bad flags, key algorithm mismatch, key not in multisig set |
| `2` | Environment error: binary/jq missing, RPC query failure |
| `3` | Account is NOT multisig (wrong tool — single-sig scripts apply) |
| `4` | Pre-flight failed: `would_succeed=false`, OR partial-signatures below threshold |
| `5` | Already migrated (same as single-sig) |
| `6` | Wrong `--kind` (`validator` on non-validator) |
| `7` | Post-migration verification failed (same as single-sig) |
| `8` | *(new)* Multisig pubkey not seeded on-chain; seed it with any multisig-signed tx first |
| `9` | *(new)* Input file integrity check failed (payload_hex mismatch, JSON parse error, or cross-file inconsistency in `combine`) |
| `10` | User aborted at a confirmation prompt |

## 7. File layout

```text
scripts/
├── evmigration-common.sh          # extended with §4 helpers
├── migrate-account.sh             # message update only (§5)
├── migrate-validator.sh           # message update only (§5)
└── migrate-multisig.sh            # NEW — subcommand dispatcher, §3
```

The script is a single file so that the subcommand dispatch logic lives in one place. Each subcommand's main logic is factored into a function (`_mms_generate`, `_mms_sign`, `_mms_combine`, `_mms_submit`) for readability and testability.

## 8. Testing strategy

### 8.1 Unit tests (bats)

`tests/scripts/migrate-multisig.bats` with end-to-end shim-driven tests covering:

- `generate` happy path (`is_multisig` true, pubkey seeded) → proof.json written
- `generate` rejects single-sig (exit 3)
- `generate` rejects when chain-id unset (exit 1)
- `generate` rejects keyring-specific flags (`--keyring-backend`, `--keyring-dir`, `--home`) as usage errors (exit 1)
- `generate` exits 8 when the on-chain pubkey is nil and prints the "seed the multisig pubkey on-chain first" remediation
- `sign` rejects a partial with tampered `payload_hex` (exit 9)
- `sign` rejects when `--from` is not in the sub-key set (exit 1)
- `sign` happy path produces a partial file
- `combine` prints the K-of-N entry-presence summary and aborts at 2-of-3 below a 3-threshold (exit 4)
- `combine` maps fewer-than-K valid signatures from `lumerad combine-proof` to exit 4 even when K entries are present
- `combine` happy path assembles tx.json
- `submit` happy path — pre-flight + broadcast + verify, exits 0
- `submit` rejects when `--from` address doesn't match tx's `new_address` (exit 1)
- `submit` aborts with exit 4 when `migration-estimate` flips to `would_succeed=false` between `generate` and `submit` (simulate via `SHIM_ESTIMATE_FIXTURE=estimate-rejected`)
- `submit` validator path requires typed downtime acknowledgement or `--i-have-stopped-the-node` (exit 10 on refusal)
- `submit` dry-run exits 0 without broadcasting

### 8.2 Shim extensions

`tests/scripts/fixtures/lumerad-shim.sh` adds:

- Routes for `tx evmigration generate-proof-payload *`, `tx evmigration sign-proof *`, `tx evmigration combine-proof *`, `tx evmigration submit-proof *` — each emits a corresponding fixture (`proof-template.json`, `partial-bob.json`, `combined-tx.json`, `broadcast-success.json`).
- Multisig auth-account fixture (`auth-account-multisig.json`) with `LegacyAminoPubKey` type and three sub-keys.
- Nil-pubkey auth-account fixture (`auth-account-nilpubkey.json`) with `"pub_key": null`.
- Per-command fixture env var `SHIM_AUTH_FIXTURE` (already exists from Task 4); extend with a `SHIM_AUTH_TYPE=multisig|nilpubkey|single` router for readability.

### 8.3 Integration (devnet matrix — manual)

Acceptance test against `make devnet-new`: fund a 2-of-3 multisig account, run the four subcommands on separate shells, confirm migration record appears and balances moved.

## 9. Documentation updates

- [docs/evm-integration/user-guides/migration-scripts.md](../evm-integration/user-guides/migration-scripts.md) — add a new top-level section "Multisig migration" covering the four subcommands, preflight messages, exit codes 8 and 9, and a walkthrough of a 2-of-3 ceremony with three terminal windows.
- [docs/evm-integration/user-guides/migration.md](../evm-integration/user-guides/migration.md) — the existing "Migrating a multisig account" section gets a pointer to `migrate-multisig.sh` at the top, before the raw-CLI walkthrough. The raw-CLI walkthrough stays as the canonical reference.
- [Makefile](../../Makefile) — `release` target already copies all `scripts/*.sh` to the tarball via an explicit list; extend the list with `migrate-multisig.sh`.
- No changes to [evmigration-multisig-design.md](evmigration-multisig-design.md) — that doc describes the wire format and keeper logic, which the scripts wrap but don't change.

## 10. Out of scope / explicit deferrals

- **Partial-file transport**: this design assumes operators move partial files between signers via existing tooling. A future enhancement could add a pastebin-style helper that e.g. base64-encodes and prints a QR code, but that's a separate effort.
- **Key orchestration**: operators still `lumerad keys add` each sub-key by hand into their machine's keyring before invoking `sign`. The script doesn't own keyring lifecycle.
- **Threshold UX beyond the summary matrix**: operators manually collect partials and pass them all to `combine`. A future enhancement could auto-detect partials in a directory, but that's speculative.
- **Recovery from partially-submitted ceremonies**: if `combine` succeeds but `submit` fails mid-broadcast, the next `submit` works identically on the already-assembled `tx.json`. No new state machine needed.
