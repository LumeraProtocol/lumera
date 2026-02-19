package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	DefaultRequestTimeout = 2 * time.Second
	DefaultPollInterval   = 300 * time.Millisecond
)

var ErrEmptyResult = errors.New("empty rpc result")

type RPCError struct {
	Code    int
	Message string
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// Call executes a JSON-RPC request and unmarshals the result into out.
func Call(ctx context.Context, rpcURL, method string, params []any, out any) error {
	httpClient := &http.Client{Timeout: DefaultRequestTimeout}
	return CallWithClient(ctx, httpClient, rpcURL, method, params, out)
}

// CallWithClient executes a JSON-RPC request using a caller-provided HTTP client.
func CallWithClient(ctx context.Context, httpClient *http.Client, rpcURL, method string, params []any, out any) error {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	bodyBz, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(bodyBz))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return &RPCError{
			Code:    rpcResp.Error.Code,
			Message: rpcResp.Error.Message,
		}
	}
	if len(rpcResp.Result) == 0 {
		return ErrEmptyResult
	}

	return json.Unmarshal(rpcResp.Result, out)
}

// WaitFor repeatedly calls a JSON-RPC method until isReady returns true or ctx expires.
func WaitFor(
	ctx context.Context,
	rpcURL, method string,
	params []any,
	out any,
	pollInterval time.Duration,
	isReady func() bool,
) error {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		if err := Call(ctx, rpcURL, method, params, out); err != nil {
			lastErr = err
		} else if isReady() {
			return nil
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("wait for %s failed: %w", method, lastErr)
			}
			return fmt.Errorf("wait for %s failed: %w", method, ctx.Err())
		case <-ticker.C:
		}
	}
}
