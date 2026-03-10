package types

import sdkerrors "cosmossdk.io/errors"

var (
	ErrInvalidParams = sdkerrors.Register(ModuleName, 1100, "invalid everlight params")
)
