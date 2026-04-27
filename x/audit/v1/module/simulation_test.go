package audit

import (
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func TestWeightedOperationsIncludesSubmitEvidence(t *testing.T) {
	am := AppModule{}

	simState := module.SimulationState{
		AppParams: make(simtypes.AppParams),
	}

	ops := am.WeightedOperations(simState)
	require.Len(t, ops, 5)

	msg, futureOps, err := ops[0].Op()(rand.New(rand.NewSource(1)), nil, sdk.Context{}, []simtypes.Account{}, "testing")
	require.NoError(t, err)
	require.Empty(t, futureOps)
	require.Equal(t, audittypes.ModuleName, msg.Route)
	require.Equal(t, sdk.MsgTypeURL(&audittypes.MsgSubmitEvidence{}), msg.Name)
	require.False(t, msg.OK)
}

func TestWeightedOperationsIncludesStorageTruthOps(t *testing.T) {
	am := AppModule{}

	simState := module.SimulationState{
		AppParams: make(simtypes.AppParams),
	}

	ops := am.WeightedOperations(simState)
	require.Len(t, ops, 5)

	wantRoutes := []string{
		sdk.MsgTypeURL(&audittypes.MsgSubmitEvidence{}),
		sdk.MsgTypeURL(&audittypes.MsgSubmitStorageRecheckEvidence{}),
		sdk.MsgTypeURL(&audittypes.MsgClaimHealComplete{}),
		sdk.MsgTypeURL(&audittypes.MsgSubmitHealVerification{}),
	}

	for i, want := range wantRoutes {
		msg, futureOps, err := ops[i].Op()(rand.New(rand.NewSource(int64(i))), nil, sdk.Context{}, []simtypes.Account{}, "testing")
		require.NoError(t, err, "op %d (%s) returned error", i, want)
		require.Empty(t, futureOps, "op %d must not schedule future ops", i)
		require.Equal(t, audittypes.ModuleName, msg.Route, "op %d route mismatch", i)
		require.Equal(t, want, msg.Name, "op %d name mismatch", i)
		require.False(t, msg.OK, "all audit sim ops are no-ops")
	}
}
