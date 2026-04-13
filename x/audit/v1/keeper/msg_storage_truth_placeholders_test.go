package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestMsgSubmitStorageRecheckEvidencePlaceholder(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.SubmitStorageRecheckEvidence(f.ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty request")

	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "creator is required")

	_, err = ms.SubmitStorageRecheckEvidence(f.ctx, &types.MsgSubmitStorageRecheckEvidence{
		Creator:                    "lumera1creator111111111111111111111111r0jv6",
		EpochId:                    1,
		ChallengedSupernodeAccount: "lumera1subject111111111111111111111111f4pnj",
		TicketId:                   "ticket-1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrNotImplemented.Error())
}

func TestMsgClaimHealCompletePlaceholder(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.ClaimHealComplete(f.ctx, &types.MsgClaimHealComplete{
		Creator:  "lumera1creator222222222222222222222222jhx4s",
		HealOpId: 3,
		TicketId: "ticket-3",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrNotImplemented.Error())
}

func TestMsgSubmitHealVerificationPlaceholder(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.SubmitHealVerification(f.ctx, &types.MsgSubmitHealVerification{
		Creator:  "lumera1creator3333333333333333333333333v56r",
		HealOpId: 7,
		Verified: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrNotImplemented.Error())
}
