package app

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	evmethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"
)

// registerLumeraLegacyAminoCodec wires Cosmos EVM crypto amino types into the
// app-level LegacyAmino codec and updates SDK global legacy.Cdc.
func registerLumeraLegacyAminoCodec(cdc *codec.LegacyAmino) {
	if cdc == nil {
		return
	}

	// Match Cosmos EVM behavior for EVM key support in legacy Amino paths:
	// register eth_secp256k1 concrete key types and sync SDK global legacy.Cdc.
	//
	// Note: unlike evmd, Lumera's depinject app wiring already pre-registers SDK
	// crypto Amino types, so we avoid re-registering full SDK crypto set here.
	cdc.RegisterConcrete(&evmethsecp256k1.PubKey{}, evmethsecp256k1.PubKeyName, nil)
	cdc.RegisterConcrete(&evmethsecp256k1.PrivKey{}, evmethsecp256k1.PrivKeyName, nil)
	legacy.Cdc = cdc
}
