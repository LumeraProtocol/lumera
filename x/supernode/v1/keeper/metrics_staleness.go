package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const (
	// Base denom for LUME.
	lumeBaseDenom = "ulume"
	// 1 LUME = 1,000,000 ulume (base denom; see denom_metadata in genesis).
	minSupernodeBalanceUlume int64 = 1_000_000
)

// HandleMetricsStaleness is called from EndBlock and applies
// metrics staleness rules:
//   - compute an "overdue" threshold from MetricsUpdateIntervalBlocks + MetricsGracePeriodBlocks
//   - only consider supernodes whose latest state is ACTIVE
//   - if the supernode account has < 1 LUME spendable balance, mark POSTPONED with
//     reason "insufficient balance"
//   - if no metrics have ever been reported and the chain height passes the threshold,
//     mark the supernode as POSTPONED with reason "no metrics reported"
//   - if the last metrics height is older than the threshold, mark POSTPONED with
//     reason "metrics overdue"
func (k Keeper) HandleMetricsStaleness(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	overdueThreshold := int64(params.MetricsUpdateIntervalBlocks + params.MetricsGracePeriodBlocks)

	supernodes, err := k.GetAllSuperNodes(ctx, types.SuperNodeStateActive)
	if err != nil {
		return err
	}

	minSupernodeBalance := sdk.NewInt64Coin(lumeBaseDenom, minSupernodeBalanceUlume)

	for i := range supernodes {
		sn := supernodes[i]
		// Defensive check: state filtering above should guarantee at least one state record.
		if len(sn.States) == 0 {
			continue
		}

		// If the supernode account does not have enough spendable balance,
		// postpone immediately (regardless of metrics freshness).
		if k.bankKeeper != nil {
			supernodeAccAddr, err := sdk.AccAddressFromBech32(sn.SupernodeAccount)
			if err != nil {
				k.Logger().Warn(
					"invalid supernode account address; skipping balance check",
					"validator", sn.ValidatorAddress,
					"supernode_account", sn.SupernodeAccount,
					"err", err,
				)
			} else {
				spendable := k.bankKeeper.SpendableCoins(ctx, supernodeAccAddr).AmountOf(lumeBaseDenom)
				if spendable.LT(minSupernodeBalance.Amount) {
					if err := markPostponed(ctx, k, &sn, "insufficient balance"); err != nil {
						k.Logger().Error(
							"failed to mark supernode postponed for insufficient balance",
							"validator", sn.ValidatorAddress,
							"supernode_account", sn.SupernodeAccount,
							"err", err,
						)
					}
					continue
				}
			}
		}

		// Load the last metrics report height from the dedicated metrics state table.
		valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
		if err != nil {
			continue
		}
		lastHeight := int64(0)
		if state, ok := k.GetMetricsState(ctx, valAddr); ok {
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
				// No meaningful registration height; skip staleness for this supernode.
				continue
			}
			if ctx.BlockHeight()-registrationHeight > overdueThreshold {
				if err := markPostponed(ctx, k, &sn, "no metrics reported"); err != nil {
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
			if err := markPostponed(ctx, k, &sn, "metrics overdue"); err != nil {
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
