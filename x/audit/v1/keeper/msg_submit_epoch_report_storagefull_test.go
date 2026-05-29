package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSubmitEpochReport_TransitionsReporterToStorageFullFromHostReport(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
	reporterVal := sdk.ValAddress([]byte("reporter_val_addr_20")).String()
	valAddr, err := sdk.ValAddressFromBech32(reporterVal)
	require.NoError(t, err)

	reporterSN := sntypes.SuperNode{
		ValidatorAddress: reporterVal,
		SupernodeAccount: reporter,
		States: []*sntypes.SuperNodeStateRecord{
			{State: sntypes.SuperNodeStateActive, Height: 1},
		},
	}

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(reporterSN, true, nil).
		Times(2)
	f.supernodeKeeper.EXPECT().
		GetParams(gomock.Any()).
		Return(sntypes.DefaultParams()).
		Times(1)
	f.supernodeKeeper.EXPECT().
		MarkSuperNodeStorageFull(gomock.Any(), valAddr).
		Return(nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetMetricsState(gomock.Any(), gomock.Any()).
		Return(sntypes.SupernodeMetricsState{}, false).
		AnyTimes()
	f.supernodeKeeper.EXPECT().
		SetMetricsState(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	err = f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 0,
		EpochStartHeight:        1,
		EpochEndHeight:          400,
		EpochLengthBlocks:       types.DefaultEpochLengthBlocks,
		Seed:                    make([]byte, 32),
		ActiveSupernodeAccounts: []string{reporter},
		TargetSupernodeAccounts: []string{},
		ParamsCommitment:        []byte{1},
		ActiveSetCommitment:     []byte{1},
		TargetsSetCommitment:    []byte{1},
	})
	require.NoError(t, err)

	_, err = ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			DiskUsagePercent: 95,
		},
		StorageChallengeObservations: nil,
	})
	require.NoError(t, err)
}

func TestSubmitEpochReport_DoesNotTransitionPostponedReporterToStorageFull(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)
	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := sdk.AccAddress([]byte("reporter_address_20e")).String()
	reporterVal := sdk.ValAddress([]byte("reporter_val_addr_23")).String()

	reporterSN := sntypes.SuperNode{
		ValidatorAddress: reporterVal,
		SupernodeAccount: reporter,
		States: []*sntypes.SuperNodeStateRecord{
			{State: sntypes.SuperNodeStatePostponed, Height: 1, Reason: "audit_missing_reports"},
		},
	}

	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(reporterSN, true, nil).
		Times(2)
	f.supernodeKeeper.EXPECT().
		GetParams(gomock.Any()).
		Return(sntypes.DefaultParams()).
		Times(1)
	f.supernodeKeeper.EXPECT().
		SetSuperNode(gomock.Any(), gomock.Any()).
		Times(0)
	f.supernodeKeeper.EXPECT().
		MarkSuperNodeStorageFull(gomock.Any(), gomock.Any()).
		Times(0)
	f.supernodeKeeper.EXPECT().
		GetMetricsState(gomock.Any(), gomock.Any()).
		Return(sntypes.SupernodeMetricsState{}, false).
		AnyTimes()
	f.supernodeKeeper.EXPECT().
		SetMetricsState(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	err := f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 0,
		EpochStartHeight:        1,
		EpochEndHeight:          400,
		EpochLengthBlocks:       types.DefaultEpochLengthBlocks,
		Seed:                    make([]byte, 32),
		ActiveSupernodeAccounts: []string{},
		TargetSupernodeAccounts: []string{reporter},
		ParamsCommitment:        []byte{1},
		ActiveSetCommitment:     []byte{1},
		TargetsSetCommitment:    []byte{1},
	})
	require.NoError(t, err)

	_, err = ms.SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{
		Creator: reporter,
		EpochId: 0,
		HostReport: types.HostReport{
			DiskUsagePercent: 95,
		},
		StorageChallengeObservations: nil,
	})
	require.NoError(t, err)
}
