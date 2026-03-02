package v1_11_0

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/stretchr/testify/require"

	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func TestPrepareVersionMapForCustomAuditInit_NilInput(t *testing.T) {
	migrationVM := prepareVersionMapForCustomAuditInit(nil)

	require.Equal(t, module.VersionMap{
		audittypes.ModuleName: audittypes.ConsensusVersion,
	}, migrationVM)
}

func TestPrepareVersionMapForCustomAuditInit_ClonesAndPinsAuditVersion(t *testing.T) {
	fromVM := module.VersionMap{
		"bank":  3,
		"auth":  2,
		"audit": 0,
	}

	migrationVM := prepareVersionMapForCustomAuditInit(fromVM)

	require.Equal(t, uint64(0), fromVM[audittypes.ModuleName], "input map must not be mutated")
	require.Equal(t, uint64(3), migrationVM["bank"])
	require.Equal(t, uint64(2), migrationVM["auth"])
	require.Equal(t, uint64(audittypes.ConsensusVersion), migrationVM[audittypes.ModuleName])
}
