package app

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cosmossdk.io/log"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"

	textutil "github.com/LumeraProtocol/lumera/pkg/text"
)

const (
	jsonrpcAliasLogModule          = "json-rpc-alias"
	jsonrpcAliasTimeout            = 5 * time.Second
	JSONRPCAliasPublicAddrAppOpt   = "lumera.json-rpc-alias.public-address"
	JSONRPCAliasUpstreamAddrAppOpt = "lumera.json-rpc-alias.upstream-address"
)

// configureJSONRPCAliasProxy reads the public/internal JSON-RPC addresses that
// were prepared by the start command and stores them on the app so startup can
// launch the compatibility proxy and OpenRPC can advertise the public address.
func (app *App) configureJSONRPCAliasProxy(appOpts servertypes.AppOptions, logger log.Logger) {
	_ = logger
	if !textutil.ParseAppOptionBool(appOpts.Get("json-rpc.enable")) {
		return
	}

	publicAddr := castStringOr(appOpts.Get(JSONRPCAliasPublicAddrAppOpt), "")
	internalAddr := castStringOr(appOpts.Get(JSONRPCAliasUpstreamAddrAppOpt), "")
	if publicAddr == "" || internalAddr == "" {
		if addr, ok := appOpts.Get("json-rpc.address").(string); ok && addr != "" {
			app.openRPCJSONRPCAddr = addr
		}
		return
	}
	app.jsonrpcAliasPublicAddr = publicAddr
	app.jsonrpcAliasUpstreamAddr = internalAddr
	app.openRPCJSONRPCAddr = publicAddr
}

// startJSONRPCAliasProxy starts a reverse proxy on the operator-configured
// JSON-RPC address and forwards requests to the internal cosmos/evm server.
// POST request bodies are rewritten so rpc.discover works alongside the native
// geth-style rpc_discover method.
//
// When rlCfg is non-nil, per-IP rate limiting is injected directly into the
// alias proxy handler, ensuring the public port is always rate-limited.
func (app *App) startJSONRPCAliasProxy(logger log.Logger, rlCfg *rateLimitConfig) {
	if app.jsonrpcAliasPublicAddr == "" || app.jsonrpcAliasUpstreamAddr == "" {
		return
	}

	aliasLogger := logger.With(log.ModuleKey, jsonrpcAliasLogModule)
	upstreamURL, err := url.Parse("http://" + app.jsonrpcAliasUpstreamAddr)
	if err != nil {
		aliasLogger.Error("failed to parse internal JSON-RPC address", "address", app.jsonrpcAliasUpstreamAddr, "error", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read JSON-RPC request", http.StatusBadRequest)
				return
			}
			_ = r.Body.Close()

			body = rewriteJSONRPCDiscoverAlias(body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			r.Header.Set("Content-Length", strconv.Itoa(len(body)))
		}
		proxy.ServeHTTP(w, r)
	})

	// Wrap the alias handler with rate limiting when enabled.
	var handler http.Handler = mux
	if rlCfg != nil {
		var limiter *ipRateLimiter
		handler, limiter = newRateLimitMiddleware(mux, rlCfg)
		cleanupStop, closeOnce := app.startRateLimitCleanup(limiter)
		app.jsonrpcRateLimitCleanupStop = cleanupStop
		app.jsonrpcRateLimitCloseOnce = closeOnce

		aliasLogger.Info(
			"JSON-RPC rate limiting enabled on public alias proxy",
			"rps", rlCfg.rps,
			"burst", rlCfg.burst,
			"entry_ttl", rlCfg.entryTTL,
		)
	}

	srv := &http.Server{
		Addr:              app.jsonrpcAliasPublicAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		ln, listenErr := net.Listen("tcp", app.jsonrpcAliasPublicAddr)
		if listenErr != nil {
			aliasLogger.Error("failed to listen for JSON-RPC alias proxy", "address", app.jsonrpcAliasPublicAddr, "error", listenErr)
			return
		}

		aliasLogger.Info(
			"JSON-RPC alias proxy started",
			"public_address", app.jsonrpcAliasPublicAddr,
			"upstream", app.jsonrpcAliasUpstreamAddr,
			"rate_limited", rlCfg != nil,
		)

		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			aliasLogger.Error("JSON-RPC alias proxy error", "error", serveErr)
		}
	}()

	app.jsonrpcAliasProxy = srv
}

func (app *App) stopJSONRPCAliasProxy() {
	if app.jsonrpcAliasProxy == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), jsonrpcAliasTimeout)
	defer cancel()

	if err := app.jsonrpcAliasProxy.Shutdown(ctx); err != nil {
		if app.App != nil {
			app.Logger().Error("failed to shutdown JSON-RPC alias proxy", "error", err)
		}
	}
	app.jsonrpcAliasProxy = nil
}

func rewriteJSONRPCDiscoverAlias(body []byte) []byte {
	replacer := strings.NewReplacer(
		`"method":"rpc.discover"`, `"method":"rpc_discover"`,
		`"method": "rpc.discover"`, `"method": "rpc_discover"`,
	)
	return []byte(replacer.Replace(string(body)))
}
