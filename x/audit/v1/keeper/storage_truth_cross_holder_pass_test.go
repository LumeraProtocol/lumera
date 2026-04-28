package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// TestApplyTicketDeteriorationDelta_CrossHolderPassBonus exercises F119-F3 residue:
// when a PASS lands on a ticket whose prior-holder state recorded a failure from a
// DIFFERENT holder, an additional -3 ticket-deterioration delta is applied on top
// of the base bucket reduction.
func TestApplyTicketDeteriorationDelta_CrossHolderPassBonus(t *testing.T) {
	t.Run("PASS by different holder applies extra -3 bonus", func(t *testing.T) {
		f := initFixture(t)
		const ticketID = "ticket-cross-holder"
		const epochID = uint64(10)

		// Seed prior failure state: holder A failed at epoch 9 with HASH_MISMATCH.
		require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
			TicketId:                   ticketID,
			DeteriorationScore:         50,
			LastUpdatedEpoch:           epochID, // Avoid decay on the read.
			LastTargetSupernodeAccount: "holder-A",
			LastResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
			LastResultEpoch:            epochID - 1,
			LastFailureEpoch:           epochID - 1,
		}))

		// Now holder B passes: per F119-F3 residue, base PASS RECENT delta is -2,
		// PLUS additional -3 cross-holder bonus = -5 total.
		passResult := &types.StorageProofResult{
			TicketId:               ticketID,
			TargetSupernodeAccount: "holder-B",
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		// delta = -2 (RECENT bucket PASS base). Decay disabled (delta=0 epochs).
		_, _, err := keeper.ApplyTicketDeteriorationDeltaForTest(
			f.keeper, f.ctx, epochID, "reporter-1", passResult, ticketID, -2, 0, false,
		)
		require.NoError(t, err)

		final, found := f.keeper.GetTicketDeteriorationState(f.ctx, ticketID)
		require.True(t, found)
		// 50 + (-2 base) + (-3 cross-holder) = 45
		require.Equal(t, int64(45), final.DeteriorationScore,
			"PASS by different holder must apply both base bucket delta and -3 cross-holder bonus")
	})

	t.Run("PASS by same holder gets only base delta (no cross-holder bonus)", func(t *testing.T) {
		f := initFixture(t)
		const ticketID = "ticket-same-holder"
		const epochID = uint64(10)

		require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
			TicketId:                   ticketID,
			DeteriorationScore:         50,
			LastUpdatedEpoch:           epochID,
			LastTargetSupernodeAccount: "holder-A",
			LastResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
			LastResultEpoch:            epochID - 1,
			LastFailureEpoch:           epochID - 1,
		}))

		passResult := &types.StorageProofResult{
			TicketId:               ticketID,
			TargetSupernodeAccount: "holder-A", // SAME holder
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		_, _, err := keeper.ApplyTicketDeteriorationDeltaForTest(
			f.keeper, f.ctx, epochID, "reporter-1", passResult, ticketID, -2, 0, false,
		)
		require.NoError(t, err)

		final, found := f.keeper.GetTicketDeteriorationState(f.ctx, ticketID)
		require.True(t, found)
		// 50 + (-2 base) = 48. NO cross-holder bonus since holder is the same.
		require.Equal(t, int64(48), final.DeteriorationScore,
			"PASS by same holder must apply only the base delta, no cross-holder bonus")
	})

	t.Run("PASS without prior failure (fresh ticket) gets only base delta", func(t *testing.T) {
		f := initFixture(t)
		const ticketID = "ticket-fresh"
		const epochID = uint64(10)

		// No prior state — fresh ticket.
		passResult := &types.StorageProofResult{
			TicketId:               ticketID,
			TargetSupernodeAccount: "holder-B",
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		_, _, err := keeper.ApplyTicketDeteriorationDeltaForTest(
			f.keeper, f.ctx, epochID, "reporter-1", passResult, ticketID, -2, 0, false,
		)
		require.NoError(t, err)

		final, found := f.keeper.GetTicketDeteriorationState(f.ctx, ticketID)
		require.True(t, found)
		// 0 + (-2) clamped at 0 = 0.
		require.Equal(t, int64(0), final.DeteriorationScore,
			"fresh ticket PASS clamps at 0; no cross-holder bonus without prior state")
	})

	t.Run("PASS by different holder after non-hash failure classes applies extra -3 bonus", func(t *testing.T) {
		failureClasses := []types.StorageProofResultClass{
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL,
			types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT,
		}
		for _, priorClass := range failureClasses {
			t.Run(priorClass.String(), func(t *testing.T) {
				f := initFixture(t)
				const epochID = uint64(10)
				ticketID := "ticket-cross-holder-" + priorClass.String()

				require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
					TicketId:                   ticketID,
					DeteriorationScore:         50,
					LastUpdatedEpoch:           epochID,
					LastTargetSupernodeAccount: "holder-A",
					LastResultClass:            priorClass,
					LastResultEpoch:            epochID - 1,
					LastFailureEpoch:           epochID - 1,
				}))

				passResult := &types.StorageProofResult{
					TicketId:               ticketID,
					TargetSupernodeAccount: "holder-B",
					ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
					BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
				}
				_, _, err := keeper.ApplyTicketDeteriorationDeltaForTest(
					f.keeper, f.ctx, epochID, "reporter-1", passResult, ticketID, -2, 0, false,
				)
				require.NoError(t, err)

				final, found := f.keeper.GetTicketDeteriorationState(f.ctx, ticketID)
				require.True(t, found)
				require.Equal(t, int64(45), final.DeteriorationScore,
					"prior failure class %s should receive base PASS delta plus cross-holder recovery bonus", priorClass)
			})
		}
	})

	t.Run("PASS by different holder but prior was PASS (not failure) — no bonus", func(t *testing.T) {
		f := initFixture(t)
		const ticketID = "ticket-prior-pass"
		const epochID = uint64(10)

		// Prior state has different holder but it was a PASS, not a failure.
		require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
			TicketId:                   ticketID,
			DeteriorationScore:         50,
			LastUpdatedEpoch:           epochID,
			LastTargetSupernodeAccount: "holder-A",
			LastResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			LastResultEpoch:            epochID - 1,
		}))

		passResult := &types.StorageProofResult{
			TicketId:               ticketID,
			TargetSupernodeAccount: "holder-B",
			ResultClass:            types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			BucketType:             types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECENT,
		}
		_, _, err := keeper.ApplyTicketDeteriorationDeltaForTest(
			f.keeper, f.ctx, epochID, "reporter-1", passResult, ticketID, -2, 0, false,
		)
		require.NoError(t, err)

		final, found := f.keeper.GetTicketDeteriorationState(f.ctx, ticketID)
		require.True(t, found)
		// 50 + (-2) = 48. Bonus only fires on prior FAILURE class, not prior PASS.
		require.Equal(t, int64(48), final.DeteriorationScore,
			"prior PASS (not failure) → no cross-holder recovery bonus")
	})
}
