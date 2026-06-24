package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	want := payload{Name: "alice", Count: 3}
	if err := AtomicWriteJSON(path, want); err != nil {
		t.Fatalf("AtomicWriteJSON: %v", err)
	}

	var got payload
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestAtomicWriteJSONReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	if err := AtomicWriteJSON(path, map[string]int{"a": 1}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := AtomicWriteJSON(path, map[string]int{"b": 2}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	var got map[string]int
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if _, ok := got["a"]; ok {
		t.Errorf("expected second write to replace first, got %v", got)
	}
	if got["b"] != 2 {
		t.Errorf("got %v, want b=2", got)
	}

	// No temporary files should remain in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "out.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory entries = %v, want only out.json", names)
	}
}

func TestReadJSONMissingFile(t *testing.T) {
	var v map[string]int
	if err := ReadJSON(filepath.Join(t.TempDir(), "nope.json"), &v); err == nil {
		t.Error("expected error reading missing file, got nil")
	}
}
