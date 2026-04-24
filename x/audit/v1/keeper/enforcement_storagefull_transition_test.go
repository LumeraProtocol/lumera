package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEnforceEpochEnd_RecoversPostponedToStorageFullWhenDiskStillHigh(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(10)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
	reporterVal := sdk.ValAddress([]byte("reporter_val_addr_20")).String()
	valAddr, err := sdk.ValAddressFromBech32(reporterVal)
	require.NoError(t, err)

	sn := sntypes.SuperNode{ValidatorAddress: reporterVal, SupernodeAccount: reporter, States: []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStatePostponed, Height: 9, Reason: "audit_missing_reports"}}}

	// Persist a compliant report with high disk usage for epoch 1.
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
	params.MinDiskFreePercent = 100 // must not block recovery path

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive, sntypes.SuperNodeStateStorageFull).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().GetParams(gomock.Any()).Return(sntypes.DefaultParams()).Times(1)
	f.supernodeKeeper.EXPECT().QuerySuperNode(gomock.AssignableToTypeOf(f.ctx), valAddr).Return(sn, true).Times(1)
	f.supernodeKeeper.EXPECT().SetSuperNode(gomock.AssignableToTypeOf(f.ctx), gomock.Any()).DoAndReturn(
		func(_ sdk.Context, updated sntypes.SuperNode) error {
			require.NotEmpty(t, updated.States)
			require.Equal(t, sntypes.SuperNodeStateStorageFull, updated.States[len(updated.States)-1].State)
			return nil
		},
	).Times(1)

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

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive, sntypes.SuperNodeStateStorageFull).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{sn}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().GetParams(gomock.Any()).Return(sntypes.DefaultParams()).Times(1)
	f.supernodeKeeper.EXPECT().RecoverSuperNodeFromPostponed(gomock.AssignableToTypeOf(f.ctx), valAddr).Return(nil).Times(1)

	err = f.keeper.EnforceEpochEnd(f.ctx, 1, params)
	require.NoError(t, err)
}

func TestEnforceEpochEnd_DiskPressureDoesNotPostponeStorageFull(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(10)

	reporter := sdk.AccAddress([]byte("reporter_address_20d")).String()
	reporterVal := sdk.ValAddress([]byte("reporter_val_addr_22")).String()
	valAddr, err := sdk.ValAddressFromBech32(reporterVal)
	require.NoError(t, err)

	sn := sntypes.SuperNode{ValidatorAddress: reporterVal, SupernodeAccount: reporter, States: []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateStorageFull, Height: 9}}}

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
	params.MinDiskFreePercent = 100

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive, sntypes.SuperNodeStateStorageFull).
		Return([]sntypes.SuperNode{sn}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), valAddr, gomock.Any()).Times(0)

	err = f.keeper.EnforceEpochEnd(f.ctx, 1, params)
	require.NoError(t, err)
}
