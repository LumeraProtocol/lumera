package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/merkle"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
)

const svcChunkSize = uint32(262144)

func TestVerifyChunkProofs(t *testing.T) {
	t.Run("AT12_valid_proofs_succeed", func(t *testing.T) {
		k, ctx, action, supernode, expected := setupSVCFixture(t, 4)

		err := k.VerifyChunkProofs(ctx, action, supernode, expected)
		require.NoError(t, err)
	})

	t.Run("AT13_wrong_chunk_index_rejected", func(t *testing.T) {
		k, ctx, action, supernode, expected := setupSVCFixture(t, 4)
		expected[0].ChunkIndex = (expected[0].ChunkIndex + 1) % 4

		err := k.VerifyChunkProofs(ctx, action, supernode, expected)
		require.ErrorIs(t, err, actiontypes.ErrWrongChallengeIndex)

		assertSVCEvidenceEvent(t, ctx.EventManager().Events())
	})

	t.Run("AT14_invalid_merkle_path_rejected", func(t *testing.T) {
		k, ctx, action, supernode, expected := setupSVCFixture(t, 4)
		expected[0].PathHashes[0][0] ^= 0xFF

		err := k.VerifyChunkProofs(ctx, action, supernode, expected)
		require.ErrorIs(t, err, actiontypes.ErrInvalidMerkleProof)

		assertSVCEvidenceEvent(t, ctx.EventManager().Events())
	})

	t.Run("AT15_wrong_proof_count_rejected", func(t *testing.T) {
		k, ctx, action, supernode, expected := setupSVCFixture(t, 4)
		short := expected[:len(expected)-1]

		err := k.VerifyChunkProofs(ctx, action, supernode, short)
		require.ErrorIs(t, err, actiontypes.ErrWrongProofCount)

		assertSVCEvidenceEvent(t, ctx.EventManager().Events())
	})

	t.Run("AT16_svc_skipped_for_small_files", func(t *testing.T) {
		k, ctx, action, supernode, _ := setupSVCFixture(t, 3)

		err := k.VerifyChunkProofs(ctx, action, supernode, nil)
		require.NoError(t, err)
	})
}

func setupSVCFixture(t *testing.T, numChunks uint32) (k actionkeeper.Keeper, ctx sdk.Context, action *actiontypes.Action, supernode string, proofs []*actiontypes.ChunkProof) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	k, baseCtx := keepertest.ActionKeeper(t, ctrl)

	priv, addr := cryptotestutils.KeyAndAddress()
	supernode = addr.String()

	chunks := make([][]byte, 0, numChunks)
	for i := uint32(0); i < numChunks; i++ {
		chunks = append(chunks, []byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3)})
	}

	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err)

	var root []byte
	root = append(root, tree.Root[:]...)

	// Client picks challenge indices at registration time.
	// For testing, use simple sequential indices: [0, 1, 2, 3].
	challengeCount := uint32(4)
	if challengeCount > numChunks {
		challengeCount = numChunks
	}
	challengeIndices := make([]uint32, 0, challengeCount)
	for i := uint32(0); i < challengeCount; i++ {
		challengeIndices = append(challengeIndices, i)
	}

	metadata := &actiontypes.CascadeMetadata{
		DataHash:   "hash",
		FileName:   "file.bin",
		RqIdsIc:    1,
		RqIdsMax:   50,
		Signatures: "sig",
		AvailabilityCommitment: &actiontypes.AvailabilityCommitment{
			CommitmentType:   "lep5/chunk-merkle/v1",
			HashAlgo:         actiontypes.HashAlgo_HASH_ALGO_BLAKE3,
			ChunkSize:        svcChunkSize,
			TotalSize:        uint64(numChunks) * uint64(svcChunkSize),
			NumChunks:        numChunks,
			Root:             root,
			ChallengeIndices: challengeIndices,
		},
	}

	metaBz, err := gogoproto.Marshal(metadata)
	require.NoError(t, err)

	action = &actiontypes.Action{
		ActionID:    "svc-action",
		ActionType:  actiontypes.ActionTypeCascade,
		Creator:     addr.String(),
		Metadata:    metaBz,
		BlockHeight: 42,
	}

	params := k.GetParams(baseCtx)
	params.SvcChallengeCount = 4
	params.SvcMinChunksForChallenge = 4
	require.NoError(t, k.SetParams(baseCtx, params))

	ctx = baseCtx.WithBlockHeight(42).WithEventManager(sdk.NewEventManager())

	// Generate proofs matching the stored challenge indices.
	proofs = make([]*actiontypes.ChunkProof, 0, len(challengeIndices))
	for _, idx := range challengeIndices {
		p, pErr := tree.GenerateProof(int(idx))
		require.NoError(t, pErr)
		proofs = append(proofs, toChunkProof(p))
	}
	_ = priv

	return k, ctx, action, supernode, proofs
}

func toChunkProof(p *merkle.Proof) *actiontypes.ChunkProof {
	leaf := make([]byte, merkle.HashSize)
	copy(leaf, p.LeafHash[:])

	pathHashes := make([][]byte, 0, len(p.PathHashes))
	for _, h := range p.PathHashes {
		b := make([]byte, merkle.HashSize)
		copy(b, h[:])
		pathHashes = append(pathHashes, b)
	}

	directions := append([]bool(nil), p.PathDirections...)

	return &actiontypes.ChunkProof{
		ChunkIndex:     p.ChunkIndex,
		LeafHash:       leaf,
		PathHashes:     pathHashes,
		PathDirections: directions,
	}
}

func assertSVCEvidenceEvent(t *testing.T, events sdk.Events) {
	t.Helper()

	found := false
	for _, e := range events {
		if e.Type == actiontypes.EventTypeSVCEvidence {
			found = true
			break
		}
	}
	require.True(t, found, "expected SVC evidence event")
}
