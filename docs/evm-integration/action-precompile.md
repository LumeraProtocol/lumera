# Action Module EVM Precompile

The Lumera action precompile exposes the `x/action` module to the EVM at a single static address, enabling Solidity contracts to request, finalize, approve, and query Cascade and Sense actions without leaving the EVM execution context.

## Design Overview

### Address

```
0x0000000000000000000000000000000000000901
```

Lumera custom precompiles start at `0x0900`, following the convention:
- `0x01`–`0x0a` — Ethereum standard precompiles
- `0x0100`–`0x0806` — Cosmos EVM standard precompiles (bank, staking, distribution, gov, ICS20, bech32, p256, slashing)
- `0x0900`+ — Lumera-specific custom precompiles

### Hybrid Typed/Generic Approach

The precompile uses **typed methods** for operations that carry action-specific metadata (request and finalize), and **generic methods** for everything else (approve, queries):

| Category | Methods | Why typed? |
|----------|---------|------------|
| **Typed (Cascade)** | `requestCascade`, `finalizeCascade` | Metadata fields differ per action type — typed params give Solidity compile-time safety |
| **Typed (Sense)** | `requestSense`, `finalizeSense` | Same reason — Sense has different metadata fields than Cascade |
| **Generic** | `approveAction`, `getAction`, `getActionFee`, `getParams`, `getActionsByState`, `getActionsByCreator`, `getActionsBySuperNode` | These are metadata-agnostic — same signature regardless of action type |

### Action Lifecycle

```
Request (Pending) → Processing → Finalize (Done) → Approve (Approved)
                                                  ↘ Rejected / Failed / Expired
```

| State | Value | Description |
|-------|-------|-------------|
| Pending | 1 | Newly created, awaiting supernode processing |
| Processing | 2 | Supernodes are working on the action |
| Done | 3 | Supernode finalized, awaiting creator approval |
| Approved | 4 | Creator approved the result |
| Rejected | 5 | Creator rejected the result |
| Failed | 6 | Processing failed |
| Expired | 7 | Exceeded expiration time |

### Action Types

| Type | Value | Use case |
|------|-------|----------|
| Sense | 1 | Data analysis — duplicate detection and fingerprinting |
| Cascade | 2 | Distributed storage — redundancy-encoded file storage |

---

## Solidity Interface

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IAction — Lumera Action Module Precompile
/// @notice Call at 0x0000000000000000000000000000000000000901
interface IAction {

    // ─── Structs ───────────────────────────────────────────

    struct ActionInfo {
        string   actionId;
        address  creator;
        uint8    actionType;    // 1=Sense, 2=Cascade
        uint8    state;         // 1=Pending … 7=Expired
        string   metadata;     // JSON string
        uint256  price;        // in ulume
        int64    expirationTime;
        int64    blockHeight;
        address[] superNodes;
    }

    // ─── Events ────────────────────────────────────────────

    event ActionRequested(
        string indexed actionId,
        address indexed creator,
        uint8 actionType,
        uint256 price
    );

    event ActionFinalized(
        string indexed actionId,
        address indexed superNode,
        uint8 newState
    );

    event ActionApproved(
        string indexed actionId,
        address indexed creator
    );

    // ─── Cascade (typed) ───────────────────────────────────

    /// @notice Request a new Cascade storage action.
    /// @param dataHash       Hash of the data to store
    /// @param fileName       Original file name
    /// @param rqIdsIc        Initial RaptorQ symbol count
    /// @param signatures     Creator signatures (encoded bytes)
    /// @param price          Payment in ulume
    /// @param expirationTime Unix timestamp for action expiry
    /// @param fileSizeKbs    File size in kilobytes (used for fee calc)
    /// @return actionId      The created action's unique identifier
    function requestCascade(
        string calldata dataHash,
        string calldata fileName,
        uint64 rqIdsIc,
        bytes  calldata signatures,
        uint256 price,
        int64  expirationTime,
        uint64 fileSizeKbs
    ) external returns (string memory actionId);

    /// @notice Finalize a Cascade action with storage proof.
    /// @param actionId  The action to finalize
    /// @param rqIdsIds  RaptorQ symbol identifiers produced by the supernode
    /// @return success  True if finalization succeeded
    function finalizeCascade(
        string calldata actionId,
        string[] calldata rqIdsIds
    ) external returns (bool success);

    // ─── Sense (typed) ─────────────────────────────────────

    /// @notice Request a new Sense analysis action.
    /// @param dataHash              Hash of the data to analyze
    /// @param ddAndFingerprintsIc   Initial duplicate-detection fingerprint count
    /// @param price                 Payment in ulume
    /// @param expirationTime        Unix timestamp for action expiry
    /// @param fileSizeKbs           File size in kilobytes
    /// @return actionId             The created action's unique identifier
    function requestSense(
        string calldata dataHash,
        uint64 ddAndFingerprintsIc,
        uint256 price,
        int64  expirationTime,
        uint64 fileSizeKbs
    ) external returns (string memory actionId);

    /// @notice Finalize a Sense action with analysis results.
    /// @param actionId              The action to finalize
    /// @param ddAndFingerprintsIds  Result fingerprint identifiers
    /// @param signatures            Supernode signatures
    /// @return success              True if finalization succeeded
    function finalizeSense(
        string calldata actionId,
        string[] calldata ddAndFingerprintsIds,
        string calldata signatures
    ) external returns (bool success);

    // ─── Generic operations ────────────────────────────────

    /// @notice Approve a finalized action (creator only).
    function approveAction(string calldata actionId) external returns (bool success);

    /// @notice Look up a single action by ID.
    function getAction(string calldata actionId) external view returns (ActionInfo memory action);

    /// @notice Calculate action fees for a given data size.
    /// @return baseFee   Base fee component (ulume)
    /// @return perKbFee  Per-kilobyte fee component (ulume)
    /// @return totalFee  baseFee + perKbFee * dataSizeKbs
    function getActionFee(uint64 dataSizeKbs)
        external view returns (uint256 baseFee, uint256 perKbFee, uint256 totalFee);

    /// @notice Query module parameters.
    function getParams()
        external view returns (
            uint256 baseActionFee,
            uint256 feePerKbyte,
            uint64  maxActionsPerBlock,
            uint64  minSuperNodes,
            int64   expirationDuration,
            string memory superNodeFeeShare,
            string memory foundationFeeShare
        );

    /// @notice List actions by state (paginated, max 100 per call).
    function getActionsByState(uint8 state, uint64 offset, uint64 limit)
        external view returns (ActionInfo[] memory actions, uint64 total);

    /// @notice List actions by creator address (paginated, max 100 per call).
    function getActionsByCreator(address creator, uint64 offset, uint64 limit)
        external view returns (ActionInfo[] memory actions, uint64 total);

    /// @notice List actions by assigned supernode (paginated, max 100 per call).
    function getActionsBySuperNode(address superNode, uint64 offset, uint64 limit)
        external view returns (ActionInfo[] memory actions, uint64 total);
}
```

---

## Example: Cascade Storage Client

A contract that requests Cascade file storage and tracks the resulting action:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./IAction.sol";

contract CascadeStorageClient {
    IAction constant ACTION = IAction(0x0000000000000000000000000000000000000901);

    /// @notice Stores a mapping of data hash → action ID for tracking.
    mapping(string => string) public uploads;

    event UploadRequested(string indexed dataHash, string actionId, uint256 totalFee);

    /// @notice Request a Cascade storage action.
    /// @dev The caller must have sufficient ulume balance for the price.
    function uploadFile(
        string calldata dataHash,
        string calldata fileName,
        uint64 rqIdsIc,
        bytes  calldata signatures,
        uint64 fileSizeKbs
    ) external {
        // 1. Query the fee to determine the price
        (,, uint256 totalFee) = ACTION.getActionFee(fileSizeKbs);

        // 2. Set expiration to 1 hour from now
        int64 expiration = int64(int256(block.timestamp)) + 3600;

        // 3. Request the Cascade action
        string memory actionId = ACTION.requestCascade(
            dataHash,
            fileName,
            rqIdsIc,
            signatures,
            totalFee,
            expiration,
            fileSizeKbs
        );

        uploads[dataHash] = actionId;
        emit UploadRequested(dataHash, actionId, totalFee);
    }

    /// @notice Check current state of an upload.
    /// @return state  1=Pending, 2=Processing, 3=Done, 4=Approved
    function checkUploadState(string calldata dataHash) external view returns (uint8 state) {
        string memory actionId = uploads[dataHash];
        IAction.ActionInfo memory info = ACTION.getAction(actionId);
        return info.state;
    }

    /// @notice Approve a completed upload (only the original creator can call).
    function approveUpload(string calldata dataHash) external {
        string memory actionId = uploads[dataHash];
        ACTION.approveAction(actionId);
    }
}
```

---

## Example: Sense Analysis Client

A contract that submits data for duplicate detection analysis:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./IAction.sol";

contract SenseAnalysisClient {
    IAction constant ACTION = IAction(0x0000000000000000000000000000000000000901);

    struct AnalysisRequest {
        string actionId;
        address requester;
        uint8  state;
    }

    mapping(string => AnalysisRequest) public analyses;

    event AnalysisRequested(string indexed dataHash, string actionId);

    /// @notice Request a Sense analysis for the given data hash.
    function analyzeData(
        string calldata dataHash,
        uint64 ddAndFingerprintsIc,
        uint64 fileSizeKbs
    ) external {
        (,, uint256 totalFee) = ACTION.getActionFee(fileSizeKbs);
        int64 expiration = int64(int256(block.timestamp)) + 7200; // 2 hours

        string memory actionId = ACTION.requestSense(
            dataHash,
            ddAndFingerprintsIc,
            totalFee,
            expiration,
            fileSizeKbs
        );

        analyses[dataHash] = AnalysisRequest({
            actionId: actionId,
            requester: msg.sender,
            state: 1 // Pending
        });

        emit AnalysisRequested(dataHash, actionId);
    }

    /// @notice Refresh cached state from the chain.
    function refreshState(string calldata dataHash) external {
        AnalysisRequest storage req = analyses[dataHash];
        IAction.ActionInfo memory info = ACTION.getAction(req.actionId);
        req.state = info.state;
    }
}
```

---

## Example: Fee Calculator View

A read-only contract for fee estimation (useful for front-ends):

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./IAction.sol";

contract ActionFeeCalculator {
    IAction constant ACTION = IAction(0x0000000000000000000000000000000000000901);

    /// @notice Estimate the total fee for a given file size.
    /// @param fileSizeBytes File size in bytes
    /// @return totalFeeUlume Total fee in ulume
    function estimateFee(uint256 fileSizeBytes) external view returns (uint256 totalFeeUlume) {
        uint64 sizeKbs = uint64((fileSizeBytes + 1023) / 1024); // round up
        (,, uint256 totalFee) = ACTION.getActionFee(sizeKbs);
        return totalFee;
    }

    /// @notice Return all module parameters.
    function moduleParams() external view returns (
        uint256 baseActionFee,
        uint256 feePerKbyte,
        uint64  maxActionsPerBlock,
        uint64  minSuperNodes
    ) {
        (baseActionFee, feePerKbyte, maxActionsPerBlock, minSuperNodes,,,) = ACTION.getParams();
    }
}
```

---

## Example: Action Dashboard (Paginated Queries)

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./IAction.sol";

contract ActionDashboard {
    IAction constant ACTION = IAction(0x0000000000000000000000000000000000000901);

    /// @notice Get pending actions count.
    function pendingCount() external view returns (uint64) {
        (, uint64 total) = ACTION.getActionsByState(1, 0, 1); // state=Pending, just get count
        return total;
    }

    /// @notice Get a page of actions for a creator.
    /// @param creator EVM address of the action creator
    /// @param page    Zero-indexed page number
    /// @param perPage Results per page (max 100)
    function getCreatorPage(address creator, uint64 page, uint64 perPage)
        external view returns (IAction.ActionInfo[] memory actions, uint64 total)
    {
        uint64 limit = perPage > 100 ? 100 : perPage;
        return ACTION.getActionsByCreator(creator, page * limit, limit);
    }

    /// @notice Get a page of actions assigned to a supernode.
    function getSuperNodePage(address superNode, uint64 page, uint64 perPage)
        external view returns (IAction.ActionInfo[] memory actions, uint64 total)
    {
        uint64 limit = perPage > 100 ? 100 : perPage;
        return ACTION.getActionsBySuperNode(superNode, page * limit, limit);
    }
}
```

---

## Using from ethers.js / viem

The precompile can be called directly from JavaScript without deploying any contract:

```typescript
import { ethers } from "ethers";

const ACTION_ADDRESS = "0x0000000000000000000000000000000000000901";

// Minimal ABI for the methods you need
const ACTION_ABI = [
  "function getActionFee(uint64 dataSizeKbs) view returns (uint256 baseFee, uint256 perKbFee, uint256 totalFee)",
  "function getParams() view returns (uint256 baseActionFee, uint256 feePerKbyte, uint64 maxActionsPerBlock, uint64 minSuperNodes, int64 expirationDuration, string superNodeFeeShare, string foundationFeeShare)",
  "function getAction(string actionId) view returns (tuple(string actionId, address creator, uint8 actionType, uint8 state, string metadata, uint256 price, int64 expirationTime, int64 blockHeight, address[] superNodes))",
  "function requestCascade(string dataHash, string fileName, uint64 rqIdsIc, bytes signatures, uint256 price, int64 expirationTime, uint64 fileSizeKbs) returns (string actionId)",
  "function approveAction(string actionId) returns (bool success)",
  "event ActionRequested(string indexed actionId, address indexed creator, uint8 actionType, uint256 price)",
];

const provider = new ethers.JsonRpcProvider("http://localhost:8545");
const signer = new ethers.Wallet(PRIVATE_KEY, provider);
const action = new ethers.Contract(ACTION_ADDRESS, ACTION_ABI, signer);

// Query fees
const [baseFee, perKbFee, totalFee] = await action.getActionFee(100n); // 100 KB
console.log(`Total fee for 100 KB: ${totalFee} ulume`);

// Request a Cascade action
const tx = await action.requestCascade(
  "abc123hash",           // dataHash
  "photo.jpg",            // fileName
  42n,                    // rqIdsIc
  "0x",                   // signatures
  totalFee,               // price
  BigInt(Math.floor(Date.now() / 1000) + 3600), // expiration
  100n                    // fileSizeKbs
);
const receipt = await tx.wait();
console.log("Action created in tx:", receipt.hash);

// Listen for ActionRequested events
action.on("ActionRequested", (actionId, creator, actionType, price) => {
  console.log(`New action ${actionId} by ${creator}, type=${actionType}, price=${price}`);
});
```

---

## Implementation Details

### Source Files

| File | Purpose |
|------|---------|
| `precompiles/action/abi.json` | Hardhat-format ABI definition |
| `precompiles/action/action.go` | Core precompile struct, `Execute()` dispatch, address constant |
| `precompiles/action/types.go` | `ActionInfo` struct, address conversion helpers |
| `precompiles/action/events.go` | EVM log emission (`ActionRequested`, `ActionFinalized`, `ActionApproved`) |
| `precompiles/action/tx_cascade.go` | `RequestCascade`, `FinalizeCascade` handlers |
| `precompiles/action/tx_sense.go` | `RequestSense`, `FinalizeSense` handlers |
| `precompiles/action/tx_common.go` | `ApproveAction` handler |
| `precompiles/action/query.go` | All read-only query handlers |

### Metadata Bridging

Typed Solidity parameters are converted to JSON inside the precompile, then passed to the Cosmos message server which handles the rest:

```
Solidity args (typed) → Go precompile → JSON metadata string → MsgRequestAction → Keeper
```

For example, `requestCascade(dataHash, fileName, rqIdsIc, signatures, ...)` becomes:

```json
{
  "data_hash": "abc123",
  "file_name": "photo.jpg",
  "rq_ids_ic": 42,
  "signatures": "base64..."
}
```

This is passed as the `Metadata` field of `MsgRequestAction`. The keeper's `ActionRegistry` then deserializes it into the appropriate protobuf type (`CascadeMetadata` or `SenseMetadata`).

### Address Translation

The precompile automatically converts between EVM hex addresses and Cosmos Bech32 addresses:

- **Inbound**: `contract.Caller()` (EVM `0x...`) → `lumera1...` (Bech32) for message server calls
- **Outbound**: `lumera1...` addresses in action records → `0x...` in `ActionInfo.creator` and `ActionInfo.superNodes`

### Gas Metering

Precompile calls consume gas like any EVM operation. The gas cost is determined by the Cosmos EVM framework's `RunNativeAction` / `RunStatefulAction` wrappers, which meter based on the underlying Cosmos gas consumption converted to EVM gas units.

### Query Pagination

All list queries (`getActionsByState`, `getActionsByCreator`, `getActionsBySuperNode`) enforce a maximum of **100 results per call**. If `limit > 100`, it is silently capped. Use `offset` for pagination:

```solidity
// Page through all pending actions, 50 at a time
uint64 offset = 0;
uint64 total;
do {
    (IAction.ActionInfo[] memory batch, total) = action.getActionsByState(1, offset, 50);
    // process batch...
    offset += 50;
} while (offset < total);
```

---

## Integration Tests

The precompile has integration test coverage in `tests/integration/evm/precompiles/`:

| Test | What it verifies |
|------|-----------------|
| `ActionPrecompileGetParamsViaEthCall` | `getParams()` returns valid non-zero module parameters |
| `ActionPrecompileGetActionFeeViaEthCall` | `getActionFee(100)` returns correct fee breakdown: `total == base + perKb * size` |
| `ActionPrecompileGetActionsByStateViaEthCall` | `getActionsByState(Pending, 0, 10)` returns empty on fresh chain |
| `ActionPrecompileGetActionsByCreatorViaEthCall` | `getActionsByCreator(addr, 0, 10)` returns empty for address with no actions |

Run with:

```bash
go test -tags='integration test' ./tests/integration/evm/precompiles/... -v -timeout 10m
```
