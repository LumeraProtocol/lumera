# EVM Parameter Tuning Guide — Mainnet Readiness Review

> **Audience:** Chain operators, governance participants, and business stakeholders preparing the Lumera EVM integration for mainnet.
>
> **Scope:** Every tunable parameter that affects fees, throughput, user experience, or economic security. Parameters are grouped by business impact and compared against peer Cosmos-EVM chains (Evmos, Kava, Cronos, Canto, Sei).

---

## Table of Contents

1. [Fee Market (EIP-1559) Parameters](#1-fee-market-eip-1559-parameters)
2. [Block Gas Limit](#2-block-gas-limit)
3. [EVM Mempool Economics](#3-evm-mempool-economics)
4. [JSON-RPC Operational Limits](#4-json-rpc-operational-limits)
5. [Rate Limiting (Public RPC)](#5-rate-limiting-public-rpc)
6. [Consensus Timing](#6-consensus-timing)
7. [Precompile & Module Governance Parameters](#7-precompile--module-governance-parameters)
8. [ERC20 Registration Policy](#8-erc20-registration-policy)
9. [Migration Parameters](#9-migration-parameters)
10. [Quick Reference Summary Table](#10-quick-reference-summary-table)

---

## 1. Fee Market (EIP-1559) Parameters

These are the **highest-impact** parameters from a business perspective. They determine how much users pay for transactions and how the chain responds to congestion.

### 1.1 `base_fee` (Initial / Genesis Base Fee)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `0.0025 ulume/gas` (~2.5 gwei equivalent in 18-decimal EVM) |
| **Where set** | `config/evm.go` → `FeeMarketDefaultBaseFee`, baked into genesis via `app/evm/genesis.go` |
| **Governance changeable** | Yes (feemarket params proposal) |
| **Min** | Must be > 0 when `no_base_fee = false` |
| **Max** | No hard ceiling; practically limited by user willingness to pay |

**What it does:** The starting price per unit of gas. After genesis, EIP-1559 adjusts this automatically based on block utilization. This value only matters at chain start or after a governance reset.

**Peer comparison:**

| Chain | Base Fee | Notes |
|-------|----------|-------|
| **Lumera** | 0.0025 ulume/gas | Conservative starting point |
| **Evmos** | 1,000,000,000 aevmos/gas (1 gwei) | Lower start, relies on dynamic adjustment |
| **Kava** | 1,000,000,000 akava/gas (1 gwei) | Standard Ethereum-like |
| **Cronos** | 5,000 basecro/gas | Higher, reflecting CRO price |
| **Canto** | 1,000,000,000 acanto/gas | Standard |

**Tuning guidance:**
- Calculate the **target simple-transfer cost** in USD: `21,000 gas * base_fee * token_price`. At $0.01/LUME and 0.0025 ulume/gas, a transfer costs ~$0.000000525 — extremely cheap.
- If LUME price is low at launch, the current value is reasonable. If LUME launches at higher value, consider lowering.
- The base fee auto-adjusts, so this is mainly about first-block UX. Err on the low side — the market will push it up.

**Recommendation:** Review once token price is known. Current value likely fine for launch.

---

### 1.2 `min_gas_price` (Base Fee Floor)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `0.0005 ulume/gas` (20% of base_fee) |
| **Where set** | `config/evm.go` → `FeeMarketMinGasPrice` |
| **Governance changeable** | Yes |
| **Min** | `0` (but 0 allows free txs — dangerous) |
| **Max** | Must be < `base_fee` for EIP-1559 to function |

**What it does:** Prevents the base fee from decaying to zero during low-activity periods. This is the **absolute minimum** a user ever pays per gas unit. It is Lumera's primary anti-spam defense during quiet periods.

**Peer comparison:**

| Chain | Min Gas Price | Ratio to Base Fee |
|-------|---------------|-------------------|
| **Lumera** | 0.0005 ulume/gas | 20% of base fee |
| **Evmos** | 0 (relies on min-gas-prices in app.toml) | 0% — risky |
| **Kava** | 0.001 ukava/gas (via validator min) | ~100% of base fee |
| **Canto** | 0 (was exploited for spam) | 0% — learned the hard way |

**Tuning guidance:**
- **Never set to 0** — Canto's experience showed that zero-floor chains get spammed during quiet periods.
- The 20% ratio is healthy. It means even in sustained low activity, txs cost 1/5th of normal.
- Calculate minimum acceptable transfer cost: `21,000 * 0.0005 * price`. Ensure this is not literally free.

**Recommendation:** **Keep at 0.0005 or raise slightly.** This is well-designed. The 20% floor ratio is more conservative than most peers.

---

### 1.3 `base_fee_change_denominator`

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `16` (~6.25% adjustment per block) |
| **Upstream cosmos-evm default** | `8` (~12.5% adjustment per block) |
| **Ethereum mainnet** | `8` (~12.5%) |
| **Governance changeable** | Yes |
| **Min** | `1` (100% change per block — extremely volatile) |
| **Max** | No upper limit; higher = more stable but slower to respond |

**What it does:** Controls how fast the base fee reacts to congestion. The formula is:

```
fee_delta = parent_base_fee * (gas_used - gas_target) / gas_target / base_fee_change_denominator
```

Higher denominator = slower, smoother fee changes. Lower = faster, more volatile.

**Peer comparison:**

| Chain | Denominator | Max Change/Block | Philosophy |
|-------|-------------|-----------------|------------|
| **Lumera** | 16 | ~6.25% | Conservative / stable fees |
| **Ethereum** | 8 | ~12.5% | Battle-tested default |
| **Evmos** | 8 | ~12.5% | Standard |
| **Kava** | 8 | ~12.5% | Standard |
| **Cronos** | 8 | ~12.5% | Standard |

**Tuning guidance:**
- Lumera chose `16` (half the upstream rate). This means fees adjust **twice as slowly** to congestion spikes.
- **Pro:** Users see more predictable fees; less MEV from fee manipulation.
- **Con:** During sudden demand spikes (NFT mints, token launches), the chain takes longer to price out spam, potentially causing more failed txs and worse UX.
- With ~5s block times, it takes Lumera ~2x more blocks to reach the same fee level as Ethereum would under identical congestion.

**Recommendation:** **This deserves active discussion.** Consider `8` (standard) if you expect volatile demand patterns. Keep `16` if fee stability is a product priority. You can always change via governance post-launch.

---

### 1.4 `no_base_fee`

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `false` (EIP-1559 is **enabled**) |

**What it does:** Master switch for the dynamic fee market. When `true`, gas price is static (like pre-EIP-1559 Ethereum).

**Recommendation:** **Keep `false`.** EIP-1559 is industry standard for congestion pricing. Disabling it removes automatic spam protection.

---

## 2. Block Gas Limit

### 2.1 `consensus_max_gas` (Block Gas Limit)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `25,000,000` |
| **Where set** | `config/evm.go` → `ChainDefaultConsensusMaxGas`, applied during `lumerad init` |
| **Changeable** | Yes, via governance (consensus params update) |
| **Min** | ~500,000 (enough for a single simple tx) |
| **Max** | Hardware-limited; see guidance below |

**What it does:** The maximum total gas consumed by all transactions in a single block. This is the chain's **throughput ceiling**. The EIP-1559 gas target is implicitly half of this (12,500,000).

**Peer comparison:**

| Chain | Block Gas Limit | Block Time | Effective Gas/sec |
|-------|----------------|------------|-------------------|
| **Lumera** | 25,000,000 | ~5s | ~5M gas/s |
| **Ethereum** | 30,000,000 | 12s | ~2.5M gas/s |
| **Evmos** | 40,000,000 | ~2s | ~20M gas/s |
| **Kava** | 25,000,000 | ~6s | ~4.2M gas/s |
| **Cronos** | 25,000,000 | ~6s | ~4.2M gas/s |
| **Sei** | 100,000,000 | 0.4s | ~250M gas/s |

**Tuning guidance:**
- 25M is a safe, well-tested value used by Kava and Cronos. It accommodates most DeFi workloads (Uniswap V3 deploy ~5M gas, complex DeFi tx ~1-3M gas).
- **Increasing** the limit allows more txs/block but increases state growth, hardware requirements, and block propagation time. Only raise if validators have confirmed hardware capacity.
- **Decreasing** improves decentralization (lower hardware bar) but may cause congestion during demand spikes.
- With 25M limit and 5s blocks, Lumera can process ~1,190 simple transfers/block or ~8-25 complex DeFi txs/block.

**Recommendation:** **25M is appropriate for launch.** Monitor block utilization post-launch; if average utilization exceeds 50% (12.5M), consider raising to 40M via governance.

---

## 3. EVM Mempool Economics

### 3.1 `min-tip` (Minimum Priority Fee)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `0` wei |
| **Where set** | `app.toml` → `[evm] min-tip` |
| **Changeable** | Yes, per-node (app.toml) |
| **Min** | `0` |
| **Max** | No hard ceiling |

**What it does:** Minimum priority fee (tip) an EVM transaction must include to enter the local mempool. This is a **per-node** setting, not consensus.

**Tuning guidance:**
- At `0`, any tx with `maxPriorityFeePerGas >= 0` is accepted. This is fine for launch.
- Validators wanting to earn tips can set this higher, but it's a competitive market — set too high and you miss txs.
- Unlike `min_gas_price`, this does NOT protect against spam (spam txs can set tip=0 and still pass).

**Recommendation:** **Keep at 0 for launch.** Let the market develop before adding mandatory tips.

---

### 3.2 `price-bump` (Replacement Tx Fee Bump)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `10` (10% minimum bump) |
| **Ethereum default** | `10` (10%) |
| **Where set** | `app.toml` → `[evm.mempool] price-bump` |

**What it does:** When a user submits a replacement transaction (same nonce), the new tx must offer at least `price-bump`% higher gas price. Prevents mempool churn from marginal fee increases.

**Recommendation:** **Keep at 10%.** Industry standard. No reason to change.

---

### 3.3 `global-slots` / `account-slots` (Mempool Capacity)

| Parameter | Lumera | Ethereum (geth) | Purpose |
|-----------|--------|-----------------|---------|
| `account-slots` | 16 | 16 | Executable tx slots per account |
| `global-slots` | 5,120 | 5,120 | Total executable slots |
| `account-queue` | 64 | 64 | Non-executable queue per account |
| `global-queue` | 1,024 | 1,024 | Total non-executable queue |
| `lifetime` | 3h | 3h | Queue eviction timeout |

**What they do:** Control mempool size and per-account fairness. These are direct copies of geth defaults.

**Tuning guidance:**
- These defaults work well for Ethereum's ~15 TPS. Lumera has similar throughput (~5M gas/s vs Ethereum's ~2.5M gas/s).
- If Lumera attracts high-frequency traders or bots, consider **reducing** `account-slots` to 8 to limit per-account mempool dominance.
- If the chain is very active, `global-slots` may need increasing to 10,240.

**Recommendation:** **Keep defaults for launch.** Monitor mempool fullness metrics post-launch.

---

### 3.4 `price-limit` (Minimum Gas Price in Mempool)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `1` wei |
| **Ethereum default** | `1` wei |

**What it does:** Absolute minimum gas price for mempool acceptance. With 18-decimal EVM pricing, 1 wei is effectively zero.

**Tuning guidance:** This is effectively overridden by `min_gas_price` at the consensus level. The mempool `price-limit` only catches truly malformed txs.

**Recommendation:** **Keep at 1.** The real floor is `min_gas_price`.

---

## 4. JSON-RPC Operational Limits

These parameters affect **RPC node operators** and **dApp developers**, not end-user fees.

### 4.1 `gas-cap` (eth_call / eth_estimateGas Limit)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `25,000,000` (matches block gas limit) |
| **Where set** | `app.toml` → `[json-rpc] gas-cap` |
| **Min** | `0` (unlimited — dangerous for public nodes) |
| **Max** | No ceiling, but higher = more DoS surface |

**What it does:** Maximum gas allowed for read-only `eth_call` and `eth_estimateGas` queries. Prevents a single query from consuming all node resources.

**Peer comparison:**

| Chain | gas-cap | Notes |
|-------|---------|-------|
| **Lumera** | 25,000,000 | Matches block limit |
| **Evmos** | 25,000,000 | Standard |
| **Kava** | 25,000,000 | Standard |

**Recommendation for public RPC nodes:** **Lower to 10,000,000.** Most legitimate `eth_call` queries use <5M gas. Public nodes should be more restrictive to prevent resource abuse. Validators can keep 25M.

---

### 4.2 `evm-timeout` (Query Timeout)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `5s` |
| **Public RPC recommended** | `3s` |
| **Archive/debug recommended** | `30s` |

**What it does:** Maximum wall-clock time for `eth_call` and `eth_estimateGas`. Kills runaway queries.

**Recommendation:** **5s is fine for validators. Lower to 3s for public RPCs.**

---

### 4.3 `logs-cap` / `block-range-cap` (Log Query Limits)

| Parameter | Lumera Default | Public RPC Recommended |
|-----------|---------------|----------------------|
| `logs-cap` | 10,000 | 2,000 |
| `block-range-cap` | 10,000 | 2,000 |

**What they do:** Limit the size of `eth_getLogs` responses. Large log queries are the #1 DoS vector for EVM RPC nodes.

**Recommendation:** **Lower both to 2,000 for public-facing nodes.** Keep 10,000 for internal/archive nodes.

---

### 4.4 `batch-request-limit` / `batch-response-max-size`

| Parameter | Lumera Default | Ethereum (geth) |
|-----------|---------------|-----------------|
| `batch-request-limit` | 1,000 | 1,000 |
| `batch-response-max-size` | 25,000,000 (25 MB) | 25,000,000 |

**Recommendation:** **Lower `batch-request-limit` to 50-100 for public RPCs.** Batch calls are a common amplification vector.

---

### 4.5 `txfee-cap` (Send Transaction Fee Cap)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `1` (in ETH-equivalent units, i.e., 1 LUME) |

**What it does:** Safety net preventing `eth_sendTransaction` from accidentally spending more than this in fees. Only relevant when the node holds keys (not common in production).

**Recommendation:** **Keep at 1.** This is a client-side safety net, not a consensus parameter.

---

### 4.6 `allow-unprotected-txs`

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `false` |

**What it does:** When `false`, rejects transactions without EIP-155 replay protection (no chain ID). Prevents replay attacks from other EVM chains.

**Recommendation:** **MUST remain `false` for mainnet.** Setting to `true` is a security vulnerability.

---

### 4.7 `max-open-connections`

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `0` (unlimited) |

**What it does:** Limits concurrent JSON-RPC connections.

**Recommendation:** **Set to 200 for public RPC nodes.** Unlimited connections on a public endpoint is a DoS risk.

---

## 5. Rate Limiting (Public RPC)

### 5.1 Rate Limiter Configuration

| Parameter | Default | Recommended (Public) | Purpose |
|-----------|---------|---------------------|---------|
| `enable` | `false` | `true` | Master switch |
| `requests-per-second` | `50` | `20-50` | Sustained rate per IP |
| `burst` | `100` | `50-100` | Token bucket burst |
| `entry-ttl` | `5m` | `5m` | Per-IP state lifetime |
| `proxy-address` | `0.0.0.0:8547` | Match deployment | Proxy listen address |

**Tuning guidance:**
- **50 rps** is generous — most dApps need 5-10 rps. Reduce to 20 for public endpoints if abuse is a concern.
- **Burst 100** allows wallets to do initial state sync (batch of ~50-80 calls on page load).
- **MUST be enabled** for any internet-facing RPC node.

**Recommendation:** **Enable for all public RPC nodes.** Start with `rps=30, burst=60` and adjust based on monitoring.

---

## 6. Consensus Timing

### 6.1 `timeout_commit` (Block Time)

| Attribute | Value |
|-----------|-------|
| **Lumera default** | `5s` |
| **Where set** | `config.toml` → `[consensus] timeout_commit` |
| **Min** | ~1s (network latency limited) |
| **Max** | No ceiling, but longer = worse UX |

**Peer comparison:**

| Chain | Block Time | EVM Finality |
|-------|------------|-------------|
| **Lumera** | ~5s | ~5s (single-slot) |
| **Ethereum** | 12s | ~13 min (32 slots) |
| **Evmos** | ~2s | ~2s |
| **Kava** | ~6s | ~6s |
| **Cronos** | ~6s | ~6s |
| **Sei** | ~0.4s | ~0.4s |

**Tuning guidance:**
- 5s is moderate. Faster block times improve UX but increase state growth and network bandwidth.
- Lumera already has single-slot finality (CometBFT), so 5s is the **actual** finality time — much better than Ethereum's 13 minutes.
- Reducing to 3s would improve EVM UX (faster tx confirmation) but requires validator consensus and may stress lower-end hardware.

**Recommendation:** **5s is reasonable for launch.** Consider reducing to 3s post-launch if validators can handle it.

---

### 6.2 `max_tx_bytes` (Max Transaction Size)

| Attribute | Value |
|-----------|-------|
| **CometBFT default** | `1,048,576` (1 MB) |

**What it does:** Maximum size of a single transaction in bytes. Affects large contract deployments.

**Tuning guidance:**
- 1 MB accommodates most smart contracts. The largest known production contracts (Uniswap V3) are ~24 KB bytecode.
- Only increase if Lumera expects very large CosmWasm or EVM contracts.

**Recommendation:** **Keep at 1 MB.**

---

## 7. Precompile & Module Governance Parameters

### 7.1 Action Module Parameters (`x/action`)

| Parameter | Type | Business Impact |
|-----------|------|-----------------|
| `base_action_fee` | uint256 (ulume) | Cost to submit any action — revenue for chain |
| `fee_per_kbyte` | uint256 (ulume) | Per-KB fee component for data-heavy actions |
| `max_actions_per_block` | uint64 | Rate limit — affects supernode throughput |
| `min_super_nodes` | uint64 | Security threshold for action processing |
| `supernode_fee_share` | decimal | Revenue split to supernodes (incentive alignment) |
| `foundation_fee_share` | decimal | Revenue split to foundation |

**Tuning guidance:**
- `base_action_fee` + `fee_per_kbyte` determine the **cost of Cascade/Sense actions**. These should be competitive with centralized alternatives while covering supernode compute costs.
- `supernode_fee_share` + `foundation_fee_share` must sum to ≤ 1.0. Higher supernode share incentivizes more supernodes; higher foundation share funds development.
- `max_actions_per_block` should match expected demand. Too low = queuing delays; too high = block time bloat.

**Recommendation:** **Requires economic modeling based on expected action volume and supernode operating costs.**

---

### 7.2 Supernode Module Parameters (`x/supernode`)

| Parameter | Business Impact |
|-----------|-----------------|
| `minimum_stake` | Barrier to entry for supernodes — too high limits supply, too low degrades quality |
| `slashing_threshold` | Punishment sensitivity — too aggressive drives supernodes away |
| `min_supernode_version` | Upgrade enforcement — forces network-wide updates |
| `min_cpu_cores` / `min_mem_gb` / `min_storage_gb` | Hardware floor — affects cost to run a supernode |

**Recommendation:** **Review minimum_stake relative to LUME price at launch.** A stake that costs $10K at $0.01/LUME costs $100K at $0.10/LUME.

---

## 8. ERC20 Registration Policy

| Attribute | Value |
|-----------|-------|
| **Lumera default** | Configurable: `"all"`, `"allowlist"`, or `"none"` |
| **Default allowed base denoms** | `uatom`, `uosmo`, `uusdc` |
| **Governance changeable** | Yes (via `MsgSetRegistrationPolicy`) |

**What it does:** Controls which IBC tokens automatically get ERC20 representations. In `"all"` mode, any IBC token that arrives gets an ERC20 contract deployed. In `"allowlist"` mode, only pre-approved tokens do.

**Tuning guidance:**
- `"all"` is convenient but creates unbounded ERC20 contracts (state bloat, audit surface).
- `"allowlist"` is safer — only vetted tokens get ERC20 pairs.
- For mainnet launch, start with `"allowlist"` and a curated list of trusted IBC tokens.

**Recommendation:** **Use `"allowlist"` for mainnet launch.** Expand the list via governance as IBC partnerships are established.

---

## 9. Migration Parameters (`x/evmigration`)

| Parameter | Default | Review Needed? |
|-----------|---------|---------------|
| `enable_migration` | `true` | Yes — disable after migration window closes |
| `migration_end_time` | `0` (no deadline) | **Yes — set a deadline for mainnet** |
| `max_migrations_per_block` | `50` | Review based on expected migration volume |
| `max_validator_delegations` | `2,000` | Review based on largest validator delegation count |

**Recommendation:** **Set `migration_end_time` to a specific date before mainnet.** Open-ended migration windows are a governance and security risk. Consider 30-90 days post-launch.

---

## 10. Quick Reference Summary Table

Priority levels: **CRITICAL** = must review before mainnet, **HIGH** = should review, **MEDIUM** = review if time permits, **LOW** = safe defaults.

| Priority | Parameter | Current Value | Action |
|----------|-----------|---------------|--------|
| **CRITICAL** | `base_fee` | 0.0025 ulume/gas | Re-validate against launch token price |
| **CRITICAL** | `min_gas_price` | 0.0005 ulume/gas | Ensure non-zero cost at launch price |
| **CRITICAL** | `allow-unprotected-txs` | `false` | Verify remains `false` in all configs |
| **CRITICAL** | `migration_end_time` | `0` (none) | **Set a mainnet deadline** |
| **CRITICAL** | `minimum_stake` (supernode) | TBD | Price-sensitive — review at launch price |
| **HIGH** | `base_fee_change_denominator` | 16 | Decide: stability (16) vs responsiveness (8) |
| **HIGH** | `consensus_max_gas` | 25,000,000 | Confirm validator hardware supports it |
| **HIGH** | ERC20 registration policy | configurable | **Set to "allowlist" for mainnet** |
| **HIGH** | `base_action_fee` / `fee_per_kbyte` | TBD | Economic modeling needed |
| **HIGH** | `supernode_fee_share` | TBD | Incentive alignment review |
| **HIGH** | Rate limiter | `disabled` | **Enable on public RPC nodes** |
| **MEDIUM** | `gas-cap` (JSON-RPC) | 25,000,000 | Lower to 10M for public nodes |
| **MEDIUM** | `logs-cap` / `block-range-cap` | 10,000 | Lower to 2,000 for public nodes |
| **MEDIUM** | `batch-request-limit` | 1,000 | Lower to 50-100 for public nodes |
| **MEDIUM** | `max-open-connections` | 0 (unlimited) | Set to 200 for public nodes |
| **MEDIUM** | `timeout_commit` | 5s | Consider 3s if validators can handle it |
| **LOW** | `price-bump` | 10% | Industry standard, no change needed |
| **LOW** | Mempool slots | geth defaults | Monitor post-launch |
| **LOW** | `no_base_fee` | `false` | Keep enabled |
| **LOW** | `txfee-cap` | 1 LUME | Client-side safety, keep as-is |

---

## Appendix A: Fee Calculation Examples

For business stakeholders, here is what users actually pay at various token prices:

### Simple EVM Transfer (21,000 gas)

| LUME Price | Base Fee (ulume/gas) | Cost (ulume) | Cost (USD) |
|------------|---------------------|--------------|------------|
| $0.001 | 0.0025 | 52.5 | $0.0000000525 |
| $0.01 | 0.0025 | 52.5 | $0.000000525 |
| $0.10 | 0.0025 | 52.5 | $0.00000525 |
| $1.00 | 0.0025 | 52.5 | $0.0000525 |
| $10.00 | 0.0025 | 52.5 | $0.000525 |

### Complex DeFi Transaction (500,000 gas)

| LUME Price | Base Fee (ulume/gas) | Cost (ulume) | Cost (USD) |
|------------|---------------------|--------------|------------|
| $0.001 | 0.0025 | 1,250 | $0.00000125 |
| $0.01 | 0.0025 | 1,250 | $0.0000125 |
| $0.10 | 0.0025 | 1,250 | $0.000125 |
| $1.00 | 0.0025 | 1,250 | $0.00125 |
| $10.00 | 0.0025 | 1,250 | $0.0125 |

### Smart Contract Deployment (3,000,000 gas)

| LUME Price | Base Fee (ulume/gas) | Cost (ulume) | Cost (USD) |
|------------|---------------------|--------------|------------|
| $0.001 | 0.0025 | 7,500 | $0.0000075 |
| $0.01 | 0.0025 | 7,500 | $0.000075 |
| $0.10 | 0.0025 | 7,500 | $0.00075 |
| $1.00 | 0.0025 | 7,500 | $0.0075 |
| $10.00 | 0.0025 | 7,500 | $0.075 |

> **Note:** These are base-fee-only costs. Actual costs include priority tips (usually small) and may be higher during congestion (base fee rises).

---

## Appendix B: Fee Comparison With Competitor Chains

| Metric | Lumera | Ethereum | Evmos | Kava | Cronos |
|--------|--------|----------|-------|------|--------|
| Simple transfer cost | ~$0.000001* | $0.50-5.00 | ~$0.001 | ~$0.001 | ~$0.01 |
| Block time | 5s | 12s | 2s | 6s | 6s |
| Finality | ~5s | ~13 min | ~2s | ~6s | ~6s |
| Block gas limit | 25M | 30M | 40M | 25M | 25M |
| Fee adjustment speed | 6.25%/block | 12.5%/block | 12.5%/block | 12.5%/block | 12.5%/block |
| Min gas price floor | Yes (0.0005) | No | No | Yes | Yes |

*At $0.01/LUME. Actual cost depends on token price.
