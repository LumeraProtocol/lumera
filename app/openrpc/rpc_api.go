package openrpc

import "encoding/json"

// API exposes OpenRPC discovery over the JSON-RPC server.
type API struct{}

// Discover returns the full OpenRPC document for this node.
func (API) Discover() (json.RawMessage, error) {
	return DiscoverDocument()
}
