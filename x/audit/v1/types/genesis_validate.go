package types

import "fmt"

// ValidateScoreStatesGenesis hard-errors on score states that are malformed
// relative to the current epoch. Per 119-F8 / 119-F12 + NEW-A-12.
func ValidateScoreStatesGenesis(g GenesisState, currentEpoch uint64) error {
	for _, s := range g.NodeSuspicionStates {
		if s.LastUpdatedEpoch > currentEpoch {
			return fmt.Errorf("node suspicion %q has LastUpdatedEpoch %d > current %d",
				s.SupernodeAccount, s.LastUpdatedEpoch, currentEpoch)
		}
		if s.WindowStartEpoch > currentEpoch {
			return fmt.Errorf("node suspicion %q has WindowStartEpoch %d > current %d",
				s.SupernodeAccount, s.WindowStartEpoch, currentEpoch)
		}
		if s.SuspicionScore < 0 {
			return fmt.Errorf("node suspicion %q negative score %d", s.SupernodeAccount, s.SuspicionScore)
		}
	}
	for _, s := range g.ReporterReliabilityStates {
		if s.LastUpdatedEpoch > currentEpoch {
			return fmt.Errorf("reporter reliability %q has LastUpdatedEpoch %d > current %d",
				s.ReporterSupernodeAccount, s.LastUpdatedEpoch, currentEpoch)
		}
		if s.WindowStartEpoch > currentEpoch {
			return fmt.Errorf("reporter reliability %q has WindowStartEpoch %d > current %d",
				s.ReporterSupernodeAccount, s.WindowStartEpoch, currentEpoch)
		}
	}
	for _, s := range g.TicketDeteriorationStates {
		if s.LastUpdatedEpoch > currentEpoch {
			return fmt.Errorf("ticket deterioration %q has LastUpdatedEpoch %d > current %d",
				s.TicketId, s.LastUpdatedEpoch, currentEpoch)
		}
	}
	return nil
}
