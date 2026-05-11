package keeper

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

// TestUpdateNodeSuspicionHistory_FutureWindowStartIsSafe verifies NEW-A-12/NEW-A-17:
// If a NodeSuspicionState arrives (e.g. from a malformed genesis that bypassed
// validation, or a future-pointing field) with WindowStartEpoch > epochID,
// the window-reset path must use saturated subtraction (epochDelta) and not
// panic via uint64 underflow.
func TestUpdateNodeSuspicionHistory_FutureWindowStartIsSafe(t *testing.T) {
	var k Keeper
	params := types.DefaultParams().WithDefaults()
	state := types.NodeSuspicionState{
		SupernodeAccount: "lumera1aaaa",
		WindowStartEpoch: 100, // future relative to currentEpoch=5
	}
	result := &types.StorageProofResult{
		ResultClass:   types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		ArtifactClass: types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX,
		BucketType:    types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
	}

	require.NotPanics(t, func() {
		k.updateNodeSuspicionHistoryFields(&state, result, 5, params)
	})

	// epochDelta saturates to 0 when WindowStartEpoch > epochID, so the
	// stale-window reset is NOT triggered (delta 0 < window). The state
	// must remain valid (no panic, score field still bumps in-window).
	require.Equal(t, uint64(100), state.WindowStartEpoch)
	require.GreaterOrEqual(t, state.DistinctTicketFailWindow, uint32(1))
}
