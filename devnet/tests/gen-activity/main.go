// Command tests-gen-activity generates realistic account activity against a
// live Lumera devnet chain. It creates and reuses test accounts, funds them
// from a local keyring funder, submits activity transactions, and persists all
// generated metadata in a rerunnable JSON registry.
//
// See docs/design/gen-activity-design.md for the full design.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gen/tests/common"
)

const defaultEVMCutoverVer = "v1.20.0"

// vestingGasTopUp is the small liquid balance sent to each vesting/locked
// account so it can pay transaction fees (locked balances cannot cover gas).
const vestingGasTopUp = "1000000ulume"

const usageDescription = "tests-gen-activity generates realistic account activity against a live Lumera devnet chain."

func main() {
	cfg := &Config{}
	configureFlags(flag.CommandLine, cfg)
	flag.Parse()

	wizard, setFlags, err := resolveConfig(cfg, flag.CommandLine)
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}
	logRuntimeBinary()

	if wizard {
		fc, _ := LoadFileConfig(cfg.ConfigPath)
		if err := runWizard(cfg, fc, setFlags, newSurveyPrompter(), run); err != nil {
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

// resolveConfig loads any config file referenced by cfg.ConfigPath, layers it
// onto cfg honoring CLI-flag precedence, and reports whether the tool should run
// in wizard mode (no flags passed, or -w/-wizard). fs must be the FlagSet that
// already parsed the command line (so fs.Visit/fs.NFlag reflect what the user
// set).
// The returned setFlags map records which CLI flags the user set explicitly; the
// wizard reuses it so a later chain re-seed cannot clobber those overrides.
func resolveConfig(cfg *Config, fs *flag.FlagSet) (wizard bool, setFlags map[string]bool, err error) {
	setFlags = map[string]bool{}
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	// When -config is not passed, prefer a config next to the executable so the
	// binary works from any cwd (e.g. /shared/release/tests-gen-activity invoked
	// with cwd=/root). Only the default relative path is rewritten; an explicit
	// -config is honored as-is.
	if !setFlags["config"] {
		cfg.ConfigPath = resolveConfigPath(cfg.ConfigPath, false, executableDir())
	}

	fc, err := LoadFileConfig(cfg.ConfigPath)
	if err != nil {
		return false, setFlags, err
	}
	if fc == nil && setFlags["config"] {
		return false, setFlags, fmt.Errorf("config file %q not found", cfg.ConfigPath)
	}
	// Make a missing default config visible rather than silently falling back to
	// flag defaults (a common source of "it ignored my chain-id" confusion).
	if fc == nil {
		log.Printf("no config file found at %q; using flag defaults only", cfg.ConfigPath)
	} else {
		log.Printf("loaded config from %s", cfg.ConfigPath)
	}
	if err := ApplyFileConfig(cfg, fc, cfg.Chain, setFlags); err != nil {
		return false, setFlags, err
	}

	wizard = cfg.Wizard || fs.NFlag() == 0
	return wizard, setFlags, nil
}

// executableDir returns the directory containing the running binary, or "" if it
// cannot be determined.
func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

func logRuntimeBinary() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("runtime binary: unknown (%v)", err)
		return
	}
	hash, err := fileSHA256(exe)
	if err != nil {
		log.Printf("runtime binary: %s (hash unavailable: %v)", exe, err)
		return
	}
	log.Printf("runtime binary: %s sha256=%s", exe, hash)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// resolveConfigPath picks the config path to load when -config was not passed
// explicitly. It prefers a file named like configPath sitting next to the
// executable (exeDir) so the tool finds its config regardless of the current
// working directory; otherwise it returns configPath unchanged (resolved against
// the cwd by the loader). An explicit -config path is returned as-is.
func resolveConfigPath(configPath string, explicit bool, exeDir string) string {
	if explicit || configPath == "" {
		return configPath
	}
	if exeDir != "" {
		alt := filepath.Join(exeDir, filepath.Base(configPath))
		if _, err := os.Stat(alt); err == nil {
			return alt
		}
	}
	return configPath
}

func configureFlags(fs *flag.FlagSet, c *Config) {
	fs.Usage = func() {
		out := fs.Output()
		_, _ = fmt.Fprintf(out, "%s\n\n", usageDescription)
		_, _ = fmt.Fprintf(out, "Usage: %s [flags]\n\n", fs.Name())
		_, _ = fmt.Fprintln(out, "Flags:")
		fs.PrintDefaults()
	}

	fs.StringVar(&c.Bin, "bin", "lumerad", "lumerad binary path")
	fs.StringVar(&c.ConfigPath, "config", "gen-activity-config.toml", "path to gen-activity TOML config file (optional)")
	fs.StringVar(&c.Chain, "chain", "", "named chain section from the config file (e.g. devnet, testnet, mainnet)")
	fs.BoolVar(&c.Wizard, "wizard", false, "run the interactive wizard (also the default when no flags are passed)")
	fs.BoolVar(&c.Wizard, "w", false, "shorthand for -wizard")
	fs.StringVar(&c.RPC, "rpc", "tcp://localhost:26657", "CometBFT RPC endpoint")
	fs.StringVar(&c.GRPC, "grpc", "localhost:9090", "gRPC endpoint")
	fs.StringVar(&c.ChainID, "chain-id", "", "chain ID (required)")
	fs.StringVar(&c.Home, "home", "", "lumerad home directory")
	fs.StringVar(&c.KeyringBackend, "keyring-backend", "test", "local funder keyring backend")
	fs.StringVar(&c.EVMCutoverVer, "evm-cutover-version", defaultEVMCutoverVer, "lumerad version where accounts switch to coin-type 60")
	fs.StringVar(&c.FundingKey, "funding-key", "governance_key", "funder key name in the local keyring")
	fs.StringVar(&c.AccountsPath, "accounts", "devnet/tests/gen-activity/accounts.json", "registry file path")
	fs.IntVar(&c.NumAccounts, "num-accounts", 10, "number of accounts to generate")
	fs.IntVar(&c.NumMultisig23, "num-multisig23-accounts", 0, "number of 2-of-3 multisig accounts to generate")
	fs.IntVar(&c.NumMultisig35, "num-multisig35-accounts", 0, "number of 3-of-5 multisig accounts to generate")
	fs.IntVar(&c.VestingPercent, "vesting-percent", 0, "percent of regular accounts to create as continuous/delayed vesting (0-100)")
	fs.IntVar(&c.NumPermanentLocked, "num-permanent-locked-accounts", 0, "number of dedicated PermanentLocked accounts to generate")
	fs.StringVar(&c.MaxAccountAmount, "max-account-amount", "10000000ulume", "upper bound for per-account funding")
	fs.StringVar(&c.AccountPrefix, "account-prefix", "gen", "name prefix for generated accounts")
	fs.StringVar(&c.Mode, "mode", "", "run mode: fresh|add-accounts|activity-existing|migrate (default fresh; -add-accounts/-activity-existing are shorthands)")
	fs.BoolVar(&c.AddAccounts, "add-accounts", false, "add -num-accounts new users to an existing registry")
	fs.BoolVar(&c.ActivityExisting, "activity-existing", false, "generate more activity for existing accounts")
	fs.BoolVar(&c.Actions, "actions", true, "include CASCADE action activity")
	fs.BoolVar(&c.RequireActions, "require-actions", false, "fail the run if action activity cannot be created")
	fs.IntVar(&c.MaxActionsPerRun, "max-actions-per-run", 3, "cap action uploads/registrations per run")
	fs.StringVar(&c.ActionStates, "action-states", "pending,done,approved", "target action states to generate")
	fs.DurationVar(&c.ActionReadinessTimeout, "action-readiness-timeout", 180*time.Second, "time to wait for usable active supernodes")
	fs.IntVar(&c.FundingBatchSize, "funding-batch-size", 1, "funder transfers to pipeline before waiting for inclusion")
	fs.IntVar(&c.Parallelism, "parallelism", 5, "maximum concurrent per-account activity workers")
	fs.BoolVar(&c.DryRun, "dry-run", true, "print planned accounts/activity without submitting txs")
}

// run executes the runtime flow described in the design. Steps 1-6 are
// read-only; -dry-run stops after printing the plan and never mutates the
// keyring or registry.
func run(cfg *Config) error {
	// Migrate mode is a distinct flow: it migrates existing registry accounts to
	// their EVM-compatible counterparts instead of generating/funding accounts.
	if cfg.resolvedMode == ModeMigrate {
		return runMigrateMode(cfg)
	}

	// Step 2: detect key style from the current lumerad runtime.
	log.Printf("phase: detect chain key style")
	keyStyle := detectKeyStyle(cfg.Bin, cfg.EVMCutoverVer)
	log.Printf("key style: %s (algo=%s coin-type=%d)", keyStyle.Name(), keyStyle.Algo, keyStyle.CoinType)

	cli := newChainCLI(cfg)

	// Step 5: load the registry if present, else start a new one. Load this
	// before validator discovery so reruns can fall back to the last known set
	// if the staking CLI query is unavailable.
	log.Printf("phase: load registry")
	now := time.Now().UTC().Format(time.RFC3339)
	reg, err := loadOrCreateRegistry(cfg, keyStyle, now)
	if err != nil {
		return err
	}

	// Steps 3-4: query validators and resolve the funder address. These are
	// read-only; in dry-run they are best-effort so planning works without a
	// node, but a live run requires both.
	log.Printf("phase: query validators and funder")
	validators := queryValidators(cli, cfg.DryRun, reg.Validators)
	log.Printf("validators: %d discovered", len(validators))
	funderAddr := resolveFunder(cli, cfg.FundingKey, cfg.DryRun, reg.FunderAddress)
	if funderAddr != "" {
		log.Printf("funder: %s -> %s", cfg.FundingKey, funderAddr)
	}

	// Step 6: reconcile envelope metadata with the current run. An existing
	// account's recorded key style is never rewritten.
	reconcile(reg, cfg, keyStyle)
	if len(validators) > 0 {
		reg.Validators = validators
	}
	if funderAddr != "" {
		reg.FunderAddress = funderAddr
	}

	// Decide how many new accounts to allocate this run.
	newCount := plannedNewAccountCount(cfg, reg)
	plannedNames := reg.AllocateNames(cfg.AccountPrefix, newCount)
	log.Printf("plan: mode=%s existing=%d new-regular=%d multisig23=%d multisig35=%d permanent-locked=%d actions=%v dry-run=%v",
		cfg.resolvedMode, len(reg.Accounts), len(plannedNames), cfg.NumMultisig23, cfg.NumMultisig35, cfg.NumPermanentLocked, cfg.Actions, cfg.DryRun)

	if cfg.DryRun {
		printPlan(cfg, reg, plannedNames)
		return nil
	}

	// Beyond this point the run mutates the keyring, chain, and registry, so the
	// read-only preconditions must hold.
	if len(validators) == 0 {
		return fmt.Errorf("no validators found on chain %s", cfg.ChainID)
	}
	if funderAddr == "" {
		return fmt.Errorf("could not resolve funder key %q address", cfg.FundingKey)
	}

	// Step 7-8: generate keys for the planned accounts and persist before any
	// funding so an interrupted run can resume.
	log.Printf("phase: create/reuse regular account keys")
	newRecs := generateAccounts(cli, plannedNames, keyStyle)
	for _, rec := range newRecs {
		reg.UpsertAccount(rec)
	}
	if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save registry after key generation: %w", err)
	}
	log.Printf("generated %d new account(s); registry saved", len(newRecs))

	// Generate multisig accounts (2-of-3 / 3-of-5) alongside regular accounts.
	var newMultisig []*AccountRecord
	if specs := multisigPlan(cfg.NumMultisig23, cfg.NumMultisig35); len(specs) > 0 && !cfg.ActivityExisting {
		log.Printf("phase: create multisig account keys")
		newMultisig = generateMultisigAccounts(cli, reg, cfg.AccountPrefix, specs, keyStyle)
		if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return fmt.Errorf("save registry after multisig key generation: %w", err)
		}
		log.Printf("generated %d new multisig account(s)", len(newMultisig))
	}

	// Designate a random share of new regular accounts as vesting, and generate
	// dedicated permanent-locked accounts. Tagged BEFORE funding so the funding
	// split routes them to the vesting funder (vesting/locked accounts must be
	// created at funding time).
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
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

	// Step 9: fund unfunded accounts via the single-funder batcher.
	chain := &cliFundingChain{cli: cli, funderKey: cfg.FundingKey, funderAddr: funderAddr, blockWait: 30 * time.Second}
	bankTargets, vestingTargets := splitFundingTargets(reg)
	log.Printf("phase: fund accounts (bank-targets=%d vesting-targets=%d)", len(bankTargets), len(vestingTargets))
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

	// Step 11: persist after funding regardless of partial failure.
	if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save registry after funding: %w", err)
	}
	if fundErr != nil {
		return fmt.Errorf("funding phase: %w", fundErr)
	}

	// Register multisig pubkeys on-chain and exercise each with one multisign
	// bank-send to a peer. Non-fatal: a failure logs and continues.
	exerciseMultisigAccounts(cli, reg, newMultisig, cfg)
	if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save registry after multisig exercise: %w", err)
	}

	// Step 10: per-account activity mix. Only funded accounts can transact, and
	// they serve as each other's peers for transfers and grants.
	log.Printf("phase: generate account activity")
	generateActivity(cli, reg, newRecs, validators, cfg, rng)
	reconcileReceivedGrants(reg.Accounts)
	if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save registry after activity: %w", err)
	}

	// Step: CASCADE action generation via the sdk-go client. Non-fatal by
	// default; -require-actions makes supernode unavailability or creation
	// failures fatal.
	if cfg.Actions {
		log.Printf("phase: generate CASCADE actions")
		actionAccts := activityTargets(reg, newRecs, cfg.ActivityExisting)
		creator := newSDKActionCreator(cfg, cli)
		if err := generateActions(creator, cli, actionAccts, validators, cfg.actionStates,
			cfg.MaxActionsPerRun, cfg.RequireActions, cfg.ActionReadinessTimeout); err != nil {
			// Persist whatever actions were recorded before failing.
			_ = reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339))
			return fmt.Errorf("action generation: %w", err)
		}
		if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return fmt.Errorf("save registry after actions: %w", err)
		}
	}

	log.Printf("gen-activity run complete")
	return nil
}

// generateActivity plans and runs the per-account activity mix across all
// funded accounts, using them as each other's peers.
func generateActivity(cli *common.ChainCLI, reg *ActivityRegistry, newRecs []*AccountRecord, validators []string, cfg *Config, rng *rand.Rand) {
	active := activityTargets(reg, newRecs, cfg.ActivityExisting)
	if len(active) == 0 {
		log.Printf("no funded accounts; skipping activity generation")
		return
	}

	// Activity amounts are sized well below per-account funding.
	unit := cfg.maxAmount.Amount / 100
	if unit < 1 {
		unit = 1
	}

	plans := buildActivityPlans(active, validators, rng, unit)
	chain := &cliActivityChain{cli: cli}
	planFor := func(rec *AccountRecord) []plannedActivity {
		return plans[rec]
	}
	log.Printf("generating activity for %d account(s) with parallelism %d", len(active), cfg.Parallelism)
	runActivityWorkers(chain, active, planFor, cfg.Parallelism)
}

// peersExcluding returns all addresses except the given one.
func peersExcluding(addrs []string, self string) []string {
	peers := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a != self {
			peers = append(peers, a)
		}
	}
	return peers
}

// newChainCLI builds a common.ChainCLI from the configuration.
func newChainCLI(cfg *Config) *common.ChainCLI {
	return &common.ChainCLI{
		Bin:            cfg.Bin,
		ChainID:        cfg.ChainID,
		RPC:            cfg.RPC,
		Home:           cfg.Home,
		KeyringBackend: cfg.KeyringBackend,
		Gas:            "500000",
		GasPrices:      "0.025" + common.ChainDenom,
		GasAdjustment:  "1.4",
	}
}

// queryValidators returns the validator set. If the live CLI query fails, a
// previously persisted registry validator set is good enough for reruns.
// Dry-run treats total failure as a warning; a live run treats an empty set as
// fatal at the call site.
func queryValidators(cli *common.ChainCLI, dryRun bool, fallback []string) []string {
	vals, err := cli.Validators()
	if err != nil {
		if len(fallback) > 0 {
			log.Printf("WARN: validator query failed, using %d validator(s) from registry: %v", len(fallback), err)
			return append([]string(nil), fallback...)
		}
		if dryRun {
			log.Printf("WARN: validator query failed (dry-run, continuing): %v", err)
			return nil
		}
		log.Printf("validator query failed: %v", err)
		return nil
	}
	return vals
}

// resolveFunder resolves the funder key's address, best-effort in dry-run. On
// reruns, a registry funder address is a useful fallback when the local keyring
// command is temporarily unavailable.
func resolveFunder(cli *common.ChainCLI, key string, dryRun bool, fallback string) string {
	addr, err := cli.ShowAddress(key)
	if err != nil {
		if fallback != "" {
			log.Printf("WARN: funder address lookup failed, using registry address %s: %v", fallback, err)
			return fallback
		}
		if dryRun {
			log.Printf("WARN: funder address lookup failed (dry-run, continuing): %v", err)
		} else {
			log.Printf("funder address lookup failed: %v", err)
		}
		return ""
	}
	return addr
}

// detectKeyStyle probes `lumerad version` and maps it to a KeyStyle. On failure
// it falls back to the legacy style with a warning; legacy keys remain usable
// on EVM-era chains, while EVM keys are unusable on pre-EVM chains.
func detectKeyStyle(bin, cutover string) common.KeyStyle {
	version, err := detectLumeradVersion(bin)
	if err != nil {
		log.Printf("WARN: detect lumerad version failed (%v); assuming legacy key style", err)
		return common.KeyStyleLegacy
	}
	ks, err := common.KeyStyleForVersion(version, cutover)
	if err != nil {
		log.Printf("WARN: classify version %q failed (%v); assuming legacy key style", version, err)
		return common.KeyStyleLegacy
	}
	log.Printf("detected lumerad %s", version)
	return ks
}

// detectLumeradVersion runs the binary's version command and extracts a semver.
func detectLumeradVersion(bin string) (string, error) {
	for _, args := range [][]string{{"version"}, {"version", "--long"}} {
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout())
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Env = envWithoutDesktopBus()
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			continue
		}
		if ver, ok := common.ExtractSemver(string(out)); ok {
			return ver, nil
		}
	}
	return "", fmt.Errorf("could not determine %s version", bin)
}

func commandTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("LUMERA_CLI_TIMEOUT"))
	if raw == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 30 * time.Second
	}
	return d
}

func envWithoutDesktopBus() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "DBUS_SESSION_BUS_ADDRESS=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// loadOrCreateRegistry loads an existing registry or creates a fresh one. A
// missing file creates a new registry; an unparseable file is a hard error.
func loadOrCreateRegistry(cfg *Config, keyStyle common.KeyStyle, now string) (*ActivityRegistry, error) {
	reg, err := LoadRegistry(cfg.AccountsPath)
	switch {
	case err == nil:
		log.Printf("loaded registry %s (%d accounts)", cfg.AccountsPath, len(reg.Accounts))
		return reg, nil
	case os.IsNotExist(err):
		log.Printf("no registry at %s; creating a new one", cfg.AccountsPath)
		return NewRegistry(cfg.ChainID, cfg.FundingKey, "", keyStyle.Name(), now), nil
	default:
		return nil, fmt.Errorf("load registry: %w", err)
	}
}

// reconcile updates envelope metadata for the current run without rewriting any
// existing account's recorded key style.
func reconcile(reg *ActivityRegistry, cfg *Config, keyStyle common.KeyStyle) {
	if reg.ChainID != "" && reg.ChainID != cfg.ChainID {
		log.Printf("WARN: registry chain-id %q differs from -chain-id %q", reg.ChainID, cfg.ChainID)
	}
	reg.ChainID = cfg.ChainID
	reg.FunderKey = cfg.FundingKey
	reg.KeyStyle = keyStyle.Name()
}

// plannedNewAccountCount determines how many new regular accounts to allocate.
// On a fresh registry it fills up to -num-accounts; -add-accounts always adds
// -num-accounts more; -activity-existing alone adds none. Dedicated multisig
// composites and permanent-locked fixtures have their own knobs, so they do not
// satisfy the regular account target.
func plannedNewAccountCount(cfg *Config, reg *ActivityRegistry) int {
	if cfg.AddAccounts {
		return cfg.NumAccounts
	}
	if cfg.ActivityExisting {
		return 0
	}
	if deficit := cfg.NumAccounts - regularAccountCount(reg); deficit > 0 {
		return deficit
	}
	return 0
}

func regularAccountCount(reg *ActivityRegistry) int {
	count := 0
	for _, rec := range reg.Accounts {
		if rec.Multisig != nil {
			continue
		}
		if rec.Vesting != nil && rec.Vesting.Type == string(common.VestingPermanentLocked) {
			continue
		}
		count++
	}
	return count
}

func printPlan(cfg *Config, reg *ActivityRegistry, plannedNames []string) {
	fmt.Printf("DRY RUN — no keyring or registry changes will be made\n")
	fmt.Printf("  chain-id:        %s\n", cfg.ChainID)
	fmt.Printf("  registry:        %s\n", cfg.AccountsPath)
	fmt.Printf("  existing accts:  %d\n", len(reg.Accounts))
	fmt.Printf("  new accts:       %d\n", len(plannedNames))
	for _, n := range plannedNames {
		fmt.Printf("    + %s\n", n)
	}
	fmt.Printf("  max funding:     %s\n", cfg.MaxAccountAmount)
	fmt.Printf("  actions:         enabled=%t states=%s max/run=%d\n", cfg.Actions, cfg.ActionStates, cfg.MaxActionsPerRun)
	fmt.Printf("  funding batch:   %d\n", cfg.FundingBatchSize)
	fmt.Printf("  parallelism:     %d\n", cfg.Parallelism)
}
