package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// typeFieldOverride curates the description (and optionally schema/required-ness)
// of a single field inside a Go struct, keyed by the parent type's Go name.
type typeFieldOverride struct {
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Required    *bool          `json:"required,omitempty"`
}

// typeOverrideFile maps Go type name (as returned by reflect.Type.String,
// e.g. "types.TraceConfig") to a map of field-name -> override.
//
// Keying by type rather than method.param.path means a description is written
// once and reused everywhere the struct appears in any RPC method.
type typeOverrideFile map[string]map[string]typeFieldOverride

func loadTypeOverrides(path string) (typeOverrideFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return typeOverrideFile{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return typeOverrideFile{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return typeOverrideFile{}, nil
	}

	var out typeOverrideFile
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if out == nil {
		out = typeOverrideFile{}
	}
	return out, nil
}

func (t typeOverrideFile) lookup(typeName, fieldName string) *typeFieldOverride {
	if t == nil {
		return nil
	}
	if m, ok := t[typeName]; ok {
		if entry, ok := m[fieldName]; ok {
			return &entry
		}
	}
	return nil
}

// activeTypeOverrides is set by main() at startup; structProperties reads it.
// A package-level handle keeps the schema-recursion signatures simple and
// matches the existing paramNameOverrides convention in main.go.
var activeTypeOverrides typeOverrideFile
