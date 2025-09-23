package supernode

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	modulev1 "github.com/LumeraProtocol/lumera/api/lumera/supernode"
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
					RpcMethod:      "GetSuperNode",
					Use:            "get-supernode [validator-address]",
					Short:          "Query get-supernode",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validatorAddress"}},
				},
				{
					RpcMethod:      "GetSuperNodeBySuperNodeAddress",
					Use:            "get-supernode-by-address [supernode-address]",
					Short:          "Query supernode by supernode address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "supernodeAddress"}},
				},

				{
					RpcMethod:      "ListSuperNodes",
					Use:            "list-supernodes",
					Short:          "Query list-supernodes",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{},
				},

				{
					RpcMethod: "GetTopSuperNodesForBlock",
					Use:       "get-top-supernodes-for-block [block-height]",
					Short:     "Query get-top-supernodes-for-block",
					Long: "Query get-top-supernodes-for-block with the following states:\n" +
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
			Service:              modulev1.Msg_ServiceDesc.ServiceName,
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
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validatorAddress"}, {ProtoField: "ipAddress"}, {ProtoField: "supernodeAccount"}},
					FlagOptions: map[string]*autocliv1.FlagOptions{
						"p2p_port": {
							Name:         "p2p-port",
							Usage:        "Optional P2P port for supernode communication",
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
					Use:            "update-supernode [validator-address] [ip-address] [note] [supernode-account]",
					Short:          "Send an update-supernode tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validatorAddress"}, {ProtoField: "ipAddress"}, {ProtoField: "note"}, {ProtoField: "supernodeAccount"}},
					FlagOptions: map[string]*autocliv1.FlagOptions{
						"p2p_port": {
							Name:         "p2p-port",
							Usage:        "Optional P2P port for supernode communication",
							DefaultValue: types.DefaultP2PPort,
						},
					},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
