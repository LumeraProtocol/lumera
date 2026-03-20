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
	"runtime/debug"
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
	evmModulePath       = "github.com/cosmos/evm"
	openRPCDiscoverName = "rpc.discover"
	openRPCMetaSchema   = "https://raw.githubusercontent.com/open-rpc/meta-schema/master/schema.json"
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
	Name        string `json:"name"`
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
	// Keep `params` always present; OpenRPC tooling expects this field.
	Params []exampleObject `json:"params"`
	Result *exampleObject  `json:"result,omitempty"`
}

type exampleObject struct {
	Name        string `json:"name"`
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
	// Keep `value` always present, including explicit null examples.
	// OpenRPC tooling expects the field to exist on result objects.
	Value any `json:"value"`
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
			Version:     cosmosEVMVersion(),
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
			if svc.Namespace == lumeraopenrpc.Namespace && m.Name == "Discover" {
				methodName = openRPCDiscoverName
			}
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

		required := t.Kind() != reflect.Ptr
		schema := schemaForType(t)
		if override := paramDescriptorOverride(methodName, paramName, t); override != nil {
			if override.Description != "" {
				paramDescription = override.Description
			}
			if override.Schema != nil {
				schema = override.Schema
			}
			if override.Required != nil {
				required = *override.Required
			}
		}

		params = append(params, contentDescriptor{
			Name:        paramName,
			Description: paramDescription,
			Required:    required,
			Schema:      schema,
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

	if methodName == openRPCDiscoverName {
		result = contentDescriptor{
			Name:        "OpenRPC Schema",
			Description: "OpenRPC schema returned by the service discovery method.",
			Schema: map[string]any{
				"$ref": openRPCMetaSchema,
			},
		}
	}

	return params, result
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

// cosmosEVMVersion reads the cosmos/evm module version from the binary's
// embedded build info (populated by the Go toolchain from go.mod). This
// avoids hardcoding a version string that drifts on dependency upgrades.
func cosmosEVMVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, dep := range bi.Deps {
			if dep.Path == evmModulePath {
				return "cosmos/evm " + dep.Version
			}
		}
	}
	return "cosmos/evm (unknown)"
}
