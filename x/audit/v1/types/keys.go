package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "audit"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	// It should be synced with the gov module's name if it is ever changed.
	// See: https://github.com/cosmos/cosmos-sdk/blob/v0.52.0-beta.2/x/gov/types/keys.go#L9
	GovModuleName = "gov"
)

// ParamsKey is the prefix to retrieve all Params
var ParamsKey = collections.NewPrefix("p_audit")

// EvidenceIDKey is the prefix for the evidence id sequence.
var EvidenceIDKey = collections.NewPrefix(1)

// EvidenceKey is the prefix for stored Evidence records.
var EvidenceKey = collections.NewPrefix(2)

// EvidenceBySubjectKey is the prefix for the secondary index (subject_address, evidence_id).
var EvidenceBySubjectKey = collections.NewPrefix(3)

// EvidenceByActionKey is the prefix for the secondary index (action_id, evidence_id).
var EvidenceByActionKey = collections.NewPrefix(4)
