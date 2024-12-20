package types

// DONTCOVER

import (
	sdkerrors "cosmossdk.io/errors"
)

// x/claim module sentinel errors
var (
	ErrInvalidSigner             = sdkerrors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrClaimDisabled             = sdkerrors.Register(ModuleName, 1101, "claim is disabled")
	ErrTooManyClaims             = sdkerrors.Register(ModuleName, 1102, "too many claims in a block")
	ErrClaimPeriodExpired        = sdkerrors.Register(ModuleName, 1103, "claim period has expired")
	ErrClaimNotFound             = sdkerrors.Register(ModuleName, 1104, "claim not found")
	ErrClaimAlreadyClaimed       = sdkerrors.Register(ModuleName, 1105, "claim already claimed")
	ErrInvalidPubKey             = sdkerrors.Register(ModuleName, 1106, "invalid public key")
	ErrMismatchReconstructedAddr = sdkerrors.Register(ModuleName, 1107, "reconstructed address does not match old address")
	ErrInvalidSignature          = sdkerrors.Register(ModuleName, 1108, "invalid signature")
	ErrInvalidParamMaxClaims     = sdkerrors.Register(ModuleName, 1109, "invalid max claims per block")
	ErrInvalidParamClaimDuration = sdkerrors.Register(ModuleName, 1110, "invalid claim duration")
)
