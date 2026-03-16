package types

import (
	"cosmossdk.io/errors"
)

// x/evmigration module sentinel errors
var (
	ErrInvalidSigner              = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrMigrationDisabled          = errors.Register(ModuleName, 1101, "migration is disabled")
	ErrMigrationWindowClosed      = errors.Register(ModuleName, 1102, "migration window has closed")
	ErrBlockRateLimitExceeded     = errors.Register(ModuleName, 1103, "block migration rate limit exceeded")
	ErrSameAddress                = errors.Register(ModuleName, 1104, "legacy and new address must be different")
	ErrAlreadyMigrated            = errors.Register(ModuleName, 1105, "legacy address has already been migrated")
	ErrNewAddressWasMigrated      = errors.Register(ModuleName, 1106, "new address is a previously-migrated legacy address")
	ErrCannotMigrateModuleAccount = errors.Register(ModuleName, 1107, "cannot migrate a module account")
	ErrUseValidatorMigration      = errors.Register(ModuleName, 1108, "legacy address is a validator operator; use MsgMigrateValidator instead")
	ErrLegacyAccountNotFound      = errors.Register(ModuleName, 1109, "legacy account not found in x/auth")
	ErrInvalidLegacyPubKey        = errors.Register(ModuleName, 1110, "invalid legacy public key")
	ErrPubKeyAddressMismatch      = errors.Register(ModuleName, 1111, "legacy public key does not derive to legacy address")
	ErrInvalidLegacySignature     = errors.Register(ModuleName, 1112, "legacy signature verification failed")
	ErrNotValidator               = errors.Register(ModuleName, 1113, "legacy address is not a validator operator")
	ErrValidatorUnbonding         = errors.Register(ModuleName, 1114, "validator is unbonding or unbonded; wait for completion")
	ErrTooManyDelegators          = errors.Register(ModuleName, 1115, "validator has too many delegators; exceeds max_validator_delegations")
	ErrInvalidNewPubKey           = errors.Register(ModuleName, 1116, "invalid new public key")
	ErrNewPubKeyAddressMismatch   = errors.Register(ModuleName, 1117, "new public key does not derive to new address")
	ErrInvalidNewSignature        = errors.Register(ModuleName, 1118, "new signature verification failed")
)
