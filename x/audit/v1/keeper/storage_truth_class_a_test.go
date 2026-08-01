package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// FINDING-24: Class-A fault accounting had NO test coverage.
//
// Mutation testing proved it: setting `isClassA = false` in
// updateNodeSuspicionHistoryFields - so HASH_MISMATCH and RECHECK_CONFIRMED_FAIL
// are never counted - left the ENTIRE storage-truth suite green.
//
// ClassACountWindow / LastClassAEpoch / CleanPassCount gate band escalation and
// recovery. Without these assertions a regression could let a node with repeated
// hash mismatches (the strongest evidence of storage dishonesty) accumulate no
// suspicion and recover as if clean, with nothing failing.
//
// The code comment is explicit that TIMEOUT-on-INDEX is a liveness/Class-B
// failure that "must not reset Class-A recovery gates or increment
// ClassACountWindow" - that separation was unasserted in BOTH directions.

func classAParams() types.Params {
	p := types.DefaultParams()
	if p.StorageTruthPatternEscalationWindow == 0 {
		p.StorageTruthPatternEscalationWindow = 10
	}
	return p
}

func TestUpdateNodeSuspicionHistoryFields_ClassAFaultAccounting(t *testing.T) {
	k := Keeper{}
	params := classAParams()
	const epochID = uint64(100)

	classA := []types.StorageProofResultClass{
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
	}

	for _, class := range classA {
		t.Run(class.String(), func(t *testing.T) {
			// WindowStartEpoch == epochID keeps the escalation window FRESH.
			// If the window is stale, updateNodeSuspicionHistoryFields resets
			// ClassACountWindow to 0 before incrementing, which masks the
			// increment and lets a broken isClassA survive.
			state := types.NodeSuspicionState{
				WindowStartEpoch:  epochID,
				CleanPassCount:    7, // must be reset by a Class-A fault
				ClassACountWindow: 2, // pre-existing count must be ADDED to, not replaced
			}
			before := state.ClassACountWindow

			k.updateNodeSuspicionHistoryFields(&state,
				&types.StorageProofResult{
					ResultClass:   class,
					BucketType:    types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
					ArtifactClass: types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_SYMBOL,
				}, epochID, params)

			require.EqualValues(t, before+1, state.ClassACountWindow,
				"%s must increment ClassACountWindow - it gates band escalation", class)
			require.Equal(t, epochID, state.LastClassAEpoch,
				"%s must stamp LastClassAEpoch", class)
			require.Zero(t, state.CleanPassCount,
				"%s must reset CleanPassCount - recovery requires clean passes "+
					"with no new Class-A failures", class)
			require.Zero(t, state.ClassBCountWindow,
				"%s is Class A and must NOT increment the Class-B counter", class)
		})
	}
}

// The inverse direction: a Class-B (liveness) failure must NOT touch Class-A
// gates. Per the code comment, TIMEOUT-on-INDEX "must not reset Class-A recovery
// gates or increment ClassACountWindow".
func TestUpdateNodeSuspicionHistoryFields_ClassBDoesNotTouchClassAGates(t *testing.T) {
	k := Keeper{}
	params := classAParams()
	const epochID = uint64(100)

	state := types.NodeSuspicionState{
		WindowStartEpoch:  epochID,
		CleanPassCount:    7,
		ClassACountWindow: 3,
		LastClassAEpoch:   42,
	}

	// NOTE: the counters live inside an `isFailure` branch that also keys on
	// BucketType / ArtifactClass. A bare {ResultClass} fixture does not exercise
	// them - my first version of this test asserted against an under-specified
	// result and failed for that reason, not because the code was wrong.
	k.updateNodeSuspicionHistoryFields(&state,
		&types.StorageProofResult{
			ResultClass:   types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
			BucketType:    types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
			ArtifactClass: types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_SYMBOL,
		}, epochID, params)

	require.EqualValues(t, 1, state.ClassBCountWindow,
		"TIMEOUT_OR_NO_RESPONSE must increment ClassBCountWindow")
	require.Equal(t, epochID, state.LastClassBEpoch,
		"TIMEOUT_OR_NO_RESPONSE must stamp LastClassBEpoch")

	require.EqualValues(t, 3, state.ClassACountWindow,
		"a Class-B liveness failure must NOT increment ClassACountWindow")
	require.EqualValues(t, 42, state.LastClassAEpoch,
		"a Class-B liveness failure must NOT stamp LastClassAEpoch")
	require.EqualValues(t, 7, state.CleanPassCount,
		"a Class-B liveness failure must NOT reset Class-A recovery gates")
}
