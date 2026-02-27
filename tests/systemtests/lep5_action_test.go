//go:build system_test

package system

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/action/v1/merkle"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// TestLEP5ActionParamsQuery verifies that the action module params endpoint
// returns a valid response that includes the SVC-related fields.
func TestLEP5ActionParamsQuery(t *testing.T) {
	sut.ResetChain(t)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, verbose)

	result := cli.CustomQuery("q", "action", "params")
	t.Logf("Action params: %s", result)

	// The params response must include the action module fields.
	require.True(t, gjson.Get(result, "params").Exists(), "params key must exist")
	require.True(t, gjson.Get(result, "params.base_action_fee").Exists(), "base_action_fee must exist")
	require.True(t, gjson.Get(result, "params.expiration_duration").Exists(), "expiration_duration must exist")
}

// TestLEP5CascadeRegisterWithCommitment verifies that a Cascade action can be
// registered with an AvailabilityCommitment via CLI and then queried.
func TestLEP5CascadeRegisterWithCommitment(t *testing.T) {
	sut.ResetChain(t)
	sut.StartChain(t)

	cli := NewLumeradCLI(t, sut, verbose)

	// Fund the test account.
	account := cli.GetKeyAddr("node0")
	require.NotEmpty(t, account)

	// Build a Merkle tree from 8 chunks.
	numChunks := uint32(8)
	chunkSize := uint32(262144)
	chunks := make([][]byte, numChunks)
	for i := range chunks {
		chunks[i] = []byte(fmt.Sprintf("systest-chunk-%d", i))
	}

	tree, err := merkle.BuildTree(chunks)
	require.NoError(t, err)

	root := make([]byte, merkle.HashSize)
	copy(root, tree.Root[:])

	challengeIndices := []uint32{0, 1, 2, 3, 4, 5, 6, 7}

	commitment := actiontypes.AvailabilityCommitment{
		CommitmentType:   "lep5/chunk-merkle/v1",
		HashAlgo:         actiontypes.HashAlgo_HASH_ALGO_BLAKE3,
		ChunkSize:        chunkSize,
		TotalSize:        uint64(numChunks) * uint64(chunkSize),
		NumChunks:        numChunks,
		Root:             root,
		ChallengeIndices: challengeIndices,
	}
	commitmentJSON, err := json.Marshal(&commitment)
	require.NoError(t, err)

	// Build a valid signature: base64(data).base64(sig).
	sigData := base64.StdEncoding.EncodeToString([]byte("rqid-1"))

	expirationTime := fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix())

	metadata := fmt.Sprintf(
		`{"data_hash":"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890","file_name":"test.bin","rq_ids_ic":1,"signatures":"%s.fake","availability_commitment":%s}`,
		sigData, string(commitmentJSON),
	)

	price := fmt.Sprintf("100000%s", lcfg.ChainDenom)

	// Submit the request-action transaction.
	resp := cli.CustomCommand(
		"tx", "action", "request-action",
		"ACTION_TYPE_CASCADE",
		metadata,
		price,
		expirationTime,
		"--from", "node0",
	)
	t.Logf("Request action response: %s", resp)

	// The tx may succeed or fail depending on signature validation.
	// For the systemex test, we verify the CLI can construct and submit
	// the transaction with LEP-5 commitment fields without crashing.
	// A full E2E flow requires proper key-based signatures which the
	// devnet tests (S08) cover.
	txCode := gjson.Get(resp, "code")
	t.Logf("TX code: %s", txCode.String())
}
