//go:build integration
// +build integration

package vm_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
)

// TestVMQueryParamsAndConfigBasic validates core `query evm params` and
// `query evm config` surfaces for Lumera-specific denom wiring and chain config
// consistency.
func testVMQueryParamsAndConfigBasic(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	paramsResp := mustQueryEVMParams(t, node)
	paramsMap, ok := paramsResp["params"].(map[string]any)
	if !ok {
		t.Fatalf("missing params payload in query evm params response: %#v", paramsResp)
	}

	evmdenom := strings.TrimSpace(fmt.Sprint(paramsMap["evm_denom"]))
	if evmdenom != lcfg.ChainDenom {
		t.Fatalf("unexpected evm_denom: got=%q want=%q", evmdenom, lcfg.ChainDenom)
	}

	extOpts, ok := paramsMap["extended_denom_options"].(map[string]any)
	if !ok {
		t.Fatalf("missing extended_denom_options in params: %#v", paramsMap)
	}
	extDenom := strings.TrimSpace(fmt.Sprint(extOpts["extended_denom"]))
	if extDenom != lcfg.ChainEVMExtendedDenom {
		t.Fatalf("unexpected extended denom: got=%q want=%q", extDenom, lcfg.ChainEVMExtendedDenom)
	}

	configResp := mustQueryEVMConfig(t, node)
	configMap, ok := configResp["config"].(map[string]any)
	if !ok {
		t.Fatalf("missing config payload in query evm config response: %#v", configResp)
	}

	configChainIDAny, ok := configMap["chain_id"]
	if !ok {
		t.Fatalf("missing config.chain_id in config response: %#v", configMap)
	}

	configChainID, err := parseUintFromAny(configChainIDAny)
	if err != nil {
		t.Fatalf("invalid config.chain_id type/value (%T): %v", configChainIDAny, err)
	}
	if configChainID == 0 {
		t.Fatalf("missing config.chain_id in config response: %#v", configMap)
	}
	if configChainID != lcfg.EVMChainID {
		t.Fatalf("unexpected config.chain_id: got=%d want=%d", configChainID, lcfg.EVMChainID)
	}
}

func parseUintFromAny(v any) (uint64, error) {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return 0, fmt.Errorf("empty value")
	}

	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse uint from %q: %w", s, err)
	}

	return n, nil
}
