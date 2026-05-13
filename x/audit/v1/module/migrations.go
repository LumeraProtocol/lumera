package audit

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
)

// NewMigrateV1ToV2 returns the v1→v2 module migration handler.
// Per 122-F4 — bump KeepLastEpochEntries to cover OldClassAFaultWindow for safe pruning.
// Per CP-R3 C-F3 — also covers StorageTruthDivergenceWindowEpochs and
// StorageTruthHealDeadlineEpochs. v2 has not shipped to mainnet, so we extend
// the same handler in place rather than introducing a v2→v3.
func NewMigrateV1ToV2(k keeper.Keeper) func(ctx sdk.Context) error {
	return func(ctx sdk.Context) error {
		params := k.GetParams(ctx)
		oldClassAFaultWindow := uint64(params.StorageTruthOldClassAFaultWindow)
		if oldClassAFaultWindow == 0 {
			oldClassAFaultWindow = 21
		}
		divergenceWindow := uint64(params.StorageTruthDivergenceWindowEpochs)
		if divergenceWindow == 0 {
			divergenceWindow = 14
		}
		healDeadline := uint64(params.StorageTruthHealDeadlineEpochs)
		if healDeadline == 0 {
			healDeadline = 3
		}
		required := oldClassAFaultWindow
		if divergenceWindow > required {
			required = divergenceWindow
		}
		if healDeadline > required {
			required = healDeadline
		}
		if params.KeepLastEpochEntries < required {
			params.KeepLastEpochEntries = required
		}
		return k.SetParams(ctx, params)
	}
}
