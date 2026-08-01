package types

import "fmt"

const (
	// Per 122-F4 — bump KeepLastEpochEntries to cover OldClassAFaultWindow for safe pruning.
	ConsensusVersion = 3
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
	if err := ValidateAccountTransitions(gs.AccountTransitions); err != nil {
		return err
	}

	return nil
}

func ValidateAccountTransitions(transitions []AccountTransition) error {
	// This types-layer check is structural because no configured address codec
	// is available here. Keeper.InitGenesis additionally requires canonical
	// Bech32 endpoint text before importing any transition index.
	if len(transitions) > MaxAccountTransitions {
		return fmt.Errorf("account transitions exceed limit %d", MaxAccountTransitions)
	}
	forward := make(map[string]AccountTransition, len(transitions))
	reverse := make(map[string]AccountTransition, len(transitions))
	for _, transition := range transitions {
		if transition.SourceAccount == "" || transition.DestinationAccount == "" || transition.SourceAccount == transition.DestinationAccount || transition.EffectiveEpoch == 0 {
			return fmt.Errorf("invalid account transition")
		}
		if _, exists := forward[transition.SourceAccount]; exists {
			return fmt.Errorf("account transition fork at %q", transition.SourceAccount)
		}
		if _, exists := reverse[transition.DestinationAccount]; exists {
			return fmt.Errorf("account transition destination collision at %q", transition.DestinationAccount)
		}
		forward[transition.SourceAccount] = transition
		reverse[transition.DestinationAccount] = transition
	}
	for start := range forward {
		seen := map[string]struct{}{}
		account := start
		var previousEpoch uint64
		for {
			if _, exists := seen[account]; exists {
				return fmt.Errorf("account transition cycle")
			}
			seen[account] = struct{}{}
			transition, exists := forward[account]
			if !exists {
				break
			}
			if previousEpoch != 0 && transition.EffectiveEpoch <= previousEpoch {
				return fmt.Errorf("account transition epochs must strictly increase")
			}
			previousEpoch = transition.EffectiveEpoch
			account = transition.DestinationAccount
		}
	}
	return nil
}
