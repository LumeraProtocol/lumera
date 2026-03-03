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
					RpcMethod:      "EpochReport",
					Use:            "epoch-report [epoch-id] [supernode-account]",
					Short:          "Query an epoch report by epoch and reporter",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "epoch_id"}, {ProtoField: "supernode_account"}},
				},
				{
					RpcMethod:      "EpochReportsByReporter",
					Use:            "epoch-reports-by-reporter [supernode-account]",
					Short:          "List epoch reports submitted by a reporter",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "supernode_account"}},
				},
				{
					RpcMethod:      "StorageChallengeReports",
					Use:            "storage-challenge-reports [supernode-account]",
					Short:          "List reports that include storage challenge observations about a supernode",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "supernode_account"}},
				},
				{
					RpcMethod: "HostReports",
					Use:       "host-reports [supernode-account]",
					Short:     "List host reports submitted by a supernode across epochs",
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
					RpcMethod:      "SubmitEpochReport",
					Use:            "submit-epoch-report [epoch-id] [host-report-json]",
					Short:          "Submit an epoch report (storage challenge observations encoded in JSON via flags)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "epoch_id"}, {ProtoField: "host_report"}},
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
