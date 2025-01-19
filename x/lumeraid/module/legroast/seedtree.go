package legroast

import (
	"bytes"
	"crypto/rand"
	"fmt"
)

type seedTree struct {
	nRounds     uint32 // Number of rounds
	nParties    uint32 // Number of parties
	nPartyDepth uint32 // Number of party depth
	RoundSize   uint32 // Size of each round in bytes

	Data []byte // Flattened data [NRounds] [(Parties*2-1) * seedBytes]
}

// newSeedTree creates a new seedTree instance.
func newSeedTree(params *LegRoastParams) *seedTree {
	// Allocate sufficient space for the seed tree
	size := params.NRounds * (params.Parties*2 - 1) * seedBytes
	return &seedTree{
		nRounds:     params.NRounds,
		nParties:    params.Parties,
		nPartyDepth: params.NPartyDepth,
		RoundSize:   (params.Parties*2 - 1) * seedBytes,
		Data:        make([]byte, size),
	}
}

// Clear resets all data in the seedTree to zero.
func (st *seedTree) Clear() {
	for i := range st.Data {
		st.Data[i] = 0
	}
}

// roundBaseIndex calculates the starting index in the flattened Data slice for a specific round.
func (st *seedTree) roundBaseIndex(round uint32) uint32 {
	if round >= st.nRounds {
		panic("seedtree roundBaseIndex: round out of bounds")
	}
	return round * st.RoundSize
}

// Utility functions for tree structure.
func leftChild(i uint32) uint32 {
	return 2*i + 1
}

func parent(i uint32) uint32 {
	return (i - 1) / 2
}

func sibling(i uint32) uint32 {
	if i%2 == 0 {
		return i - 1
	}
	return i + 1
}

// GetRoundSlice returns a slice for a specific round in the flattened Data.
// The returned slice points to the relevant portion of the Data slice.
func (st *seedTree) GetRoundSlice(round uint32) []byte {
	baseIndex := st.roundBaseIndex(round)
	return st.Data[baseIndex : baseIndex+st.RoundSize]
}

// Generate generates the seed tree using the expand function.
func (st *seedTree) Generate(round uint32) {
	roundSlice := st.GetRoundSlice(round)

	// pick root seed
	_, err := rand.Read(roundSlice[:seedBytes])
	if err != nil {
		panic(fmt.Errorf("failed to generate random seed: %w", err))
	}
	/*
		// Deterministic root seed for debugging
		roundLE := make([]byte, 4)
		binary.LittleEndian.PutUint32(roundLE, round)
		hasher := sha256.New()
		hasher.Write(roundLE)
		firstHash := hasher.Sum(nil)
		hasher.Reset()
		hasher.Write(firstHash)
		finalHash := hasher.Sum(nil)
		copy(roundSlice[:seedBytes], finalHash[:seedBytes])
	*/

	srcIndex := uint32(0)
	for i := uint32(0); i < st.nParties-1; i++ {
		// Expand the current seed into the left child
		dstIndex := leftChild(i) * seedBytes
		LRExpand(roundSlice[srcIndex:srcIndex+seedBytes], roundSlice[dstIndex:dstIndex+2*seedBytes])

		srcIndex += seedBytes
	}
}

// releaseSeeds releases the seeds from the tree for a given unopened index.
func (st *seedTree) ReleaseSeeds(round, unopenedIndex uint32, out []byte) {
	unopenedIndex += st.nParties - 1
	toReveal := st.nPartyDepth - 1
	roundSlice := st.GetRoundSlice(round)
	for {
		index := uint32(toReveal) * seedBytes
		siblingIndex := sibling(unopenedIndex) * seedBytes
		copy(out[index:index+seedBytes], roundSlice[siblingIndex:siblingIndex+seedBytes])
		if toReveal == 0 {
			break
		}
		unopenedIndex = parent(unopenedIndex)
		toReveal--
	}
}

// FillDown fills down the seeds in the tree for a given unopened index.
func (st *seedTree) FillDown(round, unopenedIndex uint32, in []byte) {
	unopenedIndex += st.nParties - 1
	roundSlice := st.GetRoundSlice(round)

	// Zero out the tree
	for i := range roundSlice {
		roundSlice[i] = 0
	}

	// Insert the "in" values into the tree
	toInsert := st.nPartyDepth - 1
	for {
		siblingIndex := sibling(unopenedIndex) * seedBytes
		index := toInsert * seedBytes
		copy(roundSlice[siblingIndex:siblingIndex+seedBytes], in[index:index+seedBytes])
		if toInsert == 0 {
			break
		}
		unopenedIndex = parent(unopenedIndex)
		toInsert--
	}

	// Generate the seed tree
	for nParty := uint32(0); nParty < st.nParties-1; nParty++ {
		// Compare the current node with the root node
		index := nParty * seedBytes
		if !bytes.Equal(roundSlice[:seedBytes], roundSlice[index:index+seedBytes]) {
			dstIndex := leftChild(nParty) * seedBytes
			LRExpand(roundSlice[index:index+seedBytes], roundSlice[dstIndex:dstIndex+2*seedBytes])
		}
	}
}
