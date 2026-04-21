package types

import "fmt"

// this line is used by starport scaffolding # genesis/types/import

// DefaultIndex is the default global index
const DefaultIndex uint64 = 1

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		// this line is used by starport scaffolding # genesis/types/default
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs *GenesisState) Validate() error {
	// this line is used by starport scaffolding # genesis/types/validate
	if gs.LastDistributionHeight < 0 {
		return fmt.Errorf("last_distribution_height must be >= 0")
	}

	return gs.Params.Validate()
}
