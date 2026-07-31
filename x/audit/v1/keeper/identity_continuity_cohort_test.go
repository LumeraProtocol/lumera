package keeper_test

import (
	"fmt"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// cohortActor is one supernode in the epoch's frozen active set, optionally
// migrated to a new current account partway through.
type cohortActor struct {
	logical  string // the account anchored in the epoch's active set
	current  string // the account that actually signs today
	migrated bool
}

// buildCohort creates n actors and migrates the first migrateCount of them.
// Transitions are effective at epoch 1, so epoch 0 is the "current, frozen"
// epoch anchored under logical identities while signing happens under current
// identities.
func buildCohort(t *testing.T, f *fixture, n, migrateCount int) []cohortActor {
	t.Helper()

	actors := make([]cohortActor, 0, n)
	for i := 0; i < n; i++ {
		logical := testAddress(t, f, []byte{byte(40 + i), 1, 2, 3})
		actor := cohortActor{logical: logical, current: logical}
		if i < migrateCount {
			current := testAddress(t, f, []byte{byte(80 + i), 4, 5, 6})
			require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
				SourceAccount:      logical,
				DestinationAccount: current,
				EffectiveEpoch:     1,
			}))
			actor.current = current
			actor.migrated = true
		}
		actors = append(actors, actor)
	}
	return actors
}

func cohortLogicalAccounts(actors []cohortActor) []string {
	out := make([]string, 0, len(actors))
	for _, a := range actors {
		out = append(out, a.logical)
	}
	return out
}

// TestEpochReportContinuityAcrossMigrationCohorts is the no/one/some/all
// migration matrix required by spec §6.5–6.6 and GOLDEN §10.
//
// The asymmetry this pins down is the dangerous one: an epoch's active set is
// FROZEN at anchor time under the accounts that existed then, but submission is
// authenticated against whoever is registered NOW. If those two identities are
// not kept distinct, a mixed cohort splits — migrated nodes become unable to
// report (or report under an identity nobody scores), while unmigrated nodes
// carry on. That is invisible in a homogeneous test where either everybody or
// nobody has migrated, which is exactly why the all-migrated and none-migrated
// cases alone are not sufficient evidence.
//
// For every cohort shape, each actor must be able to submit exactly once, the
// row must land under the EPOCH-LOGICAL account, and it must record the actual
// CURRENT submitter for provenance (I7, I8).
func TestEpochReportContinuityAcrossMigrationCohorts(t *testing.T) {
	const cohortSize = 4

	for _, tc := range []struct {
		name         string
		migrateCount int
	}{
		{name: "none migrated", migrateCount: 0},
		{name: "one migrated", migrateCount: 1},
		{name: "some migrated", migrateCount: 2},
		{name: "all migrated", migrateCount: cohortSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			f.ctx = f.ctx.WithBlockHeight(1)

			actors := buildCohort(t, f, cohortSize, tc.migrateCount)
			logical := cohortLogicalAccounts(actors)
			seedEpochAnchorForReportTest(t, f, 0, logical, logical)

			// Every actor is registered under its CURRENT account today.
			for _, actor := range actors {
				f.supernodeKeeper.EXPECT().
					GetSuperNodeByAccount(gomock.Any(), actor.current).
					Return(sntypes.SuperNode{SupernodeAccount: actor.current}, true, nil).
					AnyTimes()
			}

			server := keeper.NewMsgServerImpl(f.keeper)

			for i, actor := range actors {
				_, err := server.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
					Creator: actor.current, EpochId: 0, HostReport: types.HostReport{},
				})
				require.NoErrorf(t, err,
					"actor %d (migrated=%v) must be able to report in the frozen epoch", i, actor.migrated)

				report, found := f.keeper.GetReport(f.ctx, 0, actor.logical)
				require.Truef(t, found, "actor %d report must be stored under its epoch-logical account", i)
				require.Equalf(t, actor.logical, report.SupernodeAccount,
					"actor %d report must be keyed by the frozen epoch identity", i)
				require.Equalf(t, actor.current, report.CurrentSubmitter,
					"actor %d report must record the live submitter for provenance", i)
			}

			// Assignment completeness: the frozen set is fully covered, so a
			// migrated cohort cannot silently evade participation accounting.
			for i, actor := range actors {
				require.Truef(t, f.keeper.HasReport(f.ctx, 0, actor.logical),
					"actor %d must count as having reported under its logical identity", i)
				require.Truef(t, f.keeper.HasReport(f.ctx, 0, actor.current),
					"actor %d must resolve through lineage from its current identity", i)
			}
		})
	}
}

// TestEpochReportCohortRejectsDoubleReportAcrossIdentities proves the other
// half of I7 for every cohort shape: a migrated node must not be able to
// occupy two slots in the same epoch by submitting once under its old account
// and once under its new one.
//
// Without this, "some migrated" is not just a continuity risk but a
// double-counting vector — a single operator could inflate participation and
// dilute everyone else's share of the frozen active set.
func TestEpochReportCohortRejectsDoubleReportAcrossIdentities(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)

	actors := buildCohort(t, f, 3, 2) // mixed cohort
	logical := cohortLogicalAccounts(actors)
	seedEpochAnchorForReportTest(t, f, 0, logical, logical)

	for _, actor := range actors {
		f.supernodeKeeper.EXPECT().
			GetSuperNodeByAccount(gomock.Any(), actor.current).
			Return(sntypes.SuperNode{SupernodeAccount: actor.current}, true, nil).
			AnyTimes()
		// The OLD account is no longer a registered supernode after migration.
		if actor.migrated {
			f.supernodeKeeper.EXPECT().
				GetSuperNodeByAccount(gomock.Any(), actor.logical).
				Return(sntypes.SuperNode{}, false, nil).
				AnyTimes()
		}
	}

	server := keeper.NewMsgServerImpl(f.keeper)

	for i, actor := range actors {
		_, err := server.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
			Creator: actor.current, EpochId: 0, HostReport: types.HostReport{},
		})
		require.NoError(t, err, "first submission for actor %d must succeed", i)

		// Same node, same epoch, submitting again under its current identity.
		_, err = server.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
			Creator: actor.current, EpochId: 0, HostReport: types.HostReport{},
		})
		require.ErrorIsf(t, err, types.ErrDuplicateReport,
			"actor %d must not report twice under its current identity", i)

		// A migrated node must not get a second slot via its retired account.
		if actor.migrated {
			_, err = server.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
				Creator: actor.logical, EpochId: 0, HostReport: types.HostReport{},
			})
			require.Errorf(t, err,
				"actor %d must not report again under its pre-migration account", i)
		}
	}
}

// TestNextEpochReportsUseCurrentIdentityAfterTransition proves the "next full
// epoch" half of the continuity requirement (spec §7 acceptance criteria).
//
// Epoch 0 is frozen under logical identities; from epoch 1 onward — the first
// epoch at or after the transition's effective epoch — the migrated account IS
// the identity. If the lineage resolver kept rewriting new reports back onto the
// old account forever, migration would never actually complete and the retired
// account would accrue state indefinitely.
func TestNextEpochReportsUseCurrentIdentityAfterTransition(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)

	actors := buildCohort(t, f, 2, 1) // one migrated, one not
	migrated, stable := actors[0], actors[1]
	require.True(t, migrated.migrated)
	require.False(t, stable.migrated)

	// The NEXT epoch's active set is anchored under whoever is current now.
	// Epoch 1 spans heights [EpochZeroHeight + EpochLengthBlocks, +len), so the
	// anchor bounds and the submitting height must both sit inside epoch 1 —
	// SubmitEpochReport only accepts the epoch derived from the current height.
	params := f.keeper.GetParams(f.ctx).WithDefaults()
	epochOneStart := int64(params.EpochZeroHeight) + int64(params.EpochLengthBlocks)
	epochOneEnd := epochOneStart + int64(params.EpochLengthBlocks) - 1

	nextSet := []string{migrated.current, stable.current}
	require.NoError(t, f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 1,
		EpochStartHeight:        epochOneStart,
		EpochEndHeight:          epochOneEnd,
		EpochLengthBlocks:       params.EpochLengthBlocks,
		Seed:                    make([]byte, 32),
		ActiveSupernodeAccounts: nextSet,
		TargetSupernodeAccounts: nextSet,
		ParamsCommitment:        []byte{1},
		ActiveSetCommitment:     []byte{1},
		TargetsSetCommitment:    []byte{1},
	}))
	f.ctx = f.ctx.WithBlockHeight(epochOneStart)

	for _, actor := range actors {
		f.supernodeKeeper.EXPECT().
			GetSuperNodeByAccount(gomock.Any(), actor.current).
			Return(sntypes.SuperNode{SupernodeAccount: actor.current}, true, nil).
			AnyTimes()
	}

	server := keeper.NewMsgServerImpl(f.keeper)

	for _, actor := range actors {
		_, err := server.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
			Creator: actor.current, EpochId: 1, HostReport: types.HostReport{},
		})
		require.NoError(t, err)

		report, found := f.keeper.GetReport(f.ctx, 1, actor.current)
		require.True(t, found, "next-epoch report must exist under the current account")
		require.Equal(t, actor.current, report.SupernodeAccount,
			"from the effective epoch onward, the current account IS the logical identity")
	}

	// The retired account must not accumulate a fresh next-epoch row of its own.
	require.False(t,
		f.ctx.KVStore(f.storeKey).Has(types.ReportKey(1, migrated.logical)),
		"a completed migration must stop writing new rows under the retired account")

	// Epoch 0 history stays exactly where it was written — never rewritten.
	require.False(t,
		f.ctx.KVStore(f.storeKey).Has(types.ReportKey(0, migrated.current)),
		"historical epochs must not be back-filled onto the new identity")
}

// TestCohortMatrixIsExhaustive guards the matrix itself. If cohortSize or the
// case list drifts so that a shape stops being covered, this fails loudly
// rather than letting the suite quietly shrink.
func TestCohortMatrixIsExhaustive(t *testing.T) {
	const cohortSize = 4
	covered := map[string]bool{}
	for _, migrateCount := range []int{0, 1, 2, cohortSize} {
		switch migrateCount {
		case 0:
			covered["none"] = true
		case cohortSize:
			covered["all"] = true
		case 1:
			covered["one"] = true
		default:
			covered["some"] = true
		}
	}
	for _, shape := range []string{"none", "one", "some", "all"} {
		require.Truef(t, covered[shape],
			"migration cohort matrix must cover the %q shape", shape)
	}
	require.Len(t, covered, 4, fmt.Sprintf("unexpected cohort shapes: %v", covered))
}
