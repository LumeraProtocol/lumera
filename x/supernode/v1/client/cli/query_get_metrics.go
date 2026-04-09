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
func CmdGetMetrics() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-metrics",
		Short: "Execute the GetMetrics RPC method",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			validatorAddress, err := cmd.Flags().GetString(flagValidatorAddress)
			if err != nil {
				return err
			}
			if validatorAddress == "" {
				return fmt.Errorf("%s is required", flagValidatorAddress)
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
	_ = cmd.MarkFlagRequired(flagValidatorAddress)
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
