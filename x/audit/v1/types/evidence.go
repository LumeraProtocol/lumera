package types

import "strings"

const (
	// EvidenceTypeActionExpired is evidence that an action expired.
	EvidenceTypeActionExpired = "ACTION_EXPIRED"

	// EvidenceTypeActionWrongFinalizer is evidence that an unexpected actor attempted action finalization.
	EvidenceTypeActionWrongFinalizer = "ACTION_WRONG_FINALIZER"
)

func CanonicalEvidenceType(t string) string {
	return strings.ToUpper(strings.TrimSpace(t))
}

