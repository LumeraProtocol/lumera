package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// RepairLegacyRawShareStartingInfoForTest exposes the narrowly-scoped state
// repair to the external keeper test package without making it production API.
func (k Keeper) RepairLegacyRawShareStartingInfoForTest(
	ctx sdk.Context,
	val stakingtypes.Validator,
	del stakingtypes.Delegation,
	delAddr sdk.AccAddress,
) error {
	return k.repairLegacyRawShareStartingInfo(ctx, val, del, delAddr)
}
