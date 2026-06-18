//go:build integration
// +build integration

package evmtest

import (
	"bytes"
	"strings"
	"testing"
)

func TestWaitForJSONRPCResultReturnsErrorOnCleanProcessExit(t *testing.T) {
	waitCh := make(chan error, 1)
	waitCh <- nil
	close(waitCh)

	err := waitForJSONRPCResult("http://127.0.0.1:1", waitCh, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error when process exits before JSON-RPC readiness")
	}
	if !strings.Contains(err.Error(), "node exited before json-rpc became ready") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForJSONRPCResultReturnsErrorOnClosedWaitChannel(t *testing.T) {
	waitCh := make(chan error)
	close(waitCh)

	err := waitForJSONRPCResult("http://127.0.0.1:1", waitCh, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error when wait channel closes before JSON-RPC readiness")
	}
	if !strings.Contains(err.Error(), "node wait channel closed before json-rpc became ready") {
		t.Fatalf("unexpected error: %v", err)
	}
}
