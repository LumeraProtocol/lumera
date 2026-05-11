// openapigen merges multiple Swagger 2.0 JSON files into a single OpenAPI spec.
// It reads a TOML config file that defines source directories and their search order,
// producing a unified document with paths ordered by source priority.
//
// Usage:
//
//	go run ./tools/openapigen [-config tools/openapigen/config.toml] [-out docs/static/openapi.yml]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

type config struct {
	Info    infoConfig     `toml:"info"`
	Sources []sourceConfig `toml:"sources"`
}

type infoConfig struct {
	Title       string `toml:"title"`
	Description string `toml:"description"`
}

type sourceConfig struct {
	Dir     string `toml:"dir"`
	Pattern string `toml:"pattern"`
}

func main() {
	cfgPath := flag.String("config", "tools/openapigen/config.toml", "path to config file")
	outPath := flag.String("out", "docs/static/openapi.yml", "output file path")
	flag.Parse()

	cfgData, err := os.ReadFile(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read config %s: %v\n", *cfgPath, err)
		os.Exit(1)
	}

	var cfg config
	if err := toml.Unmarshal(cfgData, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "parse config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Sources) == 0 {
		fmt.Fprintln(os.Stderr, "no [[sources]] defined in config")
		os.Exit(1)
	}

	// Collect files in config-defined order.
	var files []string
	for _, src := range cfg.Sources {
		pattern := filepath.Join(src.Dir, "**", src.Pattern)
		matches, err := doubleStarGlob(src.Dir, src.Pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "glob %s: %v\n", pattern, err)
			os.Exit(1)
		}
		sort.Strings(matches) // deterministic within each source
		files = append(files, matches...)
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no swagger files found in any source directory")
		os.Exit(1)
	}

	// Merged spec skeleton — paths use ordered insertion via orderedMap.
	allPaths := newOrderedMap()
	allDefs := make(map[string]any)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", f, err)
			os.Exit(1)
		}

		var spec map[string]any
		if err := json.Unmarshal(data, &spec); err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", f, err)
			os.Exit(1)
		}

		if paths, ok := spec["paths"].(map[string]any); ok {
			// Sort path keys within each file for consistency.
			keys := make([]string, 0, len(paths))
			for k := range paths {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if !allPaths.has(k) {
					allPaths.set(k, paths[k])
				}
			}
		}
		if defs, ok := spec["definitions"].(map[string]any); ok {
			for k, v := range defs {
				allDefs[k] = v
			}
		}
	}

	// Build output with deterministic key order.
	// Using json.Marshal on a struct to control top-level key order.
	output := buildOutput(cfg.Info, allPaths, allDefs)

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outPath, output, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("merged %d files → %s (%d paths, %d definitions)\n",
		len(files), *outPath, allPaths.len(), len(allDefs))
}

// doubleStarGlob recursively walks dir and matches files against pattern.
func doubleStarGlob(dir, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		matched, err := filepath.Match(pattern, info.Name())
		if err != nil {
			return err
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

// orderedMap preserves insertion order for JSON output.
type orderedMap struct {
	keys   []string
	values map[string]any
}

func newOrderedMap() *orderedMap {
	return &orderedMap{values: make(map[string]any)}
}

func (m *orderedMap) set(key string, value any) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *orderedMap) has(key string) bool {
	_, ok := m.values[key]
	return ok
}

func (m *orderedMap) len() int {
	return len(m.keys)
}

func (m *orderedMap) MarshalJSON() ([]byte, error) {
	buf := []byte("{")
	for i, k := range m.keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		key, _ := json.Marshal(k)
		val, err := json.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		buf = append(buf, key...)
		buf = append(buf, ':')
		buf = append(buf, val...)
	}
	buf = append(buf, '}')
	return buf, nil
}

func buildOutput(info infoConfig, paths *orderedMap, defs map[string]any) []byte {
	// Sort definitions alphabetically.
	sortedDefs := newOrderedMap()
	defKeys := make([]string, 0, len(defs))
	for k := range defs {
		defKeys = append(defKeys, k)
	}
	sort.Strings(defKeys)
	for _, k := range defKeys {
		sortedDefs.set(k, defs[k])
	}

	type outputSpec struct {
		ID          string      `json:"id"`
		Consumes    []string    `json:"consumes"`
		Produces    []string    `json:"produces"`
		Swagger     string      `json:"swagger"`
		Info        any         `json:"info"`
		Paths       *orderedMap `json:"paths"`
		Definitions *orderedMap `json:"definitions"`
	}

	spec := outputSpec{
		ID:       "github.com/LumeraProtocol/lumera",
		Consumes: []string{"application/json"},
		Produces: []string{"application/json"},
		Swagger:  "2.0",
		Info: map[string]any{
			"title":       info.Title,
			"description": info.Description,
			"version":     "version not set",
			"contact": map[string]any{
				"name": "github.com/LumeraProtocol/lumera",
			},
		},
		Paths:       paths,
		Definitions: sortedDefs,
	}

	out, err := json.Marshal(spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	return out
}
