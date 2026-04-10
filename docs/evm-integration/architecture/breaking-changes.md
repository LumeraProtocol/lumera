# Breaking Changes and Operational Implications

This document captures the breaking changes and operational implications of integrating Cosmos EVM into Lumera mainnet (post-genesis). Each section explains what changes, what breaks (consensus vs UX/tooling), and the resolution Lumera implemented, with links to detailed sub-documents.

## Summary of breaking changes

| Change | Category | Impact | Resolution | Details |
| --- | --- | --- | --- | --- |
| Coin type 118 -> 60 | UX/wallet | Same mnemonic derives different address | Chain-assisted migration via `x/evmigration` | [coin-type-change.md](coin-type-change.md) |
| Key type `secp256k1` -> `eth_secp256k1` | Consensus/UX | Different address derivation function, dual account universes | `eth_secp256k1` as default + dual encoding model | [key-type-address.md](key-type-address.md) |
| Gas token 6 -> 18 decimals | EVM tooling | EVM expects wei-like 18-decimal units | `x/precisebank` bridges 6-decimal bank to 18-decimal EVM view | [gas-token-decimals.md](gas-token-decimals.md) |
| EIP-1559 fee market | Economics/ops | Dynamic base fee, priority tips, new fee distribution model | `x/feemarket` with Lumera-tuned defaults | [fee-market.md](fee-market.md) |
| Bank -> ERC-20 token mapping | EVM tooling | EVM dApps expect ERC-20 interfaces | STRv2 via `x/erc20` with governance-controlled IBC policy | [token-representation.md](token-representation.md) |

## How to read these documents

Each sub-document follows a consistent structure:

1. **What changes** -- the technical change and why it's needed
2. **What breaks** -- concrete impact on users, tooling, and operations
3. **Lumera's resolution** -- the approach chosen and how it was implemented
4. **Operational checklist** -- what operators, wallets, explorers, and exchanges need to know

## Related documents

- [../evmigration/main.md](../evmigration/main.md) -- Legacy account migration module (`x/evmigration`) that handles the coin-type transition
- [../main.md](../main.md) -- EVM integration overview with architecture strengths and operational outcomes
- [app-changes.md](app-changes.md) -- Detailed app-level code changes for each feature
