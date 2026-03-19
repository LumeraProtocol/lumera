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

	methods, ok := payload["methods"].([]any)
	require.True(t, ok)

	var foundDiscover bool
	var foundEthCall bool
	var foundGetLogs bool
	for _, rawMethod := range methods {
		method, ok := rawMethod.(map[string]any)
		require.True(t, ok)
		if method["name"] != "rpc.discover" {
			if method["name"] != "eth_call" && method["name"] != "eth_getLogs" {
				continue
			}
			params, ok := method["params"].([]any)
			require.True(t, ok)
			require.NotEmpty(t, params)

			firstParam, ok := params[0].(map[string]any)
			require.True(t, ok)
			schema, ok := firstParam["schema"].(map[string]any)
			require.True(t, ok)

			if method["name"] == "eth_call" {
				foundEthCall = true
				_, hasRequired := schema["required"]
				require.False(t, hasRequired, "TransactionArgs schema should not mark variant-only fields as globally required")

				properties, ok := schema["properties"].(map[string]any)
				require.True(t, ok)

				dataField, ok := properties["data"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, true, dataField["deprecated"])

				inputField, ok := properties["input"].(map[string]any)
				require.True(t, ok)
				require.Contains(t, inputField["description"], "Preferred")

				overridesParam, ok := params[2].(map[string]any)
				require.True(t, ok)
				overridesSchema, ok := overridesParam["schema"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "json.RawMessage", overridesSchema["x-go-type"])
				_, hasAccountOverrides := overridesSchema["additionalProperties"]
				require.True(t, hasAccountOverrides)
				continue
			}

			foundGetLogs = true
			require.Equal(t, "filters.FilterCriteria", schema["x-go-type"])
			properties, ok := schema["properties"].(map[string]any)
			require.True(t, ok)
			_, hasTopics := properties["topics"]
			require.True(t, hasTopics)
			continue
		}
		foundDiscover = true

		result, ok := method["result"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "OpenRPC Schema", result["name"])

		schema, ok := result["schema"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "https://raw.githubusercontent.com/open-rpc/meta-schema/master/schema.json", schema["$ref"])
	}

	require.True(t, foundDiscover, "embedded OpenRPC doc must advertise canonical rpc.discover method")
	require.True(t, foundEthCall, "embedded OpenRPC doc must include the curated eth_call TransactionArgs schema")
	require.True(t, foundGetLogs, "embedded OpenRPC doc must include the curated eth_getLogs filter schema")
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
