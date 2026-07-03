# Legacy Account Migration (`x/evmigration`)

The EVM integration changes coin type from 118 (`secp256k1`) to 60 (`eth_secp256k1`). Existing accounts derived with coin type 118 produce different addresses than the same mnemonic with coin type 60. The `x/evmigration` module provides a claim-and-move mechanism: users submit `MsgClaimLegacyAccount` signed by both old and new keys, atomically migrating on-chain state.

Module structure

```text
x/evmigration/
  keeper/
    keeper.go                      # Keeper struct, 9 external keeper deps
    msg_server.go                  # MsgServer wrapper
    msg_server_claim_legacy.go     # MsgClaimLegacyAccount handler
    msg_server_migrate_validator.go # MsgMigrateValidator handler (Phase 5)
    verify.go                      # Dual-signature verification
    migrate_auth.go                # Account record migration (vesting-aware)
    migrate_bank.go                # Coin balance transfer
    migrate_distribution.go        # Reward withdrawal
    migrate_staking.go             # Delegation/unbonding/redelegation re-keying
    migrate_authz.go               # Grant re-keying
    migrate_feegrant.go            # Fee allowance re-keying
    migrate_supernode.go           # Supernode account field update
    migrate_action.go              # Action creator/supernode update
    migrate_claim.go               # Claim destAddress update
    migrate_validator.go           # Validator record re-key (Phase 5)
    query.go                       # gRPC query stubs
    genesis.go                     # InitGenesis/ExportGenesis
  types/
    keys.go, errors.go, params.go, events.go, expected_keepers.go, codec.go
  module/
    module.go, depinject.go, autocli.go
```

### Messages

| Message                   | Signer                          | Purpose                           |
| ------------------------- | ------------------------------- | --------------------------------- |
| `MsgClaimLegacyAccount` | `new_address` (eth_secp256k1) | Migrate regular account state     |
| `MsgMigrateValidator`   | `new_address` (eth_secp256k1) | Migrate validator + account state |
| `MsgUpdateParams`       | governance authority            | Update migration params           |

### Params

| Param                         | Default  | Description                            |
| ----------------------------- | -------- | -------------------------------------- |
| `enable_migration`          | `true` | Master switch                          |
| `migration_end_time`        | `0`    | Unix timestamp deadline                |
| `max_migrations_per_block`  | `50`   | Rate limit                             |
| `max_validator_delegations` | `2500` | Max delegators for validator migration |

### Fee waiving

`ante/evmigration_fee_decorator.go` waives gas fees for migration txs (new address has zero balance before migration). Wired in `app/evm/ante.go` after `DelayedClaimFeeDecorator`.

### Migration sequence (MsgClaimLegacyAccount)

1. Pre-checks (params, window, rate limit, dual-signature verification).
   Legacy signature is`secp256k1_sign(SHA256("lumera-evm-migration:<legacy_address>:<new_address>"))`
2. Withdraw distribution rewards → legacy bank balance
3. Re-key staking (delegations, unbonding, redelegations + UnbondingID indexes)
4. Migrate auth account (vesting-aware: remove lock before bank transfer)
5. Transfer bank balances
6. Finalize vesting account at new address (if applicable)
7. Re-key authz grants
8. Re-key feegrant allowances
9. Update supernode account field
10. Update action creator/supernode references
11. Update claim destAddress
12. Store MigrationRecord, increment counters, emit event

### Queries

| Query                 | Description                         |
| --------------------- | ----------------------------------- |
| `Params`            | Current migration parameters        |
| `MigrationRecord`   | Single legacy address lookup        |
| `MigrationRecords`  | Paginated list of all records       |
| `MigrationEstimate` | Dry-run estimate of migration scope |
| `MigrationStats`    | Aggregate counters                  |
| `LegacyAccounts`    | Accounts needing migration          |
| `MigratedAccounts`  | Completed migrations                |

### Implementation status

| Phase | Description                     | Status      |
| ----- | ------------------------------- | ----------- |
| 1     | Proto + Types + Module Skeleton | Complete    |
| 2     | Verification + Core Handler     | Complete    |
| 3     | SDK Module Migrations           | Complete    |
| 4     | Lumera Module Migrations        | Complete    |
| 5     | Validator Migration             | Complete    |
| 6     | Queries + Genesis               | Complete    |
| 7     | Testing                         | Complete    |

---

## Multisig account migration

Legacy accounts backed by a flat K-of-N multisig pubkey (Cosmos `multisig.LegacyAminoPubKey` with all sub-keys `secp256k1`) migrate to a **multisig-of-`eth_secp256k1`** destination with the **same K and N** — the mirror-source rule. The CLI walkthrough lives in [main.md § Multisig account migration](main.md#multisig-account-migration) and [migration-scripts.md § Multisig migration](../user-guides/migration-scripts.md#multisig-migration); this section is the keeper-side reference.

### What is supported

Flat K-of-N multisig legacy accounts where every sub-key is `secp256k1`. The verifier is `verifyMultisigProof` in `x/evmigration/keeper/verify.go`, called independently for `legacy_proof` and `new_proof`. Co-signers collect exactly K sub-signatures per side via the `generate-proof-payload` → `sign-proof` → `combine-proof` → `submit-proof` flow; the submitted tx carries two `MultisigProof`s, both validated by the keeper and compared for shape/K/N by `types.ValidateProofPair`.

### Consensus invariants

The following are enforced by `MsgClaimLegacyAccount.ValidateBasic` and `MsgMigrateValidator.ValidateBasic`; a violation is rejected before any crypto verification runs and before the tx is dispatched to the msg server.

- **Mirror-source shape rule** — `types.ValidateProofPair`. Both sides must share shape (single↔single or multisig↔multisig); when both multisig, threshold (K) and sub-key count (N) must match across sides. Rejected with `ErrMirrorSourceMismatch` (code 1121). A 2-of-3 legacy multisig cannot migrate to a 1-of-1 or 3-of-5 destination.
- **Matching `signer_indices`** — `types.ValidateProofPair`. When both sides are multisig, `legacy_proof.signer_indices` must equal `new_proof.signer_indices` element-for-element. The same K signer positions must approve both halves — two disjoint K-subsets (e.g. legacy signed by indices `[0,1]`, new signed by `[0,2]`) are rejected with `ErrMirrorSourceMismatch`. This is what makes the docs' claim "each co-signer holds both their legacy Cosmos sub-key AND their destination-side eth sub-key" a chain-enforced invariant rather than an operational convention.
- **Sub-key uniqueness** — `MultisigProof.validateBasic`. Each side's `sub_pub_keys` list must have pairwise-distinct entries. Rejected with `ErrInvalidMigrationPubKey`. Without this check, a duplicate entry would let one keyholder be counted as two distinct signers against the K-of-N threshold (effective K would silently drop).
- **Per-side sub-key typing** — `legacy_proof` sub-keys must be Cosmos `secp256k1`; `new_proof` sub-keys must be `eth_secp256k1`. The verifier dispatches on `SubKeyType` at each side.
- **Zero-signer tx** — migration messages declare no signers. Authorization is embedded in the proof bytes; fees are waived by the evmigration ante handler; replay is prevented by `MigrationRecords.Has(legacyAddr)`. `submit-proof` does **not** take `--from`, `--fees`, `--gas-prices`, or `--sign-mode`.

The CLI `combine-proof` mirrors these invariants so that a tx file it writes will satisfy `ValidateBasic` — it intersects the valid signer-index sets across the two sides before selecting K, rather than picking each side independently. If fewer than K indices have valid signatures on BOTH sides, combine-proof errors out before writing `tx.json`.

### What is NOT supported

- Nested multisig (multisig of multisigs) on either side.
- Sub-keys of types other than `secp256k1` (legacy) / `eth_secp256k1` (new) — e.g. `ed25519` is rejected with an invalid-pubkey error during proof verification.
- Asymmetric shape or K/N migrations — e.g. 2-of-3 legacy → 1-of-1 new, or multisig legacy → single-key new. Rejected at `ValidateBasic` by the mirror-source rule.
- Native wallet (Keplr/Leap) multisig signing UX — the four-step offline CLI flow is required.
- The new multisig bech32 is a Cosmos SDK address derived from `kmultisig.NewLegacyAminoPubKey`; it is **not** an EVM-addressable 20-byte address and cannot originate `MsgEthereumTx`. Operators who want EVM DeFi access for commissions/rewards should configure a separate single-EOA withdraw address via `MsgSetWithdrawAddress` after migration.

### Wire format

Both `MsgClaimLegacyAccount` and `MsgMigrateValidator` carry `legacy_proof` and `new_proof` as protobuf oneofs of the same `MigrationProof` shape (defined in `proto/lumera/evmigration/proof.proto`):

```protobuf
message MigrationProof {
  oneof proof {
    SingleKeyProof single   = 1;
    MultisigProof  multisig = 2;
  }
}
```

`SingleKeyProof` carries `pub_key`, `signature`, and `sig_format`. `MultisigProof` carries:

| Field | Type | Description |
|-------|------|-------------|
| `threshold` | `uint32` | K — number of signatures required |
| `sub_pub_keys` | `[]bytes` | All N compressed secp256k1 sub-keys (33 bytes each), in declaration order |
| `signer_indices` | `[]uint32` | 0-based indices (into `sub_pub_keys`) of the K signers — must be strictly ascending |
| `sub_signatures` | `[]bytes` | Signatures from the K signers, parallel to `signer_indices` |
| `sig_format` | `SigFormat` | `SIG_FORMAT_CLI` or `SIG_FORMAT_ADR036` — applies to all sub-signatures |

### Invariants enforced at verification time

- `len(signer_indices) == threshold` — exactly K signatures, no more, no less
- `signer_indices` is strictly ascending — no duplicate signers
- Each entry in `sub_pub_keys` is exactly 33 bytes (compressed secp256k1)
- `sig_format` must be non-zero (`SIG_FORMAT_UNSPECIFIED` is rejected)
- `len(sub_pub_keys) <= params.MaxMultisigSubKeys` (default 20) — enforced by `ValidateParams`

### Preconditions

The legacy multisig pubkey must be non-nil on-chain. A multisig account that was funded but has never signed a transaction has a nil pubkey stored in `x/auth`. The verifier cannot reconstruct the multisig structure from a nil pubkey.

**Remediation:** have one authorized co-signer submit any valid transaction from the multisig account (e.g., a 1-ulume self-send via `lumerad tx bank send`). That transaction causes the chain to store the full multisig pubkey on-chain. Confirm with:

```bash
lumerad query auth account <multisig-legacy-address>
```

The response should show a `multisig` key with all sub-keys listed.

### Four-step CLI flow

Migration of a multisig account uses four offline commands. See [migration.md](../user-guides/migration.md#migrating-a-multisig-account) for the full walkthrough with example arguments.

1. **Coordinator** generates the proof payload template with `generate-proof-payload`.
2. **Each co-signer** signs independently on their own machine with `sign-proof`.
3. **Coordinator** merges the threshold-many partial signatures with `combine-proof`.
4. **Coordinator** broadcasts the assembled transaction with `submit-proof`.

### MigrationEstimate preflight

The `MigrationEstimate` query (`lumerad query evmigration migration-estimate <address>`) pre-flight check detects multisig shapes that would fail at `ValidateBasic`:

- `would_succeed: true` requires all of: `is_multisig = true`, every sub-key is `secp256k1`, no duplicate sub-key entries, and `num_signers <= MaxMultisigSubKeys`.
- `would_succeed: false` fires with a descriptive `rejection_reason` when any of:
  - any sub-key is not `secp256k1` (unsupported shape);
  - any two sub-key entries are byte-equal ("sub_pub_keys[i] duplicates sub_pub_keys[j]" — SDK multisig construction permits duplicates, but `MultisigProof.validateBasic` rejects them at consensus);
  - `num_signers > MaxMultisigSubKeys` (governance-controlled cap).
- `is_multisig`, `threshold`, and `num_signers` are included in the response so the portal and CLI can branch on proof shape before prompting users.
