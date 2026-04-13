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
	require.Len(t, ops, 2)

	msg, futureOps, err := ops[0].Op()(rand.New(rand.NewSource(1)), nil, sdk.Context{}, []simtypes.Account{}, "testing")
	require.NoError(t, err)
	require.Empty(t, futureOps)
	require.Equal(t, audittypes.ModuleName, msg.Route)
	require.Equal(t, sdk.MsgTypeURL(&audittypes.MsgSubmitEvidence{}), msg.Name)
	require.False(t, msg.OK)
}
