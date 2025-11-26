package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// HandleMetricsStaleness transitions overdue supernodes into POSTPONED.
func (k Keeper) HandleMetricsStaleness(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	overdueThreshold := int64(params.MetricsUpdateInterval + params.MetricsGracePeriodBlocks)

	supernodes, err := k.GetAllSuperNodes(ctx)
	if err != nil {
		return err
	}

	for i := range supernodes {
		sn := supernodes[i]
		if len(sn.States) == 0 {
			continue
		}
		lastState := sn.States[len(sn.States)-1].State
		if lastState != types.SuperNodeStateActive && lastState != types.SuperNodeStateDisabled {
			continue
		}
		lastHeight := int64(0)
		if sn.Metrics != nil {
			lastHeight = sn.Metrics.Height
		}
		if lastHeight == 0 {
			if ctx.BlockHeight() > overdueThreshold {
				_ = k.markPostponed(ctx, &sn, "no metrics reported")
			}
			continue
		}
		if ctx.BlockHeight()-lastHeight > overdueThreshold {
			_ = k.markPostponed(ctx, &sn, "metrics overdue")
		}
	}

	return nil
}
