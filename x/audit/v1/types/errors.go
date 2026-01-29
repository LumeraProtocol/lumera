package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/audit module sentinel errors
var (
	ErrInvalidSigner       = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrInvalidEvidenceType = errors.Register(ModuleName, 1101, "invalid evidence type")
	ErrInvalidMetadata     = errors.Register(ModuleName, 1102, "invalid evidence metadata")
	ErrInvalidSubject      = errors.Register(ModuleName, 1103, "invalid subject address")
	ErrInvalidReporter     = errors.Register(ModuleName, 1104, "invalid reporter address")
	ErrInvalidActionID     = errors.Register(ModuleName, 1105, "invalid action id")
)
