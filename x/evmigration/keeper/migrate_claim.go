package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
)

// MigrateClaim updates claim records where DestAddress matches legacyAddr.
// This is cosmetic/audit — claim funds were already transferred during the
// claim period. Updates the record to point to the new address.
func (k Keeper) MigrateClaim(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error {
	var matchingOldAddresses []string
	err := k.claimKeeper.IterateClaimRecords(ctx, func(record claimtypes.ClaimRecord) (bool, error) {
		if record.DestAddress == legacyAddr.String() {
			matchingOldAddresses = append(matchingOldAddresses, record.OldAddress)
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	for _, oldAddress := range matchingOldAddresses {
		record, found, err := k.claimKeeper.GetClaimRecord(ctx, oldAddress)
		if err != nil {
			return err
		}
		if !found || record.DestAddress != legacyAddr.String() {
			continue
		}
		record.DestAddress = newAddr.String()
		if err := k.claimKeeper.SetClaimRecord(ctx, record); err != nil {
			return err
		}
	}

	return nil
}
