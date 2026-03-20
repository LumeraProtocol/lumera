package types

// Event types for the evmigration module.
const (
	EventTypeClaimLegacyAccount = "claim_legacy_account"
	EventTypeMigrateValidator   = "migrate_validator"

	AttributeKeyLegacyAddress = "legacy_address"
	AttributeKeyNewAddress    = "new_address"
	AttributeKeyMigrationTime = "migration_time"
	AttributeKeyBlockHeight   = "block_height"
	AttributeKeyOldValAddr    = "old_val_addr"
	AttributeKeyNewValAddr    = "new_val_addr"
	AttributeKeyConsAddr      = "cons_addr"
)
