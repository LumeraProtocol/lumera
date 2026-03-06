package app

import "github.com/cosmos/cosmos-sdk/client"

// SetClientCtx stores the CLI/query client context for services started via
// cosmos/evm's custom server command.
func (app *App) SetClientCtx(clientCtx client.Context) {
	app.clientCtx = clientCtx
}

// RegisterTxService overrides the default runtime.App implementation so we can
// capture the clientCtx that carries the local CometBFT client. cosmos/evm's
// server/start.go calls SetClientCtx BEFORE CometBFT starts, then creates a
// local client AFTER CometBFT starts and passes it to RegisterTxService — but
// never calls SetClientCtx again.
func (app *App) RegisterTxService(clientCtx client.Context) {
	app.clientCtx = clientCtx
	app.App.RegisterTxService(clientCtx)
}

// Close stops auxiliary app goroutines before delegating to runtime.App.
func (app *App) Close() error {
	// Stop async EVM broadcaster first so no background goroutine can race with
	// runtime/app shutdown or attempt late client usage.
	app.stopEVMBroadcastWorker()
	app.stopJSONRPCRateLimitProxy()
	if app.App == nil {
		return nil
	}
	return app.App.Close()
}
