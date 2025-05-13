package app

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
)

func init() {
	// Set and seal config
	config := sdk.GetConfig()
	config.SetCoinType(ChainCoinType)
	config.SetBech32PrefixForAccount(AccountAddressPrefix, AccountPrefixPub)
	config.SetBech32PrefixForValidator(ValidatorAddressPrefix, ValidatorAddressPrefixPub)
	config.SetBech32PrefixForConsensusNode(ConsNodeAddressPrefix, ConsNodeAddressPrefixPub)
	config.Seal()
}
