package types

import (
	"fmt"
)

// DefaultIndex is the default global index
const DefaultIndex uint64 = 1

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		PastelidEntryList: []PastelidEntry{},
		// this line is used by starport scaffolding # genesis/types/default
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	// Check for duplicated index in pastelidEntry
	pastelidEntryIndexMap := make(map[string]struct{})

	for _, elem := range gs.PastelidEntryList {
		index := string(PastelidEntryKey(elem.Address))
		if _, ok := pastelidEntryIndexMap[index]; ok {
			return fmt.Errorf("duplicated index for pastelidEntry")
		}
		pastelidEntryIndexMap[index] = struct{}{}
	}
	// this line is used by starport scaffolding # genesis/types/validate

	return gs.Params.Validate()
}
