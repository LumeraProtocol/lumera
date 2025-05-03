package types

const (
	// Event Types
	EventTypeActionRegistered = "action_registered"
	EventTypeActionFinalized  = "action_finalized"
	EventTypeActionApproved   = "action_approved"
	EventTypeActionFailed     = "action_failed"
	EventTypeActionExpired    = "action_expired"

	// Common Attributes
	AttributeKeyActionID   = "action_id"
	AttributeKeyCreator    = "creator"
	AttributeKeySuperNodes = "supernodes"
	AttributeKeyActionType = "action_type"
	AttributeKeyResults    = "results"
	AttributeKeyFee        = "fee"
	AttributeKeyError      = "error"
)
