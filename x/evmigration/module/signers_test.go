package evmigration_test

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/stretchr/testify/require"

	evmigration "github.com/LumeraProtocol/lumera/x/evmigration/module"
)

// TestProvideCustomGetSigners verifies evmigration messages are explicitly
// registered as unsigned at the Cosmos tx layer.
func TestProvideCustomGetSigners(t *testing.T) {
	t.Parallel()

	custom := evmigration.ProvideCustomGetSigners()
	require.Len(t, custom, 2)

	require.Equal(t, protoreflect.FullName("lumera.evmigration.MsgClaimLegacyAccount"), custom[0].MsgType)
	require.Equal(t, protoreflect.FullName("lumera.evmigration.MsgMigrateValidator"), custom[1].MsgType)

	signers, err := custom[0].Fn(nil)
	require.NoError(t, err)
	require.Nil(t, signers)
}
