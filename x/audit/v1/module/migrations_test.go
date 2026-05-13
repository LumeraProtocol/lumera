package audit

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestAuditMigrationStaysV1ToV2UntilV2Ships(t *testing.T) {
	// Per R3 C-F9: there is no separate v2→v3 handler because audit consensus
	// version v2 has not shipped to mainnet. The C-F1/C-F3 KeepLastEpochEntries
	// backfill is intentionally folded into NewMigrateV1ToV2 before v2 release.
	require.Equal(t, 2, types.ConsensusVersion)
}
