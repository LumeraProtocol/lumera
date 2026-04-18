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
					RpcMethod:      "MigrationRecordByNewAddress",
					Use:            "migration-record-by-new-address [new-address]",
					Short:          "Query a migration record by new address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "new_address"}},
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
					Skip:      true, // custom hand-written command in x/evmigration/client/cli/tx.go (legacy_proof is a oneof)
				},
				{
					RpcMethod: "MigrateValidator",
					Skip:      true, // custom hand-written command in x/evmigration/client/cli/tx.go (legacy_proof is a oneof)
				},
			},
		},
	}
}
