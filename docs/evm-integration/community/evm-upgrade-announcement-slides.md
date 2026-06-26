---
marp: true
title: Lumera EVM Upgrade Announcement
description: Community-facing Lumera EVM testnet/mainnet announcement deck
paginate: true
theme: default
---

# Lumera EVM Upgrade Announcement

**Audience**: community, users, ecosystem partners  
**Tone**: community-friendly  
**Use**: testnet announcement deck; replace placeholders before sharing
**Publishing note**: links target the GitHub `v1.20.0` tag under `docs/evm-integration`; update them if a docs site URL becomes available.
**Render**: `npx @marp-team/marp-cli docs/evm-integration/community/evm-upgrade-announcement-slides.md --pdf`

Publish-time placeholders:

- `<NETWORK_NAME>`
- `<TESTNET_DATE>`
- `<UPGRADE_HEIGHT>`
- `<PORTAL_URL>`
- `<SUPPORT_CHANNEL>`
- `<MAINNET_TARGET_WINDOW>`

---

# Lumera Is Becoming EVM-Compatible

`<NETWORK_NAME>` EVM testnet announcement  
Target date: `<TESTNET_DATE>`

Lumera is adding Ethereum-compatible execution to the existing Lumera chain.

What this unlocks:

- MetaMask-compatible accounts
- Ethereum JSON-RPC access
- Solidity smart contract support
- EVM tooling for developers and integrators
- Native Lumera functionality exposed through precompiles

Speaker note: keep the message simple. This is not a new chain; it is an upgrade to Lumera.

---

# What Stays The Same

The EVM upgrade does not remove Lumera's existing Cosmos functionality.

Still available:

- Lumera accounts and Bech32 addresses
- Staking and validator operations
- Supernode operations
- IBC and Cosmos-style transactions
- Existing Lumera modules and application logic

Speaker note: emphasize continuity. The main visible change is account/key compatibility for the EVM-enabled chain.

---

# What Changes

After the upgrade, Lumera supports Ethereum-style keys and addresses.

The important account change:

- Legacy Lumera accounts use coin type 118 and `secp256k1`.
- EVM-compatible Lumera accounts use coin type 60 and `eth_secp256k1`.
- The same recovery phrase can derive a different Lumera address after the upgrade.

This is why migration exists.

---

# Migration In Plain English

Migration moves account state from a legacy Lumera address to the new EVM-compatible address.

Migration can move:

- Balances
- Delegations
- Rewards and staking state
- Authz and feegrant state
- Supernode-related records, when applicable

Funds are not lost because the new address looks empty before migration. They are still at the legacy address until migration is completed.

---

# Recommended User Path

For most users, the recommended path is the Lumera Portal migration flow.

Users should:

1. Open the official Lumera Portal: `<PORTAL_URL>`.
2. Connect the legacy Lumera account with Keplr.
3. Review the migration summary.
4. Sign the required proofs.
5. Confirm the migration result.
6. Reconnect using the EVM-compatible profile.

Detailed guide: [EVM Legacy Account Migration - User Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/migration.md)

---

# MetaMask Support

After the EVM upgrade, users and developers can connect MetaMask to Lumera's EVM JSON-RPC endpoint.

Important distinction:

- Keplr uses Cosmos endpoints.
- MetaMask uses Ethereum JSON-RPC endpoints.

Using a Cosmos RPC URL in MetaMask will not work.

Detailed guide: [MetaMask Configuration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/metamask-configuration.md)

---

# For Developers

Developers can test EVM workflows on Lumera using familiar Ethereum tooling.

Examples:

- Connect MetaMask.
- Deploy a Solidity contract.
- Use Ethereum JSON-RPC.
- Explore OpenRPC.
- Call Lumera-specific precompiles.

Developer guides:

- [Testing Smart Contracts on Lumera with Remix IDE](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/guides/remix-guide.md)
- [OpenRPC Discovery and Playground Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/guides/openrpc-playground.md)
- [Precompiles Overview](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/precompiles/precompiles.md)

---

# For Validators And Operators

Operators should prepare before the network upgrade.

Upgrade height: `<UPGRADE_HEIGHT>`

Validators:

- Install the EVM-enabled binary at the announced upgrade height.
- Follow the validator migration runbook if the operator account is still legacy.
- Do not change the validator consensus key.

Supernode operators:

- Prepare the EVM key configuration.
- Restart with the EVM-enabled supernode flow after the chain upgrade.

Operator references:

- [Validator Operator EVM Migration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/validator-migration.md)
- [Supernode Operator EVM Migration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/supernode-migration.md)
- [Node Operator EVM Configuration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/node-evm-config-guide.md)

---

# Testnet First, Mainnet Later

The rollout should proceed in stages:

1. Release candidate validation
2. Devnet upgrade rehearsal
3. Testnet rollout and community testing
4. Mainnet readiness review
5. Mainnet upgrade and migration window

The testnet phase is where users, validators, developers, and partners should rehearse the flow before mainnet.

Mainnet target window: `<MAINNET_TARGET_WINDOW>`

Rollout reference: [Rollout Plan](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/architecture/rollout.md)

---

# What To Do Now

Community:

- Join the testnet announcement channels.
- Try the Portal migration flow when testnet opens.
- Connect MetaMask to the EVM JSON-RPC endpoint.
- Report confusing steps or failed flows.

Operators and partners:

- Review the relevant runbooks.
- Confirm endpoint, explorer, wallet, and monitoring readiness.
- Coordinate before mainnet scheduling.

Support channel: `<SUPPORT_CHANNEL>`

Start here: [Lumera EVM User Guides](https://github.com/LumeraProtocol/lumera/tree/v1.20.0/docs/evm-integration/user-guides)
