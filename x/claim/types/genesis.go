package types

// DefaultIndex is the default global index
const DefaultIndex uint64 = 1

const DefaultClaimableAmountConst = 18_749_999_991_853

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {

	// The claim records are directly loaded from the CSV file into the genesis state,
	// at no point in time, they become part of the genesis file
	return &GenesisState{
		Params:               DefaultParams(),
		ClaimRecords:         []ClaimRecord{}, //representation only
		TotalClaimableAmount: DefaultClaimableAmountConst,
		ClaimsDenom:          DefaultClaimsDenom,
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
func NewGenesisState(params Params, records []ClaimRecord, amount uint64, claimDenom string) *GenesisState {
	return &GenesisState{
		Params:               params,
		ClaimRecords:         records,
		TotalClaimableAmount: amount,
		ClaimsDenom:          claimDenom,
	}
}
