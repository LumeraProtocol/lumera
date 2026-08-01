package keeper

import (
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
)

func TestRewriteStakeAuthorization_CloneDoesNotMutateSourceAndCollapsesDestination(t *testing.T) {
	oldVal := sdk.ValAddress(bytesOf(1))
	newVal := sdk.ValAddress(bytesOf(2))
	otherVal := sdk.ValAddress(bytesOf(3))
	source := &stakingtypes.StakeAuthorization{
		Validators: &stakingtypes.StakeAuthorization_AllowList{AllowList: &stakingtypes.StakeAuthorization_Validators{
			Address: []string{oldVal.String(), newVal.String(), otherVal.String()},
		}},
	}

	cloned := proto.Clone(source).(*stakingtypes.StakeAuthorization)
	changed, err := rewriteStakeAuthorization(cloned, oldVal, newVal)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{oldVal.String(), newVal.String(), otherVal.String()}, source.GetAllowList().Address)
	require.Equal(t, []string{newVal.String(), otherVal.String()}, cloned.GetAllowList().Address)
}

func TestBuildWithdrawAddressPlan_CapPlusOneIsReadOnly(t *testing.T) {
	key := storetypes.NewKVStoreKey(distrtypes.StoreKey)
	ctx := testutil.DefaultContextWithKeys(map[string]*storetypes.KVStoreKey{distrtypes.StoreKey: key}, nil, nil)
	svc := runtime.NewKVStoreService(key)
	k := Keeper{distributionStoreHandle: &distributionStoreHandle{svc: svc}}
	legacy := sdk.AccAddress(bytesOf(4))
	store := svc.OpenKVStore(ctx)

	for _, delegatorByte := range []byte{6, 7} {
		delegator := sdk.AccAddress(bytesOf(delegatorByte))
		withdrawKey := append(append([]byte{}, distrtypes.DelegatorWithdrawAddrPrefix...), byte(len(delegator)))
		withdrawKey = append(withdrawKey, delegator...)
		require.NoError(t, store.Set(withdrawKey, legacy.Bytes()))
	}

	_, err := k.buildWithdrawAddressPlan(ctx, legacy, 1)
	require.ErrorContains(t, err, "exceeds max 1")
	for _, delegatorByte := range []byte{6, 7} {
		delegator := sdk.AccAddress(bytesOf(delegatorByte))
		withdrawKey := append(append([]byte{}, distrtypes.DelegatorWithdrawAddrPrefix...), byte(len(delegator)))
		withdrawKey = append(withdrawKey, delegator...)
		value, getErr := store.Get(withdrawKey)
		require.NoError(t, getErr)
		require.Equal(t, legacy.Bytes(), value)
	}
}

func TestCloneTime_DeepCopy(t *testing.T) {
	original := time.Unix(123, 456)
	cloned := cloneTime(&original)
	require.NotSame(t, &original, cloned)
	*cloned = cloned.Add(time.Hour)
	require.Equal(t, time.Unix(123, 456), original)
}

func bytesOf(value byte) []byte {
	result := make([]byte, 20)
	for i := range result {
		result[i] = value
	}
	return result
}
