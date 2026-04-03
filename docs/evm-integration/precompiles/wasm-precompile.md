# CosmWasm Cross-Runtime Bridge — Wasm Precompile & EVM Plugin

Lumera is the only Cosmos EVM chain that also runs CosmWasm. This precompile and its companion plugin form the industry's first bidirectional cross-runtime bridge between CosmWasm and an EVM, with no external precedent.

## Architecture Overview

The bridge has two directions:

| Direction | Mechanism | Address / Config |
|-----------|-----------|------------------|
| **EVM -> CosmWasm** | Static precompile (`IWasm`) | `0x0000000000000000000000000000000000000903` |
| **CosmWasm -> EVM** | Custom message handler + query handler decorator | JSON `Custom` envelope in `CosmosMsg` / `QueryRequest` |

Both directions share a **reentrancy guard** (max depth 1) and execute as the **calling contract** (not tx.origin).

### Source Files

| File | Purpose |
|------|---------|
| `precompiles/wasm/wasm.go` | Precompile struct, Run, Execute, dispatch |
| `precompiles/wasm/tx.go` | `execute` handler (state-changing) |
| `precompiles/wasm/query.go` | `query`, `contractInfo`, `rawQuery` handlers |
| `precompiles/wasm/events.go` | `WasmExecuted` EVM log emission |
| `precompiles/wasm/types.go` | Method name constants, address |
| `precompiles/wasm/abi.json` | Compiled ABI from `IWasm.sol` |
| `precompiles/crossruntime/guard.go` | Reentrancy depth guard (shared both directions) |
| `precompiles/crossruntime/addr.go` | Address conversion helpers (EVM hex <-> bech32) |
| `precompiles/crossruntime/errors.go` | Cross-runtime error constants |
| `app/wasm_evm_plugin.go` | CosmWasm->EVM message handler, query decorator, gas cap |
| `precompiles/solidity/contracts/interfaces/IWasm.sol` | Solidity interface definition |

---

## Direction 1: EVM -> CosmWasm (Wasm Precompile)

### Address

```
0x0000000000000000000000000000000000000903
```

Follows the Lumera convention: `0x0900`+ for custom precompiles (Action at `0x0901`, Supernode at `0x0902`, Wasm at `0x0903`).

### Solidity Interface

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IWasm -- Lumera CosmWasm Precompile Interface
/// @notice Precompile at address 0x0000000000000000000000000000000000000903
interface IWasm {
    // Events
    event WasmExecuted(address indexed caller, string contractAddr, bytes response);

    // State-changing: execute a CosmWasm contract (non-payable, no funds)
    function execute(
        string calldata contractAddr,   // bech32 wasm contract address
        bytes  calldata msg             // JSON-encoded execute message
    ) external returns (bytes memory response);

    // Read-only: query a CosmWasm contract
    function query(
        string calldata contractAddr,   // bech32 wasm contract address
        bytes  calldata msg             // JSON-encoded query message
    ) external view returns (bytes memory response);

    // Read-only: get contract info
    function contractInfo(string calldata contractAddr)
        external view returns (
            uint64 codeId,
            string memory creator,
            string memory admin,
            string memory label
        );

    // Read-only: query raw storage key
    function rawQuery(string calldata contractAddr, bytes calldata key)
        external view returns (bytes memory value);
}
```

### Methods

| Method | Type | Description |
|--------|------|-------------|
| `execute` | Transaction | Execute a CosmWasm contract. Caller is `contract.Caller()` converted to bech32. Non-payable in Phase 1. |
| `query` | View | Smart query a CosmWasm contract. Returns raw JSON bytes. |
| `contractInfo` | View | Returns contract metadata: code ID, creator, admin, and label. |
| `rawQuery` | View | Read a raw storage key from a CosmWasm contract's KV store. |

### Events

| Event | Indexed Fields | Data Fields |
|-------|---------------|-------------|
| `WasmExecuted` | `caller` (address) | `contractAddr` (string), `response` (bytes) |

### Usage Example (Solidity)

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../interfaces/IWasm.sol";

contract WasmCaller {
    IWasm constant WASM = IWasm(0x0000000000000000000000000000000000000903);

    // Query a CosmWasm counter contract
    function getCount(string calldata wasmContract) external view returns (bytes memory) {
        bytes memory queryMsg = '{"get_count":{}}';
        return WASM.query(wasmContract, queryMsg);
    }

    // Execute a CosmWasm counter contract
    function increment(string calldata wasmContract) external returns (bytes memory) {
        bytes memory execMsg = '{"increment":{}}';
        return WASM.execute(wasmContract, execMsg);
    }

    // Check if a contract exists
    function checkContract(string calldata wasmContract) external view returns (uint64 codeId) {
        (codeId,,,) = WASM.contractInfo(wasmContract);
    }
}
```

### Data Flow

```
Solidity contract
  -> CALL 0x0903 [ABI: execute(contractAddr, msg)]
  -> WasmPrecompile.Run()
    -> RunNativeAction(evm, contract, func(ctx) {
         1. Check reentrancy guard (depth must be < 1)
         2. Decode ABI args
         3. Convert contract.Caller() -> bech32 via address codec
         4. Increment cross-runtime depth in context
         5. Call wasmPermKeeper.Execute(ctx, wasmAddr, callerAddr, msg, sdk.Coins{})
         6. Emit WasmExecuted event via stateDB.AddLog()
         7. Return ABI-encoded response
       })
```

---

## Direction 2: CosmWasm -> EVM (Custom Plugin)

CosmWasm contracts interact with EVM contracts through the standard `Custom` JSON envelope in `CosmosMsg` (for state-changing calls) and `QueryRequest` (for read-only queries).

### Custom Message Format

Send via `CosmosMsg::Custom` from a CosmWasm contract:

```json
{
  "evm_call": {
    "contract": "0x1234abcd...",
    "calldata": "0xa9059cbb000000..."
  }
}
```

- `contract`: hex-encoded EVM contract address
- `calldata`: hex-encoded EVM calldata (function selector + ABI-encoded args)
- Phase 1: non-payable (no `value` field)

### Custom Query Formats

Query via `QueryRequest::Custom`:

**EVM contract call (read-only eth_call equivalent):**

```json
{
  "evm_call": {
    "contract": "0x1234abcd...",
    "calldata": "0x70a08231000000..."
  }
}
```

Returns: `{"result":"0x<hex-encoded return data>"}`

**EVM account info:**

```json
{
  "evm_account": {
    "address": "0x1234abcd..."
  }
}
```

Returns: `{"balance":"<wei string>","nonce":<uint64>,"is_contract":<bool>}`

### CosmWasm Contract Example (Rust)

```rust
use cosmwasm_std::{CosmosMsg, CustomMsg, CustomQuery, QueryRequest, WasmMsg};
use serde::{Deserialize, Serialize};

// Custom message for EVM calls
#[derive(Serialize)]
struct EvmCustomMsg {
    evm_call: EvmCallMsg,
}

#[derive(Serialize)]
struct EvmCallMsg {
    contract: String,
    calldata: String,
}

// In your execute handler:
fn call_evm_contract(
    evm_contract: String,
    calldata: String,
) -> CosmosMsg {
    let msg = EvmCustomMsg {
        evm_call: EvmCallMsg {
            contract: evm_contract,
            calldata,
        },
    };
    CosmosMsg::Custom(serde_json::to_value(&msg).unwrap())
}

// Custom query for EVM state
#[derive(Serialize)]
struct EvmCustomQuery {
    evm_call: EvmCallQuery,
}

#[derive(Serialize)]
struct EvmCallQuery {
    contract: String,
    calldata: String,
}

#[derive(Deserialize)]
struct EvmCallResponse {
    result: String,
}
```

### Implementation Details

The plugin uses `evmKeeper.ApplyMessage()` directly (not `CallEVMWithData`) because:
1. `CallEVMWithData` hardcodes `Value: big.NewInt(0)` — cannot be extended for future payable calls
2. `CallEVMWithData` doesn't create its own stateDB — requires non-nil stateDB passed in
3. `CallEVMWithData` uses `config.DefaultGasCap` instead of a configurable per-call cap

The plugin creates a fresh `statedb.New(ctx, evmKeeper, txConfig)` for each call, matching the pattern used in the EVM keeper's own gRPC query handlers.

### Gas Cap

Every CosmWasm -> EVM call is capped at `min(DefaultCrossRuntimeGasCap, remaining_gas)` where `DefaultCrossRuntimeGasCap = 3,000,000`. This prevents a single cross-runtime call from burning the entire block gas limit.

### Query Handler Choice

The plugin uses `WithQueryHandlerDecorator` (not `WithQueryPlugins`) because:
- `CustomQuerier` signature: `func(ctx, json.RawMessage)` — **loses caller identity**
- `WasmVMQueryHandler` signature: `HandleQuery(ctx, caller, request)` — **preserves caller identity**

This matters because `eth_call` requires a `from` address. Using the wasm contract's address as `msg.From` means the EVM contract observes the wasm contract as `msg.sender` in view functions.

---

## Cross-Cutting Concerns

### Sender Identity

**Cross-runtime calls always execute as the calling contract, not the outer user (tx.origin).**

| Direction | Sender | Source |
|-----------|--------|--------|
| EVM -> Wasm | `contract.Caller()` converted to bech32 | Same pattern as Action precompile (`tx_sense.go:43`) |
| Wasm -> EVM | `contractAddr` (wasm contract address) converted to `common.Address` | Passed by wasm dispatcher (`msg_dispatcher.go:31`) |

Proxy/delegatecall patterns do NOT propagate across the runtime boundary.

### Reentrancy Guard

Both directions share a typed context-key depth counter in `precompiles/crossruntime/guard.go`. Phase 1 enforces max depth = 1 (no A->B->A calls).

**Why depth > 1 is not a simple configuration change**: Enabling EVM->Wasm->EVM requires the inner EVM leg to reuse the outer stateDB, not create a fresh one. The cosmos-evm keeper explicitly warns about this (`call_evm.go:19-22`). Future phases would need to thread the stateDB through the SDK context.

### Gas Metering

Both runtimes consume Cosmos SDK gas as the common currency:

- **EVM -> Wasm**: `RunNativeAction` creates a sub-gas-meter from `contract.Gas`. Wasm keeper consumes SDK gas. Cost deducted from EVM contract gas after return.
- **Wasm -> EVM**: Plugin calls `ApplyMessage`, then charges `res.GasUsed` to `ctx.GasMeter()`. Wasm gas register converts back to CosmWasm gas units.

### Atomicity

- **EVM -> Wasm**: `RunNativeAction` snapshots multistore + stateDB journal. On failure, both revert atomically.
- **Wasm -> EVM**: Wasm dispatcher uses cache context per sub-message. The stateDB created inside the handler operates on that cache context. On failure, the dispatcher discards the cache.

---

## Phase Roadmap

| Phase | Scope | Status |
|-------|-------|--------|
| **1** | Non-payable execute/query both directions, depth-1 reentrancy guard, per-call gas cap | **Done** |
| 2 | Payable execute with funds, denomination conversion (ulume <-> alume via PreciseBankKeeper) | Planned |
| 3 | `instantiate` on WasmPrecompile, `evm_create` on wasm handler | Planned |
| 4 | Reentrancy depth > 1 (stateDB threading), configurable gas cap via module params, security audit | Planned |

---

## Registration

### Precompile (EVM -> Wasm)

Registered in `app/evm.go` `configureEVMStaticPrecompiles()` after all keepers are initialized (step 5 in app init). The WasmKeeper is available because it was created in step 3 (`registerIBCModules`).

Address added to `LumeraActiveStaticPrecompiles` in `app/evm/precompiles.go`.

### Plugin (Wasm -> EVM)

Wired via `EVMWasmPluginOpts()` in `app/app.go`, appended to `wasmOpts` between `registerEVMModules` (step 1) and `registerIBCModules` (step 3). The EVMKeeper pointer is valid at this point.

Uses `WithMessageHandlerDecorator` (wraps default handler chain) and `WithQueryHandlerDecorator` (wraps default query handler). Non-matching messages/queries fall through to standard handlers.
