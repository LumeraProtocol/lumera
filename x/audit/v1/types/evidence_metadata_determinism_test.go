package types

import (
	"testing"

	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
)

func TestCascadeClientFailureEvidenceMetadataMarshalDeterministicForMap(t *testing.T) {
	m := &CascadeClientFailureEvidenceMetadata{
		ReporterComponent: CascadeClientFailureReporterComponent_CASCADE_CLIENT_FAILURE_REPORTER_COMPONENT_SN_API_SERVER,
		TargetSupernodeAccounts: []string{"lumera1abc", "lumera1def"},
		Details: map[string]string{
			"action_id":  "123637",
			"task_id":    "9700ec8a",
			"operation":  "download",
			"error":      "insufficient symbols",
			"iteration":  "1",
		},
	}

	first, err := gogoproto.Marshal(m)
	require.NoError(t, err)

	for i := 0; i < 40; i++ {
		got, err := gogoproto.Marshal(m)
		require.NoError(t, err)
		require.Equal(t, first, got)
	}
}
