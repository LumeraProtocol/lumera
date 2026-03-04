package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
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
