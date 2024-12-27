package types

// DONTCOVER

import (
	sdkerrors "cosmossdk.io/errors"
)

// x/supernode module sentinel errors
var (
	ErrInvalidSigner = sdkerrors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrSample        = sdkerrors.Register(ModuleName, 1101, "sample error")

	ErrEmptyIPAddress        = sdkerrors.Register(ModuleName, 1102, "ip address cannot be empty")
	ErrInvalidSuperNodeState = sdkerrors.Register(ModuleName, 1103, "invalid supernode state")
	ErrEmptyVersion          = sdkerrors.Register(ModuleName, 1104, "version cannot be empty")
	ErrValidatorNotFound     = sdkerrors.Register(ModuleName, 1105, "validator not found")

	ErrEmptyEvidenceType           = sdkerrors.Register(ModuleName, 1106, "evidence type cannot be empty")
	ErrEmptyDescription            = sdkerrors.Register(ModuleName, 1107, "evidence description cannot be empty")
	ErrSupernodeAccountUnspecified = sdkerrors.Register(ModuleName, 1108, "supernode account unspecified")
	ErrInvalidSupernodeAddress     = sdkerrors.Register(ModuleName, 1109, "invalid supernode address")
)
