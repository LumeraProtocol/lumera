package action_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/merkle"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const lep5ChunkSize = uint32(262144) // 256 KiB

// LEP5IntegrationTestSuite exercises the full LEP-5 Cascade availability
// commitment flow through the real application stack.
type LEP5IntegrationTestSuite struct {
	suite.Suite

	app       *lumeraapp.App
	ctx       sdk.Context
	keeper    keeper.Keeper
	msgServer actiontypes.MsgServer

	testAddrs []sdk.AccAddress
	privKeys  []*secp256k1.PrivKey
}

func (s *LEP5IntegrationTestSuite) SetupTest() {
	app := lumeraapp.Setup(s.T())
	ctx := app.BaseApp.NewContext(false).WithBlockHeight(1).WithBlockTime(time.Now())

	s.app = app
	s.ctx = ctx
	s.keeper = app.ActionKeeper
	s.msgServer = keeper.NewMsgServerImpl(s.keeper)

	initCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))
	s.testAddrs, s.privKeys, _ = createTestAddAddrsWithKeys(5)

	for i, addr := range s.testAddrs {
		acc := app.AuthKeeper.GetAccount(s.ctx, addr)
		if acc == nil {
			account := app.AuthKeeper.NewAccountWithAddress(s.ctx, addr)
			baseAcc := account.(*authtypes.BaseAccount)
			baseAcc.SetPubKey(s.privKeys[i].PubKey())
			app.AuthKeeper.SetAccount(s.ctx, baseAcc)
		}
		require.NoError(s.T(), app.BankKeeper.MintCoins(s.ctx, actiontypes.ModuleName, initCoins))
		require.NoError(s.T(), app.BankKeeper.SendCoinsFromModuleToAccount(s.ctx, actiontypes.ModuleName, addr, initCoins))
	}

	valAddr := sdk.ValAddress(s.privKeys[1].PubKey().Address())
	sn := sntypes.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: s.testAddrs[1].String(),
		Note:             "1.0.0",
		States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}},
		PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "192.168.1.1"}},
		P2PPort:          "2134",
	}
	require.NoError(s.T(), app.SupernodeKeeper.SetSuperNode(s.ctx, sn))

	params := actiontypes.DefaultParams()
	params.ExpirationDuration = time.Minute
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, params))
}

// TestLEP5CascadeLifecycle registers a Cascade action with an AvailabilityCommitment,
// finalizes it with valid chunk proofs, and asserts the action reaches DONE.
func (s *LEP5IntegrationTestSuite) TestLEP5CascadeLifecycle() {
	t := s.T()
	txCreator := s.testAddrs[0].String()
	snAccount := s.testAddrs[1].String()

	// --- Build Merkle tree from 8 chunks ---
	numChunks := uint32(8)
	chunks := make([][]byte, numChunks)
	for i := uint32(0); i < numChunks; i++ {
		chunks[i] = []byte(fmt.Sprintf("chunk-%d-data", i))
	}

	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err)

	root := make([]byte, merkle.HashSize)
	copy(root, tree.Root[:])

	// Client picks challenge indices — must match min(SVCChallengeCount, numChunks) = 8.
	challengeIndices := []uint32{0, 1, 2, 3, 4, 5, 6, 7}

	// --- Register with AvailabilityCommitment ---
	sigStr, err := createValidCascadeSignatureString(s.privKeys[0], 1)
	require.NoError(t, err)

	commitment := actiontypes.AvailabilityCommitment{
		CommitmentType:   "lep5/chunk-merkle/v1",
		HashAlgo:         actiontypes.HashAlgo_HASH_ALGO_BLAKE3,
		ChunkSize:        lep5ChunkSize,
		TotalSize:        uint64(numChunks) * uint64(lep5ChunkSize),
		NumChunks:        numChunks,
		Root:             root,
		ChallengeIndices: challengeIndices,
	}
	commitmentJSON, err := json.Marshal(&commitment)
	require.NoError(t, err)

	metadata := fmt.Sprintf(
		`{"data_hash":"abc123","file_name":"file.txt","rq_ids_ic":1,"signatures":"%s","availability_commitment":%s}`,
		sigStr, string(commitmentJSON),
	)

	msg := &actiontypes.MsgRequestAction{
		Creator:        txCreator,
		ActionType:     actiontypes.ActionTypeCascade.String(),
		Metadata:       metadata,
		Price:          "100000ulume",
		ExpirationTime: fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix()),
	}

	res, err := s.msgServer.RequestAction(s.ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, res.ActionId)

	// Verify commitment stored on-chain.
	action, found := s.keeper.GetActionByID(s.ctx, res.ActionId)
	require.True(t, found)
	require.Equal(t, actiontypes.ActionStatePending, action.State)

	var storedMeta actiontypes.CascadeMetadata
	require.NoError(t, gogoproto.Unmarshal(action.Metadata, &storedMeta))
	require.NotNil(t, storedMeta.AvailabilityCommitment)
	require.Equal(t, root, storedMeta.AvailabilityCommitment.Root)
	require.Equal(t, challengeIndices, storedMeta.AvailabilityCommitment.ChallengeIndices)

	// --- Finalize with chunk proofs ---
	chunkProofs := make([]*actiontypes.ChunkProof, 0, len(challengeIndices))
	for _, idx := range challengeIndices {
		p, pErr := tree.GenerateProof(int(idx))
		require.NoError(t, pErr)
		chunkProofs = append(chunkProofs, protoChunkProof(p))
	}

	ids, err := generateValidCascadeIDs(sigStr, 1, 50)
	require.NoError(t, err)

	finMeta := &actiontypes.CascadeMetadata{
		RqIdsIds:    ids,
		ChunkProofs: chunkProofs,
	}
	finMetaBytes, err := json.Marshal(finMeta)
	require.NoError(t, err)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

	finMsg := &actiontypes.MsgFinalizeAction{
		ActionId:   res.ActionId,
		Creator:    snAccount,
		ActionType: actiontypes.ActionTypeCascade.String(),
		Metadata:   string(finMetaBytes),
	}
	_, err = s.msgServer.FinalizeAction(s.ctx, finMsg)
	require.NoError(t, err)

	// Verify action reached DONE state.
	finalAction, found := s.keeper.GetActionByID(s.ctx, res.ActionId)
	require.True(t, found)
	require.Equal(t, actiontypes.ActionStateDone, finalAction.State)

	// Verify SVC verification passed event was emitted.
	foundPassedEvent := false
	for _, event := range s.ctx.EventManager().Events() {
		if event.Type == actiontypes.EventTypeSVCVerificationPassed {
			foundPassedEvent = true
			break
		}
	}
	require.True(t, foundPassedEvent, "expected SVC verification passed event")
}

// TestLEP5InvalidCommitmentRejected verifies that a Cascade action with an
// invalid AvailabilityCommitment is rejected at registration.
func (s *LEP5IntegrationTestSuite) TestLEP5InvalidCommitmentRejected() {
	t := s.T()
	txCreator := s.testAddrs[0].String()

	sigStr, err := createValidCascadeSignatureString(s.privKeys[0], 1)
	require.NoError(t, err)

	// Build a valid commitment to use as base, then tweak one field.
	numChunks := uint32(8)
	chunks := make([][]byte, numChunks)
	for i := range chunks {
		chunks[i] = []byte(fmt.Sprintf("chunk-%d", i))
	}
	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err)
	root := make([]byte, merkle.HashSize)
	copy(root, tree.Root[:])

	s.Run("wrong commitment type", func() {
		commitment := actiontypes.AvailabilityCommitment{
			CommitmentType:   "wrong-type",
			HashAlgo:         actiontypes.HashAlgo_HASH_ALGO_BLAKE3,
			ChunkSize:        lep5ChunkSize,
			TotalSize:        uint64(numChunks) * uint64(lep5ChunkSize),
			NumChunks:        numChunks,
			Root:             root,
			ChallengeIndices: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
		}
		commitmentJSON, cErr := json.Marshal(&commitment)
		require.NoError(t, cErr)

		metadata := fmt.Sprintf(
			`{"data_hash":"abc123","file_name":"file.txt","rq_ids_ic":1,"signatures":"%s","availability_commitment":%s}`,
			sigStr, string(commitmentJSON),
		)
		msg := &actiontypes.MsgRequestAction{
			Creator:        txCreator,
			ActionType:     actiontypes.ActionTypeCascade.String(),
			Metadata:       metadata,
			Price:          "100000ulume",
			ExpirationTime: fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix()),
		}
		_, err := s.msgServer.RequestAction(s.ctx, msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "commitment_type")
	})
}

// TestLEP5InvalidProofRejected verifies that invalid chunk proofs are rejected
// during finalization.
func (s *LEP5IntegrationTestSuite) TestLEP5InvalidProofRejected() {
	t := s.T()
	txCreator := s.testAddrs[0].String()
	snAccount := s.testAddrs[1].String()

	numChunks := uint32(8)
	chunks := make([][]byte, numChunks)
	for i := uint32(0); i < numChunks; i++ {
		chunks[i] = []byte(fmt.Sprintf("chunk-%d-data", i))
	}

	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err)

	root := make([]byte, merkle.HashSize)
	copy(root, tree.Root[:])

	challengeIndices := []uint32{0, 1, 2, 3, 4, 5, 6, 7}

	sigStr, err := createValidCascadeSignatureString(s.privKeys[0], 1)
	require.NoError(t, err)

	commitment := actiontypes.AvailabilityCommitment{
		CommitmentType:   "lep5/chunk-merkle/v1",
		HashAlgo:         actiontypes.HashAlgo_HASH_ALGO_BLAKE3,
		ChunkSize:        lep5ChunkSize,
		TotalSize:        uint64(numChunks) * uint64(lep5ChunkSize),
		NumChunks:        numChunks,
		Root:             root,
		ChallengeIndices: challengeIndices,
	}
	commitmentJSON, err := json.Marshal(&commitment)
	require.NoError(t, err)

	metadata := fmt.Sprintf(
		`{"data_hash":"abc123","file_name":"file.txt","rq_ids_ic":1,"signatures":"%s","availability_commitment":%s}`,
		sigStr, string(commitmentJSON),
	)

	msg := &actiontypes.MsgRequestAction{
		Creator:        txCreator,
		ActionType:     actiontypes.ActionTypeCascade.String(),
		Metadata:       metadata,
		Price:          "100000ulume",
		ExpirationTime: fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix()),
	}

	res, err := s.msgServer.RequestAction(s.ctx, msg)
	require.NoError(t, err)

	// Build proofs but tamper with one.
	chunkProofs := make([]*actiontypes.ChunkProof, 0, len(challengeIndices))
	for _, idx := range challengeIndices {
		p, pErr := tree.GenerateProof(int(idx))
		require.NoError(t, pErr)
		chunkProofs = append(chunkProofs, protoChunkProof(p))
	}
	// Tamper first proof's leaf hash.
	chunkProofs[0].LeafHash[0] ^= 0xFF

	ids, err := generateValidCascadeIDs(sigStr, 1, 50)
	require.NoError(t, err)

	finMeta := &actiontypes.CascadeMetadata{
		RqIdsIds:    ids,
		ChunkProofs: chunkProofs,
	}
	finMetaBytes, err := json.Marshal(finMeta)
	require.NoError(t, err)

	finMsg := &actiontypes.MsgFinalizeAction{
		ActionId:   res.ActionId,
		Creator:    snAccount,
		ActionType: actiontypes.ActionTypeCascade.String(),
		Metadata:   string(finMetaBytes),
	}
	_, err = s.msgServer.FinalizeAction(s.ctx, finMsg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed verification")
}

func TestLEP5IntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(LEP5IntegrationTestSuite))
}

func protoChunkProof(p *merkle.Proof) *actiontypes.ChunkProof {
	leaf := make([]byte, merkle.HashSize)
	copy(leaf, p.LeafHash[:])

	pathHashes := make([][]byte, 0, len(p.PathHashes))
	for _, h := range p.PathHashes {
		b := make([]byte, merkle.HashSize)
		copy(b, h[:])
		pathHashes = append(pathHashes, b)
	}

	return &actiontypes.ChunkProof{
		ChunkIndex:     p.ChunkIndex,
		LeafHash:       leaf,
		PathHashes:     pathHashes,
		PathDirections: append([]bool(nil), p.PathDirections...),
	}
}

