package types

// Event types and attributes for storage-truth score updates.
const (
	EventTypeStorageTruthScoreUpdated = "storage_truth_score_updated"
	EventTypeHealOpScheduled          = "storage_truth_heal_op_scheduled"
	EventTypeHealOpExpired            = "storage_truth_heal_op_expired"
	EventTypeHealOpHealerReported     = "storage_truth_heal_op_healer_reported"
	EventTypeHealOpVerified           = "storage_truth_heal_op_verified"
	EventTypeHealOpFailed             = "storage_truth_heal_op_failed"
	EventTypeStorageRecheckEvidence   = "storage_truth_recheck_evidence_submitted"

	AttributeKeyEpochID                  = "epoch_id"
	AttributeKeyReporterSupernodeAccount = "reporter_supernode_account"
	AttributeKeyTargetSupernodeAccount   = "target_supernode_account"
	AttributeKeyTicketID                 = "ticket_id"
	AttributeKeyHealOpID                 = "heal_op_id"
	AttributeKeyVerifierSupernodeAccount = "verifier_supernode_account"
	AttributeKeyHealerSupernodeAccount   = "healer_supernode_account"
	AttributeKeyVerified                 = "verified"
	AttributeKeyVerificationHash         = "verification_hash"
	AttributeKeyTranscriptHash           = "transcript_hash"
	AttributeKeyDeadlineEpochID          = "deadline_epoch_id"
	AttributeKeyResultClass              = "result_class"
	AttributeKeyBucketType               = "bucket_type"
	AttributeKeyNodeSuspicionScore       = "node_suspicion_score"
	AttributeKeyReporterReliabilityScore = "reporter_reliability_score"
	AttributeKeyTicketDeteriorationScore = "ticket_deterioration_score"
	AttributeKeyReporterTrustBand        = "reporter_trust_band"
	AttributeKeyRepeatedFailureCount     = "repeated_failure_count"
	AttributeKeyContradictionDetected    = "contradiction_detected"
	AttributeKeyContradictedReporter     = "contradicted_reporter"
)
