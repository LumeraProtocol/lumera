package supernode

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/autoclitest"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
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

func TestAutoCLIOptions_QueryGetMetricsIsCustom(t *testing.T) {
	am := AppModule{}
	opts := am.AutoCLIOptions()
	if opts == nil || opts.Query == nil {
		t.Fatalf("query autocli options must be set")
	}
	if !opts.Query.EnhanceCustomCommand {
		t.Fatalf("query EnhanceCustomCommand must be true")
	}

	found := false
	for _, rpc := range opts.Query.RpcCommandOptions {
		if rpc.RpcMethod == "GetMetrics" {
			found = true
			if !rpc.Skip {
				t.Fatalf("GetMetrics must be skipped in AutoCLI")
			}
		}
	}
	if !found {
		t.Fatalf("GetMetrics rpc option not found")
	}
}

func TestAutoCLIOptions_ReportSupernodeMetrics(t *testing.T) {
	opts := AppModule{}.AutoCLIOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.Tx)

	var found bool
	for _, rpc := range opts.Tx.RpcCommandOptions {
		if rpc.GetRpcMethod() != "ReportSupernodeMetrics" {
			continue
		}

		found = true
		require.Equal(t, "report-supernode-metrics [validator-address]", rpc.GetUse())
		require.Len(t, rpc.GetPositionalArgs(), 1)
		require.Equal(t, "validator_address", rpc.GetPositionalArgs()[0].GetProtoField())
	}

	require.True(t, found, "ReportSupernodeMetrics tx should be exposed via AutoCLI")
}
