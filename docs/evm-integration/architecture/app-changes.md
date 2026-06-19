# App Changes and Features

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
- Added explicit Bech32 constants and helper`SetBech32Prefixes`.
- Added`SetBip44CoinType` to set BIP44 purpose 44 and coin type 60 (Ethereum).
- Added EVM constants:
  - `EVMChainID = 76857769`
  - `FeeMarketDefaultBaseFee = "0.0025"`
  - `FeeMarketMinGasPrice = "0.0005"` (floor preventing base fee decay to zero)
  - `FeeMarketBaseFeeChangeDenominator = 16` (gentler ~6.25% adjustment per block)
  - `ChainDefaultConsensusMaxGas = 25_000_000`
- Centralized bank denom metadata via`ChainBankMetadata`/`UpsertChainBankMetadata`.
- Added`RegisterExtraInterfaces` to register Cosmos crypto + EVM crypto interfaces (including`eth_secp256k1`).

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
- Added depinject signer wiring for`MsgEthereumTx` via`ProvideCustomGetSigners`.
- Added depinject interface registration invoke (`RegisterExtraInterfaces`).
- Added default keeper coin info initialization (`SetKeeperDefaults`) for safe early RPC behavior.
- Added EVM module order/account permissions into genesis/begin/end/pre-block scheduling and module account perms.
- EVM tracer reads from`app.toml[evm] tracer` field /`--evm.tracer` CLI flag (valid:`json`,`struct`,`access_list`,`markdown`, or empty to disable). Enables`debug_traceTransaction`,`debug_traceBlockByNumber`,`debug_traceBlockByHash`,`debug_traceCall` JSON-RPC methods when set.

Benefits/new features:

- Full EVM module stack is bootstrapped in app runtime.
- Correct signer derivation for Ethereum tx messages.
- Lumera-specific EVM genesis defaults are applied by default.
- EVM debug/tracing API fully configurable at runtime without code changes.

### 3) Ante handler: dual routing and EVM decorators

Files:

- `app/evm/ante.go`
- `app/app.go`

Changes:

- Replaced single-path ante with dual routing:
  - Ethereum extension tx -> EVM ante chain.
  - Cosmos tx + DynamicFee extension -> Cosmos ante path.
- EVM path uses`NewEVMMonoDecorator` + pending tx listener decorator.
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

- Ethereum transactions are represented on-chain as`MsgEthereumTx` messages carried inside normal Cosmos SDK transactions.
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
  - EVM-aware`CheckTx` handler
  - EVM-aware`PrepareProposal` signer extraction adapter
- Added async broadcast queue (`evmTxBroadcastDispatcher`) to decouple txpool promotion from CometBFT`CheckTx` submission, preventing a mutex re-entry deadlock (see Architecture Strengths below).
- Added`RegisterTxService` override in`app/evm_runtime.go` to capture the`client.Context` with the local CometBFT client that cosmos/evm creates after CometBFT starts — the default`SetClientCtx` call happens before CometBFT starts and only provides an HTTP client.
- Added`Close()` override to stop the broadcast worker before runtime shutdown.
- Added configurable`[lumera.evm-mempool]` section in`app.toml` with`broadcast-debug` toggle for detailed async broadcast logging.
- Enabled app-side mempool by default in app config (`max_txs=10000`).

Benefits/new features:

- Pending tx support and txpool behavior aligned with Cosmos EVM.
- Better Ethereum tx ordering/replacement/nonce-gap behavior.
- EVM-aware proposal building for mixed workloads.
- Deadlock-free nonce-gap promotion: promoted EVM txs are enqueued and broadcast by a single background worker, never blocking the mempool`Insert()` call stack.
- Debug logging for broadcast queue processing gated behind`app.toml` config flag.

### 5) JSON-RPC and indexer defaults

Files:

- `cmd/lumera/cmd/config.go`
- `cmd/lumera/cmd/commands.go`
- `cmd/lumera/cmd/root.go`
- `app/evm_jsonrpc_ratelimit.go`

Changes:

- Enabled JSON-RPC and indexer by default in app config.
- Root command includes EVM server command wiring.
- Start command exposes JSON-RPC flags via cosmos/evm server integration.
- **Per-IP JSON-RPC rate limiting** — Optional reverse proxy (`app/evm_jsonrpc_ratelimit.go`) sits in front of the cosmos/evm JSON-RPC server. Configured via`app.toml` under`[lumera.json-rpc-ratelimit]`:
  - `enable` — toggle (default:`false`)
  - `proxy-address` — listen address (default:`0.0.0.0:8547`)
  - `requests-per-second` — sustained rate per IP (default:`50`)
  - `burst` — token bucket capacity per IP (default:`100`)
  - `entry-ttl` — inactivity expiry for per-IP state (default:`5m`)
  - Rate-limited responses return HTTP 429 with JSON-RPC error code`-32005`.
  - Stale per-IP entries are garbage-collected every 60 seconds.

Benefits/new features:

- Out-of-the-box`eth_*` RPC availability without manual config.
- Out-of-the-box receipt/tx-by-hash/indexer functionality.
- Production-ready JSON-RPC rate limiting without external infrastructure.

### 6) Keyring and CLI defaults for Ethereum keys

Files:

- `cmd/lumera/cmd/root.go`
- `cmd/lumera/cmd/testnet.go`
- `testutil/accounts/accounts.go`
- `claiming_faucet/main.go`

Changes:

- Default CLI`--key-type` set to`eth_secp256k1`.
- Added`EthSecp256k1Option` to keyring initialization in CLI/testnet/helpers/faucet paths.
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

- Integrated`x/feemarket` and`x/precisebank` keepers/modules.
- Enabled dynamic base fee in default genesis with minimum gas price floor (`0.0005 ulume/gas`) and change denominator`16`.
- Added module ordering and permissions to include feemarket/precisebank correctly.

Benefits/new features:

- EIP-1559-style fee market behavior with spam protection via minimum gas price floor.
- 18-decimal extended-denom accounting bridged to bank module semantics.

### 10) Upgrades and store migration

Files:

- `app/upgrades/v1_20_0/upgrade.go`
- `app/upgrades/store_upgrade_manager.go`
- `app/upgrades/upgrades.go`

Changes:

- Added v1.20.0 store upgrades for:
  - feemarket
  - precisebank
  - vm
  - erc20
- Added post-migration finalization for skipped EVM module state:
  - Lumera EVM params + coin info
  - Lumera feemarket params
  - ERC20 default params (`EnableErc20=true`,`PermissionlessRegistration=false`) — permissionless registration disabled for security; token pair registration requires governance
  - ERC20 registration policy (mode=`allowlist`, provenance-bound base denom entries: uatom, uosmo, uusdc, inj — inert placeholders until governance binds IBC channels)
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
- `docs/openrpc/examples_overrides.json`
- `docs/openrpc/param_overrides.json`
- `docs/openrpc/type_overrides.json`
- `docs/openrpc/result_overrides.json`
- `Makefile`

Changes:

- Added runtime OpenRPC discovery namespace (`rpc`) with JSON-RPC method:
  - `rpc_discover`
- Added HTTP OpenRPC document endpoint:
  - `GET /openrpc.json` (and `HEAD`)
  - `POST /openrpc.json` proxies JSON-RPC calls to the internal JSON-RPC server, enabling OpenRPC Playground "Try It" from the REST API port
  - Automatic `rpc.discover` → `rpc_discover` method name rewriting for playground compatibility
- Added browser CORS/preflight support for OpenRPC HTTP endpoint:
  - CORS origins controlled by `[json-rpc] ws-origins` (empty/`*` = allow all)
  - `Access-Control-Allow-Methods: GET, HEAD, POST, OPTIONS`
  - `Access-Control-Allow-Headers: Content-Type`
  - `OPTIONS /openrpc.json -> 204`
- Dynamic `servers[0].url` rewriting based on the configured JSON-RPC address, so the playground discovers the correct execution endpoint
- Improved generated example shape for strict OpenRPC tooling compatibility:
  - `examples[*].params` is always present (empty array when no params).
  - `examples[*].result.value` is always present (including explicit `null`).
- OpenRPC generator now expands struct parameters into JSON Schema `properties` with per-field types, patterns, and descriptions (e.g. `TransactionArgs` shows all 18 fields with correct Ethereum type schemas)
- Well-known Ethereum types (`common.Address`, `common.Hash`, `hexutil.Big`, `hexutil.Bytes`, etc.) mapped to correct JSON-RPC string representations with validation patterns
- OpenRPC spec version derived from `go.mod` at build time via `runtime/debug.ReadBuildInfo()` — no hardcoded version string
- Embedded spec is gzip-compressed in the binary (315 KB → 20 KB, 93% reduction); decompressed once at startup
- Added OpenRPC generation into build dependency chain:
  - `build/lumerad` and `build-debug/lumerad` depend on `app/openrpc/openrpc.json.gz`.
  - `openrpc` target generates `docs/openrpc.json` and compresses to `app/openrpc/openrpc.json.gz`.

Benefits/new features:

- Wallet/tooling clients can discover method catalogs consistently from the running node.
- OpenRPC playground/browser clients can fetch the spec cross-origin without manual proxy setup.
- Generated docs and embedded docs stay synchronized with built binaries, reducing stale-spec deployments.
