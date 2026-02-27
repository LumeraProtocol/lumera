package challenge

import (
	"encoding/binary"

	"lukechampine.com/blake3"
)

const domainTag = "lep5/challenge/v1"

// DeriveIndices generates deterministic pseudo-random unique indices.
//
// Seed formula (F03):
//   BLAKE3(prev_block_hash || action_id || uint64be(height) || signer_addr || "lep5/challenge/v1")
func DeriveIndices(actionID string, prevBlockHash []byte, height uint64, signerAddr []byte, numChunks uint32, m uint32) []uint32 {
	if numChunks == 0 || m == 0 {
		return []uint32{}
	}

	if m > numChunks {
		m = numChunks
	}

	var heightBytes [8]byte
	binary.BigEndian.PutUint64(heightBytes[:], height)

	seedInput := make([]byte, 0, len(prevBlockHash)+len(actionID)+len(heightBytes)+len(signerAddr)+len(domainTag))
	seedInput = append(seedInput, prevBlockHash...)
	seedInput = append(seedInput, actionID...)
	seedInput = append(seedInput, heightBytes[:]...)
	seedInput = append(seedInput, signerAddr...)
	seedInput = append(seedInput, domainTag...)

	seed := blake3.Sum256(seedInput)

	indices := make([]uint32, 0, m)
	used := make(map[uint32]struct{}, m)
	var counter uint32

	for uint32(len(indices)) < m {
		var counterBytes [4]byte
		binary.BigEndian.PutUint32(counterBytes[:], counter)

		h := blake3.New(32, nil)
		_, _ = h.Write(seed[:])
		_, _ = h.Write(counterBytes[:])
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
