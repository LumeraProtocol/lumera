# evmigration Multisig Support — Design

**Date:** 2026-04-22 (revised from 2026-04-18 draft)
**Module:** `x/evmigration`
**Status:** Revised — pivots destination from "single EOA" to "mirror-source shape"

## 1. Summary

Enable `MsgClaimLegacyAccount` and `MsgMigrateValidator` to accept migration proofs from legacy accounts whose on-chain pubkey is a flat Cosmos SDK multisig (`multisig.LegacyAminoPubKey`) where every sub-key is a Cosmos `secp256k1.PubKey`, and correspondingly produce a destination account that preserves the multisig's K-of-N control structure.

The current implementation (commits `3ac2f8b`…`40cc385` on branch `evm`) accepts multisig legacy proofs but migrates them to a single `eth_secp256k1` EOA. That collapses K-of-N control into 1-of-1 control, which is unacceptable for the primary use case (validator operator keys managed by K-of-N signing committees). This revision makes the **destination shape mirror the source shape**:

- **Single-key legacy** → single `eth_secp256k1` EOA destination (unchanged from current implementation). This is an EVM-addressable account: its bech32 derives via Ethereum's `keccak256(uncompressed_pubkey)[12:]` convention, it can originate `MsgEthereumTx`, and it can be `msg.sender` in Solidity contracts.
- **Multisig legacy** → **Cosmos SDK multisig destination with `eth_secp256k1` sub-keys**, same threshold K and same N. **Important scoping note:** this destination is a Cosmos SDK account — its bech32 derives via `kmultisig.LegacyAminoPubKey.Address()` (amino-encoded `RIPEMD160(SHA256(...))`), *not* via Ethereum's keccak256 convention. It signs Cosmos SDK transactions (validator ops, bank, staking, gov, IBC, authz, x/erc20 STRv2 bank mirror) but **cannot originate `MsgEthereumTx` and cannot be `msg.sender` in Solidity contracts**. Validator operator keys — the primary use case — don't need EVM-native addressability. Users who need EVM-native multisig custody deploy a Gnosis Safe at a separate address.

Each co-signer rotates their personal key from coin-type-118 (Cosmos `secp256k1`) to coin-type-60 (`eth_secp256k1`); the multisig's K-of-N governance structure is preserved. The proof shape is unified into a single `MigrationProof` oneof used on both the `legacy_proof` and `new_proof` fields of the tx messages.

### Validation spike

Before pivoting, the design was de-risked by a devnet spike on 2026-04-22 against `validator_3` with the `lumera-devnet-1` chain:

1. Assembled a 2-of-3 `LegacyAminoPubKey` from three fresh `ethsecp256k1.PubKey` sub-keys via `lumerad keys add --multisig`. Keyring accepted the heterogeneous-hash-convention shape without error.
2. Signed `MsgSend` from the multisig (signers 1 & 3, SIGN_MODE_LEGACY_AMINO_JSON) — tx committed at height 6403, code 0. Bank event attributes correctly identified the multisig bech32 as `sender` and `fee_payer`.
3. Signed `MsgCreateValidator` from the multisig (signers 1 & 3; 500 LUME self-bond, fresh ed25519 consensus key, moniker `spike-eth-msig-validator`) — committed at height 6439, code 0. Validator observed as `BOND_STATUS_BONDED` with `tokens=500000000` and `operator_address=lumeravaloper18u3sx5gfyh5jt3p2sxpfzl77duplm90ytm9y95`.
4. Signed `MsgEditValidator` from the same multisig with a **different** 2-of-3 subset (signers 2 & 3) — committed at height 6450, code 0. Moniker updated on-chain; tokens unchanged.

This empirically validates that the SDK's `LegacyAminoPubKey.VerifyMultisignature` handles per-sub-key heterogeneous hash conventions (`ethsecp256k1.PubKey.VerifySignature` applies Keccak256, versus `secp256k1.PubKey.VerifySignature`'s SHA256) and that any K-of-N sub-set produces a valid sig — no hidden ties between a multisig and a specific sub-signer set.

## 2. Goals & Non-Goals

### Goals

- Multisig-controlled balance-holding accounts can migrate via `MsgClaimLegacyAccount` and end up at a multisig destination with the same threshold.
- Multisig-controlled validator operator addresses can migrate via `MsgMigrateValidator`, preserving all delegations, distribution state, supernode records, and action references, *and* preserving K-of-N control over the new operator address.
- Multisig accounts appear in the `LegacyAccounts` query with enough metadata (threshold, N) for clients to build the correct proof shape.
- Each co-signer can sign offline on a separate machine using `lumerad` and their own keyring (both their legacy Cosmos `secp256k1` sub-key *and* their new `eth_secp256k1` sub-key), then a coordinator assembles the combined proof.
- Integration and devnet tests cover multisig→multisig end-to-end so the path is not left untested at upgrade time.

### Non-Goals

- **EVM-native addressability of multisig destinations.** As stated in §1, the multisig destination is a Cosmos SDK bech32, not a 20-byte keccak256 Ethereum address. It cannot be `msg.sender` in Solidity, cannot directly hold ERC-20s via Solidity `transfer`, cannot call EVM precompiles from the EVM side. All of those operations remain available via Cosmos-side messages (bank, staking, gov, IBC, authz) and x/erc20's STRv2 bank mirror. Users who need a multisig that ALSO interacts with EVM contracts as `msg.sender` deploy Gnosis Safe (or equivalent contract-based multisig) at a separate address after migration.
- **Changing destination shape for single-key sources.** A single-key legacy account continues to migrate to a single `eth_secp256k1` EOA. The destination-shape rule is strictly "mirror the source."
- **Cross-shape migrations.** A multisig legacy account cannot elect to collapse into a single-EOA destination at migration time, nor can a single-key account elect to become a multisig destination. If a multisig wants to consolidate post-migration, a K-of-N-signed bank send from the new multisig to a single EOA handles that cleanly — no chain-level optionality needed.
- **Nested multisig on either side.** Sub-keys that are themselves multisig are rejected on both legacy and new sides.
- **Mixed-type sub-keys on a single side.** Legacy sub-keys must all be Cosmos `secp256k1`; new sub-keys must all be `eth_secp256k1`. No mixing within one side.
- **Wallet (Keplr/Leap) multisig signing UX.** Wallet extensions have no built-in multisig-coordination primitive; co-signers use the CLI for both halves of a multisig proof.
- **Multisig accounts with nil on-chain legacy pubkey.** Unchanged from prior draft: legacy multisig migration requires the legacy account's `LegacyAminoPubKey` to already be recorded on-chain. The "sign any valid tx first" remediation stays documented.

## 3. Decisions Captured From Brainstorming

| # | Decision | Rationale |
|---|----------|-----------|
| Q1 | Scope covers both `MsgClaimLegacyAccount` and `MsgMigrateValidator` | Real multisig validator operators exist; leaving them unmigrateable would strand validator state. |
| Q2 | **Destination shape mirrors source shape**: single→single EOA, multisig→multisig-of-eth-sub-keys with same threshold | Preserves K-of-N governance for the primary use case (validator operator keys). Validated on devnet — SDK primitives handle eth sub-keys in `LegacyAminoPubKey` cleanly. The alternative ("collapse to single EOA with Gnosis Safe post-migration") requires a window of 1-of-1 control, which breaks the security model. |
| Q3 | Wire format: unified `oneof MigrationProof { SingleKey; Multisig; }` used for both `legacy_proof` and `new_proof` fields | Symmetric schema, reusable verifier logic, no asymmetric "pubkey recovery vs pubkey inclusion" split between the two sides. The EVM upgrade has not been deployed to any network, so proto changes are free — no `reserved` tags, no deprecated-field shims, no wire-compat migration. |
| Q4 | CLI: four-step offline flow (`generate-proof-payload` / `sign-proof` / `combine-proof` / `submit-proof`), with each co-signer's `sign-proof` producing both their legacy sub-signature *and* their new sub-signature in a single partial file | Same ergonomics as the prior draft; co-signers don't need to know about the legacy/new split as two separate processes — they run `sign-proof` once with both keys and contribute to both halves. |
| Q5 | `SigFormat` is uniform per side per tx — one enum value drives all K legacy sub-sigs, one drives all K new sub-sigs; sides are independent | Simpler verifier; coordinator picks CLI or ADR-036 independently for each side. |
| Q6 | Flat multisig only on both sides; module param `MaxMultisigSubKeys` default 20 (shared between sides) | Predictable worst-case verification cost. |
| Q7 | Destination multisig pubkey is persisted on the `BaseAccount.PubKey` at migration time (via `acc.SetPubKey(multiPK)` in `MigrateAuth`) | Avoids the "nil pubkey, must sign a tx first" footgun that the legacy side suffers from; the new multisig's shape is immediately discoverable on-chain, which matters for clients and for any downstream tooling that wants to verify threshold/N. |
| — | Verifier refactor: unified `VerifyMigrationProof(payload, boundAddr, proof, expectedSubKeyType)` — single function handles both sides by parameterizing the expected sub-key type | Single pair of helpers (`verifySingleKeyProof`, `verifyMultisigProof`) covers all four combinations (legacy single, legacy multi, new single, new multi) with no duplication. |

## 4. Architecture

### 4.1 Proto schema

**Renamed file `proto/lumera/evmigration/proof.proto`:**

```proto
syntax = "proto3";
package lumera.evmigration;
option go_package = "x/evmigration/types"; // matches repo-wide convention; buf generates at module root

enum SigFormat {
  SIG_FORMAT_UNSPECIFIED = 0;
  SIG_FORMAT_CLI         = 1; // Sign(SHA256(payload)) via keyring (Cosmos secp256k1) or Sign(Keccak256(payload)) (eth_secp256k1)
  SIG_FORMAT_ADR036      = 2; // ADR-036 signArbitrary canonical JSON
  SIG_FORMAT_EIP191      = 3; // Eth "\x19Ethereum Signed Message:\n…" envelope — new-side wallets only
}

message MigrationProof {
  oneof proof {
    SingleKeyProof single   = 1;
    MultisigProof  multisig = 2;
  }
}

message SingleKeyProof {
  bytes pub_key        = 1; // 33 bytes compressed secp256k1 (legacy side: Cosmos; new side: eth)
  bytes signature      = 2; // Canonical per-side wire length:
                            //   Legacy Cosmos secp256k1: 64 bytes (raw R||S). Cosmos keyring returns 64; there is no V convention for Cosmos secp256k1.
                            //   New eth_secp256k1:       65 bytes (R||S||V). go-ethereum's crypto.Sign (which Cosmos EVM wraps) always returns 65;
                            //                            Keplr/Leap personal_sign also always returns 65. The trailing V byte is recovery metadata
                            //                            that the verifier does NOT use (we do ECDSA-verify-under-pubkey, not ecrecover-and-compare),
                            //                            but we keep it on the wire for consistency with Ethereum-native tooling / block explorers.
                            // See §4.2 for the verification procedure.
  SigFormat sig_format = 3; // SIG_FORMAT_EIP191 only valid on new-side single-key proofs
}

message MultisigProof {
  uint32 threshold               = 1; // K
  repeated bytes sub_pub_keys    = 2; // all N sub-keys, original ordering, 33 bytes each
  repeated uint32 signer_indices = 3; // exactly K distinct indices, strictly ascending
  repeated bytes sub_signatures  = 4; // same order as signer_indices
  SigFormat sig_format           = 5; // SIG_FORMAT_EIP191 is INVALID for multisig (no wallet supports multisig EIP-191)
}
```

**`proto/lumera/evmigration/tx.proto` — message changes:**

```proto
message MsgClaimLegacyAccount {
  string new_address          = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string legacy_address       = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  MigrationProof legacy_proof = 3 [(gogoproto.nullable) = false];
  MigrationProof new_proof    = 4 [(gogoproto.nullable) = false];
}

message MsgMigrateValidator {
  string new_address          = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string legacy_address       = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  MigrationProof legacy_proof = 3 [(gogoproto.nullable) = false];
  MigrationProof new_proof    = 4 [(gogoproto.nullable) = false];
}
```

The flat `new_signature` bytes field is removed entirely — replaced by a `new_proof` field carrying the same shape as `legacy_proof`. Since the EVM upgrade has not been deployed to any network, no `reserved` tags are needed; field numbers are chosen for clarity, not continuity.

**`proto/lumera/evmigration/params.proto` — unchanged vs prior draft:**

```proto
message Params {
  // existing fields: enable_migration=1, migration_end_time=2,
  //                  max_migrations_per_block=3, max_validator_delegations=4
  uint32 max_multisig_sub_keys = 5; // default 20, enforced on BOTH sides
}
```

**`proto/lumera/evmigration/query.proto` — extend `LegacyAccountInfo` (same as prior draft):**

```proto
message LegacyAccountInfo {
  // existing fields: address=1, balance_summary=2, has_delegations=3, is_validator=4
  bool   is_multisig   = 5;
  uint32 threshold     = 6; // 0 when !is_multisig
  uint32 num_signers   = 7; // 0 when !is_multisig
}
```

### 4.2 Verifier

`x/evmigration/keeper/verify.go` replaces `VerifyLegacySignature`/`VerifyNewSignature` with `VerifyMigrationProof`. The `SubKeyType` enum and the per-sub-key signature primitives live in a shared package `x/evmigration/types/sigverify` so both the keeper verifier and the CLI's `combine-proof` import them from a single source of truth (prevents drift between the two verification paths):

```go
// In x/evmigration/types/sigverify/sigverify.go (shared package)
type SubKeyType int

const (
    SubKeyTypeCosmosSecp256k1 SubKeyType = iota + 1 // legacy side
    SubKeyTypeEthSecp256k1                          // new side
)
```

```go
// In x/evmigration/keeper/verify.go
import "github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"

func VerifyMigrationProof(
    chainID string, evmChainID uint64, kind string,
    legacyAddr, newAddr, boundAddr sdk.AccAddress,
    proof *types.MigrationProof,
    expectedSubKey sigverify.SubKeyType,
) error {
    payload := migrationPayload(chainID, evmChainID, kind, legacyAddr, newAddr)
    switch p := proof.Proof.(type) {
    case *types.MigrationProof_Single:
        return verifySingleKeyProof(payload, boundAddr, p.Single, expectedSubKey)
    case *types.MigrationProof_Multisig:
        return verifyMultisigProof(payload, boundAddr, p.Multisig, expectedSubKey)
    default:
        return types.ErrInvalidMigrationProof.Wrap("no proof set")
    }
}
```

- `boundAddr` is `legacyAddr` for the legacy proof and `newAddr` for the new proof — whichever address the proof is asserting control over.
- `verifySingleKeyProof` constructs a Cosmos `secp256k1.PubKey` or an `ethsecp256k1.PubKey` based on `expectedSubKey`, derives the bech32 using that type's address convention, and compares to `boundAddr`.
- `verifyMultisigProof` reconstructs `kmultisig.NewLegacyAminoPubKey(K, subKeys)` with sub-keys of the expected type, asserts `Address() == boundAddr`, then verifies each sub-signature using the matching sub-key's own `VerifySignature` method.
- `verifySecp256k1Sig` stays as the shared single-sig helper; a new `verifyEthSecp256k1Sig` handles eth sub-sigs with Keccak256-based hashing and supports the EIP-191 envelope. Both accept a `SigFormat`.
- The address-derivation convention is chosen per sub-key type: Cosmos `secp256k1` uses amino-encoded `RIPEMD160(SHA256(pub))`; eth uses `Keccak256(uncompressed_pub)[12:]`. For multisig, the *outer* address is always amino-encoded `LegacyAminoPubKey.Address()` regardless of sub-key type — this was confirmed by the devnet spike.

**Eth signature wire format — strict 65 bytes (aligned with Ethereum convention):**

New-side `eth_secp256k1` signatures are **always 65 bytes on the wire**: `R (32) || S (32) || V (1)`. Cosmos EVM v0.6.0's `ethsecp256k1.PrivKey.Sign` produces 65 bytes; Keplr/Leap `personal_sign` produces 65 bytes; go-ethereum's `crypto.Sign` (which these wrap) produces 65 bytes. There is no realistic path in our signing flow that produces a 64-byte eth signature, so requiring exactly 65 at the wire layer is a tighter contract than "accept either."

The V byte is retained on the wire because that's what Ethereum-native tooling produces and what block explorers expect; the verifier doesn't use it (we do ECDSA-verify-under-pubkey, not ecrecover-and-compare — §4.2 rationale below).

`ValidateBasic` enforces `len(signature) == 65` for every eth sub-signature and for new-side single-key `SingleKeyProof.signature`. The corresponding Cosmos-side rule is `len(signature) == 64` — no V convention on the Cosmos side.

**Verification procedure (uniform across CLI, ADR-036, and EIP-191):**

1. Build the format-specific message bytes:
   - `SIG_FORMAT_CLI`: `msg = payload` (the keyring internally applied Keccak256 during signing; VerifySignature re-applies Keccak256 to match).
   - `SIG_FORMAT_ADR036`: `msg = ADR036SignDoc(signer_addr, payload)`.
   - `SIG_FORMAT_EIP191`: `msg = "\x19Ethereum Signed Message:\n" || decimal(len(payload)) || payload`.
2. Slice to R||S: `verify_sig = proof.signature[:64]` (proof.signature is known to be exactly 65 bytes post-ValidateBasic).
3. Call `ethsecp256k1.PubKey{Key: proof.pub_key}.VerifySignature(msg, verify_sig)`. The SDK pubkey applies `Keccak256(msg)` internally and performs direct ECDSA verification against the supplied pubkey.
4. Independently, assert `sdk.AccAddress(ethsecp256k1.PubKey{...}.Address()) == new_address` (performed by `verifySingleKeyProof` for all formats).

**Rationale for direct-verification over ecrecover-and-compare:**

A recovery-based scheme would run `ecrecover(Keccak256(wrapped), raw_sig)` to derive the signer's pubkey, then compare it to the supplied `pub_key`. That's equivalent information-theoretically, but introduces two failure modes that direct verification avoids: (a) malleable recovery IDs (v=27/28/0/1 conventions diverge across clients), and (b) ambiguity when the supplied `pub_key` and the recovered pubkey differ — the verifier would have to decide whether to trust the proof's `pub_key` or the recovered one. Direct verification under the supplied pubkey has a single source of truth.

CLI and verifier implementations must use this exact procedure; divergence between the two would produce false rejects.

### 4.3 ValidateBasic

`types/proof.go` (renamed from `LegacyProof` → `MigrationProof`) factors per-proof validation into two tiers.

**Tier 1 — stateless (`MigrationProof.ValidateBasic(side)`):**

Called from `MsgClaimLegacyAccount.ValidateBasic` and `MsgMigrateValidator.ValidateBasic` — once with `side=legacy` for `legacy_proof`, once with `side=new` for `new_proof`. Dispatches to `SingleKeyProof.validateBasic(side)` or `MultisigProof.validateBasic(side)`:

- `SingleKeyProof.validateBasic(side)` enforces 33-byte pubkey, non-empty signature, specified `sig_format`; rejects `SIG_FORMAT_EIP191` on legacy side.
- `MultisigProof.validateBasic(side)` enforces:
  - `N ≥ 1`, `1 ≤ threshold ≤ N`
  - `len(signer_indices) == threshold` (exact-K rule)
  - `len(sub_signatures) == len(signer_indices)`
  - `signer_indices` strictly ascending (enforces uniqueness + canonical ordering)
  - Every index in range
  - **Every `sub_pub_keys[i]` is 33 bytes for all `i ∈ [0, N)`** — not just indexed ones. The address derivation via `LegacyAminoPubKey.Address()` uses *all* N sub-keys, so a malformed unindexed sub-key would pass `ValidateBasic` but cause a cryptic failure during `verifier.verifyMultisigProof` address reconstruction. Enforcing length on all sub-keys statelessly gives callers an immediate and clear error.
  - `sig_format ∈ {CLI, ADR036}` (EIP-191 rejected for multisig on both sides)

**Tier 2 — param-aware (`MigrationProof.ValidateParams(maxSubKeys uint32)`):**

Called from the msg server immediately after loading params and before invoking the verifier. Enforces `N ≤ maxSubKeys` on the multisig path; no-op on the single-key path. Applied to both `legacy_proof` and `new_proof`.

### 4.4 Legacy-account detection

`x/evmigration/keeper/query.go` (`remainingLegacyAccountStatus`) — same as prior draft. `isLegacyPubKey` treats Cosmos `secp256k1` *and* flat multisig-of-Cosmos-secp256k1 as legacy; everything else is skipped (including multisig-of-eth, which is a "new-side" shape that wouldn't appear as a legacy account anyway).

The `LegacyAccounts` query populates `is_multisig`, `threshold`, and `num_signers` when the pubkey is multisig.

### 4.4.1 MigrationEstimate preflight

Unchanged from prior draft. `MigrationEstimate` surfaces `is_multisig`/`threshold`/`num_signers` and rejects nested/non-secp256k1 sub-keys and oversized multisigs. The destination shape is implied by the source shape, so no additional input is needed from the client.

### 4.5 CLI multi-step flow

Four subcommands under `lumerad tx evmigration`, structurally identical to the prior draft but with combined legacy+new signing in each partial.

1. **`generate-proof-payload`** — Queries on-chain legacy account, produces a `PartialProof` JSON seeded with:
   - `legacy` block: threshold + sub-keys if legacy is multisig; `pub_key` if single.
   - `new` block: the co-signers must agree on the new multisig shape *before* running this command — either by pre-registering eth sub-keys and passing `--new-sub-pub-keys k1,k2,k3 --new-threshold 2`, or for single-key migrations just passing `--new-key <eth-key-name>` (and the CLI derives the eth pubkey + bech32).
   - `payload_hex`, `chain_id`, `evm_chain_id`, `kind`, `legacy_address`, `new_address` (derived from the new-side multisig or single eth pubkey).
   - Empty `partial_legacy_signatures` and `partial_new_signatures` arrays.

   New validation: the command verifies that `new_address` derives from the supplied new-side pubkey material and that no co-signer's new eth sub-key is reused from the legacy side (catches the "forgot to generate a fresh eth key" mistake).

2. **`sign-proof <file> [--from <legacy-key>] [--new-key <eth-key>] [--out <file>]`** — A co-signer runs this on their own machine against their own keyring. Contributes to both halves in one invocation:
   - If `--from` is supplied, matches the legacy key's pubkey against `legacy.sub_pub_keys` to determine its index, signs the payload in the legacy `sig_format`, and appends to `partial_legacy_signatures`.
   - If `--new-key` is supplied, matches against `new.sub_pub_keys` (or `new.pub_key` for single-key case), signs in the new-side `sig_format`, and appends to `partial_new_signatures`.
   - At least one of `--from`/`--new-key` must be supplied; both is the common case (co-signers hold both keys).
   - Idempotent: re-signing with the same key overwrites that index's entry, never duplicates.

3. **`combine-proof <partial1.json> [<partial2.json> …] --out <tx.json>`** — Accepts one or more partial files. Validates all inputs share the same `legacy_address`, `new_address`, `chain_id`, `evm_chain_id`, `kind`, `sig_format` (per side), `threshold` (per side), and `sub_pub_keys` (per side). Merges `partial_legacy_signatures` and `partial_new_signatures` from all inputs, deduplicating by `index` (keeping the last occurrence, for idempotency).

   **Per-partial cryptographic verification during combine:** before threshold selection, the coordinator verifies *every* merged partial signature against its claimed sub-pubkey and the canonical payload, using the same per-sub-key helpers as the keeper verifier (`verifyCosmosSecp256k1Sig` for legacy sub-sigs, `verifyEthSecp256k1Sig` for new sub-sigs). Invalid partials are dropped with a visible warning identifying the offending `index` and the failure reason. This matches the behavior of the current (pre-revision) `combine-proof` implementation and prevents a stale or corrupted partial with a low index from poisoning the combined tx when other valid partials exist at higher indices.

   After verification, `combine-proof` selects the **K valid partials with the lowest ascending indices** on each side (canonical ordering) and assembles `MigrationProof{Multisig}` on each side. If fewer than K partials verify on either side, it errors with `need <K> valid partial signatures on <side>, have <N>` and writes nothing. For single-key sides, expects exactly one entry (which is still verified before inclusion).

   **Shared verification helpers:** to avoid CLI/keeper divergence, the per-sub-key verification primitives (`verifyCosmosSecp256k1Sig`, `verifyEthSecp256k1Sig`, `eip191PersonalSignPayload`, `adr036SignDoc`) live in a shared package `x/evmigration/types/sigverify` (or `x/evmigration/crypto`), imported by both the keeper's verifier and the CLI's `combine-proof`. Single source of truth per the §4.2 rationale.

4. **`submit-proof <tx.json>`** — The tx has both proofs assembled at the application level (`legacy_proof` and `new_proof`). This command runs `ValidateBasic`, simulates gas via the migration-specific estimator, builds an **unsigned** tx, and broadcasts. **No `--from` key of any kind.** `MsgClaimLegacyAccount` and `MsgMigrateValidator` declare zero signers at the proto layer — the new EVM account doesn't exist on-chain yet at submit time (it's materialized *by* the migration), so no account is available to sign the Cosmos envelope. The chicken-and-egg is resolved by not requiring envelope signing: authorization is fully embedded in the two proofs, fees are waived by the evmigration ante handler, and replay protection comes from `MigrationRecords.Has(legacyAddr)` in the keeper's preChecks. Adding any envelope signer yields a "expected 0, got 1" validation error.

**Single-key ergonomics preserved.** The existing one-shot `claim-legacy-account <legacy-key> <new-key>` and `migrate-validator <legacy-key> <new-key>` commands remain for single-sig users. Internally they build `MigrationProof{Single}` on both sides — no behavioral change.

**`PartialProof` JSON schema** (unversioned, not a proto):

```json
{
  "version": 2,
  "kind": "claim",
  "legacy_address": "lumera1…",
  "new_address":    "lumera1…",
  "chain_id": "lumera-devnet-1",
  "evm_chain_id": 76857769,
  "payload_hex": "6c756d6572612d65766d2d6d6967726174696f6e3a…",
  "legacy": {
    "threshold": 2,
    "sub_pub_keys": ["AxYZ…", "AiBC…", "AjKL…"],
    "sig_format": "SIG_FORMAT_CLI"
  },
  "new": {
    "threshold": 2,
    "sub_pub_keys": ["AyIS…", "A9X6…", "A3Hr…"],
    "sig_format": "SIG_FORMAT_CLI"
  },
  "partial_legacy_signatures": [
    { "index": 0, "signature": "base64…" },
    { "index": 2, "signature": "base64…" }
  ],
  "partial_new_signatures": [
    { "index": 0, "signature": "base64…" },
    { "index": 2, "signature": "base64…" }
  ]
}
```

For single-sig sides, `legacy` or `new` is replaced with `{ "pub_key": "…", "sig_format": "…" }` and the corresponding `partial_*_signatures` has exactly one entry at `index: 0`.

`version: 2` — the existing on-disk format (shipped on branch `evm` but not released) is `version: 1`, with a top-level `single`/`multisig` choice and a flat `partial_signatures` slice. This revision changes the schema incompatibly (legacy+new sides each get their own side-spec and their own partial-signature slice), so the version bump is load-bearing: old v1 files must fail with a clear version-mismatch error rather than parse as v2 and trip over missing side fields.

### 4.6 Destination account persistence (new section)

`x/evmigration/keeper/migrate_auth.go:77-81` currently creates a `BaseAccount` at `newAddr` with `pubkey == nil`. For single-key destinations this is fine — the pubkey populates on first EVM-signed tx.

For multisig destinations, leave-as-nil would mean the new multisig's shape is discoverable only via the migration record, not the account keeper. That breaks tooling and creates the same "nil pubkey, sign any tx first" UX trap that the legacy side currently has.

**Rule:** the destination account must be **fresh (no existing account) OR a plain `*BaseAccount`**. Any other pre-existing type is rejected. For multisig destinations, if the existing `*BaseAccount` already has a pubkey, it must be byte-equal to the reconstructed multisig; mismatched pubkey yields `ErrPubKeyAddressMismatch`. **All rejection cases fire pre-mutation** — if any check fails, no state has been written yet, so the chain cannot end up partially migrated.

`MigrateAuth` is structured in two phases. Phase 1 gathers all information needed to make a pass/reject decision and performs all checks; Phase 2 executes state mutation only if Phase 1 passes.

```go
// --- PHASE 1: all pre-mutation checks. No state writes below this line. ---

// Phase-1 check A: stateless proof validation (cheap; do first).
if destProof != nil {
    if err := destProof.ValidateBasic(types.SideNew); err != nil {
        return nil, err
    }
}

// Phase-1 probe: single GetAccount(newAddr) call, cached for reuse in Phase 2.
existingNewAcc := k.accountKeeper.GetAccount(ctx, newAddr)

// Phase-1 check B: destination-account type safety.
if existingNewAcc != nil {
    if _, ok := existingNewAcc.(sdk.ModuleAccountI); ok {
        return nil, types.ErrCannotMigrateModuleAccount.Wrapf(
            "destination %s is a module account", newAddr,
        )
    }
    if _, ok := existingNewAcc.(*authtypes.BaseAccount); !ok {
        return nil, types.ErrPubKeyAddressMismatch.Wrapf(
            "destination %s has non-BaseAccount type %T; migration to existing special accounts (vesting, module, etc.) is not supported — choose a fresh destination",
            newAddr, existingNewAcc,
        )
    }
}

// Phase-1 check C: multisig reconstruction, address binding, and
// pubkey-compatibility on the cached pre-existing account. SDK 0.53.6's
// BaseAccount.SetPubKey is an unconditional overwrite, so without this
// pre-mutation guard we'd silently replace a different legitimate pubkey
// during Phase 2 after legacy removal had already happened.
var destMultiPK cryptotypes.PubKey
if destProof != nil {
    if ms := destProof.GetMultisig(); ms != nil {
        subKeys := make([]cryptotypes.PubKey, len(ms.SubPubKeys))
        for i, raw := range ms.SubPubKeys {
            subKeys[i] = &ethsecp256k1.PubKey{Key: raw}
        }
        multiPK := kmultisig.NewLegacyAminoPubKey(int(ms.Threshold), subKeys)
        if !sdk.AccAddress(multiPK.Address()).Equals(newAddr) {
            return nil, types.ErrPubKeyAddressMismatch.Wrapf(
                "destination multisig pubkey derives to %s, expected %s",
                sdk.AccAddress(multiPK.Address()), newAddr,
            )
        }
        if existingNewAcc != nil {
            if existingPK := existingNewAcc.GetPubKey(); existingPK != nil {
                if !bytes.Equal(existingPK.Bytes(), multiPK.Bytes()) {
                    return nil, types.ErrPubKeyAddressMismatch.Wrapf(
                        "destination account %s already has a different pubkey; refusing to overwrite",
                        newAddr,
                    )
                }
                // existing pubkey == multiPK → idempotent re-run case.
            }
        }
        destMultiPK = multiPK
    }
}

// --- PHASE 2: state mutation. All pre-mutation checks have passed. ---

// ... existing vesting-capture + legacy-account-removal logic ...

// Materialize newAcc from the cached probe (single GetAccount(newAddr) discipline).
var newAcc sdk.AccountI
if existingNewAcc != nil {
    newAcc = existingNewAcc
} else {
    newAcc = k.accountKeeper.NewAccountWithAddress(ctx, newAddr)
}

// SetPubKey only when the existing slot is nil — Phase-1 check C already
// proved that any non-nil existing pubkey matches destMultiPK.
if destMultiPK != nil && newAcc.GetPubKey() == nil {
    if err := newAcc.SetPubKey(destMultiPK); err != nil {
        return nil, err
    }
}

k.accountKeeper.SetAccount(ctx, newAcc)
```

This runs inside `MigrateAuth` (for `MsgClaimLegacyAccount`) and in the equivalent account-materialization step of `MsgMigrateValidator`.

**Why the type-safety rule is strict:** the existing `FinalizeVestingAccount` at [migrate_auth.go:95](x/evmigration/keeper/migrate_auth.go#L95) handles a non-BaseAccount destination by extracting the BaseAccount core and rebuilding the destination as a new vesting account with the *legacy's* vesting parameters. That's a silent clobber of any pre-existing special-type state at `newAddr`: a continuous-vesting destination would lose its schedule; a module account would be overwritten; a future smart-account type would lose its state. Rather than encode per-type clobber/preserve semantics, the minimum-surprise rule is "fresh or plain BaseAccount only." Users who want to migrate into a pre-existing special-type address must first convert it or pick a different destination.

**Summary of accepted destination states** — the shape-agnostic rules (type-safety) are shared; the pubkey-compatibility rule is multisig-specific. Two behaviours per pre-existing state:

| State at `newAddr` before migration | Single-key destination (destProof has `SingleKeyProof`) | Multisig destination (destProof has `MultisigProof`) |
|-------------------------------------|--------------------------------------------------------|------------------------------------------------------|
| No account exists (fresh) | ✅ `NewAccountWithAddress`; pubkey stays nil (ante handler sets on first signed tx) | ✅ `NewAccountWithAddress` + `SetPubKey(multiPK)` |
| Plain `*BaseAccount`, nil pubkey | ✅ Reuse existing; pubkey stays nil | ✅ Reuse existing + `SetPubKey(multiPK)` |
| Plain `*BaseAccount`, pubkey byte-equal to `multiPK` | ✅ Reuse existing; pubkey untouched (single-key path does not compare pubkeys) | ✅ (idempotent) Reuse existing; skip `SetPubKey` |
| Plain `*BaseAccount`, pubkey different from `multiPK` | ✅ Reuse existing; pubkey untouched (single-key path does not compare pubkeys) | ❌ `ErrPubKeyAddressMismatch` (would silently overwrite) |
| Module account | ❌ `ErrCannotMigrateModuleAccount` | ❌ `ErrCannotMigrateModuleAccount` |
| Vesting account (any variant) | ❌ `ErrPubKeyAddressMismatch` ("non-BaseAccount type") | ❌ `ErrPubKeyAddressMismatch` ("non-BaseAccount type") |
| Any other special type | ❌ `ErrPubKeyAddressMismatch` ("non-BaseAccount type") | ❌ `ErrPubKeyAddressMismatch` ("non-BaseAccount type") |

**Why the single-key column preserves pre-existing pubkeys:** the single-key destination path never calls `SetPubKey` — the ante handler populates the pubkey on the user's first signed tx, and that path already performs its own match check between the signer's recovered pubkey and `newAddr`'s eth-style derivation. So a pre-existing pubkey on `newAddr` doesn't need to be validated at migration time; it's either going to match what the user will sign with (first-tx ante succeeds) or not (first-tx ante rejects). Comparing pubkeys at migration time would be redundant with the ante handler's check.

**Why the multisig column rejects different pubkeys:** the multisig destination path DOES call `SetPubKey(multiPK)` (see Q7 rationale — we want the on-chain account to immediately reflect the K-of-N shape). SDK 0.53.6's `BaseAccount.SetPubKey` is an unconditional overwrite, so without a pre-mutation compatibility check we'd silently replace whatever was there.

**Why the account-type row is shape-agnostic:** `FinalizeVestingAccount` extracts the BaseAccount core from any pre-existing special-type account at `newAddr` and rebuilds the destination as a new vesting account with the *legacy's* vesting parameters — regardless of destProof shape. This clobbers pre-existing vesting state whether the destProof is single-key or multisig. Similarly, module accounts must never be migration targets regardless of shape. So both the "vesting/special → reject" and "module → reject" rules apply uniformly.

### 4.7 Msg-server callers

`msg_server_claim_legacy.go` and `msg_server_migrate_validator.go` change to call `VerifyMigrationProof` twice:

```go
if err := msg.LegacyProof.ValidateParams(params.MaxMultisigSubKeys); err != nil { … }
if err := msg.NewProof.ValidateParams(params.MaxMultisigSubKeys); err != nil { … }
if err := VerifyMigrationProof(
    ctx.ChainID(), evmChainID, migrationPayloadKindClaim,
    legacyAddr, newAddr, legacyAddr,
    &msg.LegacyProof, sigverify.SubKeyTypeCosmosSecp256k1,
); err != nil { … }
if err := VerifyMigrationProof(
    ctx.ChainID(), evmChainID, migrationPayloadKindClaim,
    legacyAddr, newAddr, newAddr,
    &msg.NewProof, sigverify.SubKeyTypeEthSecp256k1,
); err != nil { … }
```

The `boundAddr` parameter is `legacyAddr` for the legacy proof verification and `newAddr` for the new proof verification — whichever address the proof is proving control over.

## 5. Testing Strategy

### 5.1 Unit tests (`x/evmigration/`)

Extend existing files:

- **`keeper/verify_test.go`** — add `TestVerifyMigrationProof_LegacyMultisig`, `TestVerifyMigrationProof_NewMultisig`, `TestVerifyMigrationProof_BothMultisig` covering: valid 2-of-3 CLI/ADR-036, exact-K pass, K-1 reject, K+1 reject, invalid sub-sig, non-ascending indices, out-of-range index, address mismatch, 1-of-1 edge, N=`MaxMultisigSubKeys` boundary, N+1 reject, mixed sub-key types reject (per side), EIP-191 rejected for multisig, Cosmos `secp256k1` rejected on new side, `ethsecp256k1` rejected on legacy side.
- **`keeper/migrate_test.go`** — `TestMigrateMultisigAccount_ToMultisig` covering base + vesting + authz/feegrant variants + already-migrated reject + `BaseAccount.PubKey` on new address equals reconstructed eth multisig.
- **`keeper/msg_server_migrate_validator_test.go`** — `TestMigrateValidator_MultisigOperator_ToMultisig`: delegations re-keyed, distribution state re-keyed, supernode record re-keyed, consensus pubkey unchanged, new operator address is a multisig-of-eth that can later sign `MsgEditValidator` (follow-on assertion).
- **`keeper/query_test.go`** — `TestLegacyAccounts_Multisig`; `TestMigrationStats_IncludesMultisig`; `TestMigrationEstimate_Multisig_Supported` (WouldSucceed=true); `TestMigrationEstimate_Multisig_NonSecp256k1SubKey`, `TestMigrationEstimate_Multisig_TooManySubKeys`, `TestMigrationEstimate_Multisig_NestedRejected`.
- **New `types/proof_test.go`** — every `ValidateBasic(side)` rejection branch, for both sides.

### 5.2 Integration tests (`tests/integration/evmigration/`)

- `TestMsgClaimLegacyAccount_MultisigToMultisig` — 2-of-3 legacy → 2-of-3 new, balance migration, `BaseAccount.PubKey` set correctly.
- `TestMsgClaimLegacyAccount_MultisigVesting_ToMultisig` — continuous vesting preserved under multisig destination.
- `TestMsgMigrateValidator_MultisigToMultisig` — delegations, distribution, supernode re-keyed; assert new operator can sign `MsgEditValidator` post-migration.
- `TestMsgClaimLegacyAccount_Multisig_WrongThreshold` — K-1 legacy sigs OR K-1 new sigs rejected.
- `TestMsgClaimLegacyAccount_Multisig_ReplayRejected` — re-submit fails.
- `TestMsgClaimLegacyAccount_Multisig_ADR036_Both` — ADR-036 end-to-end on both sides.
- `TestMsgClaimLegacyAccount_SingleKeyToSingleKey` — regression test ensuring current single→single flow still works.

Shared helpers in `tests/integration/evmigration/multisig_helpers.go`:

```go
func buildLegacyMultisig(t, ctx, N, K int) (addr sdk.AccAddress, subKeys []*secp256k1.PrivKey, pubKey *kmultisig.LegacyAminoPubKey)
func buildNewMultisig(N, K int) (addr sdk.AccAddress, subKeys []*ethsecp256k1.PrivKey, pubKey *kmultisig.LegacyAminoPubKey)
func signMultisigMigrationProof(payload []byte, subKeys []cryptotypes.PrivKey, signerIdxs []int, format types.SigFormat) *types.MultisigProof
```

### 5.3 CLI tests (`x/evmigration/client/cli/tx_test.go`)

- `TestGenerateProofPayload_MultisigToMultisig` — JSON output well-formed; legacy and new sub-pubkeys seeded correctly.
- `TestGenerateProofPayload_NewSubKeyReusesLegacy_Rejected` — catches mistaken reuse of a Cosmos sub-key as an eth sub-key.
- `TestGenerateProofPayload_*` — all prior-draft cases (on-chain pubkey, nil pubkey with/without `--legacy-key`, wrong `--legacy-key`, multisig nil-pubkey rejection) still apply.
- `TestSignProof_SignsBothSides` — `--from` + `--new-key` together appends to both `partial_legacy_signatures` and `partial_new_signatures`.
- `TestSignProof_LegacyOnly` — `--from` alone; no entry added to `partial_new_signatures`.
- `TestSignProof_NewOnly` — `--new-key` alone; no entry added to `partial_legacy_signatures`.
- `TestSignProof_Idempotent` — resigning with same key (either side) overwrites, does not duplicate.
- `TestCombineProof_CanonicalOrdering` — out-of-order signers produce byte-identical tx.
- `TestCombineProof_MultiFile` — merges partials from N separate files.
- `TestCombineProof_MismatchedPayloadsRejected` — divergent `chain_id`/`legacy_address`/`new_address`/`kind`/`sig_format`/`threshold`/`sub_pub_keys` across partial files rejected.
- `TestCombineProof_BelowThresholdRejected_Legacy` — `len(legacy) < legacy_threshold` rejected.
- `TestCombineProof_BelowThresholdRejected_New` — `len(new) < new_threshold` rejected.
- `TestSubmitProof_MultisigToMultisig` — full four-step against mock chain.

### 5.4 Devnet tests (`devnet/tests/evmigration/`)

- `multisig_keys.go` — seeds a 2-of-3 Cosmos multisig with balances, delegations, and an authz grant; pre-provisions three fresh eth_secp256k1 keys for the new side. Triggers one trivial signed tx from the legacy multisig pre-test to ensure `acc.GetPubKey()` is non-nil.
- `multisig_test.go` — end-to-end separate-machine flow with combined legacy+new signing. Verifies `MigrationRecord`, balances at new multisig bech32, `BaseAccount.PubKey` equals reconstructed `LegacyAminoPubKey` over the eth sub-keys, delegations re-keyed, replay rejected. Also includes a `shared-file` variant.
- `multisig_validator_test.go` — same for a multisig validator operator. **Additional assertion**: post-migration, run a `MsgEditValidator` from the new multisig-of-eth operator (reuses the devnet spike's demonstrated flow) and verify the moniker updates.
- `multisig_estimate_test.go` — as in prior draft: supported 2-of-3 multisig (expect `would_succeed=true`); oversized multisig; nested multisig; non-secp256k1 sub-keys.

### 5.5 Documentation updates

- `docs/evm-integration/tests.md` — new rows under evmigration for multisig-to-multisig tests.
- `docs/evm-integration/evmigration.md` — section "Multisig account migration" covering the mirror-source rule, new-side multisig assembly, and the four-step CLI example with combined partials.
- `docs/evm-integration/unit-evmigration.md`, `docs/evm-integration/integration-evmigration.md` — coverage summaries.
- `docs/evm-integration/evmigration/portal-ui.md` — frontend implications: portal UIs need to help users construct the new multisig shape (pre-provision N eth keys, derive `new_address` from `LegacyAminoPubKey(K, ethSubKeys)`, display for confirmation).

## 6. Rollout

The EVM upgrade has not been deployed to any network, so there are no in-flight on-chain messages, no pending txs in the mempool, and no clients depending on current wire formats. Proto changes are clean-slate renames — no `reserved` tags, no shims:

1. Land proto changes (rename `LegacyProof` → `MigrationProof`, remove `new_signature`, add `new_proof`, `make build-proto`).
2. Update `ValidateBasic`, verifier, keeper msg-server callers (both proof halves).
3. Update `MigrateAuth` to persist multisig destination pubkey on the new `BaseAccount`.
4. Update CLI: retire the `ECDSA-recovery` new-side signature handling; keep single-key one-shot commands (`claim-legacy-account`, `migrate-validator`); extend the four-step flow to sign both halves per partial.
5. Update `LegacyAccounts` query to include multisig (unchanged vs prior draft).
6. Add unit + integration + CLI tests.
7. Add devnet scenarios.
8. Update docs.

`MaxMultisigSubKeys = 20` is set at module init; adjustable via existing `MsgUpdateParams`.

## 7. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Multisig address reconstruction diverges from SDK's amino serialization | Use `kmultisig.NewLegacyAminoPubKey` — same constructor as `lumerad keys add --multisig`. Devnet spike confirmed this works for both `secp256k1` and `ethsecp256k1` sub-keys. |
| `LegacyAminoPubKey` with `ethsecp256k1` sub-keys is a less-trodden SDK path — unknown edge cases | **De-risked by the 2026-04-22 devnet spike**: 2-of-3 eth-sub-key multisig successfully signed `MsgSend`, `MsgCreateValidator`, and `MsgEditValidator` (with a different K-of-N subset for each tx). The SDK's `VerifyMultisignature` correctly dispatches per-sub-key hash conventions. |
| N=`MaxMultisigSubKeys` tx with all invalid sigs → up to 20 verifications before reject (DoS) — now 2× since both sides verified | Bounded by `MaxMultisigSubKeys = 20` × 2 = 40 worst-case verifications. Migration msg itself is fee-free, but tx bytes are still metered by the wrapping tx. Acceptable for a one-time migration window. |
| Co-signer signs with wrong chain-id | Error message from verify.go hints at chain-id mismatch; hint extended to multisig path. |
| `PartialProof` format drift across future `lumerad` versions | `version: 2` field (bumped from the `evm`-branch v1 that had a different shape); `combine-proof` rejects unknown fields and unsupported versions. No mainnet release shipped v1, so no compat shim needed — v1 files produce a clear error. |
| Multisig sub-signer loses access mid-coordination | Out of scope — same problem exists for any multisig; pre-migration key rotation on the legacy chain is the only remedy. |
| User mistakenly reuses legacy Cosmos `secp256k1` sub-key as new-side eth sub-key | `generate-proof-payload` validates that no new-side pubkey appears in the legacy sub-key set. `sign-proof` also cross-validates the key type (eth vs Cosmos) before signing. |
| User discovers mid-migration that their legacy account has nil on-chain pubkey | Same remediation as prior draft: `generate-proof-payload` branches on intent; error message guides the user. |
| Co-signers on separate machines produce partial files that disagree | `combine-proof` validates that all inputs agree on `legacy_address`, `new_address`, `chain_id`, `evm_chain_id`, `kind`, `sig_format` (per side), `threshold` (per side), and `sub_pub_keys` (per side). Error message identifies the first divergent field. |
| Destination multisig `BaseAccount.PubKey` nil at migration time → downstream tooling can't see the multisig shape | `MigrateAuth` explicitly calls `acc.SetPubKey(multiPK)` for multisig destinations (section 4.6). Single-key destinations retain nil-pubkey behavior. |
| Operators fear that migrating a sub-signer's individual account "breaks" their multisig's ability to migrate later | **Non-risk by design** — sub-signer and multisig migrations are mutually independent. The multisig's `LegacyAminoPubKey` is stored inline on the multisig's own `BaseAccount.PubKey` (containing every sub-pubkey and the threshold), so removing a sub-signer's individual x/auth account does not affect it. Signing is an offline private-key operation — the sub-signer's private key exists regardless of chain state. The verifier never consults x/auth about sub-signer account existence; it reconstructs the multisig from pubkey bytes in the proof and verifies each sub-signature against the claimed sub-pubkey. Migration order is completely free (all sub-signers first / multisig first / interleaved / some sub-signers never migrate — each works identically). Documented explicitly in the supernode-migration user guide per plan Task 25. |

## 8. File-Change Inventory

### New files

- `proto/lumera/evmigration/proof.proto` (renamed semantics; file exists already, content rewritten)
- `x/evmigration/types/proof.go` (exists; rewritten for `MigrationProof`, side-aware `ValidateBasic`)
- `x/evmigration/types/proof_test.go` (exists; rewritten)
- `x/evmigration/client/cli/tx_multisig.go` (exists; updated for dual-side signing)
- `x/evmigration/client/cli/tx_multisig_test.go` (exists; updated)
- `tests/integration/evmigration/multisig_helpers.go` (exists; adds `buildNewMultisig`, `signMultisigMigrationProof`)
- `devnet/tests/evmigration/multisig_keys.go` (exists; updated to pre-provision eth sub-keys)
- `devnet/tests/evmigration/multisig_test.go` (exists; updated for dual-side flow)
- `devnet/tests/evmigration/multisig_validator_test.go` (exists; updated + post-migration `MsgEditValidator` assertion)
- `devnet/tests/evmigration/multisig_estimate_test.go` (exists; mostly unchanged)

### Modified files

- `proto/lumera/evmigration/tx.proto` — replace `new_signature` with `new_proof`
- `proto/lumera/evmigration/params.proto` — unchanged vs prior draft
- `proto/lumera/evmigration/query.proto` — unchanged vs prior draft
- `x/evmigration/types/types.go` — rename references from `LegacyProof` to `MigrationProof`
- `x/evmigration/types/params.go` — unchanged vs prior draft
- `x/evmigration/keeper/verify.go` — replace `VerifyLegacySignature`/`VerifyNewSignature` with unified `VerifyMigrationProof`; add `verifyEthSecp256k1Sig`
- `x/evmigration/keeper/verify_test.go` — rewritten
- `x/evmigration/keeper/msg_server_claim_legacy.go` — dual `VerifyMigrationProof` calls
- `x/evmigration/keeper/msg_server_migrate_validator.go` — dual `VerifyMigrationProof` calls
- `x/evmigration/keeper/migrate_auth.go` — `SetPubKey(multiPK)` when new-side is multisig (section 4.6)
- `x/evmigration/keeper/query.go` — unchanged vs prior draft
- `x/evmigration/keeper/migrate_test.go` — new multisig-to-multisig case
- `x/evmigration/keeper/query_test.go` — unchanged vs prior draft
- `x/evmigration/client/cli/tx.go` — retire ECDSA-recovery new-side helper; route single-key flow through `MigrationProof{Single}`
- `x/evmigration/module/autocli.go` — already `Skip: true` for the two affected RPCs; no change
- `tests/integration/evmigration/migration_test.go` — update to new schema
- `docs/evm-integration/tests.md`
- `docs/evm-integration/evmigration.md`
- `docs/evm-integration/evmigration/portal-ui.md`
- `docs/evm-integration/unit-evmigration.md`
- `docs/evm-integration/integration-evmigration.md`
- `docs/evm-integration/user-guides/supernode-migration.md` — **critical**: the existing multisig section (around [line 304](docs/evm-integration/user-guides/supernode-migration.md#L304)) describes the pre-revision flow (single EVM-key recovery, `--new <single-eth-address>`, `sign-proof --from` only, broadcast with the new EVM key). It must be rewritten to describe: (a) generation of **N fresh eth_secp256k1 sub-keys** on the supernode host (one per co-signer), (b) deriving `new_address` from `kmultisig.LegacyAminoPubKey(K, ethSubKeys)`, (c) the new `sign-proof --from <legacy-sub-key> --new-key <eth-sub-key>` dual-side invocation, (d) `submit-proof` with **no `--from`** — migration txs are unsigned at the Cosmos layer (the new EVM account doesn't exist yet; see §4.5), (e) the updated cleanup flow that detects the on-chain multisig `BaseAccount.PubKey` (set by `MigrateAuth` per §4.6). The daemon's error-message template at the top of §Multisig in that doc also needs updating to reflect the new CLI shape.

## 9. Open Questions

None blocking. Defaults:

- `MaxMultisigSubKeys = 20` — governance-adjustable.
- Uniform-per-side `SigFormat` — per-sub-signer formats can be added later without breaking wire format.
- `SIG_FORMAT_EIP191` applies only to new-side single-key proofs; multisig EIP-191 is intentionally unsupported (no wallet implements it).
