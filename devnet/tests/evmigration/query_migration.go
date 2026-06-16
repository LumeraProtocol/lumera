// query_migration.go provides query helpers for the evmigration module:
// migration estimates, stats, params, account info, and flexible JSON parsers
// for handling inconsistent Cosmos SDK query output formats across versions.
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gen/tests/common"
)

// evmigCLI builds a common.ChainCLI from this tool's connection flags so the
// shared migration query/parsing primitives can be reused instead of the local
// duplicates.
func evmigCLI() *common.ChainCLI {
	return &common.ChainCLI{
		Bin:            *flagBin,
		ChainID:        *flagChainID,
		RPC:            *flagRPC,
		Home:           *flagHome,
		KeyringBackend: "test",
		Gas:            *flagGas,
		GasPrices:      *flagGasPrices,
	}
}

const (
	protoPermanentLockedAccountType  = "/cosmos.vesting.v1beta1.PermanentLockedAccount"
	legacyPermanentLockedAccountType = "cosmos-sdk/PermanentLockedAccount"
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
// for the given legacy address. It delegates to the shared common helper and
// maps the result onto this tool's local type.
func queryMigrationEstimate(addr string) (migrationEstimate, error) {
	est, err := evmigCLI().MigrationEstimate(addr)
	if err != nil {
		return migrationEstimate{}, err
	}
	return migrationEstimate{
		WouldSucceed:       est.WouldSucceed,
		RejectionReason:    est.RejectionReason,
		DelegationCount:    est.DelegationCount,
		UnbondingCount:     est.UnbondingCount,
		RedelegationCount:  est.RedelegationCount,
		AuthzGrantCount:    est.AuthzGrantCount,
		FeegrantCount:      est.FeegrantCount,
		ActionCount:        est.ActionCount,
		ValDelegationCount: est.ValDelegationCount,
		IsValidator:        est.IsValidator,
	}, nil
}

// queryAccountNumberAndSequence returns the on-chain account number and sequence
// for an address, handling multiple SDK JSON response shapes.
func queryAccountNumberAndSequence(addr string) (accountNumber uint64, sequence uint64, err error) {
	out, err := run("query", "auth", "account", addr)
	if err != nil {
		return 0, 0, fmt.Errorf("query auth account: %s\n%w", out, err)
	}
	return parseAuthAccountNumberAndSequence(out)
}

// parseAuthAccountNumberAndSequence parses the JSON returned by
// `lumerad query auth account` and extracts (account_number, sequence). It
// tolerates the several response shapes the SDK emits across account types and
// output modes:
//
//   - BaseAccount, proto-JSON:   account.{account_number, sequence}
//   - BaseAccount, amino-JSON:   account.value.{account_number, sequence}
//   - ModuleAccount:             account.base_account.{account_number, sequence}
//   - Vesting, proto-JSON:       account.base_vesting_account.base_account.{...}
//   - Vesting, amino-JSON:       account.value.base_vesting_account.base_account.{...}
//
// The vesting paths matter for legacy multisig accounts wrapped in
// PermanentLockedAccount / ContinuousVestingAccount / etc.
func parseAuthAccountNumberAndSequence(out string) (uint64, uint64, error) {
	type baseAcc struct {
		AccountNumber string `json:"account_number"`
		Sequence      string `json:"sequence"`
	}
	type vestingWrap struct {
		BaseAccount *baseAcc `json:"base_account"`
	}
	var resp struct {
		Account struct {
			// BaseAccount (proto-JSON) — fields directly on `account`.
			AccountNumber string `json:"account_number"`
			Sequence      string `json:"sequence"`
			// Amino-JSON envelope — `account.value.*`.
			Value *struct {
				AccountNumber      string       `json:"account_number"`
				Sequence           string       `json:"sequence"`
				BaseVestingAccount *vestingWrap `json:"base_vesting_account"`
			} `json:"value"`
			// ModuleAccount-style nested BaseAccount.
			BaseAccount *baseAcc `json:"base_account"`
			// Vesting (proto-JSON) — base_vesting_account directly on `account`.
			BaseVestingAccount *vestingWrap `json:"base_vesting_account"`
		} `json:"account"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return 0, 0, fmt.Errorf("parse auth account: %s\n%w", truncate(out, 300), err)
	}

	accNumStr := resp.Account.AccountNumber
	seqStr := resp.Account.Sequence
	pick := func(num, seq string) {
		if num != "" {
			accNumStr = num
		}
		if seq != "" {
			seqStr = seq
		}
	}
	if v := resp.Account.Value; v != nil {
		pick(v.AccountNumber, v.Sequence)
		if v.BaseVestingAccount != nil && v.BaseVestingAccount.BaseAccount != nil {
			pick(v.BaseVestingAccount.BaseAccount.AccountNumber, v.BaseVestingAccount.BaseAccount.Sequence)
		}
	}
	if b := resp.Account.BaseAccount; b != nil {
		pick(b.AccountNumber, b.Sequence)
	}
	if vba := resp.Account.BaseVestingAccount; vba != nil && vba.BaseAccount != nil {
		pick(vba.BaseAccount.AccountNumber, vba.BaseAccount.Sequence)
	}

	// Cosmos SDK omits `sequence` from JSON when it's 0 (fresh accounts that
	// haven't sent any tx yet). Treat missing sequence as 0; only reject when
	// the account itself is absent from the response.
	if accNumStr == "" {
		return 0, 0, fmt.Errorf("account_number missing in auth account response: %s", truncate(out, 300))
	}
	if seqStr == "" {
		seqStr = "0"
	}

	accountNumber, err := strconv.ParseUint(accNumStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse account_number %q: %w", accNumStr, err)
	}
	sequence, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse sequence %q: %w", seqStr, err)
	}
	return accountNumber, sequence, nil
}

// queryAuthAccountType returns the auth account type name for an address.
func queryAuthAccountType(addr string) (string, error) {
	out, err := run("query", "auth", "account", addr)
	if err != nil {
		return "", fmt.Errorf("query auth account: %s\n%w", out, err)
	}

	if typeName := authAccountTypeName(out); typeName != "" {
		return typeName, nil
	}
	return "", fmt.Errorf("account type missing in auth account response: %s", truncate(out, 300))
}

// queryAccountIsVesting returns true if the on-chain account is a vesting account.
func queryAccountIsVesting(addr string) (bool, error) {
	out, err := run("query", "auth", "account", addr)
	if err != nil {
		return false, fmt.Errorf("query auth account: %s\n%w", out, err)
	}
	return authAccountLooksVesting(out), nil
}

// queryAccountIsPermanentLocked returns true if the on-chain account is a
// PermanentLockedAccount.
func queryAccountIsPermanentLocked(addr string) (bool, error) {
	out, err := run("query", "auth", "account", addr)
	if err != nil {
		return false, fmt.Errorf("query auth account: %s\n%w", out, err)
	}
	return authAccountLooksPermanentLocked(out), nil
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

// authAccountLooksPermanentLocked returns true if the auth account JSON output
// identifies a PermanentLockedAccount.
func authAccountLooksPermanentLocked(out string) bool {
	var payload any
	if err := json.Unmarshal([]byte(out), &payload); err == nil {
		return authAccountPayloadMatchesType(payload, isPermanentLockedAccountType)
	}

	lower := strings.ToLower(out)
	return strings.Contains(lower, "permanentlockedaccount")
}

// authAccountTypeName extracts the first auth account type name from query JSON.
func authAccountTypeName(out string) string {
	var payload any
	if err := json.Unmarshal([]byte(out), &payload); err == nil {
		return authAccountPayloadTypeName(payload)
	}
	return ""
}

// authAccountPayloadLooksVesting recursively checks if any value in the parsed
// JSON payload indicates a vesting account type.
func authAccountPayloadLooksVesting(v any) bool {
	return authAccountPayloadMatchesType(v, isVestingAccountType)
}

// authAccountPayloadMatchesType recursively checks whether any parsed JSON node
// advertises an auth account type satisfying match.
func authAccountPayloadMatchesType(v any, match func(string) bool) bool {
	switch value := v.(type) {
	case map[string]any:
		for key, nested := range value {
			if (key == "@type" || key == "type") && match(fmt.Sprint(nested)) {
				return true
			}
			if authAccountPayloadMatchesType(nested, match) {
				return true
			}
		}
	case []any:
		for _, nested := range value {
			if authAccountPayloadMatchesType(nested, match) {
				return true
			}
		}
	}
	return false
}

// authAccountPayloadTypeName extracts the outermost auth account type from
// parsed query JSON. At each map level the direct `@type`/`type` key wins over
// recursion, so nested pubkey `@type` fields (e.g. /cosmos.crypto.secp256k1.PubKey)
// don't mask the surrounding account type (e.g. /cosmos.auth.v1beta1.BaseAccount).
// This matters because Go map iteration order is randomized — a recurse-first
// walk could return the pubkey type on one run and the account type on another.
func authAccountPayloadTypeName(v any) string {
	switch value := v.(type) {
	case map[string]any:
		if raw, ok := value["@type"]; ok {
			return fmt.Sprint(raw)
		}
		if raw, ok := value["type"]; ok {
			return fmt.Sprint(raw)
		}
		for _, nested := range value {
			if typeName := authAccountPayloadTypeName(nested); typeName != "" {
				return typeName
			}
		}
	case []any:
		for _, nested := range value {
			if typeName := authAccountPayloadTypeName(nested); typeName != "" {
				return typeName
			}
		}
	}
	return ""
}

// isVestingAccountType returns true if the type name indicates a vesting account.
func isVestingAccountType(typeName string) bool {
	lower := strings.ToLower(strings.TrimSpace(typeName))
	return strings.Contains(lower, "vestingaccount") || strings.HasPrefix(lower, "/cosmos.vesting.")
}

// isPermanentLockedAccountType returns true if the type name indicates a
// PermanentLockedAccount in proto or amino JSON.
func isPermanentLockedAccountType(typeName string) bool {
	lower := strings.ToLower(strings.TrimSpace(typeName))
	return lower == strings.ToLower(protoPermanentLockedAccountType) ||
		lower == strings.ToLower(legacyPermanentLockedAccountType) ||
		strings.Contains(lower, "permanentlockedaccount")
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

// queryMigrationStats queries the global migration statistics from the
// evmigration module via the shared common helper.
func queryMigrationStats() (migrationStats, error) {
	s, err := evmigCLI().MigrationStats()
	if err != nil {
		return migrationStats{}, err
	}
	return migrationStats{
		TotalMigrated:           s.TotalMigrated,
		TotalLegacy:             s.TotalLegacy,
		TotalLegacyStaked:       s.TotalLegacyStaked,
		TotalValidatorsMigrated: s.TotalValidatorsMigrated,
		TotalValidatorsLegacy:   s.TotalValidatorsLegacy,
	}, nil
}

// queryLegacyAccountAddresses returns the addresses of all accounts still in
// the chain's legacy-accounts set (i.e. not yet migrated).
func queryLegacyAccountAddresses() ([]string, error) {
	out, err := run("query", "evmigration", "legacy-accounts")
	if err != nil {
		return nil, fmt.Errorf("query legacy-accounts: %s\n%w", out, err)
	}
	var resp struct {
		Accounts []struct {
			Address string `json:"address"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse legacy-accounts: %s\n%w", truncate(out, 300), err)
	}
	addrs := make([]string, 0, len(resp.Accounts))
	for _, a := range resp.Accounts {
		addrs = append(addrs, a.Address)
	}
	return addrs, nil
}

// queryMigrationParams queries the evmigration module parameters via the shared
// common helper.
func queryMigrationParams() (migrationParams, error) {
	p, err := evmigCLI().MigrationParams()
	if err != nil {
		return migrationParams{}, err
	}
	return migrationParams{
		EnableMigration:         p.EnableMigration,
		MigrationEndTime:        p.MigrationEndTime,
		MaxMigrationsPerBlock:   p.MaxMigrationsPerBlock,
		MaxValidatorDelegations: p.MaxValidatorDelegations,
	}, nil
}

// Flexible JSON scalar parsing now lives in the shared common package
// (common/migration.go); the query helpers above delegate to it.
