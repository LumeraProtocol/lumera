package merkle

import (
	"encoding/binary"
	"errors"

	"lukechampine.com/blake3"
)

const HashSize = 32

var (
	ErrEmptyInput      = errors.New("empty input")
	ErrIndexOutOfRange = errors.New("index out of range")
)

var internalPrefix = [1]byte{0x01}

// HashLeaf computes BLAKE3(0x00 || uint32be(index) || data).
func HashLeaf(index uint32, data []byte) [HashSize]byte {
	var prefix [5]byte
	prefix[0] = 0x00
	binary.BigEndian.PutUint32(prefix[1:], index)

	h := blake3.New(HashSize, nil)
	_, _ = h.Write(prefix[:])
	_, _ = h.Write(data)

	var result [HashSize]byte
	copy(result[:], h.Sum(nil))
	return result
}

// HashInternal computes BLAKE3(0x01 || left || right).
func HashInternal(left, right [HashSize]byte) [HashSize]byte {
	h := blake3.New(HashSize, nil)
	_, _ = h.Write(internalPrefix[:])
	_, _ = h.Write(left[:])
	_, _ = h.Write(right[:])

	var result [HashSize]byte
	copy(result[:], h.Sum(nil))
	return result
}

type Tree struct {
	Root      [HashSize]byte
	Leaves    [][HashSize]byte
	Levels    [][][HashSize]byte // levels[0] = leaves (possibly padded), levels[last] = root level
	LeafCount int
}

// BuildTree constructs a Merkle tree from chunk data.
// If a level has an odd number of nodes, the last node is duplicated.
func BuildTree(chunks [][]byte) (*Tree, error) {
	n := len(chunks)
	if n == 0 {
		return nil, ErrEmptyInput
	}

	leaves := make([][HashSize]byte, n)
	for i, chunk := range chunks {
		leaves[i] = HashLeaf(uint32(i), chunk)
	}

	levels := make([][][HashSize]byte, 0)
	current := make([][HashSize]byte, n)
	copy(current, leaves)
	levels = append(levels, current)

	for len(current) > 1 {
		// If odd number of nodes, duplicate the last node.
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
	PathDirections []bool // true = sibling on right, false = sibling on left
}

// GenerateProof creates a Merkle proof for a chunk index.
func (t *Tree) GenerateProof(index int) (*Proof, error) {
	if index < 0 || index >= t.LeafCount {
		return nil, ErrIndexOutOfRange
	}

	proof := &Proof{
		ChunkIndex:     uint32(index),
		LeafHash:       t.Leaves[index],
		PathHashes:     make([][HashSize]byte, 0, len(t.Levels)-1),
		PathDirections: make([]bool, 0, len(t.Levels)-1),
	}

	idx := index
	for level := 0; level < len(t.Levels)-1; level++ {
		nodes := t.Levels[level]
		if idx%2 == 0 {
			proof.PathHashes = append(proof.PathHashes, nodes[idx+1])
			proof.PathDirections = append(proof.PathDirections, true)
		} else {
			proof.PathHashes = append(proof.PathHashes, nodes[idx-1])
			proof.PathDirections = append(proof.PathDirections, false)
		}
		idx /= 2
	}

	return proof, nil
}

// Verify checks the proof against a Merkle root.
func (p *Proof) Verify(root [HashSize]byte) bool {
	if p == nil {
		return false
	}
	if len(p.PathHashes) != len(p.PathDirections) {
		return false
	}

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
