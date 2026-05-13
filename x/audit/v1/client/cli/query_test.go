package cli

import "testing"

func TestGetCustomQueryCmd_ContainsFloatReportCommands(t *testing.T) {
	cmd := GetCustomQueryCmd()
	if cmd == nil {
		t.Fatalf("expected non-nil command")
	}

	wanted := map[string]bool{
		"epoch-report":              false,
		"epoch-reports-by-reporter": false,
		"host-reports":              false,
	}

	for _, c := range cmd.Commands() {
		if _, ok := wanted[c.Name()]; ok {
			wanted[c.Name()] = true
		}
	}

	for name, found := range wanted {
		if !found {
			t.Fatalf("expected custom query command %q", name)
		}
	}
}
