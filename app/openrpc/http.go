package openrpc

import (
	"net/http"
)

const HTTPPath = "/openrpc.json"

const (
	allowMethods = "GET, HEAD, OPTIONS"
)

// ServeHTTP serves the embedded OpenRPC document over plain HTTP.
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Allow browser-based OpenRPC tooling to fetch the document cross-origin.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", allowMethods)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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
