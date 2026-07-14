package types

// Event types for the evmigration module.
const (
	EventTypeClaimLegacyAccount = "claim_legacy_account"
	EventTypeMigrateValidator   = "migrate_validator"

	// EventTypeV120StakeRepair is emitted per DelegatorStartingInfo row
	// repaired at migration time by RepairV120DistributionStake. The row was
	// corrupted by the v1.20.0 evmigration bug that wrote raw shares into
	// distribution's Stake field (v1.20.1 hotfix 4ce27cf0 fixed the writer;
	// this event surfaces the repair of pre-hotfix state at migration time).
	EventTypeV120StakeRepair = "evmigration_v120_stake_repair"

	AttributeKeyLegacyAddress = "legacy_address"
	AttributeKeyNewAddress    = "new_address"
	AttributeKeyMigrationTime = "migration_time"
	AttributeKeyBlockHeight   = "block_height"
	AttributeKeyOldValAddr    = "old_val_addr"
	AttributeKeyNewValAddr    = "new_val_addr"
	AttributeKeyConsAddr      = "cons_addr"

	// Attributes for EventTypeV120StakeRepair.
	AttributeKeyDelegatorAddress = "delegator_address"
	AttributeKeyValidatorAddress = "validator_address"
	AttributeKeyOldStake         = "old_stake"
	AttributeKeyNewStake         = "new_stake"
	AttributeKeyDelegationShares = "delegation_shares"
)
