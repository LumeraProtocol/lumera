# Unit Tests: OpenRPC & Generator

Purpose: verifies OpenRPC registration, embedded-spec serving semantics, CORS behavior, and spec generator output constraints expected by OpenRPC clients.

Primary files:
- `app/openrpc/openrpc_test.go`
- `app/openrpc/http_test.go`
- `tools/openrpcgen/main_test.go`

| Test | Description |
| --- | --- |
| `TestDiscoverDocumentValid` | Verifies embedded OpenRPC JSON is valid and parseable. |
| `TestEnsureNamespaceEnabled` | Verifies `rpc` namespace append helper is idempotent and stable. |
| `TestRegisterJSONRPCNamespaceIdempotent` | Verifies repeated JSON-RPC namespace registration is safe. |
| `TestServeHTTPGet` | Verifies `/openrpc.json` GET response shape/content type and CORS headers. |
| `TestServeHTTPHead` | Verifies `/openrpc.json` HEAD behavior and headers. |
| `TestServeHTTPMethodNotAllowed` | Verifies unsupported methods return `405` with correct `Allow` list. |
| `TestServeHTTPOptions` | Verifies CORS preflight (`OPTIONS`) returns `204` and expected CORS headers. |
| `TestServeHTTPCORSAllowedOrigin` | Verifies allowed origin from ws-origins list is echoed back in CORS header. |
| `TestServeHTTPCORSBlockedOrigin` | Verifies unlisted origin gets no `Access-Control-Allow-Origin` header. |
| `TestServeHTTPCORSNoOriginHeader` | Verifies non-browser requests (no Origin) are allowed through. |
| `TestServeHTTPCORSWildcardInList` | Verifies `*` in origins list allows all origins. |
| `TestCollectMethodsPrefersOverrideExamples` | Verifies generator prefers curated overrides from `docs/openrpc/examples_overrides.json`. |
| `TestAlignExampleParamNamesRemapsIndexedArgs` | Verifies generator remaps generic `argN` names to human-readable parameter names. |
| `TestExampleObjectSerializesNullValue` | Verifies generator keeps explicit `result.value: null` instead of dropping the field. |
| `TestCollectMethodsExamplesAlwaysIncludeParamsField` | Verifies generator always emits `params` in examples (empty array when method has no parameters). |
