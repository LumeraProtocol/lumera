package openrpc

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
)

const (
	// Namespace is the JSON-RPC namespace used by OpenRPC discovery (`rpc_discover`).
	Namespace  = "rpc"
	apiVersion = "1.0"
)

//go:embed openrpc.json.gz
var embeddedSpecGz []byte

var embeddedSpecRaw json.RawMessage

func init() {
	r, err := gzip.NewReader(bytes.NewReader(embeddedSpecGz))
	if err != nil {
		panic(fmt.Sprintf("openrpc: decompress embedded spec: %v", err))
	}
	defer func() { _ = r.Close() }()

	data, err := io.ReadAll(r)
	if err != nil {
		panic(fmt.Sprintf("openrpc: read decompressed spec: %v", err))
	}
	embeddedSpecRaw = data
}

// DiscoverDocument returns the embedded OpenRPC specification as a raw JSON object.
func DiscoverDocument() (json.RawMessage, error) {
	if !json.Valid(embeddedSpecRaw) {
		return nil, fmt.Errorf("embedded OpenRPC spec is not valid JSON")
	}
	// Return a copy to avoid accidental mutations by callers.
	return append(json.RawMessage(nil), embeddedSpecRaw...), nil
}
