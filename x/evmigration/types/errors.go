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

	ErrInvalidMigrationPubKey    = errors.Register(ModuleName, 1110, "invalid public key in migration proof")
	ErrPubKeyAddressMismatch     = errors.Register(ModuleName, 1111, "public key does not derive to claimed address")
	ErrInvalidMigrationSignature = errors.Register(ModuleName, 1112, "migration signature verification failed")

	ErrNotValidator       = errors.Register(ModuleName, 1113, "legacy address is not a validator operator")
	ErrValidatorUnbonding = errors.Register(ModuleName, 1114, "validator is unbonding; wait for the unbonding period to complete, then migrate")
	ErrTooManyDelegators  = errors.Register(ModuleName, 1115, "validator has too many delegators; exceeds max_validator_delegations")

	// Migration to a destination account of an unsupported type (vesting,
	// smart/contract, any non-plain-BaseAccount). Rejected at MigrateAuth
	// Phase 1 type-safety check because FinalizeVestingAccount would silently
	// clobber pre-existing special-type state.
	ErrInvalidMigrationDestination = errors.Register(ModuleName, 1116, "invalid migration destination account type")

	ErrNewAddressIsValidator = errors.Register(ModuleName, 1117, "new address is already a validator operator")

	// Code 1118 left intentionally unregistered — reclaimed from the
	// side-specific ErrNewPubKeyAddressMismatch / ErrInvalidNewSignature which
	// no longer exist. Do not reuse these codes in this module.

	ErrNewAddressAlreadyUsed = errors.Register(ModuleName, 1119, "new address was already used as a migration destination")
	ErrInvalidMigrationProof = errors.Register(ModuleName, 1120, "invalid migration proof")

	// Mirror-source rule: legacy and new proofs must share shape (both single
	// or both multisig); when both multisig, threshold (K) and sub-key count
	// (N) must match. A 2-of-3 legacy must migrate to a 2-of-3 destination.
	ErrMirrorSourceMismatch = errors.Register(ModuleName, 1121, "legacy and new proofs violate the mirror-source rule")

	// Distribution state inconsistency that does NOT match the known v1.20.0
	// raw-shares bug signature. Returned by AssertDistributionStakeSane when
	// stored DelegatorStartingInfo.Stake still exceeds the token-denominated
	// expected value after a repair attempt. This branch should be unreachable
	// on any chain that has ever executed v1.20.0 or later; it exists to
	// fail-closed rather than proceed into a chain-recovery-caught panic.
	ErrDistributionStateInconsistent = errors.Register(ModuleName, 1122, "distribution state inconsistency: stored delegation stake exceeds token-denominated value")
)
