// query_state.go provides on-chain state query helpers for bank, staking,
// distribution, authz, feegrant, claim, and EVM modules. These wrap lumerad
// CLI queries and parse the JSON output into Go types.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// --- File I/O ---

// saveAccounts writes the accounts file as indented JSON.
func saveAccounts(path string, af *AccountsFile) {
	data, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		log.Fatalf("marshal accounts: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}

// loadAccounts reads and parses the accounts JSON file.
func loadAccounts(path string) *AccountsFile {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}
	var af AccountsFile
	if err := json.Unmarshal(data, &af); err != nil {
		log.Fatalf("parse %s: %v", path, err)
	}
	return &af
}

// truncate returns s capped at maxLen characters with "..." appended if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// --- Staking queries ---

// queryDelegationCount returns the number of staking delegations for addr.
func queryDelegationCount(addr string) (int, error) {
	out, err := run("query", "staking", "delegations", addr)
	if err != nil {
		return 0, fmt.Errorf("query delegations: %s\n%w", out, err)
	}
	var resp struct {
		DelegationResponses []json.RawMessage `json:"delegation_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.DelegationResponses), nil
}

// queryDelegationToValidatorCount returns the number of delegations from addr to a specific validator.
func queryDelegationToValidatorCount(addr string, valoper string) (int, error) {
	out, err := run("query", "staking", "delegations", addr)
	if err != nil {
		return 0, fmt.Errorf("query delegations: %s\n%w", out, err)
	}
	var resp struct {
		DelegationResponses []struct {
			Delegation struct {
				ValidatorAddress string `json:"validator_address"`
			} `json:"delegation"`
		} `json:"delegation_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	n := 0
	for _, d := range resp.DelegationResponses {
		if d.Delegation.ValidatorAddress == valoper {
			n++
		}
	}
	return n, nil
}

// queryUnbondingCount returns the number of unbonding delegations for addr.
func queryUnbondingCount(addr string) (int, error) {
	out, err := run("query", "staking", "unbonding-delegations", addr)
	if err != nil {
		return 0, fmt.Errorf("query unbonding delegations: %s\n%w", out, err)
	}
	var resp struct {
		UnbondingResponses []json.RawMessage `json:"unbonding_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.UnbondingResponses), nil
}

// queryUnbondingFromValidatorCount returns the number of unbonding delegations from addr to a specific validator.
func queryUnbondingFromValidatorCount(addr string, valoper string) (int, error) {
	out, err := run("query", "staking", "unbonding-delegations", addr)
	if err != nil {
		return 0, fmt.Errorf("query unbonding delegations: %s\n%w", out, err)
	}
	var resp struct {
		UnbondingResponses []struct {
			ValidatorAddress string `json:"validator_address"`
		} `json:"unbonding_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	n := 0
	for _, u := range resp.UnbondingResponses {
		if u.ValidatorAddress == valoper {
			n++
		}
	}
	return n, nil
}

// queryRedelegationCount returns the number of redelegations from addr between srcVal and dstVal.
func queryRedelegationCount(addr string, srcVal string, dstVal string) (int, error) {
	out, err := run("query", "staking", "redelegation", addr, srcVal, dstVal)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no redelegation") {
			return 0, nil
		}
		return 0, fmt.Errorf("query redelegation: %s\n%w", out, err)
	}
	var resp struct {
		RedelegationResponses []json.RawMessage `json:"redelegation_responses"`
		Redelegation          json.RawMessage   `json:"redelegation"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	if len(resp.RedelegationResponses) > 0 {
		return len(resp.RedelegationResponses), nil
	}
	if len(resp.Redelegation) > 0 && string(resp.Redelegation) != "null" {
		return 1, nil
	}
	return 0, nil
}

// queryAnyRedelegationCount checks all validator pairs and returns the total
// redelegation count for addr.
func queryAnyRedelegationCount(addr string, validators []string) (int, error) {
	total := 0
	var firstErr error
	for _, src := range validators {
		for _, dst := range validators {
			if src == dst {
				continue
			}
			n, err := queryRedelegationCount(addr, src, dst)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			total += n
		}
	}
	if total > 0 {
		return total, nil
	}
	if firstErr != nil {
		return 0, firstErr
	}
	return 0, nil
}

// queryValidatorDelegationsToCount returns the number of delegations to a validator.
func queryValidatorDelegationsToCount(valoper string) (int, error) {
	out, err := run("query", "staking", "delegations-to", valoper)
	if err != nil {
		return 0, fmt.Errorf("query delegations-to: %s\n%w", out, err)
	}
	var resp struct {
		DelegationResponses []json.RawMessage `json:"delegation_responses"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	return len(resp.DelegationResponses), nil
}

// --- Distribution queries ---

// queryWithdrawAddress returns the distribution withdraw address for a delegator.
func queryWithdrawAddress(addr string) (string, error) {
	out, err := run("query", "distribution", "withdraw-addr", addr)
	if err != nil {
		out2, err2 := run("query", "distribution", "delegator-withdraw-address", "--delegator-address", addr)
		if err2 == nil {
			out, err = out2, nil
		}
	}
	if err != nil {
		return "", fmt.Errorf("query withdraw-addr: %s\n%w", out, err)
	}
	var resp struct {
		WithdrawAddress string `json:"withdraw_address"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", err
	}
	return resp.WithdrawAddress, nil
}

// --- Authz queries ---

// queryAuthzGrantExists returns true if a MsgSend authz grant exists from granter to grantee.
func queryAuthzGrantExists(granter, grantee string) (bool, error) {
	out, err := run("query", "authz", "grants", granter, grantee, "/cosmos.bank.v1beta1.MsgSend")
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no authorization") {
			return false, nil
		}
		return false, fmt.Errorf("query authz grants: %s\n%w", out, err)
	}
	var resp struct {
		Grants []json.RawMessage `json:"grants"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, err
	}
	return len(resp.Grants) > 0, nil
}

// --- Bank queries ---

// queryBalance returns the ulume balance for an address.
func queryBalance(addr string) (int64, error) {
	out, err := run("query", "bank", "balance", addr, "ulume")
	if err != nil {
		// Try alternative: some SDK versions use "balances" with --denom.
		out, err = run("query", "bank", "balances", addr, "--denom", "ulume")
		if err != nil {
			return 0, fmt.Errorf("query balance: %s\n%w", truncate(out, 300), err)
		}
	}
	var resp struct {
		Balance *struct {
			Amount string `json:"amount"`
		} `json:"balance"`
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, fmt.Errorf("parse balance: %s\n%w", truncate(out, 300), err)
	}
	amtStr := resp.Amount
	if resp.Balance != nil && resp.Balance.Amount != "" {
		amtStr = resp.Balance.Amount
	}
	if amtStr == "" {
		return 0, nil
	}
	amt, err := strconv.ParseInt(amtStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse amount %q: %w", amtStr, err)
	}
	return amt, nil
}

// queryBech32ToHex converts a bech32 address to 0x hex via lumerad.
func queryBech32ToHex(bech32Addr string) (string, error) {
	out, err := run("query", "evm", "bech32-to-0x", bech32Addr)
	if err != nil {
		return "", fmt.Errorf("bech32-to-0x: %s\n%w", truncate(out, 200), err)
	}
	hex := strings.TrimSpace(out)
	// Output may be just the hex, or JSON — handle both.
	if strings.HasPrefix(hex, "0x") || strings.HasPrefix(hex, "0X") {
		return hex, nil
	}
	// Try JSON parse.
	var resp struct {
		Hex string `json:"hex"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err == nil && resp.Hex != "" {
		return resp.Hex, nil
	}
	return hex, nil
}

// queryEVMBalanceBank queries the EVM balance-bank for ulume at a hex address.
func queryEVMBalanceBank(hexAddr string) (int64, error) {
	out, err := run("query", "evm", "balance-bank", hexAddr, "ulume")
	if err != nil {
		return 0, fmt.Errorf("evm balance-bank: %s\n%w", truncate(out, 200), err)
	}
	var resp struct {
		Balance *struct {
			Amount string `json:"amount"`
		} `json:"balance"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, err
	}
	if resp.Balance == nil || resp.Balance.Amount == "" {
		return 0, nil
	}
	return strconv.ParseInt(resp.Balance.Amount, 10, 64)
}

// queryEVMAccountBalance queries the EVM account balance (18-decimal string).
func queryEVMAccountBalance(hexAddr string) (string, error) {
	out, err := run("query", "evm", "account", hexAddr)
	if err != nil {
		return "", fmt.Errorf("evm account: %s\n%w", truncate(out, 200), err)
	}
	var resp struct {
		Balance string `json:"balance"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", err
	}
	return resp.Balance, nil
}

// queryHasAnyBalance returns true if the address holds any token balance.
func queryHasAnyBalance(addr string) (bool, error) {
	out, err := run("query", "bank", "balances", addr)
	if err != nil {
		return false, fmt.Errorf("query bank balances: %s\n%w", out, err)
	}
	var resp struct {
		Balances []json.RawMessage `json:"balances"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, err
	}
	return len(resp.Balances) > 0, nil
}

// --- Feegrant queries ---

// queryFeegrantAllowanceExists returns true if a fee grant exists from granter to grantee.
func queryFeegrantAllowanceExists(granter, grantee string) (bool, error) {
	out, err := run("query", "feegrant", "grant", granter, grantee)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not found") || strings.Contains(low, "no allowance") || strings.Contains(low, "fee-grant not found") {
			return false, nil
		}
		return false, fmt.Errorf("query feegrant grant: %s\n%w", out, err)
	}
	return true, nil
}

// --- Claim queries ---

// queryClaimRecord returns the claim record for the given Pastel (old) address.
// Returns (claimed, destAddress, vestedTier, err). If the record does not exist, returns an error.
func queryClaimRecord(oldAddress string) (claimed bool, destAddress string, vestedTier uint32, err error) {
	out, err := run("query", "claim", "claim-record", oldAddress)
	if err != nil {
		return false, "", 0, fmt.Errorf("query claim record: %s\n%w", truncate(out, 300), err)
	}
	var resp struct {
		Record struct {
			Claimed      bool   `json:"claimed"`
			DestAddress  string `json:"destAddress"`
			NewAddress   string `json:"newAddress"`
			VestedTier   uint32 `json:"vestedTier"`
			VestedTierSn uint32 `json:"vested_tier"`
		} `json:"record"`
		ClaimRecordCamel struct {
			Claimed      bool   `json:"claimed"`
			DestAddress  string `json:"destAddress"`
			NewAddress   string `json:"newAddress"`
			VestedTier   uint32 `json:"vestedTier"`
			VestedTierSn uint32 `json:"vested_tier"`
		} `json:"claimRecord"`
		ClaimRecord struct {
			Claimed      bool   `json:"claimed"`
			DestAddress  string `json:"dest_address"`
			NewAddress   string `json:"new_address"`
			VestedTier   uint32 `json:"vested_tier"`
			VestedTierCm uint32 `json:"vestedTier"`
		} `json:"claim_record"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, "", 0, fmt.Errorf("parse claim record: %s\n%w", truncate(out, 300), err)
	}

	claimed = resp.Record.Claimed || resp.ClaimRecord.Claimed || resp.ClaimRecordCamel.Claimed
	destAddress = resp.Record.DestAddress
	if destAddress == "" {
		destAddress = resp.Record.NewAddress
	}
	if destAddress == "" {
		destAddress = resp.ClaimRecord.DestAddress
	}
	if destAddress == "" {
		destAddress = resp.ClaimRecord.NewAddress
	}
	if destAddress == "" {
		destAddress = resp.ClaimRecordCamel.DestAddress
	}
	if destAddress == "" {
		destAddress = resp.ClaimRecordCamel.NewAddress
	}
	vestedTier = resp.Record.VestedTier
	if vestedTier == 0 {
		vestedTier = resp.Record.VestedTierSn
	}
	if vestedTier == 0 {
		vestedTier = resp.ClaimRecord.VestedTier
	}
	if vestedTier == 0 {
		vestedTier = resp.ClaimRecord.VestedTierCm
	}
	if vestedTier == 0 {
		vestedTier = resp.ClaimRecordCamel.VestedTier
	}
	if vestedTier == 0 {
		vestedTier = resp.ClaimRecordCamel.VestedTierSn
	}
	return claimed, destAddress, vestedTier, nil
}

// queryClaimedCountByTier returns number of claimed records for a delayed vesting tier.
func queryClaimedCountByTier(tier uint32) (int, error) {
	out, err := run("query", "claim", "list-claimed", fmt.Sprintf("%d", tier))
	if err != nil {
		return 0, fmt.Errorf("query list-claimed tier=%d: %s\n%w", tier, truncate(out, 300), err)
	}
	var resp struct {
		Claims []json.RawMessage `json:"claims"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, fmt.Errorf("parse list-claimed tier=%d: %s\n%w", tier, truncate(out, 300), err)
	}
	return len(resp.Claims), nil
}

// queryHasAnyDelayedClaim returns true if any delayed claim records exist for tiers 1-3.
func queryHasAnyDelayedClaim() (bool, error) {
	for _, tier := range []uint32{1, 2, 3} {
		n, err := queryClaimedCountByTier(tier)
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// maxNumericID returns the highest numeric ID from a slice of string IDs.
// Falls back to the last element if none are numeric.
func maxNumericID(ids []string) string {
	best := ""
	bestN := int64(-1)
	for _, id := range ids {
		n, err := strconv.ParseInt(id, 10, 64)
		if err == nil && n > bestN {
			bestN = n
			best = id
		}
	}
	if best != "" {
		return best
	}
	return ids[len(ids)-1]
}
