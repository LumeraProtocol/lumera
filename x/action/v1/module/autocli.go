package action

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
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
					RpcMethod:      "GetAction",
					Use:            "action [action-id]",
					Short:          "Get a single action by ID",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "actionID"}},
				},
				{
					RpcMethod:      "GetActionFee",
					Use:            "get-action-fee [data-size-in-kb]",
					Short:          "Query get-action-fee",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "dataSize"}},
				},
				{
					RpcMethod:      "ListActionsByCreator",
					Use:            "list-actions-by-creator [creator]",
					Short:          "List actions created by a specific address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "creator"}},
				},

				// this line is used by ignite scaffolding # autocli/query
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
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
					Use:            "finalize-action [action-id] [action-type] [metadata]",
					Short:          "Send a finalize-action tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "actionId"}, {ProtoField: "actionType"}, {ProtoField: "metadata"}},
				},
				{
					RpcMethod:      "ApproveAction",
					Use:            "approve-action [action-id]",
					Short:          "Send a approve-action tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "actionId"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
