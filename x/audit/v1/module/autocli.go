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
					RpcMethod:      "EvidenceById",
					Use:            "evidence [evidence-id]",
					Short:          "Query evidence by id",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "evidence_id"}},
				},
				{
					RpcMethod:      "EvidenceBySubject",
					Use:            "evidence-by-subject [subject-address]",
					Short:          "List evidence records by subject address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "subject_address"}},
				},
				{
					RpcMethod:      "EvidenceByAction",
					Use:            "evidence-by-action [action-id]",
					Short:          "List evidence records by action id",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "action_id"}},
				},
				{
					RpcMethod: "CurrentEpoch",
					Use:       "current-epoch",
					Short:     "Query current audit epoch boundaries",
				},
				{
					RpcMethod:      "AuditReport",
					Use:            "audit-report [epoch-id] [supernode-account]",
					Short:          "Query an audit report by epoch and reporter",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "epoch_id"}, {ProtoField: "supernode_account"}},
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
					Short:     "List self-reports submitted by a supernode across epochs",
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
					Use:            "submit-audit-report [epoch-id] [self-report-json]",
					Short:          "Submit an audit report (peer observations encoded in JSON via flags)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "epoch_id"}, {ProtoField: "self_report"}},
				},
				{
					RpcMethod:      "SubmitEvidence",
					Use:            "submit-evidence [subject-address] [evidence-type] [action-id] [metadata-json]",
					Short:          "Submit evidence about a subject (metadata is JSON)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "subject_address"}, {ProtoField: "evidence_type"}, {ProtoField: "action_id"}, {ProtoField: "metadata"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
