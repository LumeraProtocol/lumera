package types

import (
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
)

func TestAvailabilityCommitmentAndChunkProofRoundTrip(t *testing.T) {
	commitment := &AvailabilityCommitment{
		CommitmentType: "lep5/chunk-merkle/v1",
		HashAlgo:       HashAlgo_HASH_ALGO_BLAKE3,
		ChunkSize:      262144,
		TotalSize:      1048576,
		NumChunks:      4,
		Root:           []byte("0123456789abcdef0123456789abcdef"),
	}

	bz, err := proto.Marshal(commitment)
	require.NoError(t, err)

	var decodedCommitment AvailabilityCommitment
	require.NoError(t, proto.Unmarshal(bz, &decodedCommitment))
	require.Equal(t, commitment, &decodedCommitment)

	proof := &ChunkProof{
		ChunkIndex: 2,
		LeafHash:   []byte("leaf-hash-2"),
		PathHashes: [][]byte{[]byte("sibling-0"), []byte("sibling-1")},
		PathDirections: []bool{
			true,
			false,
		},
	}

	proofBz, err := proto.Marshal(proof)
	require.NoError(t, err)

	var decodedProof ChunkProof
	require.NoError(t, proto.Unmarshal(proofBz, &decodedProof))
	require.Equal(t, proof, &decodedProof)
}

func TestCascadeMetadataBackwardsCompatibleWithoutNewFields(t *testing.T) {
	legacyLike := &CascadeMetadata{
		DataHash:   "legacy-data-hash",
		FileName:   "legacy.txt",
		RqIdsIc:    10,
		RqIdsMax:   50,
		RqIdsIds:   []string{"id-1", "id-2"},
		Signatures: "legacy-signature",
		Public:     true,
	}

	bz, err := proto.Marshal(legacyLike)
	require.NoError(t, err)

	var decoded CascadeMetadata
	require.NoError(t, proto.Unmarshal(bz, &decoded))

	require.Equal(t, legacyLike.DataHash, decoded.DataHash)
	require.Equal(t, legacyLike.FileName, decoded.FileName)
	require.Equal(t, legacyLike.RqIdsIc, decoded.RqIdsIc)
	require.Equal(t, legacyLike.RqIdsMax, decoded.RqIdsMax)
	require.Equal(t, legacyLike.RqIdsIds, decoded.RqIdsIds)
	require.Equal(t, legacyLike.Signatures, decoded.Signatures)
	require.Equal(t, legacyLike.Public, decoded.Public)
	require.Nil(t, decoded.AvailabilityCommitment)
	require.Empty(t, decoded.ChunkProofs)
}

func TestCascadeMetadataRoundTripWithNewFields(t *testing.T) {
	extended := &CascadeMetadata{
		DataHash:   "extended-data-hash",
		FileName:   "extended.txt",
		RqIdsIc:    11,
		RqIdsMax:   99,
		RqIdsIds:   []string{"id-a", "id-b", "id-c"},
		Signatures: "extended-signature",
		Public:     false,
		AvailabilityCommitment: &AvailabilityCommitment{
			CommitmentType: "lep5/chunk-merkle/v1",
			HashAlgo:       HashAlgo_HASH_ALGO_BLAKE3,
			ChunkSize:      262144,
			TotalSize:      786432,
			NumChunks:      3,
			Root:           []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		},
		ChunkProofs: []*ChunkProof{
			{
				ChunkIndex:     0,
				LeafHash:       []byte("leaf-0"),
				PathHashes:     [][]byte{[]byte("s0"), []byte("s1")},
				PathDirections: []bool{true, false},
			},
		},
	}

	bz, err := proto.Marshal(extended)
	require.NoError(t, err)

	var decoded CascadeMetadata
	require.NoError(t, proto.Unmarshal(bz, &decoded))
	require.Equal(t, extended, &decoded)
}

