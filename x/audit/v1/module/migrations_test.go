package audit

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestAuditIdentityContinuityBumpsV2ToV3(t *testing.T) {
	require.Equal(t, 3, types.ConsensusVersion)
}
