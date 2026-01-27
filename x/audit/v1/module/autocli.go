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
					RpcMethod: "CurrentWindow",
					Use:       "current-window",
					Short:     "Query current audit reporting window boundaries",
				},
				{
					RpcMethod:      "AuditReport",
					Use:            "audit-report [window-id] [supernode-account]",
					Short:          "Query an audit report by window and reporter",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "window_id"}, {ProtoField: "supernode_account"}},
				},
				{
					RpcMethod:      "AuditReportsByReporter",
					Use:            "audit-reports-by-reporter [supernode-account]",
					Short:          "List audit reports submitted by a reporter",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "supernode_account"}},
				},
				{
					RpcMethod:      "SupernodeReports",
					Use:            "supernode-reports [supernode-account]",
					Short:          "List reports that include observations about a supernode",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "supernode_account"}},
				},
				{
					RpcMethod: "SelfReports",
					Use:       "self-reports [supernode-account]",
					Short:     "List self-reports submitted by a supernode across windows",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "supernode_account"},
					},
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
