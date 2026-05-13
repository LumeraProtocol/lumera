package simulation

import (
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

const TypeMsgSubmitEpochReportVariance = "submit_epoch_report_variance"

func SimulateMsgSubmitEpochReportVariance(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, _ []simtypes.Account, _ string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		epochID, _, _, err := k.GetCurrentEpochInfo(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgSubmitEpochReportVariance, err.Error()), nil, nil
		}
		sns, err := k.GetAllSuperNodes(ctx)
		if err != nil || len(sns) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgSubmitEpochReportVariance, "no supernodes"), nil, nil
		}
		sn := sns[r.Intn(len(sns))]
		if sn.SupernodeAccount == "" {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgSubmitEpochReportVariance, "empty supernode account"), nil, nil
		}

		host := types.HostReport{
			CpuUsagePercent:        10 + r.Float64()*20,
			MemUsagePercent:        10 + r.Float64()*20,
			DiskUsagePercent:       70 + r.Float64()*35, // exercises both sides of 90% threshold
			FailedActionsCount:     uint32(r.Intn(3)),
		}
		msg := &types.MsgSubmitEpochReport{
			Creator:                      sn.SupernodeAccount,
			EpochId:                      epochID,
			HostReport:                   host,
			StorageChallengeObservations: nil,
		}
		ms := keeper.NewMsgServerImpl(k)
		if _, err := ms.SubmitEpochReport(ctx, msg); err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgSubmitEpochReportVariance, fmt.Sprintf("submit failed: %v", err)), nil, nil
		}
		return simtypes.NewOperationMsg(msg, true, "success"), nil, nil
	}
}
