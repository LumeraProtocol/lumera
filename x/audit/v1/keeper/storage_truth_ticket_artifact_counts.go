package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// SetStorageTruthTicketArtifactCounts anchors canonical class-specific artifact
// counts for a ticket. Existing values are immutable once set.
func (k Keeper) SetStorageTruthTicketArtifactCounts(ctx context.Context, ticketID string, indexArtifactCount uint32, symbolArtifactCount uint32) error {
	if ticketID == "" {
		return errorsmod.Wrap(types.ErrInvalidStorageProofs, "ticket_id is required")
	}
	if indexArtifactCount == 0 || symbolArtifactCount == 0 {
		return errorsmod.Wrap(types.ErrInvalidStorageProofs, "index_artifact_count and symbol_artifact_count must be > 0")
	}

	sdkCtx, ok := ctx.(sdk.Context)
	if !ok {
		sdkCtx = sdk.UnwrapSDKContext(ctx)
	}

	if existing, found := k.GetTicketArtifactCountState(sdkCtx, ticketID); found {
		if existing.IndexArtifactCount != 0 && existing.IndexArtifactCount != indexArtifactCount {
			return errorsmod.Wrapf(
				types.ErrInvalidStorageProofs,
				"ticket %q index artifact count is immutable (existing=%d, new=%d)",
				ticketID,
				existing.IndexArtifactCount,
				indexArtifactCount,
			)
		}
		if existing.SymbolArtifactCount != 0 && existing.SymbolArtifactCount != symbolArtifactCount {
			return errorsmod.Wrapf(
				types.ErrInvalidStorageProofs,
				"ticket %q symbol artifact count is immutable (existing=%d, new=%d)",
				ticketID,
				existing.SymbolArtifactCount,
				symbolArtifactCount,
			)
		}
		if existing.IndexArtifactCount == indexArtifactCount && existing.SymbolArtifactCount == symbolArtifactCount {
			return nil
		}
		if existing.IndexArtifactCount == 0 {
			existing.IndexArtifactCount = indexArtifactCount
		}
		if existing.SymbolArtifactCount == 0 {
			existing.SymbolArtifactCount = symbolArtifactCount
		}
		return k.SetTicketArtifactCountState(sdkCtx, existing)
	}

	return k.SetTicketArtifactCountState(sdkCtx, types.TicketArtifactCountState{
		TicketId:            ticketID,
		IndexArtifactCount:  indexArtifactCount,
		SymbolArtifactCount: symbolArtifactCount,
	})
}
