package types

// Event types for the supernode module
const (
	EventTypeSupernodeRegistered   = "supernode_registered"
	EventTypeSupernodeDeRegistered = "supernode_deregistered"
	EventTypeSupernodeStarted      = "supernode_started"
	EventTypeSupernodeStopped      = "supernode_stopped"
	EventTypeSupernodeUpdated      = "supernode_updated"

	AttributeKeyValidatorAddress = "validator_address"
	AttributeKeyIPAddress        = "ip_address"
	AttributeKeyVersion          = "version"
	AttributeKeyReason           = "reason"
	AttributeKeyOldAccount       = "old_account"
	AttributeKeyNewAccount       = "new_account"
)
