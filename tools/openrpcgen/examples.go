package main

import (
	"encoding/json"
	"os"
	"strings"
)

func alignExampleParamNames(examples []examplePairing, params []contentDescriptor) []examplePairing {
	if len(examples) == 0 {
		return nil
	}

	out := make([]examplePairing, 0, len(examples))
	for _, ex := range examples {
		copied := ex
		if copied.Params == nil {
			copied.Params = []exampleObject{}
		}
		if len(ex.Params) > 0 {
			copied.Params = make([]exampleObject, len(ex.Params))
			copy(copied.Params, ex.Params)

			if len(copied.Params) == len(params) {
				allIndexedArgs := true
				for _, p := range copied.Params {
					if !isIndexedArgName(p.Name) {
						allIndexedArgs = false
						break
					}
				}
				if allIndexedArgs {
					for i := range copied.Params {
						copied.Params[i].Name = params[i].Name
					}
				}
			}
		}
		out = append(out, copied)
	}

	return out
}

func loadExampleOverrides(path string) (map[string][]examplePairing, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return map[string][]examplePairing{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]examplePairing{}, nil
		}
		return nil, err
	}

	var out map[string][]examplePairing
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string][]examplePairing{}
	}
	return out, nil
}

func methodExamples(method string, params []contentDescriptor, result contentDescriptor) []examplePairing {
	switch method {
	case "eth_chainId":
		return []examplePairing{{
			Name:    "chain-id",
			Summary: "Returns the configured EVM chain ID in hex.",
			Result:  &exampleObject{Name: "result", Value: "0x494c1a9"},
		}}
	case "eth_blockNumber":
		return []examplePairing{{
			Name:    "latest-height",
			Summary: "Returns latest block number in hex.",
			Result:  &exampleObject{Name: "result", Value: "0x5"},
		}}
	case "net_version":
		return []examplePairing{{
			Name:    "network-id",
			Summary: "Returns network ID as decimal string.",
			Result:  &exampleObject{Name: "result", Value: "76874281"},
		}}
	case "net_listening":
		return []examplePairing{{
			Name:    "listening-status",
			Summary: "Returns whether the node P2P layer is listening.",
			Result:  &exampleObject{Name: "result", Value: true},
		}}
	case "eth_getBlockByNumber":
		return []examplePairing{{
			Name:    "latest-header-only",
			Summary: "Returns latest block object without full transactions.",
			Params: []exampleObject{
				{Name: "arg1", Value: "latest"},
				{Name: "arg2", Value: false},
			},
			Result: &exampleObject{
				Name: "result",
				Value: map[string]any{
					"number":        "0x5",
					"hash":          "0x4f1c8d5b8cf530f4c01f8ca07825f8f5084f57b9d7b5e0f8031f4bca8e1c83f4",
					"baseFeePerGas": "0x9502f900",
				},
			},
		}}
	case "eth_getBalance":
		return []examplePairing{{
			Name:    "account-balance-latest",
			Summary: "Returns 18-decimal EVM view balance in wei.",
			Params: []exampleObject{
				{Name: "arg1", Value: "0x1111111111111111111111111111111111111111"},
				{Name: "arg2", Value: "latest"},
			},
			Result: &exampleObject{Name: "result", Value: "0xde0b6b3a7640000"},
		}}
	case "eth_getTransactionCount":
		return []examplePairing{{
			Name:    "account-nonce",
			Summary: "Returns account nonce at selected block tag.",
			Params: []exampleObject{
				{Name: "arg1", Value: "0x1111111111111111111111111111111111111111"},
				{Name: "arg2", Value: "pending"},
			},
			Result: &exampleObject{Name: "result", Value: "0x3"},
		}}
	case "eth_feeHistory":
		return []examplePairing{{
			Name:    "single-block-fee-history",
			Summary: "Returns base fee history and optional reward percentiles.",
			Params: []exampleObject{
				{Name: "arg1", Value: "0x1"},
				{Name: "arg2", Value: "latest"},
				{Name: "arg3", Value: []any{50}},
			},
			Result: &exampleObject{
				Name: "result",
				Value: map[string]any{
					"oldestBlock":   "0x4",
					"baseFeePerGas": []any{"0x9502f900", "0x8f0d1800"},
					"gasUsedRatio":  []any{0.21},
					"reward":        []any{[]any{"0x3b9aca00"}},
				},
			},
		}}
	case "eth_getLogs":
		return []examplePairing{{
			Name:    "range-query",
			Summary: "Returns logs in a bounded block range (can be empty).",
			Params: []exampleObject{
				{Name: "arg1", Value: map[string]any{
					"fromBlock": "0x1",
					"toBlock":   "latest",
					"topics":    []any{},
				}},
			},
			Result: &exampleObject{Name: "result", Value: []any{}},
		}}
	case "eth_newBlockFilter":
		return []examplePairing{{
			Name:    "create-block-filter",
			Summary: "Creates a block filter and returns filter id.",
			Result:  &exampleObject{Name: "result", Value: "0x1"},
		}}
	case "eth_getFilterChanges":
		return []examplePairing{{
			Name:    "poll-filter",
			Summary: "Returns new entries since last poll for a filter id.",
			Params:  []exampleObject{{Name: "arg1", Value: "0x1"}},
			Result:  &exampleObject{Name: "result", Value: []any{}},
		}}
	case "eth_uninstallFilter":
		return []examplePairing{{
			Name:    "remove-filter",
			Summary: "Uninstalls an existing filter.",
			Params:  []exampleObject{{Name: "arg1", Value: "0x1"}},
			Result:  &exampleObject{Name: "result", Value: true},
		}}
	case "eth_getTransactionByHash":
		return []examplePairing{{
			Name:    "lookup-tx",
			Summary: "Returns tx object when indexed/persisted.",
			Params: []exampleObject{
				{Name: "arg1", Value: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			},
			Result: &exampleObject{
				Name: "result",
				Value: map[string]any{
					"hash":             "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"transactionIndex": "0x0",
					"blockNumber":      "0x5",
				},
			},
		}}
	case "eth_getTransactionReceipt":
		return []examplePairing{{
			Name:    "lookup-receipt",
			Summary: "Returns receipt for a mined transaction hash.",
			Params: []exampleObject{
				{Name: "arg1", Value: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			},
			Result: &exampleObject{
				Name: "result",
				Value: map[string]any{
					"transactionHash": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"status":          "0x1",
					"gasUsed":         "0x5208",
				},
			},
		}}
	case "eth_sendRawTransaction":
		return []examplePairing{{
			Name:    "broadcast-signed-tx",
			Summary: "Broadcasts a signed raw Ethereum tx; returns tx hash.",
			Params: []exampleObject{
				{
					Name:  "arg1",
					Value: "0x02f86a82053901843b9aca00849502f9008252089411111111111111111111111111111111111111110180c001a0aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
			},
			Result: &exampleObject{Name: "result", Value: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		}}
	case "txpool_status":
		return []examplePairing{{
			Name:    "txpool-counters",
			Summary: "Returns pending and queued tx counters from mempool.",
			Result: &exampleObject{Name: "result", Value: map[string]any{
				"pending": "0x1",
				"queued":  "0x0",
			}},
		}}
	case "web3_clientVersion":
		return []examplePairing{{
			Name:    "client-version",
			Summary: "Returns Cosmos EVM client version string.",
			Result:  &exampleObject{Name: "result", Value: "lumera/v1.12.0"},
		}}
	default:
		return []examplePairing{autoGeneratedExample(method, params, result)}
	}
}

func autoGeneratedExample(method string, params []contentDescriptor, result contentDescriptor) examplePairing {
	ex := examplePairing{
		Name:    "auto-generated",
		Summary: "Type-aware example generated from Go method signature.",
	}

	for _, p := range params {
		ex.Params = append(ex.Params, exampleObject{
			Name:  p.Name,
			Value: exampleValueForDescriptor(method, p, false),
		})
	}

	if resultType, _ := result.Schema["type"].(string); resultType == "null" {
		ex.Result = &exampleObject{Name: "result", Value: nil}
	} else {
		ex.Result = &exampleObject{
			Name:  "result",
			Value: exampleValueForDescriptor(method, result, true),
		}
	}

	return ex
}

func exampleValueForDescriptor(method string, d contentDescriptor, isResult bool) any {
	goType, _ := d.Schema["x-go-type"].(string)
	schemaType, _ := d.Schema["type"].(string)
	m := strings.ToLower(method)

	switch {
	case strings.Contains(goType, "common.Address"):
		return "0x1111111111111111111111111111111111111111"
	case strings.Contains(goType, "common.Hash"):
		return "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	case strings.Contains(goType, "rpc.ID"):
		return "0x1"
	case strings.Contains(goType, "types.BlockNumberOrHash"):
		return "latest"
	case strings.Contains(goType, "types.BlockNumber"):
		if isResult {
			return "0x5"
		}
		return "latest"
	case strings.Contains(goType, "filters.FilterCriteria"), strings.Contains(goType, "types.FilterCriteria"):
		return map[string]any{
			"fromBlock": "0x1",
			"toBlock":   "latest",
			"topics":    []any{},
		}
	case strings.Contains(goType, "types.TransactionArgs"):
		return map[string]any{
			"from":  "0x1111111111111111111111111111111111111111",
			"to":    "0x2222222222222222222222222222222222222222",
			"gas":   "0x5208",
			"value": "0x1",
			"input": "0x",
		}
	case d.Name == "overrides" && strings.Contains(goType, "json.RawMessage"):
		return map[string]any{
			"0x1111111111111111111111111111111111111111": map[string]any{
				"balance": "0x0",
			},
		}
	case strings.Contains(goType, "apitypes.TypedData"):
		return map[string]any{
			"types": map[string]any{
				"EIP712Domain": []any{
					map[string]any{"name": "name", "type": "string"},
				},
			},
			"domain":      map[string]any{"name": "Lumera"},
			"primaryType": "EIP712Domain",
			"message":     map[string]any{"name": "Lumera"},
		}
	case strings.Contains(goType, "json.RawMessage"):
		return map[string]any{}
	case strings.Contains(goType, "hexutil.Bytes"):
		return "0x"
	case strings.Contains(goType, "hexutil.Big"):
		return "0x1"
	case strings.Contains(goType, "hexutil.Uint"):
		return "0x1"
	case strings.Contains(goType, "[]float64"):
		return []any{50}
	}

	switch {
	case strings.HasPrefix(m, "eth_getblock"):
		if isResult {
			return map[string]any{
				"number": "0x5",
				"hash":   "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			}
		}
	case strings.Contains(m, "receipt"):
		if isResult {
			return map[string]any{
				"status": "0x1",
			}
		}
	case strings.Contains(m, "transaction"):
		if isResult {
			return map[string]any{
				"hash": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			}
		}
	}

	switch schemaType {
	case "boolean":
		return true
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	case "null":
		return nil
	default:
		return "0x1"
	}
}
