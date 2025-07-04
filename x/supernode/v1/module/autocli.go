package supernode

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
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
					RpcMethod:      "GetSuperNode",
					Use:            "get-super-node [validator-address]",
					Short:          "Query get-super-node",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validatorAddress"}},
				},

				{
					RpcMethod:      "ListSuperNodes",
					Use:            "list-super-nodes",
					Short:          "Query list-super-nodes",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{},
				},

				{
					RpcMethod: "GetTopSuperNodesForBlock",
					Use:       "get-top-super-nodes-for-block [block-height]",
					Short:     "Query get-top-super-nodes-for-block",
					Long: "Query get-top-super-nodes-for-block with the following states:\n" +
						"  - SUPERNODE_STATE_ACTIVE\n" +
						"  - SUPERNODE_STATE_DISABLED\n" +
						"  - SUPERNODE_STATE_STOPPED\n" +
						"  - SUPERNODE_STATE_PENALIZED",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "blockHeight"},
					},
					FlagOptions: map[string]*autocliv1.FlagOptions{
						"limit": {
							Name:         "limit",
							Usage:        "Optional limit for number of super nodes",
							DefaultValue: "25",
						},
						"state": {
							Name:         "state",
							Usage:        "Optional state filter (SUPERNODE_STATE_ACTIVE, SUPERNODE_STATE_DISABLED, SUPERNODE_STATE_STOPPED, SUPERNODE_STATE_PENALIZED)",
							DefaultValue: "SUPERNODE_STATE_ACTIVE",
						},
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
					RpcMethod:      "RegisterSupernode",
					Use:            "register-supernode [validator-address] [ip-address] [supernode-account]",
					Short:          "Send a register-supernode tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "validatorAddress"},
						{ProtoField: "ipAddress"},
						{ProtoField: "supernodeAccount"},
					},
					FlagOptions: map[string]*autocliv1.FlagOptions{
	  					"p2p_port": {
							Name:		 "p2p-port",
							Usage:        "Optional P2P port for the supernode communication",
							DefaultValue: types.DefaultP2PPort,
						},
					},
				},
				{
					RpcMethod:      "DeregisterSupernode",
					Use:            "deregister-supernode [validator-address]",
					Short:          "Send a deregister-supernode tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validatorAddress"}},
				},
				{
					RpcMethod:      "StartSupernode",
					Use:            "start-supernode [validator-address]",
					Short:          "Send a start-supernode tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validatorAddress"}},
				},
				{
					RpcMethod:      "StopSupernode",
					Use:            "stop-supernode [validator-address] [reason]",
					Short:          "Send a stop-supernode tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validatorAddress"}, {ProtoField: "reason"}},
				},
				{
					RpcMethod:      "UpdateSupernode",
					Use:            "update-supernode [validator-address] [ip-address] [version] [supernode-account]",
					Short:          "Send an update-supernode tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validatorAddress"}, {ProtoField: "ipAddress"}, {ProtoField: "version"}, {ProtoField: "supernodeAccount"}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
