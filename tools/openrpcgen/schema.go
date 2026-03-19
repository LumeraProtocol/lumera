package main

import (
	"reflect"
	"strings"
)

// maxSchemaDepth limits struct expansion to avoid infinite recursion on
// self-referential or deeply nested types.
const maxSchemaDepth = 3

type descriptorOverride struct {
	Description string
	Schema      map[string]any
	Required    *bool
}

// ethereumTypeOverrides maps well-known Ethereum/go-ethereum types to their
// JSON-RPC wire representation. Without these, reflect sees common.Address as
// [20]byte (array), hexutil.Big as big.Int (struct), etc.
var ethereumTypeOverrides = map[string]map[string]any{
	"common.Address": {
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]{40}$",
		"description": "Hex-encoded Ethereum address (20 bytes)",
	},
	"common.Hash": {
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]{64}$",
		"description": "Hex-encoded 256-bit hash",
	},
	"hexutil.Big": {
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]+$",
		"description": "Hex-encoded big integer",
	},
	"hexutil.Uint64": {
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]+$",
		"description": "Hex-encoded uint64",
	},
	"hexutil.Uint": {
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]+$",
		"description": "Hex-encoded unsigned integer",
	},
	"hexutil.Bytes": {
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]*$",
		"description": "Hex-encoded byte array",
	},
	"types.BlockNumber": {
		"type":        "string",
		"description": "Block number: hex integer or tag (\"latest\", \"earliest\", \"pending\", \"safe\", \"finalized\")",
	},
	"types.BlockNumberOrHash": {
		"type":        "string",
		"description": "Block number (hex) or block hash (0x-prefixed 32-byte hex), optionally with requireCanonical flag",
	},
	"types.AccessList":           accessListSchema(),
	"uint256.Int":                uint256Schema("Hex-encoded 256-bit unsigned integer"),
	"kzg4844.Blob":               blobSchema(),
	"kzg4844.Commitment":         commitmentSchema(),
	"kzg4844.Proof":              proofSchema(),
	"types.SetCodeAuthorization": setCodeAuthorizationSchema(),
	"types.TransactionArgs":      transactionArgsSchema(),
	"filters.FilterCriteria":     filterCriteriaSchema(),
}

func paramDescriptorOverride(methodName, paramName string, t reflect.Type) *descriptorOverride {
	typeName := typeNameWithoutPointers(t)
	if paramName == "overrides" && typeName == "json.RawMessage" {
		return &descriptorOverride{
			Description: "Optional ephemeral state overrides applied only while executing this call.",
			Schema:      stateOverrideSchema(),
		}
	}
	return nil
}

func schemaForType(t reflect.Type) map[string]any {
	return schemaForTypeRecursive(t, 0)
}

func typeNameWithoutPointers(t reflect.Type) string {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.String()
}

func addressSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]{40}$",
		"description": description,
		"x-go-type":   "common.Address",
	}
}

func hashSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]{64}$",
		"description": description,
		"x-go-type":   "common.Hash",
	}
}

func hexBigSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]+$",
		"description": description,
		"x-go-type":   "hexutil.Big",
	}
}

func hexUint64Schema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]+$",
		"description": description,
		"x-go-type":   "hexutil.Uint64",
	}
}

func hexBytesSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]*$",
		"description": description,
		"x-go-type":   "hexutil.Bytes",
	}
}

func uint256Schema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]+$",
		"description": description,
		"x-go-type":   "uint256.Int",
	}
}

func blockTagSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description + ` Use a hex block number or one of "latest", "earliest", "pending", "safe", or "finalized".`,
	}
}

func accessListSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"address": map[string]any{
					"type":        "string",
					"pattern":     "^0x[0-9a-fA-F]{40}$",
					"description": "Account address",
				},
				"storageKeys": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":    "string",
						"pattern": "^0x[0-9a-fA-F]{64}$",
					},
					"description": "Storage slot keys",
				},
			},
		},
		"description": "EIP-2930 access list",
		"x-go-type":   "types.AccessList",
	}
}

func blobSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]*$",
		"minLength":   262146,
		"maxLength":   262146,
		"description": "EIP-4844 blob payload encoded as 0x-prefixed hex (131072 bytes).",
		"x-go-type":   "kzg4844.Blob",
	}
}

func commitmentSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]*$",
		"minLength":   98,
		"maxLength":   98,
		"description": "EIP-4844 KZG commitment encoded as 0x-prefixed hex (48 bytes).",
		"x-go-type":   "kzg4844.Commitment",
	}
}

func proofSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"pattern":     "^0x[0-9a-fA-F]*$",
		"minLength":   98,
		"maxLength":   98,
		"description": "EIP-4844 KZG proof encoded as 0x-prefixed hex (48 bytes).",
		"x-go-type":   "kzg4844.Proof",
	}
}

func setCodeAuthorizationSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "EIP-7702 set-code authorization.",
		"required":    []string{"chainId", "address", "nonce", "yParity", "r", "s"},
		"properties": map[string]any{
			"chainId": uint256Schema("Chain ID this authorization is valid for."),
			"address": addressSchema("Account authorizing code delegation."),
			"nonce": map[string]any{
				"type":        "string",
				"pattern":     "^0x[0-9a-fA-F]+$",
				"description": "Authorization nonce encoded as a hex uint64.",
				"x-go-type":   "uint64",
			},
			"yParity": map[string]any{
				"type":        "string",
				"pattern":     "^0x[0-9a-fA-F]+$",
				"description": "Signature y-parity encoded as a hex uint64.",
				"x-go-type":   "uint8",
			},
			"r": uint256Schema("Signature r value."),
			"s": uint256Schema("Signature s value."),
		},
		"x-go-type": "types.SetCodeAuthorization",
	}
}

func transactionArgsSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Arguments for message calls and transaction submission, using Ethereum JSON-RPC hex encoding. Use either legacy `gasPrice` or EIP-1559 fee fields. If you provide blob sidecar fields, provide `blobs`, `commitments`, and `proofs` together.",
		"properties": map[string]any{
			"from":                 addressSchema("Sender address."),
			"to":                   addressSchema("Recipient address. Omit for contract creation."),
			"gas":                  hexUint64Schema("Gas limit to use. If omitted, the node may estimate it."),
			"gasPrice":             hexBigSchema("Legacy gas price. Do not combine with EIP-1559 fee fields."),
			"maxFeePerGas":         hexBigSchema("EIP-1559 maximum total fee per gas."),
			"maxPriorityFeePerGas": hexBigSchema("EIP-1559 maximum priority fee per gas."),
			"value":                hexBigSchema("Amount of wei to transfer."),
			"nonce":                hexUint64Schema("Explicit sender nonce."),
			"data": map[string]any{
				"type":        "string",
				"pattern":     "^0x[0-9a-fA-F]*$",
				"description": "Legacy calldata field kept for backwards compatibility. Prefer `input`.",
				"deprecated":  true,
				"x-go-type":   "hexutil.Bytes",
			},
			"input":            hexBytesSchema("Preferred calldata field for contract calls and deployments."),
			"accessList":       accessListSchema(),
			"chainId":          hexBigSchema("Chain ID to sign against. If set, it must match the node chain ID."),
			"maxFeePerBlobGas": hexBigSchema("EIP-4844 maximum fee per blob gas."),
			"blobVersionedHashes": map[string]any{
				"type":        "array",
				"description": "EIP-4844 versioned blob hashes.",
				"items":       hashSchema("Hex-encoded versioned blob hash."),
				"x-go-type":   "[]common.Hash",
			},
			"blobs": map[string]any{
				"type":        "array",
				"description": "Optional EIP-4844 blob sidecar payloads.",
				"items":       blobSchema(),
				"x-go-type":   "[]kzg4844.Blob",
			},
			"commitments": map[string]any{
				"type":        "array",
				"description": "Optional EIP-4844 KZG commitments matching `blobs`.",
				"items":       commitmentSchema(),
				"x-go-type":   "[]kzg4844.Commitment",
			},
			"proofs": map[string]any{
				"type":        "array",
				"description": "Optional EIP-4844 KZG proofs matching `blobs`.",
				"items":       proofSchema(),
				"x-go-type":   "[]kzg4844.Proof",
			},
			"authorizationList": map[string]any{
				"type":        "array",
				"description": "Optional EIP-7702 set-code authorizations.",
				"items":       setCodeAuthorizationSchema(),
				"x-go-type":   "[]types.SetCodeAuthorization",
			},
		},
		"x-go-type": "types.TransactionArgs",
	}
}

func filterCriteriaSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Log filter query used by eth_getLogs and filter subscription methods. Use either `blockHash` or a `fromBlock`/`toBlock` range.",
		"properties": map[string]any{
			"blockHash": hashSchema("Restrict results to a single block hash. Mutually exclusive with fromBlock/toBlock."),
			"fromBlock": blockTagSchema("Start of the block range, inclusive."),
			"toBlock":   blockTagSchema("End of the block range, inclusive."),
			"address": map[string]any{
				"description": "Single contract address or array of addresses to match.",
				"oneOf": []any{
					addressSchema("Contract address to match."),
					map[string]any{
						"type":        "array",
						"description": "One or more contract addresses to match.",
						"items":       addressSchema("Contract address to match."),
						"minItems":    1,
					},
				},
			},
			"topics": map[string]any{
				"type":        "array",
				"description": "Up to four topic filters. Each position is AND-matched; nested arrays are OR-matched within a position; null means wildcard.",
				"maxItems":    4,
				"items": map[string]any{
					"oneOf": []any{
						map[string]any{
							"type":        "null",
							"description": "Wildcard for this topic position.",
						},
						hashSchema("Single topic hash to match at this position."),
						map[string]any{
							"type":        "array",
							"description": "OR-match any of these topic hashes at this position.",
							"items":       hashSchema("Topic hash to match."),
							"minItems":    1,
						},
					},
				},
			},
		},
		"x-go-type": "filters.FilterCriteria",
	}
}

func stateOverrideSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Optional ephemeral account state overrides applied only while executing the call. Each top-level key is an account address.",
		"propertyNames": map[string]any{
			"pattern": "^0x[0-9a-fA-F]{40}$",
		},
		"additionalProperties": overrideAccountSchema(),
		"x-go-type":            "json.RawMessage",
	}
}

func overrideAccountSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Account override applied during eth_call or access-list generation. Use either `state` to replace storage entirely or `stateDiff` to patch individual slots.",
		"properties": map[string]any{
			"nonce":                   hexUint64Schema("Override the account nonce for this call."),
			"code":                    hexBytesSchema("Override the account bytecode for this call."),
			"balance":                 hexBigSchema("Override the account balance for this call."),
			"state":                   storageOverrideSchema("Replace the full storage map for this account during the call."),
			"stateDiff":               storageOverrideSchema("Patch only the listed storage slots during the call."),
			"movePrecompileToAddress": addressSchema("Move a precompile to this address for the duration of the call."),
		},
	}
}

func storageOverrideSchema(description string) map[string]any {
	return map[string]any{
		"type":        "object",
		"description": description,
		"propertyNames": map[string]any{
			"pattern": "^0x[0-9a-fA-F]{64}$",
		},
		"additionalProperties": hashSchema("Override value for this storage slot."),
	}
}

func schemaForTypeRecursive(t reflect.Type, depth int) map[string]any {
	nullable := false
	for t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
	}

	if override, ok := ethereumTypeOverrides[t.String()]; ok {
		schema := make(map[string]any, len(override)+2)
		for k, v := range override {
			schema[k] = v
		}
		schema["x-go-type"] = t.String()
		if nullable {
			schema["nullable"] = true
		}
		return schema
	}

	schema := map[string]any{
		"x-go-type": t.String(),
	}

	switch t.Kind() {
	case reflect.Bool:
		schema["type"] = "boolean"
	case reflect.String:
		schema["type"] = "string"
	case reflect.Slice, reflect.Array:
		schema["type"] = "array"
		if depth < maxSchemaDepth {
			schema["items"] = schemaForTypeRecursive(t.Elem(), depth+1)
		} else {
			schema["items"] = map[string]any{}
		}
	case reflect.Map:
		schema["type"] = "object"
	case reflect.Interface:
		schema["type"] = "object"
	case reflect.Struct:
		schema["type"] = "object"
		if depth < maxSchemaDepth {
			props, required := structProperties(t, depth+1)
			if len(props) > 0 {
				schema["properties"] = props
			}
			if len(required) > 0 {
				schema["required"] = required
			}
		}
	default:
		schema["type"] = "string"
	}

	if nullable {
		schema["nullable"] = true
	}

	return schema
}

// structProperties expands a struct's exported fields into JSON Schema
// properties using the `json` struct tag for field names.
func structProperties(t reflect.Type, depth int) (map[string]any, []string) {
	props := make(map[string]any)
	requiredSet := make(map[string]bool)

	directNames := make(map[string]bool)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || f.Anonymous {
			continue
		}
		jsonTag := f.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		name, _ := parseJSONTag(jsonTag)
		if name == "" {
			name = f.Name
		}
		directNames[name] = true
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		if f.Anonymous {
			embeddedType := f.Type
			for embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}
			if embeddedType.Kind() == reflect.Struct {
				innerProps, innerReq := structProperties(embeddedType, depth)
				for k, v := range innerProps {
					if !directNames[k] {
						props[k] = v
					}
				}
				for _, r := range innerReq {
					if !directNames[r] {
						requiredSet[r] = true
					}
				}
			}
			continue
		}

		jsonTag := f.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		name, opts := parseJSONTag(jsonTag)
		if name == "" {
			name = f.Name
		}

		fieldSchema := schemaForTypeRecursive(f.Type, depth)
		if _, hasDesc := fieldSchema["description"]; !hasDesc {
			fieldSchema["description"] = "Go type: " + f.Type.String()
		}

		props[name] = fieldSchema

		isPtr := f.Type.Kind() == reflect.Ptr
		if !isPtr && !opts.omitempty {
			requiredSet[name] = true
		}
	}

	var required []string
	for r := range requiredSet {
		required = append(required, r)
	}

	return props, required
}

type jsonTagOpts struct {
	omitempty bool
}

func parseJSONTag(tag string) (string, jsonTagOpts) {
	parts := strings.Split(tag, ",")
	name := parts[0]
	opts := jsonTagOpts{}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			opts.omitempty = true
		}
	}
	return name, opts
}
