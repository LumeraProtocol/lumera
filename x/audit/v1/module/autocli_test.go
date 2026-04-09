package audit

import "testing"

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
	}

	for method, skipped := range wantSkipped {
		if !skipped {
			t.Fatalf("expected %s to be skipped in AutoCLI query options", method)
		}
	}
}
