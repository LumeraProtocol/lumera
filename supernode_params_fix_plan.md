## Goal
Fix `TestSupernodeUpdateParamsProposal` failing due to `lumerad q supernode params` marshal error (`unknown type float64`) by removing float64 fields from supernode params (or encoding them safely) and aligning defaults/validation/tests.

## Plan
1) Update schema: change supernode params proto to replace float64 fields with marshal-friendly types (e.g., uint64/strings or `sdk.Dec`), keeping semantics (CPU cores, usage %, memory/storage GB, open ports). Regenerate or adjust generated code accordingly.
2) Sync Go structs: update `x/supernode/v1/types` params structs/defaults/validation to the new types and ensure ParamSetPairs/WithDefaults match.
3) Update callers/tests: adjust system test genesis/proposal payloads and any module usage to the new types so CLI queries return JSON cleanly.
4) Validate: rebuild and run `go test -tags=system_test -v ./tests/systemtests` (or the targeted supernode test) to confirm the params query succeeds and tests pass.
