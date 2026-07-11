package common

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestUpgradeProposalRestoresKeyForPersistedMigratedAddress(t *testing.T) {
	scriptPath, err := filepath.Abs("../../scripts/submit-upgrade-proposal.sh")
	if err != nil {
		t.Fatalf("resolve proposal script path: %v", err)
	}

	const testScript = `
source "$1"
GOV_ADDRESS_FILE="$(mktemp)"
trap 'rm -f "$GOV_ADDRESS_FILE"' EXIT
printf '%s\n' 'lumera1migrated' >"$GOV_ADDRESS_FILE"

key_address() { return 0; }
account_exists() { [[ "$1" == "lumera1migrated" ]]; }
key_name_for_address() { return 0; }
registry_mnemonic() { printf '%s\n' 'fixture mnemonic'; }
governance_mnemonic_file() { printf '%s\n' '/does/not/exist'; }
recover_evm_key_from_mnemonic() {
  [[ "$1" == "governance_key_evm" ]]
  [[ "$2" == "fixture mnemonic" ]]
  [[ "$3" == "lumera1migrated" ]]
}

resolve_proposer
[[ "$PROPOSER_KEY_NAME" == "governance_key_evm" ]]
[[ "$PROPOSER_ADDRESS" == "lumera1migrated" ]]
`
	cmd := exec.Command("bash", "-c", testScript, "upgrade-proposal-test", scriptPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("proposal key recovery scenario failed: %v\n%s", err, out)
	}
}
