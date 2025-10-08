package config

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	// AccountAddressPrefix is the prefix for accounts addresses.
	AccountAddressPrefix = "lumera"

	// PrefixValidator is the prefix for validator keys
	PrefixValidator = "val"
	// PrefixConsensus is the prefix for consensus keys
	PrefixConsensus = "cons"
	// PrefixPublic is the prefix for public keys
	PrefixPublic = "pub"
	// PrefixOperator is the prefix for operator keys
	PrefixOperator = "oper"

	// AccountPrefixPub defines the Bech32 prefix of an account's public key
	AccountPrefixPub = AccountAddressPrefix + PrefixPublic
	// ValidatorAddressPrefix defines the Bech32 prefix of a validator's operator address
	ValidatorAddressPrefix = AccountAddressPrefix + PrefixValidator + PrefixOperator
	// ValidatorAddressPrefixPub defines the Bech32 prefix of a validator's operator public key
	ValidatorAddressPrefixPub = AccountAddressPrefix + PrefixValidator + PrefixOperator + PrefixPublic
	// ConsNodeAddressPrefix defines the Bech32 prefix of a consensus node address
	ConsNodeAddressPrefix = AccountAddressPrefix + PrefixValidator + PrefixConsensus
	// ConsNodeAddressPrefixPub defines the Bech32 prefix of a consensus node public key
	ConsNodeAddressPrefixPub = AccountAddressPrefix + PrefixValidator + PrefixConsensus + PrefixPublic

	// DefaultMaxIBCCallbackGas is the default value of maximum gas that an IBC callback can use.
	// If the callback uses more gas, it will be out of gas and the contract state changes will be reverted,
	// but the transaction will be committed.
	// Pass this to the callbacks middleware or choose a custom value.
	DefaultMaxIBCCallbackGas = uint64(1_000_000)

	// ChainCoinType is the coin type of the chain.
	ChainCoinType = 118

	// ChainDenom is the denomination of the chain's native token.
	ChainDenom = "ulume"
)

func SetupConfig() {
	// Set and seal config
	config := sdk.GetConfig()

	// Set the chain coin type
	config.SetCoinType(ChainCoinType)

	// Set the Bech32 prefixes for accounts, validators, and consensus nodes
	config.SetBech32PrefixForAccount(AccountAddressPrefix, AccountPrefixPub)
	config.SetBech32PrefixForValidator(ValidatorAddressPrefix, ValidatorAddressPrefixPub)
	config.SetBech32PrefixForConsensusNode(ConsNodeAddressPrefix, ConsNodeAddressPrefixPub)

	// Seal the config to prevent further modifications
	sdk.GetConfig().Seal()
}

func init() {
	SetupConfig()
}
