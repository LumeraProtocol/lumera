package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

const flagValidatorAddress = "validator-address"

// CmdGetMetrics queries the latest metrics state for a validator.
// Supports either positional [validator-address] or --validator-address.
func CmdGetMetrics() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-metrics [validator-address]",
		Short: "Query the latest metrics state for a validator",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			validatorAddress, err := cmd.Flags().GetString(flagValidatorAddress)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				if validatorAddress != "" && validatorAddress != args[0] {
					return fmt.Errorf("provide exactly one validator address via positional arg or --%s", flagValidatorAddress)
				}
				validatorAddress = args[0]
			}
			if validatorAddress == "" {
				return fmt.Errorf("validator address is required")
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			resp, err := queryClient.GetMetrics(cmd.Context(), &types.QueryGetMetricsRequest{
				ValidatorAddress: validatorAddress,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(resp)
		},
	}

	cmd.Flags().String(flagValidatorAddress, "", "validator operator address")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
