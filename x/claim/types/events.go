package types

// Event types and attributes for the claim module
const (
	// MsgClaim Events
	EventTypeClaimProcessed = "claim_processed"

	AttributeKeyOldAddress = "old_address"
	AttributeKeyNewAddress = "new_address"
	AttributeKeyClaimTime  = "claim_time"

	// Module events
	EventTypeClaimPeriodEnd = "claim_period_end"
	AttributeKeyEndTime     = "end_time"
)
