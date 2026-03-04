package openrpc

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const (
	// Namespace is the JSON-RPC namespace used by OpenRPC discovery (`rpc_discover`).
	Namespace  = "rpc"
	apiVersion = "1.0"
)

//go:embed openrpc.json
var embeddedSpec []byte

var embeddedSpecRaw json.RawMessage

func init() {
	embeddedSpecRaw = append(json.RawMessage(nil), embeddedSpec...)
}

// DiscoverDocument returns the embedded OpenRPC specification as a raw JSON object.
func DiscoverDocument() (json.RawMessage, error) {
	if !json.Valid(embeddedSpecRaw) {
		return nil, fmt.Errorf("embedded OpenRPC spec is not valid JSON")
	}
	// Return a copy to avoid accidental mutations by callers.
	return append(json.RawMessage(nil), embeddedSpecRaw...), nil
}
