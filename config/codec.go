package config

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	evmcryptocodec "github.com/cosmos/evm/crypto/codec"
)

// RegisterExtraInterfaces registers non-module interfaces that are not covered by SDK module wiring.
// This includes both standard Cosmos crypto codecs and EVM-specific crypto codecs.
// Note: When used via depinject in the main app, cryptocodec is already registered by the runtime,
// but we include it here for standalone use cases (tests, faucet, etc.).
func RegisterExtraInterfaces(interfaceRegistry codectypes.InterfaceRegistry) {
	if interfaceRegistry == nil {
		return
	}

	// Register standard Cosmos crypto interfaces (secp256k1, ed25519, etc.)
	cryptocodec.RegisterInterfaces(interfaceRegistry)

	// Register EVM crypto interfaces (eth_secp256k1)
	evmcryptocodec.RegisterInterfaces(interfaceRegistry)
}
