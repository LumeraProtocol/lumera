package types

import (
	errorsmod "cosmossdk.io/errors"
)

// Register error codes for the action module
var (
	ErrActionExpired      = errorsmod.Register(ModuleName, 1, "action expired")
	ErrInvalidActionType  = errorsmod.Register(ModuleName, 2, "invalid action type")
	ErrActionNotFound     = errorsmod.Register(ModuleName, 3, "action not found")
	ErrInvalidMetadata    = errorsmod.Register(ModuleName, 4, "invalid metadata")
	ErrInvalidActionState = errorsmod.Register(ModuleName, 5, "invalid action state")
	ErrDuplicateAction    = errorsmod.Register(ModuleName, 6, "duplicate action")
	ErrInvalidSignature   = errorsmod.Register(ModuleName, 7, "invalid signature")
	ErrInternalError      = errorsmod.Register(ModuleName, 8, "internal error occurred")
	ErrInvalidID          = errorsmod.Register(ModuleName, 9, "invalid ID")
	ErrUnauthorizedSN     = errorsmod.Register(ModuleName, 10, "unauthorized supernode")
	ErrInvalidExpiration  = errorsmod.Register(ModuleName, 11, "invalid expiration time")
	ErrInvalidPrice       = errorsmod.Register(ModuleName, 12, "invalid price")
	ErrInvalidAddress     = errorsmod.Register(ModuleName, 13, "invalid address")
	ErrFinalizationError  = errorsmod.Register(ModuleName, 14, "finalization error")
	ErrInvalidFileSize    = errorsmod.Register(ModuleName, 15, "invalid file size")
	ErrInvalidSigner      = errorsmod.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrInvalidPacketTimeout = errorsmod.Register(ModuleName, 1500, "invalid packet timeout")
	ErrInvalidVersion       = errorsmod.Register(ModuleName, 1501, "invalid version")
)
