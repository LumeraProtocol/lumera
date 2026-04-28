package keeper

import (
	"encoding/binary"
	"encoding/json"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

type storageProofTranscriptRecord struct {
	EpochID                  uint64   `json:"epoch_id"`
	ReporterAccount          string   `json:"reporter_account"`
	TargetAccount            string   `json:"target_account"`
	TicketID                 string   `json:"ticket_id"`
	ResultClass              int32    `json:"result_class"`
	BucketType               int32    `json:"bucket_type"`
	ArtifactClass            int32    `json:"artifact_class"`
	ArtifactKey              string   `json:"artifact_key,omitempty"`
	ArtifactOrdinal          uint32   `json:"artifact_ordinal,omitempty"`
	ArtifactCount            uint32   `json:"artifact_count,omitempty"`
	DerivationInputHash      string   `json:"derivation_input_hash,omitempty"`
	ChallengerSignature      string   `json:"challenger_signature,omitempty"`
	ObserverAttestations     []string `json:"observer_attestation_signatures,omitempty"`
	RecheckEligible          bool     `json:"recheck_eligible"`
	ConfirmedByRecheck       bool     `json:"confirmed_by_recheck,omitempty"`
	ContradictedByRecheck    bool     `json:"contradicted_by_recheck,omitempty"`
	RecheckTranscriptHash    string   `json:"recheck_transcript_hash,omitempty"`
	ChallengedTranscriptHash string   `json:"challenged_transcript_hash,omitempty"`
}

type storageTruthNodeFailureRecord struct {
	EpochID       uint64 `json:"epoch_id"`
	Reporter      string `json:"reporter"`
	Target        string `json:"target"`
	TicketID      string `json:"ticket_id"`
	ResultClass   int32  `json:"result_class"`
	BucketType    int32  `json:"bucket_type"`
	ArtifactClass int32  `json:"artifact_class"`
}

type storageTruthReporterResultRecord struct {
	EpochID             uint64 `json:"epoch_id"`
	Reporter            string `json:"reporter"`
	Target              string `json:"target"`
	TicketID            string `json:"ticket_id"`
	ResultClass         int32  `json:"result_class"`
	ConfirmedByRecheck  bool   `json:"confirmed_by_recheck,omitempty"`
	OverturnedByRecheck bool   `json:"overturned_by_recheck,omitempty"`
}

func (k Keeper) indexStorageProofTranscripts(ctx sdk.Context, epochID uint64, reporterAccount string, results []*types.StorageProofResult) error {
	for _, result := range results {
		if result == nil || result.TranscriptHash == "" {
			continue
		}
		record := storageProofTranscriptRecord{
			EpochID:              epochID,
			ReporterAccount:      reporterAccount,
			TargetAccount:        result.TargetSupernodeAccount,
			TicketID:             result.TicketId,
			ResultClass:          int32(result.ResultClass),
			BucketType:           int32(result.BucketType),
			ArtifactClass:        int32(result.ArtifactClass),
			ArtifactKey:          result.ArtifactKey,
			ArtifactOrdinal:      result.ArtifactOrdinal,
			ArtifactCount:        result.ArtifactCount,
			DerivationInputHash:  result.DerivationInputHash,
			ChallengerSignature:  result.ChallengerSignature,
			ObserverAttestations: append([]string(nil), result.ObserverAttestationSignatures...),
			RecheckEligible:      isStorageTruthRecheckEligible(result.ResultClass),
		}
		if err := k.setStorageProofTranscriptRecord(ctx, result.TranscriptHash, record); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) IndexStorageProofTranscripts(ctx sdk.Context, epochID uint64, reporterAccount string, results []*types.StorageProofResult) error {
	return k.indexStorageProofTranscripts(ctx, epochID, reporterAccount, results)
}

func (k Keeper) setStorageProofTranscriptRecord(ctx sdk.Context, transcriptHash string, record storageProofTranscriptRecord) error {
	bz, err := json.Marshal(record)
	if err != nil {
		return err
	}
	store := k.kvStore(ctx)
	store.Set(types.StorageProofTranscriptKey(transcriptHash), bz)
	// Per 122-Copilot-4/5 + 122-F1 — indexed lookup avoids DeliverTx full-table scan.
	if record.TargetAccount != "" {
		store.Set(types.TranscriptByTargetBucketEpochKey(record.TargetAccount, uint32(record.BucketType), record.EpochID, transcriptHash), bz)
	}
	return nil
}

func (k Keeper) getStorageProofTranscriptRecord(ctx sdk.Context, transcriptHash string) (storageProofTranscriptRecord, bool, error) {
	bz := k.kvStore(ctx).Get(types.StorageProofTranscriptKey(transcriptHash))
	if bz == nil {
		return storageProofTranscriptRecord{}, false, nil
	}
	var record storageProofTranscriptRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return storageProofTranscriptRecord{}, false, err
	}
	return record, true, nil
}

func (k Keeper) setStorageTruthNodeFailure(ctx sdk.Context, epochID uint64, reporterAccount string, result *types.StorageProofResult) error {
	if result == nil || result.TargetSupernodeAccount == "" || result.TicketId == "" || !isStorageTruthFailureClass(result.ResultClass) {
		return nil
	}
	record := storageTruthNodeFailureRecord{
		EpochID:       epochID,
		Reporter:      reporterAccount,
		Target:        result.TargetSupernodeAccount,
		TicketID:      result.TicketId,
		ResultClass:   int32(result.ResultClass),
		BucketType:    int32(result.BucketType),
		ArtifactClass: int32(result.ArtifactClass),
	}
	bz, err := json.Marshal(record)
	if err != nil {
		return err
	}
	k.kvStore(ctx).Set(types.NodeStorageTruthFailureKey(result.TargetSupernodeAccount, epochID, result.TicketId, reporterAccount), bz)
	return nil
}

func (k Keeper) setStorageTruthReporterResult(ctx sdk.Context, epochID uint64, reporterAccount string, result *types.StorageProofResult) error {
	if result == nil || reporterAccount == "" || result.TicketId == "" || result.TargetSupernodeAccount == "" {
		return nil
	}
	record := storageTruthReporterResultRecord{
		EpochID:     epochID,
		Reporter:    reporterAccount,
		Target:      result.TargetSupernodeAccount,
		TicketID:    result.TicketId,
		ResultClass: int32(result.ResultClass),
	}
	bz, err := json.Marshal(record)
	if err != nil {
		return err
	}
	store := k.kvStore(ctx)
	store.Set(types.ReporterStorageTruthResultKey(reporterAccount, epochID, result.TicketId, result.TargetSupernodeAccount), bz)
	// Per 122-Copilot-3 + 122-F1 — indexed lookup avoids DeliverTx full-table scan.
	store.Set(types.ReporterStorageTruthResultByTargetKey(result.TargetSupernodeAccount, epochID, result.TicketId, reporterAccount), bz)
	return nil
}

func (k Keeper) markStorageTruthReporterResultRecheck(ctx sdk.Context, reporterAccount string, transcriptHash string, confirmed bool) error {
	record, found, err := k.getStorageProofTranscriptRecord(ctx, transcriptHash)
	if err != nil || !found {
		return err
	}
	store := k.kvStore(ctx)
	resultBz := store.Get(types.ReporterStorageTruthResultKey(reporterAccount, record.EpochID, record.TicketID, record.TargetAccount))
	if resultBz != nil {
		var resultRecord storageTruthReporterResultRecord
		if err := json.Unmarshal(resultBz, &resultRecord); err != nil {
			return err
		}
		resultRecord.ConfirmedByRecheck = confirmed
		resultRecord.OverturnedByRecheck = !confirmed
		bz, err := json.Marshal(resultRecord)
		if err != nil {
			return err
		}
		store.Set(types.ReporterStorageTruthResultKey(reporterAccount, record.EpochID, record.TicketID, record.TargetAccount), bz)
		// Per 122-Copilot-3 + 122-F1 — keep secondary index in sync.
		store.Set(types.ReporterStorageTruthResultByTargetKey(record.TargetAccount, record.EpochID, record.TicketID, reporterAccount), bz)
	}
	record.ConfirmedByRecheck = confirmed
	record.ContradictedByRecheck = !confirmed
	if err := k.setStorageProofTranscriptRecord(ctx, transcriptHash, record); err != nil {
		return err
	}
	return nil
}

// linkStorageTruthRecheckTranscript wires a recheck transcript onto the
// challenged transcript so subsequent reads see the link. Per NEW-B-5 and
// the LEP-6 implementation guide § Recheck Evidence § Replay protection key:
//
//   - The recheck-evidence dedup key (st/rce/<...>) enforces single-witness
//     uniqueness PER CREATOR. A given recheck observer cannot replay the same
//     (epoch_id, ticket_id, target_account) tuple twice — that's the per-creator
//     single-witness invariant enforced at link time.
//   - Cross-creator quorum (multiple distinct recheck observers reaching the
//     same conclusion) is enforced LATER in scoring at epoch end, not here.
//     The link function deliberately does NOT count witnesses; it just records
//     the wiring.
//   - 122-F3 collision check below ensures a recheck-transcript hash cannot
//     be re-bound to a different challenged transcript later.
//
// Therefore "single-witness recheck" at this layer is intentional and not
// a missing-quorum bug.
func (k Keeper) linkStorageTruthRecheckTranscript(
	ctx sdk.Context,
	challengedTranscriptHash string,
	recheckTranscriptHash string,
	recheckerAccount string,
	recheckResultClass types.StorageProofResultClass,
) error {
	challenged, found, err := k.getStorageProofTranscriptRecord(ctx, challengedTranscriptHash)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if challenged.RecheckTranscriptHash != "" && challenged.RecheckTranscriptHash != recheckTranscriptHash {
		return errorsmod.Wrapf(types.ErrInvalidRecheckEvidence, "challenged transcript %q already linked to recheck transcript %q", challengedTranscriptHash, challenged.RecheckTranscriptHash)
	}

	challenged.RecheckTranscriptHash = recheckTranscriptHash
	if err := k.setStorageProofTranscriptRecord(ctx, challengedTranscriptHash, challenged); err != nil {
		return err
	}

	if existing, exists, err := k.getStorageProofTranscriptRecord(ctx, recheckTranscriptHash); err != nil {
		return err
	} else if exists {
		// Ensure the stored record was created by the same recheck request (122-F3).
		if existing.ChallengedTranscriptHash != challengedTranscriptHash ||
			existing.ReporterAccount != recheckerAccount {
			return errorsmod.Wrapf(types.ErrInvalidRecheckEvidence,
				"recheck transcript hash %q already recorded for a different challenged transcript or reporter",
				recheckTranscriptHash)
		}
		return nil
	}

	// Synthetic recheck record: omit challenger-specific derivation fields
	// that belong to the challenged result only (122-Copilot-2).
	recheckRecord := storageProofTranscriptRecord{
		EpochID:                  challenged.EpochID,
		ReporterAccount:          recheckerAccount,
		TargetAccount:            challenged.TargetAccount,
		TicketID:                 challenged.TicketID,
		ResultClass:              int32(recheckResultClass),
		BucketType:               int32(types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK),
		ArtifactClass:            challenged.ArtifactClass,
		ArtifactKey:              challenged.ArtifactKey,
		ArtifactOrdinal:          challenged.ArtifactOrdinal,
		ArtifactCount:            challenged.ArtifactCount,
		RecheckEligible:          false,
		ChallengedTranscriptHash: challengedTranscriptHash,
	}
	return k.setStorageProofTranscriptRecord(ctx, recheckTranscriptHash, recheckRecord)
}

func (k Keeper) distinctNodeFailedTickets(ctx sdk.Context, supernodeAccount string, startEpoch uint64, endEpoch uint64, include func(storageTruthNodeFailureRecord) bool) (map[string]struct{}, uint32, error) {
	tickets := make(map[string]struct{})
	var events uint32
	// Bounded epoch scan per CP-NEW-A-11 residue.
	start, end := types.NodeStorageTruthFailureEpochScanRange(supernodeAccount, startEpoch, endEpoch)
	it := k.kvStore(ctx).Iterator(start, end)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		var record storageTruthNodeFailureRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return nil, 0, err
		}
		if include != nil && !include(record) {
			continue
		}
		if record.TicketID != "" {
			tickets[record.TicketID] = struct{}{}
		}
		if events < ^uint32(0) {
			events++
		}
	}
	return tickets, events, nil
}

func (k Keeper) hasNodeFailure(ctx sdk.Context, supernodeAccount string, startEpoch uint64, endEpoch uint64, include func(storageTruthNodeFailureRecord) bool) (bool, error) {
	_, events, err := k.distinctNodeFailedTickets(ctx, supernodeAccount, startEpoch, endEpoch, include)
	return events > 0, err
}

func (k Keeper) hasIndependentReporterPassInWindow(
	ctx sdk.Context,
	ticketID string,
	targetAccount string,
	excludeReporter string,
	startEpoch uint64,
	endEpoch uint64,
) (bool, error) {
	// Per 122-Copilot-3 + 122-F1 — indexed lookup avoids DeliverTx full-table scan.
	// Scan secondary index: "st/rrs-tt/" + target + "/" + u64be(epoch) + "/"
	// for each epoch in [startEpoch, endEpoch].
	startKey := types.ReporterStorageTruthResultByTargetEpochPrefix(targetAccount, startEpoch)
	endKey := types.ReporterStorageTruthResultByTargetEpochPrefix(targetAccount, endEpoch+1)
	it := k.kvStore(ctx).Iterator(startKey, endKey)
	defer it.Close()

	for ; it.Valid(); it.Next() {
		var record storageTruthReporterResultRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return false, err
		}
		if record.TicketID != ticketID {
			continue
		}
		if types.StorageProofResultClass(record.ResultClass) != types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS {
			continue
		}
		if record.Reporter == "" || record.Reporter == excludeReporter {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (k Keeper) hasCleanRecheckInWindow(
	ctx sdk.Context,
	ticketID string,
	targetAccount string,
	startEpoch uint64,
	endEpoch uint64,
) (bool, error) {
	// Per 122-Copilot-4 + 122-F1 — indexed lookup avoids DeliverTx full-table scan.
	// Scan secondary index: "st/spt-tbe/" + target + "/" + u32be(RECHECK) + "/" epoch range.
	recheckBucket := uint32(types.StorageProofBucketType_STORAGE_PROOF_BUCKET_TYPE_RECHECK)
	bucketPfx := types.TranscriptByTargetBucketEpochScanPrefix(targetAccount, recheckBucket)
	startKey := binary.BigEndian.AppendUint64(append([]byte(nil), bucketPfx...), startEpoch)
	endKey := binary.BigEndian.AppendUint64(append([]byte(nil), bucketPfx...), endEpoch+1)
	it := k.kvStore(ctx).Iterator(startKey, endKey)
	defer it.Close()

	for ; it.Valid(); it.Next() {
		var record storageProofTranscriptRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return false, err
		}
		if record.TicketID != ticketID {
			continue
		}
		if types.StorageProofResultClass(record.ResultClass) != types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (k Keeper) setStorageTruthFailedHeal(ctx sdk.Context, supernodeAccount string, epochID uint64, ticketID string) {
	if supernodeAccount == "" || ticketID == "" {
		return
	}
	k.kvStore(ctx).Set(types.StorageTruthFailedHealKey(supernodeAccount, epochID, ticketID), []byte{1})
}

func (k Keeper) hasStorageTruthFailedHeal(ctx sdk.Context, supernodeAccount string, startEpoch uint64, endEpoch uint64) bool {
	prefix := types.StorageTruthFailedHealPrefix(supernodeAccount)
	it := k.kvStore(ctx).Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()
	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix)+8 {
			continue
		}
		epochID := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])
		if epochID >= startEpoch && epochID <= endEpoch {
			return true
		}
	}
	return false
}

// GetAllStorageProofTranscriptsForGenesis exports all st/spt/ records as raw JSON value bytes.
// Per NEW-C-1: secondary index st/spt-tbe/ is rebuilt by setStorageProofTranscriptRecord on InitGenesis.
func (k Keeper) GetAllStorageProofTranscriptsForGenesis(ctx sdk.Context) []types.GenesisStorageProofTranscript {
	prefix := types.StorageProofTranscriptPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	out := make([]types.GenesisStorageProofTranscript, 0)
	for ; it.Valid(); it.Next() {
		key := it.Key()
		hash := string(key[len(prefix):])
		val := append([]byte(nil), it.Value()...)
		out = append(out, types.GenesisStorageProofTranscript{
			TranscriptHash: hash,
			RecordJson:     val,
		})
	}
	return out
}

// importStorageProofTranscriptForGenesis re-emits a transcript record (writing the
// st/spt-tbe/ secondary index alongside) so genesis-imported state matches runtime.
func (k Keeper) importStorageProofTranscriptForGenesis(ctx sdk.Context, hash string, recordJSON []byte) error {
	var rec storageProofTranscriptRecord
	if err := json.Unmarshal(recordJSON, &rec); err != nil {
		return err
	}
	return k.setStorageProofTranscriptRecord(ctx, hash, rec)
}

// GetAllNodeFailureFactsForGenesis exports all st/nf/ records.
func (k Keeper) GetAllNodeFailureFactsForGenesis(ctx sdk.Context) []types.GenesisNodeFailureFact {
	prefix := types.NodeStorageTruthFailureRootPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	out := make([]types.GenesisNodeFailureFact, 0)
	for ; it.Valid(); it.Next() {
		key := it.Key()
		body := key[len(prefix):]
		// "<supernode>" + '/' + u64be(epoch) + '/' + ticket + 0x00 + reporter
		// supernode does not contain '/' (bech32) — split at first '/'.
		slash1 := -1
		for i := 0; i < len(body); i++ {
			if body[i] == '/' {
				slash1 = i
				break
			}
		}
		if slash1 < 0 || len(body) < slash1+1+8+1 {
			continue
		}
		supernode := string(body[:slash1])
		epochID := binary.BigEndian.Uint64(body[slash1+1 : slash1+1+8])
		// next byte is '/'
		rest := body[slash1+1+8+1:]
		// rest = ticket + 0x00 + reporter
		sep := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == 0 {
				sep = i
				break
			}
		}
		if sep < 0 {
			continue
		}
		ticket := string(rest[:sep])
		reporter := string(rest[sep+1:])
		val := append([]byte(nil), it.Value()...)
		out = append(out, types.GenesisNodeFailureFact{
			SupernodeAccount: supernode,
			EpochId:          epochID,
			TicketId:         ticket,
			ReporterAccount:  reporter,
			RecordJson:       val,
		})
	}
	return out
}

// importNodeFailureFactForGenesis writes a raw st/nf/ record to its key.
func (k Keeper) importNodeFailureFactForGenesis(ctx sdk.Context, fact types.GenesisNodeFailureFact) {
	k.kvStore(ctx).Set(types.NodeStorageTruthFailureKey(fact.SupernodeAccount, fact.EpochId, fact.TicketId, fact.ReporterAccount), fact.RecordJson)
}

// GetAllReporterResultFactsForGenesis exports all st/rrs/ records.
// Per NEW-C-1: secondary index st/rrs-tt/ is rebuilt by setStorageTruthReporterResult-equivalent
// import path on InitGenesis.
func (k Keeper) GetAllReporterResultFactsForGenesis(ctx sdk.Context) []types.GenesisReporterResultFact {
	prefix := types.ReporterStorageTruthResultRootPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	out := make([]types.GenesisReporterResultFact, 0)
	for ; it.Valid(); it.Next() {
		key := it.Key()
		body := key[len(prefix):]
		// "<reporter>" + '/' + u64be(epoch) + '/' + ticket + 0x00 + target
		slash1 := -1
		for i := 0; i < len(body); i++ {
			if body[i] == '/' {
				slash1 = i
				break
			}
		}
		if slash1 < 0 || len(body) < slash1+1+8+1 {
			continue
		}
		reporter := string(body[:slash1])
		epochID := binary.BigEndian.Uint64(body[slash1+1 : slash1+1+8])
		rest := body[slash1+1+8+1:]
		sep := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == 0 {
				sep = i
				break
			}
		}
		if sep < 0 {
			continue
		}
		ticket := string(rest[:sep])
		target := string(rest[sep+1:])
		val := append([]byte(nil), it.Value()...)
		out = append(out, types.GenesisReporterResultFact{
			ReporterAccount: reporter,
			EpochId:         epochID,
			TicketId:        ticket,
			TargetAccount:   target,
			RecordJson:      val,
		})
	}
	return out
}

// importReporterResultFactForGenesis writes both the primary and secondary indexes.
func (k Keeper) importReporterResultFactForGenesis(ctx sdk.Context, f types.GenesisReporterResultFact) {
	store := k.kvStore(ctx)
	store.Set(types.ReporterStorageTruthResultKey(f.ReporterAccount, f.EpochId, f.TicketId, f.TargetAccount), f.RecordJson)
	store.Set(types.ReporterStorageTruthResultByTargetKey(f.TargetAccount, f.EpochId, f.TicketId, f.ReporterAccount), f.RecordJson)
}

// GetAllFailedHealMarkersForGenesis exports all st/fh/ marker keys.
func (k Keeper) GetAllFailedHealMarkersForGenesis(ctx sdk.Context) []types.GenesisFailedHealMarker {
	prefix := types.StorageTruthFailedHealRootPrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	out := make([]types.GenesisFailedHealMarker, 0)
	for ; it.Valid(); it.Next() {
		key := it.Key()
		body := key[len(prefix):]
		// "<supernode>" + '/' + u64be(epoch) + '/' + ticket
		slash1 := -1
		for i := 0; i < len(body); i++ {
			if body[i] == '/' {
				slash1 = i
				break
			}
		}
		if slash1 < 0 || len(body) < slash1+1+8+1 {
			continue
		}
		supernode := string(body[:slash1])
		epochID := binary.BigEndian.Uint64(body[slash1+1 : slash1+1+8])
		ticket := string(body[slash1+1+8+1:])
		out = append(out, types.GenesisFailedHealMarker{
			SupernodeAccount: supernode,
			EpochId:          epochID,
			TicketId:         ticket,
		})
	}
	return out
}

func isStorageTruthRecheckEligible(class types.StorageProofResultClass) bool {
	switch class {
	case types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_TIMEOUT_OR_NO_RESPONSE,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_OBSERVER_QUORUM_FAIL,
		types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT:
		return true
	default:
		return false
	}
}
