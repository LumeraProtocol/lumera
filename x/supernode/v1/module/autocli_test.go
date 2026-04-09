package supernode

import "testing"

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
