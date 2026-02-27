package merkle

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustDecodeHex32(t *testing.T, s string) [HashSize]byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	require.Len(t, b, HashSize)

	var out [HashSize]byte
	copy(out[:], b)
	return out
}

// AT01: 4-chunk tree root matches LEP-5 vector shape (Section 10.1).
func TestBuildTree_FourChunkRootMatchesVector(t *testing.T) {
	chunks := [][]byte{[]byte("C0"), []byte("C1"), []byte("C2"), []byte("C3")}

	tree, err := BuildTree(chunks)
	require.NoError(t, err)

	// Precomputed from LEP-5 Section 10.1 formulas (Blake3).
	expectedRoot := mustDecodeHex32(t, "c226a548f56fcac8d7c63a4fa74d01970c99d23552f81744821f4e0e5feb1bed")
	require.Equal(t, expectedRoot, tree.Root)
}

// AT02: Proof for chunk 2 verifies (LEP-5 Section 10.3 path semantics).
func TestGenerateProof_Chunk2Verifies(t *testing.T) {
	chunks := [][]byte{[]byte("C0"), []byte("C1"), []byte("C2"), []byte("C3")}

	tree, err := BuildTree(chunks)
	require.NoError(t, err)

	proof, err := tree.GenerateProof(2)
	require.NoError(t, err)

	require.Equal(t, uint32(2), proof.ChunkIndex)
	require.Equal(t, HashLeaf(2, []byte("C2")), proof.LeafHash)
	require.Len(t, proof.PathHashes, 2)
	require.Len(t, proof.PathDirections, 2)
	require.Equal(t, []bool{true, false}, proof.PathDirections)
	require.True(t, proof.Verify(tree.Root))
}

// AT03: Tampered leaf hash fails verification.
func TestVerify_TamperedLeafFails(t *testing.T) {
	chunks := [][]byte{[]byte("C0"), []byte("C1"), []byte("C2"), []byte("C3")}

	tree, err := BuildTree(chunks)
	require.NoError(t, err)

	proof, err := tree.GenerateProof(2)
	require.NoError(t, err)
	require.True(t, proof.Verify(tree.Root))

	tampered := *proof
	tampered.LeafHash[0] ^= 0xFF
	require.False(t, tampered.Verify(tree.Root))
}

// AT04: Handle edge and scale cases (single chunk, 1000+ chunks).
func TestBuildTree_EdgeAndScaleCases(t *testing.T) {
	t.Run("single chunk", func(t *testing.T) {
		chunks := [][]byte{[]byte("only")}
		tree, err := BuildTree(chunks)
		require.NoError(t, err)

		expectedLeaf := HashLeaf(0, []byte("only"))
		require.Equal(t, expectedLeaf, tree.Root)

		proof, err := tree.GenerateProof(0)
		require.NoError(t, err)
		require.Empty(t, proof.PathHashes)
		require.Empty(t, proof.PathDirections)
		require.True(t, proof.Verify(tree.Root))
	})

	t.Run("1001 chunks", func(t *testing.T) {
		chunks := make([][]byte, 1001)
		for i := range chunks {
			chunks[i] = []byte{byte(i & 0xFF), byte((i >> 8) & 0xFF), 0xAA}
		}

		tree, err := BuildTree(chunks)
		require.NoError(t, err)
		require.Equal(t, 1001, tree.LeafCount)
		require.NotEqual(t, [HashSize]byte{}, tree.Root)

		indices := []int{0, 500, 1000}
		for _, idx := range indices {
			proof, err := tree.GenerateProof(idx)
			require.NoError(t, err)
			require.True(t, proof.Verify(tree.Root))
		}
	})
}

func TestBuildTree_Errors(t *testing.T) {
	_, err := BuildTree(nil)
	require.ErrorIs(t, err, ErrEmptyInput)
}

func TestGenerateProof_OutOfRange(t *testing.T) {
	tree, err := BuildTree([][]byte{[]byte("C0")})
	require.NoError(t, err)

	_, err = tree.GenerateProof(-1)
	require.ErrorIs(t, err, ErrIndexOutOfRange)

	_, err = tree.GenerateProof(1)
	require.ErrorIs(t, err, ErrIndexOutOfRange)
}

func TestProofVerify_InvalidPathLengths(t *testing.T) {
	root := HashLeaf(0, []byte("x"))
	p := &Proof{
		LeafHash:       root,
		PathHashes:     [][HashSize]byte{{}},
		PathDirections: nil,
	}
	require.False(t, p.Verify(root))
}
