package types

const (
	// Event Types
	EventTypeActionRegistered           = "action_registered"
	EventTypeActionFinalized            = "action_finalized"
	EventTypeActionFinalizationRejected = "action_finalization_rejected"
	EventTypeActionApproved             = "action_approved"
	EventTypeActionFailed               = "action_failed"
	EventTypeActionExpired              = "action_expired"
	EventTypeSVCEvidence                = "svc_verification_failed_evidence"
	EventTypeSVCVerificationPassed      = "svc_verification_passed"

	// Common Attributes
	AttributeKeyActionID           = "action_id"
	AttributeKeyCreator            = "creator"
	AttributeKeyFinalizer          = "finalizer"
	AttributeKeySuperNodes         = "supernodes"
	AttributeKeyActionType         = "action_type"
	AttributeKeyResults            = "results"
	AttributeKeyFee                = "fee"
	AttributeKeyError              = "error"
	AttributeKeyEvidenceID         = "evidence_id"
	AttributeKeyProofIndex         = "proof_index"
	AttributeKeyChunkIndex         = "chunk_index"
	AttributeKeyExpectedChunkIndex = "expected_chunk_index"
)
