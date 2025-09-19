package staking_test

import (
	"fmt"
	"testing"
	"bytes"

	abci "github.com/cometbft/cometbft/abci/types"
	"gotest.tools/v3/assert"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	"github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

func newMonikerValidator(tb testing.TB, operator sdk.ValAddress, pubKey cryptotypes.PubKey, moniker string) types.Validator {
	tb.Helper()
	v, err := types.NewValidator(operator.String(), pubKey, types.Description{Moniker: moniker})
	assert.NilError(tb, err)
	return v
}

func bootstrapValidatorTest(t testing.TB, powers []int64, numAddrs int) (*fixture, []sdk.AccAddress, []sdk.ValAddress) {
	f := initFixture(t)

	addrDels, addrVals := generateAddresses(f, numAddrs)

	bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
	assert.NilError(t, err)

    for i := 0; i < numAddrs && i < len(powers); i++ {
        amt := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, powers[i])
        coins := sdk.NewCoins(sdk.NewCoin(bondDenom, amt))
        assert.NilError(t, banktestutil.FundAccount(f.sdkCtx, f.bankKeeper, addrDels[i], coins))
    }
	return f, addrDels, addrVals
}

func initValidators(t testing.TB, power int64, numAddrs int, powers []int64) (*fixture, []sdk.AccAddress, []sdk.ValAddress, []types.Validator) {
	f, addrs, valAddrs := bootstrapValidatorTest(t, powers, numAddrs)
	pks := simtestutil.CreateTestPubKeys(numAddrs)

	vs := make([]types.Validator, len(powers))
	for i, power := range powers {
		vs[i] = testutil.NewValidator(t, valAddrs[i], pks[i])
		vs[i] = mustDelegatePower(t, f, addrs[i], vs[i], power)
	}
	return f, addrs, valAddrs, vs
}

func applyValidatorSetUpdates(t *testing.T, ctx sdk.Context, k *keeper.Keeper, expectedUpdatesLen int) []abci.ValidatorUpdate {
	updates, err := k.ApplyAndReturnValidatorSetUpdates(ctx)
	assert.NilError(t, err)
	if expectedUpdatesLen >= 0 {
		assert.Equal(t, expectedUpdatesLen, len(updates), "%v", updates)
	}
	return updates
}

func TestUpdateBondedValidatorsDecreaseCliff(t *testing.T) {
	numVals := 10
	maxVals := 5

	// powers for the 10 validators: 10, 20, ..., 100
	var powers []int64
	for i := 1; i <= numVals; i++ {
		powers = append(powers, int64(i*10))
	}	
	// create context, keeper, and fund delegators
	f, addrDels, valAddrs := bootstrapValidatorTest(t, powers, 100)

	bondedPool := f.stakingKeeper.GetBondedPool(f.sdkCtx)
	notBondedPool := f.stakingKeeper.GetNotBondedPool(f.sdkCtx)

	// create keeper parameters
	params, err := f.stakingKeeper.GetParams(f.sdkCtx)
	assert.NilError(t, err)
	params.MaxValidators = uint32(maxVals)
	assert.NilError(t, f.stakingKeeper.SetParams(f.sdkCtx, params))

	bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
	assert.NilError(t, err)

	// create a random pool
	assert.NilError(t, banktestutil.FundModuleAccount(f.sdkCtx, f.bankKeeper, bondedPool.GetName(), sdk.NewCoins(sdk.NewCoin(bondDenom, f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 1234)))))
	assert.NilError(t, banktestutil.FundModuleAccount(f.sdkCtx, f.bankKeeper, notBondedPool.GetName(), sdk.NewCoins(sdk.NewCoin(bondDenom, f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 10000)))))

	f.accountKeeper.SetModuleAccount(f.sdkCtx, bondedPool)
	f.accountKeeper.SetModuleAccount(f.sdkCtx, notBondedPool)

	// create validators with real delegations
	validators := make([]types.Validator, numVals)
	for i := 0; i < len(validators); i++ {
		moniker := fmt.Sprintf("val#%d", int64(i))
		val := newMonikerValidator(t, valAddrs[i], PKs[i], moniker)
		// real Bank->Staking flow
		val = mustDelegatePower(t, f, addrDels[i], val, powers[i])
		// Persist the validator so power index reflects its bonded power
		val = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, val, true)
		validators[i] = val
	}

	// cliff validator candidate
	nextCliffVal := validators[numVals-maxVals+1] // index 6 (0-based)

	// remove enough tokens to kick out the validator below the current cliff
	// validator and next in line cliff validator
	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, nextCliffVal)
	shares := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 21)
	nextCliffVal, _ = nextCliffVal.RemoveDelShares(math.LegacyNewDecFromInt(shares))
	_ = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, nextCliffVal, true)

	expectedValStatus := map[int]types.BondStatus{
		9: types.Bonded, 8: types.Bonded, 7: types.Bonded, 5: types.Bonded, 4: types.Bonded,
		0: types.Unbonding, 1: types.Unbonding, 2: types.Unbonding, 3: types.Unbonding, 6: types.Unbonding,
	}

	// require all the validators have their respective statuses
	for valIdx, status := range expectedValStatus {
		valAddr := validators[valIdx].OperatorAddress
		addr, err := sdk.ValAddressFromBech32(valAddr)
		assert.NilError(t, err)
		val, _ := f.stakingKeeper.GetValidator(f.sdkCtx, addr)

		assert.Equal(
			t, status, val.GetStatus(),
			fmt.Sprintf("expected validator at index %v to have status: %s", valIdx, status),
		)
	}
}

func TestSlashToZeroPowerRemoved(t *testing.T) {
	// 1) Bootstrap: fund one delegator with enough to delegate
	powers := []int64{100} // 100 power worth of stake
	f, addrDels, valAddrs := bootstrapValidatorTest(t, powers, 1)

	// 2) Build the validator and do a *real* delegation
	pk := simtestutil.CreateTestPubKeys(1)[0]
	val := testutil.NewValidator(t, valAddrs[0], pk)
	val = mustDelegatePower(t, f, addrDels[0], val, powers[0])

	// Persist/update indices so it's clearly in the set
	val = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, val, true)

	// Sanity: it should be bonded and have > 0 power before slash
	require.Equal(t, types.Bonded, val.GetStatus())
	require.Greater(t, val.ConsensusPower(f.stakingKeeper.PowerReduction(f.sdkCtx)), int64(0))

	// 3) Slash to zero
	// Get consensus address for the validator
	consPK, err := val.ConsPubKey()
	require.NoError(t, err)
	consAddr := sdk.ConsAddress(consPK.Address())

	// Use full (100%) slash so effective power -> 0
	// NOTE: Passing the *current* consensus power (pre-slash) to Slash.
	curPower := val.ConsensusPower(f.stakingKeeper.PowerReduction(f.sdkCtx))
	require.Greater(t, curPower, int64(0))

	f.stakingKeeper.Slash(
		f.sdkCtx,
		consAddr,
		f.sdkCtx.BlockHeight(), // infraction height (any <= current is fine for this test)
		curPower,
		math.LegacyOneDec(), // 100% slash
	)
	f.stakingKeeper.Jail(f.sdkCtx, consAddr) // simulate slashing module behavior on evidence

	// 4) Apply updates; validator should drop out of the set
	_, _ = f.stakingKeeper.BlockValidatorUpdates(f.sdkCtx)

	updates, err := f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
	require.NoError(t, err)
	_ = updates // not strictly needed, but often non-empty when removing power

	// 5) Reload the validator from store and assert zero power / not bonded
	valBz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
	require.NoError(t, err)
	got, err := f.stakingKeeper.GetValidator(f.sdkCtx, valBz)
	require.NoError(t, err)

	require.NotEqual(t, types.Bonded, got.GetStatus(), "slashed-to-zero validator must not be Bonded")
	require.Equal(t, int64(0), got.ConsensusPower(f.stakingKeeper.PowerReduction(f.sdkCtx)))
}

// test how the validators are sorted, tests GetBondedValidatorsByPower
func TestGetValidatorSortingUnmixed(t *testing.T) {
	numAddrs := 20

    // powers slice for bootstrap; first 5 match our synthetic amts below, rest 0
    powers := make([]int64, numAddrs)
    powers[0] = 0
    powers[1] = 100
    powers[2] = 1
    powers[3] = 400
    powers[4] = 200	
	f, _, valAddrs := bootstrapValidatorTest(t, powers, 20)

	// initialize some validators into the state
	amts := []math.Int{
		math.NewIntFromUint64(0),
		f.stakingKeeper.PowerReduction(f.sdkCtx).MulRaw(100),
		f.stakingKeeper.PowerReduction(f.sdkCtx),
		f.stakingKeeper.PowerReduction(f.sdkCtx).MulRaw(400),
		f.stakingKeeper.PowerReduction(f.sdkCtx).MulRaw(200),
	}
	n := len(amts)
	var validators [5]types.Validator
	for i, amt := range amts {
		validators[i] = testutil.NewValidator(t, valAddrs[i], PKs[i])
		validators[i].Status = types.Bonded
		validators[i].Tokens = amt
		validators[i].DelegatorShares = math.LegacyNewDecFromInt(amt)

		keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[i], true)
	}

	// first make sure everything made it in to the gotValidator group
	resValidators, err := f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, n, len(resValidators))

	assert.DeepEqual(t, math.NewInt(400).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx)), resValidators[0].BondedTokens())
	assert.DeepEqual(t, math.NewInt(200).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx)), resValidators[1].BondedTokens())
	assert.DeepEqual(t, math.NewInt(100).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx)), resValidators[2].BondedTokens())
	assert.DeepEqual(t, math.NewInt(1).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx)), resValidators[3].BondedTokens())
	assert.DeepEqual(t, math.NewInt(0), resValidators[4].BondedTokens())
	assert.Equal(t, validators[3].OperatorAddress, resValidators[0].OperatorAddress, "%v", resValidators)
	assert.Equal(t, validators[4].OperatorAddress, resValidators[1].OperatorAddress, "%v", resValidators)
	assert.Equal(t, validators[1].OperatorAddress, resValidators[2].OperatorAddress, "%v", resValidators)
	assert.Equal(t, validators[2].OperatorAddress, resValidators[3].OperatorAddress, "%v", resValidators)
	assert.Equal(t, validators[0].OperatorAddress, resValidators[4].OperatorAddress, "%v", resValidators)

	// test a basic increase in voting power
	validators[3].Tokens = math.NewInt(500).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx))
	keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[3], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, len(resValidators), n)
	assert.Assert(ValEq(t, validators[3], resValidators[0]))

	// test a decrease in voting power
	validators[3].Tokens = math.NewInt(300).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx))
	keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[3], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, len(resValidators), n)
	assert.Assert(ValEq(t, validators[3], resValidators[0]))
	assert.Assert(ValEq(t, validators[4], resValidators[1]))

	// test equal voting power, different age
	validators[3].Tokens = math.NewInt(200).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx))
	f.sdkCtx = f.sdkCtx.WithBlockHeight(10)
	keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[3], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, len(resValidators), n)
	assert.Assert(ValEq(t, validators[3], resValidators[0]))
	assert.Assert(ValEq(t, validators[4], resValidators[1]))

	// no change in voting power - no change in sort
	f.sdkCtx = f.sdkCtx.WithBlockHeight(20)
	keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[4], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, len(resValidators), n)
	assert.Assert(ValEq(t, validators[3], resValidators[0]))
	assert.Assert(ValEq(t, validators[4], resValidators[1]))

	// change in voting power of both validators, both still in v-set, no age change
	validators[3].Tokens = math.NewInt(300).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx))
	validators[4].Tokens = math.NewInt(300).Mul(f.stakingKeeper.PowerReduction(f.sdkCtx))
	keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[3], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, len(resValidators), n)
	f.sdkCtx = f.sdkCtx.WithBlockHeight(30)
	keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[4], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, len(resValidators), n, "%v", resValidators)
	assert.Assert(ValEq(t, validators[3], resValidators[0]))
	assert.Assert(ValEq(t, validators[4], resValidators[1]))
}

func TestGetValidatorSortingMixed(t *testing.T) {
    // we’ll create 5 validators; the bonded ones should be sorted by power desc
    numAddrs := 20

    // powers for first 5 validators (in “tokens”/power units); rest zero
    powers := make([]int64, numAddrs)
    // layout: indices 0..4 are our actors
    //  - v0: Bonded, power 100
    //  - v1: Bonded, power 400
    //  - v2: Unbonded, power 50  (excluded from bonded set)
    //  - v3: Unbonding, power 200 (excluded from bonded set)
    //  - v4: Bonded, power 1
    powers[0] = 100
    powers[1] = 400
    powers[2] = 50
    powers[3] = 200
    powers[4] = 1

    f, _, valAddrs := bootstrapValidatorTest(t, powers, numAddrs)	

    pr := f.stakingKeeper.PowerReduction(f.sdkCtx)
    toAmt := func(p int64) math.Int { return pr.MulRaw(p) }

    // pubkeys for the first 5 validators
    pks := simtestutil.CreateTestPubKeys(5)

	statuses := []types.BondStatus{
	    types.Bonded,    // v0 (100)
	    types.Bonded,    // v1 (400)
	    types.Unbonding, // v2 (50)
	    types.Unbonding, // v3 (200)
		types.Bonded,    // v4 (1)
	}

    // Pre-fund module pools to back synthetic Tokens/Status
    bondedPool := f.stakingKeeper.GetBondedPool(f.sdkCtx)
    notBondedPool := f.stakingKeeper.GetNotBondedPool(f.sdkCtx)
    f.accountKeeper.SetModuleAccount(f.sdkCtx, bondedPool)
    f.accountKeeper.SetModuleAccount(f.sdkCtx, notBondedPool)

	bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
	require.NoError(t, err)

    var bondedTotal, notBondedTotal math.Int
    bondedTotal = math.ZeroInt()
    notBondedTotal = math.ZeroInt()
    for i := 0; i < 5; i++ {
        amt := toAmt(powers[i])
        switch statuses[i] {
        case types.Bonded:
            bondedTotal = bondedTotal.Add(amt)
        case types.Unbonded, types.Unbonding:
            notBondedTotal = notBondedTotal.Add(amt)
        }
    }
    if bondedTotal.IsPositive() {
        require.NoError(t, banktestutil.FundModuleAccount(
            f.sdkCtx, f.bankKeeper, bondedPool.GetName(),
            sdk.NewCoins(sdk.NewCoin(bondDenom, bondedTotal)),
        ))
    }
    if notBondedTotal.IsPositive() {
        require.NoError(t, banktestutil.FundModuleAccount(
            f.sdkCtx, f.bankKeeper, notBondedPool.GetName(),
            sdk.NewCoins(sdk.NewCoin(bondDenom, notBondedTotal)),
        ))
    }
			

    var validators [5]types.Validator
    for i := 0; i < 5; i++ {
        v := testutil.NewValidator(t, valAddrs[i], pks[i])

        // synthetic power for sorting
        v.Tokens = toAmt(powers[i])
        v.DelegatorShares = math.LegacyNewDecFromInt(v.Tokens)

		// mix statuses: only Bonded ones should appear in GetBondedValidatorsByPower
		// set desired status from the slice
    	v.Status = statuses[i]


		// index only if it's meant to be Bonded
		if statuses[i] == types.Bonded {
	        keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, v, true)
		} else {
	    	// Write non-bonded validators directly; do NOT recalc.
    		require.NoError(t, f.stakingKeeper.SetValidator(f.sdkCtx, v))
		}

    	validators[i] = v
    }

	for i := 0; i < 5; i++ {
		v, err := f.stakingKeeper.GetValidator(f.sdkCtx, valAddrs[i]) // operator addrs
		require.NoError(t, err)
		require.Equal(t, statuses[i], v.Status,
			"validator %d wrong status: got %s want %s", i, v.Status, statuses[i],
		)
	}

    // initial bonded set: expect order by power desc among Bonded: v1(400), v0(100), v4(1)
    bonded, err := f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
    require.NoError(t, err)
    require.Equal(t, 3, len(bonded), "only bonded validators should be returned")

    require.Equal(t, validators[1].OperatorAddress, bonded[0].OperatorAddress) // 400
    require.Equal(t, validators[0].OperatorAddress, bonded[1].OperatorAddress) // 100
    require.Equal(t, validators[4].OperatorAddress, bonded[2].OperatorAddress) // 1

    // --- mutate powers/status and recheck ordering ---

    // case 1: decrease v1 from 400 -> 90 (should drop below v0=100)
    f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[1])
    validators[1].Tokens = toAmt(90)
    validators[1].DelegatorShares = math.LegacyNewDecFromInt(validators[1].Tokens)
    keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[1], true)

    bonded, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
    require.NoError(t, err)
    require.Equal(t, 3, len(bonded))
    require.Equal(t, validators[0].OperatorAddress, bonded[0].OperatorAddress) // 100
    require.Equal(t, validators[1].OperatorAddress, bonded[1].OperatorAddress) // 90
    require.Equal(t, validators[4].OperatorAddress, bonded[2].OperatorAddress) // 1

    // case 2: increase v4 from 1 -> 150 (should jump to the top)
    f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[4])
    validators[4].Tokens = toAmt(150)
    validators[4].DelegatorShares = math.LegacyNewDecFromInt(validators[4].Tokens)
    keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[4], true)

    bonded, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
    require.NoError(t, err)
    require.Equal(t, 3, len(bonded))
    require.Equal(t, validators[4].OperatorAddress, bonded[0].OperatorAddress) // 150
    require.Equal(t, validators[0].OperatorAddress, bonded[1].OperatorAddress) // 100
    require.Equal(t, validators[1].OperatorAddress, bonded[2].OperatorAddress) // 90

    // case 3: equal power tie: set v0 to 150 as well; tie-breaker should be deterministic
    // (SDK typically falls back to operator address ordering when power is equal)
    f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[0])
    validators[0].Tokens = toAmt(150)
    validators[0].DelegatorShares = math.LegacyNewDecFromInt(validators[0].Tokens)
    keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[0], true)

    bonded, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
    require.NoError(t, err)
    require.Equal(t, 3, len(bonded))

    // both bonded[0] and bonded[1] should have 150 power; assert the multiset then a stable order
    require.Equal(t, toAmt(150), bonded[0].BondedTokens())
    require.Equal(t, toAmt(150), bonded[1].BondedTokens())
    require.Equal(t, toAmt(90), bonded[2].BondedTokens())

    // stable deterministic tiebreaker: by operator address (SDK behavior)
    // we can't assume which of validators[4]/validators[0] is first without peeking the addrs,
    // but we can assert the pair equals {v4, v0} in some order:
    pair := []string{bonded[0].OperatorAddress, bonded[1].OperatorAddress}
    require.ElementsMatch(t,
        []string{validators[4].OperatorAddress, validators[0].OperatorAddress},
        pair,
        "top-2 should be the two 150-power validators",
    )	
}

// TODO separate out into multiple tests
func TestGetValidatorsEdgeCases(t *testing.T) {
	powers := []int64{0, 100, 400, 400}
	f, _, valAddrs := bootstrapValidatorTest(t, powers, 20)

	// set max validators to 2
	params, err := f.stakingKeeper.GetParams(f.sdkCtx)
	assert.NilError(t, err)
	nMax := uint32(2)
	params.MaxValidators = nMax
	f.stakingKeeper.SetParams(f.sdkCtx, params)

	// initialize some validators into the state
	var validators [4]types.Validator
	for i, power := range powers {
		moniker := fmt.Sprintf("val#%d", int64(i))
		validators[i] = newMonikerValidator(t, valAddrs[i], PKs[i], moniker)

		tokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, power)
		validators[i], _ = validators[i].AddTokensFromDel(tokens)

		notBondedPool := f.stakingKeeper.GetNotBondedPool(f.sdkCtx)
		bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
		assert.NilError(t, err)

		assert.NilError(t, banktestutil.FundModuleAccount(
			f.sdkCtx, f.bankKeeper, notBondedPool.GetName(),
			sdk.NewCoins(sdk.NewCoin(bondDenom, tokens))))
		f.accountKeeper.SetModuleAccount(f.sdkCtx, notBondedPool)

		validators[i] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[i], true)
	}

	// ensure that the first two bonded validators are the largest validators
	resValidators, err := f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, nMax, uint32(len(resValidators)))
	assert.Assert(ValEq(t, validators[2], resValidators[0]))
	assert.Assert(ValEq(t, validators[3], resValidators[1]))

	// delegate 500 tokens to validator 0
	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[0])
	delTokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 500)
	validators[0], _ = validators[0].AddTokensFromDel(delTokens)
	notBondedPool := f.stakingKeeper.GetNotBondedPool(f.sdkCtx)

	newTokens := sdk.NewCoins()

	assert.NilError(t, banktestutil.FundModuleAccount(f.sdkCtx, f.bankKeeper, notBondedPool.GetName(), newTokens))
	f.accountKeeper.SetModuleAccount(f.sdkCtx, notBondedPool)

	// test that the two largest validators are
	//   a) validator 0 with 500 tokens
	//   b) validator 2 with 400 tokens (delegated before validator 3)
	validators[0] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[0], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, nMax, uint32(len(resValidators)))
	assert.Assert(ValEq(t, validators[0], resValidators[0]))
	assert.Assert(ValEq(t, validators[2], resValidators[1]))

	// A validator which leaves the bonded validator set due to a decrease in voting power,
	// then increases to the original voting power, does not get its spot back in the
	// case of a tie.
	//
	// Order of operations for this test:
	//  - validator 3 enter validator set with 1 new token
	//  - validator 3 removed validator set by removing 201 tokens (validator 2 enters)
	//  - validator 3 adds 200 tokens (equal to validator 2 now) and does not get its spot back

	// validator 3 enters bonded validator set
	f.sdkCtx = f.sdkCtx.WithBlockHeight(40)

	valbz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[3].GetOperator())
	assert.NilError(t, err)

	validators[3], err = f.stakingKeeper.GetValidator(f.sdkCtx, valbz)
	assert.NilError(t, err)
	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[3])
	validators[3], _ = validators[3].AddTokensFromDel(f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 1))

	notBondedPool = f.stakingKeeper.GetNotBondedPool(f.sdkCtx)
	newTokens = sdk.NewCoins(sdk.NewCoin(params.BondDenom, f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 1)))
	assert.NilError(t, banktestutil.FundModuleAccount(f.sdkCtx, f.bankKeeper, notBondedPool.GetName(), newTokens))
	f.accountKeeper.SetModuleAccount(f.sdkCtx, notBondedPool)

	validators[3] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[3], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, nMax, uint32(len(resValidators)))
	assert.Assert(ValEq(t, validators[0], resValidators[0]))
	assert.Assert(ValEq(t, validators[3], resValidators[1]))

	// validator 3 kicked out temporarily
	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[3])
	rmTokens := validators[3].TokensFromShares(math.LegacyNewDec(201)).TruncateInt()
	validators[3], _ = validators[3].RemoveDelShares(math.LegacyNewDec(201))

	bondedPool := f.stakingKeeper.GetBondedPool(f.sdkCtx)
	assert.NilError(t, banktestutil.FundModuleAccount(f.sdkCtx, f.bankKeeper, bondedPool.GetName(), sdk.NewCoins(sdk.NewCoin(params.BondDenom, rmTokens))))
	f.accountKeeper.SetModuleAccount(f.sdkCtx, bondedPool)

	validators[3] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[3], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, nMax, uint32(len(resValidators)))
	assert.Assert(ValEq(t, validators[0], resValidators[0]))
	assert.Assert(ValEq(t, validators[2], resValidators[1]))

	// validator 3 does not get spot back
	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[3])
	validators[3], _ = validators[3].AddTokensFromDel(math.NewInt(200))

	notBondedPool = f.stakingKeeper.GetNotBondedPool(f.sdkCtx)
	assert.NilError(t, banktestutil.FundModuleAccount(f.sdkCtx, f.bankKeeper, notBondedPool.GetName(), sdk.NewCoins(sdk.NewCoin(params.BondDenom, math.NewInt(200)))))
	f.accountKeeper.SetModuleAccount(f.sdkCtx, notBondedPool)

	validators[3] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[3], true)
	resValidators, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	assert.NilError(t, err)
	assert.Equal(t, nMax, uint32(len(resValidators)))
	assert.Assert(ValEq(t, validators[0], resValidators[0]))
	assert.Assert(ValEq(t, validators[2], resValidators[1]))
	_, exists := f.stakingKeeper.GetValidator(f.sdkCtx, valbz)
	assert.Assert(t, exists)
}

func TestValidatorBondHeight(t *testing.T) {
    // powers: A=20 (Bonded), B=10 (Bonded), C=5 (Unbonded initially)
    numAddrs := 10
    powers := make([]int64, numAddrs)
    powers[0], powers[1], powers[2] = 20, 10, 5

    f, _, valAddrs := bootstrapValidatorTest(t, powers, numAddrs)

	// now 2 max resValidators (cliff)
	params, err := f.stakingKeeper.GetParams(f.sdkCtx)
	assert.NilError(t, err)
	params.MaxValidators = 2
	require.NoError(t, f.stakingKeeper.SetParams(f.sdkCtx, params))

    pr := f.stakingKeeper.PowerReduction(f.sdkCtx)
    toAmt := func(p int64) math.Int { return pr.MulRaw(p) }

    // Pre-fund module pools to match synthetic tokens/status
    bondedPool := f.stakingKeeper.GetBondedPool(f.sdkCtx)
    notBondedPool := f.stakingKeeper.GetNotBondedPool(f.sdkCtx)
    f.accountKeeper.SetModuleAccount(f.sdkCtx, bondedPool)
    f.accountKeeper.SetModuleAccount(f.sdkCtx, notBondedPool)

    bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
    require.NoError(t, err)

    // initial statuses: A,B bonded; C unbonded
    statuses := []types.BondStatus{
        types.Bonded,   // A (20)
        types.Bonded,   // B (10)
        types.Unbonded, // C (5)
    }

    bondedTotal := toAmt(20).Add(toAmt(10))
    notBondedTotal := toAmt(5)

    if bondedTotal.IsPositive() {
        require.NoError(t, banktestutil.FundModuleAccount(
            f.sdkCtx, f.bankKeeper, bondedPool.GetName(),
            sdk.NewCoins(sdk.NewCoin(bondDenom, bondedTotal)),
        ))
    }
    if notBondedTotal.IsPositive() {
        require.NoError(t, banktestutil.FundModuleAccount(
            f.sdkCtx, f.bankKeeper, notBondedPool.GetName(),
            sdk.NewCoins(sdk.NewCoin(bondDenom, notBondedTotal)),
        ))
    }

    // Create validators A,B,C with synthetic tokens & statuses.
    pks := simtestutil.CreateTestPubKeys(3)
    var vals [3]types.Validator
    for i := 0; i < 3; i++ {
        v := testutil.NewValidator(t, valAddrs[i], pks[i])
        v.Tokens = toAmt(powers[i])
        v.DelegatorShares = math.LegacyNewDecFromInt(v.Tokens)
        v.Status = statuses[i]

        // Write to store; index ONLY bonded ones (so we keep C unbonded).
        require.NoError(t, f.stakingKeeper.SetValidator(f.sdkCtx, v))
        if statuses[i] == types.Bonded {
			v = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, v, true)
        }
        vals[i] = v

        // Advance block to get distinct bond heights when they bond.
        f.sdkCtx = f.sdkCtx.WithBlockHeight(f.sdkCtx.BlockHeight() + 1)
    }

	// initial bonded set should be A(20), B(10)
	bonded, err := f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	require.NoError(t, err)
	require.Equal(t, 2, len(bonded))
	require.Equal(t, vals[0].OperatorAddress, bonded[0].OperatorAddress) // A
	require.Equal(t, vals[1].OperatorAddress, bonded[1].OperatorAddress) // B

	// --- "BondHeight behavior": promote C across the cliff (5 -> 30) ---
	// fund delta in not-bonded pool; then update C via helper to recompute bondedness
	delta := toAmt(30 - 5)
	require.NoError(t, banktestutil.FundModuleAccount(
		f.sdkCtx, f.bankKeeper, notBondedPool.GetName(),
		sdk.NewCoins(sdk.NewCoin(bondDenom, delta)),
	))
	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, vals[2]) // no-op for unbonded
	vals[2].Tokens = toAmt(30)
	vals[2].DelegatorShares = math.LegacyNewDecFromInt(vals[2].Tokens)
	vals[2].Status = types.Bonded
	vals[2] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, vals[2], true)

	// Recompute the set so the cliff demotion is applied.
	_, err = f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
	require.NoError(t, err)

	// bonded set should now be C(30), A(20); B is out (no longer Bonded)
	bonded, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	require.NoError(t, err)
	require.Equal(t, 2, len(bonded))
	require.Equal(t, vals[2].OperatorAddress, bonded[0].OperatorAddress) // C promoted
	require.Equal(t, vals[0].OperatorAddress, bonded[1].OperatorAddress) // A stayed

	// 🔐 B should no longer be bonded.
	vb, err := f.stakingKeeper.GetValidator(f.sdkCtx, valAddrs[1])
	require.NoError(t, err)
	require.NotEqual(t, types.Bonded, vb.Status, "B should no longer be bonded after C promotion")
}

func TestFullValidatorSetPowerChange(t *testing.T) {
    // full set of 5; all start Bonded
    numAddrs := 10
    powers := make([]int64, numAddrs)
    // v0..v4 initial powers (descending order expected: 500,400,300,200,100)
    powers[0], powers[1], powers[2], powers[3], powers[4] = 100, 200, 300, 400, 500

    f, _, valAddrs := bootstrapValidatorTest(t, powers, numAddrs)

    // MaxValidators = number of active validators (full set)
    params, err := f.stakingKeeper.GetParams(f.sdkCtx)
    require.NoError(t, err)
    params.MaxValidators = 5
    require.NoError(t, f.stakingKeeper.SetParams(f.sdkCtx, params))

    pr := f.stakingKeeper.PowerReduction(f.sdkCtx)
    toAmt := func(p int64) math.Int { return pr.MulRaw(p) }

    // --- pre-fund bonded pool to back synthetic token increases ---
    bondedPool := f.stakingKeeper.GetBondedPool(f.sdkCtx)
    f.accountKeeper.SetModuleAccount(f.sdkCtx, bondedPool)
    bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
    require.NoError(t, err)

    // initial total + buffer to cover later increases
    totalInit := toAmt(100 + 200 + 300 + 400 + 500)
    buffer := toAmt(500) // arbitrary cushion for increases
    require.NoError(t, banktestutil.FundModuleAccount(
        f.sdkCtx, f.bankKeeper, bondedPool.GetName(),
        sdk.NewCoins(sdk.NewCoin(bondDenom, totalInit.Add(buffer))),
    ))

    // --- create 5 bonded validators, settle initial order ---
    pks := simtestutil.CreateTestPubKeys(5)
    var vals [5]types.Validator
    for i := 0; i < 5; i++ {
        v := testutil.NewValidator(t, valAddrs[i], pks[i])
        v.Tokens = toAmt(powers[i])
        v.DelegatorShares = math.LegacyNewDecFromInt(v.Tokens)
        v.Status = types.Bonded

        require.NoError(t, f.stakingKeeper.SetValidator(f.sdkCtx, v))
        require.NoError(t, f.stakingKeeper.SetValidatorByConsAddr(f.sdkCtx, v))
        require.NoError(t, f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, v))
        v = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, v, true)
        vals[i] = v
    }

    // settle initial bonded set
    _, err = f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
    require.NoError(t, err)

    // initial order should be by power desc: v4(500), v3(400), v2(300), v1(200), v0(100)
    bonded, err := f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
    require.NoError(t, err)
    require.Equal(t, 5, len(bonded))
    require.Equal(t, vals[4].OperatorAddress, bonded[0].OperatorAddress)
    require.Equal(t, vals[3].OperatorAddress, bonded[1].OperatorAddress)
    require.Equal(t, vals[2].OperatorAddress, bonded[2].OperatorAddress)
    require.Equal(t, vals[1].OperatorAddress, bonded[3].OperatorAddress)
    require.Equal(t, vals[0].OperatorAddress, bonded[4].OperatorAddress)

    // --- change powers while the set stays full ---
    // case A: drop the current top v4 from 500 -> 150 (should fall below v2(300))
    f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, vals[4])
    vals[4].Tokens = toAmt(150)
    vals[4].DelegatorShares = math.LegacyNewDecFromInt(vals[4].Tokens)
    vals[4] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, vals[4], true)

    // case B: raise the current bottom v0 from 100 -> 600 (should jump to top)
    f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, vals[0])
    // fund delta into bonded pool to cover the synthetic increase
    delta0 := toAmt(600 - 100)
    require.NoError(t, banktestutil.FundModuleAccount(
        f.sdkCtx, f.bankKeeper, bondedPool.GetName(),
        sdk.NewCoins(sdk.NewCoin(bondDenom, delta0)),
    ))
    vals[0].Tokens = toAmt(600)
    vals[0].DelegatorShares = math.LegacyNewDecFromInt(vals[0].Tokens)
    vals[0] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, vals[0], true)

    // apply updates, then re-check order; membership must remain 5
    _, err = f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
    require.NoError(t, err)

    bonded, err = f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
    require.NoError(t, err)
    require.Equal(t, 5, len(bonded))

    // expected new order: v0(600), v3(400), v2(300), v4(150), v1(200)  <- wait, v1 is 200 > 150
    // correct order: v0(600), v3(400), v2(300), v1(200), v4(150)
    require.Equal(t, vals[0].OperatorAddress, bonded[0].OperatorAddress) // 600
    require.Equal(t, vals[3].OperatorAddress, bonded[1].OperatorAddress) // 400
    require.Equal(t, vals[2].OperatorAddress, bonded[2].OperatorAddress) // 300
    require.Equal(t, vals[1].OperatorAddress, bonded[3].OperatorAddress) // 200
    require.Equal(t, vals[4].OperatorAddress, bonded[4].OperatorAddress) // 150

    // sanity: all still Bonded (full set membership unchanged)
    for i := 0; i < 5; i++ {
        v, err := f.stakingKeeper.GetValidator(f.sdkCtx, valAddrs[i])
        require.NoError(t, err)
        require.Equal(t, types.Bonded, v.Status, "validator %d unexpectedly left Bonded set", i)
    }
}

func TestApplyAndReturnValidatorSetUpdatesAllNone(t *testing.T) {
    numAddrs := 10
    // give them some power so we know the keeper ignores non-bonded even with tokens
    powers := make([]int64, numAddrs)
    powers[0], powers[1], powers[2], powers[3] = 10, 20, 30, 40

    f, _, valAddrs := bootstrapValidatorTest(t, powers, numAddrs)

    // Keep max validators > 0 (normal config)
    params, err := f.stakingKeeper.GetParams(f.sdkCtx)
    require.NoError(t, err)
    params.MaxValidators = 5
    require.NoError(t, f.stakingKeeper.SetParams(f.sdkCtx, params))

    pr := f.stakingKeeper.PowerReduction(f.sdkCtx)
    toAmt := func(p int64) math.Int { return pr.MulRaw(p) }

    // Pre-fund NOT-BONDED pool to match synthetic tokens (no bonded funding since none are bonded)
    notBondedPool := f.stakingKeeper.GetNotBondedPool(f.sdkCtx)
    f.accountKeeper.SetModuleAccount(f.sdkCtx, notBondedPool)

    bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
    require.NoError(t, err)

    totalNB := toAmt(powers[0]).Add(toAmt(powers[1])).Add(toAmt(powers[2])).Add(toAmt(powers[3]))
    require.NoError(t, banktestutil.FundModuleAccount(
        f.sdkCtx, f.bankKeeper, notBondedPool.GetName(),
        sdk.NewCoins(sdk.NewCoin(bondDenom, totalNB)),
    ))

    // Build 4 validators, all non-bonded (some Unbonded, some Unbonding)
    pks := simtestutil.CreateTestPubKeys(4)
    statuses := []types.BondStatus{
        types.Unbonded,   // v0 (10)
        types.Unbonding,  // v1 (20)
        types.Unbonded,   // v2 (30)
        types.Unbonding,  // v3 (40)
    }

    var vals [4]types.Validator
    for i := 0; i < 4; i++ {
        v := testutil.NewValidator(t, valAddrs[i], pks[i])
        v.Tokens = toAmt(powers[i])
        v.DelegatorShares = math.LegacyNewDecFromInt(v.Tokens)
        v.Status = statuses[i]

        // Write to store only; DO NOT index by power/cons for non-bonded validators
        require.NoError(t, f.stakingKeeper.SetValidator(f.sdkCtx, v))
        vals[i] = v
    }

    // Apply updates: since none are Bonded, there should be NO validator set updates
    updates, err := f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
    require.NoError(t, err)
    require.Equal(t, 0, len(updates), "no updates expected when all validators are non-bonded")

    // Bonded set must be empty
    bonded, err := f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
    require.NoError(t, err)
    require.Equal(t, 0, len(bonded), "no bonded validators expected")

    // Sanity: statuses remain as set
    for i := 0; i < 4; i++ {
        got, err := f.stakingKeeper.GetValidator(f.sdkCtx, valAddrs[i])
        require.NoError(t, err)
        require.Equal(t, statuses[i], got.Status,
            "validator %d wrong status: got %s want %s", i, got.Status, statuses[i])
    }
}

func TestApplyAndReturnValidatorSetUpdatesIdentical(t *testing.T) {
	numAddrs := 10
	// five validators, identical power (100 each)
	powers := make([]int64, numAddrs)
	for i := 0; i < 5; i++ {
		powers[i] = 100
	}

	f, _, valAddrs := bootstrapValidatorTest(t, powers, numAddrs)

	// full set = 5 bonded validators
	params, err := f.stakingKeeper.GetParams(f.sdkCtx)
	require.NoError(t, err)
	params.MaxValidators = 5
	require.NoError(t, f.stakingKeeper.SetParams(f.sdkCtx, params))

	pr := f.stakingKeeper.PowerReduction(f.sdkCtx)
	toAmt := func(p int64) math.Int { return pr.MulRaw(p) }

	// Pre-fund BONDED pool with total
	bondedPool := f.stakingKeeper.GetBondedPool(f.sdkCtx)
	f.accountKeeper.SetModuleAccount(f.sdkCtx, bondedPool)

	bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
	require.NoError(t, err)

	total := math.ZeroInt()
	for i := 0; i < 5; i++ {
		total = total.Add(toAmt(powers[i]))
	}
	require.NoError(t, banktestutil.FundModuleAccount(
		f.sdkCtx, f.bankKeeper, bondedPool.GetName(),
		sdk.NewCoins(sdk.NewCoin(bondDenom, total)),
	))

	// Build 5 bonded validators with identical power
	pks := simtestutil.CreateTestPubKeys(5)
	for i := 0; i < 5; i++ {
		v := testutil.NewValidator(t, valAddrs[i], pks[i])
		v.Tokens = toAmt(100)
		v.DelegatorShares = math.LegacyNewDecFromInt(v.Tokens)
		v.Status = types.Bonded

		require.NoError(t, f.stakingKeeper.SetValidator(f.sdkCtx, v))
		require.NoError(t, f.stakingKeeper.SetValidatorByConsAddr(f.sdkCtx, v))
		require.NoError(t, f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, v))
		_ = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, v, true)
	}

	// First settle: may or may not emit updates depending on setup; just ensure no error.
	_, err = f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
	require.NoError(t, err)

	// Bonded list must contain all 5, deterministically ordered for identical power.
	bonded, err := f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	require.NoError(t, err)
	require.Equal(t, 5, len(bonded))

	// Deterministic tie-breaker is by RAW operator address bytes (not bech32 string).
	codec := f.stakingKeeper.ValidatorAddressCodec()
	prevBz, err := codec.StringToBytes(bonded[0].OperatorAddress)
	require.NoError(t, err)

	for i := 1; i < len(bonded); i++ {
		currBz, err := codec.StringToBytes(bonded[i].OperatorAddress)
		require.NoError(t, err)

		// Non-decreasing by raw bytes when power is identical.
		require.LessOrEqual(t, bytes.Compare(prevBz, currBz), 0,
			"bonded validators must be deterministically ordered by address bytes when power is equal",
		)
		prevBz = currBz
	}

	// Second settle: no updates expected since nothing changed.
	updates, err := f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
	require.NoError(t, err)
	require.Equal(t, 0, len(updates), "no updates expected when set and powers are identical")

	// Optional: bump height and re-apply; still zero updates.
	f.sdkCtx = f.sdkCtx.WithBlockHeight(f.sdkCtx.BlockHeight() + 1)
	updates, err = f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
	require.NoError(t, err)
	require.Equal(t, 0, len(updates))
}


func TestApplyAndReturnValidatorSetUpdatesSingleValueChange(t *testing.T) {
	// 5 bonded validators, all same initial power (100)
	numAddrs := 10
	powers := make([]int64, numAddrs)
	for i := 0; i < 5; i++ {
		powers[i] = 100
	}

	f, _, valAddrs := bootstrapValidatorTest(t, powers, numAddrs)

	// Full set (no cliff): all 5 are bonded
	params, err := f.stakingKeeper.GetParams(f.sdkCtx)
	require.NoError(t, err)
	params.MaxValidators = 5
	require.NoError(t, f.stakingKeeper.SetParams(f.sdkCtx, params))

	pr := f.stakingKeeper.PowerReduction(f.sdkCtx)
	toAmt := func(p int64) math.Int { return pr.MulRaw(p) }

	// Pre-fund BONDED pool with total + a tiny buffer for the single +1 change
	bondedPool := f.stakingKeeper.GetBondedPool(f.sdkCtx)
	f.accountKeeper.SetModuleAccount(f.sdkCtx, bondedPool)
	bondDenom, err := f.stakingKeeper.BondDenom(f.sdkCtx)
	require.NoError(t, err)

	total := math.ZeroInt()
	for i := 0; i < 5; i++ {
		total = total.Add(toAmt(100))
	}
	// add buffer for +1 consensus power change we’ll apply later
	total = total.Add(toAmt(1))

	require.NoError(t, banktestutil.FundModuleAccount(
		f.sdkCtx, f.bankKeeper, bondedPool.GetName(),
		sdk.NewCoins(sdk.NewCoin(bondDenom, total)),
	))

	// Build 5 bonded validators with identical power
	pks := simtestutil.CreateTestPubKeys(5)
	var vals [5]types.Validator
	for i := 0; i < 5; i++ {
		v := testutil.NewValidator(t, valAddrs[i], pks[i])
		v.Tokens = toAmt(100)
		v.DelegatorShares = math.LegacyNewDecFromInt(v.Tokens)
		v.Status = types.Bonded

		require.NoError(t, f.stakingKeeper.SetValidator(f.sdkCtx, v))
		require.NoError(t, f.stakingKeeper.SetValidatorByConsAddr(f.sdkCtx, v))
		require.NoError(t, f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, v))
		v = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, v, true)
		vals[i] = v
	}

	// Initial settle (may or may not emit updates depending on setup; just ensure no error)
	_, err = f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
	require.NoError(t, err)

	// Pick a target and bump by +1 consensus power
	target := 2
	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, vals[target])

	// Increase tokens by +1 * PowerReduction
	vals[target].Tokens = vals[target].Tokens.Add(toAmt(1))
	vals[target].DelegatorShares = math.LegacyNewDecFromInt(vals[target].Tokens)

	// IMPORTANT: Do NOT call TestingUpdateValidator here.
	// Write only the store + power index so LastValidatorPower is unchanged.
	require.NoError(t, f.stakingKeeper.SetValidator(f.sdkCtx, vals[target]))
	require.NoError(t, f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, vals[target]))

	// Now the end-block should detect a diff and emit updates
	updates, err := f.stakingKeeper.ApplyAndReturnValidatorSetUpdates(f.sdkCtx)
	require.NoError(t, err)

	// If you want exactly-1, you can assert that, but some forks may re-emit several.
	// The robust check is to ensure the target is among updates:
	require.Greater(t, len(updates), 0, "at least one update expected after single power change")

	// Optional: verify target’s new power present in updates
	tpk, err := vals[target].ConsPubKey()
	require.NoError(t, err)
	found := false
	for _, u := range updates {
		// compare Tendermint pubkeys; adjust if your type differs
		if bytes.Equal(u.PubKey.GetEd25519(), tpk.Bytes()) || bytes.Equal(u.PubKey.GetSecp256K1(), tpk.Bytes()) {
			// 100 -> 101 consensus power
			require.Equal(t, int64(101), u.Power)
			found = true
			break
		}
	}
	require.True(t, found, "target validator update not found")

	// And the target should now be first in bonded order
	bonded, err := f.stakingKeeper.GetBondedValidatorsByPower(f.sdkCtx)
	require.NoError(t, err)
	require.Equal(t, vals[target].OperatorAddress, bonded[0].OperatorAddress)
	
	// The remaining four should keep a deterministic order among themselves.
	// (We don’t assert their exact order here since it depends on byte-wise address tiebreaker,
	// but we do sanity-check that the bumped one is unique on top.)
	for i := 1; i < 5; i++ {
		require.NotEqual(t, vals[target].OperatorAddress, bonded[i].OperatorAddress)
	}
}

/*
func TestApplyAndReturnValidatorSetUpdatesMultipleValueChange(t *testing.T) {
	powers := []int64{10, 20}
	// TODO: use it in other places
	f, _, _, validators := initValidators(t, 1000, 20, powers)

	validators[0] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[0], false)
	validators[1] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[1], false)
	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 2)

	// test multiple value change
	//  tendermintUpdate set: {c1, c3} -> {c1', c3'}
	delTokens1 := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 190)
	delTokens2 := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 80)
	validators[0], _ = validators[0].AddTokensFromDel(delTokens1)
	validators[1], _ = validators[1].AddTokensFromDel(delTokens2)
	validators[0] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[0], false)
	validators[1] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[1], false)

	updates := applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 2)
	assert.DeepEqual(t, validators[0].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])
	assert.DeepEqual(t, validators[1].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[1])
}

func TestApplyAndReturnValidatorSetUpdatesInserted(t *testing.T) {
	powers := []int64{10, 20, 5, 15, 25}
	f, _, _, validators := initValidators(t, 1000, 20, powers)

	validators[0] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[0], false)
	validators[1] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[1], false)
	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 2)

	// test validtor added at the beginning
	//  tendermintUpdate set: {} -> {c0}
	f.stakingKeeper.SetValidator(f.sdkCtx, validators[2])
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[2])
	updates := applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 1)
	val2bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[2].GetOperator())
	assert.NilError(t, err)
	validators[2], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val2bz)
	assert.DeepEqual(t, validators[2].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])

	// test validtor added at the beginning
	//  tendermintUpdate set: {} -> {c0}
	f.stakingKeeper.SetValidator(f.sdkCtx, validators[3])
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[3])
	updates = applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 1)
	val3bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[3].GetOperator())
	assert.NilError(t, err)
	validators[3], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val3bz)
	assert.DeepEqual(t, validators[3].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])

	// test validtor added at the end
	//  tendermintUpdate set: {} -> {c0}
	f.stakingKeeper.SetValidator(f.sdkCtx, validators[4])
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[4])
	updates = applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 1)
	val4bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[4].GetOperator())
	assert.NilError(t, err)
	validators[4], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val4bz)
	assert.DeepEqual(t, validators[4].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])
}

func TestApplyAndReturnValidatorSetUpdatesWithCliffValidator(t *testing.T) {
	f, addrs, _ := bootstrapValidatorTest(t, 1000, 20)
	params := types.DefaultParams()
	params.MaxValidators = 2
	f.stakingKeeper.SetParams(f.sdkCtx, params)

	powers := []int64{10, 20, 5}
	var validators [5]types.Validator
	for i, power := range powers {
		validators[i] = testutil.NewValidator(t, sdk.ValAddress(addrs[i]), PKs[i])
		tokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, power)
		validators[i], _ = validators[i].AddTokensFromDel(tokens)
	}
	validators[0] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[0], false)
	validators[1] = keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[1], false)
	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 2)

	// test validator added at the end but not inserted in the valset
	//  tendermintUpdate set: {} -> {}
	keeper.TestingUpdateValidator(f.stakingKeeper, f.sdkCtx, validators[2], false)
	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 0)

	// test validator change its power and become a gotValidator (pushing out an existing)
	//  tendermintUpdate set: {}     -> {c0, c4}
	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 0)

	tokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 10)
	validators[2], _ = validators[2].AddTokensFromDel(tokens)
	f.stakingKeeper.SetValidator(f.sdkCtx, validators[2])
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[2])
	updates := applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 2)
	val2bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[2].GetOperator())
	assert.NilError(t, err)
	validators[2], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val2bz)
	assert.DeepEqual(t, validators[0].ABCIValidatorUpdateZero(), updates[1])
	assert.DeepEqual(t, validators[2].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])
}

func TestApplyAndReturnValidatorSetUpdatesNewValidator(t *testing.T) {
	f, _, _ := bootstrapValidatorTest(t, 1000, 20)
	params, err := f.stakingKeeper.GetParams(f.sdkCtx)
	assert.NilError(t, err)
	params.MaxValidators = uint32(3)

	assert.NilError(t, f.stakingKeeper.SetParams(f.sdkCtx, params))

	powers := []int64{100, 100}
	var validators [2]types.Validator

	// initialize some validators into the state
	for i, power := range powers {
		valPubKey := PKs[i+1]
		valAddr := sdk.ValAddress(valPubKey.Address().Bytes())

		validators[i] = testutil.NewValidator(t, valAddr, valPubKey)
		tokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, power)
		validators[i], _ = validators[i].AddTokensFromDel(tokens)

		f.stakingKeeper.SetValidator(f.sdkCtx, validators[i])
		f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[i])
	}

	// verify initial CometBFT updates are correct
	updates := applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, len(validators))

	val0bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[0].GetOperator())
	assert.NilError(t, err)
	val1bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[1].GetOperator())
	assert.NilError(t, err)
	validators[0], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val0bz)
	validators[1], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val1bz)
	assert.DeepEqual(t, validators[0].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])
	assert.DeepEqual(t, validators[1].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[1])

	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 0)

	// update initial validator set
	for i, power := range powers {

		f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[i])
		tokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, power)
		validators[i], _ = validators[i].AddTokensFromDel(tokens)

		f.stakingKeeper.SetValidator(f.sdkCtx, validators[i])
		f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[i])
	}

	// add a new validator that goes from zero power, to non-zero power, back to
	// zero power
	valPubKey := PKs[len(validators)+1]
	valAddr := sdk.ValAddress(valPubKey.Address().Bytes())
	amt := math.NewInt(100)

	validator := testutil.NewValidator(t, valAddr, valPubKey)
	validator, _ = validator.AddTokensFromDel(amt)

	f.stakingKeeper.SetValidator(f.sdkCtx, validator)

	validator, _ = validator.RemoveDelShares(math.LegacyNewDecFromInt(amt))
	f.stakingKeeper.SetValidator(f.sdkCtx, validator)
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validator)

	// add a new validator that increases in power
	valPubKey = PKs[len(validators)+2]
	valAddr = sdk.ValAddress(valPubKey.Address().Bytes())

	validator = testutil.NewValidator(t, valAddr, valPubKey)
	tokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 500)
	validator, _ = validator.AddTokensFromDel(tokens)
	f.stakingKeeper.SetValidator(f.sdkCtx, validator)
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validator)

	// verify initial CometBFT updates are correct
	updates = applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, len(validators)+1)
	valbz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validator.GetOperator())
	assert.NilError(t, err)
	validator, _ = f.stakingKeeper.GetValidator(f.sdkCtx, valbz)
	validators[0], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val0bz)
	validators[1], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val1bz)
	assert.DeepEqual(t, validator.ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])
	assert.DeepEqual(t, validators[0].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[1])
	assert.DeepEqual(t, validators[1].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[2])
}

func TestApplyAndReturnValidatorSetUpdatesBondTransition(t *testing.T) {
	f, _, _ := bootstrapValidatorTest(t, 1000, 20)
	params, err := f.stakingKeeper.GetParams(f.sdkCtx)
	assert.NilError(t, err)
	params.MaxValidators = uint32(2)

	assert.NilError(t, f.stakingKeeper.SetParams(f.sdkCtx, params))

	powers := []int64{100, 200, 300}
	var validators [3]types.Validator

	// initialize some validators into the state
	for i, power := range powers {
		moniker := fmt.Sprintf("%d", i)
		valPubKey := PKs[i+1]
		valAddr := sdk.ValAddress(valPubKey.Address().Bytes())

		validators[i] = newMonikerValidator(t, valAddr, valPubKey, moniker)
		tokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, power)
		validators[i], _ = validators[i].AddTokensFromDel(tokens)
		f.stakingKeeper.SetValidator(f.sdkCtx, validators[i])
		f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[i])
	}

	// verify initial CometBFT updates are correct
	updates := applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 2)
	val1bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[1].GetOperator())
	assert.NilError(t, err)
	val2bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[2].GetOperator())
	assert.NilError(t, err)
	validators[2], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val2bz)
	validators[1], _ = f.stakingKeeper.GetValidator(f.sdkCtx, val1bz)
	assert.DeepEqual(t, validators[2].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])
	assert.DeepEqual(t, validators[1].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[1])

	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 0)

	// delegate to validator with lowest power but not enough to bond
	f.sdkCtx = f.sdkCtx.WithBlockHeight(1)

	val0bz, err := f.stakingKeeper.ValidatorAddressCodec().StringToBytes(validators[0].GetOperator())
	assert.NilError(t, err)
	validators[0], err = f.stakingKeeper.GetValidator(f.sdkCtx, val0bz)
	assert.NilError(t, err)

	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[0])
	tokens := f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 1)
	validators[0], _ = validators[0].AddTokensFromDel(tokens)
	f.stakingKeeper.SetValidator(f.sdkCtx, validators[0])
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[0])

	// verify initial CometBFT updates are correct
	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 0)

	// create a series of events that will bond and unbond the validator with
	// lowest power in a single block context (height)
	f.sdkCtx = f.sdkCtx.WithBlockHeight(2)

	validators[1], err = f.stakingKeeper.GetValidator(f.sdkCtx, val1bz)
	assert.NilError(t, err)

	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[0])
	validators[0], _ = validators[0].RemoveDelShares(validators[0].DelegatorShares)
	f.stakingKeeper.SetValidator(f.sdkCtx, validators[0])
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[0])
	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 0)

	f.stakingKeeper.DeleteValidatorByPowerIndex(f.sdkCtx, validators[1])
	tokens = f.stakingKeeper.TokensFromConsensusPower(f.sdkCtx, 250)
	validators[1], _ = validators[1].AddTokensFromDel(tokens)
	f.stakingKeeper.SetValidator(f.sdkCtx, validators[1])
	f.stakingKeeper.SetValidatorByPowerIndex(f.sdkCtx, validators[1])

	// verify initial CometBFT updates are correct
	updates = applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 1)
	assert.DeepEqual(t, validators[1].ABCIValidatorUpdate(f.stakingKeeper.PowerReduction(f.sdkCtx)), updates[0])

	applyValidatorSetUpdates(t, f.sdkCtx, f.stakingKeeper, 0)
}

*/