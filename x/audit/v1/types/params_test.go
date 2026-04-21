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
	// UNSPECIFIED is a valid no-op mode; WithDefaults does not promote it to SHADOW.
	require.Equal(t, StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED, p.StorageTruthEnforcementMode)
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
	p3.StorageTruthReporterReliabilityLowTrustThreshold = -100
	p3.StorageTruthReporterReliabilityIneligibleThreshold = -10
	require.ErrorContains(t, p3.Validate(), "storage_truth_reporter_reliability_low_trust_threshold must be >=")

	p4 := base
	p4.StorageTruthEnforcementMode = StorageTruthEnforcementMode(99)
	require.ErrorContains(t, p4.Validate(), "storage_truth_enforcement_mode is invalid")
}

func TestParamsWithDefaults_DerivesBucketThresholdsFromEpochLength(t *testing.T) {
	p := Params{
		EpochLengthBlocks: 100,
	}
	p = p.WithDefaults()

	require.Equal(t, uint64(300), p.StorageTruthRecentBucketMaxBlocks)
	require.Equal(t, uint64(3000), p.StorageTruthOldBucketMinBlocks)
	require.NoError(t, p.Validate())
}
