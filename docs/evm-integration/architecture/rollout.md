# Rollout Plan: Lumera v1.20.0 EVM Upgrade and Account Migration

> **Notion**: [🚀 Rollout Plan: v1.20.0 EVM Upgrade and Account Migration](https://www.notion.so/341df11fee14815f929ce67421d1e6e0)

This document describes the rollout plan for upgrading Lumera to `v1.20.0` with Cosmos EVM integration and enabling legacy account migration via `x/evmigration`.

It covers:

- what has already been validated
- how we rehearse the upgrade on devnet
- how we test account migration on a live upgraded devnet
- how we promote to testnet and then mainnet
- what users, validators, exchanges, explorers, wallets, and supernode operators need to be told at each stage

## Goals

- upgrade Lumera networks to `v1.20.0` safely
- verify Cosmos and EVM functionality after upgrade
- verify legacy account migration from coin type 118 / `secp256k1` to coin type 60 / `eth_secp256k1`
- give validators and ecosystem integrators enough lead time to prepare
- make the user-facing impact predictable, especially the address / wallet change

## Non-goals

- introducing additional consensus changes beyond the `v1.20.0` EVM integration scope
- expanding feature scope during rollout
- supporting indefinite undocumented migration behavior; parameters and migration window should be explicit before mainnet

## Current Readiness Baseline

The implementation is already beyond the design phase. The current baseline before network rollout is:

- approximately `~397` unit tests across app wiring, ante, feemarket, precisebank, JSON-RPC, ERC20 policy, cross-runtime bridge, and `x/evmigration`
- approximately `~146` integration tests across contracts, JSON-RPC/indexer, mempool, fee market, IBC ERC20, precompiles, VM state, and `x/evmigration`
- multi-validator devnet tests for EVM behavior and cross-peer visibility
- dedicated devnet EVM migration tests with `7` operational modes and full upgrade rehearsal:
  - `prepare`
  - `estimate`
  - `migrate-validator`
  - `migrate`
  - `migrate-all`
  - `verify`
  - `cleanup`
- automated end-to-end devnet upgrade pipeline that starts from pre-EVM Lumera, upgrades to `v1.20.0`, migrates validators and accounts, and verifies no legacy references remain

For the full inventory and current counts, see [testing/tests.md](../testing/tests.md).

This means rollout work is now primarily operational: release qualification, staged upgrades, migration rehearsal, ecosystem communication, and soak periods.

## Rollout Summary

| Stage                                 | Approx. duration        | Objective                                                                                  |
| ------------------------------------- | ----------------------- | ------------------------------------------------------------------------------------------ |
| 0. Release candidate sign-off         | 3-5 days                | Re-run the full validation matrix on the release candidate and freeze scope                |
| 1. Devnet upgrade rehearsal           | 2-3 days                | Upgrade a live devnet chain to `v1.20.0` and confirm post-upgrade chain health           |
| 2. Devnet migration rehearsal + soak  | 5-7 days                | Exercise validator and user migration on upgraded devnet and tune docs / params            |
| 3. Testnet rollout                    | 1-2 weeks               | Upgrade a public network, let validators and integrators test against realistic conditions |
| 4. Mainnet readiness window           | 1 week                  | Final go/no-go, release notes, operator runbooks, public comms, governance scheduling      |
| 5. Mainnet rollout + migration window | Upgrade day + 2-8 weeks | Upgrade mainnet, monitor stability, and support account migration at scale                 |

## Supporting Guides

- [OpenRPC Discovery and Playground Guide](../guides/openrpc-playground.md) — OpenRPC discovery and interactive method testing
- [Testing Smart Contracts on Lumera with Remix IDE](../guides/remix-guide.md) — deploy and test a simple Solidity contract through MetaMask
- [Node Operator EVM Configuration Guide](../user-guides/node-evm-config-guide.md) — validator and RPC-node configuration checks
- [Mainnet Parameter Tuning Guide](../user-guides/tune-guide.md) — EVM parameter review and operational tuning
- [External Block Explorer Integration Plan](../guides/block-explorer.md) — block explorer rollout on testnet and mainnet
- [CosmWasm Cross-Runtime Bridge — Wasm Precompile & EVM Plugin](../precompiles/wasm-precompile.md) — bidirectional CosmWasm↔EVM bridge behavior and test targets

## Roles and Ownership

The rollout needs named role ownership even if the actual people are assigned later.

| Responsibility | Owner role | Notes |
| --- | --- | --- |
| stage go/no-go decision | Release lead | decides whether to promote from RC to devnet, devnet to testnet, and testnet to mainnet |
| governance proposal prep and timing | Governance owner | owns proposal content, deposit, timing, voting tracking, and contingency resubmission |
| validator coordination | Validator operations owner | owns validator comms, halt instructions, restart coordination, and readiness tracking |
| migration rehearsal and Portal flow | Migration owner | owns migration runbooks, Portal flow, Keplr / MetaMask migration tests |
| RPC / infra / explorer readiness | Infrastructure owner | owns RPC health, OpenRPC, rate limiting, block explorer rollout, and monitoring |
| wallet and ecosystem partner readiness | Ecosystem owner | owns chain registry, wallet partners, exchanges, custodians, and explorer contacts |
| public announcements and status updates | Communications owner | owns public announcement copy, cadence, and incident updates |
| incident command during upgrade day | Incident commander | single coordinator for halt / hold / resume instructions |
| migration-window support and triage | Support owner | owns inbound support flow, FAQ updates, and escalation during migration window |

## Communication Channels

Before testnet, Lumera should map each audience to a concrete channel, not just a message.

| Audience | Primary channel | Secondary channel | Cadence |
| --- | --- | --- | --- |
| validators | dedicated validator coordination channel | email / direct operator contact | pre-announcement, voting reminder, 24h reminder, live upgrade-day instructions |
| public users | website / blog / docs announcement | social channels / Discord / Telegram | initial announcement, 1 week reminder, 24h reminder, post-upgrade status |
| governance participants | governance forum / proposal page | public status channels | at proposal submission, during voting, at vote close |
| wallets / exchanges / explorers / custodians | direct partner email / shared ops thread | public docs | initial partner notice, follow-up before testnet, mainnet readiness reminder |
| internal incident responders | incident bridge / war room | backup out-of-band channel | continuous during upgrade and incidents |

The exact channel names should be finalized before testnet and copied into the operator runbooks.

## Rollout Prerequisites

These items come from the remaining roadmap work and should be treated as explicit rollout gates, not implied follow-up work.

| Item | Target stage | Priority | Gate |
| --- | --- | --- | --- |
| Fee market monitoring runbook | Stage 0 and Stage 4 | High | must exist before testnet promotion and be finalized before mainnet |
| Disaster recovery procedures for EVM state | Stage 0 and Stage 4 | Medium | must exist before testnet promotion and be operator-reviewed before mainnet |
| Load testing / performance benchmarks | Stage 0 and Stage 4 | Medium | baseline results required before testnet; final sign-off required before mainnet |
| External block explorer readiness | Stage 3 and Stage 4 | High | must be staged on testnet and have a production rollout decision before mainnet |
| Testnet faucet availability | Stage 3 | Medium | must be available before broad external testnet migration and contract testing, or an explicit manual funding alternative must be documented |
| Migration-proof expiry decision | Stage 0 and Stage 4 | High | must be explicitly decided before mainnet: implement a new proof format or document the accepted limitation with a finite migration window |

## Contingency Principles

Rollout should stop on meaningful bugs. Promotion from devnet to testnet to mainnet is conditional, not automatic.

### Severity bands

| Severity | Examples | Default response |
| --- | --- | --- |
| Critical | consensus failure, state corruption, incorrect migration, fund loss risk, validator safety risk | stop rollout immediately; do not promote |
| High | broken validator migration, broken user migration, startup instability, fee/accounting bug, major wallet or RPC incompatibility | pause the stage, fix, and rerun the stage exit criteria |
| Medium | Portal workflow bug, partial wallet issue with workaround, docs gap, monitoring gap | fix before promotion if operator- or user-impacting; otherwise track with owner and deadline |
| Low | wording, minor UI issues, non-blocking tooling papercuts | document and defer if needed |

### Default response flow

1. Reproduce the bug in the current stage environment.
2. Classify whether it affects:
   - consensus safety
   - migration correctness
   - validator operations
   - user funds
   - partner integrations
3. Freeze promotion to the next stage.
4. Assign an owner and retest scope.
5. Patch and rerun the relevant tests and rehearsals.
6. Update runbooks, docs, Portal behavior, and user messaging if instructions changed.

## Stage-by-Stage Contingency Plan

### RC sign-off

- if bugs are found, do not start devnet rollout
- cut a new RC and rerun the affected unit, integration, and devnet suites
- reset sign-off; partial approval from the failed RC does not carry forward

### Devnet

- if upgrade, store migration, startup, denom, fee, or RPC initialization is wrong, rebuild devnet from pre-upgrade state and rerun the entire upgrade rehearsal
- if migration behavior is wrong, rerun the devnet migration cycle from prepared legacy state:
  - `estimate`
  - `migrate-validator`
  - `migrate`
  - `verify`
- do not promote to testnet until devnet is clean again

### Testnet

- if a serious bug appears, pause mainnet scheduling immediately
- if upgrade behavior is affected, replay the full testnet upgrade
- if migration behavior is affected, rerun the migration soak after the fix
- if MetaMask, Keplr, Portal, explorer, or chain-definition behavior is affected, keep the rollout on testnet until those partner-facing flows are revalidated
- after a high-severity fix, require a fresh soak window rather than a spot check

### Mainnet

- if consensus or funds safety is at risk, coordinate an immediate validator halt and publish a short status update with exact operator instructions
- if migration is broken but the chain is otherwise safe, stop encouraging migrations and, if possible, disable or pause migration until a fix is ready
- if the issue is limited to Portal, MetaMask, Keplr, or chain-definition handling, keep the chain live if state is safe, publish a workaround, and patch the affected integration before reopening broad user flows

### Mainnet recovery posture

- take snapshots before the scheduled upgrade
- predefine the validator incident communication channel
- keep public status updates short, factual, and timestamped
- resume migration only after an explicit all-clear announcement

## Stage 0: Release Candidate Sign-off

### Release Scope

Before touching any live network, cut an RC for `v1.20.0` and re-run the full validation stack:

- unit tests
- integration tests
- system / multi-validator tests
- devnet EVM tests
- devnet EVM migration tests
- upgrade-preservation tests

### Release Additional Checks

- verify the upgrade handler for `v1.20.0` on a clean pre-EVM state snapshot
- verify `app.toml` migration for pre-EVM nodes
- verify JSON-RPC, OpenRPC, and indexer defaults on the RC binary using the [OpenRPC Discovery and Playground Guide](../guides/openrpc-playground.md)
- verify migration portal / CLI flows against the RC
- verify MetaMask connectivity and transaction flows against the RC
- verify Keplr-based migration flows against the RC
- deploy and test a simple Solidity contract against the RC using the [Remix guide](../guides/remix-guide.md)
- validate operator-facing config defaults against the [Node Operator EVM Configuration Guide](../user-guides/node-evm-config-guide.md)
- complete the first version of the fee market monitoring runbook and disaster recovery procedure for EVM state
- run baseline load and performance benchmarks for mixed Cosmos + EVM traffic
- decide whether migration-proof expiry will be implemented before mainnet or accepted as a documented limitation with a finite migration window
- verify release artifacts, checksums, build reproducibility, and operator install instructions

### Release Exit Criteria

- no open consensus, migration, or funds-safety issues
- no unresolved blocker in upgrade, fee market, RPC, or migration flows
- fee market monitoring runbook exists and is usable by operators
- disaster recovery procedure exists for upgrade-day and post-upgrade EVM-state incidents
- baseline performance benchmark results are recorded and reviewed
- release notes and operator notes drafted

### Release Communication

Audience: internal team, selected validators for early operational review, wallet / explorer / exchange partners.

Message to convey:

- `v1.20.0` is feature-complete and entering rollout qualification
- the major user-facing change is account migration due to coin type and key type change
- ecosystem partners should begin staging against the RC now

## Stage 1: Upgrade Lumera Devnet

### Devnet Upgrade Duration

`2-3 days`

### Devnet Upgrade Objective

Upgrade an existing devnet chain from pre-EVM Lumera to `v1.20.0` and confirm that the network restarts cleanly with EVM modules and store upgrades applied.

### Devnet Upgrade Execution

1. Start from the current pre-EVM devnet baseline.
2. Create realistic pre-upgrade state, including legacy accounts and validator activity.
3. Schedule the upgrade height and submit the upgrade proposal if governance is part of the devnet flow.
4. Halt the chain at the upgrade height.
5. Replace binaries and restart validators with `v1.20.0`.
6. Confirm post-upgrade health:
   - blocks resume
   - validators rejoin
   - stores load correctly
   - JSON-RPC is live
   - Cosmos txs still work
   - EVM txs work
   - feemarket base fee is non-zero
   - token / ERC20 registration state is correct

### Devnet Post-Upgrade Smoke Tests

- send Cosmos bank tx
- send EIP-1559 self-transfer
- validate OpenRPC discovery and Playground "Try It" requests using the [OpenRPC Discovery and Playground Guide](../guides/openrpc-playground.md)
- deploy and call a simple Solidity contract using the [Remix guide](../guides/remix-guide.md)
- run a bidirectional cross-runtime bridge smoke test using the [Wasm precompile guide](../precompiles/wasm-precompile.md):
  - EVM -> CosmWasm via the Wasm precompile
  - CosmWasm -> EVM via the wasm plugin path
- connect MetaMask to devnet and verify account, balance, and tx submission
- verify Keplr can still access the legacy account path needed for migration
- verify `eth_gasPrice`, `eth_chainId`, and `eth_getTransactionReceipt`
- verify cross-peer receipt visibility
- verify no unexpected denom / fee regression

### Devnet Performance Baseline

Before promoting beyond devnet, run a basic mixed-workload performance check:

- sustained EVM transaction flow for multiple consecutive blocks
- sustained mixed Cosmos + EVM transaction flow for multiple consecutive blocks
- migration traffic running at or near `max_migrations_per_block` alongside normal user traffic
- observation of:
  - block time stability
  - validator participation stability
  - mempool growth / drain behavior
  - base fee response under sustained congestion
  - RPC responsiveness under concurrent query load

The exact target numbers can be tuned by operators, but the key gate is that migration traffic must coexist with normal Cosmos and EVM activity without obvious degradation or proposer instability.

### Devnet Upgrade Exit Criteria

- all validators successfully upgraded
- no store migration errors
- no chain halt after restart
- no unexpected fee-denom, coin-info, or RPC failures

### Devnet Upgrade Communication

Audience: devnet users, internal QA, wallet / explorer partners.

Message to convey:

- devnet will halt briefly at the announced upgrade height
- after restart, EVM JSON-RPC and new wallet semantics are available
- the same mnemonic now derives a different default account under coin type 60
- existing legacy balances are still on the old address until migrated

What users should expect:

- temporary devnet downtime during the upgrade window
- post-upgrade need to test both the old legacy address and the new EVM-derived address
- some scripts using old default key assumptions may break until updated

## Stage 2: Devnet Account Migration Rehearsal and Soak

### Devnet Migration Duration

`5-7 days`

### Devnet Migration Objective

Validate the full migration lifecycle on an already upgraded live network and use devnet to finalize the operator and user runbooks.

### Devnet Migration Execution

- run `estimate` across all prepared legacy accounts
- run `migrate-validator` for validator operators first
- run `migrate` for regular accounts
- run `verify` to ensure legacy references are gone from migrated state
- repeat the cycle with additional edge cases if needed:
  - vesting accounts
  - withdraw-address chains
  - authz + feegrant overlaps
  - redelegation-heavy accounts
  - validator-supernode combinations

### Devnet Migration Soak Focus

- migration throughput and per-block rate limiting
- validator migration operational complexity
- wallet UX for deriving the new address and signing both sides of the migration
- MetaMask UX for the new EVM account after migration
- Keplr UX for selecting the legacy account and completing migration through the portal
- portal / CLI clarity
- support burden from common user confusion:
  - "my funds disappeared"
  - "why is my new address empty"
  - "which wallet path should I use now"

### Devnet Migration Exit Criteria

- validator migration works cleanly on live upgraded devnet
- regular account migration works at scale
- migration traffic coexists with normal Cosmos and EVM activity without obvious block-time or proposer instability
- verification shows no stale legacy references except legitimate historical records
- docs are updated for any confusing or error-prone step
- migration params are confirmed or adjusted for testnet

### Devnet Migration Communication

Audience: devnet users, validators, supernode operators, internal support.

Message to convey:

- migration is now being exercised on a live upgraded devnet
- users should explicitly test the migration portal or CLI using legacy accounts
- validator and supernode operators should rehearse their exact migration order and restart procedure

What users should expect:

- migration is atomic and fee-free
- the old address will become empty after successful migration
- the new address becomes the canonical address going forward
- Cosmos and EVM activity now converge on the new `eth_secp256k1` account

## Stage 3: Rollout to Testnet

### Testnet Rollout Duration

`1-2 weeks`

### Testnet Rollout Objective

Validate the upgrade and migration flow on a public network with external validators, wallets, explorers, and integrators before mainnet.

### Testnet Pre-Upgrade Preparation

`5-7 days before upgrade`

- publish testnet upgrade announcement with exact upgrade height
- submit the testnet software-upgrade governance proposal with enough time for the full voting period before the target height
- confirm the current on-chain governance parameters before submission:
  - minimum deposit requirement
  - voting period duration
  - quorum / threshold / veto rules
- track the voting period explicitly and only treat the upgrade as scheduled after the proposal passes
- define the contingency path if the proposal fails, is vetoed, or misses quorum:
  - push the target height
  - republish the timeline
  - resubmit a new proposal
- publish validator upgrade guide with links to the [Node Operator EVM Configuration Guide](../user-guides/node-evm-config-guide.md) and [Mainnet Parameter Tuning Guide](../user-guides/tune-guide.md)
- publish user migration guide and portal/CLI instructions
- confirm how external testers will obtain testnet funds:
  - preferred path: EVM-compatible faucet for the upgraded testnet
  - fallback path: documented manual distribution or partner pre-funding process
- if using the current faucet path referenced in the [Remix guide](../guides/remix-guide.md), verify that it works with the upgraded testnet account model and expected wallet flows
- update Portal before the testnet upgrade so it ships the new EVM testnet chain definition in its local JSON file; keep the current pre-EVM testnet definition in the public chain registry during the migration window, and only replace the registry definition after the migration window closes
- prepare the block explorer rollout for testnet using the [External Block Explorer Integration Plan](../guides/block-explorer.md)
- use testnet to validate whether block explorer rollout can be considered mainnet-ready or must trail mainnet launch as a staged follow-up
- ask wallets, explorers, RPC providers, and exchanges to point staging systems to upgraded testnet
- confirm snapshot / backup plan for all testnet validators

### Testnet Upgrade Execution

`upgrade day`

1. Submit and pass the testnet upgrade proposal.
2. Validators halt at the target height.
3. Validators switch to `v1.20.0`.
4. Confirm when the first post-upgrade snapshot and state-sync serve point will be published for new nodes joining the upgraded testnet.
5. Network resumes.
6. Run immediate post-upgrade smoke tests.
7. Enable and test migration flows.

### Testnet Soak Plan

`7-10 days after upgrade`

- validator operators perform `MsgMigrateValidator`
- selected users and internal QA migrate legacy accounts
- test MetaMask end-to-end:
  - add the upgraded testnet network
  - verify RPC connectivity, chain ID, balances, and EIP-1559 txs
  - verify the migrated account behaves correctly as the canonical EVM account
- test Keplr end-to-end:
  - verify legacy account access for migration
  - verify Portal + Keplr migration flow
  - verify post-migration Cosmos tx signing from the new `eth_secp256k1` account
- test OpenRPC in the browser and against the API endpoint using the [OpenRPC Discovery and Playground Guide](../guides/openrpc-playground.md)
- deploy and test a simple Solidity contract on testnet using the [Remix guide](../guides/remix-guide.md)
- test the cross-runtime bridge on testnet using the [Wasm precompile guide](../precompiles/wasm-precompile.md):
  - EVM -> CosmWasm smoke flow
  - CosmWasm -> EVM smoke flow
- verify that external testers can obtain testnet funds through the chosen faucet or documented fallback distribution path
- wallet teams verify coin type 60 defaults and `eth_secp256k1` support
- explorer / indexer teams verify receipts, logs, ERC20 views, and address handling
- stage block explorer integration on testnet following the [External Block Explorer Integration Plan](../guides/block-explorer.md)
- record testnet load and performance results under mixed Cosmos + EVM activity
- exchange / custody partners verify deposit and withdrawal expectations
- keep the new EVM-enabled testnet chain definition in the Portal local JSON file during the migration window
- update the public testnet chain registry definition only after the migration window closes and the new account model is the only supported path
- track support issues and document the most common failure modes

### Testnet Success Criteria

- stable blocks and validator participation for the soak period
- no unresolved consensus or state-corruption issues
- no major migration UX blocker
- faucet or alternative funding path is working well enough for external testers to complete wallet, migration, and contract flows
- monitoring runbook and disaster recovery procedure have been exercised by the operator team
- block explorer has been staged on testnet and its mainnet rollout plan is explicit
- post-upgrade snapshot / state-sync distribution timing is documented for integrators spinning up fresh nodes
- external integrators confirm readiness or provide bounded follow-ups

### Testnet Communication

Audience: public testnet participants, validators, wallets, explorers, exchanges, dApp partners.

Message to convey before upgrade:

- testnet will halt at the exact announced height and resume on `v1.20.0`
- the governance upgrade proposal has been submitted and validators / participants should vote during the voting period
- after the upgrade, EVM RPC and account migration are available
- the default wallet path is now Ethereum-style coin type 60

Message to convey after upgrade:

- if you import the same mnemonic, you may see a different address than before
- balances on the old testnet address are not lost; they remain on the legacy address until migrated
- use the migration portal or CLI to move state from the legacy address to the new address

What users should expect:

- temporary testnet downtime during the upgrade window
- reconfiguration of local scripts, wallets, faucets, and bots
- possible indexer / explorer catch-up lag right after the restart

## Stage 4: Mainnet Readiness Window

### Mainnet Readiness Duration

`1 week`

### Mainnet Readiness Objective

Convert successful testnet results into a production-ready mainnet release package and communication plan.

### Mainnet Required Outputs

- final `v1.20.0` release artifacts and checksums
- final upgrade guide for validators
- final migration guide for users
- final validator / supernode migration runbook
- final MetaMask and Keplr test checklist
- final OpenRPC playground test checklist
- final simple-contract deployment checklist based on the [Remix guide](../guides/remix-guide.md)
- final RPC, wallet, explorer, and exchange integration notes
- final node-operator configuration checklist based on the [Node Operator EVM Configuration Guide](../user-guides/node-evm-config-guide.md)
- final EVM parameter review using the [Mainnet Parameter Tuning Guide](../user-guides/tune-guide.md)
- final block explorer rollout checklist based on the [External Block Explorer Integration Plan](../guides/block-explorer.md)
- final fee market monitoring runbook
- final disaster recovery procedure covering EVM state, upgrade incidents, and migration incidents
- final load-testing and performance benchmark report
- developer onboarding docs beyond Remix, including any Hardhat/Foundry guide, can be scheduled as a post-rollout follow-up
- final monitoring and incident-response checklist
- final governance proposal plan, including submission date, voting period, and target upgrade height
- final governance mechanics checklist:
  - proposal type
  - minimum deposit
  - voting period
  - quorum / threshold expectations
  - failed-proposal contingency plan
- final decision on migration-proof expiry:
  - implement a new proof format before mainnet, or
  - accept the current no-expiry proof format as a known limitation and pair it with a finite `migration_end_time`
- final choice for migration parameters:
  - `enable_migration`
  - `migration_end_time`
  - `max_migrations_per_block`
  - `max_validator_delegations`

### Mainnet Go/No-Go Checklist

- testnet soak completed without blocker issues
- all high-severity findings closed
- major validators have acknowledged readiness
- migration portal / CLI are ready
- fee market monitoring runbook is finalized and owned
- disaster recovery procedure is finalized and has been rehearsed at least once
- load-testing results are accepted as sufficient for rollout
- block explorer rollout status is explicit: ready at launch or intentionally staged after launch
- migration-proof expiry decision is explicit and documented
- support and comms staff have prepared FAQ and incident templates
- pre-upgrade snapshots and rollback documentation are prepared

### Mainnet Readiness Communication

Audience: whole ecosystem.

Message to convey:

- when the governance upgrade proposal will be submitted
- that the proposal is a software-upgrade proposal and where it will be tracked publicly
- what the minimum deposit and voting period requirements are at the time of submission
- how long the voting period is and when it ends
- exact mainnet upgrade height and expected maintenance window
- what changes for ordinary users, validators, supernode operators, exchanges, explorers, and wallets
- migration is required for users who want the same mnemonic to access the new canonical EVM-compatible account
- legacy balances do not disappear at upgrade time; they remain claimable through the migration flow
- exchanges and custodians should expect a 4-6 week lead time requirement for address-format changes and operational validation, so partner notification should begin no later than testnet rollout and preferably earlier

Recommended timing:

- initial announcement `2+ weeks` before mainnet upgrade
- exchange / custody outreach should start `4-6 weeks` before mainnet if those partners require address-format validation lead time
- governance proposal submission early enough to complete the full voting period before the upgrade height, with clear reminders during voting
- validator / partner reminder `1 week` before
- final reminder `24 hours` before
- live status updates on upgrade day

## Stage 5: Mainnet Rollout and Account Migration

### Mainnet Rollout Duration

`upgrade day` plus a `2-8 week` migration support window

### Mainnet Upgrade-Day Execution

1. Validators halt at the approved upgrade height.
2. Validators install and start `v1.20.0`.
3. Network resumes and core post-upgrade smoke tests run immediately.
4. Confirm:
   - block production
   - validator voting power recovery
   - Cosmos tx path healthy
   - EVM tx path healthy
   - JSON-RPC availability
   - feemarket behavior sane
   - no store-upgrade anomalies
5. Open the migration support window and publish the post-upgrade status update.
6. Publish when post-upgrade snapshots and state-sync serve points will be available for new nodes, indexers, and integrators.

### Mainnet Migration Sequence

Recommended order:

1. validators and validator-supernode operators
2. infrastructure partners: wallets, explorers, exchanges, custodians
3. general users

This keeps validator identity and ecosystem infrastructure stable before broad user migration volume begins.

### Mainnet Migration Window

During the migration window:

- users migrate with portal or CLI
- validators use `MsgMigrateValidator` where applicable
- the Portal serves the new mainnet chain definition from its local JSON file rather than relying on the public chain registry entry
- block explorer rollout proceeds in a staged way using the [External Block Explorer Integration Plan](../guides/block-explorer.md); explorer launch can trail the chain upgrade if needed without blocking migration
- if the current migration proof format remains without expiry, operate with a finite `migration_end_time` and treat any migration-window extension as an explicit governance and security decision
- support tracks stuck or confusing cases
- governance can adjust migration params if needed, but only in a controlled and publicly announced way

The migration window should be finite on mainnet. An explicit end time reduces long-tail risk and forces ecosystem cleanup.

### Migration Window Policy

The mainnet migration window length should be decided before testnet rollout and validated during testnet operations.

Decision criteria should include:

- percentage of total stake that has migrated
- percentage of legacy accounts with meaningful balances or delegations that has migrated
- number of validators / supernode operators still pending migration
- exchange, wallet, explorer, and custody readiness
- support-ticket volume and whether user confusion is still materially high
- whether the current proof format remains non-expiring and therefore increases the cost of keeping the window open

Under the current implementation, closing the migration window means future `MsgClaimLegacyAccount` and `MsgMigrateValidator` transactions are rejected via `enable_migration` / `migration_end_time`. It does not by itself confiscate, rewrite, or auto-migrate remaining legacy state.

That means the rollout must communicate an explicit policy for unmigrated accounts before mainnet:

- whether unmigrated legacy accounts remain usable only through the legacy Cosmos account path
- whether Lumera intends to keep them operational but unsupported for new EVM-native UX
- whether reopening migration later would require an explicit governance decision

Recommended policy:

- set a finite migration window before mainnet
- treat window extension as an explicit governance decision, not an automatic extension
- after closure, treat chain-assisted migration as disabled unless governance reopens it
- publish a clear statement that unmigrated accounts are not deleted by closing the window, but they also do not gain the new canonical EVM-compatible account mapping unless migration is reopened and completed
- state explicitly that reopening migration is a governance parameter change to `enable_migration` and/or `migration_end_time`, not a chain upgrade

Operational implications for unmigrated legacy accounts after the window closes:

- legacy balances remain on the legacy account and are not deleted or confiscated
- those balances should still be usable through the Cosmos transaction path
- users should still be able to transfer those balances manually to a new EVM-compatible account using a normal Cosmos bank send to the new account's `lumera1...` address
- legacy accounts do not become native EVM accounts, so they do not gain direct MetaMask / EVM-native usability just by remaining active
- existing delegations remain delegated under the legacy account and are not automatically moved
- those delegations cannot be manually "transferred" like bank balances
- without `x/evmigration`, the practical manual fallback for delegations is:
  - withdraw rewards from the legacy account
  - undelegate from the legacy account
  - wait through the normal unbonding period
  - transfer the released balance to the new EVM-compatible account
  - delegate again from the new account

This difference should be communicated clearly:

- missing the migration window is inconvenient but survivable for simple balance-only accounts
- missing the migration window is materially worse for accounts with active delegations, validator roles, supernode roles, or other address-bound state

After the migration window closes:

- update the public chain registry definition for mainnet to the new EVM-compatible account model
- remove the temporary Portal-only chain-definition override
- finalize the public block explorer rollout if it was staged or partially enabled during the migration window
- treat the post-migration account model as canonical across wallets, docs, and partner integrations

### Mainnet Monitoring Focus

- upgrade-day restart health
- migration counts and failures
- per-block migration rate-limit saturation
- EVM tx success and fee behavior
- RPC stability and rate-limiter behavior
- validator migration success
- explorer / indexer correctness
- support ticket volume and recurring confusion patterns

### Mainnet Rollout Communication

Audience: all users and partners.

Message to convey on upgrade day:

- mainnet is now running `v1.20.0`
- EVM support is live
- old and new addresses can differ for the same mnemonic because the default path is now coin type 60 with `eth_secp256k1`
- users do not need to panic if the new address is empty; funds are still on the legacy address until migrated

What users should expect after mainnet upgrade:

- wallet import behavior changes
- some third-party services may need time to finish EVM support and new address handling
- migration is one-time and irreversible
- after successful migration, the old address is empty and the new address becomes the canonical account for future use

## Stakeholder-Specific Messaging

| Audience               | What we must tell them                                                                                                         | What they should expect                                                                              |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| Validators             | Upgrade height, binary version, restart steps, snapshot requirement, validator migration runbook                               | short halt at upgrade height, immediate restart work, validator migration if still on legacy account |
| Supernode operators    | Whether they need validator migration or account migration, config updates, restart order                                      | config changes after migration, possible validator-first coordination                                |
| Wallets                | coin type 60 default, `eth_secp256k1` support, Bech32 and `0x` dual encoding expectations                                  | same mnemonic derives different default account than before                                          |
| Portal                 | temporary local JSON chain definition during migration window, migration UX, wallet connection behavior for MetaMask and Keplr | Portal may lead chain-definition changes before public chain registry updates                        |
| Explorers / indexers   | EVM RPC, receipts/logs, ERC20 views, dual address presentation, migration-state visibility                                     | post-upgrade reindexing / catch-up work                                                              |
| Exchanges / custodians | deposit / withdrawal address handling, downtime window, migration semantics                                                    | temporary maintenance window, address handling changes, staged enablement                            |
| End users              | same mnemonic may show a different address, migration is needed, balances are not lost                                         | confusion around empty new address unless messaging is explicit                                      |

## Recovery and Rollback Strategy

This upgrade has two materially different recovery regimes:

- before any migrations execute on the target network
- after one or more migrations execute and state has been atomically moved

### Pre-Migration Rollback Procedure

This is the simplest recovery path and should be preferred if a critical issue is found immediately after upgrade but before migrations begin.

1. Trigger the incident process through the predesignated validator coordination channels.
2. Issue a clear halt / do-not-restart instruction with timestamp and target height.
3. Validators stop nodes and preserve the failed post-upgrade data directory for forensics.
4. Confirm whether the network will restore from the pre-upgrade snapshot.
5. If rollback is approved:
   - restore the agreed pre-upgrade snapshot
   - reinstall the pre-upgrade binary
   - restart validators only after a coordinated resume instruction
6. Publish a public status update that the network has returned to the pre-upgrade state and that post-upgrade activity, if any, is not being preserved.

### Post-Migration State Recovery

Once `MsgClaimLegacyAccount` or `MsgMigrateValidator` has executed on the live network, a rollback to a pre-upgrade snapshot becomes destructive:

- migrated state disappears from the restored chain
- post-upgrade transactions disappear
- users and operators can be left with conflicting expectations about which chain state is canonical

Because of that, full rollback is no longer the default recovery tool once migrations have started.

The default strategy after migrations begin is forward-fix:

1. halt the network if consensus or funds safety requires it
2. preserve current state for analysis
3. reproduce and patch the issue on staging
4. validate the fix against migrated state
5. restart from the current canonical chain state with the fix applied

Reverting to a pre-upgrade snapshot after migrations have started should require an explicit extraordinary decision with clear public acknowledgment that post-upgrade state will be discarded.

### Binary Rollback Procedure

Before mainnet, operators need a concrete binary rollback runbook, not just a principle. It should include:

- where the pre-upgrade binaries and checksums are published
- how validators verify and reinstall the old binary
- which config files must be reverted or preserved
- when a snapshot restore is required versus when a binary-only restart is sufficient
- who announces the coordinated restart time

### Halt Coordination

Before mainnet, Lumera should designate:

- a primary validator incident channel
- a backup out-of-band channel
- the people authorized to declare halt, hold, restore, and resume instructions
- a short message template for each incident state

The mainnet plan should assume that if one-third or more voting power must halt together, this coordination happens out of band and must already be rehearsed during testnet.

## Recommended Practical Timeline

If no blocker appears, the practical sequence is:

| Week      | Plan                                                                  |
| --------- | --------------------------------------------------------------------- |
| Week 1    | RC sign-off, rerun full test matrix, freeze scope                     |
| Week 2    | Upgrade devnet, run devnet migration rehearsal, fix docs / params     |
| Week 3-4  | Upgrade testnet and soak with validators and ecosystem partners       |
| Week 5    | Mainnet readiness review, final communications, governance scheduling |
| Week 6    | Mainnet upgrade                                                       |
| Week 6-10 | Mainnet migration support window, monitoring, ecosystem cleanup       |

This should be treated as approximate. Any issue affecting consensus safety, migration correctness, validator operations, or user funds should extend the relevant soak period rather than compress it.

## Immediate Next Steps

1. Convert the stage exit criteria into a go/no-go checklist.
2. Prepare the public-facing upgrade notice and migration FAQ early, not after testnet.
3. Decide the intended mainnet migration window length before the testnet rollout, so testnet can validate the same operational assumptions.
