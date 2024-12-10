package types

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// DefaultIndex is the default global index
const DefaultIndex uint64 = 1

const claimableAmount = 99999999

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {

	// Create module account
	moduleAcc := authtypes.NewEmptyModuleAccount(
		ModuleName,
		authtypes.Minter,
		authtypes.Burner,
	)

	// The claim records are directly loaded from the CSV file into the genesis state,
	// at no point in time, they become part of the genesis file
	return &GenesisState{
		Params:               DefaultParams(),
		ClaimRecords:         []ClaimRecord{}, //representation only
		ModuleAccount:        moduleAcc.String(),
		TotalClaimableAmount: claimableAmount,
	}
}

func (gs GenesisState) Validate() error {
	err := gs.Params.Validate()
	if err != nil {
		return err
	}

	return nil
}

// NewGenesisState creates a new genesis state with provided values
func NewGenesisState(params Params, records []ClaimRecord, moduleAcc *authtypes.ModuleAccount, amount uint64) *GenesisState {
	return &GenesisState{
		Params:               params,
		ClaimRecords:         records,
		ModuleAccount:        moduleAcc.String(),
		TotalClaimableAmount: amount,
	}
}
