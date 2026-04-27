package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func CmdPayoutHistory() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payout-history [validator-address]",
		Short: "Query payout history for a supernode validator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			req := &types.QueryPayoutHistoryRequest{ValidatorAddress: args[0]}

			res, err := queryClient.PayoutHistory(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
