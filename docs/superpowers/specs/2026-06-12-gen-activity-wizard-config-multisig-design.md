# gen-activity: config file, wizard mode, and multisig accounts

**Date:** 2026-06-12
**Status:** Approved (design)
**Tool:** `devnet/tests/gen-activity` (Go module `gen`, package `main`)

## 1. Goal

Make the `tests-gen-activity` utility easier to drive against multiple chains and
exercise multisig accounts. Three capabilities:

1. A `gen-activity-config.toml` file predefining named chains (devnet, testnet,
   mainnet) plus a shared `[common]` section, selectable with `-chain`.
2. An interactive wizard that is the **default** when the tool is run with no
   flags; passing any flag runs the existing non-interactive command-line mode.
3. Generation of K-of-N multisig accounts (`-num-multisig23-accounts` = 2-of-3,
   `-num-multisig35-accounts` = 3-of-5) that are created, funded, and exercised.
   The generic multisig ceremony is extracted from `devnet/tests/evmigration`
   into the shared `devnet/tests/common` package and used by both tools.

Non-goals: changing the existing activity mix for regular accounts; changing the
evmigration proof flow (`generate/sign/combine/submit-proof`), its vesting
fixtures, or its validator-discovery logic.

## 2. Background / current state

- `gen-activity` is built on the unit-tested `common.ChainCLI` shell-out
  abstraction and wraps it behind small interfaces (`fundingChain`,
  `activityChain`) with fakes. Config today is a flat flag struct
  (`config.go`) validated in `Config.Validate()`.
- `evmigration/multisig.go` implements the generic multisig primitives
  (`keys add --multisig --nosort`; the `sign×K → multisign → broadcast`
  ceremony; the 1-ulume pubkey self-send) *and* migration-specific proof logic.
  It is coupled to package globals (`*flagBin`, `*flagHome`, `*flagChainID`,
  `*flagGas`, `*flagGasPrices`, `*flagRPC`) and free helpers (`runTx`,
  `getAddress`, `queryAccountNumberAndSequence`, `waitForTxResult`, …).
- `pelletier/go-toml/v2` and `golang.org/x/term` are already present (indirect)
  in `devnet/go.mod`. There is no TUI/prompt library in the `gen` module.

## 3. Design

### 3.1 Config file + chain selection

New file `configfile.go` in the gen-activity package. Parse with
`github.com/pelletier/go-toml/v2` (promote from indirect to direct dependency).

Schema:

```toml
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

Any key valid in `[common]` may be overridden inside a `[chains.X]` section.
TOML keys mirror the existing CLI flag names (kebab-case) so config and flags map
one-to-one.

New flags:
- `-config <path>` — config file path. Default `gen-activity-config.toml` in the
  current working directory. A missing file is **not** an error: the tool then
  behaves exactly as today (pure flag-driven).
- `-chain <name>` — selects a `[chains.<name>]` section.

**Precedence** (lowest to highest):

1. Built-in flag defaults (as registered in `configureFlags`).
2. `[common]` section.
3. `[chains.<name>]` section (when `-chain` is set).
4. Explicitly-set CLI flags.

Implementation: after `flag.Parse()`, collect the set of explicitly-set flag
names with `FlagSet.Visit`. Build the effective `Config` by starting from the
parsed flags (which already hold defaults), then for every field whose flag was
**not** explicitly set, overlay the config value if the config provides one
(chain section overriding common section). Fields whose flag *was* set on the
command line are left untouched. This is the linchpin that lets config files
populate values without clobbering explicit CLI overrides, working around the
fact that Go's `flag` cannot distinguish a default from an explicit value.

`Config.Validate()` is unchanged and runs after layering.

### 3.2 Mode selection + wizard

**Mode rule** (in `main`/`parseFlags`):

- `flag.NFlag() == 0` (no flags set) → **wizard mode** (new default).
- Any flag set → **command-line mode** (current behavior; backward compatible).
- `-w` / `-wizard` → force wizard even when other flags are present; those flags
  pre-seed the wizard's defaults (e.g. `gen-activity -w -chain testnet`).

New file `wizard.go` using `github.com/AlecAivazis/survey/v2`.

A `prompter` interface decouples the wizard's control flow from survey so the
menu state machine is unit-testable with scripted answers:

```go
type prompter interface {
    SelectOne(label string, options []string, def string) (string, error)
    Input(label, def string) (string, error)
    Confirm(label string, def bool) (bool, error)
}
```

The production implementation calls survey; tests inject a scripted fake.

Flow:

1. **Chain picker.** If a config file with `[chains.*]` exists, `SelectOne` over
   the chain names. The chosen chain's effective config becomes the starting
   defaults. If no config file exists, present a single "manual" entry that
   prompts for rpc / grpc / chain-id by hand and offers to save them as a chain
   section in a new config file.
2. **Settings menu.** `SelectOne` over editable settings, each rendered with its
   current value, plus `▶ Run now` and `Quit` entries. Selecting a setting
   re-prompts (`Input` / `Confirm` / `SelectOne` as appropriate) and returns to
   the menu. Editable settings: funding-key, mode (fresh / add-accounts /
   activity-existing), num-accounts, num-multisig23-accounts,
   num-multisig35-accounts, accounts path, parallelism, actions,
   funding-batch-size, max-account-amount, dry-run.
3. **Run.** `▶ Run now` validates (`Config.Validate`) and calls the same
   `run(cfg)` entry point as command-line mode. `Quit` exits without changes.

The wizard only assembles a `Config`; all execution stays in the existing
`run(cfg)` path, so wizard and CLI modes share one code path.

### 3.3 Multisig extraction → `common`

New file `devnet/tests/common/multisig.go` (+ `multisig_test.go`) holding the
generic ceremony, built on `ChainCLI`. To stay unit-testable without a live
chain, the helpers depend on a minimal runner interface that `ChainCLI`
satisfies (exposing the raw `Run`, tx submission, account number/sequence,
tx-inclusion wait, key existence/address lookup, and the bin/home/chain-id/
gas/gas-prices/keyring-backend it needs to build args). Exact interface shape is
finalized during implementation via TDD; it must preserve the current CLI
semantics: `--nosort` on `keys add --multisig`, `--multisig <addr>` +
`--sign-mode amino-json` on `tx sign`, and `tx multisign` consuming the first K
signature files.

Exported helpers (names indicative):

- `CreateMultisigKey(name string, members []string, threshold int) (addr string, err error)`
  — rerun-safe `keys add --multisig --nosort`; reuses an existing composite.
- `EnsureMultisigMembers(names []string, style KeyStyle) error`
  — key-style-aware member key generation via the existing `AddKeyWithStyle`.
- `RegisterMultisigPubKey(name, addr string, members []string, threshold int) error`
  — the 1-ulume self-send ceremony that publishes the composite pubkey on-chain.
- `BuildUnsignedTx(...)` — generate-only unsigned-tx helper.
- `SignAndBroadcastMultisig(unsignedFile, name, addr string, members []string, threshold int) (txHash string, err error)`
  — `sign×K → multisign → broadcast`, waits for inclusion.

**evmigration refactor.** `ensureMultisigCompositeKey`, `ensureMultisigMembers`,
`registerMultisigPubKey`, `signAndBroadcastMultisigTx`,
`buildUnsignedMultisigBankSendTx`, and `buildUnsignedMultisigDelegateTx` become
thin adapters that construct a `ChainCLI` from the existing `*flag*` globals and
delegate to the common helpers. The proof flow
(`runFourStepMigrationMultisig`, `runSignProofBoth`), the permanent-locked
vesting fixture, and validator discovery stay in `evmigration`. The standalone
`multisig` / `multisig-vesting` / `multisig-validator` modes keep working
unchanged. Existing evmigration unit tests must stay green; the live multisig
modes remain integration/devnet-exercised.

### 3.4 Multisig accounts in gen-activity

New flags / config keys: `-num-multisig23-accounts` (2-of-3),
`-num-multisig35-accounts` (3-of-5), default 0 each.

**Registry schema v2.** `AccountRecord` gains an optional field:

```go
type MultisigInfo struct {
    MemberNames []string `json:"member_names"`
    Threshold   int      `json:"threshold"`
    Signers     int      `json:"signers"`
}
// AccountRecord gains: Multisig *MultisigInfo `json:"multisig,omitempty"`
```

`schemaVersion` bumps to 2. v1 files still load: a record with a nil `Multisig`
is a regular account, so v1 and v2 files are forward/backward compatible without
a migration step. The loader accepts schema_version 1 or 2.

**Naming.** Composite keys: `<prefix>-msig23-NNNN` / `<prefix>-msig35-NNNN`;
member keys `<composite>-signer-<i>`. `AllocateNames` is generalized to allocate
per-kind names continuing past the highest existing index for that prefix.

**Flow integration in `run()`:**

1. Generate composites: create member keys (`EnsureMultisigMembers` with the
   detected key style) and the composite (`CreateMultisigKey`). Upsert into the
   registry and `Save` before funding (interrupt-safe, matching the existing
   key-then-save ordering).
2. Fund composites from the funder, alongside regular accounts (they are
   `unfundedTargets`).
3. `RegisterMultisigPubKey` for each funded composite so it can sign.
4. **Exercise:** one multisign bank-send to a peer address
   (`sign×K → multisign → broadcast`), recorded in the account's activity log.
   Each composite's ceremony runs sequentially (single account sequence);
   composites participate in the existing worker pool at the configured
   parallelism.

Multisig accounts honor `-add-accounts` and `-activity-existing` the same way
regular accounts do. Multisig composites are funded from the funder (not by
transfer from a generated account), and the exercise is a single multisign
bank-send (not the full delegate/grant suite) to bound ceremony cost.

## 4. Testing (TDD)

Unit tests (no live chain, no TTY):

- Config layering: precedence order and the `flag.Visit` "explicit beats config"
  rule; TOML parse of `[common]` + `[chains.*]`; missing-file ⇒ pure-flag path.
- Wizard: menu state machine driven by a scripted `prompter` fake — chain
  selection, editing each setting type, Run vs Quit.
- Common multisig helpers: argument construction and ceremony orchestration
  against a fake runner (assert `--nosort`, `--multisig`, `amino-json`, first-K
  signature files).
- Registry: schema-v2 round-trip and v1 backward-compatible load; `MultisigInfo`
  serialization; multisig name allocation continuing past existing indices.
- evmigration: existing unit tests stay green after the refactor.

Live/devnet exercise (manual or existing harness): a bare-config run that
creates, funds, and exercises 2-of-3 and 3-of-5 accounts; the evmigration
`multisig` modes still pass end-to-end.

## 5. Dependencies, docs, build

- `devnet/go.mod`: promote `github.com/pelletier/go-toml/v2` to a direct
  dependency; add `github.com/AlecAivazis/survey/v2`; run `go mod tidy` in the
  `devnet` module.
- Commit a `gen-activity-config.toml.example` documenting the schema.
- Update `docs/design/gen-activity-design.md` to describe config, wizard, and
  multisig accounts.
- `make lint` must pass cleanly (0 issues) after all changes.

## 6. File-change summary

New:
- `devnet/tests/gen-activity/configfile.go` (+ `_test.go`)
- `devnet/tests/gen-activity/wizard.go` (+ `_test.go`)
- `devnet/tests/common/multisig.go` (+ `_test.go`)
- `gen-activity-config.toml.example`

Modified:
- `devnet/tests/gen-activity/config.go` — new fields (`Config`-side of config
  file, chain, multisig counts), validation for multisig counts.
- `devnet/tests/gen-activity/main.go` — flag registration, mode selection,
  config layering, multisig generation/funding/exercise in `run()`.
- `devnet/tests/gen-activity/registry.go` — schema v2, `MultisigInfo`,
  name allocation.
- `devnet/tests/gen-activity/chain.go` / `funder.go` — multisig funding +
  exercise plumbing as needed.
- `devnet/tests/evmigration/multisig.go` — delegate generic ceremony to common.
- `devnet/go.mod` / `devnet/go.sum` — deps.
- `docs/design/gen-activity-design.md` — documentation.
