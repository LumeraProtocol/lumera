package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// CmdSNEligibility queries whether a supernode is eligible for Everlight payouts.
// This command intentionally prints the protobuf response directly to avoid
// AutoCLI's aminojson float64 marshalling issue for this response type.
func CmdSNEligibility() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sn-eligibility [validator-address]",
		Short: "Query whether a supernode is eligible for everlight payouts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.SNEligibility(context.Background(), &types.QuerySNEligibilityRequest{
				ValidatorAddress: args[0],
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
