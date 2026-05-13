package simulation

import (
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func TestSimulateMsgSubmitEvidenceNoOpMessage(t *testing.T) {
	// Keepers/tx config are not used in this operation today.
	op := SimulateMsgSubmitEvidence(nil, nil, keeper.Keeper{}, nil)

	msg, futureOps, err := op(rand.New(rand.NewSource(1)), nil, sdk.Context{}, []simtypes.Account{}, "testing")
	require.NoError(t, err)
	require.Empty(t, futureOps)
	require.Equal(t, types.ModuleName, msg.Route)
	require.Equal(t, sdk.MsgTypeURL(&types.MsgSubmitEvidence{}), msg.Name)
	require.False(t, msg.OK)
	require.Contains(t, msg.Comment, "no public evidence types are available")
}
