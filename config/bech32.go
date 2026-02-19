package config

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	// Bech32AccountAddressPrefix is the prefix for account addresses.
	Bech32AccountAddressPrefix = "lumera"

	// Bech32PrefixValidator is the suffix used for validator Bech32 prefixes.
	Bech32PrefixValidator = "val"
	// Bech32PrefixConsensus is the suffix used for consensus Bech32 prefixes.
	Bech32PrefixConsensus = "cons"
	// Bech32PrefixPublic is the suffix used for public key Bech32 prefixes.
	Bech32PrefixPublic = "pub"
	// Bech32PrefixOperator is the suffix used for operator Bech32 prefixes.
	Bech32PrefixOperator = "oper"

	// Bech32AccountPrefixPub defines the Bech32 prefix of an account public key.
	Bech32AccountPrefixPub = Bech32AccountAddressPrefix + Bech32PrefixPublic
	// Bech32ValidatorAddressPrefix defines the Bech32 prefix of a validator operator address.
	Bech32ValidatorAddressPrefix = Bech32AccountAddressPrefix + Bech32PrefixValidator + Bech32PrefixOperator
	// Bech32ValidatorAddressPrefixPub defines the Bech32 prefix of a validator operator public key.
	Bech32ValidatorAddressPrefixPub = Bech32AccountAddressPrefix + Bech32PrefixValidator + Bech32PrefixOperator + Bech32PrefixPublic
	// Bech32ConsNodeAddressPrefix defines the Bech32 prefix of a consensus node address.
	Bech32ConsNodeAddressPrefix = Bech32AccountAddressPrefix + Bech32PrefixValidator + Bech32PrefixConsensus
	// Bech32ConsNodeAddressPrefixPub defines the Bech32 prefix of a consensus node public key.
	Bech32ConsNodeAddressPrefixPub = Bech32AccountAddressPrefix + Bech32PrefixValidator + Bech32PrefixConsensus + Bech32PrefixPublic
)

// SetBech32Prefixes sets Bech32 prefixes for account, validator, and consensus node types.
func SetBech32Prefixes(config *sdk.Config) {
	config.SetBech32PrefixForAccount(Bech32AccountAddressPrefix, Bech32AccountPrefixPub)
	config.SetBech32PrefixForValidator(Bech32ValidatorAddressPrefix, Bech32ValidatorAddressPrefixPub)
	config.SetBech32PrefixForConsensusNode(Bech32ConsNodeAddressPrefix, Bech32ConsNodeAddressPrefixPub)
}
