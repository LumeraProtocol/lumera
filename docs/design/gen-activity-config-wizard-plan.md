# gen-activity Config File + Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `gen-activity-config.toml` file with named chains + a `[common]` section selectable via `-chain`, layered config precedence (flag > chain > common > default), and an interactive survey wizard that is the default when the tool is run with no flags.

**Architecture:** A new `configfile.go` parses TOML into a pointer-field `FileConfig` (nil = unset), then layers it onto the existing flag-parsed `Config` using `flag.Visit` to skip any field the user set explicitly on the command line. `main` picks wizard vs command-line mode from `flag.NFlag()` (plus a `-w` override). A new `wizard.go` drives an arrow-key menu through a small `prompter` interface (survey in production, a scripted fake in tests) and ends by calling the same `run(cfg)` path as command-line mode.

**Tech Stack:** Go, `github.com/pelletier/go-toml/v2` (promote to direct dep), `github.com/AlecAivazis/survey/v2` (new dep), Go stdlib `flag`. Module: `gen` at `devnet/go.mod`. Package: `main` at `devnet/tests/gen-activity`.

**Scope note:** The `-num-multisig23-accounts` / `-num-multisig35-accounts` flags are *defined and validated* here (plain ints, default 0, surfaced in the wizard) but their generation **behavior** is implemented in the companion plan `2026-06-12-gen-activity-multisig-accounts.md`. Until then a non-zero value is accepted and ignored by `run()`.

---

## File Structure

New:
- `devnet/tests/gen-activity/configfile.go` — `FileConfig`/`ChainSection` types, `LoadFileConfig`, `ChainNames`, `applyLayer`, `ApplyFileConfig`.
- `devnet/tests/gen-activity/configfile_test.go` — TOML parse + precedence tests.
- `devnet/tests/gen-activity/wizard.go` — `prompter` interface, `surveyPrompter`, `menuItems`, `editSetting`, `runWizard`.
- `devnet/tests/gen-activity/wizard_test.go` — menu state-machine tests with a scripted fake prompter.
- `gen-activity-config.toml.example` — documented sample config (repo root).

Modified:
- `devnet/tests/gen-activity/config.go` — new `Config` fields (`ConfigPath`, `Chain`, `Wizard`, `NumMultisig23`, `NumMultisig35`) + multisig-count validation.
- `devnet/tests/gen-activity/main.go` — register new flags, mode selection, config layering, wizard dispatch.
- `devnet/go.mod` / `devnet/go.sum` — deps.
- `docs/design/gen-activity-design.md` — document config, wizard, mode rule.

---

## Task 1: Add dependencies (go-toml direct + survey/v2)

**Files:**
- Modify: `devnet/go.mod`

- [ ] **Step 1: Add the survey dependency and tidy**

Run (from the module dir):

```bash
cd devnet && go get github.com/AlecAivazis/survey/v2@v2.3.7 && go mod tidy
```

Expected: `devnet/go.mod` now lists `github.com/AlecAivazis/survey/v2` as a direct dependency, and `github.com/pelletier/go-toml/v2` moves out of the `// indirect` block once it is imported (it is imported in Task 4; tidy again then). `go.sum` updated.

- [ ] **Step 2: Verify the module still builds**

Run:

```bash
cd devnet && go build ./...
```

Expected: builds with no errors.

- [ ] **Step 3: Commit**

```bash
git add devnet/go.mod devnet/go.sum
git commit -m "build(gen-activity): add survey/v2 dependency for wizard"
```

---

## Task 2: Add new Config fields + multisig-count validation

**Files:**
- Modify: `devnet/tests/gen-activity/config.go`
- Test: `devnet/tests/gen-activity/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/config_test.go`:

```go
func TestConfigValidateMultisigCounts(t *testing.T) {
	t.Run("zero multisig counts pass", func(t *testing.T) {
		c := validConfig()
		c.NumMultisig23 = 0
		c.NumMultisig35 = 0
		if err := c.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("negative num-multisig23 fails", func(t *testing.T) {
		c := validConfig()
		c.NumMultisig23 = -1
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative num-multisig23-accounts")
		}
	})

	t.Run("negative num-multisig35 fails", func(t *testing.T) {
		c := validConfig()
		c.NumMultisig35 = -2
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative num-multisig35-accounts")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestConfigValidateMultisigCounts -v
```

Expected: FAIL — `c.NumMultisig23 undefined` (compile error).

- [ ] **Step 3: Add the fields and validation**

In `devnet/tests/gen-activity/config.go`, add to the `Config` struct (after the `AccountPrefix` field, in the "Registry and account generation" block):

```go
	// Multisig account generation. Behavior implemented in the multisig plan;
	// validated here so the flag/wizard surface is complete.
	NumMultisig23 int // 2-of-3 multisig accounts
	NumMultisig35 int // 3-of-5 multisig accounts
```

Add to the "Connection / runtime" block (top of struct):

```go
	// Config file + chain selection + mode.
	ConfigPath string
	Chain      string
	Wizard     bool
```

In `Config.Validate()`, add after the `MaxActionsPerRun` check (before the `MaxAccountAmount` parse):

```go
	if c.NumMultisig23 < 0 {
		return fmt.Errorf("-num-multisig23-accounts must be >= 0, got %d", c.NumMultisig23)
	}
	if c.NumMultisig35 < 0 {
		return fmt.Errorf("-num-multisig35-accounts must be >= 0, got %d", c.NumMultisig35)
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestConfigValidateMultisigCounts -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/config.go devnet/tests/gen-activity/config_test.go
git commit -m "feat(gen-activity): add config fields for config-file, chain, wizard, multisig counts"
```

---

## Task 3: Register new flags

**Files:**
- Modify: `devnet/tests/gen-activity/main.go`
- Test: `devnet/tests/gen-activity/main_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/main_test.go`:

```go
func TestConfigureFlagsRegistersNewFlags(t *testing.T) {
	var cfg Config
	fs := flag.NewFlagSet("tests-gen-activity", flag.ContinueOnError)
	configureFlags(fs, &cfg)

	for _, name := range []string{
		"config", "chain", "wizard", "w",
		"num-multisig23-accounts", "num-multisig35-accounts",
	} {
		if fs.Lookup(name) == nil {
			t.Errorf("flag -%s not registered", name)
		}
	}

	if got := fs.Lookup("config").DefValue; got != "gen-activity-config.toml" {
		t.Errorf("-config default = %q, want gen-activity-config.toml", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestConfigureFlagsRegistersNewFlags -v
```

Expected: FAIL — flag `-config` not registered (`fs.Lookup("config")` is nil).

- [ ] **Step 3: Register the flags**

In `devnet/tests/gen-activity/main.go`, inside `configureFlags`, add after the existing `fs.StringVar(&c.Bin, ...)` line and before `fs.StringVar(&c.RPC, ...)`:

```go
	fs.StringVar(&c.ConfigPath, "config", "gen-activity-config.toml", "path to gen-activity TOML config file (optional)")
	fs.StringVar(&c.Chain, "chain", "", "named chain section from the config file (e.g. devnet, testnet, mainnet)")
	fs.BoolVar(&c.Wizard, "wizard", false, "run the interactive wizard (also the default when no flags are passed)")
	fs.BoolVar(&c.Wizard, "w", false, "shorthand for -wizard")
```

Add after the existing `fs.IntVar(&c.NumAccounts, "num-accounts", ...)` line:

```go
	fs.IntVar(&c.NumMultisig23, "num-multisig23-accounts", 0, "number of 2-of-3 multisig accounts to generate")
	fs.IntVar(&c.NumMultisig35, "num-multisig35-accounts", 0, "number of 3-of-5 multisig accounts to generate")
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestConfigureFlagsRegistersNewFlags -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/main.go devnet/tests/gen-activity/main_test.go
git commit -m "feat(gen-activity): register -config/-chain/-wizard and multisig-count flags"
```

---

## Task 4: TOML config file parsing (`configfile.go`)

**Files:**
- Create: `devnet/tests/gen-activity/configfile.go`
- Test: `devnet/tests/gen-activity/configfile_test.go`

- [ ] **Step 1: Write the failing test**

Create `devnet/tests/gen-activity/configfile_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const sampleTOML = `
[common]
bin = "lumerad"
funding-key = "faucet"
parallelism = 8

[chains.devnet]
rpc = "tcp://localhost:26657"
chain-id = "lumera-devnet-1"
accounts = "accounts-devnet.json"

[chains.testnet]
rpc = "https://rpc.testnet:443"
chain-id = "lumera-testnet-1"
`

func writeTempTOML(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gen-activity-config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp toml: %v", err)
	}
	return path
}

func TestLoadFileConfigMissingFileReturnsNil(t *testing.T) {
	fc, err := LoadFileConfig(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if fc != nil {
		t.Fatalf("expected nil FileConfig for missing file, got %+v", fc)
	}
}

func TestLoadFileConfigParsesSections(t *testing.T) {
	fc, err := LoadFileConfig(writeTempTOML(t, sampleTOML))
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}
	if fc == nil {
		t.Fatal("expected non-nil FileConfig")
	}
	if fc.Common.FundingKey == nil || *fc.Common.FundingKey != "faucet" {
		t.Errorf("common funding-key not parsed: %+v", fc.Common.FundingKey)
	}
	if fc.Common.Parallelism == nil || *fc.Common.Parallelism != 8 {
		t.Errorf("common parallelism not parsed: %+v", fc.Common.Parallelism)
	}
	dev, ok := fc.Chains["devnet"]
	if !ok {
		t.Fatal("devnet chain section missing")
	}
	if dev.ChainID == nil || *dev.ChainID != "lumera-devnet-1" {
		t.Errorf("devnet chain-id not parsed: %+v", dev.ChainID)
	}
	if want := []string{"devnet", "testnet"}; !reflect.DeepEqual(fc.ChainNames(), want) {
		t.Errorf("ChainNames() = %v, want %v (sorted)", fc.ChainNames(), want)
	}
}

func TestLoadFileConfigRejectsUnknownKey(t *testing.T) {
	_, err := LoadFileConfig(writeTempTOML(t, "[common]\nbogus-key = 1\n"))
	if err == nil {
		t.Error("expected error for unknown TOML key (strict decoding)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestLoadFileConfig -v
```

Expected: FAIL — `undefined: LoadFileConfig` (compile error).

- [ ] **Step 3: Write the implementation**

Create `devnet/tests/gen-activity/configfile.go`:

```go
package main

import (
	"fmt"
	"os"
	"sort"

	toml "github.com/pelletier/go-toml/v2"
)

// ChainSection holds the config values for one chain (or the shared [common]
// section). Every field is a pointer so an absent TOML key is distinguishable
// from an explicit zero value; nil means "not set in this section". Keys mirror
// the CLI flag names so config and flags map one-to-one.
type ChainSection struct {
	Bin              *string `toml:"bin"`
	RPC              *string `toml:"rpc"`
	GRPC             *string `toml:"grpc"`
	ChainID          *string `toml:"chain-id"`
	Home             *string `toml:"home"`
	KeyringBackend   *string `toml:"keyring-backend"`
	EVMCutoverVer    *string `toml:"evm-cutover-version"`
	FundingKey       *string `toml:"funding-key"`
	AccountsPath     *string `toml:"accounts"`
	NumAccounts      *int    `toml:"num-accounts"`
	MaxAccountAmount *string `toml:"max-account-amount"`
	AccountPrefix    *string `toml:"account-prefix"`
	Actions          *bool   `toml:"actions"`
	FundingBatchSize *int    `toml:"funding-batch-size"`
	Parallelism      *int    `toml:"parallelism"`
	NumMultisig23    *int    `toml:"num-multisig23-accounts"`
	NumMultisig35    *int    `toml:"num-multisig35-accounts"`
}

// FileConfig is the parsed gen-activity-config.toml: a shared [common] section
// plus any number of named [chains.<name>] sections.
type FileConfig struct {
	Common ChainSection            `toml:"common"`
	Chains map[string]ChainSection `toml:"chains"`
}

// LoadFileConfig reads and strictly decodes the TOML config at path. A missing
// file returns (nil, nil) so the caller can fall back to pure flag behavior; an
// unparseable file or an unknown key is a hard error.
func LoadFileConfig(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var fc FileConfig
	if err := strictUnmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &fc, nil
}

// strictUnmarshal decodes TOML with unknown keys rejected so typos in the
// config file surface as errors instead of being silently ignored.
func strictUnmarshal(data []byte, v any) error {
	d := toml.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

// ChainNames returns the configured chain names in sorted order (stable wizard
// menu ordering).
func (fc *FileConfig) ChainNames() []string {
	names := make([]string, 0, len(fc.Chains))
	for name := range fc.Chains {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

Also add the small `bytesReader` helper at the bottom of the file (avoids an extra import alias confusion):

```go
import "bytes" // add to the import block

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
```

> Implementation note: delete the `dec := toml.NewDecoder(nil)` placeholder line shown above — it was illustrative. The real decode path is `strictUnmarshal`. Final `LoadFileConfig` body is:
>
> ```go
> 	var fc FileConfig
> 	if err := strictUnmarshal(data, &fc); err != nil {
> 		return nil, fmt.Errorf("parse config %s: %w", path, err)
> 	}
> 	return &fc, nil
> ```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go mod tidy && go test ./tests/gen-activity/ -run TestLoadFileConfig -v
```

Expected: PASS for all three subtests. `go mod tidy` moves `pelletier/go-toml/v2` to a direct dependency now that it is imported.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/configfile.go devnet/tests/gen-activity/configfile_test.go devnet/go.mod devnet/go.sum
git commit -m "feat(gen-activity): parse gen-activity-config.toml chains/common sections"
```

---

## Task 5: Config layering with flag precedence (`applyLayer` + `ApplyFileConfig`)

**Files:**
- Modify: `devnet/tests/gen-activity/configfile.go`
- Test: `devnet/tests/gen-activity/configfile_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/configfile_test.go`:

```go
func TestApplyFileConfigPrecedence(t *testing.T) {
	fc, err := LoadFileConfig(writeTempTOML(t, sampleTOML))
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}

	t.Run("common then chain overlay onto unset fields", func(t *testing.T) {
		c := &Config{}
		setFlags := map[string]bool{} // nothing set explicitly
		if err := ApplyFileConfig(c, fc, "devnet", setFlags); err != nil {
			t.Fatalf("ApplyFileConfig: %v", err)
		}
		if c.FundingKey != "faucet" { // from [common]
			t.Errorf("FundingKey = %q, want faucet", c.FundingKey)
		}
		if c.Parallelism != 8 { // from [common]
			t.Errorf("Parallelism = %d, want 8", c.Parallelism)
		}
		if c.ChainID != "lumera-devnet-1" { // from [chains.devnet]
			t.Errorf("ChainID = %q, want lumera-devnet-1", c.ChainID)
		}
	})

	t.Run("explicit flags win over config", func(t *testing.T) {
		c := &Config{FundingKey: "cli-funder", Parallelism: 99}
		setFlags := map[string]bool{"funding-key": true, "parallelism": true}
		if err := ApplyFileConfig(c, fc, "devnet", setFlags); err != nil {
			t.Fatalf("ApplyFileConfig: %v", err)
		}
		if c.FundingKey != "cli-funder" {
			t.Errorf("FundingKey = %q, want cli-funder (flag wins)", c.FundingKey)
		}
		if c.Parallelism != 99 {
			t.Errorf("Parallelism = %d, want 99 (flag wins)", c.Parallelism)
		}
		if c.ChainID != "lumera-devnet-1" { // not set as flag → config applies
			t.Errorf("ChainID = %q, want lumera-devnet-1", c.ChainID)
		}
	})

	t.Run("unknown chain errors", func(t *testing.T) {
		c := &Config{}
		err := ApplyFileConfig(c, fc, "nosuchchain", map[string]bool{})
		if err == nil {
			t.Error("expected error for unknown chain name")
		}
	})

	t.Run("empty chain applies only common", func(t *testing.T) {
		c := &Config{}
		if err := ApplyFileConfig(c, fc, "", map[string]bool{}); err != nil {
			t.Fatalf("ApplyFileConfig: %v", err)
		}
		if c.FundingKey != "faucet" {
			t.Errorf("FundingKey = %q, want faucet (common)", c.FundingKey)
		}
		if c.ChainID != "" {
			t.Errorf("ChainID = %q, want empty (no chain selected)", c.ChainID)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestApplyFileConfigPrecedence -v
```

Expected: FAIL — `undefined: ApplyFileConfig`.

- [ ] **Step 3: Write the implementation**

Add to `devnet/tests/gen-activity/configfile.go`:

```go
// ApplyFileConfig layers a parsed FileConfig onto c following the precedence
// defaults < [common] < [chains.<chain>] < explicit CLI flags. setFlags is the
// set of flag names the user passed on the command line (collected via
// flag.Visit); fields whose flag was set are never overwritten by the config.
// An empty chain applies only [common]; a non-empty chain that is absent from
// the file is an error.
func ApplyFileConfig(c *Config, fc *FileConfig, chain string, setFlags map[string]bool) error {
	if fc == nil {
		return nil
	}
	applyLayer(c, fc.Common, setFlags)
	if chain == "" {
		return nil
	}
	sec, ok := fc.Chains[chain]
	if !ok {
		return fmt.Errorf("chain %q not found in config (available: %v)", chain, fc.ChainNames())
	}
	applyLayer(c, sec, setFlags)
	return nil
}

// applyLayer overlays the non-nil fields of sec onto c, skipping any field
// whose corresponding flag name appears in setFlags (so explicit CLI flags are
// never clobbered). The flag names here MUST match those registered in
// configureFlags.
func applyLayer(c *Config, sec ChainSection, setFlags map[string]bool) {
	str := func(flagName string, src *string, dst *string) {
		if src != nil && !setFlags[flagName] {
			*dst = *src
		}
	}
	intf := func(flagName string, src *int, dst *int) {
		if src != nil && !setFlags[flagName] {
			*dst = *src
		}
	}
	boolf := func(flagName string, src *bool, dst *bool) {
		if src != nil && !setFlags[flagName] {
			*dst = *src
		}
	}

	str("bin", sec.Bin, &c.Bin)
	str("rpc", sec.RPC, &c.RPC)
	str("grpc", sec.GRPC, &c.GRPC)
	str("chain-id", sec.ChainID, &c.ChainID)
	str("home", sec.Home, &c.Home)
	str("keyring-backend", sec.KeyringBackend, &c.KeyringBackend)
	str("evm-cutover-version", sec.EVMCutoverVer, &c.EVMCutoverVer)
	str("funding-key", sec.FundingKey, &c.FundingKey)
	str("accounts", sec.AccountsPath, &c.AccountsPath)
	str("max-account-amount", sec.MaxAccountAmount, &c.MaxAccountAmount)
	str("account-prefix", sec.AccountPrefix, &c.AccountPrefix)
	intf("num-accounts", sec.NumAccounts, &c.NumAccounts)
	intf("funding-batch-size", sec.FundingBatchSize, &c.FundingBatchSize)
	intf("parallelism", sec.Parallelism, &c.Parallelism)
	intf("num-multisig23-accounts", sec.NumMultisig23, &c.NumMultisig23)
	intf("num-multisig35-accounts", sec.NumMultisig35, &c.NumMultisig35)
	boolf("actions", sec.Actions, &c.Actions)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestApplyFileConfigPrecedence -v
```

Expected: PASS for all four subtests.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/configfile.go devnet/tests/gen-activity/configfile_test.go
git commit -m "feat(gen-activity): layer config file with flag>chain>common>default precedence"
```

---

## Task 6: Wire config loading + mode selection into `main`

**Files:**
- Modify: `devnet/tests/gen-activity/main.go`
- Test: `devnet/tests/gen-activity/main_test.go`

This task extracts the load/layer/mode logic into a testable helper `resolveConfig`, then calls it from `main`. `main` itself stays a thin shell (untested), but `resolveConfig` is unit-tested.

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/main_test.go`:

```go
func TestResolveConfigAppliesFileAndDetectsWizard(t *testing.T) {
	t.Run("no flags -> wizard mode, common applied", func(t *testing.T) {
		cfg := &Config{}
		fs := flag.NewFlagSet("gen", flag.ContinueOnError)
		configureFlags(fs, cfg)
		if err := fs.Parse([]string{}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		path := writeTempTOML(t, sampleTOML)
		cfg.ConfigPath = path

		wizard, err := resolveConfig(cfg, fs)
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if !wizard {
			t.Error("expected wizard=true when no flags passed")
		}
		if cfg.FundingKey != "faucet" {
			t.Errorf("FundingKey = %q, want faucet (from common)", cfg.FundingKey)
		}
	})

	t.Run("flags passed -> command-line mode", func(t *testing.T) {
		cfg := &Config{}
		fs := flag.NewFlagSet("gen", flag.ContinueOnError)
		configureFlags(fs, cfg)
		path := writeTempTOML(t, sampleTOML)
		if err := fs.Parse([]string{"-chain", "devnet", "-config", path}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		wizard, err := resolveConfig(cfg, fs)
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if wizard {
			t.Error("expected wizard=false when flags passed")
		}
		if cfg.ChainID != "lumera-devnet-1" {
			t.Errorf("ChainID = %q, want lumera-devnet-1 (chain layered)", cfg.ChainID)
		}
	})

	t.Run("-w forces wizard even with flags", func(t *testing.T) {
		cfg := &Config{}
		fs := flag.NewFlagSet("gen", flag.ContinueOnError)
		configureFlags(fs, cfg)
		path := writeTempTOML(t, sampleTOML)
		if err := fs.Parse([]string{"-w", "-config", path}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		wizard, err := resolveConfig(cfg, fs)
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if !wizard {
			t.Error("expected wizard=true when -w passed")
		}
	})

	t.Run("explicit missing -config is an error", func(t *testing.T) {
		cfg := &Config{}
		fs := flag.NewFlagSet("gen", flag.ContinueOnError)
		configureFlags(fs, cfg)
		if err := fs.Parse([]string{"-config", filepath.Join(t.TempDir(), "nope.toml"), "-chain", "devnet"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if _, err := resolveConfig(cfg, fs); err == nil {
			t.Error("expected error when explicit -config path is missing")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestResolveConfigAppliesFileAndDetectsWizard -v
```

Expected: FAIL — `undefined: resolveConfig`.

- [ ] **Step 3: Write the implementation**

In `devnet/tests/gen-activity/main.go`, add the helper:

```go
// resolveConfig loads any config file referenced by cfg.ConfigPath, layers it
// onto cfg honoring CLI-flag precedence, and reports whether the tool should run
// in wizard mode (no flags passed, or -w/-wizard). fs must be the FlagSet that
// already parsed the command line (so fs.Visit/fs.NFlag reflect what the user
// set).
func resolveConfig(cfg *Config, fs *flag.FlagSet) (wizard bool, err error) {
	setFlags := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	fc, err := LoadFileConfig(cfg.ConfigPath)
	if err != nil {
		return false, err
	}
	if fc == nil && setFlags["config"] {
		return false, fmt.Errorf("config file %q not found", cfg.ConfigPath)
	}
	if err := ApplyFileConfig(cfg, fc, cfg.Chain, setFlags); err != nil {
		return false, err
	}

	wizard = cfg.Wizard || fs.NFlag() == 0
	return wizard, nil
}
```

Rewrite `main` to use it (replace the current `main` body):

```go
func main() {
	cfg := &Config{}
	configureFlags(flag.CommandLine, cfg)
	flag.Parse()

	wizard, err := resolveConfig(cfg, flag.CommandLine)
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}

	if wizard {
		fc, _ := LoadFileConfig(cfg.ConfigPath)
		if err := runWizard(cfg, fc, newSurveyPrompter(), run); err != nil {
			log.Fatalf("wizard: %v", err)
		}
		return
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	if err := run(cfg); err != nil {
		log.Fatalf("gen-activity failed: %v", err)
	}
}
```

Delete the now-unused `parseFlags` function (its logic moved into `main`).

> Note: `runWizard` and `newSurveyPrompter` are defined in Task 7/8. This task will not compile until those exist; that is expected. To keep this task's test green in isolation, the test for `resolveConfig` does not call `main`. If you implement strictly task-by-task and need a green build between Task 6 and Task 7, temporarily stub `func runWizard(*Config, *FileConfig, prompter, func(*Config) error) error { return nil }` and `func newSurveyPrompter() prompter { return nil }` in `main.go`, then remove the stubs in Task 7/8. Prefer implementing Tasks 6–8 back-to-back.

- [ ] **Step 4: Run test to verify it passes**

Run (with the temporary stubs from the note, or after Task 8):

```bash
cd devnet && go test ./tests/gen-activity/ -run TestResolveConfigAppliesFileAndDetectsWizard -v
```

Expected: PASS for all four subtests.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/main.go devnet/tests/gen-activity/main_test.go
git commit -m "feat(gen-activity): resolve config file + select wizard vs command-line mode"
```

---

## Task 7: Wizard pure helpers (`prompter`, `menuItems`, `editSetting`)

**Files:**
- Create: `devnet/tests/gen-activity/wizard.go`
- Test: `devnet/tests/gen-activity/wizard_test.go`

- [ ] **Step 1: Write the failing test**

Create `devnet/tests/gen-activity/wizard_test.go`:

```go
package main

import (
	"strings"
	"testing"
)

func TestMenuItemsRenderCurrentValues(t *testing.T) {
	c := &Config{
		FundingKey:    "faucet",
		NumAccounts:   10,
		NumMultisig23: 2,
		Parallelism:   5,
		Actions:       true,
		AccountsPath:  "accounts.json",
	}
	items := menuItems(c)

	// Must include the run + quit sentinels.
	joined := strings.Join(items, "\n")
	if !strings.Contains(joined, menuRun) {
		t.Errorf("menu missing run entry %q:\n%s", menuRun, joined)
	}
	if !strings.Contains(joined, menuQuit) {
		t.Errorf("menu missing quit entry %q:\n%s", menuQuit, joined)
	}
	// Current values rendered into labels.
	if !strings.Contains(joined, "faucet") || !strings.Contains(joined, "10") {
		t.Errorf("menu missing current values:\n%s", joined)
	}
}

func TestEditSettingUpdatesConfig(t *testing.T) {
	c := &Config{FundingKey: "old", NumAccounts: 1, Actions: false}
	p := &fakePrompter{
		inputs:   map[string]string{"funding-key": "newfunder", "num-accounts": "7"},
		confirms: map[string]bool{"actions": true},
	}

	if err := editSetting(c, settingFundingKey, p); err != nil {
		t.Fatalf("edit funding-key: %v", err)
	}
	if c.FundingKey != "newfunder" {
		t.Errorf("FundingKey = %q, want newfunder", c.FundingKey)
	}

	if err := editSetting(c, settingNumAccounts, p); err != nil {
		t.Fatalf("edit num-accounts: %v", err)
	}
	if c.NumAccounts != 7 {
		t.Errorf("NumAccounts = %d, want 7", c.NumAccounts)
	}

	if err := editSetting(c, settingActions, p); err != nil {
		t.Fatalf("edit actions: %v", err)
	}
	if !c.Actions {
		t.Errorf("Actions = %v, want true", c.Actions)
	}
}

func TestEditSettingRejectsBadInt(t *testing.T) {
	c := &Config{NumAccounts: 3}
	p := &fakePrompter{inputs: map[string]string{"num-accounts": "notanumber"}}
	if err := editSetting(c, settingNumAccounts, p); err == nil {
		t.Error("expected error for non-integer num-accounts input")
	}
	if c.NumAccounts != 3 {
		t.Errorf("NumAccounts mutated to %d on bad input, want unchanged 3", c.NumAccounts)
	}
}

// fakePrompter scripts wizard answers keyed by setting key. selectQueue is a
// FIFO of answers for SelectOne calls.
type fakePrompter struct {
	selectQueue []string
	inputs      map[string]string
	confirms    map[string]bool
}

func (f *fakePrompter) SelectOne(label string, options []string, def string) (string, error) {
	if len(f.selectQueue) == 0 {
		return def, nil
	}
	ans := f.selectQueue[0]
	f.selectQueue = f.selectQueue[1:]
	return ans, nil
}

func (f *fakePrompter) Input(key, label, def string) (string, error) {
	if v, ok := f.inputs[key]; ok {
		return v, nil
	}
	return def, nil
}

func (f *fakePrompter) Confirm(key, label string, def bool) (bool, error) {
	if v, ok := f.confirms[key]; ok {
		return v, nil
	}
	return def, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestMenuItems|TestEditSetting' -v
```

Expected: FAIL — `undefined: menuItems` / `settingFundingKey` etc.

- [ ] **Step 3: Write the implementation**

Create `devnet/tests/gen-activity/wizard.go`:

```go
package main

import (
	"fmt"
	"strconv"
)

// prompter abstracts the interactive prompts so the wizard's control flow is
// unit-testable with a scripted fake. The production implementation
// (surveyPrompter) is backed by github.com/AlecAivazis/survey/v2. Input/Confirm
// take a stable `key` (the setting key) plus a human label so tests can script
// answers without matching on display text.
type prompter interface {
	SelectOne(label string, options []string, def string) (string, error)
	Input(key, label, def string) (string, error)
	Confirm(key, label string, def bool) (bool, error)
}

// Setting keys: stable identifiers for editable settings.
const (
	settingFundingKey      = "funding-key"
	settingMode            = "mode"
	settingNumAccounts     = "num-accounts"
	settingNumMultisig23   = "num-multisig23-accounts"
	settingNumMultisig35   = "num-multisig35-accounts"
	settingAccountsPath    = "accounts"
	settingParallelism     = "parallelism"
	settingActions         = "actions"
	settingFundingBatch    = "funding-batch-size"
	settingMaxAmount       = "max-account-amount"
	settingDryRun          = "dry-run"
)

// Menu sentinels.
const (
	menuRun  = "▶ Run now"
	menuQuit = "✗ Quit"
)

// editableSettings is the ordered list of setting keys shown in the menu.
var editableSettings = []string{
	settingFundingKey,
	settingMode,
	settingNumAccounts,
	settingNumMultisig23,
	settingNumMultisig35,
	settingAccountsPath,
	settingParallelism,
	settingActions,
	settingFundingBatch,
	settingMaxAmount,
	settingDryRun,
}

// settingValue returns the current value of a setting as a display string.
func settingValue(c *Config, key string) string {
	switch key {
	case settingFundingKey:
		return c.FundingKey
	case settingMode:
		return modeLabel(c)
	case settingNumAccounts:
		return strconv.Itoa(c.NumAccounts)
	case settingNumMultisig23:
		return strconv.Itoa(c.NumMultisig23)
	case settingNumMultisig35:
		return strconv.Itoa(c.NumMultisig35)
	case settingAccountsPath:
		return c.AccountsPath
	case settingParallelism:
		return strconv.Itoa(c.Parallelism)
	case settingActions:
		return strconv.FormatBool(c.Actions)
	case settingFundingBatch:
		return strconv.Itoa(c.FundingBatchSize)
	case settingMaxAmount:
		return c.MaxAccountAmount
	case settingDryRun:
		return strconv.FormatBool(c.DryRun)
	default:
		return ""
	}
}

// modeLabel renders the current run mode derived from the AddAccounts /
// ActivityExisting flags.
func modeLabel(c *Config) string {
	switch {
	case c.AddAccounts:
		return "add-accounts"
	case c.ActivityExisting:
		return "activity-existing"
	default:
		return "fresh"
	}
}

// menuItems renders the settings menu: one entry per editable setting (with its
// current value) followed by the Run and Quit sentinels.
func menuItems(c *Config) []string {
	items := make([]string, 0, len(editableSettings)+2)
	for _, key := range editableSettings {
		items = append(items, fmt.Sprintf("%-24s %s", key, settingValue(c, key)))
	}
	items = append(items, menuRun, menuQuit)
	return items
}

// menuKeyFromItem maps a rendered menu line back to its setting key (or the
// sentinel itself for Run/Quit).
func menuKeyFromItem(item string) string {
	if item == menuRun || item == menuQuit {
		return item
	}
	// The key is the first whitespace-delimited token.
	for i := 0; i < len(item); i++ {
		if item[i] == ' ' {
			return item[:i]
		}
	}
	return item
}

// editSetting prompts for a new value for one setting and applies it to c. A
// parse error on a numeric field is returned and leaves c unchanged.
func editSetting(c *Config, key string, p prompter) error {
	switch key {
	case settingFundingKey:
		v, err := p.Input(key, "Funder key name", c.FundingKey)
		if err != nil {
			return err
		}
		c.FundingKey = v
	case settingMode:
		v, err := p.SelectOne("Run mode", []string{"fresh", "add-accounts", "activity-existing"}, modeLabel(c))
		if err != nil {
			return err
		}
		c.AddAccounts = v == "add-accounts"
		c.ActivityExisting = v == "activity-existing"
	case settingAccountsPath:
		v, err := p.Input(key, "Accounts registry path", c.AccountsPath)
		if err != nil {
			return err
		}
		c.AccountsPath = v
	case settingMaxAmount:
		v, err := p.Input(key, "Max per-account amount", c.MaxAccountAmount)
		if err != nil {
			return err
		}
		c.MaxAccountAmount = v
	case settingActions:
		v, err := p.Confirm(key, "Include CASCADE action activity?", c.Actions)
		if err != nil {
			return err
		}
		c.Actions = v
	case settingDryRun:
		v, err := p.Confirm(key, "Dry run (plan only, no txs)?", c.DryRun)
		if err != nil {
			return err
		}
		c.DryRun = v
	case settingNumAccounts:
		return editInt(c, key, "Number of accounts", &c.NumAccounts, p)
	case settingNumMultisig23:
		return editInt(c, key, "Number of 2-of-3 multisig accounts", &c.NumMultisig23, p)
	case settingNumMultisig35:
		return editInt(c, key, "Number of 3-of-5 multisig accounts", &c.NumMultisig35, p)
	case settingParallelism:
		return editInt(c, key, "Parallelism", &c.Parallelism, p)
	case settingFundingBatch:
		return editInt(c, key, "Funding batch size", &c.FundingBatchSize, p)
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
	return nil
}

// editInt prompts for an integer setting and applies it only on a clean parse.
func editInt(c *Config, key, label string, dst *int, p prompter) error {
	v, err := p.Input(key, label, strconv.Itoa(*dst))
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%s: %q is not an integer", key, v)
	}
	*dst = n
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestMenuItems|TestEditSetting' -v
```

Expected: PASS for all subtests.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/wizard.go devnet/tests/gen-activity/wizard_test.go
git commit -m "feat(gen-activity): wizard menu rendering + per-setting editing"
```

---

## Task 8: Wizard loop + survey-backed prompter + main wiring

**Files:**
- Modify: `devnet/tests/gen-activity/wizard.go`
- Test: `devnet/tests/gen-activity/wizard_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/wizard_test.go`:

```go
func TestRunWizardEditsThenRuns(t *testing.T) {
	fc, err := LoadFileConfig(writeTempTOML(t, sampleTOML))
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}
	cfg := &Config{
		Bin: "lumerad", KeyringBackend: "test",
		MaxAccountAmount: "10000000ulume", ActionStates: "pending,done,approved",
		MaxActionsPerRun: 3, FundingBatchSize: 10, Parallelism: 5,
	}

	// Script: pick chain "devnet", edit funding-key, then Run.
	p := &fakePrompter{
		selectQueue: []string{
			"devnet",                          // chain picker
			menuKeyFromItem(menuItems(cfg)[0]), // first menu choice -> funding-key setting line
			menuRun,                            // then run
		},
		inputs: map[string]string{"funding-key": "wizfunder"},
	}

	var ran *Config
	runner := func(c *Config) error { ran = c; return nil }

	if err := runWizard(cfg, fc, p, runner); err != nil {
		t.Fatalf("runWizard: %v", err)
	}
	if ran == nil {
		t.Fatal("runner was not invoked on Run")
	}
	if ran.ChainID != "lumera-devnet-1" {
		t.Errorf("ChainID = %q, want lumera-devnet-1 (chain applied)", ran.ChainID)
	}
	if ran.FundingKey != "wizfunder" {
		t.Errorf("FundingKey = %q, want wizfunder (edited)", ran.FundingKey)
	}
}

func TestRunWizardQuitDoesNotRun(t *testing.T) {
	cfg := &Config{Bin: "lumerad"}
	p := &fakePrompter{selectQueue: []string{menuQuit}}
	called := false
	runner := func(*Config) error { called = true; return nil }

	if err := runWizard(cfg, nil, p, runner); err != nil {
		t.Fatalf("runWizard: %v", err)
	}
	if called {
		t.Error("runner must not be called when user quits")
	}
}
```

> Note: the menu line selected (`menuItems(cfg)[0]`) is the funding-key line because `settingFundingKey` is first in `editableSettings`. `runWizard` maps the selected line back to its key via `menuKeyFromItem`.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestRunWizard -v
```

Expected: FAIL — `undefined: runWizard`.

- [ ] **Step 3: Write the implementation**

Add to `devnet/tests/gen-activity/wizard.go` (add `github.com/AlecAivazis/survey/v2` to the import block; `fmt` and `strconv` are already imported from Task 7):

```go
// runWizard drives the interactive flow: optionally pick a chain (re-seeding cfg
// from that chain's config section), then loop the settings menu until the user
// chooses Run (validate + invoke runner) or Quit (return without running).
// runner is injected so tests can assert invocation without a live chain.
func runWizard(cfg *Config, fc *FileConfig, p prompter, runner func(*Config) error) error {
	if fc != nil && len(fc.Chains) > 0 {
		names := fc.ChainNames()
		def := cfg.Chain
		if def == "" {
			def = names[0]
		}
		chosen, err := p.SelectOne("Select chain", names, def)
		if err != nil {
			return err
		}
		cfg.Chain = chosen
		// Re-seed defaults from the chosen chain (wizard is interactive: the
		// chain's values become the working defaults the user can then edit).
		applyLayer(cfg, fc.Common, nil)
		applyLayer(cfg, fc.Chains[chosen], nil)
	} else if fc == nil || len(fc.Chains) == 0 {
		// Manual entry when there is no config file / no chains defined.
		if err := promptManualChain(cfg, p); err != nil {
			return err
		}
	}

	for {
		choice, err := p.SelectOne("Settings (select to edit, then Run)", menuItems(cfg), menuRun)
		if err != nil {
			return err
		}
		key := menuKeyFromItem(choice)
		switch key {
		case menuRun:
			if err := cfg.Validate(); err != nil {
				fmt.Printf("  cannot run: %v\n", err)
				continue
			}
			return runner(cfg)
		case menuQuit:
			return nil
		default:
			if err := editSetting(cfg, key, p); err != nil {
				fmt.Printf("  %v\n", err)
			}
		}
	}
}

// promptManualChain asks for the minimum connection settings when no config file
// is present, so a bare `gen-activity` invocation still works.
func promptManualChain(cfg *Config, p prompter) error {
	rpc, err := p.Input("rpc", "RPC endpoint", cfg.RPC)
	if err != nil {
		return err
	}
	cfg.RPC = rpc
	grpc, err := p.Input("grpc", "gRPC endpoint", cfg.GRPC)
	if err != nil {
		return err
	}
	cfg.GRPC = grpc
	chainID, err := p.Input("chain-id", "Chain ID", cfg.ChainID)
	if err != nil {
		return err
	}
	cfg.ChainID = chainID
	return nil
}
```

Add the survey-backed prompter at the bottom of `wizard.go`:

```go
// surveyPrompter is the production prompter backed by survey/v2.
type surveyPrompter struct{}

func newSurveyPrompter() prompter { return surveyPrompter{} }

func (surveyPrompter) SelectOne(label string, options []string, def string) (string, error) {
	answer := def
	prompt := &survey.Select{Message: label, Options: options, Default: def}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

// The first parameter (the setting key) is unused by the survey impl but
// required by the prompter interface, so it is named _.
func (surveyPrompter) Input(_ string, label, def string) (string, error) {
	answer := def
	prompt := &survey.Input{Message: label, Default: def}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

func (surveyPrompter) Confirm(_ string, label string, def bool) (bool, error) {
	answer := def
	prompt := &survey.Confirm{Message: label, Default: def}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return false, err
	}
	return answer, nil
}
```

> Implementation note: remove the temporary stubs added in Task 6
> (`runWizard`/`newSurveyPrompter`) now that the real implementations exist.
> `runWizard` does not use `os`/`sort` — only add an import if your final code
> references it (the code above imports just `fmt`, `strconv`, and
> `github.com/AlecAivazis/survey/v2`).

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestRunWizard -v
```

Expected: PASS for both subtests.

- [ ] **Step 5: Run the full gen-activity package test + build**

Run:

```bash
cd devnet && go build ./... && go test ./tests/gen-activity/ -v
```

Expected: builds clean; all gen-activity tests pass (existing + new).

- [ ] **Step 6: Commit**

```bash
git add devnet/tests/gen-activity/wizard.go devnet/tests/gen-activity/wizard_test.go devnet/tests/gen-activity/main.go
git commit -m "feat(gen-activity): interactive wizard loop with survey prompts as the default mode"
```

---

## Task 9: Example config + documentation

**Files:**
- Create: `gen-activity-config.toml.example`
- Modify: `docs/design/gen-activity-design.md`

- [ ] **Step 1: Write the example config**

Create `gen-activity-config.toml.example` (repo root):

```toml
# gen-activity-config.toml — copy to gen-activity-config.toml and edit.
#
# Precedence (low -> high): built-in defaults < [common] < [chains.<name>] <
# explicit CLI flags. Select a chain with `-chain <name>`. Keys mirror the CLI
# flag names. Running with NO flags launches the interactive wizard.

[common]
bin = "lumerad"
home = "~/.lumera"
keyring-backend = "test"
funding-key = "faucet"
evm-cutover-version = "v1.20.0"
account-prefix = "gen"
max-account-amount = "10000000ulume"
parallelism = 5
funding-batch-size = 10
actions = true

[chains.devnet]
rpc = "tcp://localhost:26657"
grpc = "localhost:9090"
chain-id = "lumera-devnet-1"
accounts = "devnet/tests/gen-activity/accounts-devnet.json"

[chains.testnet]
rpc = "https://rpc.testnet.lumera.io:443"
grpc = "grpc.testnet.lumera.io:443"
chain-id = "lumera-testnet-1"
accounts = "devnet/tests/gen-activity/accounts-testnet.json"

[chains.mainnet]
rpc = "https://rpc.lumera.io:443"
grpc = "grpc.lumera.io:443"
chain-id = "lumera-mainnet-1"
accounts = "devnet/tests/gen-activity/accounts-mainnet.json"
```

- [ ] **Step 2: Document in the design doc**

In `docs/design/gen-activity-design.md`, add a section (place after the existing flags/configuration section; match the doc's heading style):

```markdown
## Config file, chain selection, and wizard

`gen-activity` reads an optional `gen-activity-config.toml` (path via `-config`,
default `gen-activity-config.toml` in the working directory). It has a shared
`[common]` section and any number of `[chains.<name>]` sections; select one with
`-chain <name>`. TOML keys mirror the CLI flag names.

**Precedence** (low to high): built-in defaults → `[common]` → `[chains.<name>]`
→ explicitly-set CLI flags. A flag passed on the command line always wins over
the config file (implemented via `flag.Visit`).

**Mode selection:**
- No flags at all → the interactive **wizard** (arrow-key chain picker + settings
  menu, powered by survey/v2).
- Any flag passed → non-interactive command-line mode (unchanged behavior).
- `-w` / `-wizard` → force the wizard even with flags present (those flags
  pre-seed the wizard defaults).

See `gen-activity-config.toml.example` for a documented template.
```

- [ ] **Step 3: Verify the build and run lint**

Run:

```bash
cd devnet && go build ./... && go vet ./tests/gen-activity/
cd /home/akobrin/p/lumera && make lint
```

Expected: build clean, vet clean, `make lint` reports 0 issues.

- [ ] **Step 4: Commit**

```bash
git add gen-activity-config.toml.example docs/design/gen-activity-design.md
git commit -m "docs(gen-activity): example config + document config/wizard/mode behavior"
```

---

## Self-Review checklist (for the plan author)

- **Spec coverage:** §3.1 config file → Tasks 4–6 + 9; §3.2 mode + wizard → Tasks 3, 6, 7, 8; multisig-flag surface (deferred behavior) → Tasks 2, 3, 7; deps/docs → Tasks 1, 9. The multisig *generation behavior* (spec §3.3, §3.4) is intentionally out of scope here — covered by the companion multisig plan.
- **Placeholder scan:** the two "Implementation note" blocks in Tasks 4 and 8 flag illustrative lines to delete/rename; they contain the exact final code, not TODOs.
- **Type consistency:** `prompter` interface (`SelectOne`/`Input`/`Confirm`) is identical across `wizard.go` and the `fakePrompter` test; `Config` field names (`NumMultisig23`, `NumMultisig35`, `ConfigPath`, `Chain`, `Wizard`) match between config.go, configfile.go, main.go, and wizard.go; flag names in `applyLayer` match those registered in `configureFlags`.
```
