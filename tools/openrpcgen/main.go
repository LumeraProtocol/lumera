package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	lumeraopenrpc "github.com/LumeraProtocol/lumera/app/openrpc"
	evmdebug "github.com/cosmos/evm/rpc/namespaces/ethereum/debug"
	evmeth "github.com/cosmos/evm/rpc/namespaces/ethereum/eth"
	evmfilters "github.com/cosmos/evm/rpc/namespaces/ethereum/eth/filters"
	evmminer "github.com/cosmos/evm/rpc/namespaces/ethereum/miner"
	evmnet "github.com/cosmos/evm/rpc/namespaces/ethereum/net"
	evmpersonal "github.com/cosmos/evm/rpc/namespaces/ethereum/personal"
	evmtxpool "github.com/cosmos/evm/rpc/namespaces/ethereum/txpool"
	evmweb3 "github.com/cosmos/evm/rpc/namespaces/ethereum/web3"
)

const (
	defaultOutputPath   = "docs/openrpc.json"
	defaultServerURL    = "http://localhost:8545"
	defaultExamplesPath = "docs/openrpc_examples_overrides.json"
	docVersion          = "cosmos/evm v0.5.1"
)

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
	// Some upstream methods intentionally use "_" for unused parameters.
	// This map restores human-readable OpenRPC parameter names for those cases.
	paramNameOverrides = map[string][]string{
		"debug_intermediateRoots":           {"txHash", "config"},
		"eth_getUncleByBlockHashAndIndex":   {"blockHash", "index"},
		"eth_getUncleByBlockNumberAndIndex": {"blockNumber", "index"},
		"eth_getUncleCountByBlockHash":      {"blockHash"},
		"eth_getUncleCountByBlockNumber":    {"blockNumber"},
		"miner_setExtra":                    {"extra"},
		"miner_setGasLimit":                 {"gasLimit"},
		"miner_start":                       {"threads"},
		"personal_sendTransaction":          {"args", "password"},
		"personal_sign":                     {"data", "address", "password"},
		"personal_unlockAccount":            {"address", "password", "duration"},
	}
)

type serviceSpec struct {
	Namespace string
	Type      reflect.Type
}

type openRPCDoc struct {
	OpenRPC  string         `json:"openrpc"`
	Info     infoObject     `json:"info"`
	Servers  []serverObject `json:"servers,omitempty"`
	Methods  []methodObject `json:"methods"`
	External *externalDocs  `json:"externalDocs,omitempty"`
}

type infoObject struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type serverObject struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url"`
}

type tagObject struct {
	Name string `json:"name"`
}

type methodObject struct {
	Name           string              `json:"name"`
	Summary        string              `json:"summary,omitempty"`
	Description    string              `json:"description,omitempty"`
	Tags           []tagObject         `json:"tags,omitempty"`
	ParamStructure string              `json:"paramStructure,omitempty"`
	Params         []contentDescriptor `json:"params"`
	Result         contentDescriptor   `json:"result"`
	Examples       []examplePairing    `json:"examples,omitempty"`
}

type contentDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Schema      map[string]any `json:"schema"`
}

type examplePairing struct {
	Name        string          `json:"name"`
	Summary     string          `json:"summary,omitempty"`
	Description string          `json:"description,omitempty"`
	// Keep `params` always present; OpenRPC tooling expects this field.
	Params      []exampleObject `json:"params"`
	Result      *exampleObject  `json:"result,omitempty"`
}

type exampleObject struct {
	Name        string `json:"name"`
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
	// Keep `value` always present, including explicit null examples.
	// OpenRPC tooling expects the field to exist on result objects.
	Value       any    `json:"value"`
}

type externalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

type methodSourceMetadata struct {
	Description   string
	ParamNames    []string
	ParamComments []string
}

type sourceInspector struct {
	fset  *token.FileSet
	files map[string]*ast.File
}

func main() {
	outPath := flag.String("out", defaultOutputPath, "output OpenRPC file path")
	serverURL := flag.String("server", defaultServerURL, "default JSON-RPC server URL")
	examplesPath := flag.String("examples", defaultExamplesPath, "JSON file with curated method examples overrides")
	flag.Parse()

	exampleOverrides, err := loadExampleOverrides(*examplesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load examples overrides from %s: %v\n", *examplesPath, err)
		os.Exit(1)
	}

	methods := collectMethods([]serviceSpec{
		{Namespace: "eth", Type: reflect.TypeOf((*evmeth.PublicAPI)(nil))},
		{Namespace: "eth", Type: reflect.TypeOf((*evmfilters.PublicFilterAPI)(nil))},
		{Namespace: "web3", Type: reflect.TypeOf((*evmweb3.PublicAPI)(nil))},
		{Namespace: "net", Type: reflect.TypeOf((*evmnet.PublicAPI)(nil))},
		{Namespace: "personal", Type: reflect.TypeOf((*evmpersonal.PrivateAccountAPI)(nil))},
		{Namespace: "txpool", Type: reflect.TypeOf((*evmtxpool.PublicAPI)(nil))},
		{Namespace: "debug", Type: reflect.TypeOf((*evmdebug.API)(nil))},
		{Namespace: "miner", Type: reflect.TypeOf((*evmminer.API)(nil))},
		{Namespace: lumeraopenrpc.Namespace, Type: reflect.TypeOf((*lumeraopenrpc.API)(nil))},
	}, exampleOverrides)

	doc := openRPCDoc{
		OpenRPC: "1.2.6",
		Info: infoObject{
			Title:       "Lumera Cosmos EVM JSON-RPC API",
			Version:     docVersion,
			Description: "Auto-generated method catalog from Cosmos EVM JSON-RPC namespace implementations.",
		},
		Servers: []serverObject{
			{Name: "Default JSON-RPC endpoint", URL: *serverURL},
		},
		Methods: methods,
		External: &externalDocs{
			Description: "Cosmos EVM Ethereum JSON-RPC reference",
			URL:         "https://cosmos-docs.mintlify.app/docs/api-reference/ethereum-json-rpc",
		},
	}

	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal openrpc: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outPath, payload, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", *outPath, err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s with %d methods\n", *outPath, len(methods))
}

func collectMethods(services []serviceSpec, exampleOverrides map[string][]examplePairing) []methodObject {
	methodMap := make(map[string]methodObject)
	inspector := &sourceInspector{
		fset:  token.NewFileSet(),
		files: map[string]*ast.File{},
	}

	for _, svc := range services {
		for i := 0; i < svc.Type.NumMethod(); i++ {
			m := svc.Type.Method(i)
			if m.PkgPath != "" {
				continue
			}
			if !isSuitableCallback(m.Type) {
				continue
			}

			methodName := svc.Namespace + "_" + formatRPCName(m.Name)
			if _, exists := methodMap[methodName]; exists {
				continue
			}

			sourceMeta := inspector.methodMetadata(svc.Type, m)
			params, result := buildMethodDescriptors(methodName, m.Type, sourceMeta)
			examples := methodExamples(methodName, params, result)
			if overrideExamples, ok := exampleOverrides[methodName]; ok && len(overrideExamples) > 0 {
				examples = overrideExamples
			}
			examples = alignExampleParamNames(examples, params)

			methodMap[methodName] = methodObject{
				Name:           methodName,
				Summary:        methodName + " JSON-RPC method",
				Description:    sourceMeta.Description,
				Tags:           []tagObject{{Name: svc.Namespace}},
				ParamStructure: "by-position",
				Params:         params,
				Result:         result,
				Examples:       examples,
			}
		}
	}

	names := make([]string, 0, len(methodMap))
	for name := range methodMap {
		names = append(names, name)
	}
	sort.Strings(names)

	methods := make([]methodObject, 0, len(names))
	for _, name := range names {
		methods = append(methods, methodMap[name])
	}

	return methods
}

func isSuitableCallback(fntype reflect.Type) bool {
	numOut := fntype.NumOut()
	if numOut > 2 {
		return false
	}

	switch {
	case numOut == 1 && isErrorType(fntype.Out(0)):
		// acceptable: func(...) error
	case numOut == 2:
		// acceptable: func(...) (T, error)
		if isErrorType(fntype.Out(0)) || !isErrorType(fntype.Out(1)) {
			return false
		}
	}

	return true
}

func buildMethodDescriptors(methodName string, fntype reflect.Type, sourceMeta methodSourceMetadata) ([]contentDescriptor, contentDescriptor) {
	argStart := 1 // receiver
	if fntype.NumIn() > argStart && fntype.In(argStart) == contextType {
		argStart++
	}

	params := make([]contentDescriptor, 0, fntype.NumIn()-argStart)
	usedNames := map[string]int{}
	for i := argStart; i < fntype.NumIn(); i++ {
		t := fntype.In(i)
		metaIndex := i - 1 // receiver occupies index 0 in function signature
		fallbackName := fmt.Sprintf("arg%d", i-argStart+1)
		paramName := fallbackName
		if metaIndex >= 0 && metaIndex < len(sourceMeta.ParamNames) {
			paramName = normalizeParamName(sourceMeta.ParamNames[metaIndex], fallbackName)
		}
		if isIndexedArgName(paramName) {
			if overrideNames, ok := paramNameOverrides[methodName]; ok {
				overrideIndex := i - argStart
				if overrideIndex >= 0 && overrideIndex < len(overrideNames) {
					paramName = normalizeParamName(overrideNames[overrideIndex], paramName)
				}
			}
		}
		paramName = ensureUniqueParamName(paramName, usedNames)

		paramDescription := fmt.Sprintf("Parameter `%s`. Go type: %s", paramName, t.String())
		if metaIndex >= 0 && metaIndex < len(sourceMeta.ParamComments) {
			paramComment := normalizeCommentText(sourceMeta.ParamComments[metaIndex])
			if paramComment != "" {
				paramDescription = paramComment + " Go type: " + t.String()
			}
		}

		params = append(params, contentDescriptor{
			Name:        paramName,
			Description: paramDescription,
			Required:    t.Kind() != reflect.Ptr,
			Schema:      schemaForType(t),
		})
	}

	result := contentDescriptor{
		Name:        "result",
		Description: "No return value",
		Schema:      map[string]any{"type": "null"},
	}

	var valueOut reflect.Type
	switch fntype.NumOut() {
	case 1:
		if !isErrorType(fntype.Out(0)) {
			valueOut = fntype.Out(0)
		}
	case 2:
		valueOut = fntype.Out(0)
	}

	if valueOut != nil {
		result = contentDescriptor{
			Name:        "result",
			Description: "Go type: " + valueOut.String(),
			Schema:      schemaForType(valueOut),
		}
	}

	return params, result
}

func schemaForType(t reflect.Type) map[string]any {
	nullable := false
	for t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
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
		schema["items"] = map[string]any{}
	case reflect.Map, reflect.Struct, reflect.Interface:
		schema["type"] = "object"
	default:
		// Keep scalar defaults as string; many Ethereum RPC numerics are hex-encoded strings.
		schema["type"] = "string"
	}

	if nullable {
		schema["nullable"] = true
	}

	return schema
}

func isErrorType(t reflect.Type) bool {
	return t.Implements(errorType)
}

func formatRPCName(name string) string {
	runes := []rune(name)
	if len(runes) > 0 {
		runes[0] = unicode.ToLower(runes[0])
	}
	return string(runes)
}

func normalizeParamName(raw, fallback string) string {
	name := strings.TrimSpace(raw)
	if name == "" || name == "_" {
		return fallback
	}
	return name
}

func ensureUniqueParamName(name string, used map[string]int) string {
	count := used[name]
	used[name] = count + 1
	if count == 0 {
		return name
	}
	return name + strconv.Itoa(count+1)
}

func normalizeCommentText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

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

func isIndexedArgName(name string) bool {
	if !strings.HasPrefix(name, "arg") {
		return false
	}
	_, err := strconv.Atoi(strings.TrimPrefix(name, "arg"))
	return err == nil
}

func (s *sourceInspector) methodMetadata(receiverType reflect.Type, method reflect.Method) methodSourceMetadata {
	fn := runtime.FuncForPC(method.Func.Pointer())
	if fn == nil {
		return methodSourceMetadata{}
	}

	file, _ := fn.FileLine(method.Func.Pointer())
	if file == "" {
		return methodSourceMetadata{}
	}
	file = filepath.Clean(file)

	astFile, ok := s.files[file]
	if !ok {
		parsedFile, err := parser.ParseFile(s.fset, file, nil, parser.ParseComments)
		if err != nil {
			return methodSourceMetadata{}
		}
		s.files[file] = parsedFile
		astFile = parsedFile
	}

	receiverName := receiverBaseName(receiverType)
	if receiverName == "" {
		return methodSourceMetadata{}
	}

	for _, decl := range astFile.Decls {
		fnDecl, ok := decl.(*ast.FuncDecl)
		if !ok || fnDecl.Recv == nil || fnDecl.Name == nil || fnDecl.Name.Name != method.Name {
			continue
		}
		if !receiverMatches(fnDecl.Recv, receiverName) {
			continue
		}

		meta := methodSourceMetadata{
			Description: normalizeCommentText(commentGroupText(fnDecl.Doc)),
		}
		if fnDecl.Type == nil || fnDecl.Type.Params == nil {
			return meta
		}

		for _, field := range fnDecl.Type.Params.List {
			comment := normalizeCommentText(commentGroupText(field.Comment))
			if comment == "" {
				comment = normalizeCommentText(commentGroupText(field.Doc))
			}

			if len(field.Names) == 0 {
				meta.ParamNames = append(meta.ParamNames, "")
				meta.ParamComments = append(meta.ParamComments, comment)
				continue
			}
			for _, name := range field.Names {
				meta.ParamNames = append(meta.ParamNames, name.Name)
				meta.ParamComments = append(meta.ParamComments, comment)
			}
		}

		return meta
	}

	return methodSourceMetadata{}
}

func commentGroupText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return group.Text()
}

func receiverBaseName(t reflect.Type) string {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func receiverMatches(recv *ast.FieldList, expectedName string) bool {
	if recv == nil || len(recv.List) == 0 {
		return false
	}
	recvType := recv.List[0].Type
	switch t := recvType.(type) {
	case *ast.Ident:
		return t.Name == expectedName
	case *ast.StarExpr:
		ident, ok := t.X.(*ast.Ident)
		return ok && ident.Name == expectedName
	default:
		return false
	}
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
	case strings.Contains(goType, "types.FilterCriteria"):
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
			"data":  "0x",
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

	// Method-specific defaults for common JSON-RPC response patterns.
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
		// Most Ethereum JSON-RPC scalar values are hex or decimal strings.
		return "0x1"
	}
}
