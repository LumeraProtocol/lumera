package app

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"cosmossdk.io/log"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/spf13/cast"
	"golang.org/x/time/rate"

	textutil "github.com/LumeraProtocol/lumera/pkg/text"
)

const (
	jsonrpcRateLimitLogModule = "json-rpc-ratelimit"

	// App option keys matching the config template in cmd/lumera/cmd/config.go.
	rlOptEnable         = "lumera.json-rpc-ratelimit.enable"
	rlOptProxyAddr      = "lumera.json-rpc-ratelimit.proxy-address"
	rlOptRPS            = "lumera.json-rpc-ratelimit.requests-per-second"
	rlOptBurst          = "lumera.json-rpc-ratelimit.burst"
	rlOptEntryTTL       = "lumera.json-rpc-ratelimit.entry-ttl"
	rlOptTrustedProxies = "lumera.json-rpc-ratelimit.trusted-proxies"

	// Defaults (also in cmd/config.go; these are safety fallbacks).
	defaultRLProxyAddr = "0.0.0.0:8547"
	defaultRLRPS       = 50
	defaultRLBurst     = 100
	defaultRLEntryTTL  = 5 * time.Minute

	rlCleanupInterval = 1 * time.Minute
	rlShutdownTimeout = 5 * time.Second
)

// ipRateLimiter manages per-IP token bucket rate limiters with automatic expiry.
type ipRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*limiterEntry
	rps      rate.Limit
	burst    int
	ttl      time.Duration
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPRateLimiter(rps int, burst int, ttl time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		limiters: make(map[string]*limiterEntry),
		rps:      rate.Limit(rps),
		burst:    burst,
		ttl:      ttl,
	}
}

// getLimiter returns the rate limiter for the given IP, creating one if needed.
func (rl *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	entry, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if exists {
		rl.mu.Lock()
		entry.lastSeen = time.Now()
		rl.mu.Unlock()
		return entry.limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock.
	if entry, exists = rl.limiters[ip]; exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(rl.rps, rl.burst)
	rl.limiters[ip] = &limiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	return limiter
}

// cleanup removes entries that have not been seen within ttl.
func (rl *ipRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.ttl)
	for ip, entry := range rl.limiters {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.limiters, ip)
		}
	}
}

// rateLimitConfig holds parsed rate-limiting parameters.
type rateLimitConfig struct {
	rps            int
	burst          int
	entryTTL       time.Duration
	trustedProxies []*net.IPNet
}

// parseRateLimitConfig reads rate-limit settings from app options.
// Returns nil if rate limiting is disabled.
func parseRateLimitConfig(appOpts servertypes.AppOptions, logger log.Logger) *rateLimitConfig {
	if !textutil.ParseAppOptionBool(appOpts.Get(rlOptEnable)) {
		return nil
	}

	rlLogger := logger.With(log.ModuleKey, jsonrpcRateLimitLogModule)
	return &rateLimitConfig{
		rps:      castIntOr(appOpts.Get(rlOptRPS), defaultRLRPS),
		burst:    castIntOr(appOpts.Get(rlOptBurst), defaultRLBurst),
		entryTTL: castDurationOr(appOpts.Get(rlOptEntryTTL), defaultRLEntryTTL),
		trustedProxies: parseTrustedProxies(
			castStringOr(appOpts.Get(rlOptTrustedProxies), ""),
			rlLogger,
		),
	}
}

// newRateLimitMiddleware wraps an http.Handler with per-IP rate limiting.
// The returned cleanup channel and sync.Once must be used for lifecycle management.
func newRateLimitMiddleware(
	inner http.Handler,
	cfg *rateLimitConfig,
) (http.Handler, *ipRateLimiter) {
	limiter := newIPRateLimiter(cfg.rps, cfg.burst, cfg.entryTTL)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r, cfg.trustedProxies)
		if !limiter.getLimiter(ip).Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32005,"message":"rate limit exceeded"},"id":null}`))
			return
		}
		inner.ServeHTTP(w, r)
	})

	return handler, limiter
}

// startJSONRPCProxyStack starts the JSON-RPC proxy infrastructure.
// When the alias proxy is active (rpc.discover aliasing), rate limiting is
// injected directly into the alias proxy handler so the public port is always
// rate-limited. When the alias proxy is NOT active, a standalone rate-limit
// proxy is started on its own port as a fallback.
func (app *App) startJSONRPCProxyStack(appOpts servertypes.AppOptions, logger log.Logger) {
	rlCfg := parseRateLimitConfig(appOpts, logger)

	if app.jsonrpcAliasPublicAddr != "" {
		// Alias proxy is active — inject rate limiting into its handler.
		app.startJSONRPCAliasProxy(logger, rlCfg)
	} else if rlCfg != nil {
		// No alias proxy — start standalone rate-limit proxy on its own port.
		app.startStandaloneRateLimitProxy(appOpts, logger, rlCfg)
	}
}

// startStandaloneRateLimitProxy starts a rate-limiting reverse proxy on a
// separate port. Used only when the alias proxy is not active.
func (app *App) startStandaloneRateLimitProxy(appOpts servertypes.AppOptions, logger log.Logger, cfg *rateLimitConfig) {
	rlLogger := logger.With(log.ModuleKey, jsonrpcRateLimitLogModule)

	proxyAddr := castStringOr(appOpts.Get(rlOptProxyAddr), defaultRLProxyAddr)
	upstreamAddr := castStringOr(appOpts.Get("json-rpc.address"), "127.0.0.1:8545")
	upstreamURL, err := url.Parse("http://" + upstreamAddr)
	if err != nil {
		rlLogger.Error("failed to parse upstream JSON-RPC address", "address", upstreamAddr, "error", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	handler, limiter := newRateLimitMiddleware(proxy, cfg)

	srv := &http.Server{
		Addr:              proxyAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	cleanupStop, closeOnce := app.startRateLimitCleanup(limiter)

	go func() {
		ln, listenErr := net.Listen("tcp", proxyAddr)
		if listenErr != nil {
			rlLogger.Error("failed to listen for JSON-RPC rate limit proxy", "address", proxyAddr, "error", listenErr)
			closeOnce.Do(func() { close(cleanupStop) })
			return
		}

		rlLogger.Info(
			"JSON-RPC rate-limiting proxy started (standalone)",
			"proxy_address", proxyAddr,
			"upstream", upstreamAddr,
			"rps", cfg.rps,
			"burst", cfg.burst,
			"entry_ttl", cfg.entryTTL,
		)

		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			rlLogger.Error("JSON-RPC rate limit proxy error", "error", serveErr)
		}
	}()

	app.jsonrpcRateLimitProxy = srv
	app.jsonrpcRateLimitCleanupStop = cleanupStop
	app.jsonrpcRateLimitCloseOnce = closeOnce
}

// startRateLimitCleanup starts the background goroutine that evicts stale
// per-IP limiter entries. Returns the stop channel and sync.Once guard.
func (app *App) startRateLimitCleanup(limiter *ipRateLimiter) (chan struct{}, *sync.Once) {
	cleanupStop := make(chan struct{})
	closeOnce := sync.Once{}

	go func() {
		ticker := time.NewTicker(rlCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				limiter.cleanup()
			case <-cleanupStop:
				return
			}
		}
	}()

	return cleanupStop, &closeOnce
}

// stopJSONRPCRateLimitProxy gracefully shuts down the standalone proxy server
// (if any) and stops the rate-limit cleanup goroutine. The cleanup goroutine
// may exist even without a standalone proxy when rate limiting is injected
// into the alias proxy.
func (app *App) stopJSONRPCRateLimitProxy() {
	// Stop the cleanup goroutine regardless of whether a standalone proxy exists.
	if app.jsonrpcRateLimitCloseOnce != nil {
		app.jsonrpcRateLimitCloseOnce.Do(func() { close(app.jsonrpcRateLimitCleanupStop) })
	}

	if app.jsonrpcRateLimitProxy == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), rlShutdownTimeout)
	defer cancel()

	if err := app.jsonrpcRateLimitProxy.Shutdown(ctx); err != nil {
		if app.App != nil {
			app.Logger().Error("failed to shutdown JSON-RPC rate limit proxy", "error", err)
		}
	}
	app.jsonrpcRateLimitProxy = nil
}

// extractIP gets the client IP from the request. Forwarded headers
// (X-Forwarded-For, X-Real-IP) are only trusted when the direct peer
// (RemoteAddr) matches one of the configured trusted proxy CIDRs.
// When there are no trusted proxies or the peer is not trusted, the
// IP is always derived from RemoteAddr.
//
// X-Forwarded-For is parsed right-to-left, skipping entries that belong
// to trusted proxy CIDRs, and returns the rightmost non-trusted IP.
// This prevents a client from injecting a spoofed leftmost entry that
// an append-style proxy would leave untouched.
func extractIP(r *http.Request, trustedProxies []*net.IPNet) string {
	peerIP := peerIPFromRequest(r)

	if len(trustedProxies) == 0 || !isTrustedProxy(peerIP, trustedProxies) {
		return peerIP
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		entries := strings.Split(xff, ",")
		// Walk right-to-left: each trusted proxy appends the IP it
		// received the request from, so the rightmost non-trusted
		// entry is the real client.
		for i := len(entries) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(entries[i])
			if ip == "" {
				continue
			}
			if !isTrustedProxy(ip, trustedProxies) {
				return ip
			}
		}
		// Every entry is a trusted proxy — fall through to X-Real-IP / peer.
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	return peerIP
}

// peerIPFromRequest extracts the IP from RemoteAddr (host:port).
func peerIPFromRequest(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// isTrustedProxy checks whether ip falls within any of the trusted CIDR ranges.
func isTrustedProxy(ip string, trusted []*net.IPNet) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range trusted {
		if cidr.Contains(parsed) {
			return true
		}
	}
	return false
}

// parseTrustedProxies parses a comma-separated list of CIDRs (e.g.
// "10.0.0.0/8, 172.16.0.0/12"). Single IPs like "10.0.0.1" are treated
// as /32 (IPv4) or /128 (IPv6). Returns nil when the input is empty.
func parseTrustedProxies(raw string, logger log.Logger) []*net.IPNet {
	if raw == "" {
		return nil
	}

	var nets []*net.IPNet
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// If no CIDR mask is present, add one.
		if !strings.Contains(entry, "/") {
			if strings.Contains(entry, ":") {
				entry += "/128"
			} else {
				entry += "/32"
			}
		}

		_, cidr, err := net.ParseCIDR(entry)
		if err != nil {
			logger.Error("invalid trusted-proxies CIDR, skipping", "entry", entry, "error", err)
			continue
		}
		nets = append(nets, cidr)
	}
	return nets
}

// castStringOr converts an interface{} to string, returning fallback on failure.
func castStringOr(v interface{}, fallback string) string {
	s, err := cast.ToStringE(v)
	if err != nil || s == "" {
		return fallback
	}
	return s
}

// castIntOr converts an interface{} to int, returning fallback on failure.
func castIntOr(v interface{}, fallback int) int {
	i, err := cast.ToIntE(v)
	if err != nil || i <= 0 {
		return fallback
	}
	return i
}

// castDurationOr converts an interface{} to time.Duration, returning fallback on failure.
func castDurationOr(v interface{}, fallback time.Duration) time.Duration {
	d, err := cast.ToDurationE(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
