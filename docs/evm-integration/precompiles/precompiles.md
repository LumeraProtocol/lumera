# Lumera EVM Precompiles

Lumera ships with **11 static precompiles**: 8 standard Cosmos EVM precompiles and 3 custom Lumera-specific precompiles exposing native modules to Solidity contracts.

## Standard Precompiles

Enabled in [`app/evm/precompiles.go`](../../../app/evm/precompiles.go). Full reference with Solidity interfaces and examples: [standard-precompiles.md](standard-precompiles.md)

| Precompile | Address | Purpose |
|------------|---------|---------|
| P256 | `0x...100` | NIST P-256 (secp256r1) signature verification (EIP-7212) |
| Bech32 | `0x...400` | Hex ↔ Bech32 address conversion |
| Staking | `0x...800` | Delegate, undelegate, redelegate, create/edit validators |
| Distribution | `0x...801` | Claim rewards, set withdraw address, fund community pool |
| ICS20 | `0x...802` | IBC fungible token transfers |
| Bank | `0x...804` | Query balances and token supply (read-only) |
| Gov | `0x...805` | Submit/vote on proposals, query governance state |
| Slashing | `0x...806` | Unjail validators, query signing info |

Vesting precompile (`0x...803`) is explicitly excluded (not installed by upstream default registry in current version).

## Custom Lumera Precompiles

| Precompile | Address | Module | Docs |
|------------|---------|--------|------|
| Action | `0x0000000000000000000000000000000000000901` | `x/action` | [action-precompile.md](action-precompile.md) |
| Supernode | `0x0000000000000000000000000000000000000902` | `x/supernode/v1` | [supernode-precompile.md](supernode-precompile.md) |
| Wasm | `0x0000000000000000000000000000000000000903` | CosmWasm (wasmd) | [wasm-precompile.md](wasm-precompile.md) |

### Action Precompile (`0x0901`)

Exposes distributed action processing (Cascade, Sense) to EVM contracts. Uses a hybrid typed/generic approach — typed methods for `requestCascade`, `finalizeCascade`, `requestSense`, `finalizeSense`, and generic methods for `approveAction`, `getAction`, `getActionFee`, `getParams`, and paginated list queries.

Source: `precompiles/action/`

See [action-precompile.md](action-precompile.md) for full ABI reference, Solidity interface, usage examples, and design notes.

### Supernode Precompile (`0x0902`)

Exposes supernode registration, lifecycle, and metrics to EVM contracts. Uses a generic-only approach — transaction methods (`registerSupernode`, `deregisterSupernode`, `startSupernode`, `stopSupernode`, `updateSupernode`, `reportMetrics`) and query methods (`getSuperNode`, `getSuperNodeByAccount`, `listSuperNodes`, `getTopSuperNodesForBlock`, `getMetrics`, `getParams`).

Source: `precompiles/supernode/`

See [supernode-precompile.md](supernode-precompile.md) for full ABI reference, Solidity interface, usage examples, and design notes.

### Wasm Precompile (`0x0903`)

Enables bidirectional CosmWasm↔EVM contract interaction — the industry's first cross-runtime bridge between CosmWasm and an EVM. Solidity contracts can execute and query CosmWasm contracts through this precompile. The reverse direction (CosmWasm→EVM) is handled by custom message/query handlers wired into the wasm keeper.

Phase 1 (current): non-payable execute, query, contractInfo, rawQuery. Reentrancy guard at depth 1 prevents circular cross-runtime calls.

Source: `precompiles/wasm/`, `precompiles/crossruntime/`, `app/wasm_evm_plugin.go`

See [wasm-precompile.md](wasm-precompile.md) for full ABI reference, Solidity interface, architecture, and design notes.

## Blocked-Address Protections

All precompile addresses are protected from accidental token sends via:
- Module account block list
- Precompile-address send restriction in bank send restrictions
