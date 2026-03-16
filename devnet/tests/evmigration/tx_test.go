package main

import (
	"testing"
	"time"
)

func TestTxWaitClientConfig(t *testing.T) {
	oldChainID := *flagChainID
	oldRPC := *flagRPC
	oldGRPC := *flagGRPC
	defer func() {
		*flagChainID = oldChainID
		*flagRPC = oldRPC
		*flagGRPC = oldGRPC
	}()

	*flagChainID = "lumera-devnet-1"
	*flagRPC = "tcp://localhost:26657"
	*flagGRPC = ""

	cfg := txWaitClientConfig()
	if cfg.ChainID != "lumera-devnet-1" {
		t.Fatalf("unexpected chain id: %s", cfg.ChainID)
	}
	if cfg.GRPCAddr != "localhost:9090" {
		t.Fatalf("unexpected grpc addr: %s", cfg.GRPCAddr)
	}
	if cfg.RPCEndpoint != "http://localhost:26657" {
		t.Fatalf("unexpected rpc endpoint: %s", cfg.RPCEndpoint)
	}
	if cfg.WaitTx.PollInterval != time.Second || cfg.WaitTx.PollMaxRetries != 0 {
		t.Fatalf("unexpected wait-tx config: interval=%s retries=%d", cfg.WaitTx.PollInterval, cfg.WaitTx.PollMaxRetries)
	}
}
