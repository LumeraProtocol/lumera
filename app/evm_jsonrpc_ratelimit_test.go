package app

import (
	"net"
	"net/http"
	"sync"
	"testing"

	"cosmossdk.io/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// P1: extractIP — trusted proxy header spoofing prevention
// ---------------------------------------------------------------------------

func mustParseCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	require.NoError(t, err)
	return n
}

func newRequest(remoteAddr string, headers map[string]string) *http.Request {
	r := &http.Request{
		RemoteAddr: remoteAddr,
		Header:     http.Header{},
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestExtractIP_NoTrustedProxies_IgnoresHeaders(t *testing.T) {
	r := newRequest("203.0.113.50:12345", map[string]string{
		"X-Forwarded-For": "10.1.1.1, 10.2.2.2",
		"X-Real-IP":       "10.1.1.1",
	})

	ip := extractIP(r, nil)
	assert.Equal(t, "203.0.113.50", ip)
}

func TestExtractIP_UntrustedPeer_IgnoresHeaders(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("203.0.113.50:12345", map[string]string{
		"X-Forwarded-For": "192.168.1.1",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "203.0.113.50", ip)
}

func TestExtractIP_TrustedPeer_RightToLeftXFF(t *testing.T) {
	// Trusted proxy appends real client IP. The rightmost non-trusted
	// entry is the real client, not the leftmost (which is spoofable).
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.1:9999", map[string]string{
		"X-Forwarded-For": "203.0.113.50, 10.0.0.1",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "203.0.113.50", ip,
		"rightmost non-trusted IP should be returned")
}

func TestExtractIP_SpoofedLeftmostXFF_ReturnsRealClient(t *testing.T) {
	// Attack: client injects a spoofed X-Forwarded-For header.
	// Client 198.51.100.10 sends: X-Forwarded-For: 1.2.3.4
	// Trusted proxy appends real client IP:
	//   X-Forwarded-For: 1.2.3.4, 198.51.100.10
	// Right-to-left parsing skips no trusted entries in the middle,
	// so it returns 198.51.100.10 (the real client), not 1.2.3.4.
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.1:9999", map[string]string{
		"X-Forwarded-For": "1.2.3.4, 198.51.100.10",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "198.51.100.10", ip,
		"must return the rightmost non-trusted IP, not the spoofed leftmost")
}

func TestExtractIP_MultiHopTrustedChain(t *testing.T) {
	// Client → proxy1 (10.0.0.1) → proxy2 (10.0.0.2) → app
	// XFF: "198.51.100.10, 10.0.0.1"
	// Both 10.x are trusted; rightmost non-trusted is the real client.
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.2:9999", map[string]string{
		"X-Forwarded-For": "198.51.100.10, 10.0.0.1",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "198.51.100.10", ip)
}

func TestExtractIP_SpoofedLeftmostWithMultiHop(t *testing.T) {
	// Attack with multi-hop: client 198.51.100.10 sends XFF: 1.2.3.4
	// proxy1 (10.0.0.1) appends client IP → "1.2.3.4, 198.51.100.10"
	// proxy2 (10.0.0.2) appends proxy1 IP → "1.2.3.4, 198.51.100.10, 10.0.0.1"
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.2:9999", map[string]string{
		"X-Forwarded-For": "1.2.3.4, 198.51.100.10, 10.0.0.1",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "198.51.100.10", ip,
		"must skip trusted 10.0.0.1 and return 198.51.100.10, not spoofed 1.2.3.4")
}

func TestExtractIP_AllXFFEntriesTrusted_FallsBackToXRealIP(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.1:9999", map[string]string{
		"X-Forwarded-For": "10.0.0.5, 10.0.0.6",
		"X-Real-IP":       "203.0.113.99",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "203.0.113.99", ip,
		"when all XFF entries are trusted, should fall back to X-Real-IP")
}

func TestExtractIP_AllXFFEntriesTrusted_NoXRealIP_FallsBackToPeer(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.1:9999", map[string]string{
		"X-Forwarded-For": "10.0.0.5, 10.0.0.6",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "10.0.0.1", ip)
}

func TestExtractIP_TrustedPeer_UsesXRealIP(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.1:9999", map[string]string{
		"X-Real-IP": "203.0.113.99",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "203.0.113.99", ip)
}

func TestExtractIP_TrustedPeer_NoHeaders_FallsBackToRemoteAddr(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.1:9999", nil)

	ip := extractIP(r, trusted)
	assert.Equal(t, "10.0.0.1", ip)
}

func TestExtractIP_TrustedPeer_TrimsWhitespace(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.1:9999", map[string]string{
		"X-Forwarded-For": "  203.0.113.50 , 10.0.0.1",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "203.0.113.50", ip)
}

func TestExtractIP_SingleXFFEntry(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR(t, "10.0.0.0/8")}

	r := newRequest("10.0.0.1:9999", map[string]string{
		"X-Forwarded-For": "203.0.113.50",
	})

	ip := extractIP(r, trusted)
	assert.Equal(t, "203.0.113.50", ip)
}

func TestExtractIP_RemoteAddrWithoutPort(t *testing.T) {
	r := newRequest("203.0.113.50", nil)

	ip := extractIP(r, nil)
	assert.Equal(t, "203.0.113.50", ip)
}

// ---------------------------------------------------------------------------
// isTrustedProxy
// ---------------------------------------------------------------------------

func TestIsTrustedProxy(t *testing.T) {
	trusted := []*net.IPNet{
		mustParseCIDR(t, "10.0.0.0/8"),
		mustParseCIDR(t, "172.16.0.0/12"),
	}

	tests := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", false},
		{"203.0.113.50", false},
		{"not-an-ip", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.ip, func(t *testing.T) {
			assert.Equal(t, tc.expected, isTrustedProxy(tc.ip, trusted))
		})
	}
}

// ---------------------------------------------------------------------------
// parseTrustedProxies
// ---------------------------------------------------------------------------

func TestParseTrustedProxies(t *testing.T) {
	logger := log.NewNopLogger()

	t.Run("empty string returns nil", func(t *testing.T) {
		result := parseTrustedProxies("", logger)
		assert.Nil(t, result)
	})

	t.Run("single CIDR", func(t *testing.T) {
		result := parseTrustedProxies("10.0.0.0/8", logger)
		require.Len(t, result, 1)
		assert.Equal(t, "10.0.0.0/8", result[0].String())
	})

	t.Run("multiple CIDRs with spaces", func(t *testing.T) {
		result := parseTrustedProxies("10.0.0.0/8, 172.16.0.0/12 , 192.168.0.0/16", logger)
		require.Len(t, result, 3)
	})

	t.Run("single IP auto-mask /32", func(t *testing.T) {
		result := parseTrustedProxies("10.0.0.1", logger)
		require.Len(t, result, 1)
		assert.Equal(t, "10.0.0.1/32", result[0].String())
	})

	t.Run("IPv6 single IP auto-mask /128", func(t *testing.T) {
		result := parseTrustedProxies("::1", logger)
		require.Len(t, result, 1)
		assert.Equal(t, "::1/128", result[0].String())
	})

	t.Run("invalid entry skipped", func(t *testing.T) {
		result := parseTrustedProxies("10.0.0.0/8, not-a-cidr, 172.16.0.0/12", logger)
		require.Len(t, result, 2)
	})

	t.Run("trailing comma ignored", func(t *testing.T) {
		result := parseTrustedProxies("10.0.0.0/8,", logger)
		require.Len(t, result, 1)
	})
}

// ---------------------------------------------------------------------------
// P2: stopJSONRPCRateLimitProxy — double-close prevention via sync.Once
//
// This exercises the real App fields (jsonrpcRateLimitProxy,
// jsonrpcRateLimitCleanupStop, jsonrpcRateLimitCloseOnce) to verify the
// production shutdown path does not panic when the cleanup channel was
// already closed by a startup failure.
// ---------------------------------------------------------------------------

func TestStopJSONRPCRateLimitProxy_AfterListenFailure_NoPanic(t *testing.T) {
	// Create a minimal App with the rate-limit fields wired up exactly
	// as startJSONRPCRateLimitProxy would.
	cleanupStop := make(chan struct{})
	closeOnce := &sync.Once{}

	// Start a real HTTP server so that Shutdown() has something to close.
	srv := &http.Server{Handler: http.NewServeMux()}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(ln) }()

	a := &App{}
	a.jsonrpcRateLimitProxy = srv
	a.jsonrpcRateLimitCleanupStop = cleanupStop
	a.jsonrpcRateLimitCloseOnce = closeOnce

	// Simulate the listen-failure goroutine path: it closes the channel
	// via the Once before stopJSONRPCRateLimitProxy runs.
	closeOnce.Do(func() { close(cleanupStop) })

	// Now call the real shutdown method. Without the sync.Once guard this
	// would panic with "close of closed channel".
	assert.NotPanics(t, func() {
		a.stopJSONRPCRateLimitProxy()
	})

	// Verify the proxy reference was nil-ed out (shutdown completed).
	assert.Nil(t, a.jsonrpcRateLimitProxy)
}

func TestStopJSONRPCRateLimitProxy_NormalShutdown(t *testing.T) {
	// Normal path: no prior close — stopJSONRPCRateLimitProxy should
	// close the channel and shut down the server cleanly.
	cleanupStop := make(chan struct{})
	closeOnce := &sync.Once{}

	srv := &http.Server{Handler: http.NewServeMux()}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(ln) }()

	a := &App{}
	a.jsonrpcRateLimitProxy = srv
	a.jsonrpcRateLimitCleanupStop = cleanupStop
	a.jsonrpcRateLimitCloseOnce = closeOnce

	assert.NotPanics(t, func() {
		a.stopJSONRPCRateLimitProxy()
	})

	assert.Nil(t, a.jsonrpcRateLimitProxy)

	// Verify the channel was actually closed.
	select {
	case <-cleanupStop:
		// ok
	default:
		t.Fatal("cleanup channel should be closed after normal shutdown")
	}
}

func TestStopJSONRPCRateLimitProxy_NilProxy_Noop(t *testing.T) {
	// When proxy was never started, stop should be a no-op.
	a := &App{}
	assert.NotPanics(t, func() {
		a.stopJSONRPCRateLimitProxy()
	})
}

// ---------------------------------------------------------------------------
// peerIPFromRequest
// ---------------------------------------------------------------------------

func TestPeerIPFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{"host:port", "192.168.1.1:8080", "192.168.1.1"},
		{"IPv6 with port", "[::1]:8080", "::1"},
		{"no port", "192.168.1.1", "192.168.1.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &http.Request{RemoteAddr: tc.remoteAddr}
			assert.Equal(t, tc.expected, peerIPFromRequest(r))
		})
	}
}
