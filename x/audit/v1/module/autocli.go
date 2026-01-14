package audit

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Shows the parameters of the module",
				},
				{
					RpcMethod:      "AuditStatus",
					Use:            "audit-status [validator-address]",
					Short:          "Query audit status for a validator",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator_address"}},
				},
				// this line is used by ignite scaffolding # autocli/query
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true,
				},
				{
					RpcMethod:      "SubmitAuditReport",
					Use:            "submit-audit-report [window-id] [self-report-json]",
					Short:          "Submit an audit report (peer observations encoded in JSON via flags)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "window_id"}, {ProtoField: "self_report"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}

