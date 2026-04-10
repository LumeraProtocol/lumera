package keeper_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/cosmos/gogoproto/jsonpb"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

const (
	testCommitmentType      = "lep5/chunk-merkle/v1"
	testCommitmentChunkSize = uint32(262144)
)

var testCommitmentHashAlgo = actiontypes.HashAlgo_HASH_ALGO_BLAKE3

type CascadeCommitmentValidationSuite struct {
	KeeperTestSuite
}

func TestCascadeCommitmentValidationSuite(t *testing.T) {
	suite.Run(t, new(CascadeCommitmentValidationSuite))
}

func makeValidAvailabilityCommitment(totalSize uint64) *actiontypes.AvailabilityCommitment {
	return makeValidAvailabilityCommitmentWithChunkSize(totalSize, testCommitmentChunkSize)
}

func makeValidAvailabilityCommitmentWithChunkSize(totalSize uint64, chunkSize uint32) *actiontypes.AvailabilityCommitment {
	numChunks := uint32(0)
	if totalSize > 0 {
		numChunks = uint32((totalSize + uint64(chunkSize) - 1) / uint64(chunkSize))
	}

	return &actiontypes.AvailabilityCommitment{
		CommitmentType: testCommitmentType,
		HashAlgo:       testCommitmentHashAlgo,
		ChunkSize:      chunkSize,
		TotalSize:      totalSize,
		NumChunks:      numChunks,
		Root:           bytes.Repeat([]byte{0xAB}, 32),
	}
}

// makeValidAvailabilityCommitmentWithIndices builds a commitment for a file
// whose chunk count is >= minChunks (4), including valid challenge indices.
// Uses the default 256 KiB chunk size.
func makeValidAvailabilityCommitmentWithIndices(numChunks uint32) *actiontypes.AvailabilityCommitment {
	totalSize := uint64(numChunks) * uint64(testCommitmentChunkSize)
	c := makeValidAvailabilityCommitment(totalSize)

	// Default challenge count is 8; use min(8, numChunks).
	count := uint32(8)
	if count > numChunks {
		count = numChunks
	}
	c.ChallengeIndices = make([]uint32, count)
	for i := uint32(0); i < count; i++ {
		c.ChallengeIndices[i] = i
	}
	return c
}

func (suite *CascadeCommitmentValidationSuite) requestCascadeAction(metadata *actiontypes.CascadeMetadata) (*actiontypes.MsgRequestActionResponse, error) {
	msgServer := keeper.NewMsgServerImpl(suite.keeper)

	var metadataJSON bytes.Buffer
	marshaler := &jsonpb.Marshaler{}
	err := marshaler.Marshal(&metadataJSON, metadata)
	suite.Require().NoError(err)

	msg := &actiontypes.MsgRequestAction{
		Creator:     suite.creatorAddress.String(),
		ActionType:  actiontypes.ActionTypeCascade.String(),
		Metadata:    metadataJSON.String(),
		Price:       "100000ulume",
		FileSizeKbs: "123",
	}

	return msgServer.RequestAction(suite.ctx, msg)
}

// AT09
func (suite *CascadeCommitmentValidationSuite) TestRegistrationWithValidCommitmentSucceedsAndStoresCommitment() {
	// Use 4 chunks at 262144 = 1 MiB — satisfies num_chunks >= 4 enforcement.
	commitment := makeValidAvailabilityCommitmentWithIndices(4)

	inputMetadata := &actiontypes.CascadeMetadata{
		DataHash:               "test_hash",
		FileName:               "test_file",
		RqIdsIc:                20,
		Signatures:             suite.signatureCascade,
		AvailabilityCommitment: commitment,
	}

	res, err := suite.requestCascadeAction(inputMetadata)
	suite.Require().NoError(err)
	suite.Require().NotNil(res)

	storedAction, found := suite.keeper.GetActionByID(suite.ctx, res.ActionId)
	suite.Require().True(found)

	var storedMetadata actiontypes.CascadeMetadata
	err = gogoproto.Unmarshal(storedAction.Metadata, &storedMetadata)
	suite.Require().NoError(err)

	suite.Require().NotNil(storedMetadata.AvailabilityCommitment)
	suite.Equal(inputMetadata.AvailabilityCommitment, storedMetadata.AvailabilityCommitment)
}

// AT09b — registration with a small file using a reduced chunk_size to satisfy minimum chunks.
func (suite *CascadeCommitmentValidationSuite) TestRegistrationWithSmallFileAndReducedChunkSizeSucceeds() {
	// 500 KiB file (512000 bytes). At 262144 chunk_size → 2 chunks (would be rejected).
	// At 131072 chunk_size → 4 chunks → accepted.
	totalSize := uint64(512000)
	chunkSize := uint32(131072)
	numChunks := uint32((totalSize + uint64(chunkSize) - 1) / uint64(chunkSize)) // = 4
	challengeCount := numChunks
	if challengeCount > 8 {
		challengeCount = 8
	}
	indices := make([]uint32, challengeCount)
	for i := uint32(0); i < challengeCount; i++ {
		indices[i] = i
	}

	commitment := &actiontypes.AvailabilityCommitment{
		CommitmentType:   testCommitmentType,
		HashAlgo:         testCommitmentHashAlgo,
		ChunkSize:        chunkSize,
		TotalSize:        totalSize,
		NumChunks:        numChunks,
		Root:             bytes.Repeat([]byte{0xAB}, 32),
		ChallengeIndices: indices,
	}

	inputMetadata := &actiontypes.CascadeMetadata{
		DataHash:               "test_hash",
		FileName:               "test_file",
		RqIdsIc:                20,
		Signatures:             suite.signatureCascade,
		AvailabilityCommitment: commitment,
	}

	res, err := suite.requestCascadeAction(inputMetadata)
	suite.Require().NoError(err, "500 KiB file with 131072 chunk_size should succeed")
	suite.Require().NotNil(res)
}

// AT09c — 4-byte file with 1-byte chunk size produces exactly 4 chunks.
func (suite *CascadeCommitmentValidationSuite) TestRegistrationWith4ByteFileAnd1ByteChunkSizeSucceeds() {
	totalSize := uint64(4)
	chunkSize := uint32(1)
	numChunks := uint32(4) // ceil(4 / 1) = 4
	challengeCount := numChunks
	if challengeCount > 8 {
		challengeCount = 8
	}
	indices := make([]uint32, challengeCount)
	for i := uint32(0); i < challengeCount; i++ {
		indices[i] = i
	}

	commitment := &actiontypes.AvailabilityCommitment{
		CommitmentType:   testCommitmentType,
		HashAlgo:         testCommitmentHashAlgo,
		ChunkSize:        chunkSize,
		TotalSize:        totalSize,
		NumChunks:        numChunks,
		Root:             bytes.Repeat([]byte{0xAB}, 32),
		ChallengeIndices: indices,
	}

	inputMetadata := &actiontypes.CascadeMetadata{
		DataHash:               "test_hash",
		FileName:               "test_file",
		RqIdsIc:                20,
		Signatures:             suite.signatureCascade,
		AvailabilityCommitment: commitment,
	}

	res, err := suite.requestCascadeAction(inputMetadata)
	suite.Require().NoError(err, "4-byte file with 1-byte chunk_size should succeed")
	suite.Require().NotNil(res)
}

// AT10
func (suite *CascadeCommitmentValidationSuite) TestRegistrationWithInvalidCommitmentRejected() {
	testCases := []struct {
		name      string
		mutate    func(*actiontypes.AvailabilityCommitment)
		errorText string
	}{
		{
			name: "invalid commitment_type",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.CommitmentType = "invalid/type"
			},
			errorText: "availability_commitment.commitment_type",
		},
		{
			name: "invalid num_chunks",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.NumChunks++
			},
			errorText: "availability_commitment.num_chunks",
		},
		{
			name: "non-power-of-2 chunk_size",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.ChunkSize = 3000 // not a power of 2
			},
			errorText: "power of 2",
		},
		{
			name: "chunk_size above maximum (524288 > 262144)",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.ChunkSize = 524288
			},
			errorText: "must be in",
		},
		{
			name: "chunk_size too large for file to produce 4 chunks",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				// 500 KiB file with 262144 chunk_size → 2 chunks < 4 → rejected.
				c.TotalSize = 512000
				c.ChunkSize = 262144
				c.NumChunks = 2
			},
			errorText: "below minimum",
		},
		{
			name: "total_size below minimum (< 4 bytes)",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.TotalSize = 3
				c.ChunkSize = 1024
				c.NumChunks = 1
			},
			errorText: "total_size must be >= 4",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Use 4 chunks (1 MiB) as the base valid commitment.
			commitment := makeValidAvailabilityCommitmentWithIndices(4)
			tc.mutate(commitment)

			metadata := &actiontypes.CascadeMetadata{
				DataHash:               "test_hash",
				FileName:               "test_file",
				RqIdsIc:                20,
				Signatures:             suite.signatureCascade,
				AvailabilityCommitment: commitment,
			}

			res, err := suite.requestCascadeAction(metadata)
			suite.Error(err)
			suite.Nil(res)
			suite.ErrorIs(err, actiontypes.ErrInvalidMetadata)
			suite.ErrorContains(err, tc.errorText)
		})
	}
}

// AT11
func (suite *CascadeCommitmentValidationSuite) TestRegistrationWithoutCommitmentStillSucceeds() {
	inputMetadata := &actiontypes.CascadeMetadata{
		DataHash:   "test_hash",
		FileName:   "test_file",
		RqIdsIc:    20,
		Signatures: suite.signatureCascade,
	}

	res, err := suite.requestCascadeAction(inputMetadata)
	suite.Require().NoError(err)
	suite.Require().NotNil(res)

	storedAction, found := suite.keeper.GetActionByID(suite.ctx, res.ActionId)
	suite.Require().True(found)

	var storedMetadata actiontypes.CascadeMetadata
	err = gogoproto.Unmarshal(storedAction.Metadata, &storedMetadata)
	suite.Require().NoError(err)

	suite.Nil(storedMetadata.AvailabilityCommitment)
}

// AT16 – registration with valid challenge indices (>= minChunks).
func (suite *CascadeCommitmentValidationSuite) TestRegistrationWithValidChallengeIndicesSucceeds() {
	testCases := []struct {
		name      string
		numChunks uint32
	}{
		{"5 chunks (fewer than 8, expects 5 indices)", 5},
		{"8 chunks (exactly 8 indices)", 8},
		{"10 chunks (capped at 8 indices)", 10},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			commitment := makeValidAvailabilityCommitmentWithIndices(tc.numChunks)

			inputMetadata := &actiontypes.CascadeMetadata{
				DataHash:               "test_hash",
				FileName:               "test_file",
				RqIdsIc:                20,
				Signatures:             suite.signatureCascade,
				AvailabilityCommitment: commitment,
			}

			res, err := suite.requestCascadeAction(inputMetadata)
			suite.Require().NoError(err, "numChunks=%d", tc.numChunks)
			suite.Require().NotNil(res)
		})
	}
}

// AT17 – invalid challenge indices are rejected at registration.
func (suite *CascadeCommitmentValidationSuite) TestRegistrationWithInvalidChallengeIndicesRejected() {
	testCases := []struct {
		name      string
		mutate    func(*actiontypes.AvailabilityCommitment)
		errorText string
	}{
		{
			name: "zero indices when file has enough chunks",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.ChallengeIndices = nil
			},
			errorText: "challenge_indices must have",
		},
		{
			name: "too many indices",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.ChallengeIndices = append(c.ChallengeIndices, 9)
			},
			errorText: "challenge_indices must have",
		},
		{
			name: "too few indices",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.ChallengeIndices = c.ChallengeIndices[:len(c.ChallengeIndices)-1]
			},
			errorText: "challenge_indices must have",
		},
		{
			name: "index out of range",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.ChallengeIndices[0] = c.NumChunks // == numChunks, out of [0, numChunks)
			},
			errorText: "out of range",
		},
		{
			name: "duplicate index",
			mutate: func(c *actiontypes.AvailabilityCommitment) {
				c.ChallengeIndices[1] = c.ChallengeIndices[0]
			},
			errorText: "duplicate",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// 10 chunks → expects exactly 8 indices by default.
			commitment := makeValidAvailabilityCommitmentWithIndices(10)
			tc.mutate(commitment)

			metadata := &actiontypes.CascadeMetadata{
				DataHash:               "test_hash",
				FileName:               "test_file",
				RqIdsIc:                20,
				Signatures:             suite.signatureCascade,
				AvailabilityCommitment: commitment,
			}

			res, err := suite.requestCascadeAction(metadata)
			suite.Error(err)
			suite.Nil(res)
			suite.ErrorIs(err, actiontypes.ErrInvalidMetadata)
			suite.ErrorContains(err, tc.errorText)
		})
	}
}

