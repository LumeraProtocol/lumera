//go:build test

package upgrades

import (
	"testing"

	"github.com/stretchr/testify/require"

	upgrade_v1_20_2 "github.com/LumeraProtocol/lumera/app/upgrades/v1_20_2"
)

// TestV1202IsRecognizedAsKnownUpgrade is the binary-level proof that a node
// built from this tree will actually accept a governance plan named "v1.20.2".
//
// This is the check that matters operationally. If SetupUpgrades does not
// return found=true for the plan name, the chain halts at the upgrade height
// with "upgrade plan not registered" and does not resume until an operator
// swaps in a binary that knows the name. Asserting the constant alone would not
// catch a missing case arm in the switch.
func TestV1202IsRecognizedAsKnownUpgrade(t *testing.T) {
	const planName = "v1.20.2"

	require.Equal(t, planName, upgrade_v1_20_2.UpgradeName,
		"the on-chain plan name and the package constant must not drift")

	require.Contains(t, upgradeNames, planName,
		"v1.20.2 must be in upgradeNames or the node will reject the gov plan")

	for _, chainID := range []string{"lumera-mainnet-1", "lumera-testnet-2", "lumera-devnet-1"} {
		cfg, found := SetupUpgrades(planName, newTestUpgradeParams(chainID))
		require.Truef(t, found,
			"node on %s must recognize plan %q; otherwise it halts at the upgrade "+
				"height with 'upgrade plan not registered'", chainID, planName)
		require.NotNilf(t, cfg.Handler,
			"plan %q must resolve to a real handler on %s", planName, chainID)
	}
}
