package types

// DONTCOVER

import (
	sdkerrors "cosmossdk.io/errors"
)

// x/pastelid module sentinel errors
var (
	ErrInvalidSigner = sdkerrors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrSample        = sdkerrors.Register(ModuleName, 1101, "sample error")

	ErrPastelIDExists        = sdkerrors.Register(ModuleName, 1, "address already has a PastelID")
	ErrInvalidSignature      = sdkerrors.Register(ModuleName, 2, "invalid signature")
	ErrSecureContainerFailed = sdkerrors.Register(ModuleName, 3, "failed to create secure container")
	ErrInvalidAddress        = sdkerrors.Register(ModuleName, 4, "invalid address")
	ErrInsufficientFunds     = sdkerrors.Register(ModuleName, 5, "insufficient funds for PastelID creation")
)
