package claim

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	modulev1 "github.com/pastelnetwork/pastel/api/pastel/claim"
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
					RpcMethod:      "ClaimRecord",
					Use:            "claim-record [address]",
					Short:          "Query claim-record",
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
					RpcMethod:      "Claim",
					Use:            "claim [old-address] [new-address] [pub-key] [signature]",
					Short:          "Send a claim tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "oldAddress"}, {ProtoField: "newAddress"}, {ProtoField: "pubKey"}, {ProtoField: "signature"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
