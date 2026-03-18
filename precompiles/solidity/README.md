# Lumera Precompile Solidity Examples

Solidity interfaces and sample contracts for interacting with Lumera's custom EVM precompiles.

## Precompile Addresses

| Precompile | Address | Module |
|------------|---------|--------|
| **IAction** | `0x0000000000000000000000000000000000000901` | `x/action/v1` — Distributed GPU compute jobs (Cascade, Sense) |
| **ISupernode** | `0x0000000000000000000000000000000000000902` | `x/supernode/v1` — Supernode registration, metrics, governance |

## Project Structure

```
contracts/
  interfaces/
    IAction.sol           # Action precompile interface (import this in your contracts)
    ISupernode.sol         # Supernode precompile interface
  examples/
    ActionClient.sol       # Query fees, submit Cascade/Sense actions
    SupernodeClient.sol    # Query nodes, check health, list by rank
    LumeraDashboard.sol    # Aggregates both modules in a single contract
scripts/
  deploy.ts               # Deploy all example contracts
  interact.ts             # Query precompiles directly + via deployed contracts
test/
  precompiles.test.ts     # Hardhat tests (run against live node)
```

## Quick Start

```bash
# Install dependencies
npm install

# Compile contracts
npm run compile

# Start devnet (from repo root)
cd ../..
make devnet-new
cd precompiles/solidity

# Deploy to devnet
npm run deploy:devnet

# Run interaction script (direct precompile calls, no deploy needed)
npm run interact:devnet

# Run tests against devnet
npx hardhat test --network devnet
```

## Using the Interfaces in Your Contracts

Import the interface and call the precompile at its fixed address:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./interfaces/IAction.sol";
import "./interfaces/ISupernode.sol";

contract MyDApp {
    IAction constant ACTION = IAction(0x0000000000000000000000000000000000000901);
    ISupernode constant SUPERNODE = ISupernode(0x0000000000000000000000000000000000000902);

    function getStorageCost(uint64 fileSizeKbs) external view returns (uint256) {
        (, , uint256 totalFee) = ACTION.getActionFee(fileSizeKbs);
        return totalFee;
    }

    function isNetworkHealthy() external view returns (bool) {
        (, uint64 totalNodes) = SUPERNODE.listSuperNodes(0, 1);
        (, , , uint64 minRequired, , , ) = ACTION.getParams();
        return totalNodes >= minRequired;
    }
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LUMERA_RPC_URL` | `http://localhost:8545` | JSON-RPC endpoint |
| `DEPLOYER_PRIVATE_KEY` | devnet default key | EVM private key for deployment |
| `ACTION_CLIENT` | — | Deployed ActionClient address (for interact.ts) |
| `SUPERNODE_CLIENT` | — | Deployed SupernodeClient address |
| `DASHBOARD` | — | Deployed LumeraDashboard address |

## Chain Configuration

| Parameter | Value |
|-----------|-------|
| Chain ID | `76857769` |
| Native denom | `ulume` (6 decimals) / `alume` (18 decimals EVM) |
| Key type | `eth_secp256k1` |
| EVM version | Shanghai |

## Notes

- **No gas cost for reads** — All `view` functions can be called via `eth_call` for free
- **Validator addresses** — The supernode module uses Bech32 `lumeravaloper...` strings (not EVM `address` type) because validator addresses don't have a meaningful 20-byte EVM representation
- **Fee denomination** — All fees are in `ulume` (1 LUME = 1,000,000 ulume). Use `ethers.formatUnits(value, 6)` for human-readable display
- **Precompiles are always available** — No deployment needed. The interfaces are just type-safe wrappers around `STATICCALL`/`CALL` to the fixed addresses
