package openrpc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServeHTTPGet(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(nil, "") // nil = allow all origins
	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Content-Type", resp.Header.Get("Access-Control-Allow-Headers"))

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "1.2.6", payload["openrpc"])
}

func TestServeHTTPHead(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(nil, "")
	req := httptest.NewRequest(http.MethodHead, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Len(t, body, 0)
}

func TestServeHTTPMethodNotAllowed(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(nil, "")
	req := httptest.NewRequest(http.MethodPut, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	require.Equal(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get("Allow"))
}

func TestServeHTTPOptions(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(nil, "")
	req := httptest.NewRequest(http.MethodOptions, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Content-Type", resp.Header.Get("Access-Control-Allow-Headers"))
}

func TestServeHTTPCORSAllowedOrigin(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler([]string{"https://explorer.lumera.io", "https://docs.lumera.io"}, "")

	// Allowed origin is echoed back.
	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	req.Header.Set("Origin", "https://explorer.lumera.io")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "https://explorer.lumera.io", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestServeHTTPCORSBlockedOrigin(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler([]string{"https://explorer.lumera.io"}, "")

	// Unknown origin gets no CORS header.
	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestServeHTTPCORSNoOriginHeader(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler([]string{"https://explorer.lumera.io"}, "")

	// No Origin header (curl, server-to-server) — allow through.
	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestServeHTTPCORSWildcardInList(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler([]string{"*"}, "")

	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	req.Header.Set("Origin", "https://anything.example.com")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestServeHTTPServerURLRewrite(t *testing.T) {
	t.Parallel()

	// Simulate the REST API serving on :1337 with JSON-RPC on :8555.
	handler := NewHTTPHandler(nil, "0.0.0.0:8555")

	req := httptest.NewRequest(http.MethodGet, "http://localhost:1337"+HTTPPath, nil)
	req.Host = "localhost:1337"
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var spec struct {
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
	}
	require.NoError(t, json.Unmarshal(body, &spec))
	require.NotEmpty(t, spec.Servers, "spec must have servers")
	require.Equal(t, "http://localhost:8555", spec.Servers[0].URL,
		"servers[0].url must be rewritten to the JSON-RPC port")
}

func TestServeHTTPServerURLNoRewriteWhenEmpty(t *testing.T) {
	t.Parallel()

	// When jsonRPCAddr is empty, the servers URL should remain unchanged.
	handler := NewHTTPHandler(nil, "")

	req := httptest.NewRequest(http.MethodGet, "http://localhost:1337"+HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var spec struct {
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
	}
	require.NoError(t, json.Unmarshal(body, &spec))
	require.NotEmpty(t, spec.Servers)
	require.Equal(t, "http://localhost:8545", spec.Servers[0].URL,
		"servers[0].url must remain at embedded default when jsonRPCAddr is empty")
}

func TestServeHTTPPostProxiesJSONRPC(t *testing.T) {
	t.Parallel()

	var gotMethod string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		gotMethod = req.Method

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
	}))
	defer upstream.Close()

	handler := NewHTTPHandler(nil, strings.TrimPrefix(upstream.URL, "http://"))
	req := httptest.NewRequest(http.MethodPost, HTTPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "eth_chainId", gotMethod)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":"0x1"}`, string(body))
}

func TestServeHTTPPostRewritesRPCDiscoverAlias(t *testing.T) {
	t.Parallel()

	var gotMethod string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		gotMethod = req.Method

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer upstream.Close()

	handler := NewHTTPHandler(nil, strings.TrimPrefix(upstream.URL, "http://"))
	req := httptest.NewRequest(http.MethodPost, HTTPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"rpc.discover","params":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "rpc_discover", gotMethod)
}
