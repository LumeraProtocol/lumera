package openrpc

import (
	"net/http"
	"strings"
)

const HTTPPath = "/openrpc.json"

const (
	allowMethods = "GET, HEAD, OPTIONS"
)

// NewHTTPHandler returns an http.HandlerFunc that serves the embedded OpenRPC
// document with CORS restricted to allowedOrigins. If the list is empty or
// contains "*", all origins are allowed (suitable for dev/testnet).
func NewHTTPHandler(allowedOrigins []string) http.HandlerFunc {
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(doc)
	}
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
