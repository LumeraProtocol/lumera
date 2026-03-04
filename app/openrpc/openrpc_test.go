package openrpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDiscoverDocumentValid ensures the embedded spec is parseable and shaped
// like an OpenRPC document.
func TestDiscoverDocumentValid(t *testing.T) {
	t.Parallel()

	doc, err := DiscoverDocument()
	require.NoError(t, err)
	require.True(t, json.Valid(doc))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(doc, &payload))
	require.Equal(t, "1.2.6", payload["openrpc"])
}

// TestEnsureNamespaceEnabled verifies the helper appends `rpc` once and is idempotent.
func TestEnsureNamespaceEnabled(t *testing.T) {
	t.Parallel()

	withRPC := EnsureNamespaceEnabled([]string{"eth", "net", "web3"})
	require.Equal(t, []string{"eth", "net", "web3", Namespace}, withRPC)

	again := EnsureNamespaceEnabled(withRPC)
	require.Equal(t, withRPC, again)
}

// TestRegisterJSONRPCNamespaceIdempotent verifies repeated calls are safe.
func TestRegisterJSONRPCNamespaceIdempotent(t *testing.T) {
	t.Parallel()

	require.NoError(t, RegisterJSONRPCNamespace())
	require.NoError(t, RegisterJSONRPCNamespace())
}
