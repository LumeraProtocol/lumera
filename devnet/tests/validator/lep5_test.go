package validator

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/merkle"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/LumeraProtocol/sdk-go/blockchain"
	"github.com/LumeraProtocol/sdk-go/cascade"
	sdkcrypto "github.com/LumeraProtocol/sdk-go/pkg/crypto"
	sdktypes "github.com/LumeraProtocol/sdk-go/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	lep5ChunkSize           = uint32(262144)
	lep5CommitmentType      = "lep5/chunk-merkle/v1"
	lep5DefaultLumeraGRPC   = "localhost:9090"
	lep5FinalizeMaxAttempts = 8
	lep5TopSupernodesLimit  = int32(25)
)

var lep5CommitmentHashAlgo = actiontypes.HashAlgo_HASH_ALGO_BLAKE3

func TestLEP5CascadeAvailabilityCommitment(t *testing.T) {
	runCascadeCommitmentTest(t, 8*uint64(lep5ChunkSize), lep5ChunkSize)
}

// TestLEP5VariableChunkSizes exercises the availability-commitment flow with
// non-default chunk sizes. Each subtest creates a file of a specific size,
// chunks it with the given chunk size, and runs the full register → finalize →
// DONE cycle.
func TestLEP5VariableChunkSizes(t *testing.T) {
	cases := []struct {
		name      string
		fileSize  uint64
		chunkSize uint32
	}{
		{
			name:      "SmallFile_5KB_ChunkSize_1024",
			fileSize:  5 * 1024, // 5 KB → 5 chunks of 1024
			chunkSize: 1024,
		},
		{
			name:      "MediumFile_500KB_ChunkSize_131072",
			fileSize:  500 * 1024, // 500 KB → 4 chunks (3×131072 + 1×24576)
			chunkSize: 131072,
		},
		{
			name:      "TineFile_4B_ChunkSize_1B",
			fileSize:  4, // 4 B → 4 chunks of 1 B
			chunkSize: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runCascadeCommitmentTest(t, tc.fileSize, tc.chunkSize)
		})
	}
}

// runCascadeCommitmentTest is the parameterised core of the LEP-5 cascade
// availability-commitment E2E test. It creates a file of fileSize bytes, splits
// it into chunks of chunkSize bytes (last chunk may be smaller), builds a
// Merkle tree, registers a Cascade action with the commitment, finalises it
// with valid proofs, and asserts the action reaches DONE with correct metadata.
func runCascadeCommitmentTest(t *testing.T, fileSize uint64, chunkSize uint32) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	rpcAddr := resolveLumeraRPC()
	if resolvedRPC, err := lep5ResolveReachableRPC(ctx); err == nil {
		rpcAddr = resolvedRPC
	}

	grpcAddr := lep5ResolveReachableGRPC(lep5NormalizeGRPCAddr(getenv("LUMERA_GRPC_ADDR", lep5DefaultLumeraGRPC)))
	chainID := getenv("LUMERA_CHAIN_ID", defaultLumeraChainID)
	denom := getenv("LUMERA_DENOM", defaultLumeraDenom)
	moniker := detectValidatorMoniker()

	if _, _, err := lep5NextFinalizeSeed(ctx, rpcAddr); err != nil {
		t.Skipf("skipping LEP-5 devnet E2E: Lumera RPC not reachable at %s (%v)", rpcAddr, err)
	}

	kr, keyName, supernodeAddr, err := lep5LoadSignerKey(ctx, chainID, moniker, rpcAddr, grpcAddr)
	if err != nil {
		t.Skipf("skipping LEP-5 devnet E2E: signer key unavailable (%v)", err)
	}

	bc, err := blockchain.New(ctx, blockchain.Config{
		ChainID:        chainID,
		GRPCAddr:       grpcAddr,
		RPCEndpoint:    rpcAddr,
		AccountHRP:     "lumera",
		FeeDenom:       denom,
		GasPrice:       sdkmath.LegacyNewDecWithPrec(25, 3),
		Timeout:        30 * time.Second,
		MaxRecvMsgSize: 10 * 1024 * 1024,
		MaxSendMsgSize: 10 * 1024 * 1024,
		InsecureGRPC:   true,
	}, kr, keyName)
	require.NoError(t, err, "create lumera blockchain client")
	defer bc.Close()

	cascadeClient, err := cascade.New(ctx, cascade.Config{
		ChainID:  chainID,
		GRPCAddr: grpcAddr,
		Address:  supernodeAddr,
		KeyName:  keyName,
		Timeout:  30 * time.Second,
	}, kr)
	require.NoError(t, err, "create cascade client")
	defer cascadeClient.Close()

	filePath, chunks, totalSize := lep5CreateTestFileWithSize(t, fileSize, chunkSize)
	numChunks := uint32(len(chunks))
	t.Logf("--- Test file: %d bytes, chunkSize=%d, numChunks=%d ---", totalSize, chunkSize, numChunks)

	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err, "build merkle tree")

	// Client picks challenge indices at registration time.
	// Default SVC challenge count is 8, capped to the actual number of chunks.
	challengeCount := uint32(8)
	if challengeCount > numChunks {
		challengeCount = numChunks
	}
	challengeIndices := make([]uint32, 0, challengeCount)
	for i := uint32(0); i < challengeCount; i++ {
		challengeIndices = append(challengeIndices, i)
	}

	lep5PrintMerkleTreeDiagram(t, tree, challengeIndices)

	commitment := &actiontypes.AvailabilityCommitment{
		CommitmentType:   lep5CommitmentType,
		HashAlgo:         lep5CommitmentHashAlgo,
		ChunkSize:        chunkSize,
		TotalSize:        totalSize,
		NumChunks:        numChunks,
		Root:             append([]byte(nil), tree.Root[:]...),
		ChallengeIndices: challengeIndices,
	}

	requestMsg, _, err := cascadeClient.CreateRequestActionMessage(ctx, supernodeAddr, filePath, &cascade.UploadOptions{Public: true})
	require.NoError(t, err, "build request action message")

	var requestMeta actiontypes.CascadeMetadata
	require.NoError(t, json.Unmarshal([]byte(requestMsg.Metadata), &requestMeta), "unmarshal request metadata")
	requestMeta.AvailabilityCommitment = commitment

	t.Log("=== REQUEST METADATA (before submit) ===")
	lep5PrintCascadeMetadata(t, &requestMeta)

	requestMetaJSON, err := json.Marshal(&requestMeta)
	require.NoError(t, err, "marshal request metadata with availability commitment")

	fileSizeKbs := lep5ResolveFileSizeKBs(filePath, requestMsg.FileSizeKbs)

	t.Log("--- Submitting RequestAction tx (BroadcastMode=SYNC) ---")
	t.Log("    The SDK broadcasts with BROADCAST_MODE_SYNC: the node validates the tx in the mempool and returns immediately.")
	t.Log("    Then WaitForTxInclusion subscribes via CometBFT websocket (or polls gRPC) until the tx is mined into a block.")
	t.Logf("    Devnet block time is ~1-5s, so inclusion is typically fast.")

	reqStart := time.Now()
	requestRes, err := bc.RequestActionTx(
		ctx,
		supernodeAddr,
		actiontypes.ActionTypeCascade,
		string(requestMetaJSON),
		requestMsg.Price,
		requestMsg.ExpirationTime,
		fileSizeKbs,
		"lep5-e2e-register",
	)
	require.NoError(t, err, "submit request action tx")
	require.NotEmpty(t, requestRes.ActionID, "request action id")
	t.Logf("--- RequestAction tx included in block (height=%d, txHash=%s, took %s) ---", requestRes.Height, requestRes.TxHash, time.Since(reqStart))
	t.Logf("    Action ID assigned by chain: %s", requestRes.ActionID)

	t.Log("--- Waiting for action to reach PENDING state (polling every 2s via gRPC GetAction) ---")
	pendingStart := time.Now()
	_, err = bc.Action.WaitForState(ctx, requestRes.ActionID, sdktypes.ActionStatePending, 2*time.Second)
	require.NoError(t, err, "wait for action pending")
	t.Logf("--- Action reached PENDING state (took %s) ---", time.Since(pendingStart))

	t.Log("--- Querying registered action from chain to verify on-chain metadata ---")
	queryClient := actiontypes.NewQueryClient(bc.GRPCConn())
	registered, err := queryClient.GetAction(ctx, &actiontypes.QueryGetActionRequest{ActionID: requestRes.ActionID})
	require.NoError(t, err, "query registered action")
	require.NotNil(t, registered.Action)

	var registeredMeta actiontypes.CascadeMetadata
	require.NoError(t, gogoproto.Unmarshal(registered.Action.Metadata, &registeredMeta), "decode registered metadata")

	t.Log("=== REGISTERED METADATA (from chain, after request) ===")
	t.Logf("  Action.State: %s", registered.Action.State)
	lep5PrintCascadeMetadata(t, &registeredMeta)

	require.NotNil(t, registeredMeta.AvailabilityCommitment, "stored availability commitment")
	require.Equal(t, commitment, registeredMeta.AvailabilityCommitment, "stored commitment must match request")

	proofCount := uint32(len(challengeIndices))

	t.Log("--- Building finalization payload (rqIDs + chunk merkle proofs) ---")
	rqIDs := make([]string, 0, registeredMeta.RqIdsMax)
	for i := registeredMeta.RqIdsIc; i < registeredMeta.RqIdsIc+registeredMeta.RqIdsMax; i++ {
		id, idErr := keeper.CreateKademliaID(registeredMeta.Signatures, i)
		require.NoError(t, idErr, "create rq id %d", i)
		rqIDs = append(rqIDs, id)
	}

	// Generate proofs for the challenge indices stored in the commitment.
	proofs := make([]*actiontypes.ChunkProof, 0, len(challengeIndices))
	for _, idx := range challengeIndices {
		proof, pErr := tree.GenerateProof(int(idx))
		require.NoError(t, pErr, "generate proof for chunk %d", idx)
		proofs = append(proofs, lep5ToChunkProof(proof))
	}

	t.Log("--- ChunkProofs prepared for finalization ---")
	for i, p := range proofs {
		t.Logf("    Proof [%d]: ChunkIndex=%d, LeafHash=%x, PathLength=%d", i, p.ChunkIndex, p.LeafHash, len(p.PathHashes))
	}

	finalizeMeta := &actiontypes.CascadeMetadata{
		RqIdsIds:    rqIDs,
		ChunkProofs: proofs,
	}
	finalizeJSON, mErr := json.Marshal(finalizeMeta)
	require.NoError(t, mErr, "marshal finalize metadata")

	t.Logf("--- Finalization payload ready: %d rqIDs, %d chunk proofs, challengeIndices=%v ---", len(rqIDs), len(proofs), challengeIndices)

	var lastTxHash string
	var finalizeSucceeded bool

	t.Logf("--- Submitting FinalizeAction tx (up to %d attempts, BroadcastMode=SYNC + WaitForTxInclusion) ---", lep5FinalizeMaxAttempts)
	for attempt := 1; attempt <= lep5FinalizeMaxAttempts; attempt++ {
		t.Logf("    [attempt %d/%d] Broadcasting FinalizeAction tx...", attempt, lep5FinalizeMaxAttempts)
		finalizeStart := time.Now()
		finalizeRes, fErr := bc.FinalizeActionTx(
			ctx,
			supernodeAddr,
			requestRes.ActionID,
			actiontypes.ActionTypeCascade,
			string(finalizeJSON),
			fmt.Sprintf("lep5-e2e-finalize-%d", attempt),
		)
		require.NoError(t, fErr, "submit finalize tx attempt %d", attempt)
		lastTxHash = finalizeRes.TxHash
		t.Logf("    [attempt %d/%d] FinalizeAction tx included in block (txHash=%s, took %s)", attempt, lep5FinalizeMaxAttempts, finalizeRes.TxHash, time.Since(finalizeStart))

		t.Logf("    [attempt %d/%d] Re-querying tx to verify on-chain result code...", attempt, lep5FinalizeMaxAttempts)
		txResp, txErr := bc.GetTx(ctx, finalizeRes.TxHash)
		require.NoError(t, txErr, "query finalize tx attempt %d", attempt)
		require.NotNil(t, txResp.TxResponse, "finalize tx response attempt %d", attempt)

		if txResp.TxResponse.Code == 0 {
			t.Logf("    [attempt %d/%d] FinalizeAction tx succeeded (code=0)", attempt, lep5FinalizeMaxAttempts)
			finalizeSucceeded = true
			break
		}

		t.Logf(
			"    [attempt %d/%d] FinalizeAction tx failed on-chain (code=%d, log=%s), sleeping 2s before retry...",
			attempt, lep5FinalizeMaxAttempts,
			txResp.TxResponse.Code,
			txResp.TxResponse.RawLog,
		)
		time.Sleep(2 * time.Second)
	}

	require.True(t, finalizeSucceeded, "finalize tx did not succeed after retries (last tx=%s)", lastTxHash)

	t.Log("--- Waiting for action to reach DONE state (polling every 2s via gRPC GetAction) ---")
	doneStart := time.Now()
	_, err = bc.Action.WaitForState(ctx, requestRes.ActionID, sdktypes.ActionStateDone, 2*time.Second)
	require.NoError(t, err, "wait for action done")
	t.Logf("--- Action reached DONE state (took %s) ---", time.Since(doneStart))

	t.Log("--- Final verification: querying action from chain to confirm DONE state and metadata ---")
	finalAction, err := queryClient.GetAction(ctx, &actiontypes.QueryGetActionRequest{ActionID: requestRes.ActionID})
	require.NoError(t, err, "query finalized action")
	require.NotNil(t, finalAction.Action)
	require.Equal(t, actiontypes.ActionStateDone, finalAction.Action.State, "action state must be DONE")

	var finalMeta actiontypes.CascadeMetadata
	require.NoError(t, gogoproto.Unmarshal(finalAction.Action.Metadata, &finalMeta), "decode finalized metadata")

	t.Log("=== FINALIZED METADATA (from chain, after finalize) ===")
	t.Logf("  Action.State: %s", finalAction.Action.State)
	lep5PrintCascadeMetadata(t, &finalMeta)

	require.NotNil(t, finalMeta.AvailabilityCommitment, "finalized metadata commitment must exist")
	require.Equal(t, commitment.Root, finalMeta.AvailabilityCommitment.Root, "commitment root must be preserved")
	require.Equal(t, chunkSize, finalMeta.AvailabilityCommitment.ChunkSize, "chunk size must be preserved")
	require.Equal(t, numChunks, finalMeta.AvailabilityCommitment.NumChunks, "num chunks must be preserved")
	require.Len(t, finalMeta.ChunkProofs, int(proofCount), "chunk proof count")
}

// TestLEP5CascadeAvailabilityCommitmentFailure registers a Cascade action with
// a valid AvailabilityCommitment, then attempts to finalize it with corrupt
// chunk proofs (flipped leaf hashes). The test asserts that the finalization
// transaction is rejected on-chain and the action state remains PENDING.
func TestLEP5CascadeAvailabilityCommitmentFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	rpcAddr := resolveLumeraRPC()
	if resolvedRPC, err := lep5ResolveReachableRPC(ctx); err == nil {
		rpcAddr = resolvedRPC
	}

	grpcAddr := lep5ResolveReachableGRPC(lep5NormalizeGRPCAddr(getenv("LUMERA_GRPC_ADDR", lep5DefaultLumeraGRPC)))
	chainID := getenv("LUMERA_CHAIN_ID", defaultLumeraChainID)
	denom := getenv("LUMERA_DENOM", defaultLumeraDenom)
	moniker := detectValidatorMoniker()

	if _, _, err := lep5NextFinalizeSeed(ctx, rpcAddr); err != nil {
		t.Skipf("skipping LEP-5 devnet E2E: Lumera RPC not reachable at %s (%v)", rpcAddr, err)
	}

	kr, keyName, supernodeAddr, err := lep5LoadSignerKey(ctx, chainID, moniker, rpcAddr, grpcAddr)
	if err != nil {
		t.Skipf("skipping LEP-5 devnet E2E: signer key unavailable (%v)", err)
	}

	bc, err := blockchain.New(ctx, blockchain.Config{
		ChainID:        chainID,
		GRPCAddr:       grpcAddr,
		RPCEndpoint:    rpcAddr,
		AccountHRP:     "lumera",
		FeeDenom:       denom,
		GasPrice:       sdkmath.LegacyNewDecWithPrec(25, 3),
		Timeout:        30 * time.Second,
		MaxRecvMsgSize: 10 * 1024 * 1024,
		MaxSendMsgSize: 10 * 1024 * 1024,
		InsecureGRPC:   true,
	}, kr, keyName)
	require.NoError(t, err, "create lumera blockchain client")
	defer bc.Close()

	cascadeClient, err := cascade.New(ctx, cascade.Config{
		ChainID:  chainID,
		GRPCAddr: grpcAddr,
		Address:  supernodeAddr,
		KeyName:  keyName,
		Timeout:  30 * time.Second,
	}, kr)
	require.NoError(t, err, "create cascade client")
	defer cascadeClient.Close()

	// --- Register action (identical to the success test) ---
	filePath, chunks, totalSize := lep5CreateTestFile(t, 8)
	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err, "build merkle tree")

	challengeCount := uint32(8)
	if challengeCount > uint32(len(chunks)) {
		challengeCount = uint32(len(chunks))
	}
	challengeIndices := make([]uint32, 0, challengeCount)
	for i := uint32(0); i < challengeCount; i++ {
		challengeIndices = append(challengeIndices, i)
	}

	lep5PrintMerkleTreeDiagram(t, tree, challengeIndices)

	commitment := &actiontypes.AvailabilityCommitment{
		CommitmentType:   lep5CommitmentType,
		HashAlgo:         lep5CommitmentHashAlgo,
		ChunkSize:        lep5ChunkSize,
		TotalSize:        totalSize,
		NumChunks:        uint32(len(chunks)),
		Root:             append([]byte(nil), tree.Root[:]...),
		ChallengeIndices: challengeIndices,
	}

	requestMsg, _, err := cascadeClient.CreateRequestActionMessage(ctx, supernodeAddr, filePath, &cascade.UploadOptions{Public: true})
	require.NoError(t, err, "build request action message")

	var requestMeta actiontypes.CascadeMetadata
	require.NoError(t, json.Unmarshal([]byte(requestMsg.Metadata), &requestMeta), "unmarshal request metadata")
	requestMeta.AvailabilityCommitment = commitment

	t.Log("=== REQUEST METADATA (before submit) ===")
	lep5PrintCascadeMetadata(t, &requestMeta)

	requestMetaJSON, err := json.Marshal(&requestMeta)
	require.NoError(t, err, "marshal request metadata with availability commitment")

	fileSizeKbs := lep5ResolveFileSizeKBs(filePath, requestMsg.FileSizeKbs)

	t.Log("--- [FAILURE TEST] Submitting RequestAction tx ---")
	requestRes, err := bc.RequestActionTx(
		ctx,
		supernodeAddr,
		actiontypes.ActionTypeCascade,
		string(requestMetaJSON),
		requestMsg.Price,
		requestMsg.ExpirationTime,
		fileSizeKbs,
		"lep5-e2e-register-bad-finalize",
	)
	require.NoError(t, err, "submit request action tx")
	require.NotEmpty(t, requestRes.ActionID, "request action id")
	t.Logf("--- [FAILURE TEST] RequestAction included (height=%d, txHash=%s, actionID=%s) ---", requestRes.Height, requestRes.TxHash, requestRes.ActionID)

	t.Log("--- [FAILURE TEST] Waiting for action PENDING state ---")
	_, err = bc.Action.WaitForState(ctx, requestRes.ActionID, sdktypes.ActionStatePending, 2*time.Second)
	require.NoError(t, err, "wait for action pending")
	t.Log("--- [FAILURE TEST] Action reached PENDING state ---")

	t.Log("--- [FAILURE TEST] Querying registered action from chain to verify on-chain metadata ---")
	queryClient := actiontypes.NewQueryClient(bc.GRPCConn())
	registered, err := queryClient.GetAction(ctx, &actiontypes.QueryGetActionRequest{ActionID: requestRes.ActionID})
	require.NoError(t, err, "query registered action")
	require.NotNil(t, registered.Action)

	var registeredMeta actiontypes.CascadeMetadata
	require.NoError(t, gogoproto.Unmarshal(registered.Action.Metadata, &registeredMeta), "decode registered metadata")

	t.Log("=== REGISTERED METADATA (from chain, after request) ===")
	t.Logf("  Action.State: %s", registered.Action.State)
	lep5PrintCascadeMetadata(t, &registeredMeta)

	// --- Build finalization payload with CORRUPT proofs ---
	rqIDs := make([]string, 0, registeredMeta.RqIdsMax)
	for i := registeredMeta.RqIdsIc; i < registeredMeta.RqIdsIc+registeredMeta.RqIdsMax; i++ {
		id, idErr := keeper.CreateKademliaID(registeredMeta.Signatures, i)
		require.NoError(t, idErr, "create rq id %d", i)
		rqIDs = append(rqIDs, id)
	}

	// Generate valid proofs then corrupt the leaf hashes by flipping all bytes.
	proofs := make([]*actiontypes.ChunkProof, 0, len(challengeIndices))
	for _, idx := range challengeIndices {
		proof, pErr := tree.GenerateProof(int(idx))
		require.NoError(t, pErr, "generate proof for chunk %d", idx)
		cp := lep5ToChunkProof(proof)
		// Corrupt the leaf hash: flip every byte so merkle verification fails.
		for j := range cp.LeafHash {
			cp.LeafHash[j] ^= 0xFF
		}
		proofs = append(proofs, cp)
	}

	t.Log("--- ChunkProofs prepared for finalization (CORRUPT) ---")
	for i, p := range proofs {
		t.Logf("    Proof [%d]: ChunkIndex=%d, LeafHash=%x, PathLength=%d", i, p.ChunkIndex, p.LeafHash, len(p.PathHashes))
	}

	finalizeMeta := &actiontypes.CascadeMetadata{
		RqIdsIds:    rqIDs,
		ChunkProofs: proofs,
	}
	finalizeJSON, mErr := json.Marshal(finalizeMeta)
	require.NoError(t, mErr, "marshal corrupt finalize metadata")

	t.Logf("--- [FAILURE TEST] Finalization payload ready: %d rqIDs, %d CORRUPT chunk proofs ---", len(rqIDs), len(proofs))

	// --- Submit bad finalization and expect on-chain rejection ---
	t.Log("--- [FAILURE TEST] Submitting FinalizeAction tx with corrupt proofs ---")
	finalizeRes, fErr := bc.FinalizeActionTx(
		ctx,
		supernodeAddr,
		requestRes.ActionID,
		actiontypes.ActionTypeCascade,
		string(finalizeJSON),
		"lep5-e2e-finalize-bad",
	)

	if fErr != nil {
		// Tx was rejected at broadcast/CheckTx level – this is an acceptable failure path.
		t.Logf("--- [FAILURE TEST] FinalizeAction tx rejected at broadcast level: %v ---", fErr)
	} else {
		// Tx was included in a block – verify the on-chain result code is non-zero.
		require.NotEmpty(t, finalizeRes.TxHash, "finalize tx hash must not be empty")
		t.Logf("--- [FAILURE TEST] FinalizeAction tx included (txHash=%s), verifying on-chain code ---", finalizeRes.TxHash)

		txResp, txErr := bc.GetTx(ctx, finalizeRes.TxHash)
		require.NoError(t, txErr, "query finalize tx")
		require.NotNil(t, txResp.TxResponse, "finalize tx response")
		require.NotEqual(t, uint32(0), txResp.TxResponse.Code,
			"corrupt finalize tx must fail on-chain (code=%d, log=%s)",
			txResp.TxResponse.Code, txResp.TxResponse.RawLog)
		t.Logf("--- [FAILURE TEST] FinalizeAction tx failed on-chain as expected (code=%d, log=%s) ---",
			txResp.TxResponse.Code, txResp.TxResponse.RawLog)
	}

	// --- Verify the action is still PENDING (not DONE) ---
	t.Log("--- [FAILURE TEST] Querying action state to confirm it remains PENDING ---")
	actionResp, err := queryClient.GetAction(ctx, &actiontypes.QueryGetActionRequest{ActionID: requestRes.ActionID})
	require.NoError(t, err, "query action after failed finalize")
	require.NotNil(t, actionResp.Action)
	require.Equal(t, actiontypes.ActionStatePending, actionResp.Action.State,
		"action state must remain PENDING after failed finalization, got %s", actionResp.Action.State)
	t.Logf("--- [FAILURE TEST] Action %s confirmed in PENDING state (corrupt finalization correctly rejected) ---", requestRes.ActionID)

	var finalMeta actiontypes.CascadeMetadata
	require.NoError(t, gogoproto.Unmarshal(actionResp.Action.Metadata, &finalMeta), "decode finalized metadata")

	t.Log("=== FINALIZED METADATA (from chain, after finalize) ===")
	t.Logf("  Action.State: %s", actionResp.Action.State)
	lep5PrintCascadeMetadata(t, &finalMeta)
}

func lep5ResolveMnemonicPath(chainID, moniker string) (string, bool) {
	if fromEnv := strings.TrimSpace(os.Getenv("LUMERA_SUPERNODE_MNEMONIC_FILE")); fromEnv != "" {
		if _, err := os.Stat(fromEnv); err == nil {
			return fromEnv, true
		}
		return "", false
	}

	if moniker == "" {
		moniker = "supernova_validator_1"
	}

	candidates := []string{
		fmt.Sprintf("/shared/status/%s/sn_mnemonic", moniker),
		fmt.Sprintf("/tmp/%s/shared/status/%s/sn_mnemonic", chainID, moniker),
		fmt.Sprintf("/tmp/lumera-devnet-1/shared/status/%s/sn_mnemonic", moniker),
		fmt.Sprintf("/tmp/lumera-devnet/shared/status/%s/sn_mnemonic", moniker),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}

	return "", false
}

func lep5LoadSignerKey(ctx context.Context, chainID, moniker, rpcAddr, grpcAddr string) (keyring.Keyring, string, string, error) {
	activeSupernodes, err := lep5QueryActiveSupernodeAccounts(ctx, rpcAddr, grpcAddr)
	if err != nil {
		return nil, "", "", fmt.Errorf("query active supernodes: %w", err)
	}

	if kr, keyName, addr, ok := lep5LoadSignerFromMnemonicCandidates(chainID, moniker, activeSupernodes); ok {
		return kr, keyName, addr, nil
	}

	backend := getenv("LUMERA_KEYRING_BACKEND", "test")
	app := getenv("LUMERA_KEYRING_APP", "lumera")

	for _, home := range lep5KeyringHomeCandidates(chainID) {
		kr, openErr := sdkcrypto.NewKeyring(sdkcrypto.KeyringParams{
			AppName: app,
			Backend: backend,
			Dir:     home,
			Input:   strings.NewReader(""),
		})
		if openErr != nil {
			continue
		}

		for _, keyName := range lep5SignerKeyNameCandidates(moniker) {
			addr, addrErr := sdkcrypto.AddressFromKey(kr, keyName, "lumera")
			if addrErr != nil {
				continue
			}
			if _, ok := activeSupernodes[addr]; ok {
				return kr, keyName, addr, nil
			}
		}

		records, listErr := kr.List()
		if listErr != nil {
			continue
		}
		for _, rec := range records {
			accAddr, addrErr := rec.GetAddress()
			if addrErr != nil {
				continue
			}
			addr := accAddr.String()
			if _, ok := activeSupernodes[addr]; ok {
				return kr, rec.Name, addr, nil
			}
		}
	}

	accounts := make([]string, 0, len(activeSupernodes))
	for addr := range activeSupernodes {
		accounts = append(accounts, addr)
	}
	sort.Strings(accounts)
	if len(accounts) > 5 {
		accounts = accounts[:5]
	}

	return nil, "", "", fmt.Errorf("no local key matched active supernode accounts; sample active accounts=%v", accounts)
}

func lep5QueryActiveSupernodeAccounts(ctx context.Context, rpcAddr, grpcAddr string) (map[string]struct{}, error) {
	seedHeight, _, err := lep5NextFinalizeSeed(ctx, rpcAddr)
	if err != nil {
		return nil, err
	}
	if seedHeight == 0 {
		return nil, fmt.Errorf("invalid block height from rpc")
	}

	queryHeight := int32(seedHeight - 1)
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		lep5NormalizeGRPCAddr(grpcAddr),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial grpc %s: %w", grpcAddr, err)
	}
	defer conn.Close()

	queryCtx, queryCancel := context.WithTimeout(ctx, 10*time.Second)
	defer queryCancel()

	client := sntypes.NewQueryClient(conn)
	resp, err := client.GetTopSuperNodesForBlock(queryCtx, &sntypes.QueryGetTopSuperNodesForBlockRequest{
		BlockHeight: queryHeight,
		Limit:       lep5TopSupernodesLimit,
		State:       sntypes.SuperNodeStateActive.String(),
	})
	if err != nil {
		return nil, err
	}

	active := make(map[string]struct{}, len(resp.Supernodes))
	for _, sn := range resp.Supernodes {
		addr := strings.TrimSpace(sn.SupernodeAccount)
		if addr != "" {
			active[addr] = struct{}{}
		}
	}
	if len(active) == 0 {
		return nil, fmt.Errorf("supernode query returned no active accounts")
	}

	return active, nil
}

func lep5LoadSignerFromMnemonicCandidates(chainID, moniker string, activeSupernodes map[string]struct{}) (keyring.Keyring, string, string, bool) {
	for _, mnemonicPath := range lep5MnemonicPathCandidates(chainID, moniker) {
		for _, keyName := range lep5SignerKeyNameCandidates(moniker) {
			kr, _, addr, err := sdkcrypto.LoadKeyringFromMnemonic(keyName, mnemonicPath)
			if err != nil {
				continue
			}
			if _, ok := activeSupernodes[addr]; ok {
				return kr, keyName, addr, true
			}
		}
	}

	return nil, "", "", false
}

func lep5MnemonicPathCandidates(chainID, moniker string) []string {
	candidates := make([]string, 0, 16)
	seen := make(map[string]struct{}, 16)

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, exists := seen[path]; exists {
			return
		}
		if _, err := os.Stat(path); err != nil {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	if fromEnv := strings.TrimSpace(os.Getenv("LUMERA_SUPERNODE_MNEMONIC_FILE")); fromEnv != "" {
		add(fromEnv)
	}
	if resolved, ok := lep5ResolveMnemonicPath(chainID, moniker); ok {
		add(resolved)
	}

	for _, pattern := range []string{
		"/shared/status/*/sn_mnemonic",
		fmt.Sprintf("/tmp/%s/shared/status/*/sn_mnemonic", chainID),
		"/tmp/lumera-devnet*/shared/status/*/sn_mnemonic",
		"/tmp/*/shared/status/*/sn_mnemonic",
	} {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			add(match)
		}
	}

	return candidates
}

func lep5SignerKeyNameCandidates(moniker string) []string {
	candidates := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		candidates = append(candidates, name)
	}

	add(os.Getenv("LUMERA_SUPERNODE_KEY_NAME"))
	add(resolveLumeraKeyName())
	add("supernova_validator_1_key")
	add("supernova_supernode_1_key")

	if moniker != "" {
		add(moniker + "_key")
		if strings.Contains(moniker, "validator") {
			add(strings.Replace(moniker, "validator", "supernode", 1) + "_key")
		}
	}

	for _, validatorCfgPath := range lep5ValidatorConfigCandidates(getenv("LUMERA_CHAIN_ID", defaultLumeraChainID)) {
		data, err := os.ReadFile(validatorCfgPath)
		if err != nil {
			continue
		}

		var vals []struct {
			KeyName string `json:"key_name"`
		}
		if err := json.Unmarshal(data, &vals); err != nil {
			continue
		}

		for _, v := range vals {
			add(v.KeyName)
			if strings.Contains(v.KeyName, "validator") {
				add(strings.Replace(v.KeyName, "validator", "supernode", 1))
			}
		}
	}

	return candidates
}

func lep5KeyringHomeCandidates(chainID string) []string {
	candidates := make([]string, 0, 24)
	seen := make(map[string]struct{}, 24)

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, exists := seen[path]; exists {
			return
		}
		if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	add(os.Getenv("LUMERA_HOME"))
	add(fmt.Sprintf("/tmp/%s/.lumera", chainID))
	add("/tmp/lumera-devnet/supernova_validator_1-data")
	add("/tmp/lumera-devnet-1/supernova_validator_1-data")
	add("/tmp/lumera-devnet/validator1-data")
	add("/tmp/lumera-devnet-1/validator1-data")

	for _, pattern := range []string{
		"/tmp/lumera-devnet*/supernova_validator_*-data",
		"/tmp/lumera-devnet*/validator*-data",
		"/tmp/*/supernova_validator_*-data",
		"/tmp/*/validator*-data",
	} {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			add(match)
		}
	}

	if userHome, err := os.UserHomeDir(); err == nil && userHome != "" {
		add(filepath.Join(userHome, ".lumera"))
		add(filepath.Join(userHome, ".lumera-devnet"))
		add(filepath.Join(userHome, ".lumera-testnet"))
		add(filepath.Join(userHome, ".lumera-upgrade-test"))
	}

	if len(candidates) == 0 {
		if userHome, err := os.UserHomeDir(); err == nil && userHome != "" {
			candidates = append(candidates, filepath.Join(userHome, ".lumera"))
		} else {
			candidates = append(candidates, "/root/.lumera")
		}
	}

	return candidates
}

func lep5ValidatorConfigCandidates(chainID string) []string {
	candidates := make([]string, 0, 16)
	seen := make(map[string]struct{}, 16)

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, exists := seen[path]; exists {
			return
		}
		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	add(getenv("LUMERA_VALIDATORS_FILE", defaultValidatorsFile))
	add("/shared/config/validators.json")
	add(fmt.Sprintf("/tmp/%s/shared/config/validators.json", chainID))
	add("/tmp/lumera-devnet/shared/config/validators.json")
	add("/tmp/lumera-devnet-1/shared/config/validators.json")

	for _, pattern := range []string{
		"/tmp/lumera-devnet*/shared/config/validators.json",
		"/tmp/*/shared/config/validators.json",
	} {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			add(match)
		}
	}

	return candidates
}

func lep5ResolveReachableRPC(ctx context.Context) (string, error) {
	candidates := []string{
		strings.TrimSpace(os.Getenv("LUMERA_RPC_ADDR")),
		resolveLumeraRPC(),
		"http://localhost:26667",
		"http://127.0.0.1:26667",
		"http://localhost:26677",
		"http://127.0.0.1:26677",
		"http://localhost:26687",
		"http://127.0.0.1:26687",
		"http://localhost:26697",
		"http://127.0.0.1:26697",
		"http://localhost:26607",
		"http://127.0.0.1:26607",
		"http://localhost:26657",
		"http://127.0.0.1:26657",
	}

	seen := make(map[string]struct{}, len(candidates))
	var lastErr error
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		if _, _, err := lep5NextFinalizeSeed(ctx, candidate); err == nil {
			return candidate, nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("no rpc candidates available")
}

func lep5ResolveReachableGRPC(preferred string) string {
	candidates := []string{
		preferred,
		lep5DefaultLumeraGRPC,
		"localhost:9091",
		"127.0.0.1:9091",
		"localhost:9092",
		"127.0.0.1:9092",
		"localhost:9093",
		"127.0.0.1:9093",
		"localhost:9094",
		"127.0.0.1:9094",
		"localhost:9095",
		"127.0.0.1:9095",
		"localhost:9090",
		"127.0.0.1:9090",
	}
	seen := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		candidate = lep5NormalizeGRPCAddr(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if lep5CanDialTCP(candidate) {
			return candidate
		}
	}

	return lep5NormalizeGRPCAddr(preferred)
}

func lep5CanDialTCP(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func lep5NormalizeGRPCAddr(addr string) string {
	out := strings.TrimSpace(addr)
	out = strings.TrimPrefix(out, "http://")
	out = strings.TrimPrefix(out, "https://")
	return out
}

func lep5ResolveFileSizeKBs(filePath, msgFileSize string) int64 {
	if msgFileSize != "" {
		if parsed, err := strconv.ParseInt(msgFileSize, 10, 64); err == nil && parsed > 0 {
			return parsed
		}
	}
	fi, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return (fi.Size() + 1023) / 1024
}

func lep5CreateTestFile(t *testing.T, numChunks int) (string, [][]byte, uint64) {
	t.Helper()
	return lep5CreateTestFileWithSize(t, uint64(numChunks)*uint64(lep5ChunkSize), lep5ChunkSize)
}

// lep5CreateTestFileWithSize creates a temporary test file of exactly fileSize
// bytes, splits it into chunks of chunkSize bytes (the last chunk may be
// shorter), writes the file to disk, and returns the path, the chunk slices,
// and the total size. Each chunk is filled with a repeating byte pattern
// derived from its index so that chunk data is deterministic but
// distinguishable across chunks.
func lep5CreateTestFileWithSize(t *testing.T, fileSize uint64, chunkSize uint32) (string, [][]byte, uint64) {
	t.Helper()

	remaining := fileSize
	var chunks [][]byte
	var fileData bytes.Buffer
	idx := 0

	for remaining > 0 {
		sz := uint64(chunkSize)
		if sz > remaining {
			sz = remaining
		}
		chunk := bytes.Repeat([]byte{byte(idx%255 + 1)}, int(sz))
		chunks = append(chunks, chunk)
		_, err := fileData.Write(chunk)
		require.NoError(t, err)
		remaining -= sz
		idx++
	}

	path := filepath.Join(t.TempDir(), "lep5-e2e.bin")
	require.NoError(t, os.WriteFile(path, fileData.Bytes(), 0o600))

	return path, chunks, uint64(fileData.Len())
}

func lep5ToChunkProof(p *merkle.Proof) *actiontypes.ChunkProof {
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

func lep5NextFinalizeSeed(ctx context.Context, rpcAddr string) (uint64, []byte, error) {
	type statusResponse struct {
		Result struct {
			SyncInfo struct {
				LatestBlockHeight string `json:"latest_block_height"`
				LatestBlockHash   string `json:"latest_block_hash"`
			} `json:"sync_info"`
		} `json:"result"`
	}

	url := strings.TrimSuffix(rpcAddr, "/") + "/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("build status request: %w", err)
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("status request failed: %s", resp.Status)
	}

	var status statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return 0, nil, fmt.Errorf("decode status response: %w", err)
	}

	latestHeight, err := strconv.ParseUint(strings.TrimSpace(status.Result.SyncInfo.LatestBlockHeight), 10, 64)
	if err != nil {
		return 0, nil, fmt.Errorf("parse latest block height: %w", err)
	}

	hashHex := strings.TrimSpace(status.Result.SyncInfo.LatestBlockHash)
	hashHex = strings.TrimPrefix(hashHex, "0x")
	hashHex = strings.TrimPrefix(hashHex, "0X")
	if hashHex == "" {
		return 0, nil, fmt.Errorf("latest block hash is empty")
	}

	latestHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return 0, nil, fmt.Errorf("decode latest block hash: %w", err)
	}

	return latestHeight + 1, latestHash, nil
}

// lep5QueryBlockPrevHash queries the previous-block hash for a specific height
// via CometBFT RPC /block endpoint. The returned hash is the LastBlockId.Hash
// of the block at the given height (i.e., the hash of block height-1).
func lep5QueryBlockPrevHash(ctx context.Context, rpcAddr string, height uint64) ([]byte, error) {
	url := fmt.Sprintf("%s/block?height=%d", strings.TrimSuffix(rpcAddr, "/"), height)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build block request: %w", err)
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("request block: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("block request failed: %s", resp.Status)
	}

	var blockResp struct {
		Result struct {
			Block struct {
				Header struct {
					LastBlockID struct {
						Hash string `json:"hash"`
					} `json:"last_block_id"`
				} `json:"header"`
			} `json:"block"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&blockResp); err != nil {
		return nil, fmt.Errorf("decode block response: %w", err)
	}

	hashHex := strings.TrimSpace(blockResp.Result.Block.Header.LastBlockID.Hash)
	hashHex = strings.TrimPrefix(hashHex, "0x")
	hashHex = strings.TrimPrefix(hashHex, "0X")
	if hashHex == "" {
		return nil, fmt.Errorf("block %d last_block_id.hash is empty", height)
	}

	return hex.DecodeString(hashHex)
}

// lep5PrintMerkleTreeDiagram prints a level-by-level ASCII diagram of the
// Merkle tree, highlighting challenged leaves (★) and the sibling nodes that
// form the proof paths (P). Padded leaves are marked with ░.
//
// The function is a no-op unless LUMERA_PRINT_MERKLE_TREE=true is set.
func lep5PrintMerkleTreeDiagram(t *testing.T, tree *merkle.Tree, challengeIndices []uint32) {
	t.Helper()

	if os.Getenv("LUMERA_PRINT_MERKLE_TREE") != "true" {
		return
	}

	// Build set of challenged leaf indices for O(1) lookup.
	challengedSet := make(map[int]struct{}, len(challengeIndices))
	for _, idx := range challengeIndices {
		challengedSet[int(idx)] = struct{}{}
	}

	// For each level, track which node indices are proof siblings.
	// A proof sibling is the node paired with the verification-path node at
	// each level; it is the hash included in the Merkle proof.
	proofSiblings := make([]map[int]struct{}, len(tree.Levels))
	for i := range proofSiblings {
		proofSiblings[i] = make(map[int]struct{})
	}

	for _, leafIdx := range challengeIndices {
		idx := int(leafIdx)
		for level := 0; level < len(tree.Levels)-1; level++ {
			if idx%2 == 0 {
				proofSiblings[level][idx+1] = struct{}{}
			} else {
				proofSiblings[level][idx-1] = struct{}{}
			}
			idx /= 2
		}
	}

	t.Logf("")
	t.Logf("╔══════════════════════════════════════════════════════════════╗")
	t.Logf("║                  MERKLE TREE DIAGRAM                       ║")
	t.Logf("╚══════════════════════════════════════════════════════════════╝")
	t.Logf("  Real leaves: %d  |  Padded total: %d  |  Levels: %d",
		tree.LeafCount, len(tree.Levels[0]), len(tree.Levels))
	t.Logf("  Challenge indices: %v", challengeIndices)
	t.Logf("")

	// Print from root level down to leaves.
	for level := len(tree.Levels) - 1; level >= 0; level-- {
		nodes := tree.Levels[level]
		label := fmt.Sprintf("Level %d", level)
		switch {
		case level == len(tree.Levels)-1:
			label += " (root)"
		case level == 0:
			label += " (leaves)"
		}
		t.Logf("  %s:", label)

		for i, hash := range nodes {
			hashHex := hex.EncodeToString(hash[:])
			short := hashHex[:6] + ".." + hashHex[len(hashHex)-6:]

			var markers []string

			if level == 0 {
				if _, ok := challengedSet[i]; ok {
					markers = append(markers, "★ challenged")
				}
				if i >= tree.LeafCount {
					markers = append(markers, "░ padding")
				}
			}
			if _, ok := proofSiblings[level][i]; ok {
				markers = append(markers, "P proof-sibling")
			}

			tag := ""
			if len(markers) > 0 {
				tag = "  ← " + strings.Join(markers, ", ")
			}

			t.Logf("    [%d] %s%s", i, short, tag)
		}
		t.Logf("")
	}

	t.Logf("  Legend: ★ = challenged leaf | P = proof sibling | ░ = padding duplicate")
	t.Logf("")
}

// lep5PrintCascadeMetadata pretty-prints a CascadeMetadata struct via t.Log
// so the operator can inspect what was created / stored on chain.
func lep5PrintCascadeMetadata(t *testing.T, meta *actiontypes.CascadeMetadata) {
	t.Helper()

	t.Logf("  DataHash:   %s", meta.DataHash)
	t.Logf("  FileName:   %s", meta.FileName)
	t.Logf("  RqIdsIc:    %d", meta.RqIdsIc)
	t.Logf("  RqIdsMax:   %d", meta.RqIdsMax)
	t.Logf("  Signatures: %s", meta.Signatures)
	t.Logf("  Public:     %v", meta.Public)
	t.Logf("  RqIdsIds:   (%d entries)", len(meta.RqIdsIds))
	for i, id := range meta.RqIdsIds {
		t.Logf("    [%d] %s", i, id)
	}

	if c := meta.AvailabilityCommitment; c != nil {
		t.Logf("  AvailabilityCommitment:")
		t.Logf("    CommitmentType:   %s", c.CommitmentType)
		t.Logf("    HashAlgo:         %s", c.HashAlgo)
		t.Logf("    ChunkSize:        %d", c.ChunkSize)
		t.Logf("    TotalSize:        %d", c.TotalSize)
		t.Logf("    NumChunks:        %d", c.NumChunks)
		t.Logf("    Root:             %s", hex.EncodeToString(c.Root))
		t.Logf("    ChallengeIndices: %v", c.ChallengeIndices)
	} else {
		t.Log("  AvailabilityCommitment: <nil>")
	}

	t.Logf("  ChunkProofs: (%d entries)", len(meta.ChunkProofs))
	for i, cp := range meta.ChunkProofs {
		t.Logf("    [%d] ChunkIndex=%d  LeafHash=%s  PathHashes=%d  PathDirections=%v",
			i, cp.ChunkIndex, hex.EncodeToString(cp.LeafHash), len(cp.PathHashes), cp.PathDirections)
	}
}

// TestLEP5QueryActionMetadata connects to the local devnet, queries an action
// by its ID, decodes the protobuf CascadeMetadata, and prints every field so
// the operator can verify the on-chain state matches what was submitted.
//
// Set LUMERA_ACTION_ID to the action ID you want to inspect (default "1").
func TestLEP5QueryActionMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	grpcAddr := lep5ResolveReachableGRPC(lep5NormalizeGRPCAddr(getenv("LUMERA_GRPC_ADDR", lep5DefaultLumeraGRPC)))

	if !lep5CanDialTCP(grpcAddr) {
		t.Skipf("skipping: cannot reach gRPC at %s", grpcAddr)
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dialCancel()

	conn, err := grpc.DialContext(
		dialCtx,
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err, "dial gRPC %s", grpcAddr)
	defer conn.Close()

	actionID := getenv("LUMERA_ACTION_ID", "1")
	t.Logf("Querying action ID: %s (set LUMERA_ACTION_ID to override)", actionID)

	queryClient := actiontypes.NewQueryClient(conn)
	resp, err := queryClient.GetAction(ctx, &actiontypes.QueryGetActionRequest{ActionID: actionID})
	require.NoError(t, err, "GetAction(%s)", actionID)
	require.NotNil(t, resp.Action, "action must not be nil")

	action := resp.Action
	t.Log("=== ACTION ===")
	t.Logf("  ActionID:       %s", action.ActionID)
	t.Logf("  Creator:        %s", action.Creator)
	t.Logf("  ActionType:     %s", action.ActionType)
	t.Logf("  State:          %s", action.State)
	t.Logf("  Price:          %s", action.Price)
	t.Logf("  ExpirationTime: %d", action.ExpirationTime)
	t.Logf("  BlockHeight:    %d", action.BlockHeight)
	t.Logf("  SuperNodes:     %v", action.SuperNodes)
	t.Logf("  FileSizeKbs:    %d", action.FileSizeKbs)
	t.Logf("  Metadata bytes: %d", len(action.Metadata))

	if action.ActionType != actiontypes.ActionTypeCascade {
		t.Logf("Action type is %s, not CASCADE – raw metadata hex: %s", action.ActionType, hex.EncodeToString(action.Metadata))
		return
	}

	var meta actiontypes.CascadeMetadata
	require.NoError(t, gogoproto.Unmarshal(action.Metadata, &meta), "decode CascadeMetadata")

	t.Log("=== CASCADE METADATA ===")
	lep5PrintCascadeMetadata(t, &meta)
}
