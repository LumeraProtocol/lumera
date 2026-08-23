# EVM Account Migration

The EVM integration changes the default coin type from 118 (`secp256k1`) to 60 (`eth_secp256k1`). Existing accounts derived with coin type 118 produce different addresses than the same mnemonic with coin type 60. The `x/evmigration` module provides an atomic claim-and-move mechanism for migrating on-chain state from legacy to new addresses.

## Documentation

| Document | Description |
| --- | --- |
| [legacy-migration.md](legacy-migration.md) | `x/evmigration` module architecture, messages, params, migration sequence, queries, and implementation status |
| [migration.md](../user-guides/migration.md) | Step-by-step migration guide for end users (Portal + Keplr and CLI methods) |
| [portal-ui.md](portal-ui.md) | EVM Migration Portal UI and wallet rollout |
| [devnet-tests.md](devnet-tests.md) | `tests_evmigration` devnet end-to-end test tool (modes, module coverage, Makefile targets) |

## Overview

When Lumera upgrades to EVM support (v1.20.0), the same mnemonic produces a **different on-chain address** under coin type 60. The `x/evmigration` module provides `MsgClaimLegacyAccount` and `MsgMigrateValidator` transactions that atomically transfer all state from the old address to the new one.

### What gets migrated

| Module | State migrated |
| --- | --- |
| `x/auth` | Account record (vesting-aware) |
| `x/bank` | All coin balances |
| `x/staking` | Delegations, unbonding, redelegations + UnbondingID indexes |
| `x/distribution` | Reward withdrawal, delegator starting info |
| `x/authz` | Grant re-keying (both grantor and grantee) |
| `x/feegrant` | Fee allowance re-keying (both granter and grantee) |
| `x/supernode` | SupernodeAccount, Evidence, PrevSupernodeAccounts, MetricsState |
| `x/action` | Creator and SuperNodes fields in action records |
| `x/evmigration` | Core migration logic, dual-signature verification, rate limiting |

### Key design decisions

- **Dual-signature verification** -- migration requires signatures from both the legacy key and the new EVM key, proving ownership of both addresses
- **Zero-fee migration** -- a custom ante decorator waives gas fees for migration txs since the new address has no balance before migration completes
- **Rate limiting** -- `max_migrations_per_block` (default 50) prevents migration flood attacks
- **Validator migration** -- dedicated `MsgMigrateValidator` handles the additional complexity of re-keying validator records, delegator references, and consensus key mappings
- **Claim records are not re-keyed** -- `x/claim` records are left untouched during migration. The claim DB is frozen (claiming ended 2025-01-01) and retained for reference only; re-keying `DestAddress` was cosmetic (funds already moved during the claim period). Scanning the ~18K-record claim store per migration was unbounded gas with no functional benefit, so it was removed. The legacy-to-new mapping lives in `MigrationRecords` and can reconstruct claim linkage offline if ever needed.

See [legacy-migration.md](legacy-migration.md) for the full module reference.

---

## Multisig account migration

When the legacy account is a K-of-N Cosmos multisig (a `LegacyAminoPubKey` recorded on the account's `BaseAccount.PubKey`), the migration target is **also** a K-of-N multisig, but constructed from `eth_secp256k1` sub-keys.

> **Consensus invariants (multisig).** These are enforced at `ValidateBasic` — before any crypto verification runs, and before the tx reaches the msg server. A violation rejects the transaction on-chain.
>
> - **Shape + K/N must mirror.** A K-of-N legacy multisig migrates to a K-of-N `eth_secp256k1` multisig. Different K, different N, or single↔multisig shape mismatch is rejected with `ErrMirrorSourceMismatch` (code 1121).
> - **Same K signer positions sign both halves.** `legacy_proof.signer_indices` must equal `new_proof.signer_indices`. Two disjoint K-subsets can't each authorize one side; a co-signer who signs only one side doesn't contribute toward the K-of-K threshold on the other.
> - **Sub-key uniqueness per side.** Each side's `sub_pub_keys` must have pairwise-distinct entries. A duplicate would silently reduce effective K; rejected with `ErrInvalidMigrationPubKey`.
> - **Zero-signer submit.** `submit-proof` carries no `--from`, no fee, no envelope signature. Authorization is the two proofs themselves; the evmigration ante handler waives fees.
>
> The CLI `combine-proof` mirrors these rules — it intersects valid signer-index sets across sides before selecting K, so a tx file it writes always satisfies `ValidateBasic`. Ground truth and error codes: [legacy-migration.md § Consensus invariants](legacy-migration.md#consensus-invariants).

| Legacy shape | New shape |
| --- | --- |
| Single `secp256k1` EOA | Single `eth_secp256k1` EOA |
| K-of-N Cosmos multisig | K-of-N multisig of `eth_secp256k1` sub-keys (same K and same N) |

The threshold and fan-out (K and N) are preserved across the migration. What changes is the sub-key algorithm on each side:

- **Legacy side**: every sub-key must be `secp256k1` (Cosmos, coin-type 118). Non-secp256k1 sub-keys cause `MigrationEstimate` to return `would_succeed=false`.
- **New side**: every sub-key must be `eth_secp256k1` (Ethereum, coin-type 60). The destination multisig address is derived from `kmultisig.NewLegacyAminoPubKey(K, subs)` over the new eth sub-keys — it is **not** an EVM 20-byte address, and cannot originate `MsgEthereumTx`.

### Four-step CLI walkthrough

The full flow uses four `lumerad tx evmigration` subcommands. Flag names below match `x/evmigration/client/cli/tx_multisig.go`.

**Step 1 — Each co-signer generates a fresh eth sub-key; coordinator derives the new multisig address.**

```bash
# Each co-signer, on their own machine:
lumerad keys add <op-name>-eth-<N> --key-type eth_secp256k1 \
  --keyring-backend <backend>

# Coordinator, once all N eth pubkeys are available:
lumerad keys add <op-name>-msig-new \
  --multisig <op-name>-eth-1,<op-name>-eth-2,<op-name>-eth-3 \
  --multisig-threshold 2 \
  --keyring-backend <backend>

lumerad keys show <op-name>-msig-new --address
# lumera1...   <-- the new multisig bech32; this is the new_address
```

**Step 2 — Coordinator generates the proof payload template.**

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

- `--new-sub-pub-keys` accepts either keyring key names (resolved locally) or base64-encoded compressed 33-byte `eth_secp256k1` pubkeys. Mix freely.
- `--new-threshold` is **required** whenever `--new-sub-pub-keys` is used.
- `--kind claim` targets `MsgClaimLegacyAccount`; use `--kind validator` for `MsgMigrateValidator`.
- `generate-proof-payload` does not broadcast anything, but it **does** need keyring access (to resolve `--new-sub-pub-keys` / `--legacy-key` entries that are local key names). Pass `--keyring-backend` (and `--keyring-dir` / `--home` when applicable).

Distribute `proof.json` to all K+ co-signers.

**Step 3 — Each co-signer signs `proof.json` on both sides in a single invocation.**

```bash
lumerad tx evmigration sign-proof proof.json \
  --from <my-legacy-sub-key-name> \
  --new-key <my-eth-sub-key-name> \
  --keyring-backend <backend> \
  --chain-id <chain-id> \
  --out <signer>-partial.json
```

Each co-signer must hold **both** their legacy Cosmos sub-key (`--from`) **and** their destination-side eth sub-key (`--new-key`) in the same keyring. `sign-proof` produces one partial for the legacy side and one for the new side in a single file; re-running replaces the signer's prior entries on both sides (idempotent).

**Step 4 — Coordinator combines partials, then submits the assembled tx.**

```bash
lumerad tx evmigration combine-proof \
  alice-partial.json bob-partial.json carol-partial.json \
  --out tx.json

lumerad tx evmigration submit-proof tx.json \
  --chain-id <chain-id>
```

`combine-proof` verifies every partial on both sides, drops invalid entries with a stderr warning, then **intersects** the valid signer-index sets across the two sides and selects the first K indices present on BOTH. This is what makes `legacy_proof.signer_indices == new_proof.signer_indices` (the consensus mirror-source rule). A co-signer who signs only one side doesn't contribute toward quorum unless another co-signer supplies the other side's signature at the same index. If the intersection has fewer than K entries, combine-proof errors out with `need <K> valid partial signatures signed on BOTH sides at matching indices, have <N>` and writes nothing.

`submit-proof` does **not** sign at the Cosmos layer. Migration messages declare zero signers (authorization is fully embedded in `legacy_proof` and `new_proof`), fees are waived by the evmigration ante handler, and replay is prevented by the keeper's `MigrationRecords.Has(legacyAddr)` check. There is no `--from`, no fee-payer, and no envelope signature — `submit-proof` just loads `tx.json`, runs `ValidateBasic`, simulates gas via the migration-specific estimator, builds an unsigned tx, and broadcasts.

### `PartialProof` v2 JSON schema

`PartialProof` is the on-disk coordination artifact passed between co-signers; it is never stored on-chain. The v2 shape is:

```json
{
  "version": 2,
  "kind": "claim",
  "legacy_address": "lumera1...",
  "new_address": "lumera1...",
  "chain_id": "lumera-mainnet-1",
  "evm_chain_id": 76857769,
  "payload_hex": "6c756d...",
  "legacy": {
    "threshold": 2,
    "sub_pub_keys": ["<base64>", "<base64>", "<base64>"],
    "sig_format": "SIG_FORMAT_CLI"
  },
  "new": {
    "threshold": 2,
    "sub_pub_keys": ["<base64>", "<base64>", "<base64>"],
    "sig_format": "SIG_FORMAT_CLI"
  },
  "partial_legacy_signatures": [
    { "index": 0, "signature": "<base64>" }
  ],
  "partial_new_signatures": [
    { "index": 0, "signature": "<base64>" }
  ]
}
```

For single-key sides, the `SideSpec` uses `pub_key` (base64 of the 33-byte compressed pubkey) with `threshold` and `sub_pub_keys` omitted. For multisig sides, `pub_key` is omitted and `threshold` + `sub_pub_keys` are set.

Ground truth: [`x/evmigration/client/cli/tx_multisig.go:53-100`](../../../x/evmigration/client/cli/tx_multisig.go#L53-L100).

### Gotchas

- **Co-signer dual key requirement.** Each participating co-signer must hold both their legacy Cosmos sub-key **and** their destination-side eth sub-key in the same keyring when they run `sign-proof`. A co-signer who only holds one side cannot produce a useful partial — `sign-proof` signs both sides in one invocation.
- **Nil-pubkey legacy accounts.** If the multisig has received funds but never broadcast a transaction, its `LegacyAminoPubKey` is absent from `BaseAccount.PubKey` and `generate-proof-payload` has nothing to attest. Submit any trivial transaction from the multisig (e.g. a 1-`ulume` self-send) before starting the migration, then confirm via `lumerad query auth account <multisig-legacy-address>` that the response shows a `multisig` pubkey structure listing all N sub-keys.
- **Non-EVM-addressable new operator.** The new multisig address is a Cosmos SDK bech32 derived from `kmultisig.NewLegacyAminoPubKey`. It can perform all Cosmos-side operations (staking, supernode, x/authz grants, IBC transfer) but **cannot** originate `MsgEthereumTx`. Operators who want EVM DeFi access for rewards should configure a separate single-EOA withdraw address via `MsgSetWithdrawAddress`.

## Migration order — FAQ

**Q: Do we need to migrate the multisig before its individual co-signers migrate their personal accounts? Or after?**

A: **Any order works, including interleaved.** This holds uniformly for every multisig migration scenario — a balance-holding multisig, a validator operator multisig, and a multisig-operated supernode. Sub-signer and multisig migrations are mutually independent because:

- The multisig's `LegacyAminoPubKey` — containing every sub-signer's 33-byte compressed pubkey and the threshold — is stored inline on the *multisig's* own `BaseAccount.PubKey`. Removing a sub-signer's individual account from x/auth (via their personal migration) does not touch this record.
- Signing is an offline private-key operation. Each co-signer's `lumerad tx evmigration sign-proof --from <legacy-sub-key>` produces a signature from their local keyring. The keyring's private key exists independently of any chain state, so it continues to work after the sub-signer's personal account has been migrated.
- The on-chain verifier reconstructs the multisig from pubkey bytes in the proof and verifies each sub-signature against the claimed sub-pubkey. It never consults x/auth about the sub-signers' individual account existence.

**Precondition (unchanged):** the multisig's own `LegacyAminoPubKey` must already be on-chain — i.e., the multisig must have signed at least one transaction in the past. If the multisig received funds but never signed anything, submit any 1-ulume self-send from the multisig first so its pubkey gets recorded. This precondition is independent of sub-signer migration state.

**Non-migrating sub-signers:** if a co-signer chooses never to migrate their own personal account, the multisig migration still succeeds as long as K of N co-signers participate in the sign-proof ceremony.

**Implication for planning:** operators can migrate in whatever order is operationally simplest — e.g., every co-signer migrates their personal account on their own schedule, and the multisig migration happens whenever K of N can coordinate. There is no chain-level ordering constraint.

This property applies to all three migration message types:
- `MsgClaimLegacyAccount` (balance-holding multisig)
- `MsgMigrateValidator` (validator operator multisig — `x/staking` delegations, `x/distribution` state, `x/supernode` records all key on the multisig bech32, not sub-signers)
- `MsgClaimLegacyAccount` / `MsgMigrateValidator` for supernode-operator multisigs (the cleanup flow described in the supernode user guide keys on the multisig's on-chain pubkey, set by `MigrateAuth`)
