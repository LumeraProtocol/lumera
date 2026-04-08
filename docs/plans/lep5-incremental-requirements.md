# LEP-5 Incremental Requirements Plan

## Features

| ID | Name | Description | ATs | Status |
|----|------|-------------|-----|--------|
| F01 | Merkle Tree Library | Pure Go library: domain-separated SHA-256 Merkle tree with leaf hashing, internal node hashing, tree construction, proof generation, proof verification | AT01-AT04 | planned |
| F02 | Protobuf Schema Extensions | AvailabilityCommitment, ChunkProof messages; extend CascadeMetadata (fields 8,9) and Params (fields 12,13); buf codegen | AT05-AT06 | planned |
| F03 | Challenge Index Derivation | Deterministic unique challenge indices from seed = SHA-256(app_hash \|\| action_id \|\| height \|\| signer \|\| tag) | AT07-AT08 | planned |
| F04 | Registration Commitment Validation | Validate AvailabilityCommitment in CascadeActionHandler.Process for MsgRequestAction | AT09-AT11 | planned |
| F05 | Finalization Proof Verification | VerifyChunkProofs in keeper; verify proofs against stored root during FinalizeAction | AT12-AT16 | planned |
| F06 | Module Parameters and Governance | svc_challenge_count, svc_min_chunks_for_challenge params with defaults, validation, genesis | AT17-AT18 | planned |
| F07 | Chain Upgrade and Migration | Upgrade handler initializing new params; backward compat for existing actions | AT19-AT20 | planned |

## Acceptance Tests

| ID | Feature | Description |
|----|---------|-------------|
| AT01 | F01 | Merkle tree from 4 chunks produces expected root matching LEP-5 test vector (section 10.1) |
| AT02 | F01 | Proof for chunk index 2 in a 4-chunk tree verifies correctly (section 10.3) |
| AT03 | F01 | Proof with tampered leaf hash fails verification |
| AT04 | F01 | Single-chunk tree and large tree (1000+ chunks) produce valid proofs for any index |
| AT05 | F02 | Generated Go types AvailabilityCommitment and ChunkProof compile and round-trip through protobuf marshal/unmarshal |
| AT06 | F02 | Extended CascadeMetadata with commitment and proofs serializes/deserializes without breaking existing fields |
| AT07 | F03 | DeriveIndices with known inputs produces m unique indices in [0, num_chunks) |
| AT08 | F03 | DeriveIndices is deterministic: same inputs always produce same outputs; different signer produces different indices |
| AT09 | F04 | Registration with valid AvailabilityCommitment succeeds and commitment is stored on-chain |
| AT10 | F04 | Registration with invalid commitment_type or wrong num_chunks is rejected |
| AT11 | F04 | Registration without commitment still succeeds (soft-launch backward compat) |
| AT12 | F05 | FinalizeAction with valid chunk_proofs for all challenged indices succeeds, state -> DONE |
| AT13 | F05 | FinalizeAction with wrong chunk_index in a proof is rejected |
| AT14 | F05 | FinalizeAction with invalid Merkle path (bad sibling hash) is rejected |
| AT15 | F05 | FinalizeAction with wrong proof count (too few or too many) is rejected |
| AT16 | F05 | FinalizeAction skips SVC when num_chunks < svc_min_chunks_for_challenge |
| AT17 | F06 | Default params include svc_challenge_count=8 and svc_min_chunks_for_challenge=4 |
| AT18 | F06 | UpdateParams via governance changes SVC params; invalid values (0 challenge count) rejected |
| AT19 | F07 | Upgrade handler initializes new SVC params with defaults without affecting existing params |
| AT20 | F07 | Existing finalized actions without commitment are unaffected after upgrade |

## Slices

| ID | Name | Features | ATs | Goal |
|----|------|----------|-----|------|
| S01 | Merkle Tree Library + Tests | F01 | AT01-AT04 | Ship standalone merkle package at x/action/v1/merkle/ |
| S02 | Protobuf Schemas + Codegen | F02 | AT05-AT06 | Add proto messages, run buf generate, verify Go types |
| S03 | Challenge Index Derivation | F03 | AT07-AT08 | Ship challenge package at x/action/v1/challenge/ |
| S04 | Registration Commitment Validation | F04 | AT09-AT11 | Extend CascadeActionHandler for commitment validation |
| S05 | Finalization Proof Verification | F05 | AT12-AT16 | Implement and integrate VerifyChunkProofs |
| S06 | Module Params + Migration | F06, F07 | AT17-AT20 | SVC params, upgrade handler, genesis |
| S07 | Integration Tests | F04, F05 | AT09, AT12-AT16 | Full register->finalize->verify integration tests |

## Open Questions

- OQ01: Soft-launch via lep5_enabled_height param or boolean? (impacts F07)
- OQ02: Maximum file size bounds? (impacts F01 tree depth)
- OQ03: Should failed SVC emit evidence for audit module? (impacts F05)

## Risks

- R01: Protobuf field number conflicts with concurrent features (mitigate: coordinate field numbers)
- R02: app_hash availability during DeliverTx (mitigate: verify in AT12; fallback to prev block)
- R03: Backward compat for existing actions post-upgrade (mitigate: AT20 coverage; skip SVC when nil commitment)

## Files to Update (for code mode)

1. `docs/requirements.json` — append features F01-F07, ATs AT01-AT20, slices S01-S07
2. `docs/context.json` — add feature_status entries, set next_slice to S01
3. `docs/human-playbook.md` — add slice verification procedures
