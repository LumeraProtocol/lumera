package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/testutil/sample"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// distributeFeesFixture builds an action keeper wired with a configurable
// RewardDistributionKeeper (bps) and registers a Done cascade action whose
// price is the given fee amount of ulume distributed to a single supernode.
type distributeFeesFixture struct {
	actionID   string
	supernode  string
	fee        sdk.Coin
	bankKeeper *keepertest.ActionBankKeeper
	keeperCtx  sdk.Context
	keeper     interface {
		SetAction(ctx sdk.Context, action *actiontypes.Action) error
		DistributeFees(ctx sdk.Context, actionID string) error
	}
}

func setupDistributeFees(t *testing.T, bps uint64, feeAmount int64) *distributeFeesFixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	rewardKeeper := &keepertest.MockRewardDistributionKeeper{Bps: bps}
	k, ctx, bank := keepertest.ActionKeeperWithRewardDistribution(t, ctrl, nil, rewardKeeper)

	supernode := sample.AccAddress()
	fee := sdk.NewInt64Coin("ulume", feeAmount)
	action := &actiontypes.Action{
		Creator:        sample.AccAddress(),
		ActionID:       "action-distfees-1",
		ActionType:     actiontypes.ActionTypeCascade,
		Metadata:       []byte("metadata"),
		Price:          fee.String(),
		ExpirationTime: 1234567890,
		BlockHeight:    10,
		State:          actiontypes.ActionStateDone,
		SuperNodes:     []string{supernode},
	}
	require.NoError(t, k.SetAction(ctx, action))

	return &distributeFeesFixture{
		actionID:   action.ActionID,
		supernode:  supernode,
		fee:        fee,
		bankKeeper: bank,
		keeperCtx:  ctx,
		keeper:     &k,
	}
}

// TestDistributeFees_RoutesRegistrationFeeShareToSupernodePool asserts that
// the configured RegistrationFeeShareBps slice of every cascade fee is routed
// to the supernode-owned reward pool (sntypes.ModuleName) before the
// foundation share + per-supernode payout happen.
func TestDistributeFees_RoutesRegistrationFeeShareToSupernodePool(t *testing.T) {
	const (
		feeAmount int64  = 1000
		bps       uint64 = 200 // 2%
	)
	f := setupDistributeFees(t, bps, feeAmount)

	preSupernodePool := f.bankKeeper.GetModuleBalance(sntypes.ModuleName)

	require.NoError(t, f.keeper.DistributeFees(f.keeperCtx, f.actionID))

	postSupernodePool := f.bankKeeper.GetModuleBalance(sntypes.ModuleName)

	expectedReward := feeAmount * int64(bps) / 10000 // 20
	delta := postSupernodePool.AmountOf("ulume").Sub(preSupernodePool.AmountOf("ulume"))
	require.Equal(t, expectedReward, delta.Int64(),
		"reward-pool delta must equal fee*bps/10000")

	// Foundation share is 10% (FoundationFeeShare=0.1) of the post-reward fee.
	postRewardFee := feeAmount - expectedReward // 980
	expectedFoundation := postRewardFee / 10    // 98
	expectedSupernodePayout := postRewardFee - expectedFoundation

	addr, err := sdk.AccAddressFromBech32(f.supernode)
	require.NoError(t, err)
	got := f.bankKeeper.GetAccountCoins(addr).AmountOf("ulume").Int64()
	require.Equal(t, expectedSupernodePayout, got,
		"supernode receives fee minus reward-share minus foundation-share")
}

// TestDistributeFees_BpsZero_NoModuleToModuleTransfer asserts that with bps=0
// the supernode reward pool balance is unchanged (no module-to-module xfer).
func TestDistributeFees_BpsZero_NoModuleToModuleTransfer(t *testing.T) {
	f := setupDistributeFees(t, 0, 1000)

	pre := f.bankKeeper.GetModuleBalance(sntypes.ModuleName)
	require.NoError(t, f.keeper.DistributeFees(f.keeperCtx, f.actionID))
	post := f.bankKeeper.GetModuleBalance(sntypes.ModuleName)

	require.True(t, pre.Equal(post),
		"sntypes.ModuleName balance must be unchanged when bps=0 (got pre=%s post=%s)",
		pre.String(), post.String())
}

// TestDistributeFees_BpsRouting_AmountTruncation asserts integer truncation
// of the reward-share calc: fee*bps/10000 truncates fractional ulume.
// fee=1003, bps=200 -> 1003*200=200600 / 10000 = 20 (0.06 truncated).
func TestDistributeFees_BpsRouting_AmountTruncation(t *testing.T) {
	const (
		feeAmount int64  = 1003
		bps       uint64 = 200
	)
	f := setupDistributeFees(t, bps, feeAmount)

	require.NoError(t, f.keeper.DistributeFees(f.keeperCtx, f.actionID))

	post := f.bankKeeper.GetModuleBalance(sntypes.ModuleName)
	got := post.AmountOf("ulume").Int64()
	expected := feeAmount * int64(bps) / 10000 // 20, NOT 21
	require.Equal(t, int64(20), expected, "sanity: arithmetic constant")
	require.Equal(t, expected, got,
		"reward-share must be truncated, not rounded")
}
