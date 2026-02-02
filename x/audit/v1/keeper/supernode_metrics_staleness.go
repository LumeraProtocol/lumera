package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// HandleSupernodeMetricsStaleness applies supernode metrics staleness rules at end-block:
//   - compute an "overdue" threshold from MetricsUpdateIntervalBlocks + MetricsGracePeriodBlocks
//   - only consider supernodes whose latest state is ACTIVE
//   - if no metrics have ever been reported and the chain height passes the threshold,
//     mark the supernode as POSTPONED with reason "no metrics reported"
//   - if the last metrics height is older than the threshold, mark POSTPONED with
//     reason "metrics overdue"
func (k Keeper) HandleSupernodeMetricsStaleness(ctx sdk.Context) error {
	params := k.supernodeKeeper.GetParams(ctx)
	overdueThreshold := int64(params.MetricsUpdateIntervalBlocks + params.MetricsGracePeriodBlocks)

	supernodes, err := k.supernodeKeeper.GetAllSuperNodes(ctx)
	if err != nil {
		return err
	}

	for i := range supernodes {
		sn := supernodes[i]
		if len(sn.States) == 0 {
			continue
		}
		lastState := sn.States[len(sn.States)-1].State
		// Only perform staleness checks for ACTIVE supernodes.
		if lastState != sntypes.SuperNodeStateActive {
			continue
		}

		valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
		if err != nil {
			continue
		}

		lastHeight := int64(0)
		if state, ok := k.supernodeKeeper.GetMetricsState(ctx, valAddr); ok {
			lastHeight = state.Height
		}

		// If no metrics have ever been reported, use the supernode's registration
		// height as the baseline for staleness, so newly-registered supernodes
		// are given a full update interval + grace period from registration.
		if lastHeight == 0 {
			var registrationHeight int64
			for _, st := range sn.States {
				if st != nil {
					registrationHeight = st.Height
					break
				}
			}
			if registrationHeight == 0 {
				continue
			}
			if ctx.BlockHeight()-registrationHeight > overdueThreshold {
				if err := k.supernodeKeeper.SetSuperNodePostponed(ctx, valAddr, "no metrics reported"); err != nil {
					k.Logger().Error(
						"failed to mark supernode postponed for missing metrics",
						"validator", sn.ValidatorAddress,
						"err", err,
					)
				}
			}
			continue
		}

		if ctx.BlockHeight()-lastHeight > overdueThreshold {
			if err := k.supernodeKeeper.SetSuperNodePostponed(ctx, valAddr, "metrics overdue"); err != nil {
				k.Logger().Error(
					"failed to mark supernode postponed for overdue metrics",
					"validator", sn.ValidatorAddress,
					"err", err,
				)
			}
		}
	}

	return nil
}
