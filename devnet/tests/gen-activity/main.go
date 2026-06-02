// Command tests_gen_activity generates realistic account activity against a
// live Lumera devnet chain. It creates and reuses test accounts, funds them
// from a local keyring funder, submits activity transactions, and persists all
// generated metadata in a rerunnable JSON registry.
//
// See docs/design/gen-activity-design.md for the full design.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"gen/tests/common"
)

const defaultEVMCutoverVer = "v1.20.0"

func main() {
	cfg := parseFlags()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	if err := run(cfg); err != nil {
		log.Fatalf("gen-activity failed: %v", err)
	}
}

func parseFlags() *Config {
	c := &Config{}
	flag.StringVar(&c.Bin, "bin", "lumerad", "lumerad binary path")
	flag.StringVar(&c.RPC, "rpc", "tcp://localhost:26657", "CometBFT RPC endpoint")
	flag.StringVar(&c.GRPC, "grpc", "localhost:9090", "gRPC endpoint")
	flag.StringVar(&c.ChainID, "chain-id", "", "chain ID (required)")
	flag.StringVar(&c.Home, "home", "", "lumerad home directory")
	flag.StringVar(&c.KeyringBackend, "keyring-backend", "test", "local funder keyring backend")
	flag.StringVar(&c.EVMCutoverVer, "evm-cutover-version", defaultEVMCutoverVer, "lumerad version where accounts switch to coin-type 60")
	flag.StringVar(&c.FundingKey, "funding-key", "", "funder key name in the local keyring (required)")
	flag.StringVar(&c.AccountsPath, "accounts", "devnet/tests/gen-activity/accounts.json", "registry file path")
	flag.IntVar(&c.NumAccounts, "num-accounts", 10, "number of accounts to generate")
	flag.StringVar(&c.MaxAccountAmount, "max-account-amount", "10000000ulume", "upper bound for per-account funding")
	flag.StringVar(&c.AccountPrefix, "account-prefix", "gen", "name prefix for generated accounts")
	flag.BoolVar(&c.AddAccounts, "add-accounts", false, "add -num-accounts new users to an existing registry")
	flag.BoolVar(&c.ActivityExisting, "activity-existing", false, "generate more activity for existing accounts")
	flag.BoolVar(&c.Actions, "actions", true, "include CASCADE action activity")
	flag.BoolVar(&c.RequireActions, "require-actions", false, "fail the run if action activity cannot be created")
	flag.IntVar(&c.MaxActionsPerRun, "max-actions-per-run", 3, "cap action uploads/registrations per run")
	flag.StringVar(&c.ActionStates, "action-states", "pending,done,approved", "target action states to generate")
	flag.DurationVar(&c.ActionReadinessTimeout, "action-readiness-timeout", 180*time.Second, "time to wait for usable active supernodes")
	flag.IntVar(&c.FundingBatchSize, "funding-batch-size", 10, "funder transfers to pipeline before waiting for inclusion")
	flag.IntVar(&c.Parallelism, "parallelism", 5, "maximum concurrent per-account activity workers")
	flag.BoolVar(&c.DryRun, "dry-run", false, "print planned accounts/activity without submitting txs")
	flag.Parse()
	return c
}

// run executes the runtime flow described in the design. Steps 1-6 are
// read-only; -dry-run stops after printing the plan and never mutates the
// keyring or registry.
func run(cfg *Config) error {
	// Step 2: detect key style from the current lumerad runtime.
	keyStyle := detectKeyStyle(cfg.Bin, cfg.EVMCutoverVer)
	log.Printf("key style: %s (algo=%s coin-type=%d)", keyStyle.Name(), keyStyle.Algo, keyStyle.CoinType)

	// Step 5: load the registry if present, else start a new one.
	now := time.Now().UTC().Format(time.RFC3339)
	reg, err := loadOrCreateRegistry(cfg, keyStyle, now)
	if err != nil {
		return err
	}

	// Step 6: reconcile envelope metadata with the current run. An existing
	// account's recorded key style is never rewritten.
	reconcile(reg, cfg, keyStyle)

	// Decide how many new accounts to allocate this run.
	newCount := plannedNewAccountCount(cfg, reg)
	plannedNames := reg.AllocateNames(cfg.AccountPrefix, newCount)

	if cfg.DryRun {
		printPlan(cfg, reg, plannedNames)
		return nil
	}

	// Step 7: persist any planned accounts, then run funding and activity.
	// The live submission layer (funder batcher + per-account activity workers,
	// built on the evmigration chain primitives) is the next implementation
	// slice; until it lands, a non-dry-run is a hard error rather than a
	// silent no-op.
	return errors.New("live activity submission is not yet wired; re-run with -dry-run to preview the plan")
}

// detectKeyStyle probes `lumerad version` and maps it to a KeyStyle. On failure
// it falls back to the EVM style with a warning, matching the design's
// non-fatal version-probe behavior.
func detectKeyStyle(bin, cutover string) common.KeyStyle {
	version, err := detectLumeradVersion(bin)
	if err != nil {
		log.Printf("WARN: detect lumerad version failed (%v); assuming EVM key style", err)
		return common.KeyStyleEVM
	}
	ks, err := common.KeyStyleForVersion(version, cutover)
	if err != nil {
		log.Printf("WARN: classify version %q failed (%v); assuming EVM key style", version, err)
		return common.KeyStyleEVM
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
