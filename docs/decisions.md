# Architectural Decision Log - lumera

<!-- Append decisions in reverse chronological order -->
<!-- Format: YYYY-MM-DD: [Decision] - [Rationale] -->

2026-05-05: Reconcile Everlight pool-account requirements to code-canonical embedded design — The implemented Phase 1 code is the source of truth: the Everlight pool is the existing `supernode` module account (`sntypes.ModuleName`) and there is no separate permissionless `everlight` module account. Requirements and status docs were updated to remove the older account-permission expectation from Phase 1 scope. A dedicated permissionless pool account remains future hardening only if explicitly scoped.

2026-05-05: Remove F17/AT40 block reward share routing completely — Everlight funding for this project is Foundation transfers, registration fee share, and Community Pool governance transfers. The former idea to split block rewards through `x/distribution AllocateTokens` is not deferred; it is removed from this project and should not appear as a future candidate unless a new requirements process explicitly reintroduces it.

2026-04-28: Preserve `SUPERNODE_STATE_*` proto enum names despite buf `ENUM_VALUE_PREFIX` findings — These names are part of the public supernode API surface consumed by downstream clients and devnet tooling. Renaming them to `SUPER_NODE_STATE_*` for lint compliance would create unnecessary API compatibility risk, so the S10-S15 Everlight gate explicitly waives these enum-prefix findings.

2026-04-28: Everlight Phase 1 uses the existing `supernode` module account as the pool — The implemented and tested Phase 1 design consolidates Everlight into `x/supernode`; there is no standalone `x/everlight` module and no dedicated permissionless `everlight` account. Devnet evaluation should verify the embedded `supernode` module account exists and accepts funding, not assert that it has no existing supernode permissions. A dedicated permissionless pool account remains a separate future hardening feature if desired.

2026-02-26: Challenge indices provided by client at registration, not derived from block hash at finalization — Client picks `challenge_indices` and stores them in `AvailabilityCommitment` (proto field 7). The keeper reads `commitment.ChallengeIndices` directly during finalization verification. This removes the timing dependency on finalization block state, simplifies the on-chain verification path, and eliminates the need for the SuperNode and Action Module to independently re-derive identical indices from block hash.

2026-02-26: BLAKE3 replaces SHA-256 for all LEP-5 hashing — Lumera already uses BLAKE3 throughout the codebase (`lukechampine.com/blake3`); adopting it for LEP-5 avoids introducing a second hash dependency and provides faster hashing. The `hash_algo` field in `AvailabilityCommitment` was changed from a plain string (`"SHA256"`) to a `HashAlgo` enum (`HASH_ALGO_BLAKE3 = 1`, `HASH_ALGO_SHA256 = 2`). All Merkle leaf/internal hashing and challenge seed derivation use BLAKE3.
