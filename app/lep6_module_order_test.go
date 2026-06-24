package app

import (
	"testing"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	supernodetypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/stretchr/testify/require"
)

func TestLEP6ModuleOrderingPinsSupernodeAuditAction(t *testing.T) {
	assertOrdered := func(t *testing.T, name string, modules []string) {
		t.Helper()
		supernodeIdx := indexOfModule(modules, supernodetypes.ModuleName)
		auditIdx := indexOfModule(modules, audittypes.ModuleName)
		actionIdx := indexOfModule(modules, actiontypes.ModuleName)

		require.NotEqual(t, -1, supernodeIdx, "%s missing supernode module", name)
		require.NotEqual(t, -1, auditIdx, "%s missing audit module", name)
		require.NotEqual(t, -1, actionIdx, "%s missing action module", name)
		require.Less(t, supernodeIdx, auditIdx, "%s must run supernode before audit for LEP-6 dependency ordering", name)
		require.Less(t, auditIdx, actionIdx, "%s must run audit before action so action finalization can anchor LEP-6 artifact counts", name)
	}

	assertOrdered(t, "genesisModuleOrder", genesisModuleOrder)
	assertOrdered(t, "beginBlockers", beginBlockers)
	assertOrdered(t, "endBlockers", endBlockers)
}
