package keeper

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestStorageTruthIsClassAFault_IndexTimeoutIsClassB(t *testing.T) {
	record := storageTruthNodeFailureRecord{
		ResultClass:   int32(types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE),
		ArtifactClass: int32(types.StorageProofArtifactClass_STORAGE_PROOF_ARTIFACT_CLASS_INDEX),
	}

	require.False(t, storageTruthIsClassAFault(record))
	require.True(t, storageTruthIsClassBFault(record))
}
