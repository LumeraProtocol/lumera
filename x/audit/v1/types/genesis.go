package types

import "fmt"

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
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	seenEvidenceIDs := make(map[uint64]struct{}, len(gs.Evidence))
	for _, evidence := range gs.Evidence {
		if _, found := seenEvidenceIDs[evidence.EvidenceId]; found {
			return fmt.Errorf("duplicate evidence_id %d in genesis", evidence.EvidenceId)
		}
		seenEvidenceIDs[evidence.EvidenceId] = struct{}{}
	}

	seenHealOpIDs := make(map[uint64]struct{}, len(gs.HealOps))
	for _, healOp := range gs.HealOps {
		if _, found := seenHealOpIDs[healOp.HealOpId]; found {
			return fmt.Errorf("duplicate heal_op_id %d in genesis", healOp.HealOpId)
		}
		seenHealOpIDs[healOp.HealOpId] = struct{}{}
	}

	return nil
}
