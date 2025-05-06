//go:build system_test

package system

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	claimtestutils "github.com/LumeraProtocol/lumera/x/claim/testutils"
)

// System-wide constants that define test parameters and expected behavior
const (
	// Amount of tokens allocated per individual claim
	tokensPerClaim = 1000000 // 1M tokens per claim

	// Response codes that indicate transaction status
	successCode          = "0"    // Indicates successful claim processing
	errCodeTooManyClaims = "1102" // Error code when block claim limit is exceeded

	// Test configuration parameters
	numTestEntries    = 10                    // Total number of test cases to generate
	maxClaimsPerBlock = 2                     // Maximum allowed claims per block
	numTxToSubmit     = maxClaimsPerBlock + 1 // Intentionally exceed block limit by 1
)

// TestMaxClaimsPerBlockReset is a system test that verifies the block-level claim limiting mechanism.
// The test ensures that:
// 1. Claims are properly limited per block according to maxClaimsPerBlock
// 2. The claim counter correctly resets between blocks
// 3. Failed claims receive appropriate error codes
// 4. Successful claims are processed correctly
//
// Test Flow:
// Block N:
//   - Submit maxClaimsPerBlock + 1 transactions concurrently
//   - Verify exactly maxClaimsPerBlock succeed
//   - Verify the excess transaction fails with correct error
//
// Block N+1:
//   - Submit another batch of maxClaimsPerBlock + 1 transactions concurrently
//   - Verify the limit has reset and exactly maxClaimsPerBlock new claims succeed
//   - Verify the excess transaction fails appropriately
func TestMaxClaimsPerBlockReset(t *testing.T) {
	t.Log("Starting TestMaxClaimsPerBlockReset...")

	// Initialize clean test environment
	t.Log("Resetting chain...")
	sut.ResetChain(t)

	// Create test dataset and calculate total tokens needed
	testDataSet, totalClaimableAmount := generateClaimTestData(t)
	csvPath := createClaimsCSV(t, testDataSet)
	defer cleanupCSV(t, csvPath)

	// Configure chain with test parameters
	claimEndTime := time.Now().Add(1 * time.Hour).Unix()
	setClaimsGenesis(t, totalClaimableAmount, fmt.Sprintf("%d", maxClaimsPerBlock), true, claimEndTime)

	t.Log("Starting chain...")
	sut.StartChain(t)
	cli := NewLumeradCLI(t, sut, true)

	// Record initial block for comparison
	status := cli.CustomQuery("status")
	startingHeight := gjson.Get(status, "sync_info.latest_block_height").Int()
	t.Logf("Starting block height: %d", startingHeight)

	// First Block: Test claim limiting with concurrent submissions
	t.Logf("Block %d: Submitting %d claims concurrently (max allowed is %d)...", startingHeight+1, numTxToSubmit, maxClaimsPerBlock)

	// WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup
	// mutex to safely append to the responses slice
	var mu sync.Mutex
	firstBlockResponses := make([]string, numTxToSubmit)

	for i := 0; i < numTxToSubmit; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			testData := testDataSet[index]
			// Construct claim transaction with necessary parameters
			args := []string{
				"tx", "claim", "claim",
				testData.OldAddress,
				testData.NewAddress,
				testData.PubKey,
				testData.Signature,
				"--from", fmt.Sprintf("node%d", index),
				"--broadcast-mode", "sync",
				"--chain-id", cli.chainID,
				"--home", cli.homeDir,
				"--keyring-backend", "test",
				"--node", cli.nodeAddress,
				"--output", "json",
				"--yes",
				"--fees", cli.fees,
			}

			resp, ok := cli.run(args)

			// Thread-safe logging and response storage
			mu.Lock()
			t.Logf("Claim %d submission output: %s", index+1, resp)
			require.True(t, ok)
			firstBlockResponses[index] = resp
			mu.Unlock()
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Allow time for block completion and transaction processing
	t.Log("Waiting for first block to complete...")
	time.Sleep(12 * time.Second)

	// Extract first block details for verification
	firstBlockTxHash := gjson.Get(firstBlockResponses[0], "txhash").String()
	firstBlockTxResp := cli.CustomQuery("q", "tx", firstBlockTxHash)
	firstBlockHeight := gjson.Get(firstBlockTxResp, "height").String()

	// Analyze first block results
	var firstBlockSuccess, firstBlockErrors int
	for i, resp := range firstBlockResponses {
		txHash := gjson.Get(resp, "txhash").String()
		require.NotEmpty(t, txHash, "txhash not found in response")

		txResp := cli.CustomQuery("q", "tx", txHash)
		txCode := gjson.Get(txResp, "code").String()
		txHeight := gjson.Get(txResp, "height").String()

		// Verify all transactions are in same block
		require.Equal(t, firstBlockHeight, txHeight, "All first block transactions should be in the same block")

		if txCode == successCode {
			firstBlockSuccess++
			verifySuccessfulClaim(t, txResp, testDataSet[i])
		}
		if txCode == errCodeTooManyClaims {
			firstBlockErrors++
		}
	}

	t.Logf("First block results - Successful: %d, Failed: %d", firstBlockSuccess, firstBlockErrors)
	require.Equal(t, maxClaimsPerBlock, firstBlockSuccess, "Expected exactly maxClaimsPerBlock successful claims in first block")
	require.Equal(t, 1, firstBlockErrors, "Expected exactly one failed claim in first block")

	// Second Block: Verify limit reset with concurrent submissions
	t.Log("Submitting second batch of claims concurrently in new block...")

	// Reset WaitGroup for second block
	secondBlockResponses := make([]string, numTxToSubmit)

	for i := 0; i < numTxToSubmit; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			testData := testDataSet[index+numTxToSubmit] // Use next set of test data
			args := []string{
				"tx", "claim", "claim",
				testData.OldAddress,
				testData.NewAddress,
				testData.PubKey,
				testData.Signature,
				"--from", fmt.Sprintf("node%d", index),
				"--broadcast-mode", "sync",
				"--chain-id", cli.chainID,
				"--home", cli.homeDir,
				"--keyring-backend", "test",
				"--node", cli.nodeAddress,
				"--output", "json",
				"--yes",
				"--fees", cli.fees,
			}

			resp, ok := cli.run(args)

			// Thread-safe logging and response storage
			mu.Lock()
			t.Logf("Second block claim %d submission output: %s", index+1, resp)
			require.True(t, ok)
			secondBlockResponses[index] = resp
			mu.Unlock()
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Allow time for second block processing
	t.Log("Waiting for second block to complete...")
	time.Sleep(12 * time.Second)

	// Extract second block details
	secondBlockTxHash := gjson.Get(secondBlockResponses[0], "txhash").String()
	secondBlockTxResp := cli.CustomQuery("q", "tx", secondBlockTxHash)
	secondBlockHeight := gjson.Get(secondBlockTxResp, "height").String()

	// Ensure blocks are distinct
	require.NotEqual(t, firstBlockHeight, secondBlockHeight, "Second batch should be in a different block")

	// Analyze second block results
	var secondBlockSuccess, secondBlockErrors int
	for i, resp := range secondBlockResponses {
		txHash := gjson.Get(resp, "txhash").String()
		require.NotEmpty(t, txHash, "txhash not found in response")

		txResp := cli.CustomQuery("q", "tx", txHash)
		txCode := gjson.Get(txResp, "code").String()
		txHeight := gjson.Get(txResp, "height").String()

		require.Equal(t, secondBlockHeight, txHeight, "All second block transactions should be in the same block")

		if txCode == successCode {
			secondBlockSuccess++
			verifySuccessfulClaim(t, txResp, testDataSet[i+numTxToSubmit])
		}
		if txCode == errCodeTooManyClaims {
			secondBlockErrors++
		}
	}

	t.Logf("Second block results - Successful: %d, Failed: %d", secondBlockSuccess, secondBlockErrors)
	require.Equal(t, maxClaimsPerBlock, secondBlockSuccess, "Expected exactly maxClaimsPerBlock successful claims in second block")
	require.Equal(t, 1, secondBlockErrors, "Expected exactly one failed claim in second block")
}

// generateClaimTestData creates a set of test data entries for claims testing.
// Returns:
// - Array of TestData structures containing claim information
// - Total amount of tokens that should be made available for claims
func generateClaimTestData(t *testing.T) ([]claimtestutils.TestData, int64) {
	t.Log("Generating test data...")
	var testDataSet []claimtestutils.TestData
	var totalClaimableAmount int64

	for i := 0; i < numTestEntries; i++ {
		testData, err := claimtestutils.GenerateClaimingTestData()
		require.NoError(t, err)
		testDataSet = append(testDataSet, testData)
		totalClaimableAmount += tokensPerClaim
	}
	t.Logf("Generated %d test data entries", len(testDataSet))

	return testDataSet, totalClaimableAmount
}

// createClaimsCSV generates a CSV file containing claim data for testing.
// The file format is: old_address,token_amount
// Returns the path to the created CSV file.
func createClaimsCSV(t *testing.T, testDataSet []claimtestutils.TestData) string {
	t.Log("Creating CSV file...")
	csvContent := ""
	for _, data := range testDataSet {
		csvContent += fmt.Sprintf("%s,%d\n", data.OldAddress, tokensPerClaim)
	}

	homedir, err := os.UserHomeDir()
	require.NoError(t, err)
	csvPath := homedir + "/claims.csv"
	err = os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)
	t.Log("CSV file created successfully")

	return csvPath
}

// cleanupCSV registers a cleanup function to remove the test CSV file
// after test completion.
func cleanupCSV(t *testing.T, csvPath string) {
	t.Cleanup(func() { _ = os.Remove(csvPath) })
}

// setClaimsGenesis configures the genesis state for claims testing.
// Parameters:
// - totalClaimableAmount: Total tokens available for claims
// - maxClaimsPerBlock: Maximum number of claims allowed per block
// - enableClaims: Whether claiming is enabled
// - claimEndTime: Unix timestamp when claiming period ends
func setClaimsGenesis(t *testing.T, totalClaimableAmount int64, maxClaimsPerBlock string, enableClaims bool, claimEndTime int64) {
	t.Log("Modifying genesis configuration...")
	sut.ModifyGenesisJSON(t, func(genesis []byte) []byte {
		state := genesis
		var err error

		// Configure total tokens available for claims
		state, err = sjson.SetRawBytes(state, "app_state.claim.total_claimable_amount", []byte(fmt.Sprintf("%d", totalClaimableAmount)))
		require.NoError(t, err)

		// Set maximum claims allowed per block
		state, err = sjson.SetRawBytes(state, "app_state.claim.params.max_claims_per_block", []byte(maxClaimsPerBlock))
		require.NoError(t, err)

		// Enable or disable claiming functionality
		state, err = sjson.SetRawBytes(state, "app_state.claim.params.enable_claims", []byte("true"))
		require.NoError(t, err)

		// Set the deadline for making claims
		state, err = sjson.SetRawBytes(state, "app_state.claim.params.claim_end_time", []byte(fmt.Sprintf("%d", claimEndTime)))
		require.NoError(t, err)

		return state
	})
}

// verifySuccessfulClaim checks that a successful claim transaction
// contains all required events and correct data.
// Parameters:
// - txResp: Transaction response JSON
// - testData: Expected claim data to verify against
func verifySuccessfulClaim(t *testing.T, txResp string, testData claimtestutils.TestData) {
	events := gjson.Get(txResp, "events").Array()

	var foundClaimProcessed bool
	var foundTransfer bool
	var foundMsgClaim bool

	// Examine each event in the transaction
	for _, event := range events {
		eventType := event.Get("type").String()
		attrs := event.Get("attributes").Array()

		switch eventType {
		case "claim_processed":
			// Verify claim processing details
			foundClaimProcessed = true
			for _, attr := range attrs {
				key := attr.Get("key").String()
				value := attr.Get("value").String()

				switch key {
				case "module":
					require.Equal(t, "claim", value)
				case "amount":
					require.Equal(t, fmt.Sprintf("%dulume", tokensPerClaim), value)
				case "old_address":
					require.Equal(t, testData.OldAddress, value)
				case "new_address":
					require.Equal(t, testData.NewAddress, value)
				case "claim_time":
					require.NotEmpty(t, value)
				}
			}

		case "transfer":
			// Verify token transfer details
			var hasCorrectTransfer bool
			for _, attr := range attrs {
				if attr.Get("key").String() == "amount" &&
					attr.Get("value").String() == fmt.Sprintf("%dulume", tokensPerClaim) {
					hasCorrectTransfer = true
					foundTransfer = true
					break
				}
			}
			if hasCorrectTransfer {
				// Confirm recipient address
				for _, attr := range attrs {
					if attr.Get("key").String() == "recipient" {
						require.Equal(t, testData.NewAddress, attr.Get("value").String())
					}
				}
			}

		case "message":
			// Verify message type
			for _, attr := range attrs {
				if attr.Get("key").String() == "action" &&
					attr.Get("value").String() == "/lumera.claim.MsgClaim" {
					foundMsgClaim = true
				}
			}
		}
	}

	// Ensure all required events were found
	require.True(t, foundClaimProcessed, "claim_processed event not found")
	require.True(t, foundTransfer, "transfer event not found")
	require.True(t, foundMsgClaim, "MsgClaim action not found")
}
