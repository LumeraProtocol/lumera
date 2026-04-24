package cli

import "testing"

func TestGetCustomQueryCmd_ContainsGetMetrics(t *testing.T) {
	cmd := GetCustomQueryCmd()
	if cmd == nil {
		t.Fatalf("expected non-nil command")
	}

	found := false
	for _, c := range cmd.Commands() {
		if c.Name() == "get-metrics" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected custom get-metrics command")
	}
}
