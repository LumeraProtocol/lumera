package keeper

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestStorageTruthAssignmentUsesOneThirdCoverage(t *testing.T) {
	params := types.DefaultParams().WithDefaults()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	params.StorageTruthChallengeTargetDivisor = 3

	active := []string{"sn-a", "sn-b", "sn-c", "sn-d", "sn-e", "sn-f"}
	seed := []byte("01234567890123456789012345678901")

	assignedTargets := make(map[string]struct{})
	proberCount := 0
	for _, reporter := range active {
		targets, isProber, err := computeAuditPeerTargetsForReporter(&params, active, active, seed, reporter)
		require.NoError(t, err)
		require.True(t, isProber)
		require.LessOrEqual(t, len(targets), 1)
		if len(targets) == 0 {
			continue
		}
		proberCount++
		require.NotEqual(t, reporter, targets[0])
		assignedTargets[targets[0]] = struct{}{}
	}

	require.Equal(t, 2, proberCount)
	require.Len(t, assignedTargets, 2)
}

func TestStorageTruthAssignmentIncludesPostponedTargets(t *testing.T) {
	params := types.DefaultParams().WithDefaults()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_FULL
	params.StorageTruthChallengeTargetDivisor = 1

	active := []string{"sn-a", "sn-b"}
	targets := []string{"sn-postponed"}
	seed := []byte("01234567890123456789012345678901")

	assigned := false
	for _, reporter := range active {
		reporterTargets, isProber, err := computeAuditPeerTargetsForReporter(&params, active, targets, seed, reporter)
		require.NoError(t, err)
		require.True(t, isProber)
		if containsString(reporterTargets, "sn-postponed") {
			assigned = true
		}
	}

	require.True(t, assigned, "active challengers must be able to target postponed supernodes for storage-truth recovery")
}

func TestStorageTruthAssignmentDisabledUsesLegacyCoverage(t *testing.T) {
	params := types.DefaultParams().WithDefaults()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED
	params.MinProbeTargetsPerEpoch = 1
	params.MaxProbeTargetsPerEpoch = 1
	params.PeerQuorumReports = 1

	active := []string{"sn-a", "sn-b", "sn-c"}
	seed := []byte("01234567890123456789012345678901")

	targets, isProber, err := computeAuditPeerTargetsForReporter(&params, active, active, seed, "sn-a")
	require.NoError(t, err)
	require.True(t, isProber)
	require.Len(t, targets, 1)
}
