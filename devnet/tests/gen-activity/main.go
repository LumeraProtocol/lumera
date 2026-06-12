// Command tests-gen-activity generates realistic account activity against a
// live Lumera devnet chain. It creates and reuses test accounts, funds them
// from a local keyring funder, submits activity transactions, and persists all
// generated metadata in a rerunnable JSON registry.
//
// See docs/design/gen-activity-design.md for the full design.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"gen/tests/common"
)

const defaultEVMCutoverVer = "v1.20.0"

const usageDescription = "tests-gen-activity generates realistic account activity against a live Lumera devnet chain."

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
	fs.StringVar(&c.FundingKey, "funding-key", "", "funder key name in the local keyring (required)")
	fs.StringVar(&c.AccountsPath, "accounts", "devnet/tests/gen-activity/accounts.json", "registry file path")
	fs.IntVar(&c.NumAccounts, "num-accounts", 10, "number of accounts to generate")
	fs.IntVar(&c.NumMultisig23, "num-multisig23-accounts", 0, "number of 2-of-3 multisig accounts to generate")
	fs.IntVar(&c.NumMultisig35, "num-multisig35-accounts", 0, "number of 3-of-5 multisig accounts to generate")
	fs.StringVar(&c.MaxAccountAmount, "max-account-amount", "10000000ulume", "upper bound for per-account funding")
	fs.StringVar(&c.AccountPrefix, "account-prefix", "gen", "name prefix for generated accounts")
	fs.BoolVar(&c.AddAccounts, "add-accounts", false, "add -num-accounts new users to an existing registry")
	fs.BoolVar(&c.ActivityExisting, "activity-existing", false, "generate more activity for existing accounts")
	fs.BoolVar(&c.Actions, "actions", true, "include CASCADE action activity")
	fs.BoolVar(&c.RequireActions, "require-actions", false, "fail the run if action activity cannot be created")
	fs.IntVar(&c.MaxActionsPerRun, "max-actions-per-run", 3, "cap action uploads/registrations per run")
	fs.StringVar(&c.ActionStates, "action-states", "pending,done,approved", "target action states to generate")
	fs.DurationVar(&c.ActionReadinessTimeout, "action-readiness-timeout", 180*time.Second, "time to wait for usable active supernodes")
	fs.IntVar(&c.FundingBatchSize, "funding-batch-size", 10, "funder transfers to pipeline before waiting for inclusion")
	fs.IntVar(&c.Parallelism, "parallelism", 5, "maximum concurrent per-account activity workers")
	fs.BoolVar(&c.DryRun, "dry-run", false, "print planned accounts/activity without submitting txs")
}

// run executes the runtime flow described in the design. Steps 1-6 are
// read-only; -dry-run stops after printing the plan and never mutates the
// keyring or registry.
func run(cfg *Config) error {
	// Step 2: detect key style from the current lumerad runtime.
	keyStyle := detectKeyStyle(cfg.Bin, cfg.EVMCutoverVer)
	log.Printf("key style: %s (algo=%s coin-type=%d)", keyStyle.Name(), keyStyle.Algo, keyStyle.CoinType)

	cli := newChainCLI(cfg)

	// Steps 3-4: query validators and resolve the funder address. These are
	// read-only; in dry-run they are best-effort so planning works without a
	// node, but a live run requires both.
	validators := queryValidators(cli, cfg.DryRun)
	funderAddr := resolveFunder(cli, cfg.FundingKey, cfg.DryRun)

	// Step 5: load the registry if present, else start a new one.
	now := time.Now().UTC().Format(time.RFC3339)
	reg, err := loadOrCreateRegistry(cfg, keyStyle, now)
	if err != nil {
		return err
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
	newRecs := generateAccounts(cli, plannedNames, keyStyle)
	for _, rec := range newRecs {
		reg.UpsertAccount(rec)
	}
	if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save registry after key generation: %w", err)
	}
	log.Printf("generated %d new account(s); registry saved", len(newRecs))

	// Step 9: fund unfunded accounts via the single-funder batcher.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	chain := &cliFundingChain{cli: cli, funderKey: cfg.FundingKey, funderAddr: funderAddr, blockWait: 30 * time.Second}
	targets := unfundedTargets(reg)
	amountFor := func(*AccountRecord) string {
		return common.Coin{Amount: randomFundingAmount(cfg.maxAmount.Amount, rng), Denom: common.ChainDenom}.String()
	}
	funded, fundErr := FundAccounts(chain, targets, amountFor, cfg.FundingBatchSize, 3)
	log.Printf("funded %d/%d account(s)", funded, len(targets))

	// Step 11: persist after funding regardless of partial failure.
	if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save registry after funding: %w", err)
	}
	if fundErr != nil {
		return fmt.Errorf("funding phase: %w", fundErr)
	}

	// Step 10: per-account activity mix. Only funded accounts can transact, and
	// they serve as each other's peers for transfers and grants.
	generateActivity(cli, reg, newRecs, validators, cfg, rng)
	reconcileReceivedGrants(reg.Accounts)
	if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save registry after activity: %w", err)
	}

	// Step: CASCADE action generation via the sdk-go client. Non-fatal by
	// default; -require-actions makes supernode unavailability or creation
	// failures fatal.
	if cfg.Actions {
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

// queryValidators returns the validator set. In dry-run a query failure is a
// warning; a live run treats an empty set as fatal at the call site.
func queryValidators(cli *common.ChainCLI, dryRun bool) []string {
	vals, err := cli.Validators()
	if err != nil {
		if dryRun {
			log.Printf("WARN: validator query failed (dry-run, continuing): %v", err)
			return nil
		}
		log.Printf("validator query failed: %v", err)
		return nil
	}
	return vals
}

// resolveFunder resolves the funder key's address, best-effort in dry-run.
func resolveFunder(cli *common.ChainCLI, key string, dryRun bool) string {
	addr, err := cli.ShowAddress(key)
	if err != nil {
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
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err != nil {
			continue
		}
		if ver, ok := common.ExtractSemver(string(out)); ok {
			return ver, nil
		}
	}
	return "", fmt.Errorf("could not determine %s version", bin)
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

// plannedNewAccountCount determines how many new accounts to allocate. On a
// fresh registry it fills up to -num-accounts; -add-accounts always adds
// -num-accounts more; -activity-existing alone adds none.
func plannedNewAccountCount(cfg *Config, reg *ActivityRegistry) int {
	if cfg.AddAccounts {
		return cfg.NumAccounts
	}
	if cfg.ActivityExisting {
		return 0
	}
	if deficit := cfg.NumAccounts - len(reg.Accounts); deficit > 0 {
		return deficit
	}
	return 0
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
