package action

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/autoclitest"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/stretchr/testify/require"
)

func TestAutoCLIOptions_CoversAllRPCs(t *testing.T) {
	opts := AppModule{}.AutoCLIOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.Query)
	require.NotNil(t, opts.Tx)

	autoclitest.AssertServiceMethodsCovered(t, types.Query_serviceDesc, opts.Query.RpcCommandOptions)
	autoclitest.AssertServiceMethodsCovered(t, types.Msg_serviceDesc, opts.Tx.RpcCommandOptions, "UpdateParams")
}
