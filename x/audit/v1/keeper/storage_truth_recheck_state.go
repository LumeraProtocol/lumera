package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
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

// GetAllRecheckEvidenceForGenesis iterates the st/rce/ prefix and returns all
// recheck-evidence dedup entries for genesis export. Per NEW-C-1.
func (k Keeper) GetAllRecheckEvidenceForGenesis(ctx sdk.Context) []types.GenesisRecheckEvidence {
	prefix := types.RecheckEvidencePrefix()
	store := k.kvStore(ctx)
	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer func() { _ = it.Close() }()

	out := make([]types.GenesisRecheckEvidence, 0)
	for ; it.Valid(); it.Next() {
		key := it.Key()
		// Layout: prefix + u64be(epoch) + '/' + ticket_id + 0x00 + creator
		body := key[len(prefix):]
		if len(body) < 8+1 {
			continue
		}
		epochID := binary.BigEndian.Uint64(body[:8])
		// body[8] is '/'
		rest := body[9:]
		// find 0x00 separator between ticket and creator
		var sep = -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == 0 {
				sep = i
				break
			}
		}
		if sep < 0 {
			continue
		}
		ticketID := string(rest[:sep])
		creator := string(rest[sep+1:])
		out = append(out, types.GenesisRecheckEvidence{
			EpochId:        epochID,
			TicketId:       ticketID,
			CreatorAccount: creator,
		})
	}
	return out
}
