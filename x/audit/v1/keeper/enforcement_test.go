package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/mock/gomock"
)

func TestPeerPortPostponementThresholdPercent(t *testing.T) {
	makeReports := func(t *testing.T, f *fixture, epochID uint64, target sntypes.SuperNode, peers []sntypes.SuperNode, portStateForPeer []types.PortState) {
		t.Helper()

		// Target must submit a report to avoid missing-report postponement.
		err := f.keeper.SetReport(f.ctx, types.AuditReport{
			SupernodeAccount: target.SupernodeAccount,
			EpochId:          epochID,
			ReportHeight:     f.ctx.BlockHeight(),
			SelfReport:       types.AuditSelfReport{},
		})
		if err != nil {
			t.Fatalf("failed to set target report: %v", err)
		}

		for i, peer := range peers {
			err := f.keeper.SetReport(f.ctx, types.AuditReport{
				SupernodeAccount: peer.SupernodeAccount,
				EpochId:          epochID,
				ReportHeight:     f.ctx.BlockHeight(),
				SelfReport:       types.AuditSelfReport{},
				PeerObservations: []*types.AuditPeerObservation{
					{
						TargetSupernodeAccount: target.SupernodeAccount,
						PortStates:             []types.PortState{portStateForPeer[i]},
					},
				},
			})
			if err != nil {
				t.Fatalf("failed to set peer report: %v", err)
			}
			f.keeper.SetSupernodeReportIndex(f.ctx, target.SupernodeAccount, epochID, peer.SupernodeAccount)
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
			GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
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
			GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive).
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
