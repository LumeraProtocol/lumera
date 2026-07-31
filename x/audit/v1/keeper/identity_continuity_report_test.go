package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSubmitEpochReportUsesEpochLogicalReporterAndCurrentSubmitter(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)
	old := testAddress(t, f, []byte{11, 12, 13, 14})
	current := testAddress(t, f, []byte{21, 22, 23, 24})
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
		SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 1,
	}))
	seedEpochAnchorForReportTest(t, f, 0, []string{old}, []string{old})
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), current).
		Return(sntypes.SuperNode{SupernodeAccount: current}, true, nil).AnyTimes()

	_, err := keeper.NewMsgServerImpl(f.keeper).SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: current, EpochId: 0, HostReport: types.HostReport{},
	})
	require.NoError(t, err)
	report, found := f.keeper.GetReport(f.ctx, 0, old)
	require.True(t, found)
	require.Equal(t, old, report.SupernodeAccount)
	require.Equal(t, current, report.CurrentSubmitter)
	require.True(t, f.keeper.HasReport(f.ctx, 0, old))
	require.True(t, f.keeper.HasReport(f.ctx, 0, current), "lineage-aware reads find the epoch-logical row")
	require.False(t, f.ctx.KVStore(f.storeKey).Has(types.ReportKey(0, current)), "historical row is not rewritten")

	_, err = keeper.NewMsgServerImpl(f.keeper).SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: current, EpochId: 0, HostReport: types.HostReport{},
	})
	require.ErrorIs(t, err, types.ErrDuplicateReport)
}

func TestHistoricalReportReadFollowsLineageWithoutRewriting(t *testing.T) {
	f := initFixture(t)
	old := testAddress(t, f, []byte{111, 112, 113, 114})
	current := testAddress(t, f, []byte{121, 122, 123, 124})
	require.NoError(t, f.keeper.SetReportRaw(f.ctx, types.EpochReport{EpochId: 2, SupernodeAccount: old}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
		SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 3,
	}))

	report, found := f.keeper.GetReport(f.ctx, 2, current)
	require.True(t, found)
	require.Equal(t, old, report.SupernodeAccount)
	_, rawStillPresent := f.keeper.GetReport(f.ctx, 3, old)
	require.False(t, rawStillPresent)
}

func TestReportBeforeTransitionThenCurrentDuplicateDoesNotWrite(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)
	old := testAddress(t, f, []byte{71, 72, 73, 74})
	current := testAddress(t, f, []byte{75, 76, 77, 78})
	require.NoError(t, f.keeper.SetReportRaw(f.ctx, types.EpochReport{EpochId: 0, SupernodeAccount: old, HostReport: types.HostReport{DiskUsagePercent: 17}}))
	seedEpochAnchorForReportTest(t, f, 0, []string{old}, []string{old})
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 1}))
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), current).Return(sntypes.SuperNode{SupernodeAccount: current}, true, nil).AnyTimes()
	before := auditStoreSnapshot(f)
	_, err := keeper.NewMsgServerImpl(f.keeper).SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{Creator: current, EpochId: 0})
	require.ErrorIs(t, err, types.ErrDuplicateReport)
	require.Equal(t, before, auditStoreSnapshot(f))
	report, found := f.keeper.GetReport(f.ctx, 0, current)
	require.True(t, found)
	require.Equal(t, float64(17), report.HostReport.DiskUsagePercent)
	require.Empty(t, report.CurrentSubmitter)
	require.False(t, f.ctx.KVStore(f.storeKey).Has(types.ReportKey(0, current)))
}
