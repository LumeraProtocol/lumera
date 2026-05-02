package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// TestParamsWithDefaults_FillsZeroSVCFields ensures that an older params blob
// (read back as zero-valued SVC fields) is normalised by WithDefaults to the
// LEP-5 defaults. This is the soft-compat path used by keeper.GetParams and
// SetParams to avoid a ConsensusVersion bump for purely additive params.
func TestParamsWithDefaults_FillsZeroSVCFields(t *testing.T) {
	in := types.Params{} // zero values everywhere
	out := in.WithDefaults()
	require.Equal(t, types.DefaultSVCChallengeCount, out.SvcChallengeCount,
		"WithDefaults must populate SvcChallengeCount when zero")
	require.Equal(t, types.DefaultSVCMinChunksForChallenge, out.SvcMinChunksForChallenge,
		"WithDefaults must populate SvcMinChunksForChallenge when zero")
}

// TestParamsWithDefaults_PreservesNonZero ensures explicit non-zero values are
// not overwritten by WithDefaults.
func TestParamsWithDefaults_PreservesNonZero(t *testing.T) {
	in := types.Params{
		SvcChallengeCount:        16,
		SvcMinChunksForChallenge: 8,
	}
	out := in.WithDefaults()
	require.Equal(t, uint32(16), out.SvcChallengeCount)
	require.Equal(t, uint32(8), out.SvcMinChunksForChallenge)
}
