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
				// simple, non-paginated queries
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
				// paginated list queries
				{
					RpcMethod: "ListActions",
					Use:       "list-actions",
					Short:     "List actions with optional type and state filters",
					FlagOptions: map[string]*autocliv1.FlagOptions{
						"actionType": {
							Name:  "action-type",
							Usage: "Optional action type filter (ACTION_TYPE_SENSE, ACTION_TYPE_CASCADE, ...)",
						},
						"actionState": {
							Name:  "action-state",
							Usage: "Optional action state filter (ACTION_STATE_PENDING, ACTION_STATE_DONE, ...)",
						},
					},
				},
				{
					RpcMethod:      "ListActionsByCreator",
					Use:            "list-actions-by-creator [creator]",
					Short:          "List actions created by a specific address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "creator"}},
				},
				{
					RpcMethod: "ListActionsBySuperNode",
					Use:       "list-actions-by-supernode [supernode-address]",
					Short:     "List actions for a specific supernode",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "superNodeAddress"},
					},
				},
				{
					RpcMethod: "ListActionsByBlockHeight",
					Use:       "list-actions-by-block-height [block-height]",
					Short:     "List actions created at a specific block height",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "blockHeight"},
					},
				},
				{
					RpcMethod: "ListExpiredActions",
					Use:       "list-expired-actions",
					Short:     "List actions that are in EXPIRED state",
				},
				{
					RpcMethod: "QueryActionByMetadata",
					Use:       "query-action-by-metadata [action-type] [metadata-query]",
					Short:     "Query actions by type and metadata (e.g. \"collection_id=123\")",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "actionType"},
						{ProtoField: "metadataQuery"},
					},
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
