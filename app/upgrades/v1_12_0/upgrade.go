package v1_12_0

import (
	storetypes "cosmossdk.io/store/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.12.0"

// StoreUpgrades declares store additions for this upgrade.
var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{
		feemarkettypes.StoreKey,   // added EVM fee market store key
		precisebanktypes.StoreKey, // added EVM precise bank store key
		evmtypes.StoreKey,         // added EVM state store key
		erc20types.StoreKey,       // added ERC20 token pairs store key
	},
}
