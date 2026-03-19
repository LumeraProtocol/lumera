package openrpc

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const HTTPPath = "/openrpc.json"

const (
	allowMethods = "GET, HEAD, POST, OPTIONS"
)

// NewHTTPHandler returns an http.HandlerFunc that serves the embedded OpenRPC
// document with CORS restricted to allowedOrigins. If the list is empty or
// contains "*", all origins are allowed (suitable for dev/testnet).
//
// jsonRPCAddr is the address of the JSON-RPC server (e.g. "127.0.0.1:8545").
// The handler rewrites the spec's servers[0].url to point to this address so
// that tools can discover the intended transport URL. The handler also accepts
// POST and forwards JSON-RPC calls to the local JSON-RPC server. This keeps the
// OpenRPC Playground working even when it POSTs back to `/openrpc.json` on the
// REST port instead of using servers[0].url directly.
func NewHTTPHandler(allowedOrigins []string, jsonRPCAddr string) http.HandlerFunc {
	// Build a fast lookup set. An empty list or a "*" entry means allow-all.
	allowAll := len(allowedOrigins) == 0
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "*" {
			allowAll = true
		}
		originSet[strings.ToLower(o)] = struct{}{}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		corsOrigin := resolveCORSOrigin(origin, allowAll, originSet)

		if corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
			w.Header().Set("Access-Control-Allow-Methods", allowMethods)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method == http.MethodPost {
			if err := proxyJSONRPC(w, r, jsonRPCAddr); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
			}
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", allowMethods)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		doc, err := DiscoverDocument()
		if err != nil {
			http.Error(w, "failed to load OpenRPC document", http.StatusInternalServerError)
			return
		}

		// Rewrite the spec's servers[0].url to point to the JSON-RPC port
		// so the OpenRPC Playground sends method calls to the right endpoint.
		if jsonRPCAddr != "" {
			doc = rewriteServerURL(doc, r, jsonRPCAddr)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(doc)
	}
}

func proxyJSONRPC(w http.ResponseWriter, r *http.Request, jsonRPCAddr string) error {
	if jsonRPCAddr == "" {
		return io.EOF
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	_ = r.Body.Close()

	body = rewriteRPCDiscoverAlias(body)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "http://"+jsonRPCAddr, strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	for key, values := range r.Header {
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}

	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	for key, values := range resp.Header {
		if strings.HasPrefix(strings.ToLower(key), "access-control-") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}

func rewriteRPCDiscoverAlias(body []byte) []byte {
	replacer := strings.NewReplacer(
		`"method":"rpc.discover"`, `"method":"rpc_discover"`,
		`"method": "rpc.discover"`, `"method": "rpc_discover"`,
	)
	return []byte(replacer.Replace(string(body)))
}

// rewriteServerURL replaces the server URL in the OpenRPC spec using a
// targeted string replacement. This avoids full JSON unmarshal/remarshal
// which would reorder keys alphabetically and break the OpenRPC Playground
// (which expects "openrpc" as the first field).
func rewriteServerURL(doc json.RawMessage, r *http.Request, jsonRPCAddr string) json.RawMessage {
	// Determine scheme from the incoming request.
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}

	// Build the JSON-RPC URL using the request's host for the hostname
	// part and the JSON-RPC address for the port. This handles devnet
	// port mappings where the request comes via localhost:1337 but the
	// JSON-RPC port is localhost:8555.
	host := r.Host
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	port := jsonRPCAddr
	if idx := strings.LastIndex(port, ":"); idx >= 0 {
		port = port[idx+1:]
	}

	newURL := scheme + "://" + host + ":" + port

	// The embedded spec contains a known server URL pattern. Replace it
	// with a targeted byte substitution to preserve JSON key order.
	const defaultURL = `"url": "http://localhost:8545"`
	replacement := `"url": "` + newURL + `"`
	return json.RawMessage(strings.Replace(string(doc), defaultURL, replacement, 1))
}

// resolveCORSOrigin returns the value for Access-Control-Allow-Origin.
// It returns "*" when all origins are allowed, the request origin when it
// matches the allowlist, or "" when the origin is not permitted.
func resolveCORSOrigin(origin string, allowAll bool, originSet map[string]struct{}) string {
	if allowAll {
		return "*"
	}
	if origin == "" {
		// Non-browser requests (curl, etc.) have no Origin header.
		// Allow them through — CORS is a browser-enforced mechanism.
		return "*"
	}
	if _, ok := originSet[strings.ToLower(origin)]; ok {
		return origin
	}
	return ""
}
