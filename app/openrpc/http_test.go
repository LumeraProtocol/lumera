package openrpc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServeHTTPGet(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(nil) // nil = allow all origins
	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET, HEAD, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Content-Type", resp.Header.Get("Access-Control-Allow-Headers"))

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "1.2.6", payload["openrpc"])
}

func TestServeHTTPHead(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(nil)
	req := httptest.NewRequest(http.MethodHead, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Len(t, body, 0)
}

func TestServeHTTPMethodNotAllowed(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(nil)
	req := httptest.NewRequest(http.MethodPost, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	require.Equal(t, "GET, HEAD, OPTIONS", resp.Header.Get("Allow"))
}

func TestServeHTTPOptions(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET, HEAD, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Content-Type", resp.Header.Get("Access-Control-Allow-Headers"))
}

func TestServeHTTPCORSAllowedOrigin(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler([]string{"https://explorer.lumera.io", "https://docs.lumera.io"})

	// Allowed origin is echoed back.
	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	req.Header.Set("Origin", "https://explorer.lumera.io")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "https://explorer.lumera.io", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestServeHTTPCORSBlockedOrigin(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler([]string{"https://explorer.lumera.io"})

	// Unknown origin gets no CORS header.
	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestServeHTTPCORSNoOriginHeader(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler([]string{"https://explorer.lumera.io"})

	// No Origin header (curl, server-to-server) — allow through.
	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestServeHTTPCORSWildcardInList(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler([]string{"*"})

	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	req.Header.Set("Origin", "https://anything.example.com")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}
