# Supernode Module EVM Precompile

The Lumera supernode precompile exposes the `x/supernode/v1` module to the EVM at a single static address, enabling Solidity contracts to register, manage, query, and monitor supernodes without leaving the EVM execution context.

## Design Overview

### Address

```
0x0000000000000000000000000000000000000902
```

Lumera custom precompiles start at `0x0900`, following the convention:
- `0x01`–`0x0a` — Ethereum standard precompiles
- `0x0100`–`0x0806` — Cosmos EVM standard precompiles (bank, staking, distribution, gov, ICS20, bech32, p256, slashing)
- `0x0900`+ — Lumera-specific custom precompiles

### Generic-Only Design

Unlike the action precompile (which uses typed/generic split for metadata polymorphism), all supernode operations are structurally uniform — the same field patterns across all lifecycle methods. A single generic interface covers the full surface without any typed variants.

### Supernode Lifecycle

```
Register → Active → Stop → Stopped → Start → Active
                  → Deregister → Disabled
                  → Metrics non-compliance → Postponed → Recovery → Active
```

| State | Value | Description |
|-------|-------|-------------|
| Active | 1 | Operational, participating in block consensus and action processing |
| Disabled | 2 | Deregistered by owner |
| Stopped | 3 | Temporarily stopped by owner (can be restarted) |
| Penalized | 4 | Slashed due to misbehavior evidence |
| Postponed | 5 | Suspended due to metrics non-compliance |

### Validator Address Handling

Validator addresses (`lumeravaloper...`) have no meaningful 20-byte EVM representation. Rather than force an incorrect `address` type mapping, the ABI uses `string` for all validator and account addresses. This lets Solidity contracts pass them through cleanly without lossy conversion.

### Float-to-Integer Bridging

Protobuf `SupernodeMetrics` uses `float64` for hardware fields (CPU cores, GB, percentages). Since Solidity has no floating-point type, the precompile uses `uint32`/`uint64` in the ABI and converts via `math.Round()` (not truncation) to handle floating-point imprecision (e.g., 7.999999 → 8).

---

## Solidity Interface

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title ISupernode — Lumera Supernode Module Precompile
/// @notice Call at 0x0000000000000000000000000000000000000902
interface ISupernode {

    // ─── Structs ───────────────────────────────────────────

    struct SuperNodeInfo {
        string  validatorAddress;
        string  supernodeAccount;
        uint8   currentState;    // 1=Active … 5=Postponed
        int64   stateHeight;     // block height of last state change
        string  ipAddress;
        string  p2pPort;
        string  note;
        uint64  evidenceCount;
    }

    struct MetricsReport {
        uint32  versionMajor;
        uint32  versionMinor;
        uint32  versionPatch;
        uint32  cpuCoresTotal;
        uint64  cpuUsagePercent;
        uint64  memTotalGb;
        uint64  memUsagePercent;
        uint64  memFreeGb;
        uint64  diskTotalGb;
        uint64  diskUsagePercent;
        uint64  diskFreeGb;
        uint64  uptimeSeconds;
        uint32  peersCount;
    }

    // ─── Events ────────────────────────────────────────────

    event SupernodeRegistered(
        string indexed validatorAddress,
        address indexed creator,
        uint8 newState
    );

    event SupernodeDeregistered(
        string indexed validatorAddress,
        address indexed creator,
        uint8 oldState
    );

    event SupernodeStateChanged(
        string indexed validatorAddress,
        address indexed creator,
        uint8 newState
    );

    // ─── Transactions ──────────────────────────────────────

    /// @notice Register a new supernode (or re-register from Disabled state).
    /// @param validatorAddress The validator's lumeravaloper... address
    /// @param ipAddress        Public IP or hostname
    /// @param supernodeAccount The supernode's lumera1... account address
    /// @param p2pPort          P2P listening port
    /// @return success         True if registration succeeded
    function registerSupernode(
        string calldata validatorAddress,
        string calldata ipAddress,
        string calldata supernodeAccount,
        string calldata p2pPort
    ) external returns (bool success);

    /// @notice Deregister a supernode (moves to Disabled state).
    function deregisterSupernode(string calldata validatorAddress)
        external returns (bool success);

    /// @notice Start a stopped supernode (Stopped → Active).
    function startSupernode(string calldata validatorAddress)
        external returns (bool success);

    /// @notice Stop an active supernode (Active → Stopped).
    /// @param validatorAddress The validator's address
    /// @param reason           Human-readable reason for stopping
    function stopSupernode(string calldata validatorAddress, string calldata reason)
        external returns (bool success);

    /// @notice Update supernode configuration fields.
    function updateSupernode(
        string calldata validatorAddress,
        string calldata ipAddress,
        string calldata note,
        string calldata supernodeAccount,
        string calldata p2pPort
    ) external returns (bool success);

    /// @notice Report hardware/software metrics for compliance checking.
    /// @return compliant Whether the metrics meet minimum requirements
    /// @return issues    List of compliance issues (empty if compliant)
    function reportMetrics(
        string calldata validatorAddress,
        string calldata supernodeAccount,
        MetricsReport calldata metrics
    ) external returns (bool compliant, string[] memory issues);

    // ─── Queries ───────────────────────────────────────────

    /// @notice Look up a supernode by validator address.
    function getSuperNode(string calldata validatorAddress)
        external view returns (SuperNodeInfo memory info);

    /// @notice Look up a supernode by its account address (secondary index).
    function getSuperNodeByAccount(string calldata supernodeAddress)
        external view returns (SuperNodeInfo memory info);

    /// @notice List all supernodes (paginated, max 100 per call).
    function listSuperNodes(uint64 offset, uint64 limit)
        external view returns (SuperNodeInfo[] memory nodes, uint64 total);

    /// @notice Get supernodes ranked by XOR distance from block hash.
    /// @param blockHeight Target block height for distance calculation
    /// @param limit       Max number of results
    /// @param state       Filter by state (0 = all states)
    function getTopSuperNodesForBlock(int32 blockHeight, int32 limit, uint8 state)
        external view returns (SuperNodeInfo[] memory nodes);

    /// @notice Get the latest metrics report for a supernode.
    /// @return metrics          The most recent metrics snapshot
    /// @return reportCount      Total number of reports submitted
    /// @return lastReportHeight Block height of the last report
    function getMetrics(string calldata validatorAddress)
        external view returns (MetricsReport memory metrics, uint64 reportCount, int64 lastReportHeight);

    /// @notice Query module parameters.
    function getParams()
        external view returns (
            uint256 minimumStake,
            uint64  reportingThreshold,
            uint64  slashingThreshold,
            string memory minSupernodeVersion,
            uint64  minCpuCores,
            uint64  minMemGb,
            uint64  minStorageGb
        );
}
```

---

## Example: Supernode Manager Contract

A contract that manages the full supernode lifecycle:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./ISupernode.sol";

contract SupernodeManager {
    ISupernode constant SN = ISupernode(0x0000000000000000000000000000000000000902);

    event Registered(string validatorAddress);
    event MetricsReported(string validatorAddress, bool compliant);

    /// @notice Register a new supernode.
    function register(
        string calldata validatorAddress,
        string calldata ipAddress,
        string calldata supernodeAccount,
        string calldata p2pPort
    ) external {
        SN.registerSupernode(validatorAddress, ipAddress, supernodeAccount, p2pPort);
        emit Registered(validatorAddress);
    }

    /// @notice Report metrics and check compliance.
    function reportAndCheck(
        string calldata validatorAddress,
        string calldata supernodeAccount,
        ISupernode.MetricsReport calldata metrics
    ) external returns (bool compliant, string[] memory issues) {
        (compliant, issues) = SN.reportMetrics(validatorAddress, supernodeAccount, metrics);
        emit MetricsReported(validatorAddress, compliant);
    }

    /// @notice Gracefully stop a supernode with a reason.
    function gracefulStop(string calldata validatorAddress, string calldata reason) external {
        SN.stopSupernode(validatorAddress, reason);
    }

    /// @notice Restart a previously stopped supernode.
    function restart(string calldata validatorAddress) external {
        SN.startSupernode(validatorAddress);
    }
}
```

---

## Example: Supernode Dashboard (Read-Only)

A view contract for monitoring supernodes:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./ISupernode.sol";

contract SupernodeDashboard {
    ISupernode constant SN = ISupernode(0x0000000000000000000000000000000000000902);

    /// @notice Get the total number of registered supernodes.
    function totalSupernodes() external view returns (uint64) {
        (, uint64 total) = SN.listSuperNodes(0, 1);
        return total;
    }

    /// @notice Get a page of supernodes.
    function getPage(uint64 page, uint64 perPage)
        external view returns (ISupernode.SuperNodeInfo[] memory nodes, uint64 total)
    {
        uint64 limit = perPage > 100 ? 100 : perPage;
        return SN.listSuperNodes(page * limit, limit);
    }

    /// @notice Get the top N supernodes for a given block (active only).
    function topForBlock(int32 blockHeight, int32 count)
        external view returns (ISupernode.SuperNodeInfo[] memory)
    {
        return SN.getTopSuperNodesForBlock(blockHeight, count, 1); // 1 = Active
    }

    /// @notice Check a supernode's compliance status.
    function isHealthy(string calldata validatorAddress)
        external view returns (bool hasReported, uint64 reportCount)
    {
        (, reportCount,) = SN.getMetrics(validatorAddress);
        hasReported = reportCount > 0;
    }

    /// @notice Get minimum stake required to register.
    function minimumStake() external view returns (uint256) {
        (uint256 stake,,,,,,) = SN.getParams();
        return stake;
    }
}
```

---

## Using from ethers.js / viem

The precompile can be called directly from JavaScript without deploying any contract:

```typescript
import { ethers } from "ethers";

const SN_ADDRESS = "0x0000000000000000000000000000000000000902";

// Minimal ABI for the methods you need
const SN_ABI = [
  "function getParams() view returns (uint256 minimumStake, uint64 reportingThreshold, uint64 slashingThreshold, string minSupernodeVersion, uint64 minCpuCores, uint64 minMemGb, uint64 minStorageGb)",
  "function getSuperNode(string validatorAddress) view returns (tuple(string validatorAddress, string supernodeAccount, uint8 currentState, int64 stateHeight, string ipAddress, string p2pPort, string note, uint64 evidenceCount))",
  "function listSuperNodes(uint64 offset, uint64 limit) view returns (tuple(string validatorAddress, string supernodeAccount, uint8 currentState, int64 stateHeight, string ipAddress, string p2pPort, string note, uint64 evidenceCount)[], uint64 total)",
  "function getTopSuperNodesForBlock(int32 blockHeight, int32 limit, uint8 state) view returns (tuple(string validatorAddress, string supernodeAccount, uint8 currentState, int64 stateHeight, string ipAddress, string p2pPort, string note, uint64 evidenceCount)[])",
  "function getMetrics(string validatorAddress) view returns (tuple(uint32 versionMajor, uint32 versionMinor, uint32 versionPatch, uint32 cpuCoresTotal, uint64 cpuUsagePercent, uint64 memTotalGb, uint64 memUsagePercent, uint64 memFreeGb, uint64 diskTotalGb, uint64 diskUsagePercent, uint64 diskFreeGb, uint64 uptimeSeconds, uint32 peersCount), uint64 reportCount, int64 lastReportHeight)",
  "function registerSupernode(string validatorAddress, string ipAddress, string supernodeAccount, string p2pPort) returns (bool)",
  "function reportMetrics(string validatorAddress, string supernodeAccount, tuple(uint32 versionMajor, uint32 versionMinor, uint32 versionPatch, uint32 cpuCoresTotal, uint64 cpuUsagePercent, uint64 memTotalGb, uint64 memUsagePercent, uint64 memFreeGb, uint64 diskTotalGb, uint64 diskUsagePercent, uint64 diskFreeGb, uint64 uptimeSeconds, uint32 peersCount) metrics) returns (bool compliant, string[] issues)",
  "event SupernodeRegistered(string indexed validatorAddress, address indexed creator, uint8 newState)",
  "event SupernodeStateChanged(string indexed validatorAddress, address indexed creator, uint8 newState)",
];

const provider = new ethers.JsonRpcProvider("http://localhost:8545");
const signer = new ethers.Wallet(PRIVATE_KEY, provider);
const supernode = new ethers.Contract(SN_ADDRESS, SN_ABI, signer);

// Query module params
const params = await supernode.getParams();
console.log(`Min stake: ${params.minimumStake} ulume`);
console.log(`Min version: ${params.minSupernodeVersion}`);

// List first 10 supernodes
const [nodes, total] = await supernode.listSuperNodes(0n, 10n);
console.log(`Total supernodes: ${total}`);
for (const node of nodes) {
  console.log(`  ${node.validatorAddress} — state=${node.currentState}`);
}

// Register a supernode (state-changing tx)
const tx = await supernode.registerSupernode(
  "lumeravaloper1...",   // validatorAddress
  "203.0.113.42",        // ipAddress
  "lumera1...",           // supernodeAccount
  "26656"                // p2pPort
);
const receipt = await tx.wait();
console.log("Registered in tx:", receipt.hash);

// Listen for registration events
supernode.on("SupernodeRegistered", (validatorAddress, creator, newState) => {
  console.log(`Supernode ${validatorAddress} registered by ${creator}, state=${newState}`);
});
```

---

## Implementation Details

### Source Files

| File | Purpose |
|------|---------|
| `precompiles/supernode/abi.json` | Hardhat-format ABI definition |
| `precompiles/supernode/supernode.go` | Core precompile struct, `Execute()` dispatch, address constant |
| `precompiles/supernode/types.go` | `SuperNodeInfo`, `MetricsReport` structs, float↔int conversion helpers |
| `precompiles/supernode/events.go` | EVM log emission (`SupernodeRegistered`, `SupernodeDeregistered`, `SupernodeStateChanged`) |
| `precompiles/supernode/tx.go` | All 6 transaction handlers |
| `precompiles/supernode/query.go` | All 6 query handlers |

### State Extraction

The keeper's `SuperNode` protobuf stores state history as a slice (`States []SuperNodeStateRecord`). The precompile extracts the **latest** entry for `currentState` and `stateHeight`. Similarly, IP address is read from the last entry in `PrevIpAddresses`, not a dedicated field.

### Metrics Compliance

`reportMetrics` is unique among precompile transactions — it returns structured data (`compliant bool, issues []string`) rather than just a success flag. The underlying keeper checks hardware metrics against minimum thresholds (`minCpuCores`, `minMemGb`, `minStorageGb`, `minSupernodeVersion`) and returns specific failure reasons.

### Query Pagination

`listSuperNodes` enforces a maximum of **100 results per call**. If `limit > 100`, it is silently capped. Use `offset` for pagination:

```solidity
uint64 offset = 0;
uint64 total;
do {
    (ISupernode.SuperNodeInfo[] memory batch, total) = SN.listSuperNodes(offset, 50);
    // process batch...
    offset += 50;
} while (offset < total);
```

### Gas Metering

Precompile calls consume gas like any EVM operation. The gas cost is determined by the Cosmos EVM framework's `RunNativeAction` wrapper, which meters based on the underlying Cosmos gas consumption converted to EVM gas units.

---

## Integration Tests

The precompile has integration test coverage in `tests/integration/evm/precompiles/`:

| Test | What it verifies |
|------|-----------------|
| `SupernodePrecompileGetParamsViaEthCall` | `getParams()` returns 7 values, `minSupernodeVersion` is non-empty |
| `SupernodePrecompileListSuperNodesViaEthCall` | `listSuperNodes(0, 10)` returns valid data (total may be 0 on fresh chain) |
| `SupernodePrecompileGetTopSuperNodesForBlockViaEthCall` | `getTopSuperNodesForBlock(1, 10, 0)` returns valid data |

Run with:

```bash
go test -tags='integration test' ./tests/integration/evm/precompiles/... -v -timeout 10m
```
