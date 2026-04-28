package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestProcessStorageTruthHealOpsAtEpochEnd_SchedulesByPriority(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(400).WithEventManager(sdk.NewEventManager())

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthTicketDeteriorationHealThreshold = 40
	params.StorageTruthMaxSelfHealOpsPerEpoch = 2
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	activeAccounts := []string{"sn-aaa", "sn-bbb", "sn-ccc"}
	seedEpochAnchorForReportTest(t, f, 0, activeAccounts, activeAccounts)

	// Existing non-final op keeps this ticket ineligible.
	require.NoError(t, f.keeper.SetHealOp(f.ctx, types.HealOp{
		HealOpId:                  500,
		TicketId:                  "ticket-open",
		ScheduledEpochId:          0,
		HealerSupernodeAccount:    "sn-aaa",
		VerifierSupernodeAccounts: []string{"sn-bbb"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_IN_PROGRESS,
		DeadlineEpochId:           10,
	}))

	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:                   "ticket-high",
		DeteriorationScore:         90,
		DistinctHolderFailureCount: 2, // meets eligibility predicate
	}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:                   "ticket-mid",
		DeteriorationScore:         50,
		DistinctHolderFailureCount: 2, // meets eligibility predicate
	}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:           "ticket-low",
		DeteriorationScore: 10, // below threshold
	}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:            "ticket-probation",
		DeteriorationScore:  100,
		ProbationUntilEpoch: 2, // epoch 0 should skip
	}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:           "ticket-open",
		DeteriorationScore: 200,
		ActiveHealOpId:     500,
	}))

	f.keeper.SetNextHealOpID(f.ctx, 100)
	require.NoError(t, f.keeper.ProcessStorageTruthHealOpsAtEpochEnd(f.ctx, 0, params))

	first, found := f.keeper.GetHealOp(f.ctx, 100)
	require.True(t, found)
	require.Equal(t, "ticket-high", first.TicketId)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED, first.Status)
	require.NotEmpty(t, first.HealerSupernodeAccount)

	second, found := f.keeper.GetHealOp(f.ctx, 101)
	require.True(t, found)
	require.Equal(t, "ticket-mid", second.TicketId)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED, second.Status)

	ticketHigh, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-high")
	require.True(t, found)
	require.Equal(t, uint64(100), ticketHigh.ActiveHealOpId)

	ticketMid, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-mid")
	require.True(t, found)
	require.Equal(t, uint64(101), ticketMid.ActiveHealOpId)

	ticketOpen, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-open")
	require.True(t, found)
	require.Equal(t, uint64(500), ticketOpen.ActiveHealOpId)

	require.Equal(t, uint64(102), f.keeper.GetNextHealOpID(f.ctx))
}

func TestProcessStorageTruthHealOpsAtEpochEnd_ExpiresPastDeadline(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1600).WithEventManager(sdk.NewEventManager()) // epoch 3 end

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthMaxSelfHealOpsPerEpoch = 0 // focus on expiry only
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	require.NoError(t, f.keeper.SetHealOp(f.ctx, types.HealOp{
		HealOpId:                  700,
		TicketId:                  "ticket-expire",
		ScheduledEpochId:          1,
		HealerSupernodeAccount:    "sn-healer",
		VerifierSupernodeAccounts: []string{"sn-verifier"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		DeadlineEpochId:           3,
	}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:           "ticket-expire",
		DeteriorationScore: 100,
		ActiveHealOpId:     700,
	}))

	require.NoError(t, f.keeper.ProcessStorageTruthHealOpsAtEpochEnd(f.ctx, 3, params))

	expired, found := f.keeper.GetHealOp(f.ctx, 700)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_EXPIRED, expired.Status)

	ticketState, found := f.keeper.GetTicketDeteriorationState(f.ctx, "ticket-expire")
	require.True(t, found)
	require.Equal(t, uint64(0), ticketState.ActiveHealOpId)
}

// NEW-B-1 — verify expireStorageTruthHealOpsAtEpochEnd applies the §20 no-show
// cooldown to EXPIRED heal-ops (mirror of FAILED branch): score +=15,
// probation advanced, st/fh/ marker written.
func TestExpireStorageTruthHealOps_AdvancesProbationAndCooldown(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1600).WithEventManager(sdk.NewEventManager())

	params := f.keeper.GetParams(f.ctx).WithDefaults()
	params.StorageTruthMaxSelfHealOpsPerEpoch = 0 // expire-only
	params.StorageTruthProbationEpochs = 4
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	const (
		ticketID = "ticket-cooldown"
		healer   = "lumera1cccccccccccccccccccccccccccccccccc7gqs5y"
		healOpID = uint64(800)
		epochID  = uint64(3)
	)

	require.NoError(t, f.keeper.SetHealOp(f.ctx, types.HealOp{
		HealOpId:                  healOpID,
		TicketId:                  ticketID,
		ScheduledEpochId:          1,
		HealerSupernodeAccount:    healer,
		VerifierSupernodeAccounts: []string{"lumera1eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeennf6kk"},
		Status:                    types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		DeadlineEpochId:           epochID, // due at this epoch
	}))
	require.NoError(t, f.keeper.SetTicketDeteriorationState(f.ctx, types.TicketDeteriorationState{
		TicketId:           ticketID,
		DeteriorationScore: 50,
		ActiveHealOpId:     healOpID,
	}))

	require.NoError(t, f.keeper.ProcessStorageTruthHealOpsAtEpochEnd(f.ctx, epochID, params))

	expired, found := f.keeper.GetHealOp(f.ctx, healOpID)
	require.True(t, found)
	require.Equal(t, types.HealOpStatus_HEAL_OP_STATUS_EXPIRED, expired.Status)

	state, found := f.keeper.GetTicketDeteriorationState(f.ctx, ticketID)
	require.True(t, found)
	require.Equal(t, int64(65), state.DeteriorationScore, "deterioration score must bump by 15")
	require.GreaterOrEqual(t, state.ProbationUntilEpoch, epochID+uint64(params.StorageTruthProbationEpochs),
		"probation must be advanced by ProbationEpochs")
	require.Equal(t, uint64(0), state.ActiveHealOpId)
}
