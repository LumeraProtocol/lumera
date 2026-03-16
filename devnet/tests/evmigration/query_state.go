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

func saveAccounts(path string, af *AccountsFile) {
	data, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		log.Fatalf("marshal accounts: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}

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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// --- Staking queries ---

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
