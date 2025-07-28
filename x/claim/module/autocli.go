package claim

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/LumeraProtocol/lumera/x/claim/types"
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
					RpcMethod:      "ClaimRecord",
					Use:            "claim-record [address]",
					Short:          "Query claim-record",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "address"}},
				},

				{
					RpcMethod:      "ListClaimed",
					Use:            "list-claimed [vested-term - 0 if not vested]",
					Short:          "Query listClaimed",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "vestedTerm"}},
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
					RpcMethod:      "Claim",
					Use:            "claim [old-address] [new-address] [pub-key] [signature]",
					Short:          "Send a claim tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "oldAddress"}, {ProtoField: "newAddress"}, {ProtoField: "pubKey"}, {ProtoField: "signature"}},
				},
				{
					RpcMethod:      "DelayedClaim",
					Use:            "delayed-claim [old-address] [new-address] [pub-key] [signature] [tier]",
					Short:          "Send a delayed-claim tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "oldAddress"}, {ProtoField: "newAddress"}, {ProtoField: "pubKey"}, {ProtoField: "signature"}, {ProtoField: "tier"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
