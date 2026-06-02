package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// paramOverrideEntry is the JSON shape accepted from the external override
// file. It is intentionally narrower than descriptorOverride so the file stays
// hand-editable; required is a *bool so authors can opt-in to flipping it.
type paramOverrideEntry struct {
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Required    *bool          `json:"required,omitempty"`
}

// paramOverrideFile maps methodName -> paramName -> override.
type paramOverrideFile map[string]map[string]paramOverrideEntry

func loadParamOverrides(path string) (paramOverrideFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return paramOverrideFile{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return paramOverrideFile{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return paramOverrideFile{}, nil
	}

	var out paramOverrideFile
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if out == nil {
		out = paramOverrideFile{}
	}
	return out, nil
}

func (p paramOverrideFile) lookup(methodName, paramName string) *paramOverrideEntry {
	if p == nil {
		return nil
	}
	if m, ok := p[methodName]; ok {
		if entry, ok := m[paramName]; ok {
			return &entry
		}
	}
	return nil
}

// resultOverrideFile maps methodName -> result override. Same entry shape as
// paramOverrideEntry; results have no name dimension, so the inner map is
// flattened away.
type resultOverrideFile map[string]paramOverrideEntry

func loadResultOverrides(path string) (resultOverrideFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return resultOverrideFile{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return resultOverrideFile{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return resultOverrideFile{}, nil
	}

	var out resultOverrideFile
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if out == nil {
		out = resultOverrideFile{}
	}
	return out, nil
}

func (r resultOverrideFile) lookup(methodName string) *paramOverrideEntry {
	if r == nil {
		return nil
	}
	if entry, ok := r[methodName]; ok {
		return &entry
	}
	return nil
}
