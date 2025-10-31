package cmd

import (
	"fmt"
	"reflect"

	"github.com/cosmos/cosmos-sdk/client"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/spf13/cobra"
)

func debugResolveTypeURLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve-type-url [type-url]",
		Short: "Resolve a protobuf type URL using the interface registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			resolver, ok := clientCtx.InterfaceRegistry.(interface {
				Resolve(string) (gogoproto.Message, error)
			})
			if !ok {
				return fmt.Errorf("interface registry %T does not support Resolve", clientCtx.InterfaceRegistry)
			}

			msg, err := resolver.Resolve(args[0])
			if err != nil {
				return err
			}

			cmd.Printf("resolved to %v\n", reflect.TypeOf(msg))
			return nil
		},
	}

	return cmd
}
