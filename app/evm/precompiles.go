package evm

import (
	actionprecompile "github.com/LumeraProtocol/lumera/precompiles/action"
	supernodeprecompile "github.com/LumeraProtocol/lumera/precompiles/supernode"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// LumeraActiveStaticPrecompiles lists static precompile addresses that are both
// enabled in genesis params and registered in the keeper precompile map.
//
// NOTE: Vesting precompile is intentionally excluded because Cosmos EVM's
// DefaultStaticPrecompiles registry does not currently install an implementation
// for evmtypes.VestingPrecompileAddress in v0.5.1.
var LumeraActiveStaticPrecompiles = []string{
	evmtypes.P256PrecompileAddress,
	evmtypes.Bech32PrecompileAddress,
	evmtypes.StakingPrecompileAddress,
	evmtypes.DistributionPrecompileAddress,
	evmtypes.ICS20PrecompileAddress,
	evmtypes.BankPrecompileAddress,
	evmtypes.GovPrecompileAddress,
	evmtypes.SlashingPrecompileAddress,
	// Lumera custom precompiles
	actionprecompile.ActionPrecompileAddress,
	supernodeprecompile.SupernodePrecompileAddress,
}
