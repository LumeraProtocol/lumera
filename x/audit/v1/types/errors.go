package types

import (
	errorsmod "cosmossdk.io/errors"
)

var (
	ErrInvalidSigner           = errorsmod.Register(ModuleName, 1, "invalid signer")
	ErrInvalidEpochID          = errorsmod.Register(ModuleName, 2, "invalid epoch id")
	ErrDuplicateReport         = errorsmod.Register(ModuleName, 4, "duplicate report")
	ErrInvalidPeerObservations = errorsmod.Register(ModuleName, 5, "invalid peer observations")
	ErrInvalidPortStatesLength = errorsmod.Register(ModuleName, 6, "invalid port states length")
	ErrReporterNotFound        = errorsmod.Register(ModuleName, 7, "reporter supernode not found")
	ErrInvalidReporterState    = errorsmod.Register(ModuleName, 8, "invalid reporter state")

	ErrInvalidEvidenceType = errorsmod.Register(ModuleName, 1101, "invalid evidence type")
	ErrInvalidMetadata     = errorsmod.Register(ModuleName, 1102, "invalid evidence metadata")
	ErrInvalidSubject      = errorsmod.Register(ModuleName, 1103, "invalid subject address")
	ErrInvalidReporter     = errorsmod.Register(ModuleName, 1104, "invalid reporter address")
	ErrInvalidActionID     = errorsmod.Register(ModuleName, 1105, "invalid action id")
)
