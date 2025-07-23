package ibctesting_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/LumeraProtocol/lumera/tests/ibctesting"
)

// TestChangeValSet tests that the IBC client on a counterparty chain (chainB) can update
// successfully after a validator set change on chainA.
func TestChangeValSet(t *testing.T) {
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))

	path := ibctesting.NewPath(chainA, chainB)
	path.Setup()

	// define two large staking amounts for delegation to influence the validator set
	amount, ok := sdkmath.NewIntFromString("10000000000000000000")
	require.True(t, ok)
	amount2, ok := sdkmath.NewIntFromString("30000000000000000000")
	require.True(t, ok)

	// fetch the top 4 validators on chainA
	val, err := chainA.GetLumeraApp().StakingKeeper.GetValidators(chainA.GetContext(), 4)
	require.NoError(t, err)

	// delegate the amounts to two different validators, likely modifying their power in the validator set.
	chainA.GetLumeraApp().StakingKeeper.Delegate(chainA.GetContext(), chainA.SenderAccounts[1].SenderAccount.GetAddress(), //nolint:errcheck // ignore error for test
		amount, types.Unbonded, val[1], true)
	chainA.GetLumeraApp().StakingKeeper.Delegate(chainA.GetContext(), chainA.SenderAccounts[3].SenderAccount.GetAddress(), //nolint:errcheck // ignore error for test
		amount2, types.Unbonded, val[3], true)

	coord.CommitBlock(chainA)

	// verify that update clients works even after validator update goes into effect
	err = path.EndpointB.UpdateClient()
	require.NoError(t, err)
	err = path.EndpointB.UpdateClient()
	require.NoError(t, err)
}

// TestJailProposerValidator tests how the system behaves when a proposer validator 
// (the one selected to propose a block) is jailed. Checks if:
// 1. The validator is actually removed from the active validator set.
// 2. The next block is proposed by a different validator (new proposer).
// 3. The IBC client can still update successfully after the jailing.
func TestJailProposerValidator(t *testing.T) {
	coord := ibctesting.NewCoordinator(t, 2)
	chainA := coord.GetChain(ibctesting.GetChainID(1))
	chainB := coord.GetChain(ibctesting.GetChainID(2))

	path := ibctesting.NewPath(chainA, chainB)
	path.Setup()

	// save valset length before jailing
	valsetLen := len(chainA.Vals.Validators)

	// jail the proposer validator in chain A
	propAddr := sdk.ConsAddress(chainA.Vals.Proposer.Address)

	err := chainA.GetLumeraApp().StakingKeeper.Jail(chainA.GetContext(), propAddr)
	require.NoError(t, err)

	coord.CommitBlock(chainA)

	// verify that update clients works even after validator update goes into effect
	err = path.EndpointB.UpdateClient()
	require.NoError(t, err)
	err = path.EndpointB.UpdateClient()
	require.NoError(t, err)

	// check that the jailing has taken effect in chain A
	require.Equal(t, valsetLen-1, len(chainA.Vals.Validators))

	// check that the valset in chain A has a new proposer
	require.False(t, propAddr.Equals(sdk.ConsAddress(chainA.Vals.Proposer.Address)))
}
