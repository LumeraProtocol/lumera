package evm

import (
	"cosmossdk.io/x/tx/signing"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// Configure is a no-op placeholder. The EVM global configuration (coin info,
// EIP activators, chain config) is set by the x/vm module itself:
//   - On first chain init: InitGenesis -> SetGlobalConfigVariables
//   - On node restart: PreBlock -> SetGlobalConfigVariables
//
// The keeper's WithDefaultEvmCoinInfo provides fallback values before genesis init.
// The genesis params (overridden in DefaultGenesis) ensure the correct Lumera denoms.
func Configure() error { return nil }

// ProvideCustomGetSigners returns the custom GetSigner implementations required
// by EVM message types (e.g. MsgEthereumTx) that don't use the standard
// cosmos.msg.v1.signer proto annotation. These are collected by depinject into
// the []signing.CustomGetSigner slice consumed by runtime.ProvideInterfaceRegistry.
func ProvideCustomGetSigners() signing.CustomGetSigner {
	return signing.CustomGetSigner{
		MsgType: evmtypes.MsgEthereumTxCustomGetSigner.MsgType,
		Fn:      evmtypes.MsgEthereumTxCustomGetSigner.Fn,
	}
}
