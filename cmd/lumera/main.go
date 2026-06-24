package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	clienthelpers "cosmossdk.io/client/v2/helpers"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/cmd/lumera/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, clienthelpers.EnvPrefix, app.DefaultNodeHome); err != nil {
		// A context cancellation (e.g. SIGTERM) is a graceful shutdown, not an error.
		if !errors.Is(err, context.Canceled) {
			_, _ = fmt.Fprintln(rootCmd.OutOrStderr(), err)
			os.Exit(1)
		}
	}
}
