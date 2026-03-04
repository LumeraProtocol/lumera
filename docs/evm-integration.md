# Lumera Cosmos EVM Integration

## Summary

Lumera now has first-class Cosmos EVM integration across runtime wiring, ante, mempool, JSON-RPC/indexer, key management, static precompiles, IBC ERC20 middleware, denom metadata, and upgrade/migration paths.

This document groups:

- App/runtime changes and what they enable.
- Unit tests by subgroup.
- Integration tests by subgroup.

## App Changes and Features

### 1) Chain config, denoms, addresses, and HD path

Files:

- `config/config.go`
- `config/bech32.go`
- `config/bip44.go`
- `config/evm.go`
- `config/bank_metadata.go`
- `config/codec.go`

Changes:

- Added canonical chain token constants:
  - `ChainDenom = "ulume"`
  - `ChainDisplayDenom = "lume"`
  - `ChainEVMExtendedDenom = "alume"`
  - `ChainTokenName = "Lumera"`
  - `ChainTokenSymbol = "LUME"`
- Added explicit Bech32 constants and helper `SetBech32Prefixes`.
- Added `SetBip44CoinType` to set BIP44 purpose 44 and coin type 60 (Ethereum).
- Added EVM constants:
  - `EVMChainID = 76857769`
  - `FeeMarketDefaultBaseFee = "0.0025"`
  - `FeeMarketMinGasPrice = "0.0005"` (floor preventing base fee decay to zero)
  - `FeeMarketBaseFeeChangeDenominator = 16` (gentler ~6.25% adjustment per block)
  - `ChainDefaultConsensusMaxGas = 10_000_000`
- Centralized bank denom metadata via`ChainBankMetadata`/`UpsertChainBankMetadata`.
- Added `RegisterExtraInterfaces` to register Cosmos crypto + EVM crypto interfaces (including `eth_secp256k1`).

Benefits/new features:

- Ethereum-compatible key derivation and wallet UX.
- Consistent denom metadata for SDK + EVM paths.
- Stable chain-wide EVM chain-id/base-fee/min-gas-price/max-gas defaults.

### 2) EVM module wiring (keepers, stores, genesis, depinject)

Files:

- `app/app.go`
- `app/evm.go`
- `app/evm/config.go`
- `app/evm/genesis.go`
- `app/evm/modules.go`
- `app/app_config.go`

Changes:

- Registered EVM stores/keepers/modules:
  - `x/vm`,`x/feemarket`,`x/precisebank`,`x/erc20`.
- Added Lumera EVM genesis overrides:
  - EVM denom and extended denom.
  - Active static precompile list.
  - Feemarket defaults with dynamic base fee enabled, minimum gas price floor (`0.0005 ulume/gas`), and gentler base fee change denominator (`16`).
- Added depinject signer wiring for `MsgEthereumTx` via `ProvideCustomGetSigners`.
- Added depinject interface registration invoke (`RegisterExtraInterfaces`).
- Added default keeper coin info initialization (`SetKeeperDefaults`) for safe early RPC behavior.
- Added EVM module order/account permissions into genesis/begin/end/pre-block scheduling and module account perms.

Benefits/new features:

- Full EVM module stack is bootstrapped in app runtime.
- Correct signer derivation for Ethereum tx messages.
- Lumera-specific EVM genesis defaults are applied by default.

### 3) Ante handler: dual routing and EVM decorators

Files:

- `app/evm/ante.go`
- `app/app.go`

Changes:

- Replaced single-path ante with dual routing:
  - Ethereum extension tx -> EVM ante chain.
  - Cosmos tx + DynamicFee extension -> Cosmos ante path.
- EVM path uses `NewEVMMonoDecorator` + pending tx listener decorator.
- Cosmos path includes:
  - Lumera decorators (delayed claim fee, wasm, circuit breaker).
  - Cosmos EVM decorators (reject MsgEthereumTx in Cosmos path, authz limiter, min gas price, dynamic fee checker, gas wanted decorator).

Benefits/new features:

- Correct Ethereum tx validation/nonce/fee semantics.
- Cosmos and EVM txs coexist safely with explicit route separation.
- Pending tx notifications can be emitted for JSON-RPC pending subscriptions.

### 3a) How Ethereum txs appear on-chain and execute

Files:

- `app/evm/ante.go`
- `app/evm_broadcast.go`
- `app/evm_mempool.go`

Changes / execution model:

- Ethereum transactions are represented on-chain as `MsgEthereumTx` messages carried inside normal Cosmos SDK transactions.
- They are not executed in a separate consensus system or a separate block stream.
- Cosmos txs and Ethereum txs share:
  - the same blocks,
  - the same final transaction ordering inside a block,
  - the same proposer / consensus process,
  - the same committed state root progression.
- This means execution order is shared and consensus-relevant across both transaction families. Ordering therefore matters equally for:
  - balance changes,
  - nonce consumption,
  - state dependencies between transactions,
  - same-block arbitrage / MEV-sensitive behavior.

Different execution paths:

- Even though they share block ordering and consensus, Cosmos and Ethereum transactions do not use the same ante / execution pipeline.
- Ethereum txs take the EVM-specific route and are validated/executed with Ethereum-style semantics for signature recovery, fee caps, priority tips, nonce checks, gas accounting, receipt/log generation, and EVM state transition.
- Cosmos txs take the standard SDK route with Lumera/Cosmos decorators and normal SDK message execution.

Gas and fee accounting:

- Gas accounting is separate at execution-path level but reconciled at block level.
- Ethereum txs use EVM-style gas semantics internally, including intrinsic gas checks, execution gas consumption, and refund handling.
- Cosmos txs use standard SDK gas meter semantics.
- Both still contribute to the same block production process and to the chain's overall fee/distribution accounting.
- The fee market is unified at block level in the sense that EVM tx fees ultimately flow into the same chain-level fee collection and distribution path once execution is finalized.

Mempool and nonce behavior:

- Mempool behavior is intentionally different for Ethereum txs.
- Lumera wires an app-side EVM mempool to preserve Ethereum-like sender ordering, nonce-gap handling, and same-nonce replacement rules.
- Cosmos txs continue to follow standard SDK / CometBFT mempool behavior.
- Nonce systems are also different:
  - Ethereum txs use Ethereum account nonces with strict per-sender sequencing semantics.
  - Cosmos txs use SDK account sequence semantics.
- These systems coexist on the same chain, but each transaction family is validated according to its own rules before entering the shared block ordering.

Benefits/new features:

- Ethereum transactions are first-class citizens in Lumera without splitting consensus or block production into a separate subsystem.
- Mixed Cosmos/EVM blocks preserve deterministic ordering and shared state transitions.
- The chain can expose Ethereum-native UX and semantics while remaining a single Cosmos chain operationally.

### 4) App-side EVM mempool integration

Files:

- `app/evm_mempool.go`
- `app/evm_broadcast.go`
- `app/evm_runtime.go`
- `app/app.go`
- `cmd/lumera/cmd/config.go`

Changes:

- Wired Cosmos EVM experimental mempool into BaseApp:
  - `app.SetMempool(evmMempool)`
  - EVM-aware `CheckTx` handler
  - EVM-aware `PrepareProposal` signer extraction adapter
- Added async broadcast queue (`evmTxBroadcastDispatcher`) to decouple txpool promotion from CometBFT `CheckTx` submission, preventing a mutex re-entry deadlock (see Architecture Strengths below).
- Added `RegisterTxService` override in `app/evm_runtime.go` to capture the `client.Context` with the local CometBFT client that cosmos/evm creates after CometBFT starts — the default `SetClientCtx` call happens before CometBFT starts and only provides an HTTP client.
- Added `Close()` override to stop the broadcast worker before runtime shutdown.
- Added configurable `[lumera.evm-mempool]` section in `app.toml` with `broadcast-debug` toggle for detailed async broadcast logging.
- Enabled app-side mempool by default in app config (`max_txs=5000`).

Benefits/new features:

- Pending tx support and txpool behavior aligned with Cosmos EVM.
- Better Ethereum tx ordering/replacement/nonce-gap behavior.
- EVM-aware proposal building for mixed workloads.
- Deadlock-free nonce-gap promotion: promoted EVM txs are enqueued and broadcast by a single background worker, never blocking the mempool `Insert()` call stack.
- Debug logging for broadcast queue processing gated behind `app.toml` config flag.

### 5) JSON-RPC and indexer defaults

Files:

- `cmd/lumera/cmd/config.go`
- `cmd/lumera/cmd/commands.go`
- `cmd/lumera/cmd/root.go`

Changes:

- Enabled JSON-RPC and indexer by default in app config.
- Root command includes EVM server command wiring.
- Start command exposes JSON-RPC flags via cosmos/evm server integration.

Benefits/new features:

- Out-of-the-box `eth_*` RPC availability without manual config.
- Out-of-the-box receipt/tx-by-hash/indexer functionality.

### 6) Keyring and CLI defaults for Ethereum keys

Files:

- `cmd/lumera/cmd/root.go`
- `cmd/lumera/cmd/testnet.go`
- `testutil/accounts/accounts.go`
- `claiming_faucet/main.go`

Changes:

- Default CLI`--key-type` set to`eth_secp256k1`.
- Added `EthSecp256k1Option` to keyring initialization in CLI/testnet/helpers/faucet paths.
- Test/devnet account helpers aligned with EVM key algorithms.

Benefits/new features:

- `keys add/import` flows default to Ethereum-compatible key type.
- Reduced accidental creation of non-EVM keys for EVM users.

### 7) Static precompiles and blocked-address protections

Files:

- `app/evm/precompiles.go`
- `app/evm.go`
- `app/app.go`

Changes:

- Enabled static precompile set:
  - P256
  - Bech32
  - Staking
  - Distribution
  - ICS20
  - Bank
  - Gov
  - Slashing
- Explicitly excluded vesting precompile (not installed by upstream default registry in current version).
- Added blocked-address protections:
  - Module account block list.
  - Precompile-address send restriction in bank send restrictions.

Benefits/new features:

- Rich EVM-to-Cosmos precompile API surface enabled.
- Prevents accidental token sends to precompile addresses.

### 8) IBC + ERC20 middleware wiring

Files:

- `app/ibc.go`
- `app/evm.go`

Changes:

- Wired ERC20 keeper with transfer keeper pointer.
- Added ERC20 IBC middleware into transfer stack (v1 and v2).
- Wired EVM transfer keeper wrapping IBC transfer keeper.

Benefits/new features:

- ICS20 receive path can auto-register token pairs.
- Cross-chain ERC20/IBC integration path is now present.

### 9) Fee market and precisebank adoption

Files:

- `app/evm.go`
- `app/evm/genesis.go`
- `app/app_config.go`

Changes:

- Integrated `x/feemarket` and `x/precisebank` keepers/modules.
- Enabled dynamic base fee in default genesis with minimum gas price floor (`0.0005 ulume/gas`) and change denominator `16`.
- Added module ordering and permissions to include feemarket/precisebank correctly.

Benefits/new features:

- EIP-1559-style fee market behavior with spam protection via minimum gas price floor.
- 18-decimal extended-denom accounting bridged to bank module semantics.

### 10) Upgrades and store migration

Files:

- `app/upgrades/v1_12_0/upgrade.go`
- `app/upgrades/store_upgrade_manager.go`
- `app/upgrades/upgrades.go`

Changes:

- Added v1.12.0 store upgrades for:
  - feemarket
  - precisebank
  - vm
  - erc20
- Updated adaptive store upgrade manager coverage for missing stores in dev/test skip-upgrade flows.

Benefits/new features:

- Safer rollouts and upgrade compatibility for EVM stores.
- Easier devnet/testnet evolution with adaptive store management.

### 11) OpenRPC discovery, HTTP spec serving, and build consistency

Files:

- `app/openrpc/spec.go`
- `app/openrpc/rpc_api.go`
- `app/openrpc/register.go`
- `app/openrpc/http.go`
- `app/app.go`
- `tools/openrpcgen/main.go`
- `docs/openrpc_examples_overrides.json`
- `Makefile`

Changes:

- Added runtime OpenRPC discovery namespace (`rpc`) with JSON-RPC method:
  - `rpc_discover`
- Added HTTP OpenRPC document endpoint:
  - `GET /openrpc.json` (and `HEAD`)
- Added browser CORS/preflight support for OpenRPC HTTP endpoint:
  - `Access-Control-Allow-Origin: *`
  - `Access-Control-Allow-Methods: GET, HEAD, OPTIONS`
  - `Access-Control-Allow-Headers: Content-Type`
  - `OPTIONS /openrpc.json -> 204`
- Improved generated example shape for strict OpenRPC tooling compatibility:
  - `examples[*].params` is always present (empty array when no params).
  - `examples[*].result.value` is always present (including explicit `null`).
- Added OpenRPC generation into build dependency chain:
  - `build/lumerad` and `build-debug/lumerad` depend on `app/openrpc/openrpc.json`.
  - `openrpc` target generates `docs/openrpc.json` and copies to `app/openrpc/openrpc.json`.

Benefits/new features:

- Wallet/tooling clients can discover method catalogs consistently from the running node.
- OpenRPC playground/browser clients can fetch the spec cross-origin without manual proxy setup.
- Generated docs and embedded docs stay synchronized with built binaries, reducing stale-spec deployments.

## Detailed Integration Semantics

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
- Supports type-2 fee model (`maxFeePerGas`, `maxPriorityFeePerGas`).
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
- This lets Lumera keep `ulume` semantics in Cosmos while exposing `alume` precision to EVM.

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
- Keyring defaults and CLI defaults now use `eth_secp256k1`.

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
- EVM JSON-RPC/wallets use `0x...` hex addresses.
- For EVM-derived accounts, both are representations of the same underlying 20-byte address bytes.

Why it matters:
- Cosmos SDK workflows and EVM wallet workflows can coexist without changing user-facing APIs on either side.
- Indexers/explorers/wallet UIs need to display both forms where appropriate.

### 5) Gas token decimals `6 -> 18` view (`ulume` + `alume`)

What changed:
- Cosmos base denom remains `ulume` (6 decimals).
- EVM extended denom is `alume` (18 decimals).
- Conversion factor is `10^12`: `1 ulume = 10^12 alume`.

Precisebank arithmetic model:
- Let `I(a)` be integer bank balance in `ulume` units for account `a`.
- Let `F(a)` be precisebank fractional remainder in `[0, 10^12)`.
- EVM-view total for account `a` (in `alume`) is:
  - `EVMBalance(a) = I(a) * 10^12 + F(a)`

Why it matters:
- EVM fee/value transfers can operate at 18-decimal granularity.
- Cosmos bank invariants and integrations continue to operate with 6-decimal canonical storage.

### 6) EIP-1559 in Lumera (`x/feemarket`)

What changed:

- Dynamic base fee is enabled by default (`NoBaseFee=false`) with Lumera defaults.
- Type-2 transaction fee fields are supported and enforced.
- Minimum gas price floor (`MinGasPrice=0.0005 ulume/gas`) prevents the base fee from decaying to zero on low-activity chains. Without this floor, empty blocks cause the EIP-1559 algorithm to reduce the base fee by ~6.25% per block until it reaches zero, effectively disabling all fee enforcement.
- Base fee change denominator is set to `16` (upstream default is `8`), producing gentler ~6.25% adjustments per block instead of ~12.5%. This reduces fee volatility and slows decay during low-activity periods.

Behavioral consequences:

- Base fee adapts block-to-block with gas usage.
- Effective gas price is bounded by fee cap and includes priority tip behavior.
- Transactions are prioritized by fee competitiveness (including tip), plus nonce constraints per sender.
- The base fee cannot drop below `0.0005 ulume/gas` (0.5 gwei equivalent), ensuring a minimum cost for all transactions even during sustained low activity.

Current fee-routing behavior:

- Lumera currently uses standard SDK fee collection for EVM transactions.
- The EVM keeper computes and deducts the full effective gas price (`base fee + effective priority tip`) up front and sends it to the normal fee collector module account.
- Unused gas is refunded from the fee collector back to the sender after execution.
- The remaining collected fees are then distributed by `x/distribution` using the normal SDK path:
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
- Lumera integrates STRv2-style `x/erc20` representation with canonical bank-backed supply.
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

## Tests

## Unit Tests

### A) App wiring/config/genesis and command-level tests

Purpose: verifies that EVM runtime/CLI wiring is correctly initialized (genesis overrides, module order, precompiles, mempool, listeners, and command defaults).
Primary files:

- `app/evm_test.go`
- `app/evm_static_precompiles_test.go`
- `app/blocked_addresses_test.go`
- `app/evm_mempool_test.go`
- `app/evm_mempool_reentry_test.go`
- `app/evm_broadcast_test.go`
- `app/pending_tx_listener_test.go`
- `app/ibc_erc20_middleware_test.go`
- `app/ibc_test.go`
- `app/vm_preinstalls_test.go`
- `app/amino_codec_test.go`
- `cmd/lumera/cmd/config_test.go`
- `cmd/lumera/cmd/root_test.go`

| Test                                          | Description                                                                                    |
| --------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `TestRegisterEVMDefaultGenesis`             | Verifies EVM-related modules are registered and expose Lumera-specific default genesis values. |
| `TestEVMModuleOrderAndPermissions`          | Verifies module order constraints and module-account permissions for EVM modules.              |
| `TestEVMStoresAndModuleAccountsInitialized` | Verifies EVM KV/transient stores and module accounts are initialized in app startup.           |
| `TestEVMStaticPrecompilesConfigured`        | Verifies expected static precompiles are configured on the EVM keeper.                         |
| `TestBlockedAddressesMatrix`                | Verifies blocked-address set contains expected module/precompile addresses.                    |
| `TestPrecompileSendRestriction`             | Verifies bank send restriction blocks sends to EVM precompile addresses.                       |
| `TestEVMMempoolWiringOnAppStartup`          | Verifies app-side EVM mempool wiring occurs at startup with expected handlers.                 |
| `TestEVMMempoolReentrantInsertBlocks`       | Demonstrates mutex re-entry hazard that the async broadcast queue prevents.                    |
| `TestConfigureEVMBroadcastOptionsFromAppOptions` | Verifies broadcast debug flag parsing from app options (bool, string, nil).               |
| `TestEVMTxBroadcastDispatcherDedupesQueuedAndInFlight` | Verifies dispatcher deduplicates queued and in-flight tx hashes.                    |
| `TestEVMTxBroadcastDispatcherQueueFullReleasesPending` | Verifies queue-full path releases pending hash reservations.                        |
| `TestEVMTxBroadcastDispatcherReleasesPendingAfterProcessError` | Verifies pending hashes are released after broadcast process errors.           |
| `TestEVMTxBroadcastDispatcherEnqueueRemainsNonBlocking` | Verifies enqueue does not block while worker is processing.                    |
| `TestBroadcastEVMTxFromFieldRecovery`                   | Regression guard: `FromEthereumTx` leaves `From` empty; `FromSignedEthereumTx` recovers the sender. |
| `TestRegisterPendingTxListenerFanout`       | Verifies registered pending-tx listeners are invoked for each pending hash event.              |
| `TestIBCERC20MiddlewareWiring`              | Verifies IBC transfer stack includes ERC20 middleware wiring in app composition.               |
| `TestIsInterchainAccount`                   | Verifies ICA account type detection helper behavior.                                           |
| `TestIsInterchainAccountAddr`               | Verifies ICA detection by address lookup through account keeper.                               |
| `TestEVMAddPreinstallsMatrix`               | Verifies preinstall contract registration matrix in VM keeper setup paths.                     |
| `TestRegisterLumeraLegacyAminoCodecEnablesEthSecp256k1StdSignature` | Verifies legacy Amino registration covers eth_secp256k1 so SDK ante tx-size signature marshaling does not panic. |
| `TestInitAppConfigEVMDefaults`              | Verifies default app config enables EVM/JSON-RPC values expected by Lumera.                    |
| `TestNewRootCmdStartWiresEVMFlags`          | Verifies start/root command exposes key EVM JSON-RPC flags.                                    |
| `TestNewRootCmdDefaultKeyTypeOverridden`    | Verifies root command default key algorithm is overridden to `eth_secp256k1`.                |

### B) EVM ante unit tests (`app/evm`)

Purpose: verifies dual-route ante behavior and decorator-level Ethereum/Cosmos transaction validation logic.
Primary files:

- `app/evm/ante_decorators_test.go`
- `app/evm/ante_fee_checker_test.go`
- `app/evm/ante_gas_wanted_test.go`
- `app/evm/ante_handler_test.go`
- `app/evm/ante_min_gas_price_test.go`
- `app/evm/ante_mono_decorator_test.go`
- `app/evm/ante_nonce_test.go`
- `app/evm/ante_sigverify_test.go`

| Test                                                            | Description                                                                              |
| --------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `TestRejectMessagesDecorator`                                 | Verifies Cosmos ante path rejects blocked message types (for example MsgEthereumTx).     |
| `TestAuthzLimiterDecorator`                                   | Verifies authz limiter blocks grants for restricted message types.                       |
| `TestDynamicFeeCheckerMatrix`                                 | Verifies dynamic fee checker decisions across representative gas-fee inputs.             |
| `TestGasWantedDecoratorMatrix`                                | Verifies gas-wanted accounting updates are applied correctly per tx path.                |
| `TestNewAnteHandlerRequiredDependencies`                      | Verifies NewAnteHandler fails fast when required keeper/dependency inputs are missing.   |
| `TestNewAnteHandlerRoutesEthereumExtension`                   | Verifies extension option routes Ethereum txs to EVM ante chain.                         |
| `TestNewAnteHandlerRoutesDynamicFeeExtensionToCosmosPath`     | Verifies dynamic-fee extension routes tx to Cosmos ante path.                            |
| `TestNewAnteHandlerDefaultRouteWithoutExtension`              | Verifies txs without EVM extension use default Cosmos ante path.                         |
| `TestNewAnteHandlerPendingTxListenerTriggeredForEVMCheckTx`   | Verifies pending-tx listener fires for EVM CheckTx path.                                 |
| `TestNewAnteHandlerPendingTxListenerNotTriggeredOnCosmosPath` | Verifies pending-tx listener does not trigger on Cosmos ante path.                       |
| `TestMinGasPriceDecoratorMatrix`                              | Verifies min gas price decorator behavior across accepted/rejected fee cases.            |
| `TestEVMMonoDecoratorMatrix`                                  | Verifies EVM mono decorator baseline validation matrix.                                  |
| `TestEVMMonoDecoratorRejectsInvalidTxType`                    | Verifies EVM mono decorator rejects unsupported tx types.                                |
| `TestEVMMonoDecoratorRejectsNonEthereumMessage`               | Verifies EVM mono decorator rejects non-Ethereum message payloads.                       |
| `TestEVMMonoDecoratorRejectsSenderMismatch`                   | Verifies EVM mono decorator rejects signer/from mismatches.                              |
| `TestEVMMonoDecoratorRejectsInsufficientBalance`              | Verifies EVM mono decorator rejects txs with insufficient sender balance for fees/value. |
| `TestEVMMonoDecoratorRejectsNonEOASender`                     | Verifies EVM mono decorator rejects non-EOA senders where required.                      |
| `TestEVMMonoDecoratorAllowsDelegatedCodeSender`               | Verifies delegated-code sender case is accepted when rules permit it.                    |
| `TestEVMMonoDecoratorRejectsGasFeeCapBelowBaseFee`            | Verifies tx is rejected when fee cap is below current base fee.                          |
| `TestIncrementNonceMatrix`                                    | Verifies nonce increment semantics across successful tx paths.                           |
| `TestSigVerificationGasConsumerMatrix`                        | Verifies signature verification gas charging across key/signature types.                 |

### C) EVM module/config guard and genesis tests (`app/evm`)

Purpose: verifies EVM module registration/genesis defaults and production guardrails around test-only global resets.
Primary files:

- `app/evm/config_modules_genesis_test.go`
- `app/evm/prod_guard_test.go`

| Test                                     | Description                                                                              |
| ---------------------------------------- | ---------------------------------------------------------------------------------------- |
| `TestConfigureNoOp`                    | Verifies `Configure()` remains a safe no-op with current x/vm global config lifecycle. |
| `TestProvideCustomGetSigners`          | Verifies custom signer provider exposes MsgEthereumTx custom get-signer registration.    |
| `TestLumeraGenesisDefaults`            | Verifies Lumera EVM and feemarket genesis defaults match expected chain settings.        |
| `TestRegisterModulesMatrix`            | Verifies CLI-side registration map includes all EVM modules and wrappers.                |
| `TestResetGlobalStateRequiresTestTag`  | Verifies reset helper is guarded and requires `test` build tag.                        |
| `TestSetKeeperDefaultsRequiresTestTag` | Verifies keeper-default mutation helper is guarded behind `test` tag.                  |

### D) Fee market unit tests

Purpose: verifies feemarket arithmetic, lifecycle hooks, query APIs, and type validation invariants.
Primary files:

- `app/feemarket_test.go`
- `app/feemarket_types_test.go`

| Test                                               | Description                                                                         |
| -------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `TestFeeMarketCalculateBaseFee`                  | Verifies base-fee calculation matrix across target gas and min-gas-price scenarios. |
| `TestFeeMarketBeginBlockUpdatesBaseFee`          | Verifies BeginBlock updates base fee from prior gas usage inputs.                   |
| `TestFeeMarketEndBlockGasWantedClamp`            | Verifies EndBlock clamps block gas wanted using configured multiplier logic.        |
| `TestFeeMarketQueryMethods`                      | Verifies keeper query methods return consistent params/base-fee/block-gas values.   |
| `TestFeeMarketUpdateParamsAuthority`             | Verifies only authorized authority can update feemarket params.                     |
| `TestFeeMarketGRPCQueryClient`                   | Verifies gRPC query client paths for feemarket endpoints.                           |
| `TestFeeMarketTypesParamsValidateMatrix`         | Verifies feemarket params validation rules across valid/invalid combinations.       |
| `TestFeeMarketTypesMsgUpdateParamsValidateBasic` | Verifies basic validation for fee market MsgUpdateParams messages.                  |
| `TestFeeMarketTypesGenesisValidateMatrix`        | Verifies genesis validation matrix for feemarket state.                             |

### E) Precisebank unit tests

Purpose: verifies precisebank fractional accounting, bank parity behavior, mint/burn transitions, and type-level invariants.
Primary files:

- `app/precisebank_test.go`
- `app/precisebank_fractional_test.go`
- `app/precisebank_mint_burn_behavior_test.go`
- `app/precisebank_mint_burn_parity_test.go`
- `app/precisebank_types_test.go`

| Test                                                                    | Description                                                                              |
| ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `TestPreciseBankSplitAndRecomposeBalance`                             | Verifies extended balance splits into integer+fractional parts and recomposes correctly. |
| `TestPreciseBankSendExtendedCoinBorrowCarry`                          | Verifies fractional borrow/carry behavior during extended-denom transfers.               |
| `TestPreciseBankMintTransferBurnRestoresReserveAndRemainder`          | Verifies reserve/remainder bookkeeping round-trips after mint-transfer-burn sequence.    |
| `TestPreciseBankSendCoinsErrorParityWithBank`                         | Verifies send error messages/parity match bank keeper behavior.                          |
| `TestPreciseBankSendCoinsFromModuleToAccountBlockedRecipientParity`   | Verifies blocked-recipient behavior matches bank keeper for module-to-account sends.     |
| `TestPreciseBankSendCoinsFromModuleToAccountMissingModulePanicParity` | Verifies missing sender module panic parity with bank keeper.                            |
| `TestPreciseBankSendCoinsFromAccountToModuleMissingModulePanicParity` | Verifies missing recipient module panic parity with bank keeper.                         |
| `TestPreciseBankSendCoinsFromModuleToModuleMissingModulePanicParity`  | Verifies module-to-module missing-account panic parity with bank keeper.                 |
| `TestPreciseBankSendCoinsFromModuleToModuleErrorParityWithBank`       | Verifies module-to-module error-path parity with bank keeper.                            |
| `TestPreciseBankSendCoinsFromAccountToPrecisebankModuleBlocked`       | Verifies direct sends to precisebank module account are blocked as expected.             |
| `TestPreciseBankSendCoinsFromPrecisebankModuleToAccountBlocked`       | Verifies restricted sends from precisebank module account are blocked as expected.       |
| `TestPreciseBankMintCoinsToPrecisebankModulePanic`                    | Verifies minting directly into precisebank module account triggers expected panic.       |
| `TestPreciseBankBurnCoinsFromPrecisebankModulePanic`                  | Verifies burning directly from precisebank module account triggers expected panic.       |
| `TestPreciseBankRemainderAmountLifecycle`                             | Verifies remainder amount updates correctly through lifecycle operations.                |
| `TestPreciseBankInvalidRemainderAmountPanics`                         | Verifies invalid remainder values trigger expected panic behavior.                       |
| `TestPreciseBankReserveAddressHiddenForExtendedDenom`                 | Verifies reserve internals are hidden behind extended-denom abstractions.                |
| `TestPreciseBankGetBalanceAndSpendableCoin`                           | Verifies balance/spendable responses for extended-denom accounts.                        |
| `TestPreciseBankSetGetFractionalBalanceMatrix`                        | Verifies set/get fractional balance matrix across representative values.                 |
| `TestPreciseBankSetFractionalBalanceEmptyAddrPanics`                  | Verifies empty address input panics in fractional balance setter.                        |
| `TestPreciseBankSetFractionalBalanceZeroDeletes`                      | Verifies setting zero fractional balance removes persisted entry.                        |
| `TestPreciseBankIterateFractionalBalancesAndAggregateSum`             | Verifies iteration and aggregate sum over fractional balance entries.                    |
| `TestPreciseBankMintCoinsPermissionMatrix`                            | Verifies mint permission checks by module/denom path.                                    |
| `TestPreciseBankBurnCoinsPermissionMatrix`                            | Verifies burn permission checks by module/denom path.                                    |
| `TestPreciseBankMintExtendedCoinStateTransitions`                     | Verifies state transitions for minting extended-denom coins.                             |
| `TestPreciseBankBurnExtendedCoinStateTransitions`                     | Verifies state transitions for burning extended-denom coins.                             |
| `TestPreciseBankMintCoinsStateMatrix`                                 | Verifies mint state matrix across integer/fractional edge cases.                         |
| `TestPreciseBankMintCoinsMissingModulePanicParity`                    | Verifies missing-module panic parity for mint path.                                      |
| `TestPreciseBankBurnCoinsMissingModulePanicParity`                    | Verifies missing-module panic parity for burn path.                                      |
| `TestPreciseBankMintCoinsInvalidCoinsErrorParity`                     | Verifies invalid coin error parity for mint path.                                        |
| `TestPreciseBankBurnCoinsInvalidCoinsErrorParity`                     | Verifies invalid coin error parity for burn path.                                        |
| `TestPreciseBankTypesConversionFactorInvariants`                      | Verifies conversion factor constants and invariants for precisebank math.                |
| `TestPreciseBankTypesNewFractionalBalance`                            | Verifies constructor behavior for fractional balance type.                               |
| `TestPreciseBankTypesFractionalBalanceValidateMatrix`                 | Verifies validation matrix for single fractional balance entries.                        |
| `TestPreciseBankTypesFractionalBalancesValidateMatrix`                | Verifies validation matrix for collections of fractional balances.                       |
| `TestPreciseBankTypesFractionalBalancesSumAndOverflow`                | Verifies sum/overflow behavior in fractional balance aggregation.                        |
| `TestPreciseBankTypesGenesisValidateMatrix`                           | Verifies precisebank genesis validation matrix.                                          |
| `TestPreciseBankTypesGenesisTotalAmountWithRemainder`                 | Verifies total-amount computation with remainder in genesis state.                       |
| `TestPreciseBankTypesFractionalBalanceKey`                            | Verifies deterministic key derivation for fractional balance store entries.              |
| `TestPreciseBankTypesSumExtendedCoin`                                 | Verifies helper math for summing extended-denom coin amounts.                            |

### F) OpenRPC/generator unit tests

Purpose: verifies OpenRPC registration, embedded-spec serving semantics, CORS behavior, and spec generator output constraints expected by OpenRPC clients.
Primary files:

- `app/openrpc/openrpc_test.go`
- `app/openrpc/http_test.go`
- `tools/openrpcgen/main_test.go`

| Test                                                | Description                                                                                                        |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `TestDiscoverDocumentValid`                       | Verifies embedded OpenRPC JSON is valid and parseable.                                                            |
| `TestEnsureNamespaceEnabled`                      | Verifies `rpc` namespace append helper is idempotent and stable.                                                |
| `TestRegisterJSONRPCNamespaceIdempotent`          | Verifies repeated JSON-RPC namespace registration is safe.                                                        |
| `TestServeHTTPGet`                                | Verifies `/openrpc.json` GET response shape/content type and CORS headers.                                       |
| `TestServeHTTPHead`                               | Verifies `/openrpc.json` HEAD behavior and headers.                                                              |
| `TestServeHTTPMethodNotAllowed`                   | Verifies unsupported methods return `405` with correct `Allow` list.                                           |
| `TestServeHTTPOptions`                            | Verifies CORS preflight (`OPTIONS`) returns `204` and expected CORS headers.                                   |
| `TestCollectMethodsPrefersOverrideExamples`       | Verifies generator prefers curated overrides from `docs/openrpc_examples_overrides.json`.                       |
| `TestAlignExampleParamNamesRemapsIndexedArgs`     | Verifies generator remaps generic `argN` names to human-readable parameter names.                               |
| `TestExampleObjectSerializesNullValue`            | Verifies generator keeps explicit `result.value: null` instead of dropping the field.                           |
| `TestCollectMethodsExamplesAlwaysIncludeParamsField` | Verifies generator always emits `params` in examples (empty array when method has no parameters).             |

## Integration Tests

All integration tests are under `tests/integration/evm`.
Most packages use `-tags='integration test'`. The IBC ERC20 middleware package currently uses `-tags='test'`.

### A) Ante integration

Purpose: validates Cosmos-path ante behavior after EVM integration, including fee enforcement and authz message filtering.
Suite: `tests/integration/evm/ante/suite_test.go`

| Test                                         | Description                                                                                  |
| -------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `CosmosTxFeeEnforcement`                   | Verifies low-fee Cosmos txs are rejected and valid-fee txs pass under current ante settings. |
| `AuthzGenericGrantRejectsBlockedMsgTypes`  | Ensures authz generic grants cannot authorize blocked EVM message types.                     |
| `AuthzGenericGrantAllowsNonBlockedMsgType` | Ensures authz generic grants still work for allowed non-EVM message types.                   |

### B) Contracts integration

Purpose: exercises contract lifecycle paths (deploy/call/revert) and persistence guarantees across restarts.
Suite: `tests/integration/evm/contracts/suite_test.go`

| Test                                         | Description                                                                        |
| -------------------------------------------- | ---------------------------------------------------------------------------------- |
| `ContractDeployCallAndLogsE2E`             | Deploys a contract, executes calls, and validates receipt/log behavior end to end. |
| `ContractRevertTxReceiptAndGasE2E`         | Sends a reverting tx and checks expected revert/receipt/gas semantics.             |
| `CALLBetweenContracts`                     | Deploys caller/callee pair, validates CALL opcode returns data cross-contract.     |
| `DELEGATECALLPreservesContext`             | Verifies DELEGATECALL writes to proxy's storage, not target contract's storage.    |
| `CREATE2DeterministicAddress`              | Factory deploys child via CREATE2; verifies deterministic address off-chain.       |
| `STATICCALLCannotModifyState`              | Confirms STATICCALL reverts when the target contract attempts SSTORE.              |
| `TestContractCodePersistsAcrossRestart`    | Confirms deployed runtime bytecode remains queryable after node restart.           |
| `TestContractStoragePersistsAcrossRestart` | Confirms contract storage values remain intact after node restart.                 |

### C) Fee market integration

Purpose: validates EIP-1559 RPC behavior, effective gas price accounting, and dynamic-fee admission rules.
Suite: `tests/integration/evm/feemarket/suite_test.go`

| Test                                             | Description                                                                 |
| ------------------------------------------------ | --------------------------------------------------------------------------- |
| `FeeHistoryReportsCanonicalShape`              | Checks `eth_feeHistory` response shape and core fields for compatibility. |
| `ReceiptEffectiveGasPriceRespectsBlockBaseFee` | Verifies receipt `effectiveGasPrice` reflects block base fee constraints. |
| `FeeHistoryRewardPercentilesShape`             | Validates reward percentile formatting/structure in fee history results.    |
| `MaxPriorityFeePerGasReturnsValidHex`          | Ensures `eth_maxPriorityFeePerGas` returns a valid hex value.             |
| `GasPriceIsAtLeastLatestBaseFee`               | Ensures `eth_gasPrice` is not below current base fee expectations.        |
| `DynamicFeeType2EffectiveGasPriceFormula`      | Verifies type-2 tx effective gas price calculation is correct.              |
| `DynamicFeeType2RejectsFeeCapBelowBaseFee`     | Ensures txs with fee cap below base fee are rejected.                       |

### D) IBC ERC20 middleware integration

Purpose: validates ERC20 middleware behavior on ICS20 receive and edge-case handling for mapping registration.
Suite: `tests/integration/evm/ibc/suite_test.go`

| Test                                 | Description                                                                                  |
| ------------------------------------ | -------------------------------------------------------------------------------------------- |
| `RegistersTokenPairOnRecv`           | Ensures valid incoming ICS20 transfers auto-register ERC20 token pairs/maps.                 |
| `NoRegistrationWhenDisabled`         | Ensures registration is skipped when ERC20 middleware feature is disabled.                   |
| `NoRegistrationForInvalidReceiver`   | Ensures invalid receiver payloads do not create token mappings.                              |
| `DenomCollisionKeepsExistingMap`     | Ensures existing denom-map collisions are preserved and not overwritten.                     |
| `RoundTripTransfer`                  | Full IBC forward+reverse transfer with ERC20 registration, BalanceOf, and balance restore.   |
| `SecondaryDenomRegistration`         | Verifies non-native denom (ufoo) gets ERC20 auto-registration and dynamic precompile.        |
| `TransferBackBurnsVoucher`           | Verifies return transfer zeros bank and ERC20 balances while token pair persists.            |

### E) JSON-RPC/indexer integration

Purpose: validates JSON-RPC compatibility, tx/receipt lookup/indexer behavior, mixed Cosmos+EVM block behavior, and restart durability.
Suites:

- `tests/integration/evm/jsonrpc/suite_test.go`
- `tests/integration/evm/jsonrpc/mixed_block_suite_test.go`

| Test                                           | Description                                                                                        |
| ---------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `BasicRPCMethods`                            | Verifies baseline RPC methods (`eth_chainId`, `eth_blockNumber`, etc.) return expected values. |
| `BackendBlockCountAndUncleSemantics`         | Validates block-count and uncle-related method semantics on this backend.                          |
| `BackendNetAndWeb3UtilityMethods`            | Verifies `net_*` and `web3_*` utility methods return sane values.                              |
| `BlockLookupIncludesTransaction`             | Ensures block queries include expected transaction objects/hashes.                                 |
| `TransactionLookupByBlockAndIndex`           | Validates tx lookup by block hash/number + index works correctly.                                  |
| `MultiTxOrderingSameBlock`                   | Verifies deterministic `transactionIndex` ordering for multiple txs in one block.                |
| `ReceiptIncludesCanonicalFields`             | Ensures receipts expose canonical Ethereum fields and expected encodings.                          |
| `MixedCosmosAndEVMTransactionsCanShareBlock` | Confirms Cosmos and EVM txs can be included together in the same committed block.                  |
| `MixedBlockOrderingPersistsAcrossRestart`    | Confirms mixed-block tx ordering is preserved across restart.                                      |
| `TestEOANonceByBlockTagAndRestart`           | Verifies nonce query semantics by block tag and restart persistence.                               |
| `TestSelfTransferFeeAccounting`              | Verifies self-transfer balance delta equals `gasUsed * effectiveGasPrice`.                       |
| `TestIndexerDisabledLookupUnavailable`       | Verifies tx/receipt lookups are unavailable when indexers are disabled.                            |
| `TestLogsIndexerPathAcrossRestart`           | Verifies `eth_getLogs` indexer queries remain correct across restart.                            |
| `TestReceiptPersistsAcrossRestart`           | Verifies `eth_getTransactionReceipt` remains available after restart.                            |
| `TestIndexerStartupSmoke`                    | Smoke-tests JSON-RPC/WebSocket/indexer startup path and startup logs.                              |
| `TestTransactionByHashPersistsAcrossRestart` | Verifies `eth_getTransactionByHash` consistency before/after restart.                            |
| `OpenRPCDiscoverMethodCatalog`               | Verifies `rpc_discover` returns non-empty, deduplicated catalog with required namespace coverage. |
| `OpenRPCDiscoverMatchesEmbeddedSpec`         | Verifies runtime `rpc_discover` output matches the embedded OpenRPC document in the node binary. |
| `TestOpenRPCHTTPDocumentEndpoint`            | Verifies `/openrpc.json` (API server) is served and matches JSON-RPC `rpc_discover` method set. |

### F) Mempool integration

Purpose: validates app-side EVM mempool behavior for ordering, pending visibility, nonce handling, and replacement policy.
Suite: `tests/integration/evm/mempool/suite_test.go`

| Test                                        | Description                                                                     |
| ------------------------------------------- | ------------------------------------------------------------------------------- |
| `DeterministicOrderingUnderContention`    | Verifies deterministic inclusion ordering under concurrent submission pressure. |
| `EVMFeePriorityOrderingSameBlock`         | Verifies higher-fee tx priority ordering when txs land in the same block.       |
| `PendingTxSubscriptionEmitsHash`          | Verifies pending subscription emits tx hashes for pending EVM txs.              |
| `NonceGapPromotionAfterGapFilled`         | Verifies queued nonce-gap txs are promoted once missing nonce is filled.        |
| `TestMempoolDisabledWithJSONRPCFailsFast` | Verifies txpool namespace behavior when app-side mempool is disabled.           |
| `TestNonceReplacementRequiresPriceBump`   | Verifies same-nonce replacement requires configured fee bump threshold.         |

### G) Precisebank integration

Purpose: validates transaction-level and query-level behavior of fractional balance accounting under EVM flows.
Suite: `tests/integration/evm/precisebank/suite_test.go`

| Test                                                    | Description                                                                       |
| ------------------------------------------------------- | --------------------------------------------------------------------------------- |
| `PreciseBankFractionalBalanceQueryMatrix`             | Verifies fractional-balance query responses across representative account states. |
| `PreciseBankFractionalBalanceRejectsInvalidAddress`   | Verifies invalid address formats are rejected by precisebank queries.             |
| `PreciseBankEVMTransferSendSplitMatrix`               | Verifies integer/fractional split behavior across EVM transfer scenarios.         |
| `PreciseBankSecondarySenderBurnMintWorkflow`          | Verifies mint/send/burn workflow behavior using secondary sender flows.           |
| `TestPreciseBankRemainderQueryPersistsAcrossRestart`  | Verifies precisebank remainder query results persist after restart.               |
| `TestPreciseBankModuleAccountFractionalBalanceIsZero` | Verifies module account fractional balance invariants remain zero as expected.    |

### H) Precompiles integration

Purpose: validates static precompile read/write paths exposed to EVM callers.
Suite: `tests/integration/evm/precompiles/suite_test.go`

| Test                                                         | Description                                                            |
| ------------------------------------------------------------ | ---------------------------------------------------------------------- |
| `BankPrecompileBalancesViaEthCall`                         | Verifies bank precompile balance queries via `eth_call`.             |
| `DistributionPrecompileQueryPathsViaEthCall`               | Verifies distribution precompile query methods via `eth_call`.       |
| `GovPrecompileQueryPathsViaEthCall`                        | Verifies governance precompile query methods via `eth_call`.         |
| `StakingPrecompileValidatorViaEthCall`                     | Verifies staking precompile validator query behavior via `eth_call`. |
| `Bech32PrecompileRoundTripViaEthCall`                      | Verifies Bech32 precompile address conversion round-trips correctly.   |
| `P256PrecompileVerifyViaEthCall`                           | Verifies P256 precompile signature verification behavior.              |
| `StakingPrecompileDelegateTxPath`                          | Verifies staking delegate tx path through precompile execution.        |
| `DistributionPrecompileSetWithdrawAddressTxPath`           | Verifies distribution withdraw-address tx path via precompile.         |
| `GovPrecompileCancelProposalTxPathFailsForUnknownProposal` | Verifies expected failure behavior for canceling unknown proposals.    |

### I) VM query/state integration

Purpose: validates `x/vm` query APIs and consistency against JSON-RPC/accounting/state snapshots.
Suite: `tests/integration/evm/vm/suite_test.go`

| Test                                               | Description                                                                   |
| -------------------------------------------------- | ----------------------------------------------------------------------------- |
| `VMQueryParamsAndConfigBasic`                    | Verifies vm params/config query endpoints return expected baseline values.    |
| `VMAddressConversionRoundTrip`                   | Verifies VM address conversion utilities round-trip correctly.                |
| `VMQueryAccountMatchesEthRPC`                    | Verifies VM account query fields match equivalent JSON-RPC account state.     |
| `VMQueryAccountRejectsInvalidAddress`            | Verifies VM account query rejects invalid addresses.                          |
| `VMQueryAccountAcceptsHexAndBech32`              | Verifies VM account query accepts both hex and Bech32 forms where supported.  |
| `VMBalanceBankMatchesBankQuery`                  | Verifies VM bank-balance query is consistent with bank module query results.  |
| `VMStorageQueryKeyFormatEquivalence`             | Verifies storage queries are equivalent across supported key encodings.       |
| `VMQueryCodeAndStorageMatchJSONRPC`              | Verifies VM code/storage queries align with JSON-RPC responses.               |
| `VMQueryAccountHistoricalHeightNonceProgression` | Verifies historical-height account queries show expected nonce progression.   |
| `VMQueryHistoricalCodeAndStorageSnapshots`       | Verifies historical code/storage snapshots are queryable and consistent.      |
| `VMBalanceERC20MatchesEthCall`                   | Verifies VM ERC20 balance query matches direct contract `eth_call` results. |
| `VMBalanceERC20RejectsNonERC20Runtime`           | Verifies ERC20 balance query fails cleanly for non-ERC20 runtimes.            |

## Operational Outcomes

After this integration:

- Lumera can execute Ethereum transactions and EVM bytecode natively through Cosmos EVM (`x/vm`).
- JSON-RPC/WebSocket/indexer are enabled by default, so standard Ethereum client flows work without extra node flags.
- Wallet UX is improved:
  - MetaMask-compatible account/key model (`eth_secp256k1`, BIP44 coin type 60).
  - Ethereum-style address/key expectations align with common EVM tooling.
- Smart contract developer UX is unlocked:
  - Solidity/Vyper contracts can be deployed and interacted with using standard EVM JSON-RPC methods.
  - Common toolchains (for example Hardhat/Foundry/Web3/Ethers libraries) can target Lumera via RPC.
- EIP-1559 dynamic base fee is active with Lumera defaults (base fee 0.0025, min 0.0005, denominator 16), enabling predictable fee market behavior with spam protection.
- Precisebank enables 18-decimal extended-denom accounting while preserving Cosmos bank compatibility.
- Static precompiles expose Cosmos functionality (bank/staking/distribution/gov/bech32/p256/slashing/ics20) to EVM contracts.
- IBC ERC20 middleware wiring enables ERC20-aware ICS20 receive/mapping flows for cross-chain token paths.
- Upgrade path includes EVM store migrations (v1.12.0) with adaptive store-manager support for safer network evolution.
- OpenRPC method catalog is available from the running node over:
  - JSON-RPC: `rpc_discover`
  - HTTP API server: `/openrpc.json` (CORS-enabled for browser tooling)

## Architecture Strengths

### Circular dependency resolution

The EVM keeper graph has unavoidable cycles (EVMKeeper needs Erc20Keeper for precompiles; Erc20Keeper needs EVMKeeper for contract calls). The wiring in `app/evm.go` resolves this cleanly via pointer-based forward references:

```go
EVMKeeper  = NewKeeper(..., &app.Erc20Keeper)   // populated below
Erc20Keeper = NewKeeper(..., app.EVMKeeper, &app.EVMTransferKeeper)
```

Both keepers are usable at runtime without `nil`-pointer races because the IBC transfer keeper (the last link in the cycle) is resolved before any block execution begins.

### Dual-route ante handler with explicit extension routing

Transaction routing is deterministic and non-ambiguous. The ante handler in `app/evm/ante.go` inspects `ExtensionOptions[0].TypeUrl` to choose between three paths:

| Extension | Route | Decorators |
| --- | --- | --- |
| `ExtensionOptionsEthereumTx` | EVM path | EVMMonoDecorator + pending tx listener |
| `ExtensionOptionDynamicFeeTx` | Cosmos path | Full Lumera + EVM-aware Cosmos decorator chain |
| _(none)_ | Default Cosmos path | Same Cosmos chain, DynamicFeeChecker disabled |

This prevents Ethereum messages from leaking into the Cosmos validation path (or vice versa) and ensures fee semantics match the transaction type.

### Module ordering correctness

The genesis/begin/end block ordering in `app/app_config.go` satisfies all dependency constraints:

- **EVM initializes first in genesis** (before erc20, precisebank, genutil) so coin info is available for all downstream consumers.
- **FeeMarket EndBlocker runs last** to capture full block gas usage for accurate base fee calculation.
- **EVM PreBlocker** runs after upgrade and auth to ensure coin info is populated before early RPC queries hit the node.

### Production guardrails

Build-tag protection (`//go:build !test` in `app/evm/defaults_prod.go`) prevents test-only global state resets from compiling into production binaries. The `SetKeeperDefaults` function initializes EVM coin info on app startup to prevent RPC panics before genesis runs. Both guardrails have dedicated unit tests.

### Async broadcast queue prevents mempool deadlock

The EVM txpool's `runReorg` calls `BroadcastTxFn` synchronously while holding the mempool mutex (`m.mtx`). If `BroadcastTxFn` submits a tx via CometBFT's local ABCI client, `CheckTx` calls back into `Insert()` on the same mempool — which tries to acquire `m.mtx` again, deadlocking the chain.

The `evmTxBroadcastDispatcher` in `app/evm_broadcast.go` breaks this cycle:

1. `BroadcastTxFn` (called inside `runReorg`) enqueues promoted txs into a bounded channel and returns immediately — never blocking `Insert()`.
2. A single background worker goroutine drains the channel and submits txs via `BroadcastTxSync` after the mutex is released.
3. Tx hashes are tracked in a `pending` set for deduplication; hashes are released after processing or on queue-full/error paths.

The `RegisterTxService` override in `app/evm_runtime.go` ensures the broadcast worker uses the local CometBFT client (not the stale HTTP client that `SetClientCtx` provides before CometBFT starts). The re-entry hazard is validated by `TestEVMMempoolReentrantInsertBlocks`, and the full promotion-to-inclusion path is validated by the `NonceGapPromotionAfterGapFilled` integration test.

### Precompile address protection

Bank send restrictions block token sends to all 8 precompile addresses plus module accounts. This prevents accidental token loss to system addresses that cannot sign outbound transactions.

### IBC-EVM middleware layering

The transfer stack is properly layered for both IBC v1 and v2:

```text
v1: EVMTransferKeeper -> ERC20IBCMiddleware -> CallbacksMiddleware -> PFM
v2: TransferV2Module -> CallbacksV2Middleware -> ERC20IBCMiddlewareV2
```

The `EVMTransferKeeper` maintains an `ICS4Wrapper` back-reference for callback chains, ensuring packet acknowledgments propagate correctly through the full middleware stack.

### OpenRPC build-time synchronization

The OpenRPC spec is regenerated on every `make build` via the `tools/openrpcgen` tool, which uses Go reflection and AST parsing to introspect the actual RPC implementation types. The spec is then `//go:embed`-ded into the binary. This eliminates stale-spec drift: the running node always serves a spec that matches its compiled RPC surface.

### 18-decimal precision bridge design

The `x/precisebank` module preserves Cosmos bank invariants (6-decimal `ulume`) while exposing 18-decimal `alume` to EVM. The arithmetic model (`EVMBalance(a) = I(a) * 10^12 + F(a)`) keeps canonical supply accounting in `x/bank` and tracks only sub-`ulume` fractional remainders in precisebank state. This avoids dual-supply risks and keeps the Cosmos-side accounting simple.

## Design Document vs Implementation Gap Analysis

Comparing the requirements in `docs/Lumera_Cosmos_EVM_Integration.pdf` against the current codebase:

| Requirement | Status | Notes |
| --- | --- | --- |
| Core EVM execution (`x/evm`) | Done | Full keeper/module/store wiring |
| EIP-1559 fee market (`x/feemarket`) | Done | Base fee 0.0025 ulume/gas, min 0.0005, denominator 16 |
| Decimal precision bridge (`x/precisebank`) | Done | ulume <-> alume bridging |
| STRv2 / ERC20 representation (`x/erc20`) | Done | IBC middleware integrated |
| Dual ante handler pipeline | Done | EVM + Cosmos paths with claim fee decorator |
| EVM mempool with nonce ordering | Done | ExperimentalEVMMempool wired |
| Ethereum JSON-RPC server | Done | 7 namespaces + rpc_discover |
| EVM chain ID configured | Done | 76857769 |
| Store upgrades at activation height | Done | v1.12.0 handler for 4 stores |
| **Base fee burn vs distribute decision** | **Open** | Document flags this as unresolved; has tokenomics implications |
| **IBC voucher ERC20 registration policy** | **Open** | No explicit policy for which IBC denoms get auto-registration |
| **Lumera module precompiles** | **Not started** | Document hints at future precompiles for action/claim/supernode/lumeraid |
| **CosmWasm + EVM interaction** | **Not addressed** | Neither the document nor the code defines an interaction model |
| **Ops runbooks for fee market monitoring** | **Not done** | Document calls this out as needed for production readiness |

## Test Coverage Assessment

### Coverage by area

| Area | Unit tests | Integration tests | Coverage quality |
| --- | --- | --- | --- |
| Fee market (EIP-1559) | 9 | 7 | Excellent — matrix testing, edge cases, queries, receipt formula verification |
| Precisebank | 30 | 6 | Excellent — invariants, error parity with bank, lifecycle, permissions |
| Ante decorators | 21 | 3 | High — path routing, authz limits, nonce, gas, sig verification, mono decorator |
| JSON-RPC / indexer | 0 | 19 | Very high — API surface, receipts, logs, mixed blocks, restart persistence |
| Precompiles | 0 | 9 | High — query + tx paths for bank/staking/distribution/gov/bech32/p256 |
| Mempool | 7 | 6 | High — async broadcast queue, dedupe, re-entry hazard, ordering, nonce gaps, priority; missing eviction/capacity pressure |
| Contracts | 0 | 8 | High — deploy/call/revert/persistence, CALL, DELEGATECALL, CREATE2, STATICCALL |
| IBC ERC20 middleware | 1 | 7 | High — registration, round-trip transfer, secondary denom, burn-back, BalanceOf |
| VM / state queries | 0 | 12 | Medium — queries adequate; historical snapshots covered |
| OpenRPC | 4 | 3 | High — generator output, embedded spec, HTTP endpoint, runtime consistency |
| App wiring / config | 22 | 0 | High — genesis, module order, permissions, commands, precompiles, broadcast queue |

### Critical test gaps

1. **Multi-block fee progression** — No test simulates sustained blocks above or below target gas over many blocks to verify base fee convergence and stability.

2. **Mempool eviction and capacity pressure** — Current tests cover ordering and nonce gaps but not behavior under full mempool capacity or rapid replacement races.

3. **Batch JSON-RPC requests** — No test validates multi-request batching behavior.

4. **WebSocket subscriptions** — Infrastructure exists but coverage is limited to `PendingTxSubscriptionEmitsHash`.

### Moderate test gaps

- Precompile gas metering accuracy validation
- Multi-validator EVM consensus scenarios (devnet tests use single validator assertions)
- Chain upgrade with EVM state preservation
- Concurrent operation race condition detection
- ERC20 allowance/transferFrom/approve flows via precompile

## Recommended Next Steps

### High priority

1. **Decide whether to separate base fee from tip in accounting/distribution** — Current behavior sends both components through standard SDK fee collection and distribution. If Lumera wants a true "base fee only" policy surface, the execution path must isolate the base-fee portion from the tip portion before distribution.

2. **Define IBC voucher ERC20 registration policy** — Decide which IBC denoms get automatic ERC20 registration (all, allowlist, governance-gated) and configure this in genesis or as a governance parameter.

### Medium priority

1. **Lumera module precompiles** — Design precompiles for custom modules (action, claim, supernode, lumeraid) so EVM contracts can query or interact with Lumera-specific functionality. Start with read-only query precompiles and expand to write paths after audit.

2. **Add multi-block fee progression test** — Simulate 50+ blocks with varying gas usage and verify base fee converges toward the target range without oscillation.

3. **Add mempool stress tests** — Eviction under capacity pressure, rapid nonce replacement races, same-fee-priority tiebreaking, and interaction with `PrepareProposal` signer extraction.

4. **CosmWasm + EVM interaction design** — Document whether/how CosmWasm contracts and EVM contracts can interact. Consider a bridge mechanism, shared query paths, or explicit isolation.

### Low priority

1. **Batch JSON-RPC tests** — Validate multi-request batching returns correct results for mixed-method batches.

2. **WebSocket subscription tests** — `eth_subscribe` for `newHeads`, `logs`, `newPendingTransactions` with filter parameters.

3. **Precompile gas metering benchmarks** — Validate actual gas consumption vs expected for each precompile and compare against upstream Cosmos EVM defaults.

4. **Ops monitoring runbook** — Document fee market monitoring (base fee tracking, gas utilization trends), alerting thresholds, and common failure mode diagnosis.

## Devnet Tests

Devnet tests run inside the Docker multi-validator testnet (`make devnet-new`).
Test source: `devnet/tests/validator/evm_test.go`

| Test | Description |
| ---- | ----------- |
| `TestEVMFeeMarketBaseFeeActive` | Validates `eth_gasPrice` returns a non-zero base fee on an active devnet. |
| `TestEVMDynamicFeeTxE2E` | Sends a type-2 (EIP-1559) self-transfer and verifies receipt status 0x1. |
| `TestEVMTransactionVisibleAcrossPeerValidator` | Sends a tx to the local validator and verifies the receipt is visible on a peer validator with matching blockHash — exercises the broadcast worker re-gossip path. |

## Bugs Found and Fixed

Tracking issues discovered during EVM integration testing and devnet operation.

### 1) EVM broadcast worker: sender address not recovered

**Symptom**: All validators log `failed to broadcast promoted evm transactions … sender address is missing: invalid request` (code 18) after EVM txs land.

**Root cause**: `broadcastEVMTransactionsSync` used `msg.FromEthereumTx(ethTx)` which copies raw tx bytes but does **not** populate the `From` field. The Cosmos ante handler then rejects the message because `GetSigners()` returns an empty sender.

**Fix** (`app/evm_broadcast.go`): Replaced with `msg.FromSignedEthereumTx(ethTx, ethSigner)` which recovers the sender address from the ECDSA signature using the chain's EVM signer.

**Why tests passed**: The JSON-RPC ingestion path (`eth_sendRawTransaction` → txpool → mempool `Insert`) already uses `FromSignedEthereumTx`. The broadcast worker only re-gossips promoted txs to peer validators, so single-validator integration tests never exercise this path.

**Tests added**: `TestBroadcastEVMTxFromFieldRecovery` (unit — validates `FromSignedEthereumTx` recovers sender while `FromEthereumTx` does not), `TestEVMTransactionVisibleAcrossPeerValidator` (devnet — end-to-end cross-validator propagation).

---

### 2) Feemarket base fee decays to zero on idle devnet

**Symptom**: `TestEVMFeeMarketBaseFeeActive` fails because `eth_gasPrice` / `baseFeePerGas` returns 0 after a few hundred blocks with no EVM traffic.

**Root cause**: Devnet uses a **static genesis template** (`devnet/default-config/devnet-genesis-evm.json`) that bypasses the app's `LumeraFeemarketGenesisState()`. The template had stale values: `min_gas_price: 0` (no floor) and `base_fee_change_denominator: 8` (aggressive decay).

**Fix** (`devnet/default-config/devnet-genesis-evm.json`): Updated the template to match `config/evm.go` constants — `min_gas_price: 0.0005`, `base_fee_change_denominator: 16`.

**Lesson**: Any change to `config/evm.go` or `app/evm/genesis.go` feemarket defaults must also be mirrored in the static devnet genesis template.

---

### 3) Gentx rejected by MinGasPriceDecorator during InitGenesis

**Symptom**: After fixing the feemarket genesis params (non-zero `min_gas_price`), `lumerad` fails to start: `fee not provided … minimum global fee is 100ulume: insufficient fee`.

**Root cause**: The cosmos/evm `MinGasPriceDecorator` enforces minimum gas prices unconditionally, including during InitGenesis (block height 0). The standard Cosmos SDK fee decorators skip enforcement at genesis, but cosmos/evm's decorator does not.

**Fix** (`app/evm/ante.go`): Added `genesisSkipDecorator` — a generic wrapper that skips the inner decorator when `BlockHeight() == 0`. Applied to `MinGasPriceDecorator` in the Cosmos ante chain so gentxs are processed without fees, matching standard SDK behavior.

---

### 4) IBC transfer silently fails with out-of-gas

**Symptom**: `TestIBCTransferWithEVMModeStillRelays` fails — transfer appears to succeed but tokens never arrive on the destination chain.

**Root cause**: Two issues combined:
1. Gas estimation returned 70907 but actual execution cost 72619. The `--gas-adjustment 1.3` margin was insufficient.
2. `lumerad tx --broadcast-mode sync` exits with code 0 even when CheckTx rejects the tx. The test helper discarded command output, so the rejection was invisible.

**Fix** (`devnet/tests/ibcutil/ibcutil.go`):
- Increased `--gas-adjustment` from 1.3 to 1.5.
- Added `--output json` and JSON response parsing to detect non-zero result codes.

**Also** (`devnet/hermes/config.toml`): Reduced `clear_interval` from 100 to 10 as a safety net for missed WebSocket packet events.

---

### 5) EVM mempool deadlock on nonce-gap promotion (BroadcastTxFn re-entry)

**Symptom**: The chain hangs permanently when an EVM transaction fills a nonce gap in the txpool. All block production stops and the node becomes unresponsive.

**Root cause**: The cosmos/evm `ExperimentalEVMMempool` calls `BroadcastTxFn` synchronously from inside `runReorg` while holding the mempool mutex (`m.mtx`). If `BroadcastTxFn` submits the promoted tx via CometBFT's local ABCI client, the resulting `CheckTx` calls back into `Insert()` on the same mempool — which tries to acquire `m.mtx` again. Since Go's `sync.Mutex` is not reentrant, this deadlocks the goroutine and halts the chain.

The call stack that deadlocks:

```text
Insert() → [acquires m.mtx] → runReorg() → BroadcastTxFn()
  → BroadcastTxSync() → local ABCI CheckTx → Insert() → [blocks on m.mtx] ← DEADLOCK
```

**Fix** (`app/evm_broadcast.go`): Implemented `evmTxBroadcastDispatcher` — an async broadcast queue that decouples txpool promotion from CometBFT CheckTx submission:

1. `BroadcastTxFn` (called inside `runReorg`) enqueues promoted txs into a bounded channel and returns immediately — never blocking `Insert()`.
2. A single background worker goroutine drains the channel and submits txs via `BroadcastTxSync` after the mutex is released.
3. Tx hashes are tracked in a `pending` set for deduplication; hashes are released after processing or on queue-full/error paths.

Additionally, `RegisterTxService` override in `app/evm_runtime.go` ensures the broadcast worker uses the local CometBFT client (not the stale HTTP client from `SetClientCtx` which runs before CometBFT starts).

**Tests**: The re-entry hazard is validated by `TestEVMMempoolReentrantInsertBlocks` (unit), and the full promotion-to-inclusion path is validated by `NonceGapPromotionAfterGapFilled` (integration).

---

## Notes and Intentional Constraints

- Vesting precompile is intentionally not enabled because upstream default static precompile registry in current Cosmos EVM version does not provide it.
- Some restart-heavy or custom-startup integration tests remain standalone by design to avoid shared-suite state interference and keep CI deterministic.
- OpenRPC HTTP spec endpoint is exposed by the API server (`--api.enable=true`, typically port `1317`), not by the EVM JSON-RPC root port (`8545`/mapped devnet JSON-RPC ports).
- `rpc_discover` (underscore) is the registered JSON-RPC method name; `rpc.discover` (dot) is not currently aliased by Cosmos EVM JSON-RPC dispatch.
