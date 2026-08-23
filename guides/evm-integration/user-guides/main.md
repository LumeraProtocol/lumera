# Lumera EVM User Guides

**Last updated**: 2026-06-17
**Applies to**: Lumera chain post-EVM upgrade (`x/evmigration` enabled, Cosmos EVM v0.6.0)

This directory holds the operator- and end-user-facing documentation for living on the EVM-enabled Lumera chain. Architecture, internals, and audit material live one level up under [main.md](../main.md); this set is the "what do I do, and in what order" layer.

## Who should read what

| You are… | Start here | Then |
| --- | --- | --- |
| An end user with a legacy (coin-type 118) account | [migration.md](migration.md) | [migration-scripts.md](migration-scripts.md) if you have many accounts to batch |
| A validator operator | [validator-migration.md](validator-migration.md) | [migration-scripts.md](migration-scripts.md) for the `migrate-validator.sh` wrapper |
| A supernode operator | [supernode-migration.md](supernode-migration.md) | [migration.md](migration.md) for chain-level mechanics |
| A Hermes IBC relayer operator | [relayer-migration.md](relayer-migration.md) | [migration-scripts.md](migration-scripts.md) for the `migrate-account.sh` wrapper |
| A node operator (full node, sentry, public RPC) | [node-evm-config-guide.md](node-evm-config-guide.md) | [tune-guide.md](tune-guide.md) for parameter sizing |
| A MetaMask user or public-RPC operator | [metamask-configuration.md](metamask-configuration.md) | [node-evm-config-guide.md](node-evm-config-guide.md) for node-side JSON-RPC settings |
| A governance participant or chain steward | [tune-guide.md](tune-guide.md) | [node-evm-config-guide.md](node-evm-config-guide.md) for what each knob controls |

## Guides

### [migration.md](migration.md) — EVM Legacy Account Migration

The canonical migration walkthrough for end users. Covers the three supported paths (Portal + Keplr wizard, shell scripts, raw `lumerad` CLI), the four post-migration follow-up states on the Claim page, the Keplr re-import dance, and a full multisig section with the offline four-step proof ceremony (`generate-proof-payload` → `sign-proof` → `combine-proof` → `submit-proof`). Includes the FAQ that most operators end up needing.

### [migration-scripts.md](migration-scripts.md) — EVM Migration Helper Scripts

Reference for the bundled `scripts/migrate-account.sh`, `scripts/migrate-validator.sh`, and `scripts/migrate-multisig.sh`. These layer pre-flight estimates, destination-freshness checks, post-migration verification, structured exit codes, and an optional same-mnemonic file flow on top of the raw CLI commands. Use when migrating in bulk, scripting CI, or when you want safety rails the raw CLI doesn't provide.

### [validator-migration.md](validator-migration.md) — Validator Operator Migration

The validator-specific procedure: maintenance window planning against `downtime_jail_duration` (1 hour on mainnet), the `max_validator_delegations` cap (default 2500), the `unjail` recovery flow if pre-flight reports the validator is jailed or unbonded, and the consensus-key safety guarantee (`priv_validator_key.json` is **not** affected — only the operator key changes from `secp256k1` to `eth_secp256k1`). Includes the multisig validator operator path.

### [supernode-migration.md](supernode-migration.md) — Supernode Operator Migration

Two paths to the same end state: **Path A** (the daemon migrates for you on restart once `evm_key_name` is set in `config.yml`) or **Path B** (you migrate via Portal/Keplr or the shell helpers first, then the daemon detects the on-chain record and just performs local cleanup). Covers the multisig refusal behavior — the daemon won't drive a K-of-N ceremony and points you to the offline `lumerad` CLI flow — plus troubleshooting for address-mismatch and proto-skew errors.

### [relayer-migration.md](relayer-migration.md) — Hermes IBC Relayer Migration

Migrating a Hermes relayer's Lumera signing account from legacy `secp256k1` to `eth_secp256k1`. The relayer is the one account where a **second tool** (Hermes) must re-derive the same key from the mnemonic, so the guide centers on the HD-path gotcha: Hermes' default `ethermint` derivation does *not* match lumerad's `m/44'/60'/0'/0/0`, so you must set `address_type = { derivation = 'ethermint', proto_type = { pk_type = '/cosmos.evm.crypto.v1.ethsecp256k1.PubKey' } }` and pass `--hd-path "m/44'/60'/0'/0/0"` to `hermes keys add`. Includes a mandatory derived-address gate before the irreversible migration, and the stop → migrate → re-key → restart sequence.

### [node-evm-config-guide.md](node-evm-config-guide.md) — Node Operator EVM Config

Every EVM-relevant `app.toml` section explained: `[evm]`, `[evm.mempool]`, `[json-rpc]`, `[lumera.json-rpc-ratelimit]`, `[lumera.evm-mempool]`. Documents the chain-id namespace policy (mainnet rejects `debug`, `personal`, `admin`), the v1.20.0+ automatic config migration helper that fills in missing sections on first start, deployment-pattern checklists for validator / public-RPC / archive nodes, and the Prometheus metrics endpoints (`127.0.0.1:6065` for RPC, `127.0.0.1:8100` for the geth engine).

### [metamask-configuration.md](metamask-configuration.md) — MetaMask Configuration

End-user and operator guide for connecting MetaMask to Lumera EVM networks. Covers the public devnet MetaMask values, a full screenshot walkthrough of connecting the Lumera Portal to MetaMask (the `eth_chainId` → `wallet_switchEthereumChain` → `wallet_addEthereumChain` → `eth_requestAccounts` handshake, the `0x…`/`lumera1…` address mapping, importing a migrated EVM account, and verifying on the EVM Migration page), validator-local EVM ports, HTTPS reverse proxy setup, path-based proxy fallback, WSS proxying for dapps and tools, and `eth_chainId` troubleshooting.

### [tune-guide.md](tune-guide.md) — EVM Parameter Tuning

Mainnet-readiness review of every parameter that affects fees, throughput, UX, or economic security — base fee, min gas price, base-fee change denominator, block gas limit, mempool slots, JSON-RPC operational caps, rate limits, consensus timing, ERC20 registration policy, and migration parameters. Each parameter is benchmarked against Evmos / Kava / Cronos / Canto / Sei. Use this when preparing governance proposals or sizing a public-RPC fleet.

## Cross-cutting facts worth knowing before you start

- **Coin type 118 → 60 is the source of all migration friction.** The chain switched from Cosmos `secp256k1` (BIP44 path `m/44'/118'/...`) to Ethereum `eth_secp256k1` (path `m/44'/60'/...`) at the EVM upgrade. The same mnemonic now derives a *different* Lumera address. Migration moves your on-chain state from the old address to the new one in a single atomic transaction; the message itself carries dual proofs (ADR-036 over the legacy key, EIP-191 `personal_sign` over the new key) and is fee-free.
- **Validators must use `MsgMigrateValidator`, not `MsgClaimLegacyAccount`.** The chain explicitly rejects `claim-legacy-account` for validator operator addresses. `MsgMigrateValidator` is a superset that re-keys delegations, distribution state, and any bound supernode atomically.
- **Multisig migrations always mirror.** A K-of-N legacy multisig migrates to a K-of-N `eth_secp256k1` multisig — same K, same N. This is a consensus invariant, enforced at `ValidateBasic` (`ErrMirrorSourceMismatch`, code 1121). The destination can do all Cosmos-side ops (staking, IBC, supernode, authz) but cannot originate `MsgEthereumTx`; configure a separate single-EOA `MsgSetWithdrawAddress` if you need EVM DeFi access.
- **Migration is irreversible.** The legacy account is removed from the auth module post-migration; balances become 0 at the legacy address; the migration record (legacy → new mapping) is permanent.
- **After migrating, both Keplr and MetaMask work for the same account.** The migrated `eth_secp256k1` key has two representations — a Cosmos bech32 (`lumera1...`, shown by Keplr) and an Ethereum hex (`0x...`, shown by MetaMask) — that point to the same funds and state through different endpoints (Cosmos LCD vs. EVM JSON-RPC). Use MetaMask for EVM/DeFi, Keplr for Cosmos-native ops. Keplr requires switching to the EVM chain profile (coin-type 60) and re-importing first. See [migration.md § Using both Keplr and MetaMask after migration](migration.md#using-both-keplr-and-metamask-after-migration).
