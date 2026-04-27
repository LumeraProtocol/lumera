package audit

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/autoclitest"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
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

func TestAutoCLIOptions_QueryFloatCommandsAreCustom(t *testing.T) {
	am := AppModule{}
	opts := am.AutoCLIOptions()
	if opts == nil || opts.Query == nil {
		t.Fatalf("query autocli options must be set")
	}
	if !opts.Query.EnhanceCustomCommand {
		t.Fatalf("query EnhanceCustomCommand must be true")
	}

	wantSkipped := map[string]bool{
		"EpochReport":            false,
		"EpochReportsByReporter": false,
		"HostReports":            false,
	}

	for _, rpc := range opts.Query.RpcCommandOptions {
		if _, ok := wantSkipped[rpc.RpcMethod]; ok {
			wantSkipped[rpc.RpcMethod] = rpc.Skip
		}
		if rpc.RpcMethod == "AssignedTargets" {
			require.Len(t, rpc.PositionalArgs, 1)
			require.Equal(t, "supernode_account", rpc.PositionalArgs[0].ProtoField)
		}
	}

	for method, skipped := range wantSkipped {
		if !skipped {
			t.Fatalf("expected %s to be skipped in AutoCLI query options", method)
		}
	}
}
