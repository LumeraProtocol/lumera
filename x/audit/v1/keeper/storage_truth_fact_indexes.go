package keeper

import (
	"encoding/binary"
	"encoding/json"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

type storageProofTranscriptRecord struct {
	EpochID               uint64 `json:"epoch_id"`
	ReporterAccount       string `json:"reporter_account"`
	TargetAccount         string `json:"target_account"`
	TicketID              string `json:"ticket_id"`
	ResultClass           int32  `json:"result_class"`
	BucketType            int32  `json:"bucket_type"`
	ArtifactClass         int32  `json:"artifact_class"`
	ArtifactKey           string `json:"artifact_key,omitempty"`
	ArtifactOrdinal       uint32 `json:"artifact_ordinal,omitempty"`
	RecheckEligible       bool   `json:"recheck_eligible"`
	ConfirmedByRecheck    bool   `json:"confirmed_by_recheck,omitempty"`
	ContradictedByRecheck bool   `json:"contradicted_by_recheck,omitempty"`
	RecheckTranscriptHash string `json:"recheck_transcript_hash,omitempty"`
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
			EpochID:         epochID,
			ReporterAccount: reporterAccount,
			TargetAccount:   result.TargetSupernodeAccount,
			TicketID:        result.TicketId,
			ResultClass:     int32(result.ResultClass),
			BucketType:      int32(result.BucketType),
			ArtifactClass:   int32(result.ArtifactClass),
			ArtifactKey:     result.ArtifactKey,
			ArtifactOrdinal: result.ArtifactOrdinal,
			RecheckEligible: isStorageTruthRecheckEligible(result.ResultClass),
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
	k.kvStore(ctx).Set(types.StorageProofTranscriptKey(transcriptHash), bz)
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
	k.kvStore(ctx).Set(types.ReporterStorageTruthResultKey(reporterAccount, epochID, result.TicketId, result.TargetSupernodeAccount), bz)
	return nil
}

func (k Keeper) markStorageTruthReporterResultRecheck(ctx sdk.Context, reporterAccount string, transcriptHash string, confirmed bool) error {
	record, found, err := k.getStorageProofTranscriptRecord(ctx, transcriptHash)
	if err != nil || !found {
		return err
	}
	resultBz := k.kvStore(ctx).Get(types.ReporterStorageTruthResultKey(reporterAccount, record.EpochID, record.TicketID, record.TargetAccount))
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
		k.kvStore(ctx).Set(types.ReporterStorageTruthResultKey(reporterAccount, record.EpochID, record.TicketID, record.TargetAccount), bz)
	}
	record.ConfirmedByRecheck = confirmed
	record.ContradictedByRecheck = !confirmed
	if err := k.setStorageProofTranscriptRecord(ctx, transcriptHash, record); err != nil {
		return err
	}
	return nil
}

func (k Keeper) distinctNodeFailedTickets(ctx sdk.Context, supernodeAccount string, startEpoch uint64, endEpoch uint64, include func(storageTruthNodeFailureRecord) bool) (map[string]struct{}, uint32, error) {
	tickets := make(map[string]struct{})
	var events uint32
	prefix := types.NodeStorageTruthFailurePrefix(supernodeAccount)
	it := k.kvStore(ctx).Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()
	for ; it.Valid(); it.Next() {
		var record storageTruthNodeFailureRecord
		if err := json.Unmarshal(it.Value(), &record); err != nil {
			return nil, 0, err
		}
		if record.EpochID < startEpoch || record.EpochID > endEpoch {
			continue
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
