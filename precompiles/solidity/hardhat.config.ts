import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";

// Default devnet private key — the first validator's EVM key.
// Override with DEPLOYER_PRIVATE_KEY env var for non-default accounts.
// NEVER use this key on mainnet or public testnets.
const DEVNET_DEFAULT_KEY =
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.24",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200,
      },
      evmVersion: "shanghai",
    },
  },
  networks: {
    // Local devnet (make devnet-up)
    devnet: {
      url: process.env.LUMERA_RPC_URL || "http://localhost:8545",
      chainId: 76857769,
      accounts: [process.env.DEPLOYER_PRIVATE_KEY || DEVNET_DEFAULT_KEY],
    },
    // Single-node integration test
    localnode: {
      url: "http://localhost:8545",
      chainId: 76857769,
      accounts: [process.env.DEPLOYER_PRIVATE_KEY || DEVNET_DEFAULT_KEY],
    },
  },
  // Type generation for ethers.js contract bindings
  typechain: {
    outDir: "typechain-types",
    target: "ethers-v6",
  },
};

export default config;
