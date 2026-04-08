# Architectural Decision Log - lumera

<!-- Append decisions in reverse chronological order -->
<!-- Format: YYYY-MM-DD: [Decision] - [Rationale] -->

2026-02-26: Challenge indices provided by client at registration, not derived from block hash at finalization — Client picks `challenge_indices` and stores them in `AvailabilityCommitment` (proto field 7). The keeper reads `commitment.ChallengeIndices` directly during finalization verification. This removes the timing dependency on finalization block state, simplifies the on-chain verification path, and eliminates the need for the SuperNode and Action Module to independently re-derive identical indices from block hash.

2026-02-26: BLAKE3 replaces SHA-256 for all LEP-5 hashing — Lumera already uses BLAKE3 throughout the codebase (`lukechampine.com/blake3`); adopting it for LEP-5 avoids introducing a second hash dependency and provides faster hashing. The `hash_algo` field in `AvailabilityCommitment` was changed from a plain string (`"SHA256"`) to a `HashAlgo` enum (`HASH_ALGO_BLAKE3 = 1`, `HASH_ALGO_SHA256 = 2`). All Merkle leaf/internal hashing and challenge seed derivation use BLAKE3.
