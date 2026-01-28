package types

import (
	errorsmod "cosmossdk.io/errors"
)

var (
	ErrInvalidSigner           = errorsmod.Register(ModuleName, 1, "invalid signer")
	ErrInvalidWindowID         = errorsmod.Register(ModuleName, 2, "invalid window id")
	ErrWindowSnapshotNotFound  = errorsmod.Register(ModuleName, 3, "window snapshot not found")
	ErrDuplicateReport         = errorsmod.Register(ModuleName, 4, "duplicate report")
	ErrInvalidPeerObservations = errorsmod.Register(ModuleName, 5, "invalid peer observations")
	ErrInvalidPortStatesLength = errorsmod.Register(ModuleName, 6, "invalid port states length")
	ErrReporterNotFound        = errorsmod.Register(ModuleName, 7, "reporter supernode not found")
	ErrInvalidReporterState    = errorsmod.Register(ModuleName, 8, "invalid reporter state")
	ErrInvalidWindowSnapshot   = errorsmod.Register(ModuleName, 9, "invalid window snapshot")
)
