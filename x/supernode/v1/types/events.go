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
	AttributeKeyReason           = "reason"
	AttributeKeyOldAccount       = "old_account"
	AttributeKeyNewAccount       = "new_account"
	AttributeKeySupernodeAccount = "supernode_account"
	AttributeKeyOldP2PPort       = "old_p2p_port"
	AttributeKeyP2PPort          = "p2p_port"
	AttributeKeyReRegistered     = "re_registered"
	AttributeKeyOldState         = "old_state"
	AttributeKeyOldIPAddress     = "old_ip_address"
	AttributeKeyHeight           = "height"
	AttributeKeyFieldsUpdated    = "fields_updated"
)
