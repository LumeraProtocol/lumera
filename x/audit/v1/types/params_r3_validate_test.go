package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// CP-R3 C-F1 — StrongRecoveryCleanPassCount must be validated.
func TestParamsValidate_StrongRecoveryCleanPassCount(t *testing.T) {
	base := DefaultParams()

	t.Run("zero rejected", func(t *testing.T) {
		p := base
		p.StorageTruthStrongRecoveryCleanPassCount = 0
		require.ErrorContains(t, p.Validate(), "storage_truth_strong_recovery_clean_pass_count must be > 0")
	})

	t.Run("less than normal recovery rejected", func(t *testing.T) {
		p := base
		p.StorageTruthRecoveryCleanPassCount = 5
		p.StorageTruthStrongRecoveryCleanPassCount = 3
		require.ErrorContains(t, p.Validate(), "must be >= storage_truth_recovery_clean_pass_count")
	})

	t.Run("equal to normal recovery accepted", func(t *testing.T) {
		p := base
		p.StorageTruthRecoveryCleanPassCount = 3
		p.StorageTruthStrongRecoveryCleanPassCount = 3
		require.NoError(t, p.Validate())
	})
}

// CP-R3 C-F2 — HealVerifierCount must have an upper bound (gov DoS surface).
func TestParamsValidate_HealVerifierCountUpperBound(t *testing.T) {
	base := DefaultParams()

	t.Run("zero rejected", func(t *testing.T) {
		p := base
		p.StorageTruthHealVerifierCount = 0
		require.ErrorContains(t, p.Validate(), "must be within 1..32")
	})

	t.Run("over cap rejected", func(t *testing.T) {
		p := base
		p.StorageTruthHealVerifierCount = 33
		require.ErrorContains(t, p.Validate(), "must be within 1..32")
	})

	t.Run("at cap accepted", func(t *testing.T) {
		p := base
		p.StorageTruthHealVerifierCount = 32
		require.NoError(t, p.Validate())
	})
}

// CP-R3 C-F3 — KeepLastEpochEntries must be >= DivergenceWindowEpochs and HealDeadlineEpochs.
func TestParamsValidate_KeepLastEpochEntriesCoversDivergenceAndHealWindows(t *testing.T) {
	t.Run("KeepLast < DivergenceWindow rejected", func(t *testing.T) {
		p := DefaultParams()
		p.StorageTruthDivergenceWindowEpochs = 30
		p.KeepLastEpochEntries = 7
		require.ErrorContains(t, p.Validate(), "keep_last_epoch_entries must be >= max epoch lookback windows")
	})

	t.Run("KeepLast < HealDeadlineEpochs rejected", func(t *testing.T) {
		p := DefaultParams()
		p.StorageTruthHealDeadlineEpochs = 50
		p.KeepLastEpochEntries = 21 // covers OldClassA(21) but not HealDeadline(50)
		require.ErrorContains(t, p.Validate(), "keep_last_epoch_entries must be >= max epoch lookback windows")
	})

	t.Run("defaults satisfy invariant", func(t *testing.T) {
		require.NoError(t, DefaultParams().Validate())
	})
}

// CP-R3 C-F5 — Postpone < StrongPostpone strict (equality collapses the band).
func TestParamsValidate_PostponeStrictlyLessThanStrongPostpone(t *testing.T) {
	base := DefaultParams()

	t.Run("equality rejected", func(t *testing.T) {
		p := base
		p.StorageTruthNodeSuspicionThresholdPostpone = 100
		p.StorageTruthNodeSuspicionThresholdStrongPostpone = 100
		require.ErrorContains(t, p.Validate(), "must be < storage_truth_node_suspicion_threshold_strong_postpone")
	})

	t.Run("strict ordering accepted", func(t *testing.T) {
		p := base
		p.StorageTruthNodeSuspicionThresholdPostpone = 90
		p.StorageTruthNodeSuspicionThresholdStrongPostpone = 140
		require.NoError(t, p.Validate())
	})
}
