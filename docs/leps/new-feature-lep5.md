# LEP-5 — Cascade Availability Commitment (Merkle Proof Challenge)

## Status: Draft
## Author: Lumera Protocol Team
## Created: 2025-02-08
## Requires: LEP1 (Minimizing Cascade Metadata Size)

---

## 1. Executive Summary

### 1.1 The Problem

The current Cascade protocol has a critical vulnerability: **a malicious SuperNode can finalize actions and claim fees without ever receiving or storing the actual file data.**

This attack is possible because:
- All information needed to compute valid `rq_ids_ids` is available on-chain (creator's signature + counter)
- The verification formula uses only on-chain metadata
- No proof of actual file possession is required at finalization

**Impact:**
- Users pay fees but data is never stored
- Network storage guarantees are undermined
- Malicious operators can extract value without providing service

### 1.2 The Solution

Introduce an **Availability Commitment** system that requires the finalizing SuperNode to prove possession of actual file data:

1. **Merkle Root Commitment:** At registration, the client computes a Merkle root over fixed-size chunks of the uploaded file and includes it on-chain.

2. **Challenge-Response Proofs:** At registration, the client also commits a set of challenge indices. At finalization, the SuperNode must produce valid Merkle proofs for those challenged chunks — which is only possible if the SuperNode actually has the file.

**Key Properties:**

| Property | Guarantee |
|----------|-----------|
| Commitment Binding | Client commits to all file chunks at registration time |
| Challenge Commitment | Client commits challenge indices upfront; SuperNode must prove possession of those exact chunks |
| Proof Compactness | O(log N) proof size per challenged chunk |
| Verification Efficiency | O(m × log N) on-chain verification |

### 1.3 Why Chunk-Based (Not Symbol-Based)

The Merkle tree is built over **fixed-size chunks of the original file**, not RaptorQ symbols:

- **Client practicality:** The web client (JS SDK + rq-lite-wasm) currently calls the RQ API which returns layout metadata and symbol *indices* — not the raw symbol bytes themselves. Exposing potentially thousands of symbol buffers to the browser would require significant WASM API changes and create memory pressure.
- **No WASM changes required:** Chunk-based commitment uses only BLAKE3 hashing over file bytes — fast and available via npm packages (e.g., `blake3`).
- **Determinism without rq_oti:** RaptorQ deterministic re-encoding requires exact OTI parameters and bit-identical library versions across client and SuperNode. Chunk hashing of the original file avoids this fragile coupling entirely.
- **Upgrade path preserved:** A future `SYMBOL_MERKLE_V1` mode can be added when/if rq-library exposes symbol-level iteration in its WASM build, providing even stronger binding to actual Cascade work.

---

## 2. Background

### 2.1 Current Protocol Flow (Cascade)

```
  1. Client registers action with:
     - data_hash (hash of entire file)
     - rq_ids_signature = Base64(index_file).creators_signature

  2. SuperNode finalizes with:
     - rq_ids_ids = [ID_1, ID_2, ..., ID_50]
     where ID_i = Base58(BLAKE3(zstd(rq_ids_signature.counter)))

  3. Action Module verifies:
     - Random ID matches formula              ✓
     - SuperNode is in top 10                  ✓
     - NO PROOF OF FILE POSSESSION             ✗
```

### 2.2 The Attack Vector

```
  MALICIOUS SUPERNODE STRATEGY:

  1. Monitor blockchain for new Cascade actions
  2. Extract from on-chain data:
     - action_id, rq_ids_signature, rq_ids_ic, rq_ids_max
  3. Compute valid IDs WITHOUT the file:
     for counter in range(rq_ids_ic, rq_ids_max):
         ID = Base58(BLAKE3(zstd(rq_ids_signature.counter)))
  4. Submit FinalizeAction with computed IDs
  5. Collect fees without storing anything

  RESULT: User pays, but data is NEVER stored.
```

### 2.3 Why Current Verification Fails

| What's Verified | What's NOT Verified |
|-----------------|---------------------|
| ID formula correctness | Actual file receipt |
| SuperNode authorization | Symbol computation |
| Signature validity | Data storage in Kademlia |
| Expiration time | Proof of data possession |

---

## 3. Design Overview

### 3.1 High-Level Flow

```
  REGISTRATION (Client):
    File bytes → Split into fixed chunks → Hash each chunk (BLAKE3) →
    Build Merkle tree → Generate challenge indices →
    Commit root + challenge indices on-chain

  FINALIZATION (SuperNode):
    Receive file → Verify data_hash → Recompute Merkle tree (BLAKE3) →
    Verify root matches on-chain commitment →
    Read challenge indices from on-chain commitment →
    Produce Merkle proofs for challenged chunks →
    Submit MsgFinalizeAction with proofs

  VERIFICATION (Action Module):
    Read expected challenge indices from stored commitment →
    Verify each Merkle proof against stored root →
    If all valid → state = DONE, release fees
```

---

## 4. Technical Specification

### 4.1 Merkle Tree Construction

#### 4.1.1 Chunking

Let file bytes be `B` of length `N`. Let chunk size be `S`.

**Hard boundaries enforced by the chain:**

| Rule | Value | Enforcement |
|------|-------|-------------|
| Minimum file size | 4 bytes | `total_size >= 4` — reject trivially tiny files |
| Maximum chunk size | 256 KiB (262,144) | `chunk_size <= 262144` |
| Minimum chunk size | 1 byte | `chunk_size >= 1` |
| Minimum chunk count | 4 | `num_chunks >= svc_min_chunks_for_challenge` (default 4) — **unconditional** |
| Minimum challenge indices | 4 | `num_indices = min(svc_challenge_count, num_chunks)` — always ≥ 4 since num_chunks ≥ 4 |
| Maximum challenge indices | 8 | `num_indices = min(svc_challenge_count, num_chunks)` — capped by `svc_challenge_count` (default 8) |

**Chunk size rules:**
- `S` must be a power of 2
- `S` must be in `[1, 262144]` (1 byte floor, 256 KiB ceiling)
- Default: `S = 262144` (256 KiB) for files ≥ 1 MiB
- For smaller files, the client MUST reduce `S` so that `ceil(N / S) >= svc_min_chunks_for_challenge` (default 4)
- The minimum chunk count is enforced **unconditionally** — all files with an AvailabilityCommitment must produce ≥ 4 chunks

**Client chunk size selection algorithm:**
```
S = 262144
while ceil(N / S) < svc_min_chunks_for_challenge AND S > 1:
    S = S / 2
```

- `num_chunks = ceil(N / S)`
- `chunk_i = B[i*S : min((i+1)*S, N)]`

The last chunk may be smaller than `S`.

**Examples:**

| File Size | Chunk Size | Chunks | Indices | Accepted? |
|-----------|-----------|--------|---------|-----------|
| 2 MiB     | 256 KiB   | 8      | 8       | ✓ (8 chunks, 8 indices) |
| 1 MiB     | 256 KiB   | 4      | 4       | ✓ (4 chunks, 4 indices) |
| 500 KiB   | 128 KiB   | 4      | 4       | ✓ (4 chunks, 4 indices) |
| 100 KiB   | 32 KiB    | 4      | 4       | ✓ (4 chunks, 4 indices) |
| 4 KiB     | 1 KiB     | 4      | 4       | ✓ (4 chunks, 4 indices) |
| 4 bytes   | 1 byte    | 4      | 4       | ✓ (4 chunks, 4 indices) |
| 3 bytes   | —         | —      | —       | ✗ (below min file size of 4 bytes) |
| 500 KiB   | 256 KiB   | 2      | —       | ✗ (only 2 chunks < min 4) |

#### 4.1.2 Leaf Hashing (Domain Separated)

To prevent second-preimage attacks, leaves and internal nodes use different domain prefixes:

```
leaf_i = BLAKE3(0x00 || uint32be(i) || chunk_i)
```

Where `uint32be(i)` is the chunk index as 4 bytes big-endian.

#### 4.1.3 Internal Node Hashing (Domain Separated)

```
parent = BLAKE3(0x01 || left_hash || right_hash)
```

If a level has an odd number of nodes, duplicate the last node (`right = left`).

#### 4.1.4 Tree Structure

```
                         ┌─────────────────┐
                         │   MERKLE ROOT    │
                         │   (32 bytes)     │
                         └────────┬─────────┘
                                  │
                    ┌─────────────┴─────────────┐
                    │                           │
              ┌─────┴─────┐               ┌─────┴─────┐
              │ H(01||L||R)│              │ H(01||L||R)│
              └─────┬─────┘               └─────┬─────┘
                    │                           │
              ...  ...                    ...  ...
                    │                           │
         ┌─────────┴─────────┐       ┌─────────┴─────────┐
         │                   │       │                   │
    H(00||0||C0)       H(00||1||C1)  H(00||2||C2)   H(00||3||C3)
         │                   │       │                   │
      Chunk 0            Chunk 1   Chunk 2           Chunk 3
     (256 KiB)          (256 KiB) (256 KiB)         (≤256 KiB)
```

#### 4.1.5 Merkle Proof Structure

To prove chunk `i` belongs to the tree, provide sibling hashes along the path from leaf to root:

```
  PROOF FOR CHUNK i:
  {
    chunk_index: i,
    leaf_hash: BLAKE3(0x00 || uint32be(i) || chunk_i),
    path_hashes: [sibling_0, sibling_1, ..., sibling_d],
    path_directions: [bit_0, bit_1, ..., bit_d]   // true = sibling on right
  }

  VERIFICATION:
  current = leaf_hash
  for j in 0..d:
      if path_directions[j]:   // sibling on right
          current = BLAKE3(0x01 || current || path_hashes[j])
      else:                    // sibling on left
          current = BLAKE3(0x01 || path_hashes[j] || current)

  ACCEPT if: current == stored_merkle_root
```

### 4.2 Protocol Changes

#### 4.2.1 On-Chain Commitment: `AvailabilityCommitment`

Added to CascadeMetadata at registration:

```protobuf
enum HashAlgo {
  HASH_ALGO_UNSPECIFIED = 0;
  HASH_ALGO_BLAKE3 = 1;
  HASH_ALGO_SHA256 = 2;       // Reserved for future use
}

message AvailabilityCommitment {
  string commitment_type = 1;         // "lep5/chunk-merkle/v1"
  HashAlgo hash_algo = 2;             // HASH_ALGO_BLAKE3
  uint32 chunk_size = 3;              // e.g. 262144 for 256 KiB
  uint64 total_size = 4;              // Original file size in bytes
  uint32 num_chunks = 5;              // ceil(total_size / chunk_size)
  bytes root = 6;                     // 32 bytes - Merkle root
  repeated uint32 challenge_indices = 7;  // Client-chosen challenge chunk indices
}
```

#### 4.2.2 Extended CascadeMetadata

```protobuf
message CascadeMetadata {
  // ═══════════════════════════════════════════════════
  // EXISTING FIELDS (unchanged)
  // ═══════════════════════════════════════════════════
  string data_hash = 1;
  string file_name = 2;
  uint64 rq_ids_ic = 3;
  uint64 rq_ids_max = 4;
  repeated string rq_ids_ids = 5;
  string signatures = 6;
  bool public = 7;

  // ═══════════════════════════════════════════════════
  // NEW FIELDS — LEP-5
  // ═══════════════════════════════════════════════════

  // Set at Registration (by client) — includes root and challenge_indices
  AvailabilityCommitment availability_commitment = 8;

  // Set at Finalization (by SuperNode)
  repeated ChunkProof chunk_proofs = 9;
}

message ChunkProof {
  uint32 chunk_index = 1;             // Which chunk this proves
  bytes leaf_hash = 2;                // BLAKE3(0x00 || uint32be(idx) || chunk_bytes)
  repeated bytes path_hashes = 3;     // Sibling hashes (ceil(log2(N)) × 32 bytes)
  repeated bool path_directions = 4;  // true = sibling on right
}
```

#### 4.2.3 Module Parameters

```protobuf
message Params {
  // ... existing params ...

  // NEW: LEP-5 parameters
  uint32 svc_challenge_count = 12;             // m — number of chunks to challenge (default: 8)
  uint32 svc_min_chunks_for_challenge = 13;    // Minimum chunks to require SVC (default: 4)
}
```

**Chunk size protocol constants** (not governance-tunable):

| Constant | Value | Purpose |
|----------|-------|---------|
| `cascadeCommitmentMaxChunkSize` | 262,144 (256 KiB) | Maximum allowed chunk size |
| `cascadeCommitmentMinChunkSize` | 1 (1 byte) | Minimum allowed chunk size |
| `cascadeCommitmentMinTotalSize` | 4 | Minimum file size in bytes (reject trivially tiny files) |

The client's `chunk_size` must be a power of 2 within this range. The chain **unconditionally rejects** registrations where `num_chunks < svc_min_chunks_for_challenge` (default 4). The number of challenge indices is always `min(svc_challenge_count, num_chunks)`, yielding a value in [4, 8] since `num_chunks >= 4`.

### 4.3 Challenge Index Generation

Challenge indices are **generated by the client at registration time** and stored in the `AvailabilityCommitment.challenge_indices` field. The indices must be:
- **Deterministic:** Generated via BLAKE3-based derivation from known inputs
- **Unique:** No duplicate indices in a single challenge set
- **Within range:** All indices in `[0, num_chunks)`

The `DeriveIndices` helper function (available in `x/action/v1/challenge/`) can be used by clients:

```
seed = BLAKE3(
    entropy_input ||       // e.g. random nonce or hash of file content
    action_id ||
    uint64be(num_chunks) ||
    signer_addr ||
    "lep5/challenge/v1"
)

for j in 0..m-1:
    raw = BLAKE3(seed || uint32be(j))
    idx_j = uint64(raw[0:8]) mod num_chunks
    // If duplicate, increment j and retry
```

The keeper does **not** re-derive indices. At finalization, it reads `commitment.ChallengeIndices` directly from the stored on-chain commitment and validates that submitted proofs match those indices.

### 4.4 Updated Protocol Flows

#### 4.4.1 Registration Phase (Client)

```
  CLIENT REGISTRATION:

  1. Read file, compute data_hash (existing)
  2. Split file into chunks of size S (default 256 KiB)
  3. Compute leaf hashes:
     leaf_i = BLAKE3(0x00 || uint32be(i) || chunk_i)
  4. Build Merkle tree, obtain root (32 bytes)
  5. Generate challenge indices:
     Use DeriveIndices to pick m unique chunk indices
  6. Create AvailabilityCommitment:
     {
       commitment_type: "lep5/chunk-merkle/v1",
       hash_algo: HASH_ALGO_BLAKE3,
       chunk_size: 262144,
       total_size: <file size>,
       num_chunks: ceil(file_size / chunk_size),
       root: <merkle_root>,
       challenge_indices: [idx_0, idx_1, ..., idx_m-1]
     }
  7. Include commitment in MsgRequestAction metadata
  8. (Existing) Generate rq_ids, sign, submit RegisterAction
```

**JS SDK addition:**

```javascript
import { blake3 } from 'blake3';  // or equivalent BLAKE3 npm package

async function computeCommitment(fileBlob, chunkSize = 262144) {
  const totalSize = fileBlob.size;
  const numChunks = Math.ceil(totalSize / chunkSize);
  const leafHashes = [];

  for (let i = 0; i < numChunks; i++) {
    const start = i * chunkSize;
    const end = Math.min(start + chunkSize, totalSize);
    const chunkBytes = new Uint8Array(await fileBlob.slice(start, end).arrayBuffer());

    // Domain-separated leaf: 0x00 || uint32be(i) || chunk
    const prefix = new Uint8Array(5);
    prefix[0] = 0x00;
    new DataView(prefix.buffer).setUint32(1, i, false); // big-endian

    const leafInput = new Uint8Array(prefix.length + chunkBytes.length);
    leafInput.set(prefix);
    leafInput.set(chunkBytes, prefix.length);

    const hash = blake3(leafInput);  // BLAKE3 hash → 32 bytes
    leafHashes.push(new Uint8Array(hash));
  }

  const root = await buildMerkleRoot(leafHashes); // standard bottom-up construction

  // Generate challenge indices using BLAKE3-based derivation
  const challengeIndices = deriveIndices(root, numChunks, challengeCount);

  return { root, totalSize, numChunks, chunkSize, challengeIndices };
}
```

#### 4.4.2 Finalization Phase (SuperNode)

```
  SUPERNODE FINALIZATION:

  1. Receive file from client via gRPC stream
  2. Verify: BLAKE3(received_file) == action.data_hash  (existing)
  3. Split file into chunks using commitment.chunk_size
  4. Compute leaf hashes with domain separation (BLAKE3)
  5. Build Merkle tree
  6. VERIFY: computed_root == action.availability_commitment.root
     (If mismatch → client provided bad commitment, report evidence)
  7. Perform existing RQ encoding, ID generation, Kademlia storage
  8. Read challenge indices from on-chain commitment:
     indices = action.availability_commitment.challenge_indices
  9. Generate Merkle proofs for each challenged chunk
  10. Submit MsgFinalizeAction with:
      - Existing fields (rq_ids_ids, rq_ids_oti)
      - NEW: chunk_proofs array
```

#### 4.4.3 Verification Phase (Action Module)

```
  ACTION MODULE — On receiving MsgFinalizeAction:

  1. Existing checks (expiration, SN authorization, rq_ids verification)

  2. Skip SVC if num_chunks < svc_min_chunks_for_challenge

  3. Read expected challenge indices from stored commitment:
     expectedIndices = commitment.ChallengeIndices

  4. Validate proof count matches expected challenge count

  5. For each chunk_proof:
     a. Verify chunk_proof.chunk_index == expectedIndices[i]
     b. Verify Merkle proof:
        - Walk path_hashes with path_directions
        - Use domain-separated hashing (0x01 prefix for internal nodes, BLAKE3)
        - Computed root must equal stored commitment.root

  6. If all proofs valid → state = DONE, release fees
```

### 4.5 Complete Sequence Diagram

```
┌──────┐          ┌────────┐          ┌──────────┐          ┌────────┐
│Client│          │ Action │          │SuperNode │          │Kademlia│
└──┬───┘          │ Module │          └────┬─────┘          └───┬────┘
   │              └───┬────┘               │                    │
   │                  │                    │                    │
   │ REGISTRATION     │                    │                    │
   │══════════════════│                    │                    │
   │ Chunk file       │                    │                    │
   │ Build Merkle tree│                    │                    │
   │ (BLAKE3)         │                    │                    │
   │ Compute root     │                    │                    │
   │ Pick challenge   │                    │                    │
   │ indices          │                    │                    │
   │                  │                    │                    │
   │ RegisterAction   │                    │                    │
   │ (+commitment     │                    │                    │
   │  +indices)       │                    │                    │
   │─────────────────>│                    │                    │
   │                  │ Store action       │                    │
   │                  │ + Merkle root      │                    │
   │                  │ + challenge indices │                    │
   │                  │                    │                    │
   │ PROCESSING       │                    │                    │
   │══════════════════│                    │                    │
   │ Upload file ────────────────────────> │                    │
   │                  │                    │ Verify hash        │
   │                  │                    │ Chunk + tree       │
   │                  │                    │ (BLAKE3)           │
   │                  │                    │ Verify root match  │
   │                  │                    │ RQ encode          │
   │                  │                    │ Store symbols ────>│
   │                  │                    │                    │
   │ FINALIZATION     │                    │                    │
   │══════════════════│                    │                    │
   │                  │                    │ Read challenge     │
   │                  │                    │ indices from       │
   │                  │                    │ stored commitment  │
   │                  │                    │ Generate Merkle    │
   │                  │                    │ proofs (BLAKE3)    │
   │                  │                    │                    │
   │                  │  FinalizeAction    │                    │
   │                  │  (+chunk_proofs)   │                    │
   │                  │<───────────────────│                    │
   │                  │                    │                    │
   │                  │ Read stored        │                    │
   │                  │ challenge indices  │                    │
   │                  │ Verify each proof  │                    │
   │                  │ against stored root│                    │
   │                  │ (BLAKE3)           │                    │
   │                  │                    │                    │
   │                  │ All valid?         │                    │
   │                  │ → DONE             │                    │
   │                  │ → Release fees     │                    │
```

---

## 5. Security Analysis

### 5.1 Attack: Finalize Without Data

**Defense:** The finalizer must produce Merkle proofs for chunk indices that were committed at registration. Without the actual file bytes, the finalizer cannot compute the correct leaf hashes — the domain-separated BLAKE3 hash includes the raw chunk content.

### 5.2 Attack: Pre-computation / Index Prediction

**Defense:** Challenge indices are committed by the client at registration time. A malicious SuperNode cannot know which chunks it will be challenged on until it reads the on-chain commitment. Once the commitment is on-chain, the indices are immutable and the SuperNode must provide valid proofs for those exact chunks.

### 5.3 Attack: Selective Storage (Store Only Some Chunks)

With m challenges drawn uniformly from N chunks, and an attacker storing fraction p of chunks:

```
P(evade detection) = p^m
```

| Chunks Stored | p   | P(evade) m=5 | P(evade) m=8 | P(evade) m=16 |
|---------------|-----|-------------|-------------|--------------|
| 90%           | 0.9 | 59.0%       | 43.0%       | 18.5%        |
| 80%           | 0.8 | 32.8%       | 16.8%       | 2.8%         |
| 50%           | 0.5 | 3.1%        | 0.39%       | 0.0015%      |
| 20%           | 0.2 | 0.032%      | negligible  | negligible   |

With m=8 (recommended default), storing less than half the file has <0.4% chance of evading a single challenge round. Failed attempts accumulate Audit module evidence.

### 5.4 Attack: Client–SuperNode Collusion on Challenge Indices

**Risk:** A malicious client could collude with a SuperNode to choose "easy" challenge indices (e.g., indices for chunks the SuperNode already has from a partial transfer).

**Defense:** The governance-enforced `svc_challenge_count` (m) ensures a minimum number of challenged chunks. The indices must be unique and within `[0, num_chunks)`. Since the client's goal is to have its data stored, collusion against its own interest is economically irrational. A future enhancement could mix server-side randomness into the index generation to further harden against this vector.

### 5.5 Attack: Merkle Root Forgery (Collision)

**Defense:** BLAKE3 collision resistance provides ~2^128 security (birthday attack). Domain-separated hashing (0x00 for leaves, 0x01 for internal nodes) additionally prevents second-preimage attacks where a leaf could be confused with an internal node.

### 5.6 Security Summary

| Guarantee | Mechanism | Strength |
|-----------|-----------|----------|
| **Commitment Binding** | BLAKE3 collision resistance | 2^128 security |
| **Challenge Commitment** | Indices stored on-chain at registration | Immutable once committed |
| **Proof Soundness** | Merkle tree structure | Information-theoretic |
| **Collusion Resistance** | Client incentive alignment + governance minimum m | Economic (client pays for storage) |

---

## 6. Implementation Details

### 6.1 Merkle Tree (Go)

```go
package merkle

import (
    "encoding/binary"
    "errors"

    "lukechampine.com/blake3"
)

const HashSize = 32

var (
    LeafPrefix     = []byte{0x00}
    InternalPrefix = []byte{0x01}
    ErrEmptyInput      = errors.New("empty input")
    ErrIndexOutOfRange = errors.New("index out of range")
)

// HashLeaf computes BLAKE3(0x00 || uint32be(index) || data)
func HashLeaf(index uint32, data []byte) [HashSize]byte {
    var prefix [5]byte
    prefix[0] = 0x00
    binary.BigEndian.PutUint32(prefix[1:], index)

    h := blake3.New(HashSize, nil)
    h.Write(prefix[:])
    h.Write(data)
    var result [HashSize]byte
    copy(result[:], h.Sum(nil))
    return result
}

// HashInternal computes BLAKE3(0x01 || left || right)
func HashInternal(left, right [HashSize]byte) [HashSize]byte {
    h := blake3.New(HashSize, nil)
    h.Write(InternalPrefix)
    h.Write(left[:])
    h.Write(right[:])
    var result [HashSize]byte
    copy(result[:], h.Sum(nil))
    return result
}

type Tree struct {
    Root      [HashSize]byte
    Leaves    [][HashSize]byte
    Levels    [][][HashSize]byte // levels[0] = leaves (possibly padded), levels[last] = [root]
    LeafCount int
}

// BuildTree constructs a Merkle tree from chunk data.
// If a level has an odd number of nodes, the last node is duplicated.
func BuildTree(chunks [][]byte) (*Tree, error) {
    n := len(chunks)
    if n == 0 {
        return nil, ErrEmptyInput
    }

    // Compute leaf hashes
    leaves := make([][HashSize]byte, n)
    for i, chunk := range chunks {
        leaves[i] = HashLeaf(uint32(i), chunk)
    }

    levels := [][][HashSize]byte{leaves}

    // Build tree bottom-up
    current := leaves
    for len(current) > 1 {
        // If odd number of nodes, duplicate the last node
        if len(current)%2 != 0 {
            current = append(current, current[len(current)-1])
            levels[len(levels)-1] = current
        }

        next := make([][HashSize]byte, len(current)/2)
        for i := 0; i < len(current); i += 2 {
            next[i/2] = HashInternal(current[i], current[i+1])
        }
        levels = append(levels, next)
        current = next
    }

    return &Tree{
        Root:      current[0],
        Leaves:    leaves,
        Levels:    levels,
        LeafCount: n,
    }, nil
}

type Proof struct {
    ChunkIndex     uint32
    LeafHash       [HashSize]byte
    PathHashes     [][HashSize]byte
    PathDirections []bool // true = sibling on right
}

// GenerateProof creates a Merkle proof for chunk at index
func (t *Tree) GenerateProof(index int) (*Proof, error) {
    if index < 0 || index >= t.LeafCount {
        return nil, ErrIndexOutOfRange
    }

    proof := &Proof{
        ChunkIndex: uint32(index),
        LeafHash:   t.Leaves[index],
    }

    idx := index
    for level := 0; level < len(t.Levels)-1; level++ {
        if idx%2 == 0 {
            proof.PathHashes = append(proof.PathHashes, t.Levels[level][idx+1])
            proof.PathDirections = append(proof.PathDirections, true)
        } else {
            proof.PathHashes = append(proof.PathHashes, t.Levels[level][idx-1])
            proof.PathDirections = append(proof.PathDirections, false)
        }
        idx /= 2
    }

    return proof, nil
}

// Verify checks proof against a root
func (p *Proof) Verify(root [HashSize]byte) bool {
    current := p.LeafHash
    for i, sibling := range p.PathHashes {
        if p.PathDirections[i] {
            current = HashInternal(current, sibling)
        } else {
            current = HashInternal(sibling, current)
        }
    }
    return current == root
}
```

### 6.2 Challenge Index Generation (Go)

The `DeriveIndices` helper generates deterministic unique indices using BLAKE3. This function is available for client-side use. The keeper does **not** call this at finalization — it reads stored indices from `commitment.ChallengeIndices`.

```go
package challenge

import (
    "encoding/binary"

    "lukechampine.com/blake3"
)

const domainTag = "lep5/challenge/v1"

// DeriveIndices generates m deterministic pseudo-random unique indices.
// Used by clients to generate challenge indices at registration time.
func DeriveIndices(actionID string, entropyInput []byte, height uint64,
    signerAddr []byte, numChunks uint32, m uint32) []uint32 {

    if numChunks == 0 || m == 0 {
        return []uint32{}
    }

    if m > numChunks {
        m = numChunks
    }

    // Compute seed
    var heightBytes [8]byte
    binary.BigEndian.PutUint64(heightBytes[:], height)

    seedInput := make([]byte, 0, len(entropyInput)+len(actionID)+8+len(signerAddr)+len(domainTag))
    seedInput = append(seedInput, entropyInput...)
    seedInput = append(seedInput, actionID...)
    seedInput = append(seedInput, heightBytes[:]...)
    seedInput = append(seedInput, signerAddr...)
    seedInput = append(seedInput, domainTag...)

    seed := blake3.Sum256(seedInput)

    indices := make([]uint32, 0, m)
    used := make(map[uint32]struct{}, m)
    counter := uint32(0)

    for uint32(len(indices)) < m {
        var counterBytes [4]byte
        binary.BigEndian.PutUint32(counterBytes[:], counter)

        h := blake3.New(32, nil)
        h.Write(seed[:])
        h.Write(counterBytes[:])
        raw := h.Sum(nil)

        idx := uint32(binary.BigEndian.Uint64(raw[:8]) % uint64(numChunks))
        if _, exists := used[idx]; !exists {
            used[idx] = struct{}{}
            indices = append(indices, idx)
        }
        counter++
    }

    return indices
}
```

### 6.3 Keeper Verification (Go)

The keeper reads challenge indices from the stored commitment — it does **not** re-derive them from block state.

```go
// x/action/v1/keeper/svc.go
package keeper

import (
    sdk "github.com/cosmos/cosmos-sdk/types"
    "github.com/LumeraProtocol/lumera/x/action/v1/types"
    "github.com/LumeraProtocol/lumera/x/action/v1/merkle"
)

// VerifyChunkProofs validates the SVC proofs in a FinalizeAction message.
// Challenge indices are read from the stored AvailabilityCommitment,
// not derived from block state.
func (k Keeper) VerifyChunkProofs(
    ctx sdk.Context,
    action *types.Action,
    superNodeAccount string,
    proofs []*types.ChunkProof,
) error {
    metadata := action.GetCascadeMetadata()
    commitment := metadata.AvailabilityCommitment

    if commitment == nil {
        // Backward compatibility: pre-LEP-5 actions do not include commitments.
        return nil
    }

    params := k.GetParams(ctx)
    challengeCount, minChunks := getSVCParamsOrDefault(params)

    // Skip SVC for very small files
    if commitment.NumChunks < minChunks {
        return nil
    }

    expectedCount := min(challengeCount, commitment.NumChunks)
    if uint32(len(proofs)) != expectedCount {
        return types.ErrWrongProofCount.Wrapf(
            "expected %d proofs, got %d", expectedCount, len(proofs))
    }

    // Read expected challenge indices from the stored commitment
    expectedIndices := commitment.ChallengeIndices
    if uint32(len(expectedIndices)) != expectedCount {
        return types.ErrInvalidMetadata.Wrapf(
            "commitment has %d challenge_indices, expected %d",
            len(expectedIndices), expectedCount)
    }

    // Verify each proof
    for i, proof := range proofs {
        // Check index matches expected challenge
        if proof.ChunkIndex != expectedIndices[i] {
            return types.ErrWrongChallengeIndex.Wrapf(
                "proof %d: expected index %d, got %d",
                i, expectedIndices[i], proof.ChunkIndex)
        }

        // Verify BLAKE3 Merkle proof against stored root
        merkleProof := &merkle.Proof{
            ChunkIndex:     proof.ChunkIndex,
            LeafHash:       proof.LeafHash,
            PathHashes:     proof.PathHashes,
            PathDirections: proof.PathDirections,
        }

        if !merkleProof.Verify(commitment.Root) {
            return types.ErrInvalidMerkleProof.Wrapf(
                "proof for chunk %d failed verification", proof.ChunkIndex)
        }
    }

    return nil
}
```

### 6.4 Gas and Size Impact

**Registration message increase:**

| Field | Size |
|-------|------|
| commitment_type | ~24 bytes |
| hash_algo | 1 byte (enum varint) |
| chunk_size | 4 bytes |
| total_size | 8 bytes |
| num_chunks | 4 bytes |
| root | 32 bytes |
| challenge_indices (m=8) | ~32 bytes |
| **Total** | **~105 bytes** |

**Finalization message increase (per proof):**

| Field | Size |
|-------|------|
| chunk_index | 4 bytes |
| leaf_hash | 32 bytes |
| path_hashes | ~320 bytes (10 levels for ~1000 chunks) |
| path_directions | ~10 bytes |
| **Per proof** | **~366 bytes** |
| **Total for m=8** | **~2,928 bytes** |

**Comparison:**

| | Current | With LEP-5 (m=8) |
|---|---------|------------------|
| Registration | ~2,500 bytes | ~2,605 bytes (+4%) |
| Finalization | ~2,500 bytes | ~5,428 bytes (+117%) |
| **Trade-off** | No data integrity proof | Full protection against fake storage |

**Gas costs:**

| Operation | Estimated Gas |
|-----------|--------------|
| Commitment storage | ~25,000 |
| Per-proof verification | ~15,000 |
| **Total finalization overhead (m=8)** | **~145,000** |

---

## 7. Migration and Compatibility

### 7.1 Upgrade Strategy

**Phase 1: Soft Launch** (activation height to activation + ~50,000 blocks)
- SVC fields accepted but not required
- SuperNodes and clients upgraded to support SVC
- Monitoring for issues, parameter tuning

**Phase 2: Enforcement** (after soft launch period)
- SVC required for all new Cascade actions
- Actions without `AvailabilityCommitment` rejected
- Full protection enabled

### 7.2 Activation

Add chain parameter: `lep5_enabled_height`
- Before activation: existing finalization rules apply
- After activation: `MsgRequestAction` for Cascade must include `AvailabilityCommitment` (with `challenge_indices`); `MsgFinalizeAction` must include valid `chunk_proofs`

### 7.3 Backward Compatibility

| Component | Impact | Migration |
|-----------|--------|-----------|
| Existing finalized actions | None | Already complete |
| SuperNode software | Must upgrade | Proof generation in finalize path |
| Client JS SDK | Must upgrade | Add `computeCommitment()` + challenge index generation |
| rq-library (WASM) | **No changes** | Chunk commitment is pre-RQ |
| Action Module | New validation | Chain upgrade required |

---

## 8. Recommended Parameters

| Parameter | Mainnet | Testnet | Rationale |
|-----------|---------|---------|-----------|
| `total_size` (min) | 4 bytes | 4 bytes | Reject trivially tiny files |
| `chunk_size` (max) | 262,144 (256 KiB) | 262,144 | Default for large files |
| `chunk_size` (min) | 1 (1 byte) | 1 | Floor; allows small files to have enough chunks |
| `chunk_size` constraint | power of 2 | power of 2 | Simplifies client + tree construction |
| `svc_challenge_count` (m) | 8 | 5 | Strong security without excessive message size |
| `svc_min_chunks_for_challenge` | 4 | 4 | Unconditionally enforced — all files must produce ≥ 4 chunks |
| Challenge indices count | min(m, num_chunks) | min(m, num_chunks) | Always in [4, 8] since num_chunks ≥ 4 |

---

## 9. Future Extensions

### 9.1 Symbol-Level Commitment (`SYMBOL_MERKLE_V1`)

When/if rq-library exposes a symbol-iteration API in its WASM build, a stronger mode can commit directly to RaptorQ symbols. This binds the Merkle root to the actual Cascade storage artifacts, not just the input file. The `commitment_type` field enables this upgrade without breaking the protocol.

### 9.2 Continuous Storage Challenges

LEP-5 proves possession at finalization time. Future work can introduce periodic re-challenges (Proof of Space-Time) where SuperNodes must prove ongoing retention at random intervals, with slashing for failures.

### 9.3 Compressed Multi-Proofs

For large m values, batch Merkle proofs can share common path segments, reducing total proof size.

### 9.4 Server-Side Randomness Mixing

A future enhancement could mix validator-provided randomness (e.g., from block hash) into the challenge index generation to further harden against client–SuperNode collusion on index selection.

---

## 10. Test Vectors

### 10.1 Merkle Tree Construction

```
INPUT:
  File: 4 chunks of data "C0", "C1", "C2", "C3"

STEP 1 — Leaf hashes (domain separated, BLAKE3):
  L0 = BLAKE3(0x00 || 0x00000000 || "C0")
  L1 = BLAKE3(0x00 || 0x00000001 || "C1")
  L2 = BLAKE3(0x00 || 0x00000002 || "C2")
  L3 = BLAKE3(0x00 || 0x00000003 || "C3")

STEP 2 — Internal nodes (domain separated, BLAKE3):
  I01 = BLAKE3(0x01 || L0 || L1)
  I23 = BLAKE3(0x01 || L2 || L3)

STEP 3 — Root:
  ROOT = BLAKE3(0x01 || I01 || I23)
```

### 10.2 Challenge Index Generation

```
INPUT:
  action_id = "cascade_test_001"
  entropy_input = 0x999888777666... (32 bytes)
  height = 12345
  signer = 0xABCDEF... (20 bytes)
  num_chunks = 1000
  m = 5

COMPUTATION:
  seed = BLAKE3(entropy_input || "cascade_test_001" || uint64be(12345) || signer || "lep5/challenge/v1")

  For j=0: idx = uint64(BLAKE3(seed || uint32be(0))[0:8]) % 1000
  For j=1: idx = uint64(BLAKE3(seed || uint32be(1))[0:8]) % 1000
  ... (skip duplicates, increment counter)

EXPECTED: 5 unique indices in [0, 999]

These indices are stored in AvailabilityCommitment.challenge_indices at registration.
```

### 10.3 Proof Verification

```
Given 4-chunk tree with ROOT from test 10.1:

Proof for chunk 2:
  {
    chunk_index: 2,
    leaf_hash: L2,
    path_hashes: [L3, I01],
    path_directions: [true, false]  // L3 is RIGHT sibling, I01 is LEFT sibling
  }

Verification (BLAKE3):
  current = L2
  step 1: current = BLAKE3(0x01 || current || L3)   = I23   (L3 on RIGHT)
  step 2: current = BLAKE3(0x01 || I01 || current)   = ROOT  (I01 on LEFT)

  current == ROOT?  ✓ VALID
```

---

## 11. Canonical Encoding Rules (Normative)

- **Hash function:** BLAKE3 (via `lukechampine.com/blake3` in Go, `blake3` npm package in JS)
- **Hash output size:** 32 bytes
- **Leaf domain separator:** `0x00`
- **Internal node domain separator:** `0x01`
- **Integer encoding:** Big-endian (`uint32be`, `uint64be`)
- **Odd-level padding:** Duplicate last hash
- **Commitment type constant:** `"lep5/chunk-merkle/v1"`
- **Hash algo enum:** `HASH_ALGO_BLAKE3 = 1`
- **Challenge domain tag:** `"lep5/challenge/v1"`
- **Challenge indices:** Client-provided, stored in `AvailabilityCommitment.challenge_indices`

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 0.1 | 2025-02-08 | Initial drafts (two separate proposals) |
| 0.2 | 2026-02-08 | Combined proposal: chunk-based commitment, single-SN Merkle proof finalization |
| 0.3 | 2026-02-08 | Removed quorum/attestation layer; finalization relies solely on SuperNode-produced Merkle proofs |
| 0.4 | 2026-02-26 | BLAKE3 replaces SHA-256 for all hashing; hash_algo changed to HashAlgo enum; challenge indices now client-provided at registration (stored in AvailabilityCommitment.challenge_indices) instead of derived from block hash at finalization |
| 0.5 | 2026-02-26 | Variable chunk_size: chunk_size is now client-chosen power-of-2 in [1024, 262144]; chain enforces num_chunks >= svc_min_chunks_for_challenge for files >= 4 KiB, reducing SVC skip threshold from < 1 MiB to < 4 KiB |
| 0.6 | 2026-02-27 | Strict chunking boundaries: min file size 4 bytes; unconditional min 4 chunks; min 4 / max 8 challenge indices; max chunk size 256 KiB |
| 0.7 | 2026-02-27 | Min chunk size lowered from 1 KiB to 1 byte to allow 4-byte files to produce 4 chunks with chunk_size=1 |

---

**Document Status:** Draft — Pending Review
