package crossruntime

import "errors"

var (
	// ErrReentrancyNotAllowed is returned when a cross-runtime call would exceed
	// the maximum nesting depth.
	ErrReentrancyNotAllowed = errors.New("cross-runtime reentrancy not allowed (max depth reached)")
)
