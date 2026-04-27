package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPeerPortPostponementThresholdPercent(t *testing.T) {
	makeReports := func(t *testing.T, f *fixture, epochID uint64, target sntypes.SuperNode, peers []sntypes.SuperNode, portStateForPeer []types.PortState) {
		t.Helper()

		// Target must submit a report to avoid missing-report postponement.
		err := f.keeper.SetReport(f.ctx, types.EpochReport{
			SupernodeAccount: target.SupernodeAccount,
			EpochId:          epochID,
			ReportHeight:     f.ctx.BlockHeight(),
			HostReport:       types.HostReport{},
		})
		if err != nil {
			t.Fatalf("failed to set target report: %v", err)
		}

		for i, peer := range peers {
			err := f.keeper.SetReport(f.ctx, types.EpochReport{
				SupernodeAccount: peer.SupernodeAccount,
				EpochId:          epochID,
				ReportHeight:     f.ctx.BlockHeight(),
				HostReport:       types.HostReport{},
				StorageChallengeObservations: []*types.StorageChallengeObservation{
					{
						TargetSupernodeAccount: target.SupernodeAccount,
						PortStates:             []types.PortState{portStateForPeer[i]},
					},
				},
			})
			if err != nil {
				t.Fatalf("failed to set peer report: %v", err)
			}
			f.keeper.SetStorageChallengeReportIndex(f.ctx, target.SupernodeAccount, epochID, peer.SupernodeAccount)
		}
	}

	epochID := uint64(0)

	_, targetAcc, targetVal := cryptotestutils.SupernodeAddresses()
	target := sntypes.SuperNode{
		SupernodeAccount: targetAcc.String(),
		ValidatorAddress: sdk.ValAddress(targetVal).String(),
	}

	_, peer1Acc, _ := cryptotestutils.SupernodeAddresses()
	_, peer2Acc, _ := cryptotestutils.SupernodeAddresses()
	_, peer3Acc, _ := cryptotestutils.SupernodeAddresses()

	peers := []sntypes.SuperNode{
		{SupernodeAccount: peer1Acc.String()},
		{SupernodeAccount: peer2Acc.String()},
		{SupernodeAccount: peer3Acc.String()},
	}

	peerStates := []types.PortState{
		types.PortState_PORT_STATE_CLOSED,
		types.PortState_PORT_STATE_CLOSED,
		types.PortState_PORT_STATE_OPEN,
	}

	t.Run("threshold_100_requires_unanimous", func(t *testing.T) {
		f := initFixture(t)

		params := types.DefaultParams()
		params.RequiredOpenPorts = []uint32{4444}
		params.ConsecutiveEpochsToPostpone = 1
		params.PeerPortPostponeThresholdPercent = 100

		makeReports(t, f, epochID, target, peers, peerStates)

		f.supernodeKeeper.EXPECT().
			GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive, sntypes.SuperNodeStateStorageFull).
			Return([]sntypes.SuperNode{target}, nil).
			Times(1)
		f.supernodeKeeper.EXPECT().
			GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
			Return([]sntypes.SuperNode{}, nil).
			Times(1)
		f.supernodeKeeper.EXPECT().
			SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(targetVal), gomock.Any()).
			Times(0)

		if err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("threshold_66_allows_two_of_three", func(t *testing.T) {
		f := initFixture(t)

		params := types.DefaultParams()
		params.RequiredOpenPorts = []uint32{4444}
		params.ConsecutiveEpochsToPostpone = 1
		params.PeerPortPostponeThresholdPercent = 66

		makeReports(t, f, epochID, target, peers, peerStates)

		f.supernodeKeeper.EXPECT().
			GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive, sntypes.SuperNodeStateStorageFull).
			Return([]sntypes.SuperNode{target}, nil).
			Times(1)
		f.supernodeKeeper.EXPECT().
			GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
			Return([]sntypes.SuperNode{}, nil).
			Times(1)
		f.supernodeKeeper.EXPECT().
			SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(targetVal), "audit_peer_ports").
			Return(nil).
			Times(1)

		if err := f.keeper.EnforceEpochEnd(f.ctx, epochID, params); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestEnforceEpochEnd_EmitsStorageTruthRecoveredEvent(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())

	_, postponedAcc, postponedVal := cryptotestutils.SupernodeAddresses()
	postponed := sntypes.SuperNode{
		SupernodeAccount: postponedAcc.String(),
		ValidatorAddress: sdk.ValAddress(postponedVal).String(),
	}

	params := types.DefaultParams().WithDefaults()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_FULL

	// First epoch-end call: force storage-truth postpone and set postponed marker.
	require.NoError(t, f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{
		SupernodeAccount:  postponed.SupernodeAccount,
		SuspicionScore:    200,
		LastUpdatedEpoch:  5,
		ClassACountWindow: 2,
	}))

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{postponed}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		SetSuperNodePostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(postponedVal), "audit_storage_truth_suspicion").
		Return(nil).
		Times(1)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 5, params))

	// Second epoch-end call: score decayed below watch + clean passes => recover.
	require.NoError(t, f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{
		SupernodeAccount: postponed.SupernodeAccount,
		SuspicionScore:   1,
		LastUpdatedEpoch: 6,
		CleanPassCount:   params.StorageTruthRecoveryCleanPassCount,
	}))

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{postponed}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		RecoverSuperNodeFromPostponed(gomock.AssignableToTypeOf(f.ctx), sdk.ValAddress(postponedVal)).
		Return(nil).
		Times(1)

	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 6, params))

	events := f.ctx.EventManager().Events()
	found := false
	for _, event := range events {
		if event.Type == types.EventTypeStorageTruthRecovered {
			found = true
			break
		}
	}
	require.True(t, found, "expected storage_truth_recovered event")
}
