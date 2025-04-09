package action

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	modulev1 "github.com/LumeraProtocol/lumera/api/lumera/action"
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
					RpcMethod:      "GetActionFee",
					Use:            "get-action-fee [data-size]",
					Short:          "Query get-action-fee",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "dataSize"}},
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
					RpcMethod:      "RequestAction",
					Use:            "request-action [action-type] [metadata] [price] [expiration-time]",
					Short:          "Send a request-action tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "actionType"}, {ProtoField: "metadata"}, {ProtoField: "price"}, {ProtoField: "expirationTime"}},
				},
				{
					RpcMethod:      "FinalizeAction",
					Use:            "finalize-action [super-node] [action-id] [metadata]",
					Short:          "Send a finalize-action tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "actionId"}, {ProtoField: "actionType"}, {ProtoField: "metadata"}},
				},
				{
					RpcMethod:      "ApproveAction",
					Use:            "approve-action [action-id] [signature]",
					Short:          "Send a approve-action tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "actionId"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
