package cli

import (
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// GetCustomQueryCmd returns custom supernode query commands.
func GetCustomQueryCmd() *cobra.Command {
	supernodeQueryCmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "Supernode query commands",
		RunE:  client.ValidateCmd,
	}

	supernodeQueryCmd.AddCommand(
		CmdGetMetrics(),
	)

	return supernodeQueryCmd
}
