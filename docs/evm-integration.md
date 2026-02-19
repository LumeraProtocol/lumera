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
  - `ChainDefaultConsensusMaxGas = 10_000_000`
- Centralized bank denom metadata via`ChainBankMetadata`/`UpsertChainBankMetadata`.
- Added `RegisterExtraInterfaces` to register Cosmos crypto + EVM crypto interfaces (including `eth_secp256k1`).

Benefits/new features:

- Ethereum-compatible key derivation and wallet UX.
- Consistent denom metadata for SDK + EVM paths.
- Stable chain-wide EVM chain-id/base-fee/max-gas defaults.

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
  - Feemarket defaults with dynamic base fee enabled.
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

### 4) App-side EVM mempool integration

Files:

- `app/mempool.go`
- `app/app.go`
- `cmd/lumera/cmd/config.go`

Changes:

- Wired Cosmos EVM experimental mempool into BaseApp:
  - `app.SetMempool(evmMempool)`
  - EVM-aware `CheckTx` handler
  - EVM-aware `PrepareProposal` signer extraction adapter
- Added runtime broadcaster for promoted EVM txs.
- Enabled app-side mempool by default in app config (`max_txs=5000`).

Benefits/new features:

- Pending tx support and txpool behavior aligned with Cosmos EVM.
- Better Ethereum tx ordering/replacement/nonce-gap behavior.
- EVM-aware proposal building for mixed workloads.

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
- Enabled dynamic base fee in default genesis.
- Added module ordering and permissions to include feemarket/precisebank correctly.

Benefits/new features:

- EIP-1559-style fee market behavior.
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

Behavioral consequences:
- Base fee adapts block-to-block with gas usage.
- Effective gas price is bounded by fee cap and includes priority tip behavior.
- Transactions are prioritized by fee competitiveness (including tip), plus nonce constraints per sender.

Why it matters:
- Wallet fee estimation and transaction inclusion behavior now match common Ethereum user expectations.

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
- `app/mempool_test.go`
- `app/pending_tx_listener_test.go`
- `app/ibc_erc20_middleware_test.go`
- `app/ibc_test.go`
- `app/vm_preinstalls_test.go`
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
| `TestRegisterPendingTxListenerFanout`       | Verifies registered pending-tx listeners are invoked for each pending hash event.              |
| `TestBroadcastEVMTransactionsWithoutNode`   | Verifies EVM tx broadcast callback behavior without requiring full node process startup.       |
| `TestIBCERC20MiddlewareWiring`              | Verifies IBC transfer stack includes ERC20 middleware wiring in app composition.               |
| `TestIsInterchainAccount`                   | Verifies ICA account type detection helper behavior.                                           |
| `TestIsInterchainAccountAddr`               | Verifies ICA detection by address lookup through account keeper.                               |
| `TestEVMAddPreinstallsMatrix`               | Verifies preinstall contract registration matrix in VM keeper setup paths.                     |
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

| Test                                 | Description                                                                  |
| ------------------------------------ | ---------------------------------------------------------------------------- |
| `RegistersTokenPairOnRecv`         | Ensures valid incoming ICS20 transfers auto-register ERC20 token pairs/maps. |
| `NoRegistrationWhenDisabled`       | Ensures registration is skipped when ERC20 middleware feature is disabled.   |
| `NoRegistrationForInvalidReceiver` | Ensures invalid receiver payloads do not create token mappings.              |
| `DenomCollisionKeepsExistingMap`   | Ensures existing denom-map collisions are preserved and not overwritten.     |

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
- EIP-1559 dynamic base fee is active with Lumera defaults, enabling predictable fee market behavior.
- Precisebank enables 18-decimal extended-denom accounting while preserving Cosmos bank compatibility.
- Static precompiles expose Cosmos functionality (bank/staking/distribution/gov/bech32/p256/slashing/ics20) to EVM contracts.
- IBC ERC20 middleware wiring enables ERC20-aware ICS20 receive/mapping flows for cross-chain token paths.
- Upgrade path includes EVM store migrations (v1.12.0) with adaptive store-manager support for safer network evolution.

## Notes and Intentional Constraints

- Vesting precompile is intentionally not enabled because upstream default static precompile registry in current Cosmos EVM version does not provide it.
- Some restart-heavy or custom-startup integration tests remain standalone by design to avoid shared-suite state interference and keep CI deterministic.
