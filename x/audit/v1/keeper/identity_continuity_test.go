package keeper_test

import (
	"encoding/binary"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func buildAccountTransitionPlan(t *testing.T, f *fixture, transition types.AccountTransition) (keeper.AccountTransitionPlan, error) {
	t.Helper()
	params := f.keeper.GetParams(f.ctx).WithDefaults()
	height := int64(params.EpochZeroHeight)
	if transition.EffectiveEpoch > 0 {
		height += int64(transition.EffectiveEpoch-1) * int64(params.EpochLengthBlocks)
	}
	return f.keeper.BuildAccountTransitionPlan(f.ctx.WithBlockHeight(height), transition)
}

func recordAccountTransition(t *testing.T, f *fixture, transition types.AccountTransition) error {
	t.Helper()
	params := f.keeper.GetParams(f.ctx).WithDefaults()
	height := int64(params.EpochZeroHeight)
	if transition.EffectiveEpoch > 0 {
		height += int64(transition.EffectiveEpoch-1) * int64(params.EpochLengthBlocks)
	}
	return f.keeper.RecordAccountTransition(f.ctx.WithBlockHeight(height), transition)
}

func testAddress(t *testing.T, f *fixture, bz []byte) string {
	t.Helper()
	address, err := f.addressCodec.BytesToString(bz)
	require.NoError(t, err)
	return address
}

func TestAccountTransitionLineageBoundariesAndTwoHop(t *testing.T) {
	f := initFixture(t)
	old := testAddress(t, f, []byte{1, 2, 3, 4})
	mid := testAddress(t, f, []byte{5, 6, 7, 8})
	current := testAddress(t, f, []byte{9, 10, 11, 12})

	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: mid, EffectiveEpoch: 5}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: mid, DestinationAccount: current, EffectiveEpoch: 9}))

	for _, tc := range []struct {
		epoch uint64
		want  string
	}{{4, old}, {5, mid}, {8, mid}, {9, current}, {100, current}} {
		got, err := f.keeper.AccountForEpoch(f.ctx, old, tc.epoch)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
	got, err := f.keeper.CurrentAccount(f.ctx, old)
	require.NoError(t, err)
	require.Equal(t, current, got)
}

func TestAccountTransitionRejectsCycleForkAndOutOfOrder(t *testing.T) {
	f := initFixture(t)
	a := testAddress(t, f, []byte{1, 1, 1, 1})
	b := testAddress(t, f, []byte{2, 2, 2, 2})
	c := testAddress(t, f, []byte{3, 3, 3, 3})
	d := testAddress(t, f, []byte{4, 4, 4, 4})

	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: a, DestinationAccount: b, EffectiveEpoch: 5}))
	require.Error(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: a, DestinationAccount: c, EffectiveEpoch: 6}), "fork")
	require.Error(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: b, DestinationAccount: a, EffectiveEpoch: 6}), "cycle")
	require.Error(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: b, DestinationAccount: d, EffectiveEpoch: 4}), "epochs must increase")
}

func TestAccountTransitionMovesCurrentSingletonState(t *testing.T) {
	f := initFixture(t)
	old := testAddress(t, f, []byte{1, 2, 3, 4})
	current := testAddress(t, f, []byte{5, 6, 7, 8})
	f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{SupernodeAccount: old, SuspicionScore: 7})
	f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{ReporterSupernodeAccount: old, ReliabilityScore: 8})

	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 2}))
	_, found := f.keeper.GetNodeSuspicionState(f.ctx, old)
	require.False(t, found)
	ns, found := f.keeper.GetNodeSuspicionState(f.ctx, current)
	require.True(t, found)
	require.Equal(t, int64(7), ns.SuspicionScore)
	_, found = f.keeper.GetReporterReliabilityState(f.ctx, old)
	require.False(t, found)
	rr, found := f.keeper.GetReporterReliabilityState(f.ctx, current)
	require.True(t, found)
	require.Equal(t, int64(8), rr.ReliabilityScore)
}

func TestBuildAccountTransitionPlanRejectsEverySingletonCollisionAndMalformedSource(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   func(string) []byte
		valid []byte
	}{
		{"action marker", types.ActionFinalizationPostponementKey, binary.BigEndian.AppendUint64(nil, 4)},
		{"storage marker", types.StorageTruthPostponementKey, binary.BigEndian.AppendUint64(nil, 4)},
		{"strong marker", types.StorageTruthPostponementStrongKey, []byte{1}},
	} {
		t.Run(tc.name+" destination collision", func(t *testing.T) {
			f := initFixture(t)
			old := testAddress(t, f, []byte{51, 52, 53, 54})
			current := testAddress(t, f, []byte{61, 62, 63, 64})
			store := f.ctx.KVStore(f.storeKey)
			store.Set(tc.key(current), tc.valid)
			_, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 2})
			require.ErrorContains(t, err, "destination collision")
			require.False(t, store.Has(types.AccountTransitionForwardKey(old)))
		})
		t.Run(tc.name+" malformed source", func(t *testing.T) {
			f := initFixture(t)
			old := testAddress(t, f, []byte{71, 72, 73, 74})
			current := testAddress(t, f, []byte{81, 82, 83, 84})
			store := f.ctx.KVStore(f.storeKey)
			store.Set(tc.key(old), []byte{9, 9})
			_, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 2})
			require.ErrorContains(t, err, "malformed")
			require.False(t, store.Has(types.AccountTransitionForwardKey(old)))
		})
	}
}

func TestBuildAccountTransitionPlanRejectsMalformedProtobufSingletonsWithoutWriting(t *testing.T) {
	for _, key := range []func(string) []byte{types.NodeSuspicionStateKey, types.ReporterReliabilityStateKey} {
		f := initFixture(t)
		old := testAddress(t, f, []byte{91, 92, 93, 94})
		current := testAddress(t, f, []byte{101, 102, 103, 104})
		store := f.ctx.KVStore(f.storeKey)
		store.Set(key(old), []byte{0xff})
		_, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 2})
		require.ErrorContains(t, err, "malformed")
		require.False(t, store.Has(types.AccountTransitionForwardKey(old)))
	}
}
