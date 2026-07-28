package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVoteAllComposePathIsScriptRelative(t *testing.T) {
	scriptPath, err := filepath.Abs("../../scripts/vote-all.sh")
	if err != nil {
		t.Fatalf("resolve vote script path: %v", err)
	}
	contents, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read vote script: %v", err)
	}
	script := string(contents)
	for _, required := range []string{
		`SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`,
		`DEVNET_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"`,
		`COMPOSE_FILE="${DEVNET_ROOT}/docker-compose.yml"`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("vote script must contain %q", required)
		}
	}
	if strings.Contains(script, `COMPOSE_FILE="../docker-compose.yml"`) {
		t.Fatal("vote script must not resolve compose path from caller working directory")
	}
}
