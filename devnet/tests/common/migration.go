package common

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// MigrationParams mirrors the governance-controlled x/evmigration params that
// gate the migration window and bound per-block work.
type MigrationParams struct {
	EnableMigration         bool
	MigrationEndTime        int64
	MaxMigrationsPerBlock   int
	MaxValidatorDelegations int
	MaxMultisigSubKeys      int
}

// MigrationEstimate is the result of an evmigration migration-estimate query for
// one legacy address.
type MigrationEstimate struct {
	WouldSucceed       bool
	RejectionReason    string
	DelegationCount    int
	UnbondingCount     int
	RedelegationCount  int
	AuthzGrantCount    int
	FeegrantCount      int
	ActionCount        int
	ValDelegationCount int
	IsValidator        bool
	IsMultisig         bool
	BalanceSummary     string
}

// MigrationRecord is an on-chain evmigration record mapping a legacy address to
// its new EVM-compatible address.
type MigrationRecord struct {
	LegacyAddress string
	NewAddress    string
	Height        int64
}

// MigrationStats is the global migration progress from the migration-stats query.
type MigrationStats struct {
	TotalMigrated           int
	TotalLegacy             int
	TotalLegacyStaked       int
	TotalValidatorsMigrated int
	TotalValidatorsLegacy   int
}

// MigrationStats queries the global migration statistics.
func (c *ChainCLI) MigrationStats() (MigrationStats, error) {
	out, err := c.Run("query", "evmigration", "migration-stats")
	if err != nil {
		return MigrationStats{}, fmt.Errorf("query migration-stats: %s: %w", truncate(out, 200), err)
	}
	return parseMigrationStats(out)
}

func parseMigrationStats(out string) (MigrationStats, error) {
	payload := out
	if extracted, ok := ExtractJSONPayload(out); ok {
		payload = extracted
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return MigrationStats{}, fmt.Errorf("parse migration-stats: %s: %w", truncate(payload, 200), err)
	}
	var s MigrationStats
	var err error
	if s.TotalMigrated, err = flexInt(raw["total_migrated"]); err != nil {
		return MigrationStats{}, fmt.Errorf("migration-stats.total_migrated: %w", err)
	}
	if s.TotalLegacy, err = flexInt(raw["total_legacy"]); err != nil {
		return MigrationStats{}, fmt.Errorf("migration-stats.total_legacy: %w", err)
	}
	if s.TotalLegacyStaked, err = flexInt(raw["total_legacy_staked"]); err != nil {
		return MigrationStats{}, fmt.Errorf("migration-stats.total_legacy_staked: %w", err)
	}
	if s.TotalValidatorsMigrated, err = flexInt(raw["total_validators_migrated"]); err != nil {
		return MigrationStats{}, fmt.Errorf("migration-stats.total_validators_migrated: %w", err)
	}
	if s.TotalValidatorsLegacy, err = flexInt(raw["total_validators_legacy"]); err != nil {
		return MigrationStats{}, fmt.Errorf("migration-stats.total_validators_legacy: %w", err)
	}
	return s, nil
}

// MigrationParams queries the evmigration module parameters.
func (c *ChainCLI) MigrationParams() (MigrationParams, error) {
	out, err := c.Run("query", "evmigration", "params")
	if err != nil {
		return MigrationParams{}, fmt.Errorf("query evmigration params: %s: %w", truncate(out, 200), err)
	}
	return parseMigrationParams(out)
}

// MigrationEstimate queries the migration estimate for a legacy address.
func (c *ChainCLI) MigrationEstimate(addr string) (MigrationEstimate, error) {
	out, err := c.Run("query", "evmigration", "migration-estimate", addr)
	if err != nil {
		return MigrationEstimate{}, fmt.Errorf("query migration-estimate %s: %s: %w", addr, truncate(out, 200), err)
	}
	return parseMigrationEstimate(out)
}

// MigrationRecord queries the migration record for a legacy address. The bool
// reports whether a record exists (false when the account is not yet migrated).
func (c *ChainCLI) MigrationRecord(legacyAddr string) (MigrationRecord, bool, error) {
	out, err := c.Run("query", "evmigration", "migration-record", legacyAddr)
	if err != nil {
		return MigrationRecord{}, false, fmt.Errorf("query migration-record %s: %s: %w", legacyAddr, truncate(out, 200), err)
	}
	return parseMigrationRecord(out)
}

// MigrationRecordByNewAddress queries the migration record by the new (EVM)
// address. The bool reports whether a record exists.
func (c *ChainCLI) MigrationRecordByNewAddress(newAddr string) (MigrationRecord, bool, error) {
	out, err := c.Run("query", "evmigration", "migration-record-by-new-address", newAddr)
	if err != nil {
		return MigrationRecord{}, false, fmt.Errorf("query migration-record-by-new-address %s: %s: %w", newAddr, truncate(out, 200), err)
	}
	return parseMigrationRecord(out)
}

func parseMigrationParams(out string) (MigrationParams, error) {
	payload := out
	if extracted, ok := ExtractJSONPayload(out); ok {
		payload = extracted
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &top); err != nil {
		return MigrationParams{}, fmt.Errorf("parse evmigration params: %s: %w", truncate(payload, 200), err)
	}
	// Params may be wrapped in a "params" envelope or returned bare.
	raw := top
	if inner, ok := top["params"]; ok && len(inner) > 0 {
		raw = map[string]json.RawMessage{}
		if err := json.Unmarshal(inner, &raw); err != nil {
			return MigrationParams{}, fmt.Errorf("parse evmigration params payload: %s: %w", truncate(string(inner), 200), err)
		}
	}

	var p MigrationParams
	var err error
	if p.EnableMigration, err = flexBool(raw["enable_migration"]); err != nil {
		return MigrationParams{}, fmt.Errorf("params.enable_migration: %w", err)
	}
	if p.MigrationEndTime, err = flexInt64(raw["migration_end_time"]); err != nil {
		return MigrationParams{}, fmt.Errorf("params.migration_end_time: %w", err)
	}
	if p.MaxMigrationsPerBlock, err = flexInt(raw["max_migrations_per_block"]); err != nil {
		return MigrationParams{}, fmt.Errorf("params.max_migrations_per_block: %w", err)
	}
	if p.MaxValidatorDelegations, err = flexInt(raw["max_validator_delegations"]); err != nil {
		return MigrationParams{}, fmt.Errorf("params.max_validator_delegations: %w", err)
	}
	if p.MaxMultisigSubKeys, err = flexInt(raw["max_multisig_sub_keys"]); err != nil {
		return MigrationParams{}, fmt.Errorf("params.max_multisig_sub_keys: %w", err)
	}
	return p, nil
}

func parseMigrationEstimate(out string) (MigrationEstimate, error) {
	payload := out
	if extracted, ok := ExtractJSONPayload(out); ok {
		payload = extracted
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return MigrationEstimate{}, fmt.Errorf("parse migration-estimate: %s: %w", truncate(payload, 200), err)
	}

	var est MigrationEstimate
	var err error
	if est.WouldSucceed, err = flexBool(raw["would_succeed"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.would_succeed: %w", err)
	}
	est.RejectionReason = flexString(raw["rejection_reason"])
	est.BalanceSummary = flexString(raw["balance_summary"])
	if est.DelegationCount, err = flexInt(raw["delegation_count"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.delegation_count: %w", err)
	}
	if est.UnbondingCount, err = flexInt(raw["unbonding_count"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.unbonding_count: %w", err)
	}
	if est.RedelegationCount, err = flexInt(raw["redelegation_count"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.redelegation_count: %w", err)
	}
	if est.AuthzGrantCount, err = flexInt(raw["authz_grant_count"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.authz_grant_count: %w", err)
	}
	if est.FeegrantCount, err = flexInt(raw["feegrant_count"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.feegrant_count: %w", err)
	}
	if est.ActionCount, err = flexInt(raw["action_count"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.action_count: %w", err)
	}
	if est.ValDelegationCount, err = flexInt(raw["val_delegation_count"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.val_delegation_count: %w", err)
	}
	if est.IsValidator, err = flexBool(raw["is_validator"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.is_validator: %w", err)
	}
	if est.IsMultisig, err = flexBool(raw["is_multisig"]); err != nil {
		return MigrationEstimate{}, fmt.Errorf("migration-estimate.is_multisig: %w", err)
	}
	return est, nil
}

func parseMigrationRecord(out string) (MigrationRecord, bool, error) {
	payload := out
	if extracted, ok := ExtractJSONPayload(out); ok {
		payload = extracted
	}
	var top struct {
		Record *struct {
			LegacyAddress   string          `json:"legacy_address"`
			NewAddress      string          `json:"new_address"`
			Height          json.RawMessage `json:"height"`
			MigrationHeight json.RawMessage `json:"migration_height"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(payload), &top); err != nil {
		return MigrationRecord{}, false, fmt.Errorf("parse migration-record: %s: %w", truncate(payload, 200), err)
	}
	if top.Record == nil || (top.Record.LegacyAddress == "" && top.Record.NewAddress == "") {
		return MigrationRecord{}, false, nil
	}
	heightRaw := top.Record.MigrationHeight
	if len(heightRaw) == 0 {
		heightRaw = top.Record.Height
	}
	height, err := flexInt64(heightRaw)
	if err != nil {
		return MigrationRecord{}, false, fmt.Errorf("migration-record.height: %w", err)
	}
	return MigrationRecord{
		LegacyAddress: top.Record.LegacyAddress,
		NewAddress:    top.Record.NewAddress,
		Height:        height,
	}, true, nil
}

// --- Flexible JSON scalar parsers ---
// Cosmos SDK query output is inconsistent across versions: numeric fields may
// appear as JSON numbers or quoted strings. These helpers accept both.

func flexInt(raw json.RawMessage) (int, error) {
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
	return 0, fmt.Errorf("unsupported numeric format: %s", string(raw))
}

func flexInt64(raw json.RawMessage) (int64, error) {
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
	return 0, fmt.Errorf("unsupported numeric format: %s", string(raw))
}

func flexBool(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return false, nil
	}
	var asBool bool
	if err := json.Unmarshal(raw, &asBool); err == nil {
		return asBool, nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		switch strings.TrimSpace(strings.ToLower(asString)) {
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

func flexString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}
	return strings.TrimSpace(string(raw))
}
