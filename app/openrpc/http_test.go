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

	req := httptest.NewRequest(http.MethodGet, HTTPPath, nil)
	rec := httptest.NewRecorder()
	ServeHTTP(rec, req)

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

	req := httptest.NewRequest(http.MethodHead, HTTPPath, nil)
	rec := httptest.NewRecorder()
	ServeHTTP(rec, req)

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

	req := httptest.NewRequest(http.MethodPost, HTTPPath, nil)
	rec := httptest.NewRecorder()
	ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	require.Equal(t, "GET, HEAD, OPTIONS", resp.Header.Get("Allow"))
}

func TestServeHTTPOptions(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodOptions, HTTPPath, nil)
	rec := httptest.NewRecorder()
	ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET, HEAD, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Content-Type", resp.Header.Get("Access-Control-Allow-Headers"))
}
