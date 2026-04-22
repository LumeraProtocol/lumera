# evmigration Multisig Support — Design

**Date:** 2026-04-18
**Module:** `x/evmigration`
**Status:** Draft (awaiting user review)

## 1. Summary

Enable `MsgClaimLegacyAccount` and `MsgMigrateValidator` to accept migration proofs from legacy accounts whose on-chain pubkey is a flat Cosmos SDK multisig (`multisig.LegacyAminoPubKey`) where every sub-key is a Cosmos `secp256k1.PubKey`.

The current implementation (as of commit `1cabdc7`) hard-codes a single 33-byte compressed secp256k1 pubkey in both the tx messages and the signature verifier, which makes multisig accounts unmigrateable. This design replaces the two flat `legacy_pub_key` / `legacy_signature` fields with a single structured `LegacyProof` oneof carrying either the existing single-key proof or a new multisig proof. Destination (new) addresses remain single `eth_secp256k1` EOAs in all cases. Co-signers coordinate offline via a new four-step CLI flow (`generate-proof-payload` → `sign-proof` → `combine-proof` → `submit-proof`) modeled on the SDK's `tx multisign` pattern.

## 2. Goals & Non-Goals

### Goals

- Multisig-controlled balance-holding accounts can migrate via `MsgClaimLegacyAccount`.
- Multisig-controlled validator operator addresses can migrate via `MsgMigrateValidator`, preserving all delegations, distribution state, supernode records, and action references.
- Multisig accounts appear in the `LegacyAccounts` query with enough metadata (threshold, N) for clients to build the correct proof shape.
- Each co-signer can sign offline on a separate machine using `lumerad` and their own keyring, then a coordinator assembles the multi-party proof.
- Integration and devnet tests cover multisig end-to-end so the path is not left untested at upgrade time.

### Non-Goals

- **Multisig destination addresses.** The new (coin-type-60) address is always a single `eth_secp256k1` EOA. Multisig members who want ongoing multisig custody on the EVM side deploy a Gnosis Safe (or similar contract-based multisig) at a separate address after migration.
- **Nested multisig.** Sub-keys that are themselves multisig are rejected.
- **Non-secp256k1 sub-keys.** `ed25519`, `sr25519`, `eth_secp256k1`, or any other key type as a sub-key is rejected.
- **Wallet (Keplr/Leap) multisig signing UX.** Wallet extensions have no built-in multisig-coordination primitive; co-signers use the CLI.
- **Multisig accounts with nil on-chain pubkey.** An account that received funds but never signed a transaction has `acc.GetPubKey() == nil`. The single-key path already supports these accounts because the CLI obtains the pubkey from the co-signer's keyring. For multisig we require the full `LegacyAminoPubKey` (threshold + N sub-pubkeys) to already be stored on-chain, because `generate-proof-payload` seeds the `PartialProof` JSON from on-chain data and there is no trusted local source for a nil-pubkey multisig's structure. In practice a multisig's pubkey is recorded on-chain the first time it signs any tx; multisigs that received funds and never signed must first sign any valid tx (the smallest is a 1-`ulume` self-send via `lumerad tx bank send`) to register their pubkey before attempting migration. This limitation is documented in the user-facing docs.

## 3. Decisions Captured From Brainstorming

| # | Decision | Rationale |
|---|----------|-----------|
| Q1 | Scope covers both `MsgClaimLegacyAccount` and `MsgMigrateValidator` | Real multisig validator operators exist; leaving them unmigrateable would strand validator state. |
| Q2 | Destination is only a single `eth_secp256k1` EOA | EVM has no native multi-pubkey address derivation; `VerifyNewSignature` stays unchanged. |
| Q3 | Wire format: full `oneof LegacyProof { SingleKey; Multisig; }` replacing the two flat fields | Cleanest schema; no on-chain migration risk because module is pre-EVM-upgrade; block explorers and client tooling can introspect. |
| Q4 | CLI: four-step offline flow (`generate-proof-payload` / `sign-proof` / `combine-proof` / `submit-proof`) | Matches SDK's `tx multisign` mental model; works for cold-wallet and air-gapped co-signers. |
| Q5 | `SigFormat` is uniform per-tx (one enum value drives all K sub-signatures) | Simpler verifier; coordinator picks CLI or ADR-036 once. |
| Q6 | Flat multisig only; module param `MaxMultisigSubKeys` default 20 | Predictable worst-case verification cost; nested multisig is rare and increases verifier complexity without real demand. |
| — | Verifier refactor: Approach 1 — minimal branch in `VerifyLegacyProof` | Current code is already pure and testable; a dedicated verifier interface/package would be over-engineered for two proof types. |

## 4. Architecture

### 4.1 Proto schema

**New file `proto/lumera/evmigration/proof.proto`:**

```proto
syntax = "proto3";
package lumera.evmigration;
option go_package = "x/evmigration/types";

enum SigFormat {
  SIG_FORMAT_UNSPECIFIED = 0;
  SIG_FORMAT_CLI         = 1; // Sign(SHA256(payload)) via keyring
  SIG_FORMAT_ADR036      = 2; // ADR-036 signArbitrary canonical JSON
}

message LegacyProof {
  oneof proof {
    SingleKeyProof single   = 1;
    MultisigProof  multisig = 2;
  }
}

message SingleKeyProof {
  bytes pub_key        = 1; // 33 bytes compressed secp256k1
  bytes signature      = 2; // 64-byte raw secp256k1 sig (CLI) or canonical ADR-036 sig
  SigFormat sig_format = 3;
}

message MultisigProof {
  uint32 threshold               = 1; // K
  repeated bytes sub_pub_keys    = 2; // all N sub-keys, original ordering, 33 bytes each
  repeated uint32 signer_indices = 3; // exactly K distinct indices, strictly ascending
  repeated bytes sub_signatures  = 4; // same order as signer_indices
  SigFormat sig_format           = 5;
}
```

**`proto/lumera/evmigration/tx.proto` — message changes:**

```proto
message MsgClaimLegacyAccount {
  string new_address    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string legacy_address = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  LegacyProof legacy_proof = 3 [(gogoproto.nullable) = false];
  bytes new_signature   = 5;
  reserved 4;
  reserved "legacy_pub_key", "legacy_signature";
}

message MsgMigrateValidator {
  string new_address    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string legacy_address = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  LegacyProof legacy_proof = 3 [(gogoproto.nullable) = false];
  bytes new_signature   = 5;
  reserved 4;
  reserved "legacy_pub_key", "legacy_signature";
}
```

**`proto/lumera/evmigration/params.proto` — add one field:**

```proto
message Params {
  // existing fields: enable_migration=1, migration_end_time=2,
  //                  max_migrations_per_block=3, max_validator_delegations=4
  uint32 max_multisig_sub_keys = 5; // default 20
}
```

**`proto/lumera/evmigration/query.proto` — extend `LegacyAccountInfo`:**

```proto
message LegacyAccountInfo {
  // existing fields: address=1, balance_summary=2, has_delegations=3, is_validator=4
  bool   is_multisig   = 5;
  uint32 threshold     = 6; // 0 when !is_multisig
  uint32 num_signers   = 7; // 0 when !is_multisig
}
```

### 4.2 Verifier

`x/evmigration/keeper/verify.go` replaces `VerifyLegacySignature` with `VerifyLegacyProof`:

```go
func VerifyLegacyProof(
    chainID string, evmChainID uint64, kind string,
    legacyAddr, newAddr sdk.AccAddress,
    proof *types.LegacyProof,
) error {
    payload := migrationPayload(chainID, evmChainID, kind, legacyAddr, newAddr)
    switch p := proof.Proof.(type) {
    case *types.LegacyProof_Single:
        return verifySingleKeyProof(payload, legacyAddr, p.Single)
    case *types.LegacyProof_Multisig:
        return verifyMultisigProof(payload, legacyAddr, p.Multisig)
    default:
        return types.ErrInvalidLegacyProof.Wrap("no proof set")
    }
}
```

- `verifySingleKeyProof` reuses existing single-key logic plus `sig_format` dispatch.
- `verifyMultisigProof` reconstructs `kmultisig.NewLegacyAminoPubKey(K, subKeys)`, asserts `Address() == legacyAddr`, then verifies each sub-signature against its claimed sub-key using the shared helper below.
- `verifySecp256k1Sig` consolidates the CLI-vs-ADR-036 dispatch — same implementation serves single-key and multisig paths; for multisig the ADR-036 doc is reconstructed per-sub-signer using the signer's own bech32 derived from the sub-pubkey.

`kmultisig.NewLegacyAminoPubKey` is the same constructor used by `lumerad keys add --multisig`, so address derivation is byte-for-byte identical to how the legacy account was originally registered.

### 4.3 ValidateBasic

New `types/proof.go` factors per-proof validation into two tiers, because the Msg's `ValidateBasic()` is stateless (runs in mempool before any keeper access) but `MaxMultisigSubKeys` is a governance-controlled module param only readable via keeper.

**Tier 1 — stateless (`LegacyProof.ValidateBasic()`):**

Called from `MsgClaimLegacyAccount.ValidateBasic` and `MsgMigrateValidator.ValidateBasic`. Dispatches to `SingleKeyProof.validateBasic()` or `MultisigProof.validateBasic()`:

- `SingleKeyProof.validateBasic()` enforces 33-byte pubkey, non-empty signature, specified `sig_format`.
- `MultisigProof.validateBasic()` enforces:
  - `N ≥ 1` (sub-keys non-empty)
  - `1 ≤ threshold ≤ N`
  - `len(signer_indices) == threshold` (exact-K rule, not "at least K")
  - `len(sub_signatures) == len(signer_indices)`
  - `signer_indices` strictly ascending (enforces uniqueness + canonical ordering)
  - Every `signer_indices[i]` in range; every indexed sub-pubkey 33 bytes
  - `sig_format != UNSPECIFIED`

**Tier 2 — param-aware (`LegacyProof.ValidateParams(maxSubKeys uint32)`):**

Called from the msg server immediately after loading params and before invoking the verifier. Enforces `N ≤ maxSubKeys` on the multisig path; no-op on the single-key path. Split this way so fuzz / simulation tests can exercise stateless rules without a keeper, and so the governance-adjustable cap never has to be duplicated in `ValidateBasic` logic.

### 4.4 Legacy-account detection

`x/evmigration/keeper/query.go:39-84` (`remainingLegacyAccountStatus`) currently excludes any pubkey that isn't `*secp256k1.PubKey`. Extend the type check to also include flat multisig-of-secp256k1:

```go
func isLegacyPubKey(pk cryptotypes.PubKey) bool {
    switch key := pk.(type) {
    case *secp256k1.PubKey:
        return true
    case *kmultisig.LegacyAminoPubKey:
        for _, sub := range key.GetPubKeys() {
            if _, ok := sub.(*secp256k1.PubKey); !ok {
                return false
            }
        }
        return true
    default:
        return false
    }
}
```

The `LegacyAccounts` query populates the new `is_multisig`, `threshold`, and `num_signers` fields when the pubkey is multisig so clients can build the correct proof shape without a second round-trip.

### 4.4.1 MigrationEstimate preflight

`MigrationEstimate` ([query.go:135](x/evmigration/keeper/query.go#L135)) currently only counts touched state and validator conditions. It must also surface multisig feasibility so users don't discover mid-tx that their multisig shape is unsupported.

Extend `MigrationEstimate` to inspect the account's on-chain pubkey:

```go
acc := qs.k.accountKeeper.GetAccount(ctx, addr)
if acc != nil {
    if pk := acc.GetPubKey(); pk != nil {
        if ms, ok := pk.(*kmultisig.LegacyAminoPubKey); ok {
            resp.IsMultisig = true
            resp.Threshold  = uint32(ms.Threshold)
            resp.NumSigners = uint32(len(ms.GetPubKeys()))

            // Feasibility: nested / non-secp256k1 sub-keys reject.
            for _, sub := range ms.GetPubKeys() {
                if _, ok := sub.(*secp256k1.PubKey); !ok {
                    resp.WouldSucceed = false
                    resp.RejectionReason = "multisig contains non-secp256k1 sub-key (unsupported)"
                    break
                }
            }
            // Size cap.
            if resp.WouldSucceed && resp.NumSigners > params.MaxMultisigSubKeys {
                resp.WouldSucceed = false
                resp.RejectionReason = fmt.Sprintf("multisig has %d sub-keys; max is %d", resp.NumSigners, params.MaxMultisigSubKeys)
            }
        }
    } else {
        // Nil pubkey. Single-key migration still works (the CLI obtains the
        // pubkey from the co-signer's keyring). Multisig migration is NOT
        // supported for nil-pubkey addresses (see Non-Goals) because the chain
        // has no record of the threshold or sub-pubkeys. MigrationEstimate
        // cannot distinguish the two cases from the account alone and
        // therefore does NOT flag nil-pubkey accounts. That failure mode is
        // surfaced at `generate-proof-payload` time on the CLI — see 4.5.
    }
}
```

Corresponding new fields on `QueryMigrationEstimateResponse` (existing fields occupy indexes 1–15):

```proto
bool   is_multisig  = 16;
uint32 threshold    = 17;
uint32 num_signers  = 18;
```

This ensures `WouldSucceed=true` for a multisig account only when the verifier would actually accept a correctly-constructed proof.

### 4.5 CLI multi-step flow

Four new subcommands under `lumerad tx evmigration`. The flow is designed for co-signers on **physically separate machines** — each signer gets a fresh copy of the payload template, signs locally, and returns their own partial file. The coordinator merges the returned partial files.

1. **`generate-proof-payload`** — Queries on-chain account, produces a `PartialProof` JSON seeded with pubkey material, payload bytes, threshold, format, and an empty `partial_signatures` array.

   Pubkey seeding rules:
   - **Multisig account (non-nil on-chain pubkey):** Seeds `multisig.sub_pub_keys`, `multisig.threshold`, and `multisig.num_signers` from the on-chain `LegacyAminoPubKey`. The `--legacy-key` flag is rejected (not applicable).
   - **Multisig account (nil on-chain pubkey):** Rejected with the "sign any valid tx first" remediation error (see Non-Goals).
   - **Single-key account (non-nil on-chain pubkey):** Seeds `single.pub_key` from the on-chain `secp256k1.PubKey`. `--legacy-key` is optional; if provided, the command verifies the keyring key's pubkey matches the on-chain value and errors otherwise (catches "wrong key" mistakes before sub-signers waste effort).
   - **Single-key account (nil on-chain pubkey):** Requires `--legacy-key <name>`; seeds `single.pub_key` from the local keyring. The command verifies `sdk.AccAddress(keyringPubKey.Address()) == --legacy` bech32 before writing the file, so a mistyped key name is caught immediately. Without `--legacy-key`, the command errors with: "account at {addr} has no on-chain pubkey record; pass --legacy-key to seed the pubkey from your keyring (single-sig only), or for a multisig address submit a 1-ulume self-send first."

   The coordinator distributes copies of the resulting file to each co-signer (email, USB, etc.).

2. **`sign-proof <file> --from <key> [--out <file>]`** — A co-signer runs this on their own machine against their own keyring. Matches `--from` key's pubkey against `sub_pub_keys` to determine its index, signs the payload in the configured format, and writes a copy of the file containing exactly ONE new `partial_signatures` entry `{index, signature}` appended to whatever was already there. If `--out` is omitted, overwrites the input file. This command is idempotent — resigning with the same key overwrites the previous entry for that index, never duplicates. Returns the output path so it can be emailed back to the coordinator.

3. **`combine-proof <partial1.json> [<partial2.json> ...] --out <tx.json>`** — Accepts one or more partial files. Validates that all inputs share the same `legacy_address`, `new_address`, `chain_id`, `evm_chain_id`, `kind`, `sig_format`, `threshold`, and `sub_pub_keys` (rejects with a descriptive error otherwise — catches "co-signers signed payloads for different chains" mistakes). Merges `partial_signatures` from all inputs, deduplicating by `index` (keeping the last occurrence, since a resigned partial is intentional). Validates `len(merged) >= threshold`. Picks the K lowest indices, reorders to ascending, assembles `LegacyProof{Multisig: ...}`, and writes an unsigned tx JSON to `--out`.

   Single-file invocation (`combine-proof combined.json`) still works and supports the simple case where all signers wrote to one shared file (e.g., a CI pipeline or a single developer testing locally).

4. **`submit-proof <tx.json> --from <new-eth-key>`** — Signs the `new_signature` using the destination EVM key (existing `signNewMigrationProof` helper), runs `ValidateBasic`, simulates gas, broadcasts.

The existing one-shot `claim-legacy <legacy-key> <new-key>` and `migrate-validator <legacy-key> <new-key>` commands remain for single-sig users — internally they just build a `LegacyProof{Single: ...}`.

The multi-step flow also works for single-sig accounts (with `len(partial_signatures) == 1`), which lets cold-wallet holders use the offline path.

**`PartialProof` JSON schema** (unversioned, not a proto):

```json
{
  "version": 1,
  "kind": "claim",
  "legacy_address": "lumera1...",
  "new_address":    "lumera1...",
  "chain_id": "lumera-devnet",
  "evm_chain_id": 76857769,
  "payload_hex": "6c756d6572612d65766d2d6d6967726174696f6e3a...",
  "multisig": {
    "threshold": 2,
    "sub_pub_keys": ["AxYZ...", "AiBC...", "AjKL..."],
    "sig_format": "SIG_FORMAT_CLI"
  },
  "partial_signatures": [
    { "index": 0, "signature": "base64..." },
    { "index": 2, "signature": "base64..." }
  ]
}
```

For single-sig, `multisig` is replaced by `single` (with a single `pub_key`) and `partial_signatures` has exactly one entry at `index: 0`.

### 4.6 Msg-server callers

`msg_server_claim_legacy.go` and `msg_server_migrate_validator.go` change from:

```go
if err := VerifyLegacySignature(
    ctx.ChainID(), evmChainID, migrationPayloadKindClaim,
    legacyAddr, newAddr, msg.LegacyPubKey, msg.LegacySignature,
); err != nil { ... }
```

to:

```go
if err := msg.LegacyProof.ValidateParams(params.MaxMultisigSubKeys); err != nil { ... }
if err := VerifyLegacyProof(
    ctx.ChainID(), evmChainID, migrationPayloadKindClaim,
    legacyAddr, newAddr, msg.LegacyProof,
); err != nil { ... }
```

## 5. Testing Strategy

### 5.1 Unit tests (`x/evmigration/`)

Extend existing files:

- **`keeper/verify_test.go`** — `TestVerifyLegacyProof_Multisig` covering: valid 2-of-3 CLI, valid 2-of-3 ADR-036, exact-K pass, K-1 reject, K+1 reject, invalid sub-sig, non-ascending indices, out-of-range index, address mismatch, 1-of-1 edge, N=`MaxMultisigSubKeys` boundary, N=`MaxMultisigSubKeys + 1` reject, mixed sub-key types reject.
- **`keeper/migrate_test.go`** — `TestMigrateMultisigAccount` covering base + vesting + authz/feegrant variants + already-migrated reject.
- **`keeper/msg_server_migrate_validator_test.go`** — `TestMigrateValidator_MultisigOperator`: delegations re-keyed, distribution state re-keyed, supernode record re-keyed, consensus pubkey unchanged.
- **`keeper/query_test.go`** — `TestLegacyAccounts_Multisig`: returns multisig accounts with correct `is_multisig`/`threshold`/`num_signers`; `TestMigrationStats_IncludesMultisig`; `TestMigrationEstimate_Multisig_Supported` (valid 2-of-3 secp256k1 → `WouldSucceed=true`); `TestMigrationEstimate_Multisig_NonSecp256k1SubKey` (rejected with descriptive `RejectionReason`); `TestMigrationEstimate_Multisig_TooManySubKeys` (N > `MaxMultisigSubKeys` → rejected); `TestMigrationEstimate_Multisig_NestedRejected` (sub-key is itself multisig → rejected).
- **New `types/proof_test.go`** — every `ValidateBasic` rejection branch.

### 5.2 Integration tests (`tests/integration/evmigration/`)

- `TestMsgClaimLegacyAccount_Multisig` — 2-of-3 balance migration.
- `TestMsgClaimLegacyAccount_MultisigVesting` — continuous vesting preserved.
- `TestMsgMigrateValidator_Multisig` — delegations, distribution, supernode re-keyed.
- `TestMsgClaimLegacyAccount_Multisig_WrongThreshold` — K-1 sigs rejected.
- `TestMsgClaimLegacyAccount_Multisig_ReplayRejected` — re-submit fails.
- `TestMsgClaimLegacyAccount_Multisig_ADR036` — ADR-036 format end-to-end.

Shared helpers in `tests/integration/evmigration/multisig_helpers.go`:

```go
func buildMultisigLegacyAccount(t, ctx, N, K int) (multisigAddr sdk.AccAddress, subKeys []*secp256k1.PrivKey, pubKey *kmultisig.LegacyAminoPubKey)
func signMultisigProof(payload []byte, subKeys []*secp256k1.PrivKey, signerIdxs []int, format types.SigFormat) *types.MultisigProof
```

### 5.3 CLI tests (`x/evmigration/client/cli/tx_test.go`)

- `TestGenerateProofPayload_Multisig` — JSON output well-formed; sub-pubkeys seeded from on-chain multisig.
- `TestGenerateProofPayload_SingleKey_OnChainPubKey` — JSON output well-formed; `single.pub_key` seeded from on-chain `secp256k1.PubKey`.
- `TestGenerateProofPayload_SingleKey_OnChainPubKey_LegacyKeyFlag_Match` — `--legacy-key` matches on-chain pubkey; succeeds.
- `TestGenerateProofPayload_SingleKey_OnChainPubKey_LegacyKeyFlag_Mismatch` — `--legacy-key` points to different key than on-chain; errors.
- `TestGenerateProofPayload_SingleKey_NilPubKey_WithLegacyKey` — nil on-chain pubkey + `--legacy-key` seeds pubkey from keyring after verifying address derivation; succeeds.
- `TestGenerateProofPayload_SingleKey_NilPubKey_WithoutLegacyKey` — nil on-chain pubkey, no `--legacy-key`; errors with remediation message.
- `TestGenerateProofPayload_SingleKey_NilPubKey_WrongLegacyKey` — nil on-chain pubkey, `--legacy-key` derives to a different address; errors before writing file.
- `TestGenerateProofPayload_Multisig_NilPubKey_Rejected` — multisig address with nil on-chain pubkey is rejected with the "sign any tx first" workaround hint; `--legacy-key` (if passed) is rejected as inapplicable.
- `TestSignProof_MultisigPartial` — signer appends; non-member key rejected.
- `TestSignProof_Idempotent` — resigning with same key overwrites, does not duplicate.
- `TestCombineProof_CanonicalOrdering` — out-of-order signers produce byte-identical tx.
- `TestCombineProof_MultiFile` — merges partials from N separate files (simulating separate-machine co-signers).
- `TestCombineProof_MismatchedPayloadsRejected` — different `chain_id`, `legacy_address`, `new_address`, or `kind` across partial files rejected with descriptive error.
- `TestCombineProof_BelowThresholdRejected` — merged partial count < K rejected.
- `TestSubmitProof_Multisig` — full four-step against mock chain.

### 5.4 Devnet tests (`devnet/tests/evmigration/`)

- `multisig_keys.go` — seeds a 2-of-3 multisig with balances, delegations, and an authz grant. Also triggers one trivial signed tx from the multisig pre-test so that `acc.GetPubKey()` is non-nil (mirrors the real-world precondition documented in Non-Goals).
- `multisig_test.go` — end-to-end CLI driver exercising the **separate-machine** flow: one `generate-proof-payload` invocation, three distinct `sign-proof ... --out signerN.json` calls, one `combine-proof signer1.json signer2.json --out tx.json`, one `submit-proof`. Verifies `MigrationRecord`, balances at new EOA, delegations re-keyed, replay rejected. Also includes a `shared-file` variant (one proof.json mutated in place across three `sign-proof` calls then `combine-proof proof.json`) to exercise both coordination styles.
- `multisig_validator_test.go` — same for a multisig validator operator.
- `multisig_estimate_test.go` — calls `lumerad query evmigration migration-estimate` on: a supported 2-of-3 multisig (expect `would_succeed=true`, correct threshold/num_signers); a multisig with N > `MaxMultisigSubKeys` (expect `would_succeed=false` with size-cap reason); a nested multisig seeded into the devnet fixture (expect `would_succeed=false` with nested/non-secp256k1 reason).

### 5.5 Documentation updates

- `docs/evm-integration/tests.md` — new rows under evmigration.
- `docs/evm-integration/evmigration.md` (or similar) — new section "Multisig account migration" with four-step CLI example and `PartialProof` JSON schema.
- `docs/evm-integration/unit-evmigration.md`, `docs/evm-integration/integration-evmigration.md` — coverage summaries.

## 6. Rollout

Module is pre-mainnet-EVM-upgrade — no in-flight on-chain messages, no wire-compat concerns:

1. Land proto changes (`make build-proto`).
2. Update `ValidateBasic`, verifier, keeper msg-server callers.
3. Update CLI (keep existing one-shot commands; add four new subcommands).
4. Update `LegacyAccounts` query to include multisig.
5. Add unit + integration + CLI tests.
6. Add devnet scenario.
7. Update docs.

`MaxMultisigSubKeys = 20` is set at module init; adjustable via existing `MsgUpdateParams` governance path.

## 7. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Multisig address reconstruction diverges from SDK's amino serialization | Use `kmultisig.NewLegacyAminoPubKey` — same constructor as `lumerad keys add --multisig`. Integration test creates multisig via keyring to catch any divergence. |
| N=`MaxMultisigSubKeys` tx with all invalid sigs → up to 20 verifications before reject (DoS vector) | Bounded by `MaxMultisigSubKeys = 20`. Tx pays gas via the wrapping tx (migration msg itself is fee-free, but tx bytes still metered). Acceptable for a one-time migration window. |
| Co-signer signs with wrong chain-id → cryptic "signature invalid" | Error message at [verify.go:95](x/evmigration/keeper/verify.go#L95) already hints at chain-id mismatch; extend the hint to multisig path. |
| `PartialProof` JSON format drift across `lumerad` versions during migration window | Version field (`"version": 1`); `combine-proof` rejects unknown fields or unsupported versions. |
| Multisig sub-signer loses access mid-coordination | Out of scope — same problem exists for any multisig; pre-migration key rotation on the legacy chain is the only remedy, which is already impossible if keys are lost. Document as caveat. |
| User discovers mid-migration that their account has nil on-chain pubkey (funded but never signed) | `generate-proof-payload` detects `acc.GetPubKey() == nil` and branches by intent: (a) if `--legacy-key` is supplied, seeds `single.pub_key` from the local keyring after verifying its derived address equals `--legacy`; (b) without `--legacy-key`, errors with remediation: "account at {addr} has no on-chain pubkey record; pass --legacy-key to seed the pubkey from your keyring (single-sig only), or for a multisig address submit a 1-ulume self-send first." **Note:** `MigrationEstimate` does **not** flag nil-pubkey accounts because the account alone carries no signal distinguishing single-key from multisig (both just look "unsigned"). Detection is deferred to the CLI. This tradeoff is documented in `docs/evm-integration/evmigration.md`. |
| Co-signers on separate machines produce partial files that disagree (wrong chain-id, out-of-date payload) | `combine-proof` validates that all inputs agree on `legacy_address`, `new_address`, `chain_id`, `evm_chain_id`, `kind`, `sig_format`, `threshold`, and `sub_pub_keys`. Error message identifies the first divergent field. |

## 8. File-Change Inventory

### New files

- `proto/lumera/evmigration/proof.proto`
- `x/evmigration/types/proof.go`
- `x/evmigration/types/proof_test.go`
- `x/evmigration/client/cli/tx_multisig.go`
- `x/evmigration/client/cli/tx_multisig_test.go`
- `tests/integration/evmigration/multisig_helpers.go`
- `devnet/tests/evmigration/multisig_keys.go`
- `devnet/tests/evmigration/multisig_test.go`
- `devnet/tests/evmigration/multisig_validator_test.go`
- `devnet/tests/evmigration/multisig_estimate_test.go`
- `docs/design/2026-04-18-evmigration-multisig-design.md` (this document)

### Modified files

- `proto/lumera/evmigration/tx.proto`
- `proto/lumera/evmigration/params.proto`
- `proto/lumera/evmigration/query.proto`
- `x/evmigration/types/types.go`
- `x/evmigration/types/params.go`
- `x/evmigration/keeper/verify.go`
- `x/evmigration/keeper/verify_test.go`
- `x/evmigration/keeper/msg_server_claim_legacy.go`
- `x/evmigration/keeper/msg_server_migrate_validator.go`
- `x/evmigration/keeper/query.go`
- `x/evmigration/keeper/migrate_test.go`
- `x/evmigration/keeper/query_test.go`
- `x/evmigration/client/cli/tx.go`
- `x/evmigration/module/autocli.go` — the `ClaimLegacyAccount` and `MigrateValidator` RPC descriptors at [lines 68-86](x/evmigration/module/autocli.go#L68-L86) hard-code the old `legacy_pub_key` / `legacy_signature` positional args. Mark both `Skip: true` (matching how `UpdateParams` is already handled at [line 65](x/evmigration/module/autocli.go#L65)) and rely entirely on the hand-written commands in `x/evmigration/client/cli/tx.go` and `tx_multisig.go`. AutoCLI cannot auto-generate a usable CLI for a `oneof`-typed field anyway.
- `tests/integration/evmigration/migration_test.go`
- `docs/evm-integration/tests.md`
- `docs/evm-integration/evmigration.md` (or equivalent)
- `docs/evm-integration/evmigration/portal-ui.md` — the migration portal UI doc at [line 32](docs/evm-integration/evmigration/portal-ui.md#L32) documents the old `legacy_pub_key` / `legacy_signature` field names for frontend consumers. Update to describe the new `legacy_proof` oneof (both `single` and `multisig` shapes) and the multi-step multisig CLI flow so portal developers building on top can surface the right UX.
- `docs/evm-integration/unit-evmigration.md`
- `docs/evm-integration/integration-evmigration.md`

## 9. Open Questions

None blocking. Defaults were taken for:

- `MaxMultisigSubKeys = 20` — governance-adjustable post-launch if real multisigs exceed this.
- Uniform-per-tx `SigFormat` — if per-sub-signer formats become needed later, `sig_format` can be moved from `MultisigProof` onto each `SubSignature` in a subsequent proto release; the existing wire format has a natural extension point.
