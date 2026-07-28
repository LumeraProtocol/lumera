package common

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAccountRegistryContainingMnemonicsIsOwnerOnly(t *testing.T) {
	scriptPath, err := filepath.Abs("../../scripts/account-registry.sh")
	if err != nil {
		t.Fatalf("resolve account registry script path: %v", err)
	}

	const scenario = `
set -euo pipefail
source "$1"
accounts_registry_init "$2"

ensure_accounts_registry
[[ "$(stat -c '%a' "$ACCOUNTS_FILE")" == "600" ]]

chmod 644 "$ACCOUNTS_FILE"
ensure_accounts_registry
[[ "$(stat -c '%a' "$ACCOUNTS_FILE")" == "600" ]]

accounts_registry_upsert test-account lumera1fixture 'fixture mnemonic' cosmos 1ulume genesis ABC123
[[ "$(stat -c '%a' "$ACCOUNTS_FILE")" == "600" ]]
jq -e '. == [{name:"test-account",address:"lumera1fixture",mnemonic:"fixture mnemonic",type:"cosmos",funded:{display_amount:"0.000001",display_denom:"lume",base_amount:"1",base_denom:"ulume"},funding_key:"genesis",funding_txhash:"ABC123",created_at:(.[] | .created_at)}]' "$ACCOUNTS_FILE" >/dev/null
`
	cmd := exec.Command("bash", "-c", scenario, "account-registry-permissions-test", scriptPath, t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("account registry must remain owner-only across every write path: %v\n%s", err, out)
	}
}
