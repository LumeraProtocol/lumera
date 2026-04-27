package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestEnforceEpochEnd_RecoversPostponedNodeToActive verifies that a postponed node with
// a compliant peer port report is recovered to Active via RecoverSuperNodeFromPostponed.
// Per LEP-6 §17: recovery to StorageFull is no longer managed in the audit enforcement path;
// that transition is handled by the supernode module's own state machine.
func TestEnforceEpochEnd_RecoversPostponedToStorageFullWhenDiskStillHigh(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(10)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
	reporterVal := sdk.ValAddress([]byte("reporter_val_addr_20")).String()
	valAddr, err := sdk.ValAddressFromBech32(reporterVal)
	require.NoError(t, err)

	sn := sntypes.SuperNode{ValidatorAddress: reporterVal, SupernodeAccount: reporter, States: []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStatePostponed, Height: 9, Reason: "audit_missing_reports"}}}

	// Persist a compliant report for epoch 1.
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), reporter).Return(sn, true, nil).Times(1)
	f.supernodeKeeper.EXPECT().GetParams(gomock.Any()).Return(sntypes.DefaultParams()).Times(1)
	err = f.keeper.SetReport(f.ctx, types.EpochReport{SupernodeAccount: reporter, EpochId: 1, ReportHeight: f.ctx.BlockHeight(), HostReport: types.HostReport{DiskUsagePercent: 95}})
	require.NoError(t, err)

	peer := sdk.AccAddress([]byte("peer_for_recovery_____")).String()
	err = f.keeper.SetReport(f.ctx, types.EpochReport{
		SupernodeAccount: peer,
		EpochId:          1,
		ReportHeight:     f.ctx.BlockHeight(),
		HostReport:       types.HostReport{},
		StorageChallengeObservations: []*types.StorageChallengeObservation{{
			TargetSupernodeAccount: reporter,
			PortStates:             []types.PortState{types.PortState_PORT_STATE_OPEN},
		}},
	})
	require.NoError(t, err)
	f.keeper.SetStorageChallengeReportIndex(f.ctx, reporter, 1, peer)

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.MinCpuFreePercent = 0
	params.MinMemFreePercent = 0
	params.MinDiskFreePercent = 0 // disk-pressure postponement not active in audit module

	// Per LEP-6 §17: audit EnforceEpochEnd only queries Active nodes (not StorageFull).
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().RecoverSuperNodeFromPostponed(gomock.AssignableToTypeOf(f.ctx), valAddr).Return(nil).Times(1)

	err = f.keeper.EnforceEpochEnd(f.ctx, 1, params)
	require.NoError(t, err)
}

func TestEnforceEpochEnd_RecoversPostponedToActiveWhenDiskBelowThreshold(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(10)

	reporter := sdk.AccAddress([]byte("reporter_address_20c")).String()
	reporterVal := sdk.ValAddress([]byte("reporter_val_addr_21")).String()
	valAddr, err := sdk.ValAddressFromBech32(reporterVal)
	require.NoError(t, err)

	sn := sntypes.SuperNode{ValidatorAddress: reporterVal, SupernodeAccount: reporter, States: []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStatePostponed, Height: 9, Reason: "audit_missing_reports"}}}

	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), reporter).Return(sn, true, nil).Times(1)
	f.supernodeKeeper.EXPECT().GetParams(gomock.Any()).Return(sntypes.DefaultParams()).Times(1)
	err = f.keeper.SetReport(f.ctx, types.EpochReport{SupernodeAccount: reporter, EpochId: 1, ReportHeight: f.ctx.BlockHeight(), HostReport: types.HostReport{DiskUsagePercent: 40}})
	require.NoError(t, err)

	peer := sdk.AccAddress([]byte("peer_for_recovery_____")).String()
	err = f.keeper.SetReport(f.ctx, types.EpochReport{
		SupernodeAccount: peer,
		EpochId:          1,
		ReportHeight:     f.ctx.BlockHeight(),
		HostReport:       types.HostReport{},
		StorageChallengeObservations: []*types.StorageChallengeObservation{{
			TargetSupernodeAccount: reporter,
			PortStates:             []types.PortState{types.PortState_PORT_STATE_OPEN},
		}},
	})
	require.NoError(t, err)
	f.keeper.SetStorageChallengeReportIndex(f.ctx, reporter, 1, peer)

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.MinCpuFreePercent = 0
	params.MinMemFreePercent = 0

	// Per LEP-6 §17: audit EnforceEpochEnd only queries Active nodes (not StorageFull).
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().RecoverSuperNodeFromPostponed(gomock.AssignableToTypeOf(f.ctx), valAddr).Return(nil).Times(1)

	err = f.keeper.EnforceEpochEnd(f.ctx, 1, params)
	require.NoError(t, err)
}

// TestEnforceEpochEnd_DiskPressureDoesNotPostponeStorageFull verifies that StorageFull nodes
// are not evaluated or postponed by the audit enforcement path (per LEP-6 §17 which limits
// audit enforcement to Active nodes only).
func TestEnforceEpochEnd_DiskPressureDoesNotPostponeStorageFull(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(10)

	reporter := sdk.AccAddress([]byte("reporter_address_20d")).String()
	reporterVal := sdk.ValAddress([]byte("reporter_val_addr_22")).String()

	sn := sntypes.SuperNode{ValidatorAddress: reporterVal, SupernodeAccount: reporter, States: []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateStorageFull, Height: 9}}}

	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), reporter).Return(sn, true, nil).Times(1)
	f.supernodeKeeper.EXPECT().GetParams(gomock.Any()).Return(sntypes.DefaultParams()).Times(1)
	err := f.keeper.SetReport(f.ctx, types.EpochReport{SupernodeAccount: reporter, EpochId: 1, ReportHeight: f.ctx.BlockHeight(), HostReport: types.HostReport{DiskUsagePercent: 95}})
	require.NoError(t, err)

	params := types.DefaultParams()
	params.RequiredOpenPorts = []uint32{4444}
	params.MinCpuFreePercent = 0
	params.MinMemFreePercent = 0
	params.MinDiskFreePercent = 100

	// Per LEP-6 §17: audit EnforceEpochEnd queries only Active nodes; StorageFull nodes
	// are not evaluated for postponement in the audit module.
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil). // StorageFull node is not returned here
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), gomock.Any(), gomock.Any()).Times(0)

	err = f.keeper.EnforceEpochEnd(f.ctx, 1, params)
	require.NoError(t, err)
}
