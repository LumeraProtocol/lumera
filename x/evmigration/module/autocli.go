package evmigration

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
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
					RpcMethod:      "MigrationRecord",
					Use:            "migration-record [legacy-address]",
					Short:          "Query a migration record by legacy address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "legacy_address"}},
				},
				{
					RpcMethod: "MigrationRecords",
					Use:       "migration-records",
					Short:     "List all migration records",
				},
				{
					RpcMethod:      "MigrationEstimate",
					Use:            "migration-estimate [legacy-address]",
					Short:          "Dry-run estimate of what would be migrated for a legacy address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "legacy_address"}},
				},
				{
					RpcMethod: "MigrationStats",
					Use:       "migration-stats",
					Short:     "Show aggregate migration statistics",
				},
				{
					RpcMethod: "LegacyAccounts",
					Use:       "legacy-accounts",
					Short:     "List accounts that still need migration",
				},
				{
					RpcMethod: "MigratedAccounts",
					Use:       "migrated-accounts",
					Short:     "List all completed migrations",
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true, // skipped because authority gated
				},
				{
					RpcMethod: "ClaimLegacyAccount",
					Use:       "claim-legacy-account [new-address] [legacy-address] [legacy-pub-key] [legacy-signature]",
					Short:     "Migrate on-chain state from legacy to new address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "new_address"},
						{ProtoField: "legacy_address"},
						{ProtoField: "legacy_pub_key"},
						{ProtoField: "legacy_signature"},
					},
				},
				{
					RpcMethod: "MigrateValidator",
					Use:       "migrate-validator [new-address] [legacy-address] [legacy-pub-key] [legacy-signature]",
					Short:     "Migrate a validator operator from legacy to new address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "new_address"},
						{ProtoField: "legacy_address"},
						{ProtoField: "legacy_pub_key"},
						{ProtoField: "legacy_signature"},
					},
				},
			},
		},
	}
}
