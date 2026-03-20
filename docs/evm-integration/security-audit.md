# EVM Integration Security Audit

**Date:** 2026-03-20
**Auditor:** Codex static review
**Scope:** Lumera EVM app wiring, ante, mempool/broadcast, JSON-RPC exposure, static precompiles, ERC20 IBC registration policy, and `x/evmigration`

## Executive Summary

The EVM integration is materially stronger than a typical first Cosmos-EVM launch. The codebase already contains fixes for several classes of high-impact failures that commonly escape into production:

- EVM mempool re-entry deadlock mitigation via async broadcast worker
- ICS20 precompile store-key registration fix
- JSON-RPC namespace lockdown on mainnet
- Supernode precompile caller-binding fix
- Action precompile soft-rejection handling fix

The remaining risk is concentrated in three places:

1. public JSON-RPC rate limiting is easy to bypass with the current proxy topology
2. validator-migration gas bounding undercounts redelegations after the destination-side redelegation fix
3. ERC20 auto-registration allowlisting trusts base denoms without IBC provenance

I did not find evidence of an active critical auth bypass in the currently checked-in EVM entry points. The main launch blockers are operational and denial-of-service related rather than signature-validation failures.

## Method

This review was a code and documentation audit of the current repository state. It did not include:

- dynamic fuzzing
- external dependency audit of upstream `cosmos/evm`, IBC-Go, or geth
- infrastructure review of reverse proxies, firewalls, or validator deployment scripts

## Findings

### 1. High: JSON-RPC rate-limit proxy does not actually front the public JSON-RPC address

**Affected code**

- `cmd/lumera/cmd/commands.go:117-145`
- `app/evm_jsonrpc_ratelimit.go:111-149`
- `app/app.go:397-399`

**What happens**

At startup, `wrapJSONRPCAliasStartPreRun` rewrites `json-rpc.address` to an internal loopback address and remembers the original public address for the alias proxy. The alias proxy is then started on the original public address.

The rate-limit proxy, however, uses the rewritten internal `json-rpc.address` as its upstream and listens on its own separate `lumera.json-rpc-ratelimit.proxy-address`.

That means enabling the rate-limit proxy does **not** rate-limit the normal public JSON-RPC port. It creates an additional rate-limited port while leaving the main public alias port unrestricted.

**Impact**

- operators can believe public RPC is protected when it is not
- attackers can bypass the limiter by using the normal public JSON-RPC address instead of the alternate proxy port
- the main public RPC endpoint remains exposed to request floods, expensive trace calls if enabled, and subscription abuse

**Why this matters**

This is a security-control bypass caused by startup wiring, not by misconfigured nginx. The built-in limiter is currently an opt-in alternate endpoint, not an in-line control on the public endpoint.

**Recommendation**

- make the rate limiter wrap the public alias listener instead of exposing a second port
- or, when rate limiting is enabled, move the alias proxy behind the limiter and fail startup if both are configured inconsistently
- at minimum, document that operators must firewall the public alias port and only expose the rate-limited port

**Priority**

Blocker before advertising the built-in rate limiter as a public-RPC protection mechanism.

### 2. Medium: validator migration gas cap undercounts destination-side redelegations

**Affected code**

- `x/evmigration/keeper/msg_server_migrate_validator.go:46-69`
- `x/evmigration/keeper/migrate_validator.go:155-199`
- `x/evmigration/keeper/query.go:71-90`

**What happens**

`MsgMigrateValidator` is supposed to bound work using `MaxValidatorDelegations`. The pre-check counts:

- delegations to the validator
- unbonding delegations from the validator
- redelegations where the validator is the **source**

But the actual migration logic was correctly expanded to rewrite redelegations where the validator is either the **source or destination**.

So the gas-bounding pre-check and estimate query both undercount the real amount of work.

**Impact**

- a validator with many destination-side redelegations can pass the safety check unexpectedly
- migration transactions can consume materially more gas and state writes than governance intended
- `MigrationEstimate` can tell operators a migration is safe when the real execution set is larger

**Why this matters**

This is a classic post-fix invariant drift: execution logic was widened, but the safety bound was not widened with it.

**Recommendation**

- count redelegations where the validator appears as source or destination in both the pre-check and `MigrationEstimate`
- add a regression test where the validator has many destination-side redelegations but few source-side redelegations
- consider exposing a keeper helper dedicated to "all records touched by validator migration" so the bound and the executor share the same enumeration logic

**Priority**

Fix before relying on `MaxValidatorDelegations` as a DoS guardrail.

### 3. Medium: ERC20 allowlist is provenance-blind for base denoms, including default genesis entries

**Affected code**

- `app/evm_erc20_policy.go:41-49`
- `app/evm_erc20_policy.go:117-127`
- `app/evm_erc20_policy.go:285-293`

**What happens**

In allowlist mode, an IBC voucher is auto-registered as an ERC20 if either:

- its exact `ibc/...` denom hash is allowlisted, or
- its **base denom** is allowlisted

The base-denom path is explicitly channel-independent. The default genesis allowlist pre-approves:

- `uatom`
- `uosmo`
- `uusdc`

So any IBC asset arriving with one of those base denoms from any channel or path is eligible for auto-registration, even if its provenance is not the intended hub/chain/path.

**Impact**

- counterfeit or lookalike vouchers can gain first-class ERC20 UX simply by sharing a base denom
- users and integrators can confuse assets with different provenance but the same base symbol/denom
- a governance decision intended to approve one source of `uusdc` or `uatom` effectively approves all sources

**Why this matters**

IBC security is denomination-plus-provenance, not base denom alone. Collapsing trust to the base denom weakens asset admission policy.

**Recommendation**

- prefer exact `ibc/...` denom allowlisting for production
- if base-denom approval is retained, bind it to additional provenance such as source channel, client, or canonical trace
- reconsider shipping permissive default base-denom entries at genesis

**Priority**

Should be tightened before mainnet if the chain intends to present ERC20 auto-registration as an asset-trust control.

### 4. Low: migration proofs are domain-separated by message kind and addresses, but not by chain ID or expiry

**Affected code**

- `x/evmigration/keeper/verify.go:19-21`
- `x/evmigration/keeper/verify.go:40-44`
- `x/evmigration/keeper/verify.go:67-68`

**What happens**

The signed payload is:

`lumera-evm-migration:<kind>:<legacyAddr>:<newAddr>`

It does not include:

- chain ID
- genesis hash
- expiration time
- timeout height

**Impact**

- the same proof is replayable across Lumera environments or forks that share address formats and state ancestry
- signed migration intents do not expire

This is not a direct theft vector because the proof still binds funds to the intended `newAddr`, but it weakens domain separation and makes operational replay harder to reason about.

**Recommendation**

- include chain ID and a deadline in any future proof format
- if compatibility must be preserved, support a v2 proof alongside the current format and deprecate the old one for new migrations

## Strengths

The current implementation has several meaningful security-positive properties:

- EVM and Cosmos tx paths are explicitly separated in ante, reducing mixed-semantics footguns.
- `MsgEthereumTx` signer handling is wired through custom signer extraction instead of relying on SDK defaults.
- EVM mempool promotion is decoupled from synchronous `CheckTx`, preventing a consensus-halting mutex re-entry deadlock.
- Mainnet startup rejects dangerous JSON-RPC namespaces (`admin`, `debug`, `personal`).
- Custom precompiles generally bind authority to `contract.Caller()` rather than calldata-provided identities.
- `x/evmigration` requires proof from both the legacy key and the destination key, which prevents unilateral state capture.

## Hardening Recommendations

These are not all code bugs, but they are worth doing before or shortly after launch:

- Set a finite `migration_end_time` before mainnet. Open-ended migration windows increase long-tail operational risk.
- Treat JSON-RPC tracing as a privileged operator feature. Keep it disabled on public RPC unless traffic is tightly controlled.
- Add metrics for mempool queue depth, EVM broadcast failures, and rate-limit hits so operators can see attacks in progress.
- Add an integration test that verifies "rate-limit enabled" really constrains the public RPC port, not only the alternate proxy port.
- Add a validator-migration regression test for destination-only redelegation fan-in.
- Add policy tests around "same base denom, different IBC trace" to force an explicit trust decision.

## Conclusion

The EVM integration is close to mainnet-ready from a code-security perspective, and it is notably ahead of many first-wave Cosmos-EVM launches in defensive engineering. The biggest remaining issue is that one advertised protection, built-in JSON-RPC rate limiting, is not actually in the request path of the public RPC endpoint by default. After that, the next most important fixes are aligning validator-migration safety bounds with real execution and tightening ERC20 admission policy to respect IBC provenance.
