# Plan: Sync Docs for BLAKE3 + Client-Provided Challenge Indices

## Context

Two protocol changes were made in code but **never synced to docs**:

1. **BLAKE3 instead of SHA-256** — All Merkle hashing and challenge derivation uses `lukechampine.com/blake3`. The proto `hash_algo` field is now a `HashAlgo` enum (`HASH_ALGO_BLAKE3 = 1`, `HASH_ALGO_SHA256 = 2`), not a plain string.
2. **Client-provided challenge indices** — The `AvailabilityCommitment` proto has a `challenge_indices` field (field 7). The client picks indices at registration and stores them on-chain. At finalization, `svc.go` reads `commitment.ChallengeIndices` directly — it does NOT call `challenge.DeriveIndices` from block hash.

### Code evidence

| File | Reality |
|------|---------|
| `x/action/v1/merkle/merkle.go` | `blake3.New(HashSize, nil)` for all leaf/internal hashing |
| `x/action/v1/challenge/challenge.go` | `blake3.Sum256(...)` and `blake3.New(32, nil)` |
| `x/action/v1/keeper/svc.go:70` | `expectedIndices := commitment.ChallengeIndices` — reads from stored commitment, no block-hash derivation |
| `x/action/v1/keeper/action_cascade.go:26` | `cascadeCommitmentHashAlgo = actiontypes.HashAlgo_HASH_ALGO_BLAKE3` |
| `x/action/v1/types/metadata.pb.go:165-177` | `HashAlgo` enum field + `ChallengeIndices []uint32` field 7 |

---

## File-by-File Change Plan

### 1. `docs/decisions.md`

Currently empty body. Add two decision entries in reverse chronological order:

```
2026-02-26: BLAKE3 replaces SHA-256 for all LEP-5 hashing — Lumera already uses BLAKE3 everywhere; it is faster and avoids introducing a second hash dependency. hash_algo field changed from string to HashAlgo enum.

2026-02-26: Challenge indices provided by client at registration, not derived from block hash at finalization — Client picks challenge_indices and stores them in AvailabilityCommitment field 7. Removes need for block-hash-based derivation in the keeper, simplifying the protocol and eliminating the timing dependency on finalization block state.
```

### 2. `docs/context.json`

**Feature notes to update:**
- **F01** notes: mention BLAKE3 instead of SHA-256 for domain-separated hashing
- **F03** notes: change from "previous-block hash" derivation to "client-provided challenge indices stored in AvailabilityCommitment at registration"
- **F04** notes: change hash_algo validation from `SHA256` string to `HashAlgo_HASH_ALGO_BLAKE3` enum
- **F05** notes: remove "previous-block hash challenge derivation" wording; replace with "reads challenge indices from stored AvailabilityCommitment"

**recent_decisions:** Add two new entries for BLAKE3 and client indices; update older entries that reference SHA-256/block-hash.

**New top-level field `build_instructions`:** Add the make commands as specified in the task.

### 3. `docs/requirements.json`

**scope.assumptions:**
- Replace "SHA-256 is available via crypto/sha256" with "BLAKE3 via lukechampine.com/blake3"
- Replace "Challenge entropy source must be deterministic during DeliverTx; use previous-block hash" with "Challenge indices are chosen by the client at registration and stored in AvailabilityCommitment.challenge_indices"

**scope.in_scope:**
- Change "Challenge index derivation from block state" to "Challenge index storage and validation from client-provided indices"

**constraints.must_use:**
- Replace "crypto/sha256 for all hashing" with "BLAKE3 via lukechampine.com/blake3 for all hashing"

**features:**
- **F01** description: SHA-256 → BLAKE3
- **F03** description: Replace derivation formula with client-provided indices explanation
- **F04** description: Replace `hash_algo must be 'SHA256'` with `hash_algo must be HASH_ALGO_BLAKE3 enum`; root size stays 32 bytes
- **F05** description: Replace "Recompute challenge seed from block state, derive expected indices" with "Read expected challenge indices from stored AvailabilityCommitment"

**domain_model.core_entities:**
- ChallengeIndices: update from "derived from block state" to "client-provided at registration, stored in AvailabilityCommitment"

**user_flows:**
- **UF01** step: add "Client generates challenge indices" step
- **UF02** steps: remove "derives challenge indices from previous-block hash" — replace with "reads expected challenge indices from stored commitment"

**nfr.security:**
- Replace "SHA-256 collision resistance: ~2^128 security" with "BLAKE3 collision resistance: ~2^128 security"

### 4. `docs/leps/LEP5 - Cascade Availability Commitment.md`

This is the largest file with ~60 references to SHA-256 and block-hash challenge derivation.

**Global replacements:**
- `SHA-256` → `BLAKE3` in all hashing contexts
- `sha256` → `blake3` in Go code snippets
- `crypto/sha256` → `lukechampine.com/blake3` in imports
- `sha256.New()` → `blake3.New(HashSize, nil)` in code
- `sha256.Sum256(...)` → `blake3.Sum256(...)` in code
- Hash function references in canonical encoding rules section 11

**Structural changes:**

- **Section 1.2 table:** "Challenge Unpredictability" — change from "Indices derived from block hash unknown at registration" to "Indices chosen by client, committed at registration; SuperNode must prove possession of those exact chunks"
- **Section 3.1 flow:** FINALIZATION step — change "Get unpredictable challenge indices from block hash" to "Read committed challenge indices from on-chain AvailabilityCommitment"
- **Section 4.2.1 proto:** Change `string hash_algo = 2` to `HashAlgo hash_algo = 2` and add `repeated uint32 challenge_indices = 7` field; add HashAlgo enum definition
- **Section 4.3 Challenge Index Derivation:** Rewrite to explain client-provided model. DeriveIndices still exists as a helper but is used client-side, not block-hash-based.
- **Section 4.4.1 Registration Phase:** Add step for client generating challenge indices
- **Section 4.4.2 Finalization Phase:** Remove "Get challenge seed from current block state" — SuperNode reads indices from on-chain commitment
- **Section 4.4.3 Verification Phase:** Replace "Recompute challenge seed" / "Derive expected challenge indices" with "Read challenge_indices from stored commitment"
- **Section 4.5 Sequence Diagram:** Update finalization column to read stored indices rather than "Derive challenges from block hash"
- **Section 5.2 Pre-computation attack:** Update defense — indices are client-provided, not block-derived. The defense shifts to: client commits indices before knowing which SuperNode finalizes; SuperNode cannot predict which indices it will need to prove until it sees the on-chain commitment
- **Section 5.4 Challenge Grinding:** Update — grinding is now about client choosing favorable indices. Defense: indices must cover a representative sample; governance params enforce minimum challenge count
- **Section 5.6 Security Summary table:** Update hash algo and challenge mechanism
- **Section 6.1 Go code:** blake3 imports and calls
- **Section 6.2 Go code:** blake3 imports and calls — note this function may still be used client-side
- **Section 6.3 Keeper verification code:** Show reading commitment.ChallengeIndices instead of calling DeriveIndices
- **Section 10.x Test vectors:** Replace SHA-256 with BLAKE3 in all formulas
- **Section 11 Canonical Encoding Rules:** `SHA-256` → `BLAKE3`

**JS SDK snippet (Section 4.4.1):**
- Change `crypto.subtle.digest("SHA-256", ...)` to note about BLAKE3 in browser (e.g., using `@aspect-build/aspect-blake3` or `blake3` npm package)
- Add challenge index generation step

**Changelog:** Add version 0.4 entry for BLAKE3 and client-provided indices changes

---

## Execution Order

1. `docs/decisions.md` — smallest, independent
2. `docs/context.json` — feature notes + build instructions
3. `docs/requirements.json` — scope/features/flows
4. `docs/leps/LEP5 - Cascade Availability Commitment.md` — largest, most references
5. Signal completion
