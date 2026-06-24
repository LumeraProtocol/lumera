package main

import (
	"os"
	"path/filepath"
	"testing"
)

// When -config is not passed, the default config is resolved next to the
// executable first, so `tests-gen-activity` finds its config regardless of the
// current working directory (e.g. /shared/release/tests-gen-activity run from
// /root). An explicit -config path is never rewritten.
func TestResolveConfigPathPrefersExeDir(t *testing.T) {
	const base = "gen-activity-config.toml"

	exeDir := t.TempDir()
	exeCfg := filepath.Join(exeDir, base)
	if err := os.WriteFile(exeCfg, []byte("[common]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("exe-dir config is preferred over the bare cwd default", func(t *testing.T) {
		if got := resolveConfigPath(base, false, exeDir); got != exeCfg {
			t.Errorf("got %q, want %q", got, exeCfg)
		}
	})

	t.Run("explicit -config is left untouched", func(t *testing.T) {
		if got := resolveConfigPath(base, true, exeDir); got != base {
			t.Errorf("got %q, want %q (explicit must not be rewritten)", got, base)
		}
	})

	t.Run("falls back to the original path when exe-dir has no config", func(t *testing.T) {
		if got := resolveConfigPath(base, false, t.TempDir()); got != base {
			t.Errorf("got %q, want %q", got, base)
		}
	})

	t.Run("empty exe-dir leaves the original path", func(t *testing.T) {
		if got := resolveConfigPath(base, false, ""); got != base {
			t.Errorf("got %q, want %q", got, base)
		}
	})
}
