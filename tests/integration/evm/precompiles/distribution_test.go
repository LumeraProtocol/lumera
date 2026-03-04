//go:build integration
// +build integration

package precompiles_test

import (
	"strings"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	cmn "github.com/cosmos/evm/precompiles/common"
	distributionprecompile "github.com/cosmos/evm/precompiles/distribution"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestDistributionPrecompileQueryPathsViaEthCall verifies key read-only
// distribution precompile methods (withdraw address + community pool).
func testDistributionPrecompileQueryPathsViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	delegatorHex := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())

	withdrawAddrInput, err := distributionprecompile.ABI.Pack(distributionprecompile.DelegatorWithdrawAddressMethod, delegatorHex)
	if err != nil {
		t.Fatalf("pack delegatorWithdrawAddress input: %v", err)
	}

	withdrawAddrResult := mustEthCallPrecompile(t, node, evmtypes.DistributionPrecompileAddress, withdrawAddrInput)
	out, err := distributionprecompile.ABI.Unpack(distributionprecompile.DelegatorWithdrawAddressMethod, withdrawAddrResult)
	if err != nil {
		t.Fatalf("unpack delegatorWithdrawAddress output: %v", err)
	}
	withdrawAddr, ok := out[0].(string)
	if !ok {
		t.Fatalf("unexpected delegatorWithdrawAddress output type: %#v", out)
	}
	if withdrawAddr != node.KeyInfo().Address {
		t.Fatalf("unexpected withdraw address: got=%s want=%s", withdrawAddr, node.KeyInfo().Address)
	}

	communityPoolInput, err := distributionprecompile.ABI.Pack(distributionprecompile.CommunityPoolMethod)
	if err != nil {
		t.Fatalf("pack communityPool input: %v", err)
	}

	communityPoolResult := mustEthCallPrecompile(t, node, evmtypes.DistributionPrecompileAddress, communityPoolInput)
	var cpOut struct {
		Coins []cmn.DecCoin `abi:"coins"`
	}
	if err := distributionprecompile.ABI.UnpackIntoInterface(&cpOut, distributionprecompile.CommunityPoolMethod, communityPoolResult); err != nil {
		t.Fatalf("unpack communityPool output: %v", err)
	}
	// Empty pool is valid; just ensure decoded entries are structurally valid.
	for _, coin := range cpOut.Coins {
		if strings.TrimSpace(coin.Denom) == "" {
			t.Fatalf("communityPool contains empty denom entry: %#v", cpOut.Coins)
		}
	}
}
