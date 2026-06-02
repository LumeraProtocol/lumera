package common

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reAccountSequence = regexp.MustCompile(`expected\s+(\d+),\s+got\s+(\d+)`)
	reAccountNumber   = regexp.MustCompile(`account number \((\d+)\)`)
)

// ParseIncorrectAccountSequence extracts the expected and got sequence numbers
// from an "incorrect account sequence" error message.
func ParseIncorrectAccountSequence(err error) (expected, got uint64, ok bool) {
	if err == nil {
		return 0, 0, false
	}
	if !strings.Contains(strings.ToLower(err.Error()), "incorrect account sequence") {
		return 0, 0, false
	}
	m := reAccountSequence.FindStringSubmatch(err.Error())
	if len(m) != 3 {
		return 0, 0, false
	}
	expected, err1 := strconv.ParseUint(m[1], 10, 64)
	got, err2 := strconv.ParseUint(m[2], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return expected, got, true
}

// ParseSignatureMismatchAccountNumber extracts the expected account number from
// a "signature verification failed" error message.
func ParseSignatureMismatchAccountNumber(err error) (uint64, bool) {
	if err == nil {
		return 0, false
	}
	if !strings.Contains(strings.ToLower(err.Error()), "signature verification failed") {
		return 0, false
	}
	m := reAccountNumber.FindStringSubmatch(err.Error())
	if len(m) != 2 {
		return 0, false
	}
	n, parseErr := strconv.ParseUint(m[1], 10, 64)
	if parseErr != nil {
		return 0, false
	}
	return n, true
}

// ExtractJSONPayload pulls the outermost JSON object out of mixed stdout/stderr
// command output (e.g. a gas-estimate line preceding the broadcast response).
func ExtractJSONPayload(out string) (string, bool) {
	start := strings.IndexByte(out, '{')
	end := strings.LastIndexByte(out, '}')
	if start == -1 || end == -1 || end < start {
		return "", false
	}
	return strings.TrimSpace(out[start : end+1]), true
}

// ParseAuthAccountNumberAndSequence parses the JSON from `lumerad query auth
// account` and extracts (account_number, sequence). It tolerates the several
// response shapes the SDK emits across account types and output modes:
// BaseAccount (proto-JSON), amino-JSON envelope (account.value.*),
// ModuleAccount (account.base_account.*), and vesting accounts
// (account.base_vesting_account.base_account.*).
func ParseAuthAccountNumberAndSequence(out string) (uint64, uint64, error) {
	type baseAcc struct {
		AccountNumber string `json:"account_number"`
		Sequence      string `json:"sequence"`
	}
	type vestingWrap struct {
		BaseAccount *baseAcc `json:"base_account"`
	}
	var resp struct {
		Account struct {
			AccountNumber string `json:"account_number"`
			Sequence      string `json:"sequence"`
			Value         *struct {
				AccountNumber      string       `json:"account_number"`
				Sequence           string       `json:"sequence"`
				BaseVestingAccount *vestingWrap `json:"base_vesting_account"`
			} `json:"value"`
			BaseAccount        *baseAcc     `json:"base_account"`
			BaseVestingAccount *vestingWrap `json:"base_vesting_account"`
		} `json:"account"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, 0, fmt.Errorf("parse auth account: %s: %w", truncate(out, 200), err)
	}

	numStr, seqStr := resp.Account.AccountNumber, resp.Account.Sequence
	pick := func(num, seq string) {
		if num != "" {
			numStr = num
		}
		if seq != "" {
			seqStr = seq
		}
	}
	switch {
	case resp.Account.Value != nil:
		pick(resp.Account.Value.AccountNumber, resp.Account.Value.Sequence)
		if v := resp.Account.Value.BaseVestingAccount; v != nil && v.BaseAccount != nil {
			pick(v.BaseAccount.AccountNumber, v.BaseAccount.Sequence)
		}
	case resp.Account.BaseAccount != nil:
		pick(resp.Account.BaseAccount.AccountNumber, resp.Account.BaseAccount.Sequence)
	case resp.Account.BaseVestingAccount != nil && resp.Account.BaseVestingAccount.BaseAccount != nil:
		pick(resp.Account.BaseVestingAccount.BaseAccount.AccountNumber, resp.Account.BaseVestingAccount.BaseAccount.Sequence)
	}

	num, err := strconv.ParseUint(emptyAsZero(numStr), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse account number %q: %w", numStr, err)
	}
	seq, err := strconv.ParseUint(emptyAsZero(seqStr), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse sequence %q: %w", seqStr, err)
	}
	return num, seq, nil
}

// ParseBankBalance parses the JSON from `lumerad query bank balance` (singular
// or flat shape) and returns the integer amount. An absent balance is 0.
func ParseBankBalance(out string) (int64, error) {
	var resp struct {
		Balance *struct {
			Amount string `json:"amount"`
		} `json:"balance"`
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, fmt.Errorf("parse balance: %s: %w", truncate(out, 200), err)
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

func emptyAsZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// truncate shortens a string for error messages.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ParseRedelegationCount counts redelegations in the output of
// `query staking redelegation`, tolerating both the plural
// (redelegation_responses) and singular (redelegation) shapes.
func ParseRedelegationCount(out string) (int, error) {
	var resp struct {
		RedelegationResponses []json.RawMessage `json:"redelegation_responses"`
		Redelegation          json.RawMessage   `json:"redelegation"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, fmt.Errorf("parse redelegation: %s: %w", truncate(out, 200), err)
	}
	if len(resp.RedelegationResponses) > 0 {
		return len(resp.RedelegationResponses), nil
	}
	if len(resp.Redelegation) > 0 && string(resp.Redelegation) != "null" {
		return 1, nil
	}
	return 0, nil
}

// ParseAuthzGrantCount counts grants in the output of `query authz grants`.
func ParseAuthzGrantCount(out string) (int, error) {
	var resp struct {
		Grants []json.RawMessage `json:"grants"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, fmt.Errorf("parse authz grants: %s: %w", truncate(out, 200), err)
	}
	return len(resp.Grants), nil
}

// ParseFeegrantExists reports whether `query feegrant grant` returned a
// non-null allowance.
func ParseFeegrantExists(out string) (bool, error) {
	var resp struct {
		Allowance json.RawMessage `json:"allowance"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, fmt.Errorf("parse feegrant allowance: %s: %w", truncate(out, 200), err)
	}
	return len(resp.Allowance) > 0 && string(resp.Allowance) != "null", nil
}

// ParseValidatorAddresses extracts validator operator addresses from the JSON
// output of `lumerad query staking validators`.
func ParseValidatorAddresses(data []byte) ([]string, error) {
	var result struct {
		Validators []struct {
			OperatorAddress string `json:"operator_address"`
		} `json:"validators"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse validators: %w", err)
	}
	addrs := make([]string, 0, len(result.Validators))
	for _, v := range result.Validators {
		addrs = append(addrs, v.OperatorAddress)
	}
	return addrs, nil
}
