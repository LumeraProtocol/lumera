# gen-activity Vesting Accounts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add vesting account generation to gen-activity: `-vesting-percent` randomly designates a share of regular accounts as continuous/delayed vesting, and `-num-permanent-locked-accounts` creates a dedicated set of PermanentLocked accounts. All are created at funding time via `x/vesting` `create-*` txs, then exercised by the normal activity mix.

**Architecture:** A new `common.Vesting`-style helper set (`common/vesting.go`) provides pure CLI arg builders + funder-signed wrappers for `create-vesting-account` (continuous / `--delayed`) and `create-permanent-locked-account`. gen-activity tags selected accounts with a `VestingInfo` before funding, then a funding split routes vesting/locked accounts through a create-tx + liquid top-up path instead of the batched bank-send. Registry schema v2 (from the multisig plan) carries `AccountRecord.Vesting`.

**Tech Stack:** Go, `gen` module, `common.ChainCLI`, stock Cosmos-SDK `x/vesting` CLI. Companion to `gen-activity-config-wizard-plan.md` and `gen-activity-multisig-plan.md`.

**Prerequisites:**
- The multisig plan's Task 4 (registry **schema v2**, `MultisigInfo`, `AccountRecord` with embedded identity/activity) must be done — this plan adds a sibling `Vesting` field.
- The config/wizard plan's flag-registration + wizard menu must be done — this plan adds two flags and two wizard entries.

**SDK constraint (why "at funding time"):** `MsgCreateVestingAccount` / `MsgCreatePermanentLockedAccount` reject a destination that already exists on-chain. An account that has been bank-funded or done activity already exists, so it cannot be converted. Hence vesting/locked accounts are created by the funding tx itself (the account does not exist on-chain until then), and the regular bank-send funding path must skip them.

---

## File Structure

New:
- `devnet/tests/common/vesting.go` — vesting/permanent-locked arg builders + funder-signed wrappers.
- `devnet/tests/common/vesting_test.go` — arg-builder tests.
- `devnet/tests/gen-activity/vesting.go` — selection/assignment, permanent-locked generation, funding split, vesting funder.
- `devnet/tests/gen-activity/vesting_test.go` — selection + split tests.

Modified:
- `devnet/tests/gen-activity/config.go` — `VestingPercent`, `NumPermanentLocked` fields + validation.
- `devnet/tests/gen-activity/main.go` — register flags; tag/generate/fund vesting accounts in `run()`.
- `devnet/tests/gen-activity/registry.go` — `VestingInfo` type + `AccountRecord.Vesting` field.
- `devnet/tests/gen-activity/wizard.go` — two new editable settings.
- `docs/design/gen-activity-design.md` — document vesting accounts.

---

## Task 1: Common vesting CLI helpers

**Files:**
- Create: `devnet/tests/common/vesting.go`
- Test: `devnet/tests/common/vesting_test.go`

- [ ] **Step 1: Write the failing test**

Create `devnet/tests/common/vesting_test.go`:

```go
package common

import "testing"

func vcontains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestVestingCreateArgsContinuous(t *testing.T) {
	args := VestingCreateArgs("faucet", "lumera1to", "5000000ulume", 1800000000, false)
	for _, want := range []string{"vesting", "create-vesting-account", "lumera1to", "5000000ulume", "1800000000"} {
		if !vcontains(args, want) {
			t.Errorf("continuous vesting args missing %q: %v", want, args)
		}
	}
	if !vcontains(args, "--from") || !vcontains(args, "faucet") {
		t.Errorf("must sign with funder: %v", args)
	}
	if vcontains(args, "--delayed") {
		t.Errorf("continuous vesting must NOT pass --delayed: %v", args)
	}
}

func TestVestingCreateArgsDelayed(t *testing.T) {
	args := VestingCreateArgs("faucet", "lumera1to", "5000000ulume", 1800000000, true)
	if !vcontains(args, "--delayed") {
		t.Errorf("delayed vesting must pass --delayed: %v", args)
	}
}

func TestPermanentLockedArgs(t *testing.T) {
	args := PermanentLockedArgs("faucet", "lumera1to", "5000000ulume")
	for _, want := range []string{"vesting", "create-permanent-locked-account", "lumera1to", "5000000ulume"} {
		if !vcontains(args, want) {
			t.Errorf("permanent-locked args missing %q: %v", want, args)
		}
	}
	if !vcontains(args, "--from") || !vcontains(args, "faucet") {
		t.Errorf("must sign with funder: %v", args)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/common/ -run 'TestVestingCreateArgs|TestPermanentLockedArgs' -v
```

Expected: FAIL — `undefined: VestingCreateArgs`.

- [ ] **Step 3: Write the implementation**

Create `devnet/tests/common/vesting.go`:

```go
package common

import (
	"fmt"
	"strconv"
)

// VestingType labels the vesting/locked account variants gen-activity creates.
type VestingType string

const (
	VestingContinuous      VestingType = "continuous"
	VestingDelayed         VestingType = "delayed"
	VestingPermanentLocked VestingType = "permanent_locked"
)

// VestingCreateArgs builds `tx vesting create-vesting-account <to> <amount>
// <end-unix> [--delayed] --from <funder>`. Gas/broadcast/keyring/node flags are
// appended by ChainCLI.SubmitTx, so they are intentionally absent here.
func VestingCreateArgs(funderKey, to, amount string, endUnix int64, delayed bool) []string {
	args := []string{
		"tx", "vesting", "create-vesting-account",
		to, amount, strconv.FormatInt(endUnix, 10),
		"--from", funderKey,
	}
	if delayed {
		args = append(args, "--delayed")
	}
	return args
}

// PermanentLockedArgs builds `tx vesting create-permanent-locked-account <to>
// <amount> --from <funder>`.
func PermanentLockedArgs(funderKey, to, amount string) []string {
	return []string{
		"tx", "vesting", "create-permanent-locked-account",
		to, amount,
		"--from", funderKey,
	}
}

// CreateVestingAccount creates a continuous (delayed=false) or delayed/cliff
// (delayed=true) vesting account funded with amount, signed by funderKey. It
// waits for inclusion (SubmitTx semantics) and returns the tx hash.
func (c *ChainCLI) CreateVestingAccount(funderKey, to, amount string, endUnix int64, delayed bool) (string, error) {
	txHash, err := c.SubmitTx(VestingCreateArgs(funderKey, to, amount, endUnix, delayed)...)
	if err != nil {
		return txHash, fmt.Errorf("create vesting account %s: %w", to, err)
	}
	return txHash, nil
}

// CreatePermanentLockedAccount creates a PermanentLockedAccount funded with
// amount, signed by funderKey.
func (c *ChainCLI) CreatePermanentLockedAccount(funderKey, to, amount string) (string, error) {
	txHash, err := c.SubmitTx(PermanentLockedArgs(funderKey, to, amount)...)
	if err != nil {
		return txHash, fmt.Errorf("create permanent-locked account %s: %w", to, err)
	}
	return txHash, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/common/ -run 'TestVestingCreateArgs|TestPermanentLockedArgs' -v
```

Expected: PASS for all three.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/common/vesting.go devnet/tests/common/vesting_test.go
git commit -m "feat(common): vesting + permanent-locked account create helpers"
```

---

## Task 2: Registry VestingInfo field

**Files:**
- Modify: `devnet/tests/gen-activity/registry.go`
- Test: `devnet/tests/gen-activity/registry_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/registry_test.go`:

```go
func TestVestingRecordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")
	reg := NewRegistry("lumera-devnet-1", "faucet", "", "legacy", "2026-06-12T00:00:00Z")
	reg.UpsertAccount(&AccountRecord{
		AccountIdentity: common.AccountIdentity{Name: "gen-0003", Address: "lumera1vest"},
		Vesting:         &VestingInfo{Type: "continuous", EndTime: 1800000000, LockedAmount: "5000000ulume"},
		Funded:          true,
	})
	if err := reg.Save(path, "2026-06-12T00:00:00Z"); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Accounts[0].Vesting == nil || got.Accounts[0].Vesting.Type != "continuous" {
		t.Fatalf("vesting info not persisted: %+v", got.Accounts[0].Vesting)
	}
	if got.Accounts[0].Vesting.EndTime != 1800000000 {
		t.Errorf("vesting end-time = %d, want 1800000000", got.Accounts[0].Vesting.EndTime)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestVestingRecordRoundTrip -v
```

Expected: FAIL — `VestingInfo` / `Vesting` field undefined.

- [ ] **Step 3: Write the implementation**

In `devnet/tests/gen-activity/registry.go`, add the type:

```go
// VestingInfo describes a vesting or permanent-locked account. Type is one of
// the common.VestingType values ("continuous", "delayed", "permanent_locked").
// EndTime is the unix unlock time (0 for permanent_locked).
type VestingInfo struct {
	Type         string `json:"type"`
	EndTime      int64  `json:"end_time,omitempty"`
	LockedAmount string `json:"locked_amount"`
}
```

In the `AccountRecord` struct, add (next to the `Multisig` field added by the multisig plan):

```go
	Vesting *VestingInfo `json:"vesting,omitempty"`
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestVestingRecordRoundTrip -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/registry.go devnet/tests/gen-activity/registry_test.go
git commit -m "feat(gen-activity): registry VestingInfo for vesting/locked accounts"
```

---

## Task 3: Config flags + validation

**Files:**
- Modify: `devnet/tests/gen-activity/config.go`
- Modify: `devnet/tests/gen-activity/main.go`
- Test: `devnet/tests/gen-activity/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/config_test.go`:

```go
func TestConfigValidateVesting(t *testing.T) {
	t.Run("valid vesting-percent passes", func(t *testing.T) {
		c := validConfig()
		c.VestingPercent = 30
		c.NumPermanentLocked = 2
		if err := c.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("vesting-percent over 100 fails", func(t *testing.T) {
		c := validConfig()
		c.VestingPercent = 101
		if err := c.Validate(); err == nil {
			t.Error("expected error for vesting-percent > 100")
		}
	})
	t.Run("negative vesting-percent fails", func(t *testing.T) {
		c := validConfig()
		c.VestingPercent = -1
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative vesting-percent")
		}
	})
	t.Run("negative num-permanent-locked fails", func(t *testing.T) {
		c := validConfig()
		c.NumPermanentLocked = -1
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative num-permanent-locked-accounts")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestConfigValidateVesting -v
```

Expected: FAIL — `c.VestingPercent undefined`.

- [ ] **Step 3: Add the fields, validation, and flags**

In `devnet/tests/gen-activity/config.go`, add to the `Config` struct (in the "Registry and account generation" block, near the multisig counts):

```go
	VestingPercent     int // percent of regular accounts created as vesting (0-100)
	NumPermanentLocked int // dedicated PermanentLocked accounts
```

In `Config.Validate()`, add after the multisig-count checks:

```go
	if c.VestingPercent < 0 || c.VestingPercent > 100 {
		return fmt.Errorf("-vesting-percent must be in [0,100], got %d", c.VestingPercent)
	}
	if c.NumPermanentLocked < 0 {
		return fmt.Errorf("-num-permanent-locked-accounts must be >= 0, got %d", c.NumPermanentLocked)
	}
```

In `devnet/tests/gen-activity/main.go` `configureFlags`, add after the multisig-count flags:

```go
	fs.IntVar(&c.VestingPercent, "vesting-percent", 0, "percent of regular accounts to create as continuous/delayed vesting (0-100)")
	fs.IntVar(&c.NumPermanentLocked, "num-permanent-locked-accounts", 0, "number of dedicated PermanentLocked accounts to generate")
```

If implementing alongside the config plan, also add these two keys to `ChainSection` in `configfile.go` and to `applyLayer` (mirroring the `num-multisig*` handling):

```go
	// in ChainSection:
	VestingPercent     *int `toml:"vesting-percent"`
	NumPermanentLocked *int `toml:"num-permanent-locked-accounts"`
	// in applyLayer:
	intf("vesting-percent", sec.VestingPercent, &c.VestingPercent)
	intf("num-permanent-locked-accounts", sec.NumPermanentLocked, &c.NumPermanentLocked)
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestConfigValidateVesting -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/config.go devnet/tests/gen-activity/main.go devnet/tests/gen-activity/config_test.go
git commit -m "feat(gen-activity): -vesting-percent and -num-permanent-locked-accounts flags"
```

---

## Task 4: Vesting selection + assignment

**Files:**
- Create: `devnet/tests/gen-activity/vesting.go`
- Test: `devnet/tests/gen-activity/vesting_test.go`

- [ ] **Step 1: Write the failing test**

Create `devnet/tests/gen-activity/vesting_test.go`:

```go
package main

import (
	"math/rand"
	"testing"

	"gen/tests/common"
)

func vestingRecs(n int) []*AccountRecord {
	recs := make([]*AccountRecord, n)
	for i := range recs {
		recs[i] = &AccountRecord{AccountIdentity: common.AccountIdentity{
			Name: "gen-acct", Address: "lumera1a",
		}}
	}
	return recs
}

func TestPlanVestingSelectsPercentage(t *testing.T) {
	recs := vestingRecs(10)
	rng := rand.New(rand.NewSource(1))
	selected := planVesting(recs, 30, "1000000ulume", rng, 1_700_000_000)
	if len(selected) != 3 { // floor(10 * 30 / 100)
		t.Fatalf("selected %d accounts, want 3", len(selected))
	}
	for _, rec := range selected {
		if rec.Vesting == nil {
			t.Fatalf("selected account %s has no Vesting info", rec.Name)
		}
		if rec.Vesting.Type != string(common.VestingContinuous) && rec.Vesting.Type != string(common.VestingDelayed) {
			t.Errorf("vesting type = %q, want continuous or delayed", rec.Vesting.Type)
		}
		if rec.Vesting.Type == string(common.VestingContinuous) || rec.Vesting.Type == string(common.VestingDelayed) {
			if rec.Vesting.EndTime <= 1_700_000_000 {
				t.Errorf("end-time %d must be after now", rec.Vesting.EndTime)
			}
		}
		if rec.Vesting.LockedAmount != "1000000ulume" {
			t.Errorf("locked amount = %q, want 1000000ulume", rec.Vesting.LockedAmount)
		}
	}
}

func TestPlanVestingZeroPercentSelectsNone(t *testing.T) {
	recs := vestingRecs(10)
	rng := rand.New(rand.NewSource(1))
	if selected := planVesting(recs, 0, "1000000ulume", rng, 1_700_000_000); len(selected) != 0 {
		t.Errorf("0%% selected %d accounts, want 0", len(selected))
	}
}

func TestNewPermanentLockedInfo(t *testing.T) {
	info := newPermanentLockedInfo("5000000ulume")
	if info.Type != string(common.VestingPermanentLocked) {
		t.Errorf("type = %q, want permanent_locked", info.Type)
	}
	if info.EndTime != 0 {
		t.Errorf("permanent-locked end-time = %d, want 0", info.EndTime)
	}
	if info.LockedAmount != "5000000ulume" {
		t.Errorf("locked amount = %q, want 5000000ulume", info.LockedAmount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestPlanVesting|TestNewPermanentLockedInfo' -v
```

Expected: FAIL — `undefined: planVesting`.

- [ ] **Step 3: Write the implementation**

Create `devnet/tests/gen-activity/vesting.go`:

```go
package main

import (
	"math/rand"

	"gen/tests/common"
)

// vestingWindow bounds the random vesting end time: now + [1h, 30 days].
const (
	vestingMinSeconds int64 = 3600
	vestingMaxSeconds int64 = 30 * 24 * 3600
)

// planVesting randomly designates floor(len*percent/100) of recs as vesting
// accounts, assigning each a continuous or delayed type and a random end time in
// the vesting window. rng and nowUnix are injected for deterministic tests.
// Returns the selected records (whose .Vesting was set in place).
func planVesting(recs []*AccountRecord, percent int, lockedAmount string, rng *rand.Rand, nowUnix int64) []*AccountRecord {
	if percent <= 0 || len(recs) == 0 {
		return nil
	}
	count := len(recs) * percent / 100
	if count == 0 {
		return nil
	}
	// Random subset via partial Fisher-Yates over a copy of indices.
	idx := rng.Perm(len(recs))[:count]
	selected := make([]*AccountRecord, 0, count)
	for _, i := range idx {
		rec := recs[i]
		typ := common.VestingContinuous
		if rng.Intn(2) == 0 {
			typ = common.VestingDelayed
		}
		endTime := nowUnix + vestingMinSeconds + rng.Int63n(vestingMaxSeconds-vestingMinSeconds+1)
		rec.Vesting = &VestingInfo{
			Type:         string(typ),
			EndTime:      endTime,
			LockedAmount: lockedAmount,
		}
		selected = append(selected, rec)
	}
	return selected
}

// newPermanentLockedInfo builds the VestingInfo for a dedicated PermanentLocked
// account (no end time).
func newPermanentLockedInfo(lockedAmount string) *VestingInfo {
	return &VestingInfo{
		Type:         string(common.VestingPermanentLocked),
		LockedAmount: lockedAmount,
	}
}

// generatePermanentLockedAccounts creates `count` fresh keys named
// "<prefix>-plock-NNNN", tags each with a permanent-locked VestingInfo, upserts
// them into the registry, and returns the new records. Keys are created with the
// detected key style; the accounts are NOT yet on-chain (the funding phase
// creates them via create-permanent-locked-account). Rerun-safe.
func generatePermanentLockedAccounts(cli *common.ChainCLI, reg *ActivityRegistry, accountPrefix string, count int, keyStyle common.KeyStyle, lockedAmount, now string) []*AccountRecord {
	if count <= 0 {
		return nil
	}
	names := reg.AllocateNames(accountPrefix+"-plock", count)
	var recs []*AccountRecord
	for _, name := range names {
		var addr string
		if cli.HasKey(name) {
			a, err := cli.ShowAddress(name)
			if err != nil {
				continue
			}
			addr = a
		} else {
			gk, err := cli.AddKeyWithStyle(name, keyStyle)
			if err != nil {
				continue
			}
			addr = gk.Address
		}
		rec := &AccountRecord{
			AccountIdentity: common.AccountIdentity{Name: name, Address: addr, KeyStyle: keyStyle.Name()},
			Vesting:         newPermanentLockedInfo(lockedAmount),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		reg.UpsertAccount(rec)
		recs = append(recs, rec)
	}
	return recs
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestPlanVesting|TestNewPermanentLockedInfo' -v
```

Expected: PASS for all three.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/vesting.go devnet/tests/gen-activity/vesting_test.go
git commit -m "feat(gen-activity): vesting account selection + permanent-locked generation"
```

---

## Task 5: Funding split + vesting funder

**Files:**
- Modify: `devnet/tests/gen-activity/vesting.go`
- Test: `devnet/tests/gen-activity/vesting_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/vesting_test.go`:

```go
func TestSplitFundingTargets(t *testing.T) {
	reg := NewRegistry("c", "f", "", "legacy", "t")
	reg.Accounts = []*AccountRecord{
		{AccountIdentity: common.AccountIdentity{Name: "regular"}},                                   // bank
		{AccountIdentity: common.AccountIdentity{Name: "msig"}, Multisig: &MultisigInfo{Threshold: 2}}, // bank (composite funded from funder)
		{AccountIdentity: common.AccountIdentity{Name: "vest"}, Vesting: &VestingInfo{Type: "continuous"}}, // vesting
		{AccountIdentity: common.AccountIdentity{Name: "plock"}, Vesting: &VestingInfo{Type: "permanent_locked"}}, // vesting
		{AccountIdentity: common.AccountIdentity{Name: "already"}, Funded: true},                      // skipped
	}
	bank, vesting := splitFundingTargets(reg)

	names := func(recs []*AccountRecord) []string {
		var out []string
		for _, r := range recs {
			out = append(out, r.Name)
		}
		return out
	}
	gotBank := names(bank)
	if len(gotBank) != 2 || gotBank[0] != "regular" || gotBank[1] != "msig" {
		t.Errorf("bank targets = %v, want [regular msig]", gotBank)
	}
	gotVest := names(vesting)
	if len(gotVest) != 2 || gotVest[0] != "vest" || gotVest[1] != "plock" {
		t.Errorf("vesting targets = %v, want [vest plock]", gotVest)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestSplitFundingTargets -v
```

Expected: FAIL — `undefined: splitFundingTargets`.

- [ ] **Step 3: Write the implementation**

Add to `devnet/tests/gen-activity/vesting.go` (add imports `fmt`, `log`, `time`):

```go
// splitFundingTargets divides unfunded accounts into those funded by the normal
// batched bank-send (regular + multisig composites) and those funded by a
// vesting create tx (any account carrying VestingInfo).
func splitFundingTargets(reg *ActivityRegistry) (bank, vesting []*AccountRecord) {
	for _, rec := range reg.Accounts {
		if rec.Funded {
			continue
		}
		if rec.Vesting != nil {
			vesting = append(vesting, rec)
		} else {
			bank = append(bank, rec)
		}
	}
	return bank, vesting
}

// fundVestingAccounts creates each vesting/locked account on-chain via the
// appropriate create-* tx (funding the locked amount), then tops it up with a
// small liquid amount so it can pay gas. Marks Funded on success. Failures are
// logged and skipped (never fatal). Returns the count funded.
func fundVestingAccounts(cli *common.ChainCLI, funderKey, funderAddr string, targets []*AccountRecord, liquidTopUp string) int {
	funded := 0
	for _, rec := range targets {
		if rec.Vesting == nil {
			continue
		}
		if err := createVestingOnChain(cli, funderKey, rec); err != nil {
			log.Printf("  WARN: create vesting account %s: %v", rec.Name, err)
			continue
		}
		// Liquid top-up so the locked account can pay fees / make small sends.
		if _, err := cli.SubmitTx("tx", "bank", "send", funderAddr, rec.Address, liquidTopUp, "--from", funderKey); err != nil {
			log.Printf("  WARN: top up vesting account %s: %v", rec.Name, err)
			// Account exists and is locked; mark funded so it is recorded, but it
			// may be unable to pay gas. Still counts as created.
		}
		rec.Funded = true
		rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		funded++
		log.Printf("  funded %s vesting account %s (%s)", rec.Vesting.Type, rec.Name, rec.Address)
	}
	return funded
}

// createVestingOnChain dispatches the correct create-* tx for the account's
// vesting type.
func createVestingOnChain(cli *common.ChainCLI, funderKey string, rec *AccountRecord) error {
	switch rec.Vesting.Type {
	case string(common.VestingContinuous):
		_, err := cli.CreateVestingAccount(funderKey, rec.Address, rec.Vesting.LockedAmount, rec.Vesting.EndTime, false)
		return err
	case string(common.VestingDelayed):
		_, err := cli.CreateVestingAccount(funderKey, rec.Address, rec.Vesting.LockedAmount, rec.Vesting.EndTime, true)
		return err
	case string(common.VestingPermanentLocked):
		_, err := cli.CreatePermanentLockedAccount(funderKey, rec.Address, rec.Vesting.LockedAmount)
		return err
	default:
		return fmt.Errorf("unknown vesting type %q for %s", rec.Vesting.Type, rec.Name)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestSplitFundingTargets -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/vesting.go devnet/tests/gen-activity/vesting_test.go
git commit -m "feat(gen-activity): funding split + vesting/locked account funder"
```

---

## Task 6: Integrate vesting into `run()`

**Files:**
- Modify: `devnet/tests/gen-activity/main.go`

This task wires Tasks 4–5 into the runtime flow. No new unit test (it is I/O orchestration verified on devnet in Task 8); the gate is build + existing tests pass.

- [ ] **Step 1: Tag regular accounts + generate permanent-locked accounts after key generation**

In `devnet/tests/gen-activity/main.go` `run()`, immediately after the new-account key generation + first `reg.Save` (and after the multisig generation block from the multisig plan), add:

```go
	// Designate a random share of new regular accounts as vesting, and generate
	// dedicated permanent-locked accounts. Tagged BEFORE funding so the funding
	// split routes them to the vesting funder (vesting/locked accounts must be
	// created at funding time — see design §3.5).
	if !cfg.ActivityExisting {
		lockedAmount := common.Coin{Amount: randomFundingAmount(cfg.maxAmount.Amount, rng), Denom: common.ChainDenom}.String()
		if sel := planVesting(newRecs, cfg.VestingPercent, lockedAmount, rng, time.Now().Unix()); len(sel) > 0 {
			log.Printf("designated %d/%d new account(s) as vesting", len(sel), len(newRecs))
		}
		if cfg.NumPermanentLocked > 0 {
			plocked := generatePermanentLockedAccounts(cli, reg, cfg.AccountPrefix, cfg.NumPermanentLocked, keyStyle, lockedAmount, time.Now().UTC().Format(time.RFC3339))
			log.Printf("generated %d permanent-locked account(s)", len(plocked))
		}
		if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return fmt.Errorf("save registry after vesting designation: %w", err)
		}
	}
```

> Note: `rng` and `newRecs` already exist in `run()` (the funding RNG and the new account records). `planVesting` mutates `newRecs` entries in place, so the subsequent registry save persists the `Vesting` tags. Each vesting/locked account currently shares one `lockedAmount`; if you want per-account amounts, move the `randomFundingAmount` call inside `planVesting`/`generatePermanentLockedAccounts` — optional refinement, not required.

- [ ] **Step 2: Replace the funding call with the split**

In `run()`, replace the existing funding block:

```go
	targets := unfundedTargets(reg)
	amountFor := func(*AccountRecord) string {
		return common.Coin{Amount: randomFundingAmount(cfg.maxAmount.Amount, rng), Denom: common.ChainDenom}.String()
	}
	funded, fundErr := FundAccounts(chain, targets, amountFor, cfg.FundingBatchSize, 3)
	log.Printf("funded %d/%d account(s)", funded, len(targets))
```

with:

```go
	bankTargets, vestingTargets := splitFundingTargets(reg)
	amountFor := func(*AccountRecord) string {
		return common.Coin{Amount: randomFundingAmount(cfg.maxAmount.Amount, rng), Denom: common.ChainDenom}.String()
	}
	funded, fundErr := FundAccounts(chain, bankTargets, amountFor, cfg.FundingBatchSize, 3)
	log.Printf("funded %d/%d bank account(s)", funded, len(bankTargets))

	// Vesting/locked accounts are created-and-funded via create-* txs + a liquid
	// top-up so they can pay gas.
	if len(vestingTargets) > 0 {
		vfunded := fundVestingAccounts(cli, cfg.FundingKey, funderAddr, vestingTargets, vestingGasTopUp)
		log.Printf("funded %d/%d vesting/locked account(s)", vfunded, len(vestingTargets))
	}
```

Add the top-up constant near the top of `main.go`:

```go
// vestingGasTopUp is the small liquid balance sent to each vesting/locked
// account so it can pay transaction fees (locked balances cannot cover gas).
const vestingGasTopUp = "1000000ulume"
```

`FundAccounts` already takes `chain` (the `cliFundingChain`); leave it as-is. The `cli` and `funderAddr` variables already exist in `run()`.

- [ ] **Step 3: Build and run the package tests**

Run:

```bash
cd devnet && go build ./... && go test ./tests/gen-activity/ -v
```

Expected: builds clean; all gen-activity tests pass. Vesting/locked accounts now flow through the vesting funder, and (being `Funded`) are picked up by the existing `activityTargets` for the normal activity mix.

- [ ] **Step 4: Commit**

```bash
git add devnet/tests/gen-activity/main.go
git commit -m "feat(gen-activity): create + fund vesting/locked accounts in run()"
```

---

## Task 7: Wizard entries

**Files:**
- Modify: `devnet/tests/gen-activity/wizard.go`
- Test: `devnet/tests/gen-activity/wizard_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/wizard_test.go`:

```go
func TestEditSettingVesting(t *testing.T) {
	c := &Config{}
	p := &fakePrompter{inputs: map[string]string{
		"vesting-percent":               "25",
		"num-permanent-locked-accounts": "3",
	}}
	if err := editSetting(c, settingVestingPercent, p); err != nil {
		t.Fatalf("edit vesting-percent: %v", err)
	}
	if c.VestingPercent != 25 {
		t.Errorf("VestingPercent = %d, want 25", c.VestingPercent)
	}
	if err := editSetting(c, settingNumPermLocked, p); err != nil {
		t.Fatalf("edit num-permanent-locked: %v", err)
	}
	if c.NumPermanentLocked != 3 {
		t.Errorf("NumPermanentLocked = %d, want 3", c.NumPermanentLocked)
	}
}

func TestMenuItemsIncludeVesting(t *testing.T) {
	items := menuItems(&Config{})
	joined := ""
	for _, it := range items {
		joined += it + "\n"
	}
	for _, want := range []string{settingVestingPercent, settingNumPermLocked} {
		found := false
		for _, it := range items {
			if menuKeyFromItem(it) == want {
				found = true
			}
		}
		if !found {
			t.Errorf("menu missing setting %q:\n%s", want, joined)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestEditSettingVesting|TestMenuItemsIncludeVesting' -v
```

Expected: FAIL — `undefined: settingVestingPercent`.

- [ ] **Step 3: Write the implementation**

In `devnet/tests/gen-activity/wizard.go`, add the setting keys (in the existing `const` block of setting keys):

```go
	settingVestingPercent = "vesting-percent"
	settingNumPermLocked  = "num-permanent-locked-accounts"
```

Add them to `editableSettings` (after `settingNumMultisig35`):

```go
	settingVestingPercent,
	settingNumPermLocked,
```

Add cases to `settingValue`:

```go
	case settingVestingPercent:
		return strconv.Itoa(c.VestingPercent)
	case settingNumPermLocked:
		return strconv.Itoa(c.NumPermanentLocked)
```

Add cases to `editSetting`:

```go
	case settingVestingPercent:
		return editInt(c, key, "Vesting percent (0-100)", &c.VestingPercent, p)
	case settingNumPermLocked:
		return editInt(c, key, "Number of permanent-locked accounts", &c.NumPermanentLocked, p)
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestEditSettingVesting|TestMenuItemsIncludeVesting' -v
```

Expected: PASS for both.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/wizard.go devnet/tests/gen-activity/wizard_test.go
git commit -m "feat(gen-activity): wizard entries for vesting-percent and permanent-locked count"
```

---

## Task 8: Documentation + verification

**Files:**
- Modify: `docs/design/gen-activity-design.md`

- [ ] **Step 1: Document vesting accounts**

In `docs/design/gen-activity-design.md`, add a section:

```markdown
## Vesting accounts

`-vesting-percent N` (0–100) randomly designates that share of the run's regular
generated accounts as **vesting** accounts; each is randomly continuous or
delayed (cliff) with an end time in now + [1h, 30d]. `-num-permanent-locked-accounts N`
creates N dedicated **PermanentLocked** accounts (`<prefix>-plock-NNNN`).

Because Cosmos-SDK rejects creating a vesting/locked account at an address that
already exists, these accounts are created **at funding time** via
`tx vesting create-vesting-account` / `create-permanent-locked-account` (funding
the locked amount), then topped up with a small liquid balance so they can pay
gas. They then participate in the normal activity mix (delegation can use locked
coins; sends draw on the liquid top-up). Records carry `AccountRecord.Vesting`
(type, end time, locked amount) under registry schema v2. The create helpers live
in `devnet/tests/common/vesting.go`.
```

- [ ] **Step 2: Full verification sweep**

Run:

```bash
cd devnet && go build ./... && go test ./tests/common/ ./tests/gen-activity/ -v
cd /home/akobrin/p/lumera && make lint
```

Expected: all packages build; all unit tests pass; `make lint` reports 0 issues.

- [ ] **Step 3: Exercise vesting generation (devnet)**

Run (against a running devnet):

```bash
cd devnet/tests/gen-activity
go run . -chain devnet -num-accounts 10 -vesting-percent 30 -num-permanent-locked-accounts 2
```

Expected: ~3 of the 10 regular accounts and 2 dedicated `plock` accounts appear in the accounts JSON with a `vesting` block; `lumerad query auth account <addr>` shows `ContinuousVestingAccount` / `DelayedVestingAccount` / `PermanentLockedAccount` types; the run completes without fatal errors.

- [ ] **Step 4: Commit**

```bash
git add docs/design/gen-activity-design.md
git commit -m "docs(gen-activity): document vesting accounts"
```

---

## Self-Review checklist (for the plan author)

- **Spec coverage:** §3.5 account types + create helpers → Task 1; `VestingInfo` registry → Task 2; flags/validation → Task 3; `-vesting-percent` selection + `plock` generation → Task 4; funding split + funder → Tasks 5–6; activity inclusion → reuses existing `activityTargets` (Funded flag), noted in Task 6; wizard surface → Task 7; testing/docs/devnet verification → Task 8.
- **Placeholder scan:** no TBDs. The Task 3 config-file note and Task 6 per-account-amount note are explicit optional refinements with exact code, not placeholders.
- **Type consistency:** `VestingInfo{Type, EndTime, LockedAmount}` matches across registry.go, vesting.go, and tests; `common.VestingType` constants (`VestingContinuous`/`VestingDelayed`/`VestingPermanentLocked`) are used consistently; `planVesting(recs, percent, lockedAmount, rng, nowUnix)` and `splitFundingTargets(reg) (bank, vesting)` signatures match their tests; wizard `settingVestingPercent`/`settingNumPermLocked` match config field names `VestingPercent`/`NumPermanentLocked`.
- **Dependency note:** assumes `AccountRecord.Multisig` and schema v2 from the multisig plan exist; `splitFundingTargets`'s test references `MultisigInfo`, so run after the multisig plan's Task 4.
- **Coverage gap (intentional, called out):** the on-chain create+topup flow (`fundVestingAccounts`, `createVestingOnChain`) and `run()` wiring are verified on devnet (Task 8 Step 3), not by unit tests, since they require a live funder + chain. Pure logic (selection, split, arg builders) is unit-tested.
```
