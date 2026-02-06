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

func TestSubmitAuditReport_ValidatesInboundPortStatesLength(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)

	ms := keeper.NewMsgServerImpl(f.keeper)

	reporter := sdk.AccAddress([]byte("reporter_address_20b")).String()
	active := sdk.AccAddress([]byte("active_address__20b")).String()

	// Reporter exists on-chain as a supernode, but is not necessarily ACTIVE at epoch start.
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{}, true, nil).
		AnyTimes()

	// Seeded epoch anchor for epoch 0 (content not important for this test beyond existence).
	err := f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 0,
		EpochStartHeight:        1,
		EpochEndHeight:          400,
		EpochLengthBlocks:       types.DefaultEpochLengthBlocks,
		Seed:                    make([]byte, 32),
		ActiveSupernodeAccounts: []string{active},
		TargetSupernodeAccounts: []string{active},
		ParamsCommitment:        []byte{1},
		ActiveSetCommitment:     []byte{1},
		TargetsSetCommitment:    []byte{1},
	})
	require.NoError(t, err)

	requiredPortsLen := len(types.DefaultRequiredOpenPorts)
	require.Greater(t, requiredPortsLen, 0)

	// Empty inbound_port_states is allowed (unknown/unreported).
	_, err = ms.SubmitAuditReport(f.ctx, &types.MsgSubmitAuditReport{
		SupernodeAccount: reporter,
		EpochId:          0,
		SelfReport:       types.AuditSelfReport{},
		PeerObservations: nil,
	})
	require.NoError(t, err)

	// Partial inbound_port_states is rejected.
	_, err = ms.SubmitAuditReport(f.ctx, &types.MsgSubmitAuditReport{
		SupernodeAccount: reporter,
		EpochId:          0,
		SelfReport: types.AuditSelfReport{
			InboundPortStates: []types.PortState{types.PortState_PORT_STATE_OPEN},
		},
		PeerObservations: nil,
	})
	require.Error(t, err)

	// Oversized inbound_port_states is rejected.
	oversized := make([]types.PortState, requiredPortsLen+1)
	_, err = ms.SubmitAuditReport(f.ctx, &types.MsgSubmitAuditReport{
		SupernodeAccount: reporter,
		EpochId:          0,
		SelfReport: types.AuditSelfReport{
			InboundPortStates: oversized,
		},
		PeerObservations: nil,
	})
	require.Error(t, err)
}

