package audit

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
)

// NewMigrateV1ToV2 returns the v1→v2 module migration handler.
// Per 122-F4 — bump KeepLastEpochEntries to cover OldClassAFaultWindow for safe pruning.
func NewMigrateV1ToV2(k keeper.Keeper) func(ctx sdk.Context) error {
	return func(ctx sdk.Context) error {
		params := k.GetParams(ctx)
		oldClassAFaultWindow := uint64(params.StorageTruthOldClassAFaultWindow)
		if oldClassAFaultWindow == 0 {
			oldClassAFaultWindow = 21
		}
		if params.KeepLastEpochEntries < oldClassAFaultWindow {
			params.KeepLastEpochEntries = oldClassAFaultWindow
		}
		return k.SetParams(ctx, params)
	}
}
