package types

const (
	// Per 122-F4 — bump KeepLastEpochEntries to cover OldClassAFaultWindow for safe pruning.
	ConsensusVersion = 2
)

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:         DefaultParams(),
		NextEvidenceId: 1,
		NextHealOpId:   1,
	}
}

func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}
