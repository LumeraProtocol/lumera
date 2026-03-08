package action_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/tests/ibctesting"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/merkle"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const systemChunkSize = uint32(262144)

type actionSystemSuite struct {
	app    *app.App
	sdkCtx sdk.Context
	ctx    context.Context
}

func setupActionSystemSuite(t *testing.T) *actionSystemSuite {
	os.Setenv("SYSTEM_TESTS", "true")
	t.Cleanup(func() { os.Unsetenv("SYSTEM_TESTS") })

	s := &actionSystemSuite{}
	coord := ibctesting.NewCoordinator(t, 1)
	chain := coord.GetChain(ibctesting.GetChainID(1))

	a := chain.App.(*app.App)
	s.app = a
	s.sdkCtx = chain.GetContext()
	s.ctx = sdk.WrapSDKContext(s.sdkCtx)

	// Create and bond a validator for supernode registration.
	valPrivKey := secp256k1.GenPrivKey()
	valPubKey := valPrivKey.PubKey()
	valAddr := sdk.ValAddress(valPubKey.Address().Bytes())

	validator, err := stakingtypes.NewValidator(valAddr.String(), valPubKey, stakingtypes.Description{})
	require.NoError(t, err)
	validator.Status = stakingtypes.Bonded
	validator.Tokens = sdkmath.NewInt(1_000_000)
	a.StakingKeeper.SetValidator(s.sdkCtx, validator)

	// Set action module params with SVC settings.
	params := actiontypes.DefaultParams()
	params.ExpirationDuration = time.Minute
	require.NoError(t, a.ActionKeeper.SetParams(s.sdkCtx, params))

	return s
}

// TestLEP5CascadeSystemFlow exercises the full LEP-5 register-with-commitment
// and finalize-with-proofs flow on a full chain started via ibctesting.Coordinator.
func TestLEP5CascadeSystemFlow(t *testing.T) {
	s := setupActionSystemSuite(t)

	// --- Accounts ---
	creatorPriv := secp256k1.GenPrivKey()
	creatorAddr := sdk.AccAddress(creatorPriv.PubKey().Address())

	snPriv := secp256k1.GenPrivKey()
	snAddr := sdk.AccAddress(snPriv.PubKey().Address())
	snValAddr := sdk.ValAddress(snAddr)

	// Fund creator and register account with public key.
	initCoins := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1_000_000))
	require.NoError(t, s.app.BankKeeper.MintCoins(s.sdkCtx, actiontypes.ModuleName, initCoins))
	require.NoError(t, s.app.BankKeeper.SendCoinsFromModuleToAccount(s.sdkCtx, actiontypes.ModuleName, creatorAddr, initCoins))

	creatorAcc := s.app.AuthKeeper.NewAccountWithAddress(s.sdkCtx, creatorAddr)
	baseAcc := creatorAcc.(*authtypes.BaseAccount)
	require.NoError(t, baseAcc.SetPubKey(creatorPriv.PubKey()))
	s.app.AuthKeeper.SetAccount(s.sdkCtx, baseAcc)

	// Register a bonded validator and supernode for the SN account.
	val, err := stakingtypes.NewValidator(snValAddr.String(), snPriv.PubKey(), stakingtypes.Description{})
	require.NoError(t, err)
	val.Status = stakingtypes.Bonded
	val.Tokens = sdkmath.NewInt(1_000_000)
	s.app.StakingKeeper.SetValidator(s.sdkCtx, val)

	sn := sntypes.SuperNode{
		ValidatorAddress: snValAddr.String(),
		SupernodeAccount: snAddr.String(),
		Note:             "1.0.0",
		States:           []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}},
		PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "10.0.0.1"}},
		P2PPort:          "4001",
	}
	require.NoError(t, s.app.SupernodeKeeper.SetSuperNode(s.sdkCtx, sn))

	// --- Build Merkle tree ---
	numChunks := uint32(8)
	chunks := make([][]byte, numChunks)
	for i := range chunks {
		chunks[i] = []byte(fmt.Sprintf("sys-chunk-%d", i))
	}

	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err)

	root := make([]byte, merkle.HashSize)
	copy(root, tree.Root[:])

	challengeIndices := []uint32{0, 1, 2, 3, 4, 5, 6, 7}

	// --- Register Cascade with commitment ---
	commitment := actiontypes.AvailabilityCommitment{
		CommitmentType:   "lep5/chunk-merkle/v1",
		HashAlgo:         actiontypes.HashAlgo_HASH_ALGO_BLAKE3,
		ChunkSize:        systemChunkSize,
		TotalSize:        uint64(numChunks) * uint64(systemChunkSize),
		NumChunks:        numChunks,
		Root:             root,
		ChallengeIndices: challengeIndices,
	}
	commitmentJSON, err := json.Marshal(&commitment)
	require.NoError(t, err)

	// Build a minimal valid signature for the registration metadata.
	sigData := "c2lnLWRhdGE=" // base64("sig-data")
	sig, err := creatorPriv.Sign([]byte(sigData))
	require.NoError(t, err)

	sigStr := fmt.Sprintf("%s.%s", sigData, base64.StdEncoding.EncodeToString(sig))

	metadata := fmt.Sprintf(
		`{"data_hash":"syshash","file_name":"sys.bin","rq_ids_ic":1,"signatures":"%s","availability_commitment":%s}`,
		sigStr, string(commitmentJSON),
	)

	msgServer := keeper.NewMsgServerImpl(s.app.ActionKeeper)
	res, err := msgServer.RequestAction(s.sdkCtx, &actiontypes.MsgRequestAction{
		Creator:        creatorAddr.String(),
		ActionType:     actiontypes.ActionTypeCascade.String(),
		Metadata:       metadata,
		Price:          "100000ulume",
		ExpirationTime: fmt.Sprintf("%d", s.sdkCtx.BlockTime().Add(10*time.Minute).Unix()),
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.ActionId)

	// Verify pending state and stored commitment.
	action, found := s.app.ActionKeeper.GetActionByID(s.sdkCtx, res.ActionId)
	require.True(t, found)
	require.Equal(t, actiontypes.ActionStatePending, action.State)

	var storedMeta actiontypes.CascadeMetadata
	require.NoError(t, gogoproto.Unmarshal(action.Metadata, &storedMeta))
	require.NotNil(t, storedMeta.AvailabilityCommitment)
	require.Equal(t, root, storedMeta.AvailabilityCommitment.Root)

	// --- Finalize with valid chunk proofs ---
	chunkProofs := make([]*actiontypes.ChunkProof, 0, len(challengeIndices))
	for _, idx := range challengeIndices {
		p, pErr := tree.GenerateProof(int(idx))
		require.NoError(t, pErr)
		chunkProofs = append(chunkProofs, toProtoChunkProof(p))
	}

	// Generate Kademlia IDs for CASCADE finalization.
	ids := make([]string, 50)
	for i := range ids {
		id, kErr := keeper.CreateKademliaID(sigStr, uint64(1+i))
		require.NoError(t, kErr)
		ids[i] = id
	}

	finMeta := &actiontypes.CascadeMetadata{
		RqIdsIds:    ids,
		ChunkProofs: chunkProofs,
	}
	finMetaBytes, err := json.Marshal(finMeta)
	require.NoError(t, err)

	s.sdkCtx = s.sdkCtx.WithEventManager(sdk.NewEventManager())

	_, err = msgServer.FinalizeAction(s.sdkCtx, &actiontypes.MsgFinalizeAction{
		ActionId:   res.ActionId,
		Creator:    snAddr.String(),
		ActionType: actiontypes.ActionTypeCascade.String(),
		Metadata:   string(finMetaBytes),
	})
	require.NoError(t, err)

	finalAction, found := s.app.ActionKeeper.GetActionByID(s.sdkCtx, res.ActionId)
	require.True(t, found)
	require.Equal(t, actiontypes.ActionStateDone, finalAction.State)

	// Verify SVC verification passed event.
	foundPassedEvent := false
	for _, ev := range s.sdkCtx.EventManager().Events() {
		if ev.Type == actiontypes.EventTypeSVCVerificationPassed {
			foundPassedEvent = true
			break
		}
	}
	require.True(t, foundPassedEvent, "expected SVC verification passed event")
}

func toProtoChunkProof(p *merkle.Proof) *actiontypes.ChunkProof {
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
