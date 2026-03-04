//go:build integration
// +build integration

package jsonrpc_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	appopenrpc "github.com/LumeraProtocol/lumera/app/openrpc"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// openRPCDoc captures the OpenRPC fields used by integration assertions.
type openRPCDoc struct {
	OpenRPC string `json:"openrpc"`
	Info    struct {
		Title string `json:"title"`
	} `json:"info"`
	Methods []struct {
		Name string `json:"name"`
	} `json:"methods"`
}

const defaultAPIURL = "http://127.0.0.1:1317"

// testOpenRPCDiscoverMethodCatalog verifies `rpc_discover` returns a populated
// method catalog with expected namespace coverage.
//
// Coverage matrix:
// 1. OpenRPC metadata is non-empty.
// 2. Method catalog has no empty/duplicate method names.
// 3. Methods from enabled namespaces are present in the catalog.
func testOpenRPCDiscoverMethodCatalog(t *testing.T, node *evmtest.Node) {
	t.Helper()

	doc := mustDiscoverOpenRPCDoc(t, node)
	if strings.TrimSpace(doc.OpenRPC) == "" {
		t.Fatalf("rpc_discover returned empty openrpc version")
	}
	if strings.TrimSpace(doc.Info.Title) == "" {
		t.Fatalf("rpc_discover returned empty info.title")
	}
	if len(doc.Methods) < 50 {
		t.Fatalf("rpc_discover returned too few methods: got=%d want>=50", len(doc.Methods))
	}

	seen := make(map[string]struct{}, len(doc.Methods))
	for _, method := range doc.Methods {
		name := strings.TrimSpace(method.Name)
		if name == "" {
			t.Fatalf("rpc_discover returned a method with empty name")
		}
		if _, ok := seen[name]; ok {
			t.Fatalf("rpc_discover returned duplicate method name %q", name)
		}
		seen[name] = struct{}{}
	}

	requiredMethods := []string{
		"rpc_discover",
		"eth_chainId",
		"net_version",
		"web3_clientVersion",
		"txpool_status",
		"debug_traceTransaction",
		"personal_listAccounts",
	}
	for _, method := range requiredMethods {
		if _, ok := seen[method]; !ok {
			t.Fatalf("rpc_discover output does not include required method %q", method)
		}
	}

	// Cross-check that runtime module discovery includes the dedicated
	// OpenRPC namespace so downstream tooling can call rpc_discover.
	var modules map[string]string
	node.MustJSONRPC(t, "rpc_modules", []any{}, &modules)
	if _, ok := modules[appopenrpc.Namespace]; !ok {
		t.Fatalf("rpc_modules does not expose %q namespace (modules=%v)", appopenrpc.Namespace, modules)
	}
}

// testOpenRPCDiscoverMatchesEmbeddedSpec checks that runtime `rpc_discover`
// serves the same method catalog that is embedded into the node binary.
func testOpenRPCDiscoverMatchesEmbeddedSpec(t *testing.T, node *evmtest.Node) {
	t.Helper()

	runtimeDoc := mustDiscoverOpenRPCDoc(t, node)
	embeddedRaw, err := appopenrpc.DiscoverDocument()
	if err != nil {
		t.Fatalf("load embedded openrpc doc: %v", err)
	}

	var embeddedDoc openRPCDoc
	if err := json.Unmarshal(embeddedRaw, &embeddedDoc); err != nil {
		t.Fatalf("decode embedded openrpc doc: %v", err)
	}

	if runtimeDoc.OpenRPC != embeddedDoc.OpenRPC {
		t.Fatalf("openrpc version mismatch runtime=%q embedded=%q", runtimeDoc.OpenRPC, embeddedDoc.OpenRPC)
	}
	if strings.TrimSpace(runtimeDoc.Info.Title) != strings.TrimSpace(embeddedDoc.Info.Title) {
		t.Fatalf("openrpc title mismatch runtime=%q embedded=%q", runtimeDoc.Info.Title, embeddedDoc.Info.Title)
	}

	runtimeMethods := methodNameSet(runtimeDoc)
	embeddedMethods := methodNameSet(embeddedDoc)

	if len(runtimeMethods) != len(embeddedMethods) {
		t.Fatalf("openrpc method count mismatch runtime=%d embedded=%d", len(runtimeMethods), len(embeddedMethods))
	}

	for method := range embeddedMethods {
		if _, ok := runtimeMethods[method]; !ok {
			t.Fatalf("runtime rpc_discover is missing embedded method %q", method)
		}
	}
	for method := range runtimeMethods {
		if _, ok := embeddedMethods[method]; !ok {
			t.Fatalf("runtime rpc_discover returned unexpected method %q", method)
		}
	}
}

// TestOpenRPCHTTPDocumentEndpoint validates that `/openrpc.json` is served by
// the API server when API mode is enabled, and that it matches `rpc_discover`.
func TestOpenRPCHTTPDocumentEndpoint(t *testing.T) {
	t.Helper()

	node := evmtest.NewEVMNode(t, "lumera-openrpc-http", 120)
	node.AppendStartArgs("--api.enable=true")
	node.StartAndWaitRPC()
	defer node.Stop()

	httpDoc := mustFetchOpenRPCDocOverHTTP(t, defaultAPIURL+appopenrpc.HTTPPath, 20*time.Second)
	rpcDoc := mustDiscoverOpenRPCDoc(t, node)

	httpMethods := methodNameSet(httpDoc)
	rpcMethods := methodNameSet(rpcDoc)
	if len(httpMethods) != len(rpcMethods) {
		t.Fatalf("openrpc method count mismatch http=%d rpc_discover=%d", len(httpMethods), len(rpcMethods))
	}

	for method := range httpMethods {
		if _, ok := rpcMethods[method]; !ok {
			t.Fatalf("http /openrpc.json contains method missing in rpc_discover: %q", method)
		}
	}
	for method := range rpcMethods {
		if _, ok := httpMethods[method]; !ok {
			t.Fatalf("rpc_discover contains method missing in http /openrpc.json: %q", method)
		}
	}
}

func mustDiscoverOpenRPCDoc(t *testing.T, node *evmtest.Node) openRPCDoc {
	t.Helper()

	var doc openRPCDoc
	node.MustJSONRPC(t, "rpc_discover", []any{}, &doc)
	return doc
}

func methodNameSet(doc openRPCDoc) map[string]struct{} {
	out := make(map[string]struct{}, len(doc.Methods))
	for _, method := range doc.Methods {
		name := strings.TrimSpace(method.Name)
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

func mustFetchOpenRPCDocOverHTTP(t *testing.T, endpoint string, timeout time.Duration) openRPCDoc {
	t.Helper()

	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			t.Fatalf("build openrpc request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			time.Sleep(300 * time.Millisecond)
			continue
		}

		var doc openRPCDoc
		if err := json.Unmarshal(body, &doc); err != nil {
			lastErr = fmt.Errorf("decode /openrpc.json: %w", err)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if len(doc.Methods) == 0 {
			lastErr = fmt.Errorf("openrpc document has no methods")
			time.Sleep(300 * time.Millisecond)
			continue
		}

		return doc
	}

	t.Fatalf("failed to fetch /openrpc.json from %s within %s: %v", endpoint, timeout, lastErr)
	return openRPCDoc{}
}
