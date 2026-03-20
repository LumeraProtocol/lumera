package docs

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// swaggerSpec represents the subset of Swagger 2.0 we validate.
type swaggerSpec struct {
	Swagger     string                            `json:"swagger"`
	Info        map[string]any                    `json:"info"`
	Paths       map[string]map[string]any         `json:"paths"`
	Definitions map[string]map[string]any         `json:"definitions"`
	Consumes    []string                          `json:"consumes"`
	Produces    []string                          `json:"produces"`
}

func loadEmbeddedSpec(t *testing.T) swaggerSpec {
	t.Helper()
	data, err := fs.ReadFile(Static, "static/openapi.yml")
	require.NoError(t, err, "embedded openapi.yml must be readable")
	require.NotEmpty(t, data, "embedded openapi.yml must not be empty")

	var spec swaggerSpec
	require.NoError(t, json.Unmarshal(data, &spec), "openapi.yml must be valid JSON")
	return spec
}

func TestEmbeddedSpecIsValidSwagger(t *testing.T) {
	spec := loadEmbeddedSpec(t)

	assert.Equal(t, "2.0", spec.Swagger, "must be Swagger 2.0")
	assert.NotEmpty(t, spec.Info, "info must be present")
	assert.Contains(t, spec.Consumes, "application/json")
	assert.Contains(t, spec.Produces, "application/json")
	assert.NotEmpty(t, spec.Paths, "paths must be present")
	assert.NotEmpty(t, spec.Definitions, "definitions must be present")
}

func TestEmbeddedSpecContainsLumeraModules(t *testing.T) {
	spec := loadEmbeddedSpec(t)

	// Every Lumera custom module should have at least one path.
	requiredModulePrefixes := []struct {
		module string
		prefix string
	}{
		{"action", "/LumeraProtocol/lumera/action/"},
		{"claim", "/LumeraProtocol/lumera/claim/"},
		{"supernode", "/LumeraProtocol/lumera/supernode/"},
		{"lumeraid", "/LumeraProtocol/lumera/lumeraid/"},
	}

	for _, mod := range requiredModulePrefixes {
		t.Run(mod.module, func(t *testing.T) {
			found := false
			for path := range spec.Paths {
				if strings.HasPrefix(path, mod.prefix) {
					found = true
					break
				}
			}
			assert.True(t, found, "module %s should have paths with prefix %s", mod.module, mod.prefix)
		})
	}
}

func TestEmbeddedSpecContainsEVMModules(t *testing.T) {
	spec := loadEmbeddedSpec(t)

	evmModulePrefixes := []struct {
		module string
		prefix string
	}{
		{"erc20", "/cosmos.evm.erc20."},
		{"feemarket", "/cosmos.evm.feemarket."},
		{"vm", "/cosmos.evm.vm."},
	}

	for _, mod := range evmModulePrefixes {
		t.Run(mod.module, func(t *testing.T) {
			found := false
			for path := range spec.Paths {
				if strings.HasPrefix(path, mod.prefix) {
					found = true
					break
				}
			}
			assert.True(t, found, "EVM module %s should have paths with prefix %s", mod.module, mod.prefix)
		})
	}
}

func TestEmbeddedSpecPathsHaveResponses(t *testing.T) {
	spec := loadEmbeddedSpec(t)

	for path, methods := range spec.Paths {
		for method, opRaw := range methods {
			op, ok := opRaw.(map[string]any)
			if !ok {
				continue
			}
			responses, hasResp := op["responses"]
			assert.True(t, hasResp, "%s %s must have responses", method, path)
			if respMap, ok := responses.(map[string]any); ok {
				assert.NotEmpty(t, respMap, "%s %s responses must not be empty", method, path)
			}
		}
	}
}

func TestEmbeddedSpecDefinitionRefsResolve(t *testing.T) {
	spec := loadEmbeddedSpec(t)
	raw, _ := fs.ReadFile(Static, "static/openapi.yml")

	// Collect all $ref values from the entire spec.
	var refs []string
	collectRefs(t, raw, &refs)

	// Every #/definitions/X ref should exist in the definitions map.
	const prefix = "#/definitions/"
	var unresolved []string
	for _, ref := range refs {
		if strings.HasPrefix(ref, prefix) {
			defName := ref[len(prefix):]
			if _, ok := spec.Definitions[defName]; !ok {
				unresolved = append(unresolved, defName)
			}
		}
	}

	assert.Empty(t, unresolved, "all $ref targets must resolve; unresolved: %v", unresolved)
}

func TestEmbeddedSpecMinimumCoverage(t *testing.T) {
	spec := loadEmbeddedSpec(t)

	// Sanity check: the spec should have a reasonable number of paths and definitions.
	assert.GreaterOrEqual(t, len(spec.Paths), 50,
		"spec should have at least 50 paths (got %d)", len(spec.Paths))
	assert.GreaterOrEqual(t, len(spec.Definitions), 80,
		"spec should have at least 80 definitions (got %d)", len(spec.Definitions))
}

// collectRefs extracts all "$ref" string values from raw JSON.
func collectRefs(t *testing.T, data []byte, refs *[]string) {
	t.Helper()
	var raw any
	require.NoError(t, json.Unmarshal(data, &raw))
	walkJSON(raw, refs)
}

func walkJSON(v any, refs *[]string) {
	switch val := v.(type) {
	case map[string]any:
		if ref, ok := val["$ref"].(string); ok {
			*refs = append(*refs, ref)
		}
		for _, child := range val {
			walkJSON(child, refs)
		}
	case []any:
		for _, child := range val {
			walkJSON(child, refs)
		}
	}
}
