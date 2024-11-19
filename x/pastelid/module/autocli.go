package pastelid

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	modulev1 "github.com/pastelnetwork/pastel/api/pastel/pastelid"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: modulev1.Query_ServiceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Shows the parameters of the module",
				},
				{
					RpcMethod: "PastelidEntryAll",
					Use:       "list-pastelid-entry",
					Short:     "List all pastelid-entry",
				},
				{
					RpcMethod:      "PastelidEntry",
					Use:            "show-pastelid-entry [id]",
					Short:          "Shows a pastelid-entry",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "address"}},
				},
				// this line is used by ignite scaffolding # autocli/query
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              modulev1.Msg_ServiceDesc.ServiceName,
			EnhanceCustomCommand: true, // only required if you want to use the custom command
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true, // skipped because authority gated
				},
				{
					RpcMethod: "CreatePastelId",
					Skip:      true, // skipped because use different command in x/pastelid/client/cli/tx_create_pastel_id.go
					//Use:            "create-pastel-id [id-type] [pastel-id] [pq-key] [signature] [time-stamp] [version]",
					//Short:          "Send a create-pastel-id tx",
					//PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "idType"}, {ProtoField: "pastelId"}, {ProtoField: "pqKey"}, {ProtoField: "signature"}, {ProtoField: "timeStamp"}, {ProtoField: "version"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
