# Lumera EVM Migration Quickstart

**Audience**: public users  
**Tone**: community-friendly  
**Use**: short public guide for testnet and later mainnet migration announcements; replace placeholders before sharing
**Publishing note**: links target the GitHub `v1.20.0` tag under `docs/evm-integration`; update them if a docs site URL becomes available.

Publish-time placeholders:

- `<NETWORK_NAME>`
- `<PORTAL_URL>`
- `<SUPPORT_CHANNEL>`
- `<MIGRATION_WINDOW>`

Lumera's EVM upgrade adds Ethereum-compatible accounts and tooling to the Lumera chain. Because the upgraded chain uses Ethereum-style account keys, some users need to migrate their existing Lumera account state from a legacy address to a new EVM-compatible address.

This quickstart explains what that means and where to go next.

## Do I Need To Migrate?

You likely need to migrate if:

- You had a Lumera account before the EVM upgrade.
- Your wallet uses the legacy Lumera account type.
- The Portal shows a legacy account with balances, delegations, or other state that needs migration.

You likely do not need to migrate if:

- You created a fresh EVM-compatible Lumera account after the upgrade.
- You only want to test new EVM functionality with a new testnet account.
- The Portal or chain query shows your legacy account has already been migrated.

## Why The New Address Looks Empty

The EVM upgrade changes how addresses are derived from a recovery phrase.

- Legacy Lumera accounts use Cosmos-style derivation.
- EVM-compatible Lumera accounts use Ethereum-style derivation.
- The same recovery phrase can produce a different Lumera address after the upgrade.

If the new EVM-compatible address looks empty before migration, that does not mean funds are lost. Your state remains at the legacy address until migration moves it.

## Recommended Path: Portal + Keplr

For most users, the recommended migration path is the Lumera Portal.

High-level flow:

1. Open the official Lumera Portal: `<PORTAL_URL>`.
2. Select the legacy `<NETWORK_NAME>` network profile.
3. Connect the legacy account with Keplr.
4. Open the EVM migration page.
5. Review the migration summary.
6. Sign the required prompts.
7. Wait for the Portal to confirm success.
8. Reconnect using the EVM-compatible Lumera profile.

Full walkthrough with screenshots: [EVM Legacy Account Migration - User Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/migration.md)

## After Migration

After a successful migration:

- Your legacy account is no longer the active account.
- Your migrated balances and state are associated with the new EVM-compatible Lumera address.
- Wallets may need to reconnect or re-import the account using the EVM-compatible profile.
- MetaMask can be used with Lumera's EVM JSON-RPC endpoint.

MetaMask setup guide: [MetaMask Configuration Guide](https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/metamask-configuration.md)

## Important Safety Notes

- Migration is one-way. Do not start unless you are using the official flow and understand the destination address.
- Do not send funds to the fresh EVM destination address before migration unless the official guide explicitly tells you to. Migration expects the destination to be fresh.
- Never paste a seed phrase into a website, chat, support ticket, or community channel.
- Use official Lumera links and verify the network name before signing.
- If you are a validator, supernode operator, relayer, multisig coordinator, exchange, or custodian, use the operator guides instead of this quickstart.

## Special Cases

Some account types need additional care:

- Validators must use the validator migration flow.
- Supernode operators may need daemon configuration changes.
- Multisig accounts use an offline multi-step ceremony.
- Relayers need both Lumera and Hermes key derivation to match.

Start from the guide hub if any of these apply: [Lumera EVM User Guides](https://github.com/LumeraProtocol/lumera/tree/v1.20.0/docs/evm-integration/user-guides)

## Where To Get Help

Before asking for support, collect:

- Network name: `<NETWORK_NAME>`
- Legacy Lumera address
- New EVM-compatible Lumera address, if shown
- Portal status message
- Transaction hash, if a migration transaction was submitted

Do not share seed phrases, private keys, key files, or full wallet backups.

Support channel: `<SUPPORT_CHANNEL>`  
Migration window: `<MIGRATION_WINDOW>`
