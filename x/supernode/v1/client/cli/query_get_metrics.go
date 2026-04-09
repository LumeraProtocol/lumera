package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// CmdGetMetrics queries the latest stored metrics for a validator.
// This bypasses AutoCLI's aminojson output path so float fields print correctly.
func CmdGetMetrics() *cobra.Command {
	var validatorAddress string

	cmd := &cobra.Command{
		Use:   "get-metrics [validator-address]",
		Short: "Query the latest metrics state for a validator",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case len(args) == 1 && validatorAddress == "":
				validatorAddress = args[0]
			case len(args) == 0 && validatorAddress != "":
			case len(args) == 1 && validatorAddress != "" && validatorAddress == args[0]:
			default:
				return fmt.Errorf("provide exactly one validator address via positional arg or --validator-address")
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.GetMetrics(context.Background(), &types.QueryGetMetricsRequest{
				ValidatorAddress: validatorAddress,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	cmd.Flags().StringVar(&validatorAddress, "validator-address", "", "validator operator address")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
