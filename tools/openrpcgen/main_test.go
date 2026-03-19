package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	evmeth "github.com/cosmos/evm/rpc/namespaces/ethereum/eth"
	evmfilters "github.com/cosmos/evm/rpc/namespaces/ethereum/eth/filters"
)

type testAPI struct{}

func (*testAPI) Echo(input string) string { return input }
func (*testAPI) Ping() string             { return "pong" }

func TestCollectMethodsPrefersOverrideExamples(t *testing.T) {
	t.Parallel()

	override := map[string][]examplePairing{
		"test_echo": {
			{
				Name: "custom-example",
				Params: []exampleObject{
					{Name: "arg1", Value: "hello"},
				},
				Result: &exampleObject{Name: "result", Value: "hello"},
			},
		},
	}

	methods := collectMethods([]serviceSpec{
		{Namespace: "test", Type: reflect.TypeOf((*testAPI)(nil))},
	}, override)

	var echo methodObject
	found := false
	for _, m := range methods {
		if m.Name == "test_echo" {
			echo = m
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected method name test_echo to exist")
	}
	if len(echo.Examples) != 1 {
		t.Fatalf("expected 1 example, got %d", len(echo.Examples))
	}
	if got := echo.Examples[0].Name; got != "custom-example" {
		t.Fatalf("expected override example name custom-example, got %s", got)
	}
}

func TestAlignExampleParamNamesRemapsIndexedArgs(t *testing.T) {
	t.Parallel()

	examples := []examplePairing{
		{
			Name: "indexed",
			Params: []exampleObject{
				{Name: "arg1", Value: "0x1"},
				{Name: "arg2", Value: "latest"},
			},
		},
	}
	params := []contentDescriptor{
		{Name: "address"},
		{Name: "blockNrOrHash"},
	}

	got := alignExampleParamNames(examples, params)
	if len(got) != 1 || len(got[0].Params) != 2 {
		t.Fatalf("unexpected remap output shape: %#v", got)
	}
	if got[0].Params[0].Name != "address" || got[0].Params[1].Name != "blockNrOrHash" {
		t.Fatalf("unexpected remapped names: %#v", got[0].Params)
	}
}

func TestExampleObjectSerializesNullValue(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(exampleObject{
		Name:  "result",
		Value: nil,
	})
	if err != nil {
		t.Fatalf("marshal exampleObject: %v", err)
	}

	got := string(raw)
	if !strings.Contains(got, `"value":null`) {
		t.Fatalf("expected serialized null value, got: %s", got)
	}
}

func TestCollectMethodsExamplesAlwaysIncludeParamsField(t *testing.T) {
	t.Parallel()

	methods := collectMethods([]serviceSpec{
		{Namespace: "test", Type: reflect.TypeOf((*testAPI)(nil))},
	}, nil)

	var ping methodObject
	found := false
	for _, m := range methods {
		if m.Name == "test_ping" {
			ping = m
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected test_ping method in generated list")
	}
	if len(ping.Examples) != 1 {
		t.Fatalf("expected one example, got %d", len(ping.Examples))
	}
	if ping.Examples[0].Params == nil {
		t.Fatalf("expected params to be non-nil")
	}
	if len(ping.Examples[0].Params) != 0 {
		t.Fatalf("expected empty params array, got %d items", len(ping.Examples[0].Params))
	}

	raw, err := json.Marshal(ping.Examples[0])
	if err != nil {
		t.Fatalf("marshal example: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, `"params":[]`) {
		t.Fatalf("expected params array in JSON, got: %s", got)
	}
}

func TestCollectMethodsUsesCuratedTransactionArgsSchema(t *testing.T) {
	t.Parallel()

	methods := collectMethods([]serviceSpec{
		{Namespace: "eth", Type: reflect.TypeOf((*evmeth.PublicAPI)(nil))},
	}, nil)

	var call methodObject
	found := false
	for _, m := range methods {
		if m.Name == "eth_call" {
			call = m
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected eth_call method in generated list")
	}
	if len(call.Params) == 0 {
		t.Fatalf("expected eth_call to have params")
	}

	schema := call.Params[0].Schema
	if got := schema["x-go-type"]; got != "types.TransactionArgs" {
		t.Fatalf("expected TransactionArgs schema, got %#v", got)
	}
	if _, ok := schema["required"]; ok {
		t.Fatalf("expected curated TransactionArgs schema to omit blanket required fields, got %#v", schema["required"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected object properties, got %#v", schema["properties"])
	}

	data, ok := props["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data property schema, got %#v", props["data"])
	}
	if deprecated, _ := data["deprecated"].(bool); !deprecated {
		t.Fatalf("expected data field to be marked deprecated, got %#v", data["deprecated"])
	}

	input, ok := props["input"].(map[string]any)
	if !ok {
		t.Fatalf("expected input property schema, got %#v", props["input"])
	}
	description, _ := input["description"].(string)
	if !strings.Contains(description, "Preferred") {
		t.Fatalf("expected input description to mention preferred field, got %q", description)
	}
	summary, _ := schema["description"].(string)
	if !strings.Contains(summary, "EIP-1559") {
		t.Fatalf("expected TransactionArgs description to mention fee rules, got %q", summary)
	}
}

func TestCollectMethodsUsesCuratedFilterCriteriaSchema(t *testing.T) {
	t.Parallel()

	methods := collectMethods([]serviceSpec{
		{Namespace: "eth", Type: reflect.TypeOf((*evmfilters.PublicFilterAPI)(nil))},
	}, nil)

	var getLogs methodObject
	found := false
	for _, m := range methods {
		if m.Name == "eth_getLogs" {
			getLogs = m
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected eth_getLogs method in generated list")
	}
	if len(getLogs.Params) == 0 {
		t.Fatalf("expected eth_getLogs to have params")
	}

	schema := getLogs.Params[0].Schema
	if got := schema["x-go-type"]; got != "filters.FilterCriteria" {
		t.Fatalf("expected FilterCriteria schema, got %#v", got)
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected object properties, got %#v", schema["properties"])
	}
	if _, ok := props["address"].(map[string]any); !ok {
		t.Fatalf("expected address property schema, got %#v", props["address"])
	}
	if _, ok := props["topics"].(map[string]any); !ok {
		t.Fatalf("expected topics property schema, got %#v", props["topics"])
	}

	summary, _ := schema["description"].(string)
	if !strings.Contains(summary, "blockHash") {
		t.Fatalf("expected FilterCriteria description to mention blockHash exclusivity, got %q", summary)
	}
}

func TestCollectMethodsUsesCuratedStateOverrideSchema(t *testing.T) {
	t.Parallel()

	methods := collectMethods([]serviceSpec{
		{Namespace: "eth", Type: reflect.TypeOf((*evmeth.PublicAPI)(nil))},
	}, nil)

	var call methodObject
	found := false
	for _, m := range methods {
		if m.Name == "eth_call" {
			call = m
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected eth_call method in generated list")
	}
	if len(call.Params) < 3 {
		t.Fatalf("expected eth_call to have overrides param, got %d params", len(call.Params))
	}

	overrides := call.Params[2]
	if overrides.Name != "overrides" {
		t.Fatalf("expected third param to be overrides, got %q", overrides.Name)
	}
	if !strings.Contains(overrides.Description, "state overrides") {
		t.Fatalf("expected overrides description to mention state overrides, got %q", overrides.Description)
	}

	schema := overrides.Schema
	if got := schema["x-go-type"]; got != "json.RawMessage" {
		t.Fatalf("expected json.RawMessage schema, got %#v", got)
	}
	if _, ok := schema["additionalProperties"].(map[string]any); !ok {
		t.Fatalf("expected account override schema in additionalProperties, got %#v", schema["additionalProperties"])
	}
}
