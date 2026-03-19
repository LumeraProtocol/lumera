// query_migration.go provides query helpers for the evmigration module:
// migration estimates, stats, params, account info, and flexible JSON parsers
// for handling inconsistent Cosmos SDK query output formats across versions.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// migrationEstimate holds the result of a migration-estimate query for a single address.
type migrationEstimate struct {
	WouldSucceed       bool   `json:"would_succeed"`
	RejectionReason    string `json:"rejection_reason"`
	DelegationCount    int    `json:"delegation_count"`
	UnbondingCount     int    `json:"unbonding_count"`
	RedelegationCount  int    `json:"redelegation_count"`
	AuthzGrantCount    int    `json:"authz_grant_count"`
	FeegrantCount      int    `json:"feegrant_count"`
	ActionCount        int    `json:"action_count"`
	ValDelegationCount int    `json:"val_delegation_count"`
	IsValidator        bool   `json:"is_validator"`
}

// migrationStats holds the global migration statistics from the evmigration module.
type migrationStats struct {
	TotalMigrated           int `json:"total_migrated"`
	TotalLegacy             int `json:"total_legacy"`
	TotalLegacyStaked       int `json:"total_legacy_staked"`
	TotalValidatorsMigrated int `json:"total_validators_migrated"`
	TotalValidatorsLegacy   int `json:"total_validators_legacy"`
}

// migrationParams holds the evmigration module parameters.
type migrationParams struct {
	EnableMigration         bool  `json:"enable_migration"`
	MigrationEndTime        int64 `json:"migration_end_time"`
	MaxMigrationsPerBlock   int   `json:"max_migrations_per_block"`
	MaxValidatorDelegations int   `json:"max_validator_delegations"`
}

// queryMigrationEstimate queries the evmigration module for a migration estimate
// for the given legacy address.
func queryMigrationEstimate(addr string) (migrationEstimate, error) {
	out, err := run("query", "evmigration", "migration-estimate", addr)
	if err != nil {
		return migrationEstimate{}, fmt.Errorf("query migration-estimate: %s\n%w", out, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate: %s\n%w", truncate(out, 300), err)
	}

	estimate := migrationEstimate{}
	if estimate.WouldSucceed, err = parseFlexibleJSONBool(raw["would_succeed"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.would_succeed: %w", err)
	}
	estimate.RejectionReason = parseFlexibleJSONString(raw["rejection_reason"])
	if estimate.DelegationCount, err = parseFlexibleJSONInt(raw["delegation_count"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.delegation_count: %w", err)
	}
	if estimate.UnbondingCount, err = parseFlexibleJSONInt(raw["unbonding_count"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.unbonding_count: %w", err)
	}
	if estimate.RedelegationCount, err = parseFlexibleJSONInt(raw["redelegation_count"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.redelegation_count: %w", err)
	}
	if estimate.AuthzGrantCount, err = parseFlexibleJSONInt(raw["authz_grant_count"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.authz_grant_count: %w", err)
	}
	if estimate.FeegrantCount, err = parseFlexibleJSONInt(raw["feegrant_count"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.feegrant_count: %w", err)
	}
	if estimate.ActionCount, err = parseFlexibleJSONInt(raw["action_count"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.action_count: %w", err)
	}
	if estimate.ValDelegationCount, err = parseFlexibleJSONInt(raw["val_delegation_count"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.val_delegation_count: %w", err)
	}
	if estimate.IsValidator, err = parseFlexibleJSONBool(raw["is_validator"]); err != nil {
		return migrationEstimate{}, fmt.Errorf("parse migration-estimate.is_validator: %w", err)
	}

	return estimate, nil
}

// queryAccountNumberAndSequence returns the on-chain account number and sequence
// for an address, handling multiple SDK JSON response shapes.
func queryAccountNumberAndSequence(addr string) (accountNumber uint64, sequence uint64, err error) {
	out, err := run("query", "auth", "account", addr)
	if err != nil {
		return 0, 0, fmt.Errorf("query auth account: %s\n%w", out, err)
	}

	var resp struct {
		Account struct {
			AccountNumber string `json:"account_number"`
			Sequence      string `json:"sequence"`
			Value         *struct {
				AccountNumber string `json:"account_number"`
				Sequence      string `json:"sequence"`
			} `json:"value"`
			BaseAccount *struct {
				AccountNumber string `json:"account_number"`
				Sequence      string `json:"sequence"`
			} `json:"base_account"`
		} `json:"account"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, 0, fmt.Errorf("parse auth account: %s\n%w", truncate(out, 300), err)
	}

	accNumStr := resp.Account.AccountNumber
	seqStr := resp.Account.Sequence
	if resp.Account.Value != nil {
		if resp.Account.Value.AccountNumber != "" {
			accNumStr = resp.Account.Value.AccountNumber
		}
		if resp.Account.Value.Sequence != "" {
			seqStr = resp.Account.Value.Sequence
		}
	}
	if resp.Account.BaseAccount != nil {
		if resp.Account.BaseAccount.AccountNumber != "" {
			accNumStr = resp.Account.BaseAccount.AccountNumber
		}
		if resp.Account.BaseAccount.Sequence != "" {
			seqStr = resp.Account.BaseAccount.Sequence
		}
	}
	if accNumStr == "" || seqStr == "" {
		return 0, 0, fmt.Errorf("account_number/sequence missing in auth account response: %s", truncate(out, 300))
	}

	accountNumber, err = strconv.ParseUint(accNumStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse account_number %q: %w", accNumStr, err)
	}
	sequence, err = strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse sequence %q: %w", seqStr, err)
	}
	return accountNumber, sequence, nil
}

// queryAccountIsVesting returns true if the on-chain account is a vesting account.
func queryAccountIsVesting(addr string) (bool, error) {
	out, err := run("query", "auth", "account", addr)
	if err != nil {
		return false, fmt.Errorf("query auth account: %s\n%w", out, err)
	}
	return authAccountLooksVesting(out), nil
}

// authAccountLooksVesting returns true if the auth account JSON output contains vesting indicators.
func authAccountLooksVesting(out string) bool {
	var payload any
	if err := json.Unmarshal([]byte(out), &payload); err == nil {
		return authAccountPayloadLooksVesting(payload)
	}

	lower := strings.ToLower(out)
	return strings.Contains(lower, "vestingaccount") || strings.Contains(lower, "/cosmos.vesting.")
}

// authAccountPayloadLooksVesting recursively checks if any value in the parsed
// JSON payload indicates a vesting account type.
func authAccountPayloadLooksVesting(v any) bool {
	switch value := v.(type) {
	case map[string]any:
		for key, nested := range value {
			if (key == "@type" || key == "type") && isVestingAccountType(fmt.Sprint(nested)) {
				return true
			}
			if authAccountPayloadLooksVesting(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range value {
			if authAccountPayloadLooksVesting(nested) {
				return true
			}
		}
	}
	return false
}

// isVestingAccountType returns true if the type name indicates a vesting account.
func isVestingAccountType(typeName string) bool {
	lower := strings.ToLower(strings.TrimSpace(typeName))
	return strings.Contains(lower, "vestingaccount") || strings.HasPrefix(lower, "/cosmos.vesting.")
}

// isAccountNotFoundErr returns true if the error indicates the account does not exist on-chain.
func isAccountNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "account") &&
		strings.Contains(low, "not found")
}

// accountSequenceForFirstTx returns the account number and sequence, defaulting
// to (0, 0) if the account does not yet exist on-chain.
func accountSequenceForFirstTx(addr string) (accountNumber uint64, sequence uint64, err error) {
	accountNumber, sequence, err = queryAccountNumberAndSequence(addr)
	if err == nil {
		return accountNumber, sequence, nil
	}
	if isAccountNotFoundErr(err) {
		return 0, 0, nil
	}
	return 0, 0, err
}

// parseSignatureMismatchAccountNumber extracts the expected account number from
// a "signature verification failed" error message.
func parseSignatureMismatchAccountNumber(err error) (uint64, bool) {
	if err == nil {
		return 0, false
	}
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "signature verification failed") {
		return 0, false
	}
	// Example:
	// "signature verification failed; please verify account number (76) and chain-id (...): unauthorized"
	m := regexp.MustCompile(`account number \((\d+)\)`).FindStringSubmatch(err.Error())
	if len(m) != 2 {
		return 0, false
	}
	n, parseErr := strconv.ParseUint(m[1], 10, 64)
	if parseErr != nil {
		return 0, false
	}
	return n, true
}

// parseIncorrectAccountSequence extracts the expected and got sequence numbers
// from an "incorrect account sequence" error message.
func parseIncorrectAccountSequence(err error) (expected uint64, got uint64, ok bool) {
	if err == nil {
		return 0, 0, false
	}
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "incorrect account sequence") {
		return 0, 0, false
	}

	m := regexp.MustCompile(`expected\s+(\d+),\s+got\s+(\d+)`).FindStringSubmatch(err.Error())
	if len(m) != 3 {
		return 0, 0, false
	}

	expected, err = strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	got, err = strconv.ParseUint(m[2], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return expected, got, true
}

// waitForAccountOnChain polls until the account is queryable on-chain or the timeout expires.
func waitForAccountOnChain(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		if _, _, err := queryAccountNumberAndSequence(addr); err == nil {
			return nil
		} else {
			lastErr = err
			if !isAccountNotFoundErr(err) {
				return err
			}
		}
		time.Sleep(time.Second)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timed out waiting for account")
	}
	return fmt.Errorf("account %s not available on-chain after %s: %w", addr, timeout, lastErr)
}

// queryMigrationStats queries the global migration statistics from the evmigration module.
func queryMigrationStats() (migrationStats, error) {
	out, err := run("query", "evmigration", "migration-stats")
	if err != nil {
		return migrationStats{}, fmt.Errorf("query migration-stats: %s\n%w", out, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return migrationStats{}, fmt.Errorf("parse migration-stats: %s\n%w", truncate(out, 300), err)
	}

	stats := migrationStats{}
	if stats.TotalMigrated, err = parseFlexibleJSONInt(raw["total_migrated"]); err != nil {
		return migrationStats{}, fmt.Errorf("parse migration-stats.total_migrated: %w", err)
	}
	if stats.TotalLegacy, err = parseFlexibleJSONInt(raw["total_legacy"]); err != nil {
		return migrationStats{}, fmt.Errorf("parse migration-stats.total_legacy: %w", err)
	}
	if stats.TotalLegacyStaked, err = parseFlexibleJSONInt(raw["total_legacy_staked"]); err != nil {
		return migrationStats{}, fmt.Errorf("parse migration-stats.total_legacy_staked: %w", err)
	}
	if stats.TotalValidatorsMigrated, err = parseFlexibleJSONInt(raw["total_validators_migrated"]); err != nil {
		return migrationStats{}, fmt.Errorf("parse migration-stats.total_validators_migrated: %w", err)
	}
	if stats.TotalValidatorsLegacy, err = parseFlexibleJSONInt(raw["total_validators_legacy"]); err != nil {
		return migrationStats{}, fmt.Errorf("parse migration-stats.total_validators_legacy: %w", err)
	}

	return stats, nil
}

// queryMigrationParams queries the evmigration module parameters.
func queryMigrationParams() (migrationParams, error) {
	out, err := run("query", "evmigration", "params")
	if err != nil {
		return migrationParams{}, fmt.Errorf("query evmigration params: %s\n%w", out, err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &top); err != nil {
		return migrationParams{}, fmt.Errorf("parse evmigration params: %s\n%w", truncate(out, 300), err)
	}

	paramsRaw := top["params"]
	if len(paramsRaw) == 0 {
		paramsRaw = []byte(out)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(paramsRaw, &raw); err != nil {
		return migrationParams{}, fmt.Errorf("parse evmigration params payload: %s\n%w", truncate(string(paramsRaw), 300), err)
	}

	params := migrationParams{}
	if params.EnableMigration, err = parseFlexibleJSONBool(raw["enable_migration"]); err != nil {
		return migrationParams{}, fmt.Errorf("parse params.enable_migration: %w", err)
	}
	if params.MigrationEndTime, err = parseFlexibleJSONInt64(raw["migration_end_time"]); err != nil {
		return migrationParams{}, fmt.Errorf("parse params.migration_end_time: %w", err)
	}
	if params.MaxMigrationsPerBlock, err = parseFlexibleJSONInt(raw["max_migrations_per_block"]); err != nil {
		return migrationParams{}, fmt.Errorf("parse params.max_migrations_per_block: %w", err)
	}
	if params.MaxValidatorDelegations, err = parseFlexibleJSONInt(raw["max_validator_delegations"]); err != nil {
		return migrationParams{}, fmt.Errorf("parse params.max_validator_delegations: %w", err)
	}

	return params, nil
}

// --- Flexible JSON parsers ---
// Cosmos SDK query output is inconsistent across versions: numeric fields may
// appear as JSON numbers or as quoted strings. These helpers handle both.

// parseFlexibleJSONInt parses an int from JSON that may be a number or a quoted string.
func parseFlexibleJSONInt(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(asString)
		if err != nil {
			return 0, fmt.Errorf("parse %q as int: %w", asString, err)
		}
		return n, nil
	}

	var asInt int
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return asInt, nil
	}

	var asInt64 int64
	if err := json.Unmarshal(raw, &asInt64); err == nil {
		return int(asInt64), nil
	}

	return 0, fmt.Errorf("unsupported numeric format: %s", string(raw))
}

// parseFlexibleJSONInt64 parses an int64 from JSON that may be a number or a quoted string.
func parseFlexibleJSONInt64(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return 0, nil
		}
		n, err := strconv.ParseInt(asString, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse %q as int64: %w", asString, err)
		}
		return n, nil
	}

	var asInt64 int64
	if err := json.Unmarshal(raw, &asInt64); err == nil {
		return asInt64, nil
	}

	var asInt int
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return int64(asInt), nil
	}

	return 0, fmt.Errorf("unsupported numeric format: %s", string(raw))
}

// parseFlexibleJSONBool parses a bool from JSON that may be a boolean or a quoted string.
func parseFlexibleJSONBool(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return false, nil
	}

	var asBool bool
	if err := json.Unmarshal(raw, &asBool); err == nil {
		return asBool, nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(strings.ToLower(asString))
		switch asString {
		case "", "false", "0":
			return false, nil
		case "true", "1":
			return true, nil
		default:
			return false, fmt.Errorf("parse %q as bool", asString)
		}
	}

	return false, fmt.Errorf("unsupported bool format: %s", string(raw))
}

// parseFlexibleJSONString parses a string from JSON, falling back to raw content if unquoted.
func parseFlexibleJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}
	return strings.TrimSpace(string(raw))
}
