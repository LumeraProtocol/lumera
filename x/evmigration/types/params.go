package types

import "fmt"

var (
	DefaultEnableMigration         = false
	DefaultMigrationEndTime        int64  = 0
	DefaultMaxMigrationsPerBlock   uint64 = 50
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
