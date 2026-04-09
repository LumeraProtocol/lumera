package supernode

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAutoCLIOptions_GetMetrics(t *testing.T) {
	opts := AppModule{}.AutoCLIOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.Query)

	var found bool
	for _, rpc := range opts.Query.RpcCommandOptions {
		if rpc.GetRpcMethod() != "GetMetrics" {
			continue
		}

		found = true
		require.Equal(t, "get-metrics [validator-address]", rpc.GetUse())
		require.Len(t, rpc.GetPositionalArgs(), 1)
		require.Equal(t, "validatorAddress", rpc.GetPositionalArgs()[0].GetProtoField())
	}

	require.True(t, found, "GetMetrics query should be exposed via AutoCLI")
}
