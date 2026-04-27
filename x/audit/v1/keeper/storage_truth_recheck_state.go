package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// HasRecheckEvidence returns true if a recheck evidence submission already exists
// for the given (epochID, ticketID, creatorAccount) triple, preventing replay.
func (k Keeper) HasRecheckEvidence(ctx sdk.Context, epochID uint64, ticketID string, creatorAccount string) bool {
	store := k.kvStore(ctx)
	return store.Has(types.RecheckEvidenceKey(epochID, ticketID, creatorAccount))
}

// SetRecheckEvidence records that a recheck evidence submission has been accepted
// for the given (epochID, ticketID, creatorAccount) triple.
func (k Keeper) SetRecheckEvidence(ctx sdk.Context, epochID uint64, ticketID string, creatorAccount string) {
	store := k.kvStore(ctx)
	store.Set(types.RecheckEvidenceKey(epochID, ticketID, creatorAccount), []byte{1})
}
