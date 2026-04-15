package types

// Event types and attributes for storage-truth score updates.
const (
	EventTypeStorageTruthScoreUpdated = "storage_truth_score_updated"

	AttributeKeyEpochID                  = "epoch_id"
	AttributeKeyReporterSupernodeAccount = "reporter_supernode_account"
	AttributeKeyTargetSupernodeAccount   = "target_supernode_account"
	AttributeKeyTicketID                 = "ticket_id"
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
