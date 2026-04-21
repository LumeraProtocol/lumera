# EVM Account Migration

The EVM integration changes the default coin type from 118 (`secp256k1`) to 60 (`eth_secp256k1`). Existing accounts derived with coin type 118 produce different addresses than the same mnemonic with coin type 60. The `x/evmigration` module provides an atomic claim-and-move mechanism for migrating on-chain state from legacy to new addresses.

## Documentation

| Document | Description |
| --- | --- |
| [legacy-migration.md](legacy-migration.md) | `x/evmigration` module architecture, messages, params, migration sequence, queries, and implementation status |
| [migration.md](../user-guides/migration.md) | Step-by-step migration guide for end users (Portal + Keplr and CLI methods) |
| [portal-ui.md](portal-ui.md) | EVM Migration Portal UI and wallet rollout |
| [devnet-tests.md](devnet-tests.md) | `tests_evmigration` devnet end-to-end test tool (modes, module coverage, Makefile targets) |

## Overview

When Lumera upgrades to EVM support (v1.20.0), the same mnemonic produces a **different on-chain address** under coin type 60. The `x/evmigration` module provides `MsgClaimLegacyAccount` and `MsgMigrateValidator` transactions that atomically transfer all state from the old address to the new one.

### What gets migrated

| Module | State migrated |
| --- | --- |
| `x/auth` | Account record (vesting-aware) |
| `x/bank` | All coin balances |
| `x/staking` | Delegations, unbonding, redelegations + UnbondingID indexes |
| `x/distribution` | Reward withdrawal, delegator starting info |
| `x/authz` | Grant re-keying (both grantor and grantee) |
| `x/feegrant` | Fee allowance re-keying (both granter and grantee) |
| `x/supernode` | SupernodeAccount, Evidence, PrevSupernodeAccounts, MetricsState |
| `x/action` | Creator and SuperNodes fields in action records |
| `x/claim` | DestAddress in claim records |
| `x/evmigration` | Core migration logic, dual-signature verification, rate limiting |

### Key design decisions

- **Dual-signature verification** -- migration requires signatures from both the legacy key and the new EVM key, proving ownership of both addresses
- **Zero-fee migration** -- a custom ante decorator waives gas fees for migration txs since the new address has no balance before migration completes
- **Rate limiting** -- `max_migrations_per_block` (default 50) prevents migration flood attacks
- **Validator migration** -- dedicated `MsgMigrateValidator` handles the additional complexity of re-keying validator records, delegator references, and consensus key mappings

See [legacy-migration.md](legacy-migration.md) for the full module reference.
