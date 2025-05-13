package types

import (
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"
)

const (
	// ConsensusVersion is a sequence number for state-breaking change of the module.
	ConsensusVersion = 1

	// DefaultIndex is the default global index
	DefaultIndex uint64 = 1
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
		PortId: PortID,
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := host.PortIdentifierValidator(gs.PortId); err != nil {
		return err
	}

	return gs.Params.Validate()
}
