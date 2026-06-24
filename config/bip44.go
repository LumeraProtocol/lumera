package config

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmhd "github.com/cosmos/evm/crypto/hd"
)

// SetBip44CoinType sets the EVM BIP44 coin type (60) and purpose (44).
// This configures the chain to use Ethereum-compatible HD derivation paths.
func SetBip44CoinType(config *sdk.Config) {
	config.SetPurpose(sdk.Purpose)          // BIP44 purpose = 44
	config.SetCoinType(evmhd.Bip44CoinType) // Ethereum coin type = 60
}
