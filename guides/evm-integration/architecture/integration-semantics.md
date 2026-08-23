# Detailed Integration Semantics

This section explains the key behavioral changes and why they matter operationally.

### 1) Added modules and what each one does

#### `x/vm` (EVM execution layer)

What it does:

- Executes Ethereum transactions and EVM bytecode.
- Owns EVM params/config (chain id, coin info, precompile activation).
- Exposes EVM-facing query/state paths used by JSON-RPC.

Why it matters:

- This is the core execution engine that enables Solidity/Vyper contract runtime compatibility.
- It establishes EVM-native semantics for nonce, gas accounting, receipt/log generation, and tx hashing.

#### `x/erc20` (STRv2 representation layer)

What it does:

- Implements Single Token Representation v2 (STRv2) behavior.
- Exposes ERC-20-compatible interfaces over canonical Cosmos token state.
- Maintains denom/token-pair registrations and ERC-20 allowances/mappings.
- Works with IBC middleware to register token pairs for incoming ICS20 denoms.

Why it matters:

- EVM dApps can use ERC-20-style APIs without forcing a second canonical supply model.
- Reduces liquidity/supply fragmentation compared to ad-hoc wrapped-token patterns.

#### `x/feemarket` (EIP-1559 fee layer)

What it does:

- Maintains dynamic base fee and fee-related block accounting.
- Supports type-2 fee model (`maxFeePerGas`,`maxPriorityFeePerGas`).
- Provides fee endpoints used by wallets/clients (`eth_feeHistory`, gas price hints, etc.).

Why it matters:

- Lumera gets Ethereum-style fee behavior with dynamic pricing under congestion.
- Priority tips become explicit inclusion incentives and influence tx ordering.

#### `x/precisebank` (18-decimal accounting bridge)

What it does:

- Bridges Cosmos 6-decimal bank representation to EVM 18-decimal representation.
- Tracks fractional remainder state that does not fit into 6-decimal integer bank units.
- Preserves canonical bank compatibility while exposing EVM-friendly precision.

Why it matters:

- EVM tooling expects wei-like precision (18 decimals).
- This lets Lumera keep`ulume` semantics in Cosmos while exposing`alume` precision to EVM.

### 2) Coin type change (`118 -> 60`) and HD derivation consequences

What changed:

- Default derivation path moved from Cosmos-style branch (`m/44'/118'/...`) to Ethereum-style branch (`m/44'/60'/...`).

Important consequence:

- Same mnemonic now derives a different private key/address branch by default.
- Cryptography is unchanged; key selection subtree changed.

Operational impact:

- Existing users importing old mnemonics into new default wallets may see different addresses.
- On-chain balances are keyed by address bytes, not mnemonic; old funds remain on old addresses.
- CLI/faucet/test scripts that derive keys by default will produce different addresses than before.

Common rollout strategies:

- Default-to-60 with user-driven migration (old accounts remain valid; users transfer funds).
- Association/claim flow (chain-assisted mapping or migration with ownership proof).
- Keep-118 canonical (lower migration risk, lower EVM wallet/tool plug-and-play).

### 3) `eth_secp256k1` key type and what it changes

What changed:

- Keyring defaults and CLI defaults now use`eth_secp256k1`.

What this affects:

- Address derivation semantics align with Ethereum expectations.
- EVM transaction signing/recovery and wallet interoperability are improved.

Address derivation distinction:

- Cosmos-style addresses are derived from a Cosmos hash pipeline over pubkey bytes.
- Ethereum-style addresses are derived as the last 20 bytes of Keccak256 over the uncompressed public key (without prefix).
- These are different derivation functions, so outputs differ even for the same key material.
- This is why legacy Cosmos-derived and new EVM-derived accounts can coexist and point to different on-chain entries.

### 4) Dual-address model (Cosmos Bech32 + EVM `0x`)

How it works:

- Cosmos-facing messages/CLI still use Bech32 (`lumera1...`).
- EVM JSON-RPC/wallets use`0x...` hex addresses.
- For EVM-derived accounts, both are representations of the same underlying 20-byte address bytes.

Why it matters:

- Cosmos SDK workflows and EVM wallet workflows can coexist without changing user-facing APIs on either side.
- Indexers/explorers/wallet UIs need to display both forms where appropriate.

### 5) Gas token decimals `6 -> 18` view (`ulume` + `alume`)

What changed:

- Cosmos base denom remains`ulume` (6 decimals).
- EVM extended denom is`alume` (18 decimals).
- Conversion factor is`10^12`:`1 ulume = 10^12 alume`.

Precisebank arithmetic model:

- Let`I(a)` be integer bank balance in`ulume` units for account`a`.
- Let`F(a)` be precisebank fractional remainder in`[0, 10^12)`.
- EVM-view total for account`a` (in`alume`) is:
  - `EVMBalance(a) = I(a) * 10^12 + F(a)`

Why it matters:

- EVM fee/value transfers can operate at 18-decimal granularity.
- Cosmos bank invariants and integrations continue to operate with 6-decimal canonical storage.

### 6) EIP-1559 in Lumera (`x/feemarket`)

What changed:

- Dynamic base fee is enabled by default (`NoBaseFee=false`) with Lumera defaults.
- Type-2 transaction fee fields are supported and enforced.
- Minimum gas price floor (`MinGasPrice=0.0005 ulume/gas`) prevents the base fee from decaying to zero on low-activity chains. Without this floor, empty blocks cause the EIP-1559 algorithm to reduce the base fee by ~6.25% per block until it reaches zero, effectively disabling all fee enforcement.
- Base fee change denominator is set to`16` (upstream default is`8`), producing gentler ~6.25% adjustments per block instead of ~12.5%. This reduces fee volatility and slows decay during low-activity periods.

Behavioral consequences:

- Base fee adapts block-to-block with gas usage.
- Effective gas price is bounded by fee cap and includes priority tip behavior.
- Transactions are prioritized by fee competitiveness (including tip), plus nonce constraints per sender.
- The base fee cannot drop below`0.0005 ulume/gas` (0.5 gwei equivalent), ensuring a minimum cost for all transactions even during sustained low activity.

Current fee-routing behavior:

- Lumera currently uses standard SDK fee collection for EVM transactions.
- The EVM keeper computes and deducts the full effective gas price (`base fee + effective priority tip`) up front and sends it to the normal fee collector module account.
- Unused gas is refunded from the fee collector back to the sender after execution.
- The remaining collected fees are then distributed by`x/distribution` using the normal SDK path:
  - fees move from the fee collector to the distribution module account,
  - community tax is applied,
  - the remainder is allocated across validators by voting power / stake fraction,
  - each validator share is then split into validator commission and delegator rewards.
- There is currently no custom Lumera path that isolates the EVM base-fee component from the tip component.
- There is currently no burn path for EVM base fees.

Why it matters:

- Wallet fee estimation and transaction inclusion behavior now match common Ethereum user expectations.
- The minimum gas price floor prevents zero-fee transaction spam that would otherwise be possible when the base fee decays to zero on quiet chains.

### 7) Priority tips and tx prioritization

What changed:

- Fee competitiveness now includes explicit priority-tip bidding in EVM tx paths.
- App-side EVM mempool behavior supports Ethereum-like nonce and replacement semantics.

Behavioral consequences:

- Higher-fee/higher-tip transactions are generally preferred under contention.
- Same-nonce replacement follows bump rules instead of arbitrary replacement.
- Nonce-gap handling and promotion behavior are explicit and test-covered.

### 8) Token representation inside EVM (bank <-> ERC-20, STRv2)

What changed:

- Lumera integrates STRv2-style`x/erc20` representation with canonical bank-backed supply.
- ERC-20 interfaces map to Cosmos denoms/token pairs rather than introducing uncontrolled parallel supply semantics.

Behavioral consequences:

- EVM contracts and wallets see ERC-20 interfaces where mappings exist.
- Underlying canonical accounting remains rooted in bank/precisebank state.
- Allowances and mapping state live in ERC20 module state, while balances reconcile with bank/precisebank storage model.

### 9) IBC transfer v2 / STRv2 interplay

What changed:

- IBC transfer stack includes ERC20 middleware for v1 and v2 paths.
- Incoming IBC assets can be registered into ERC20 mapping paths automatically (when enabled).

Why it matters:

- Cross-chain assets can become EVM-usable through registration/mapping flows.
- This reduces manual post-transfer token onboarding friction for EVM-side apps.

### 10) Migration consequences and rollout guidance

Main breakpoints to communicate:

- Default wallet derivation branch change (`118 -> 60`) changes default derived addresses.
- New default key algorithm (`eth_secp256k1`) changes account creation/import expectations.
- Fee behavior is now EIP-1559-like for EVM tx flows.

Recommended rollout checklist:

- Publish migration guidance for legacy mnemonic users (old vs new derived address visibility).
- Ensure explorers/indexers/wallet docs show dual address forms.
- Verify exchange/custody integrations handle 18-decimal EVM view and fee-market fields.
- Validate denom/token mapping expectations for ERC20/IBC-facing integrations.

