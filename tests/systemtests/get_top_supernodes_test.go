//go:build system_test && supernode_test

package system

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestQueryGetTopSuperNodesForBlock(t *testing.T) {
	// Initialize and reset chain
	sut.ResetChain(t)

	// Create CLI helper
	cli := NewLumeradCLI(t, sut, true)

	// Start the chain
	sut.StartChain(t)

	// Register all 4 validator nodes as supernodes
	var valAddrs []string
	for i := 0; i < 4; i++ {
		nodeName := "node" + string(rune(i+'0'))
		accountAddr := cli.GetKeyAddr(nodeName)
		valAddr := strings.TrimSpace(cli.Keys("keys", "show", nodeName, "--bech", "val", "-a"))
		valAddrs = append(valAddrs, valAddr)

		// Register supernode
		registerResp := cli.CustomCommand(
			"tx", "supernode", "register-supernode",
			valAddr,       // validator address
			"192.168.1.1", // IP address
			accountAddr,   // supernode account
			"--from", nodeName,
		)
		RequireTxSuccess(t, registerResp)
	}

	// Wait for registrations to be processed
	sut.AwaitNextBlock(t)

	// Get our query height (current height after registrations)
	queryHeight := sut.AwaitNextBlock(t)
	t.Logf("Using query height: %d", queryHeight)

	// Get initial response using default ACTIVE filter
	args := []string{
		"query",
		"supernode",
		"get-top-supernodes-for-block",
		fmt.Sprint(queryHeight),
		"--output", "json",
	}

	// Get initial response to compare against
	initialResp := cli.CustomQuery(args...)
	initialHash := fmt.Sprintf("%x", sha256.Sum256([]byte(initialResp)))

	// Validate initial response
	initialNodes := gjson.Get(initialResp, "supernodes").Array()
	require.NotEmpty(t, initialNodes, "Response should not be empty")
	require.Len(t, initialNodes, 4, "Should have exactly 4 supernodes")

	// Query same height in next few blocks to verify deterministic results
	for i := 0; i < 3; i++ {
		currentHeight := sut.AwaitNextBlock(t)
		t.Logf("Querying height %d from block %d", queryHeight, currentHeight)

		resp := cli.CustomQuery(args...)
		currentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(resp)))

		// Verify the response isn't empty and has correct number of nodes
		nodes := gjson.Get(resp, "supernodes").Array()
		require.NotEmpty(t, nodes, "Response should not be empty")
		require.Len(t, nodes, 4, "Should have exactly 4 supernodes")

		// Verify response is deterministic
		if initialHash != currentHash {
			t.Logf("Initial response: %s", initialResp)
			t.Logf("Current response: %s", resp)
			t.Fatal("Response hash mismatch - results are not deterministic")
		}
	}
}

func TestQueryGetTopSuperNodesForBlockFlags(t *testing.T) {
	// Initialize and reset chain
	sut.ResetChain(t)

	// Create CLI helper
	cli := NewLumeradCLI(t, sut, true)

	// Start the chain
	sut.StartChain(t)

	// Register all 4 validator nodes as supernodes
	var valAddrs []string
	for i := 0; i < 4; i++ {
		nodeName := "node" + string(rune(i+'0'))
		accountAddr := cli.GetKeyAddr(nodeName)
		valAddr := strings.TrimSpace(cli.Keys("keys", "show", nodeName, "--bech", "val", "-a"))
		valAddrs = append(valAddrs, valAddr)

		// Register supernode
		registerResp := cli.CustomCommand(
			"tx", "supernode", "register-supernode",
			valAddr,       // validator address
			"192.168.1.1", // IP address
			accountAddr,   // supernode account
			"--from", nodeName,
		)
		RequireTxSuccess(t, registerResp)
	}

	// Wait for registrations to be processed
	sut.AwaitNextBlock(t)

	// Now stop all nodes
	for i := 0; i < 4; i++ {
		nodeName := "node" + string(rune(i+'0'))
		stopResp := cli.CustomCommand(
			"tx", "supernode", "stop-supernode",
			valAddrs[i], "",
			"--from", nodeName,
		)
		RequireTxSuccess(t, stopResp)
	}

	// Wait for stops to be processed
	sut.AwaitNextBlock(t)

	// Get our query height after all operations
	queryHeight := sut.AwaitNextBlock(t)
	t.Logf("Using query height: %d", queryHeight)

	testCases := []struct {
		name          string
		state         string
		limit         string
		expectedCount int
		shouldBeEmpty bool // New field to indicate if we expect empty response
	}{
		{
			name:          "default_state_active",
			state:         "SUPERNODE_STATE_ACTIVE",
			limit:         "2",
			expectedCount: 0,
			shouldBeEmpty: true, // We expect empty response for active nodes
		},
		{
			name:          "query_stopped_state",
			state:         "SUPERNODE_STATE_STOPPED",
			expectedCount: 4,
			shouldBeEmpty: false,
		},
		{
			name:          "query_stopped_with_limit",
			state:         "SUPERNODE_STATE_STOPPED",
			limit:         "2",
			expectedCount: 2,
			shouldBeEmpty: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build query args
			args := []string{
				"query",
				"supernode",
				"get-top-supernodes-for-block",
				fmt.Sprint(queryHeight),
				"--output", "json",
			}

			if tc.state != "" {
				args = append(args, "--state", tc.state)
			}
			if tc.limit != "" {
				args = append(args, "--limit", tc.limit)
			}

			// Get initial response and create hash
			resp := cli.CustomQuery(args...)
			initialHash := fmt.Sprintf("%x", sha256.Sum256([]byte(resp)))

			// Verify response structure and count
			nodes := gjson.Get(resp, "supernodes").Array()

			// Only check NotEmpty when we don't expect an empty response
			if !tc.shouldBeEmpty {
				require.NotEmpty(t, nodes, "Response should not be empty")
			}

			require.Len(t, nodes, tc.expectedCount, "Should have expected number of nodes")

			// Skip further checks if we expect empty response
			if tc.shouldBeEmpty {
				return
			}

			// Verify state filter works correctly
			if tc.state == "SUPERNODE_STATE_STOPPED" {
				for _, node := range nodes {
					states := node.Get("states").Array()
					require.NotEmpty(t, states, "States array should not be empty")
					lastState := states[len(states)-1]
					require.Equal(t, "SUPERNODE_STATE_STOPPED",
						lastState.Get("state").String(),
						"All returned nodes should be in stopped state")
				}
			}

			// Verify sorting is deterministic by comparing multiple queries
			for i := 0; i < 3; i++ {
				compareResp := cli.CustomQuery(args...)
				currentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(compareResp)))

				// Verify response is deterministic
				if initialHash != currentHash {
					t.Logf("Initial response: %s", resp)
					t.Logf("Current response: %s", compareResp)
					t.Fatal("Response hash mismatch - results are not deterministic")
				}
			}
		})
	}
}
