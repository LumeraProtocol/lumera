package keeper_test

import (
	"testing"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	"github.com/LumeraProtocol/lumera/x/action/v1/merkle"
	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFinalizeCascade_WithValidChunkProofs_SetsDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	creatorKey, creatorAddr := cryptotestutils.KeyAndAddress()
	_, supernodeAddr := cryptotestutils.KeyAndAddress()

	k, ctx := keepertest.ActionKeeperWithAddress(t, ctrl, []keepertest.AccountPair{
		{Address: creatorAddr, PubKey: creatorKey.PubKey()},
	})

	ctx = ctx.WithBlockHeight(42).WithEventManager(sdk.NewEventManager())

	signatureCascade, err := cryptotestutils.CreateSignatureString([]secp256k1.PrivKey{creatorKey}, 50)
	require.NoError(t, err)

	chunks := [][]byte{[]byte("C0"), []byte("C1"), []byte("C2"), []byte("C3")}
	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err)

	// Client-picked challenge indices stored at registration.
	challengeIndices := []uint32{0, 1, 2, 3}

	root := append([]byte(nil), tree.Root[:]...)
	registerMeta := &actiontypes.CascadeMetadata{
		DataHash:   "hash",
		FileName:   "file.bin",
		RqIdsIc:    20,
		RqIdsMax:   50,
		Signatures: signatureCascade,
		AvailabilityCommitment: &actiontypes.AvailabilityCommitment{
			CommitmentType:   "lep5/chunk-merkle/v1",
			HashAlgo:         actiontypes.HashAlgo_HASH_ALGO_BLAKE3,
			ChunkSize:        svcChunkSize,
			TotalSize:        uint64(4) * uint64(svcChunkSize),
			NumChunks:        4,
			Root:             root,
			ChallengeIndices: challengeIndices,
		},
	}

	registerMetaBz, err := gogoproto.Marshal(registerMeta)
	require.NoError(t, err)

	action := &actiontypes.Action{
		Creator:    creatorAddr.String(),
		ActionType: actiontypes.ActionTypeCascade,
		Price:      "100000ulume",
		Metadata:   registerMetaBz,
	}

	_, err = k.RegisterAction(ctx, action)
	require.NoError(t, err)

	mockQuery, ok := k.GetSupernodeQueryServer().(*supernodemocks.MockQueryServer)
	require.True(t, ok)
	mockQuery.EXPECT().
		GetTopSuperNodesForBlock(gomock.AssignableToTypeOf(ctx), gomock.AssignableToTypeOf(&sntypes.QueryGetTopSuperNodesForBlockRequest{})).
		Return(&sntypes.QueryGetTopSuperNodesForBlockResponse{Supernodes: []*sntypes.SuperNode{{SupernodeAccount: supernodeAddr.String()}}}, nil).
		Times(1)

	// Generate proofs matching the stored challenge indices.
	proofs := make([]*actiontypes.ChunkProof, 0, len(challengeIndices))
	for _, idx := range challengeIndices {
		proof, pErr := tree.GenerateProof(int(idx))
		require.NoError(t, pErr)
		proofs = append(proofs, toChunkProof(proof))
	}

	rqIDs := make([]string, 0, 50)
	for i := uint64(20); i < 70; i++ {
		id, idErr := actionkeeper.CreateKademliaID(signatureCascade, i)
		require.NoError(t, idErr)
		rqIDs = append(rqIDs, id)
	}

	finalizeMeta := &actiontypes.CascadeMetadata{
		RqIdsIds:    rqIDs,
		ChunkProofs: proofs,
	}
	finalizeMetaBz, err := gogoproto.Marshal(finalizeMeta)
	require.NoError(t, err)

	err = k.FinalizeAction(ctx, action.ActionID, supernodeAddr.String(), finalizeMetaBz)
	require.NoError(t, err)

	stored, found := k.GetActionByID(ctx, action.ActionID)
	require.True(t, found)
	require.Equal(t, actiontypes.ActionStateDone, stored.State)

	var storedMeta actiontypes.CascadeMetadata
	require.NoError(t, gogoproto.Unmarshal(stored.Metadata, &storedMeta))
	require.Len(t, storedMeta.GetChunkProofs(), 4)
}
