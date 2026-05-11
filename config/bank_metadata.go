package config

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// ChainBankMetadata returns the canonical bank metadata for Lumera's native
// denominations (base, display, and extended EVM unit).
func ChainBankMetadata() banktypes.Metadata {
	return banktypes.Metadata{
		Description: "The native staking token of the Lumera network",
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: ChainDenom, Exponent: 0},
			{Denom: ChainDisplayDenom, Exponent: 6},
			{Denom: ChainEVMExtendedDenom, Exponent: 18},
		},
		Base:    ChainDenom,
		Display: ChainDisplayDenom,
		Name:    ChainTokenName,
		Symbol:  ChainTokenSymbol,
	}
}

// UpsertChainBankMetadata inserts (or replaces) Lumera's denom metadata entry.
// It replaces any entry keyed by the chain base denom and also the SDK default
// bond denom to handle legacy/default genesis templates.
func UpsertChainBankMetadata(metadata []banktypes.Metadata) []banktypes.Metadata {
	chainMetadata := ChainBankMetadata()
	for i, md := range metadata {
		if md.Base == ChainDenom || md.Base == sdk.DefaultBondDenom {
			metadata[i] = chainMetadata
			return metadata
		}
	}

	return append(metadata, chainMetadata)
}
