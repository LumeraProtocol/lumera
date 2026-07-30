package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestAccountTransitionsGenesisRoundTrip(t *testing.T) {
	f := initFixture(t)
	a := testAddress(t, f, []byte{31, 32, 33, 34})
	b := testAddress(t, f, []byte{41, 42, 43, 44})
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: a, DestinationAccount: b, EffectiveEpoch: 3}))

	exported, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Equal(t, []types.AccountTransition{{SourceAccount: a, DestinationAccount: b, EffectiveEpoch: 3}}, exported.AccountTransitions)
}

func TestGenesisRejectsInvalidAccountTransitionGraph(t *testing.T) {
	genesis := types.DefaultGenesis()
	genesis.AccountTransitions = []types.AccountTransition{
		{SourceAccount: "a", DestinationAccount: "b", EffectiveEpoch: 3},
		{SourceAccount: "b", DestinationAccount: "a", EffectiveEpoch: 4},
	}
	require.Error(t, genesis.Validate())
}

func TestGenesisRejectsLiveSingletonAtTransitionSource(t *testing.T) {
	f := initFixture(t)
	source := testAddress(t, f, []byte{51, 52, 53, 54})
	destination := testAddress(t, f, []byte{61, 62, 63, 64})
	genesis := types.DefaultGenesis()
	genesis.AccountTransitions = []types.AccountTransition{{SourceAccount: source, DestinationAccount: destination, EffectiveEpoch: 2}}
	genesis.NodeSuspicionStates = []types.NodeSuspicionState{{SupernodeAccount: source, SuspicionScore: 7}}
	err := f.keeper.InitGenesis(f.ctx, *genesis)
	require.ErrorContains(t, err, "non-current transition source")
	require.False(t, f.ctx.KVStore(f.storeKey).Has(types.AccountTransitionForwardKey(source)), "validation fails before importing lineage")
	require.False(t, f.ctx.KVStore(f.storeKey).Has(types.NodeSuspicionStateKey(source)), "validation fails before singleton writes")
}
