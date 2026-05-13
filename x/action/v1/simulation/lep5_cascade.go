package simulation

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/merkle"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
)

const lep5ChunkSize = uint32(262144)

// SimulateMsgCascadeWithSVCFlow simulates a full LEP-5 Cascade lifecycle:
// register with AvailabilityCommitment, finalize with valid chunk proofs, verify DONE.
func SimulateMsgCascadeWithSVCFlow(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		params := k.GetParams(ctx)
		challengeCount := keeper.SVCChallengeCount
		minChunks := keeper.SVCMinChunksForChallenge

		// Use enough chunks so SVC is exercised.
		numChunks := minChunks + uint32(r.Intn(int(challengeCount)))
		if numChunks < minChunks {
			numChunks = minChunks
		}

		// Build Merkle tree from random chunks.
		chunks := make([][]byte, numChunks)
		for i := range chunks {
			chunk := make([]byte, 4+r.Intn(60))
			r.Read(chunk)
			chunks[i] = chunk
		}

		tree, err := merkle.BuildTree(chunks)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, "lep5_cascade", fmt.Sprintf("build tree: %v", err)), nil, nil
		}

		root := make([]byte, merkle.HashSize)
		copy(root, tree.Root[:])

		// Generate challenge indices.
		m := challengeCount
		if m > numChunks {
			m = numChunks
		}
		challengeIndices := generateUniqueIndices(r, numChunks, m)

		// Select a funded account and register the action.
		feeAmount := generateRandomFee(r, ctx, params.BaseActionFee.Add(params.FeePerKbyte))
		simAccount := selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak, feeAmount, []string{""})

		sigStr := generateCascadeSignature(simAccount)

		commitment := types.AvailabilityCommitment{
			CommitmentType:   "lep5/chunk-merkle/v1",
			HashAlgo:         types.HashAlgo_HASH_ALGO_BLAKE3,
			ChunkSize:        lep5ChunkSize,
			TotalSize:        uint64(numChunks) * uint64(lep5ChunkSize),
			NumChunks:        numChunks,
			Root:             root,
			ChallengeIndices: challengeIndices,
		}
		commitmentJSON, err := json.Marshal(&commitment)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, "lep5_cascade", fmt.Sprintf("marshal commitment: %v", err)), nil, nil
		}

		dataHash := generateRandomHash(r)
		fileName := generateRandomFileName(r)

		metadata := fmt.Sprintf(
			`{"data_hash":"%s","file_name":"%s","rq_ids_ic":1,"signatures":"%s","availability_commitment":%s}`,
			dataHash, fileName, sigStr, string(commitmentJSON),
		)

		expirationTime := getRandomExpirationTime(ctx, r, params)

		msg := types.NewMsgRequestAction(
			simAccount.Address.String(),
			types.ActionTypeCascade.String(),
			metadata,
			feeAmount.String(),
			strconv.FormatInt(expirationTime, 10),
			"",
		)

		msgServSim := keeper.NewMsgServerImpl(k)
		result, err := msgServSim.RequestAction(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), fmt.Sprintf("register: %v", err)), nil, nil
		}

		// Verify action is pending.
		action, found := k.GetActionByID(ctx, result.ActionId)
		if !found {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "action not found after register"), nil, nil
		}
		if action.State != types.ActionStatePending {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "not in PENDING state"), nil, nil
		}

		// Get a supernode to finalize.
		supernodes, snErr := getRandomActiveSupernodes(r, ctx, 1, ak, k, accs)
		if snErr != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), fmt.Sprintf("no supernodes: %v", snErr)), nil, nil
		}

		// Build chunk proofs for challenged indices.
		chunkProofs := make([]*types.ChunkProof, 0, len(challengeIndices))
		for _, idx := range challengeIndices {
			p, pErr := tree.GenerateProof(int(idx))
			if pErr != nil {
				return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), fmt.Sprintf("gen proof: %v", pErr)), nil, nil
			}
			chunkProofs = append(chunkProofs, simChunkProof(p))
		}

		ids := generateKademliaIDs(1, 50, sigStr)

		finMeta := &types.CascadeMetadata{
			RqIdsIds:    ids,
			ChunkProofs: chunkProofs,
		}
		finMetaBytes, err := json.Marshal(finMeta)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), fmt.Sprintf("marshal finalize: %v", err)), nil, nil
		}

		finMsg := types.NewMsgFinalizeAction(
			supernodes[0].Address.String(),
			result.ActionId,
			types.ActionTypeCascade.String(),
			string(finMetaBytes),
		)

		_, err = msgServSim.FinalizeAction(ctx, finMsg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(finMsg), fmt.Sprintf("finalize: %v", err)), nil, nil
		}

		// Verify DONE state.
		finalAction, found := k.GetActionByID(ctx, result.ActionId)
		if !found || finalAction.State != types.ActionStateDone {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(finMsg), "not in DONE state after finalize"), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "lep5_cascade_svc_flow_success"), nil, nil
	}
}

func generateUniqueIndices(r *rand.Rand, numChunks, m uint32) []uint32 {
	if m > numChunks {
		m = numChunks
	}
	perm := r.Perm(int(numChunks))
	indices := make([]uint32, m)
	for i := uint32(0); i < m; i++ {
		indices[i] = uint32(perm[i])
	}
	return indices
}

func simChunkProof(p *merkle.Proof) *types.ChunkProof {
	leaf := make([]byte, merkle.HashSize)
	copy(leaf, p.LeafHash[:])

	pathHashes := make([][]byte, 0, len(p.PathHashes))
	for _, h := range p.PathHashes {
		b := make([]byte, merkle.HashSize)
		copy(b, h[:])
		pathHashes = append(pathHashes, b)
	}

	return &types.ChunkProof{
		ChunkIndex:     p.ChunkIndex,
		LeafHash:       leaf,
		PathHashes:     pathHashes,
		PathDirections: append([]bool(nil), p.PathDirections...),
	}
}
