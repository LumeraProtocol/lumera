package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultParamsIncludeStorageTruthDefaults(t *testing.T) {
	p := DefaultParams()

	require.Equal(t, DefaultStorageTruthRecentBucketMaxBlocks, p.StorageTruthRecentBucketMaxBlocks)
	require.Equal(t, DefaultStorageTruthOldBucketMinBlocks, p.StorageTruthOldBucketMinBlocks)
	require.Equal(t, DefaultStorageTruthChallengeTargetDivisor, p.StorageTruthChallengeTargetDivisor)
	require.Equal(t, DefaultStorageTruthCompoundRangesPerArtifact, p.StorageTruthCompoundRangesPerArtifact)
	require.Equal(t, DefaultStorageTruthCompoundRangeLenBytes, p.StorageTruthCompoundRangeLenBytes)
	require.Equal(t, DefaultStorageTruthEnforcementMode, p.StorageTruthEnforcementMode)
	require.NoError(t, p.Validate())
}

func TestParamsWithDefaultsSetsStorageTruthFields(t *testing.T) {
	p := Params{}
	p = p.WithDefaults()

	require.Equal(t, DefaultStorageTruthRecentBucketMaxBlocks, p.StorageTruthRecentBucketMaxBlocks)
	require.Equal(t, DefaultStorageTruthOldBucketMinBlocks, p.StorageTruthOldBucketMinBlocks)
	require.Equal(t, DefaultStorageTruthChallengeTargetDivisor, p.StorageTruthChallengeTargetDivisor)
	require.Equal(t, DefaultStorageTruthMaxSelfHealOpsPerEpoch, p.StorageTruthMaxSelfHealOpsPerEpoch)
	require.Equal(t, DefaultStorageTruthEnforcementMode, p.StorageTruthEnforcementMode)
}

func TestParamsValidateStorageTruthFailures(t *testing.T) {
	base := DefaultParams()

	p1 := base
	p1.StorageTruthRecentBucketMaxBlocks = p1.StorageTruthOldBucketMinBlocks
	require.ErrorContains(t, p1.Validate(), "storage_truth_recent_bucket_max_blocks must be < storage_truth_old_bucket_min_blocks")

	p2 := base
	p2.StorageTruthNodeSuspicionThresholdWatch = 100
	p2.StorageTruthNodeSuspicionThresholdProbation = 90
	require.ErrorContains(t, p2.Validate(), "storage_truth_node_suspicion_threshold_watch must be <=")

	p3 := base
	p3.StorageTruthReporterReliabilityLowTrustThreshold = 90
	p3.StorageTruthReporterReliabilityIneligibleThreshold = 20
	require.ErrorContains(t, p3.Validate(), "storage_truth_reporter_reliability_low_trust_threshold must be <=")

	p3a := base
	p3a.StorageTruthReporterReliabilityLowTrustThreshold = -1
	require.ErrorContains(t, p3a.Validate(), "storage_truth_reporter_reliability_low_trust_threshold must be >= 0")

	p3b := base
	p3b.StorageTruthReporterReliabilityIneligibleThreshold = -1
	require.ErrorContains(t, p3b.Validate(), "storage_truth_reporter_reliability_ineligible_threshold must be >= 0")

	p3c := base
	p3c.StorageTruthTicketDeteriorationHealThreshold = 0
	require.ErrorContains(t, p3c.Validate(), "storage_truth_ticket_deterioration_heal_threshold must be > 0")

	p3d := base
	p3d.StorageTruthNodeSuspicionDecayPerEpoch = 0
	require.ErrorContains(t, p3d.Validate(), "storage_truth_node_suspicion_decay_per_epoch must be within 1..1000")

	p3e := base
	p3e.StorageTruthReporterReliabilityDecayPerEpoch = 1001
	require.ErrorContains(t, p3e.Validate(), "storage_truth_reporter_reliability_decay_per_epoch must be within 1..1000")

	p3f := base
	p3f.StorageTruthTicketDeteriorationDecayPerEpoch = -5
	require.ErrorContains(t, p3f.Validate(), "storage_truth_ticket_deterioration_decay_per_epoch must be within 1..1000")

	p4 := base
	p4.StorageTruthEnforcementMode = StorageTruthEnforcementMode(99)
	require.ErrorContains(t, p4.Validate(), "storage_truth_enforcement_mode is invalid")
}
