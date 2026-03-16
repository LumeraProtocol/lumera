// Package types defines the parameter set for the evmigration module.
//
// The evmigration module manages the migration of legacy (pre-EVM) chain state
// — accounts, delegations, and validators — onto the new EVM-enabled chain.
// Its parameters act as governance-controlled knobs that determine when
// migrations are accepted and how much work the chain performs per block.
//
// # Parameters
//
// EnableMigration (bool, default: true)
//
//	Master switch. When false the module rejects every MsgClaimLegacyAccount
//	and MsgMigrateValidator regardless of other parameter values. Governance
//	should flip this to false once the migration window closes.
//
// MigrationEndTime (int64 unix seconds, default: 0 — no deadline)
//
//	Optional hard deadline. If non-zero, any migration message whose block
//	time is after this timestamp is rejected. A value of 0 disables the
//	deadline, leaving EnableMigration as the sole on/off control.
//
// MaxMigrationsPerBlock (uint64, default: 50)
//
//	Throttle for MsgClaimLegacyAccount messages. The keeper tracks how many
//	claim messages have been processed in the current block; once this limit
//	is reached, additional claims in the same block are rejected. This
//	prevents a burst of migrations from consuming excessive block gas.
//
// MaxValidatorDelegations (uint64, default: 2000)
//
//	Safety cap for MsgMigrateValidator. A validator migration must re-key
//	every delegation record. If the total number of delegation + unbonding
//	records exceeds this threshold the message is rejected, because the
//	gas cost of iterating over all records would be prohibitive. Validators
//	that exceed the cap must shed delegations before migrating.
package types

import "fmt"

var (
	// DefaultEnableMigration is the default value for the EnableMigration param.
	DefaultEnableMigration = true
	// DefaultMigrationEndTime of 0 means no deadline is enforced.
	DefaultMigrationEndTime int64 = 0
	// DefaultMaxMigrationsPerBlock caps claim messages per block.
	DefaultMaxMigrationsPerBlock uint64 = 50
	// DefaultMaxValidatorDelegations caps delegation records for validator migration.
	DefaultMaxValidatorDelegations uint64 = 2000
)

// NewParams creates a new Params instance.
func NewParams(
	enableMigration bool,
	migrationEndTime int64,
	maxMigrationsPerBlock uint64,
	maxValidatorDelegations uint64,
) Params {
	return Params{
		EnableMigration:         enableMigration,
		MigrationEndTime:        migrationEndTime,
		MaxMigrationsPerBlock:   maxMigrationsPerBlock,
		MaxValidatorDelegations: maxValidatorDelegations,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(
		DefaultEnableMigration,
		DefaultMigrationEndTime,
		DefaultMaxMigrationsPerBlock,
		DefaultMaxValidatorDelegations,
	)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if p.MaxMigrationsPerBlock == 0 {
		return fmt.Errorf("max_migrations_per_block must be positive")
	}
	if p.MaxValidatorDelegations == 0 {
		return fmt.Errorf("max_validator_delegations must be positive")
	}
	return nil
}
