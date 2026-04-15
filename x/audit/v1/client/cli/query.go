package cli

import (
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// GetCustomQueryCmd returns custom audit query commands.
func GetCustomQueryCmd() *cobra.Command {
	auditQueryCmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "Audit query commands",
		RunE:  client.ValidateCmd,
	}

	auditQueryCmd.AddCommand(
		CmdEpochReport(),
		CmdEpochReportsByReporter(),
		CmdHostReports(),
	)

	return auditQueryCmd
}
