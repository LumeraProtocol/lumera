package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func CmdEpochReport() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epoch-report [epoch-id] [supernode-account]",
		Short: "Query an epoch report by epoch and reporter",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			epochID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch-id %q: %w", args[0], err)
			}
			supernodeAccount := args[1]

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			qc := types.NewQueryClient(clientCtx)
			resp, err := qc.EpochReport(cmd.Context(), &types.QueryEpochReportRequest{
				EpochId:          epochID,
				SupernodeAccount: supernodeAccount,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdEpochReportsByReporter() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epoch-reports-by-reporter [supernode-account]",
		Short: "List epoch reports submitted by a reporter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			supernodeAccount := args[0]

			epochID, err := cmd.Flags().GetUint64("epoch-id")
			if err != nil {
				return err
			}
			filterByEpochID, err := cmd.Flags().GetBool("filter-by-epoch-id")
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			qc := types.NewQueryClient(clientCtx)
			resp, err := qc.EpochReportsByReporter(cmd.Context(), &types.QueryEpochReportsByReporterRequest{
				SupernodeAccount: supernodeAccount,
				EpochId:          epochID,
				FilterByEpochId:  filterByEpochID,
				Pagination:       pageReq,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(resp)
		},
	}

	cmd.Flags().Uint64("epoch-id", 0, "epoch id")
	cmd.Flags().Bool("filter-by-epoch-id", false, "filter by epoch id")
	flags.AddPaginationFlagsToCmd(cmd, "epoch-reports-by-reporter")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdHostReports() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host-reports [supernode-account]",
		Short: "List host reports submitted by a supernode across epochs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			supernodeAccount := args[0]

			epochID, err := cmd.Flags().GetUint64("epoch-id")
			if err != nil {
				return err
			}
			filterByEpochID, err := cmd.Flags().GetBool("filter-by-epoch-id")
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			qc := types.NewQueryClient(clientCtx)
			resp, err := qc.HostReports(cmd.Context(), &types.QueryHostReportsRequest{
				SupernodeAccount: supernodeAccount,
				EpochId:          epochID,
				FilterByEpochId:  filterByEpochID,
				Pagination:       pageReq,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(resp)
		},
	}

	cmd.Flags().Uint64("epoch-id", 0, "epoch id")
	cmd.Flags().Bool("filter-by-epoch-id", false, "filter by epoch id")
	flags.AddPaginationFlagsToCmd(cmd, "host-reports")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
