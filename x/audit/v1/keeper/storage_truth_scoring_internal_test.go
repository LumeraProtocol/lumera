package keeper

import (
	"math"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestStorageTruthScoreDeltasForResultClass(t *testing.T) {
	tests := []struct {
		name   string
		class  types.StorageProofResultClass
		expect storageTruthScoreDeltas
	}{
		{
			name:  "pass",
			class: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
			expect: storageTruthScoreDeltas{
				nodeSuspicion:       -2,
				reporterReliability: 2,
				ticketDeterioration: -3,
			},
		},
		{
			name:  "hash mismatch",
			class: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
			expect: storageTruthScoreDeltas{
				nodeSuspicion:       12,
				reporterReliability: 1,
				ticketDeterioration: 12,
			},
		},
		{
			name:  "timeout",
			class: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
			expect: storageTruthScoreDeltas{
				nodeSuspicion:       4,
				reporterReliability: -1,
				ticketDeterioration: 4,
			},
		},
		{
			name:  "observer quorum fail",
			class: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL,
			expect: storageTruthScoreDeltas{
				nodeSuspicion:       3,
				reporterReliability: -3,
				ticketDeterioration: 5,
			},
		},
		{
			name:  "no eligible ticket",
			class: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_NO_ELIGIBLE_TICKET,
			expect: storageTruthScoreDeltas{
				nodeSuspicion:       0,
				reporterReliability: 1,
				ticketDeterioration: 0,
			},
		},
		{
			name:  "invalid transcript",
			class: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT,
			expect: storageTruthScoreDeltas{
				nodeSuspicion:       0,
				reporterReliability: -8,
				ticketDeterioration: 0,
			},
		},
		{
			name:  "recheck confirmed fail",
			class: types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_RECHECK_CONFIRMED_FAIL,
			expect: storageTruthScoreDeltas{
				nodeSuspicion:       20,
				reporterReliability: 3,
				ticketDeterioration: 20,
			},
		},
		{
			name:   "unknown",
			class:  types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_UNSPECIFIED,
			expect: storageTruthScoreDeltas{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expect, storageTruthScoreDeltasForResultClass(tc.class))
		})
	}
}

func TestDecayTowardZero(t *testing.T) {
	tests := []struct {
		name    string
		score   int64
		decay   int64
		elapsed uint64
		expect  int64
	}{
		{name: "positive score", score: 10, decay: 2, elapsed: 3, expect: 4},
		{name: "positive clamps to zero", score: 5, decay: 3, elapsed: 3, expect: 0},
		{name: "negative score", score: -9, decay: 2, elapsed: 3, expect: -3},
		{name: "negative clamps to zero", score: -3, decay: 2, elapsed: 2, expect: 0},
		{name: "zero decay", score: 7, decay: 0, elapsed: 10, expect: 7},
		{name: "negative decay treated as disabled", score: 7, decay: -1, elapsed: 10, expect: 7},
		{name: "zero elapsed", score: 7, decay: 1, elapsed: 0, expect: 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expect, decayTowardZero(tc.score, tc.decay, tc.elapsed))
		})
	}
}

func TestAddInt64Saturated(t *testing.T) {
	require.Equal(t, int64(math.MaxInt64), addInt64Saturated(math.MaxInt64-1, 10))
	require.Equal(t, int64(math.MinInt64), addInt64Saturated(math.MinInt64+1, -10))
	require.Equal(t, int64(8), addInt64Saturated(3, 5))
}

func TestReporterTrustBandForScore(t *testing.T) {
	params := types.DefaultParams().WithDefaults()
	params.StorageTruthReporterReliabilityLowTrustThreshold = -20
	params.StorageTruthReporterReliabilityIneligibleThreshold = -50

	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_NORMAL, reporterTrustBandForScore(0, params))
	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_LOW_TRUST, reporterTrustBandForScore(-20, params))
	require.Equal(t, types.ReporterTrustBand_REPORTER_TRUST_BAND_CHALLENGER_INELIGIBLE, reporterTrustBandForScore(-50, params))
}

func TestStorageTruthResultsContradict(t *testing.T) {
	require.True(t, storageTruthResultsContradict(
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
	))
	require.True(t, storageTruthResultsContradict(
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
	))
	require.False(t, storageTruthResultsContradict(
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
	))
}

func TestScaleInt64TowardZero(t *testing.T) {
	require.Equal(t, int64(6), scaleInt64TowardZero(12, 50, 100))
	require.Equal(t, int64(-1), scaleInt64TowardZero(-3, 50, 100))
	require.Equal(t, int64(0), scaleInt64TowardZero(-3, 25, 100))
	require.Equal(t, int64(12), scaleInt64TowardZero(12, 100, 100))
}
