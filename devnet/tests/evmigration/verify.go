// verify.go implements the "verify" mode, which scans all migrated legacy
// addresses and checks that no leftover state references remain across bank,
// staking, distribution, authz, feegrant, action, claim, and supernode modules.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// runVerify checks all migrated legacy addresses across every chain module via
// RPC queries to ensure no leftover state references remain.
func runVerify() {
	af := loadAccounts(*flagFile)

	var targets []verifyTarget
	for _, rec := range af.Accounts {
		if rec.IsLegacy && rec.Migrated && rec.Address != "" {
			targets = append(targets, verifyTarget{
				name:       rec.Name,
				legacyAddr: rec.Address,
				newAddr:    rec.NewAddress,
			})
		}
	}
	if len(targets) == 0 {
		log.Println("no migrated legacy addresses to verify")
		return
	}
	log.Printf("verifying %d migrated legacy addresses across all chain modules (except evmigration)", len(targets))

	var issues []issue
	addIssue := func(t verifyTarget, module, detail string) {
		issues = append(issues, issue{t.name, t.legacyAddr, module, detail})
	}

	for i, t := range targets {
		log.Printf("  [%d/%d] %s (%s)", i+1, len(targets), t.name, t.legacyAddr)

		// ── bank ──────────────────────────────────────────────────────
		if hasBalance, err := queryHasAnyBalance(t.legacyAddr); err == nil && hasBalance {
			bal, _ := queryBalance(t.legacyAddr)
			addIssue(t, "bank", fmt.Sprintf("still has balance: %d ulume", bal))
		}

		// ── staking: delegations ──────────────────────────────────────
		if n, err := queryDelegationCount(t.legacyAddr); err == nil && n > 0 {
			addIssue(t, "staking", fmt.Sprintf("still has %d delegation(s)", n))
		}

		// ── staking: unbonding delegations ────────────────────────────
		if n, err := queryUnbondingCount(t.legacyAddr); err == nil && n > 0 {
			addIssue(t, "staking", fmt.Sprintf("still has %d unbonding delegation(s)", n))
		}

		// ── staking: redelegations ────────────────────────────────────
		if n, err := verifyRedelegationCount(t.legacyAddr); n > 0 {
			addIssue(t, "staking", fmt.Sprintf("still has %d redelegation(s)", n))
		} else if err != nil {
			log.Printf("    WARN: redelegation query: %v", err)
		}

		// ── distribution: withdraw address still pointing to legacy ───
		if t.newAddr != "" {
			if wdAddr, err := queryWithdrawAddress(t.newAddr); err == nil && wdAddr == t.legacyAddr {
				addIssue(t, "distribution", fmt.Sprintf("new address withdraw-addr still points to legacy: %s", wdAddr))
			}
		}

		// ── distribution: rewards on legacy (would imply delegations) ─
		if rewards, err := verifyDistributionRewards(t.legacyAddr); err == nil && rewards {
			addIssue(t, "distribution", "legacy address still has pending rewards")
		}

		// ── authz: grants by legacy as granter ────────────────────────
		if n, err := verifyAuthzGrantsByGranter(t.legacyAddr); err == nil && n > 0 {
			addIssue(t, "authz", fmt.Sprintf("legacy address still has %d authz grant(s) as granter", n))
		}

		// ── authz: grants by legacy as grantee ────────────────────────
		if n, err := verifyAuthzGrantsByGrantee(t.legacyAddr); err == nil && n > 0 {
			addIssue(t, "authz", fmt.Sprintf("legacy address still has %d authz grant(s) as grantee", n))
		}

		// ── feegrant: allowances from legacy as granter ───────────────
		if n, err := verifyFeegrantsByGranter(t.legacyAddr); err == nil && n > 0 {
			addIssue(t, "feegrant", fmt.Sprintf("legacy address still has %d feegrant(s) as granter", n))
		}

		// ── feegrant: allowances to legacy as grantee ─────────────────
		if n, err := verifyFeegrantsByGrantee(t.legacyAddr); err == nil && n > 0 {
			addIssue(t, "feegrant", fmt.Sprintf("legacy address still has %d feegrant(s) as grantee", n))
		}

		// ── action: actions created by legacy ─────────────────────────
		if ids, err := queryActionsByCreator(t.legacyAddr); err == nil && len(ids) > 0 {
			addIssue(t, "action", fmt.Sprintf("still owns %d action(s): %s",
				len(ids), strings.Join(ids, ", ")))
		}

		// ── action: actions referencing legacy as supernode ────────────
		if ids, err := queryActionsBySupernode(t.legacyAddr); err == nil && len(ids) > 0 {
			addIssue(t, "action", fmt.Sprintf("still referenced as supernode in %d action(s): %s",
				len(ids), strings.Join(ids, ", ")))
		}

		// ── claim: claim record pointing to legacy ────────────────────
		if claimed, destAddr, _, err := queryClaimRecord(t.legacyAddr); err == nil {
			if !claimed {
				addIssue(t, "claim", "unclaimed claim record still exists for legacy address")
			} else if destAddr == t.legacyAddr {
				addIssue(t, "claim", "claim record dest_address still points to legacy address")
			}
		}
		// claim query errors are expected (no record = good)

		// ── evmigration: migration record must exist ──────────────────
		hasMigRecord, recordNewAddr := queryMigrationRecord(t.legacyAddr)
		if !hasMigRecord {
			addIssue(t, "evmigration", "no migration record found")
		} else if t.newAddr != "" && recordNewAddr != t.newAddr {
			addIssue(t, "evmigration",
				fmt.Sprintf("migration record -> %s, expected %s", recordNewAddr, t.newAddr))
		}

		// ── evmigration: estimate should report already migrated ──────
		if est, err := queryMigrationEstimate(t.legacyAddr); err == nil {
			if est.RejectionReason != "already migrated" {
				addIssue(t, "evmigration",
					fmt.Sprintf("estimate rejection=%q, expected \"already migrated\"", est.RejectionReason))
			}
		}
	}

	// ── supernode: scan all supernodes for legacy address references ──
	log.Println("  scanning supernode records for legacy address references...")
	verifySupernodeRecords(targets, &issues)

	// ── JSON-RPC: verify EVM chain ID is correctly configured ──────────
	log.Println("  verifying JSON-RPC chain ID configuration...")
	verifyJSONRPCChainID(&issues)

	// Report results.
	log.Println("--- Verify Results ---")

	// Filter out evmigration issues (those are expected/allowed).
	var nonEvmIssues []issue
	for _, iss := range issues {
		if iss.module != "evmigration" {
			nonEvmIssues = append(nonEvmIssues, iss)
		} else {
			log.Printf("  [evmigration] %s (%s): %s", iss.name, iss.addr, iss.detail)
		}
	}

	if len(nonEvmIssues) == 0 {
		log.Printf("PASS: all %d migrated legacy addresses are clean across all modules", len(targets))
		return
	}

	addrIssues := make(map[string][]issue)
	for _, iss := range nonEvmIssues {
		addrIssues[iss.addr] = append(addrIssues[iss.addr], iss)
	}

	log.Printf("FAIL: found %d issue(s) across %d address(es):", len(nonEvmIssues), len(addrIssues))
	for addr, ii := range addrIssues {
		log.Printf("  %s (%s):", addr, ii[0].name)
		for _, iss := range ii {
			log.Printf("    [%s] %s", iss.module, iss.detail)
		}
	}
	log.Fatalf("FAIL: %d legacy addresses have leftover state", len(addrIssues))
}

// ─── Query helpers specific to verify ────────────────────────────────────────

// verifyRedelegationCount queries redelegations for addr by iterating all
// validator pairs. SDK v0.53+ only exposes "redelegation" (singular) which
// requires src-validator-addr, so we enumerate all validators.
func verifyRedelegationCount(addr string) (int, error) {
	validators, err := getValidators()
	if err != nil {
		return 0, fmt.Errorf("list validators for redelegation check: %w", err)
	}
	return queryAnyRedelegationCount(addr, validators)
}

// verifyDistributionRewards returns true if the address has pending distribution rewards.
func verifyDistributionRewards(addr string) (bool, error) {
	out, err := run("query", "distribution", "rewards", addr)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no delegation") {
			return false, nil
		}
		return false, err
	}
	var resp struct {
		Rewards []json.RawMessage `json:"rewards"`
		Total   []json.RawMessage `json:"total"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, err
	}
	return len(resp.Rewards) > 0, nil
}

// verifyAuthzGrantsByGranter returns the number of authz grants where addr is the granter.
func verifyAuthzGrantsByGranter(addr string) (int, error) {
	out, err := run("query", "authz", "grants-by-granter", addr)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no authorization") {
			return 0, nil
		}
		return 0, err
	}
	var resp struct {
		Grants []json.RawMessage `json:"grants"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.Grants), nil
}

// verifyAuthzGrantsByGrantee returns the number of authz grants where addr is the grantee.
func verifyAuthzGrantsByGrantee(addr string) (int, error) {
	out, err := run("query", "authz", "grants-by-grantee", addr)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no authorization") {
			return 0, nil
		}
		return 0, err
	}
	var resp struct {
		Grants []json.RawMessage `json:"grants"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.Grants), nil
}

// verifyFeegrantsByGranter returns the number of fee grants where addr is the granter.
func verifyFeegrantsByGranter(addr string) (int, error) {
	out, err := run("query", "feegrant", "grants-by-granter", addr)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no fee allowance") {
			return 0, nil
		}
		return 0, err
	}
	var resp struct {
		Allowances []json.RawMessage `json:"allowances"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.Allowances), nil
}

// verifyFeegrantsByGrantee returns the number of fee grants where addr is the grantee.
func verifyFeegrantsByGrantee(addr string) (int, error) {
	out, err := run("query", "feegrant", "grants-by-grantee", addr)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no fee allowance") {
			return 0, nil
		}
		return 0, err
	}
	var resp struct {
		Allowances []json.RawMessage `json:"allowances"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.Allowances), nil
}

// issue records a single verification failure for a migrated address.
type issue struct {
	name   string
	addr   string
	module string
	detail string
}

// verifySupernodeRecords lists all supernodes and checks if any field still
// references a legacy address from the migration set.
func verifySupernodeRecords(targets []verifyTarget, issues *[]issue) {
	legacySet := make(map[string]string, len(targets))
	for _, t := range targets {
		legacySet[t.legacyAddr] = t.name
	}

	out, err := run("query", "supernode", "list-supernodes")
	if err != nil {
		log.Printf("    WARN: list-supernodes: %v", err)
		return
	}

	var resp struct {
		Supernodes []json.RawMessage `json:"supernodes"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		log.Printf("    WARN: parse list-supernodes: %v", err)
		return
	}

	for _, raw := range resp.Supernodes {
		snJSON := string(raw)
		for legacyAddr, name := range legacySet {
			if strings.Contains(snJSON, legacyAddr) {
				// Decode to identify which field.
				var sn SuperNodeRecord
				_ = json.Unmarshal(raw, &sn)
				var fields []string
				if sn.SupernodeAccount == legacyAddr {
					fields = append(fields, "supernode_account")
				}
				for _, ev := range sn.Evidence {
					if ev.ReporterAddress == legacyAddr {
						fields = append(fields, "evidence.reporter_address")
						break
					}
				}
				// NOTE: prev_supernode_accounts legitimately contains legacy
				// addresses as historical records — skip flagging those.
				if len(fields) == 0 {
					// Only a prev_supernode_accounts match (or unknown) — not an issue.
					continue
				}
				*issues = append(*issues, issue{
					name:   name,
					addr:   legacyAddr,
					module: "supernode",
					detail: fmt.Sprintf("legacy addr found in supernode %s: %s",
						sn.ValidatorAddress, strings.Join(fields, ", ")),
				})
			}
		}
	}
}

type verifyTarget = struct {
	name       string
	legacyAddr string
	newAddr    string
}

// expectedEVMChainID is the Lumera EVM chain ID (config/evm.go).
const expectedEVMChainID uint64 = 76857769

// verifyJSONRPCChainID calls eth_chainId and net_version on the local
// JSON-RPC endpoint and verifies both return the expected Lumera EVM chain ID.
// A mismatch here means the app.toml config migration did not run or the
// [evm] section has the wrong evm-chain-id value (bug #19).
func verifyJSONRPCChainID(issues *[]issue) {
	const jsonRPCAddr = "http://localhost:8545"

	// eth_chainId — returns hex-encoded EIP-155 chain ID.
	ethChainID, err := jsonRPCCall(jsonRPCAddr, "eth_chainId")
	if err != nil {
		log.Printf("  WARN: eth_chainId query failed: %v", err)
		*issues = append(*issues, issue{
			name: "json-rpc", addr: "n/a", module: "evm",
			detail: fmt.Sprintf("eth_chainId query failed: %v", err),
		})
	} else {
		parsed, parseErr := strconv.ParseUint(strings.TrimPrefix(ethChainID, "0x"), 16, 64)
		if parseErr != nil {
			*issues = append(*issues, issue{
				name: "json-rpc", addr: "n/a", module: "evm",
				detail: fmt.Sprintf("eth_chainId returned unparseable value: %s", ethChainID),
			})
		} else if parsed != expectedEVMChainID {
			*issues = append(*issues, issue{
				name: "json-rpc", addr: "n/a", module: "evm",
				detail: fmt.Sprintf("eth_chainId mismatch: expected %d, got %d (0x%s)", expectedEVMChainID, parsed, ethChainID),
			})
		} else {
			log.Printf("  eth_chainId: %d (0x%x) ✓", parsed, parsed)
		}
	}

	// net_version — returns decimal string network ID (should match chain ID).
	netVersion, err := jsonRPCCall(jsonRPCAddr, "net_version")
	if err != nil {
		log.Printf("  WARN: net_version query failed: %v", err)
		*issues = append(*issues, issue{
			name: "json-rpc", addr: "n/a", module: "evm",
			detail: fmt.Sprintf("net_version query failed: %v", err),
		})
	} else {
		parsed, parseErr := strconv.ParseUint(netVersion, 10, 64)
		if parseErr != nil {
			*issues = append(*issues, issue{
				name: "json-rpc", addr: "n/a", module: "evm",
				detail: fmt.Sprintf("net_version returned unparseable value: %s", netVersion),
			})
		} else if parsed != expectedEVMChainID {
			*issues = append(*issues, issue{
				name: "json-rpc", addr: "n/a", module: "evm",
				detail: fmt.Sprintf("net_version mismatch: expected %d, got %d", expectedEVMChainID, parsed),
			})
		} else {
			log.Printf("  net_version: %d ✓", parsed)
		}
	}
}

// jsonRPCCall performs a single JSON-RPC 2.0 call with no params and returns
// the result as a raw string (stripped of surrounding quotes).
func jsonRPCCall(addr, method string) (string, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":[],"id":1}`, method)
	resp, err := http.Post(addr, "application/json", bytes.NewBufferString(payload)) //nolint:gosec // local devnet only
	if err != nil {
		return "", fmt.Errorf("HTTP POST: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w (body: %s)", err, truncate(string(body), 200))
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Strip surrounding quotes from string results.
	result := strings.Trim(string(rpcResp.Result), `"`)
	return result, nil
}
