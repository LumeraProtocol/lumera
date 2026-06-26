# Lumera EVM Upgrade Operator Brief

**Audience**: validators, supernode operators, relayers, RPC providers, explorers, exchanges, custodians  
**Tone**: formal release-note style  
**Use**: operational coordination brief for testnet and mainnet upgrade preparation; replace placeholders before sharing
**Publishing note**: links target the GitHub `v1.20.0` tag under `docs/evm-integration`; update them if a docs site URL becomes available.

Publish-time placeholders:

- `<NETWORK_NAME>`
- `<BINARY_VERSION>`
- `<UPGRADE_HEIGHT>`
- `<UPGRADE_TIME_UTC>`
- `<CHECKSUMS_URL>`
- `<MIGRATION_WINDOW>`
- `<INCIDENT_CHANNEL>`
- `<SUPPORT_CHANNEL>`

## Summary

Lumera's EVM upgrade introduces Cosmos EVM support, Ethereum JSON-RPC access, EVM-compatible account keys, and legacy account migration through `x/evmigration`.

This brief summarizes operator responsibilities. It does not replace the detailed runbooks.

Canonical references:

- [Rollout Plan](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/architecture/rollout.md)
- [Lumera EVM User Guides](https://github.com/LumeraProtocol/lumera/tree/v1.20.0/docs/evm-integration/user-guides)
- [Node Operator EVM Configuration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/node-evm-config-guide.md)
- [Validator Operator EVM Migration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/validator-migration.md)
- [Supernode Operator EVM Migration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/supernode-migration.md)
- [Hermes IBC Relayer EVM Migration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/relayer-migration.md)

## Scope Of The Upgrade

The upgrade adds:

- EVM module integration
- Ethereum JSON-RPC service support
- EIP-1559-style fee market behavior
- EVM-compatible `eth_secp256k1` account keys
- Dual address presentation for EVM-compatible accounts
- Static precompiles, including Lumera-specific module precompiles
- Legacy account migration from pre-EVM account derivation to EVM-compatible account derivation

Operators should treat the upgrade as both a binary upgrade and an account/key migration event.

## Rollout Sequence

Target network: `<NETWORK_NAME>`  
Binary version: `<BINARY_VERSION>`  
Upgrade height: `<UPGRADE_HEIGHT>`  
Expected upgrade time: `<UPGRADE_TIME_UTC>`

Expected rollout stages:

1. Release candidate validation
2. Devnet upgrade rehearsal
3. Devnet migration rehearsal and soak
4. Testnet upgrade and public testing
5. Mainnet readiness review
6. Mainnet upgrade and migration window

Mainnet scheduling should not proceed until the release candidate, devnet, and testnet stages have produced acceptable operational evidence.

## Validator Operators

Validator operators must prepare for two separate concerns:

- Network binary upgrade at the announced height.
- Validator operator account migration, if the operator account is still legacy.

Key points:

- The validator consensus key is not migrated.
- Do not modify `priv_validator_key.json` for account migration.
- Validator operator accounts must use the validator-specific migration path.
- Validator migration may require a maintenance window.
- Operators should run pre-flight checks before stopping their node.
- Operators who also run a supernode should migrate the validator first when both roles share the same account.

Detailed runbook: [Validator Operator EVM Migration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/validator-migration.md)

## Supernode Operators

Supernode operators must prepare the EVM-compatible key configuration before restarting against the upgraded chain.

Key points:

- The common path is daemon-assisted migration after `evm_key_name` is configured.
- If the account was already migrated through the Portal or CLI, the daemon can detect the on-chain migration record and perform local cleanup.
- Multisig supernode accounts require the manual multisig migration ceremony.
- Operators who are also validators should follow the validator migration path first when both roles share the same account.

Detailed runbook: [Supernode Operator EVM Migration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/supernode-migration.md)

## Relayer Operators

Hermes relayer operators must ensure Lumera's EVM-compatible key derivation matches the key used by `lumerad`.

Key points:

- The Lumera EVM key path is `m/44'/60'/0'/0/0`.
- Hermes configuration must use the correct Ethereum-style key settings for Lumera.
- A derived-address check should pass before account migration.
- Relaying should be paused during the migration sequence.

Detailed runbook: [Hermes IBC Relayer EVM Migration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/relayer-migration.md)

## RPC Providers And Infrastructure Operators

Operators exposing EVM functionality must provide Ethereum JSON-RPC endpoints separately from Cosmos endpoints.

Key points:

- MetaMask and EVM tooling use Ethereum JSON-RPC.
- Keplr and Cosmos tooling use Cosmos LCD/REST or CometBFT RPC.
- Public EVM RPC endpoints should be HTTPS.
- WebSocket support should be configured where dapps and indexers need it.
- Rate limiting, namespace exposure, and tracing settings should be reviewed before public traffic.

Detailed runbook: [Node Operator EVM Configuration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/node-evm-config-guide.md)

## Explorers, Wallets, Exchanges, And Custodians

Partners should review address, deposit, withdrawal, and indexing behavior before public rollout.

Preparation checklist:

- Confirm expected Cosmos chain ID and EVM chain ID.
- Confirm public EVM JSON-RPC endpoint availability.
- Confirm address handling for Bech32 and `0x` representations.
- Confirm migration-state visibility requirements.
- Confirm deposit and withdrawal maintenance windows.
- Confirm whether legacy addresses require special handling during the migration window.

## Communication Requirements

Before testnet and mainnet upgrade announcements, publish:

- Upgrade height or target window: `<UPGRADE_HEIGHT>`
- Binary version: `<BINARY_VERSION>`
- Checksums: `<CHECKSUMS_URL>`
- Operator runbook links
- Public RPC and explorer status
- Migration window policy: `<MIGRATION_WINDOW>`
- Support channel: `<SUPPORT_CHANNEL>`
- Incident channel: `<INCIDENT_CHANNEL>`
- Known limitations or delayed partner services

For mainnet, operator instructions should include exact timing, expected downtime, rollback posture, and who is authorized to publish halt or resume instructions.

## Go/No-Go Inputs

Recommended inputs before promotion from testnet to mainnet:

- Release candidate validation complete
- Upgrade rehearsal complete
- Migration rehearsal complete
- Validator and supernode runbooks reviewed
- Public RPC endpoint tested
- MetaMask connection tested
- Explorer and indexer plan accepted
- Support channels staffed
- Migration parameters confirmed
- Incident coordination path rehearsed

## References

- Public user quickstart: [Lumera EVM Migration Quickstart](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/community/user-migration-quickstart.md)
- Community announcement deck source: [Lumera EVM Upgrade Announcement Deck](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/community/evm-upgrade-announcement-slides.md)
- Full guide hub: [Lumera EVM User Guides](https://github.com/LumeraProtocol/lumera/tree/v1.20.0/docs/evm-integration/user-guides)
