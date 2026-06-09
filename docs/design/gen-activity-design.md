# Devnet Gen Activity Design

## Goal

Build a standalone `tests-gen-activity` binary that generates realistic account
activity against an actual Lumera devnet chain. The tool should create and reuse
test user accounts, fund them from a local keyring funder, submit activity
transactions against the live validator and supernode set, and persist all
generated account metadata and activity in a rerunnable JSON registry.

## Non-Goals

- Do not depend on the controlled Docker devnet status directories.
- Do not create EVM migration-specific fixtures such as permanent-locked
  accounts, multisig migration accounts, migration records, or pre/post-upgrade
  modes.
- Do not require devnet chain resets between runs.
- Do not make action generation fatal by default when supernodes are
  temporarily unavailable.

## Package Layout

### `devnet/tests/common`

Create a shared helper package for logic currently embedded in
`devnet/tests/evmigration` but useful outside migration tests:

- CLI command execution with standard `lumerad` flags.
- Transaction submission, tx result polling, block waiting, and sequence retry
  helpers.
- Keyring helpers for list/show/add/delete and mnemonic import.
- Runtime key-style detection from the current `lumerad` version:
  - pre-EVM: Cosmos `secp256k1`, coin type `118`
  - EVM-enabled: `eth_secp256k1`, coin type `60`
- Account generation from random mnemonics.
- Validator, bank, staking, distribution, authz, feegrant, action, and
  supernode query helpers.
- Shared account identity types, activity record types, and activity tracking
  methods. Top-level registry envelopes remain tool-specific.
- SDK client helpers for bank funding and CASCADE action creation.

`devnet/tests/evmigration` should keep migration-specific orchestration and
fixtures, but use the shared package for common primitives where practical.

### `devnet/tests/gen-activity`

Create a new standalone command package built as `tests-gen-activity`. It owns
the real-devnet activity workflow, command-line flags, logging, and registry
update policy.

## Command-Line Interface

Initial flags:

```sh
tests-gen-activity \
  -bin=lumerad \
  -rpc=tcp://devnet-host:26657 \
  -grpc=devnet-host:9090 \
  -chain-id=lumera-devnet-1 \
  -home=/path/to/lumerad-home \
  -funding-key=funder \
  -accounts=devnet/tests/gen-activity/accounts.json \
  -num-accounts=10 \
  -max-account-amount=10000000ulume
```

Additional flags:

- `-keyring-backend=test`: local funder keyring backend.
- `-add-accounts=true`: add `-num-accounts` new users to an existing registry.
- `-activity-existing=true`: generate more activity for accounts already in the
  registry.
- `-actions=true`: include CASCADE action activity, enabled by default.
- `-require-actions=false`: fail the run if action activity cannot be created.
- `-max-actions-per-run=3`: cap action uploads/registrations per run.
- `-action-states=pending,done,approved`: target action states to generate.
- `-action-readiness-timeout=180s`: time to wait for usable active supernodes.
- `-funding-batch-size=10`: number of funder transfers to pipeline before
  waiting for chain inclusion.
- `-parallelism=5`: maximum concurrent per-account activity workers.
- `-dry-run=false`: print planned accounts/activity without submitting txs.

The funding signer is selected by `-funding-key` and must exist in the local
keyring. The registry records both the funding key name and its address.

## Registry Sharing And Compatibility

The gen-activity registry is a separate top-level envelope from evmigration's
existing `AccountsFile`. Do not change evmigration's on-disk envelope fields
such as `funder`, `validators`, or `accounts`, and do not replace
evmigration's `is_legacy` semantics with gen-activity `key_style`.

The shared package should share the stable inner layer:

- core account identity fields needed by both tools
- detailed per-account activity record types
- activity tracking and deduplication methods

The shared package should not force one top-level registry schema on both
tools. This keeps existing evmigration `accounts.json` files compatible while
avoiding two independent definitions for the activity records.

Compatibility note: evmigration keeps its migration-specific account fields and
legacy shorthand activity fields (`delegated_to`, `redelegated_to`, and similar)
for existing registry compatibility. Gen-activity uses the shared detailed
activity arrays and does not need those legacy scalar shorthand fields.

## Gen-Activity Registry Format

The gen-activity registry JSON is owned by this tool and is intentionally a
different envelope from evmigration's migration registry.

Top-level fields:

- `schema_version`
- `chain_id`
- `created_at`
- `updated_at`
- `funder_key`
- `funder_address`
- `key_style` (the key style detected for the *current* run)
- `validators`
- `accounts`

The top-level `key_style` and the per-account `key_style` are deliberately
distinct: the envelope records the style of the most recent run, while each
account records the style it was *created* with. They can differ when a registry
is reused across an EVM cutover — accounts generated before the cutover keep
their original (`secp256k1`/coin-118) style even though later runs detect the
EVM (`eth_secp256k1`/coin-60) style. Reconciliation (step 6) updates the
envelope but never rewrites an existing account's recorded style.

Each account record includes:

- `name`
- `mnemonic`
- `address`
- `pubkey_b64`
- `key_style`
- `has_balance`
- `funded`
- `created_at`
- `updated_at`
- activity flags and detailed activity arrays

Activity arrays track:

- bank sends
- staking delegations
- unbonding delegations
- redelegations
- withdraw-address changes
- authz grants sent and received
- feegrants sent and received
- CASCADE actions

The tool owns the registry file named by `-accounts`. Reruns update existing
account records by stable `name` and `address`, append new activity records,
and keep existing mnemonics. If the file cannot be parsed as this registry
schema, the command fails instead of rewriting it.

## Shared Type Boundary

Shared common types:

- `AccountIdentity`: `name`, `mnemonic`, `address`, `pubkey_b64`, and optional
  key-style metadata for tools that need it. Evmigration keeps `is_legacy` in
  its tool-specific account record for on-disk compatibility.
- `ActivityLog`: detailed arrays for bank sends, staking delegations,
  unbondings, redelegations, withdraw-address changes, authz grants, feegrants,
  and CASCADE actions.
- Individual activity item structs such as `DelegationActivity`,
  `RedelegationActivity`, `FeegrantActivity`, and `ActionActivity`.

Tool-specific types:

- evmigration keeps its current `AccountsFile` top-level envelope and
  migration-specific account fields.
- gen-activity defines its own `ActivityRegistry` top-level envelope with
  `schema_version`, `updated_at`, `funder_key`, `funder_address`, and
  `key_style`.
- gen-activity account records embed or compose the shared identity and activity
  log, then add funding timestamps and generated-activity metadata.

## Runtime Flow

1. Parse flags and initialize shared command context.
2. Detect key style from the current `lumerad` runtime.
3. Query chain validators with `lumerad query staking validators`.
4. Resolve the funding key address from the local keyring.
5. Load the registry if it exists; otherwise create a new one.
6. Reconcile registry metadata with the current chain ID, key style, funder, and
   validator snapshot.
7. Generate and import new user keys when requested.
8. Save the registry after key generation so interrupted runs can resume.
9. Fund unfunded accounts up to `-max-account-amount`.
10. Generate account activity for eligible existing and new accounts.
11. Save the registry after each major phase and at the end of the run.

When `-dry-run=true`, the run is side-effect free: steps 1–6 execute normally
(flags, key-style detection, validator and funder queries, registry load and
reconciliation are all read-only), then the tool prints the accounts it *would*
generate and the activity it *would* submit and exits. It does not import keys
into the keyring (step 7), fund accounts (step 9), submit activity txs
(step 10), or write the registry to disk (steps 8 and 11).

## Concurrency Model

Funding and user activity have different signing contention risks:

- The funding phase is signed by one account, so it is handled by a single
  funder batcher. No `-parallelism` worker signs as the funder.
- The funder batcher queries the funder's current account number and sequence,
  builds up to `-funding-batch-size` bank send transactions with increasing
  explicit sequence numbers, broadcasts them in a burst, then waits for chain
  inclusion before sending the next batch.
- The default `-funding-batch-size=10` targets the desired "ten transfers in
  one block" behavior while keeping sequence ownership centralized.
- If a batch hits an account-sequence mismatch, the batcher waits for the next
  block, refreshes the funder sequence, and retries only transfers that are not
  confirmed funded.
- After funding is complete, `-parallelism` applies to per-account activity.
  Each worker signs only with its assigned generated user account and runs that
  account's txs sequentially.
- Cross-account operations that sign from a peer account, such as grants from
  another generated account, are either scheduled under that peer's worker or
  run in a serial cross-account phase.

## Activity Generation

Activity should follow the existing devnet prepare mix, adjusted for a real
devnet chain and bounded by `-max-account-amount`.

Default activity:

- Fund each new account with a random amount from a safe range whose upper bound
  is `-max-account-amount`.
- Send small bank transfers between generated accounts.
- Delegate to one to three randomly selected real validators.
- Occasionally unbond from an existing delegation.
- Occasionally redelegate when the chain has at least two validators.
- Occasionally set a third-party withdraw address to another generated account.
- Create authz grants for `/cosmos.bank.v1beta1.MsgSend`.
- Create feegrants with small spend limits.
- Create CASCADE actions through `sdk-go`.

Action activity:

- Query/wait for ACTIVE supernodes with usable gateway status before attempting
  CASCADE activity.
- Create a bounded number of actions per run.
- Generate pending, done, and approved action states according to
  `-action-states`.
- Record action ID, type, price, expiration, state, metadata, supernodes, block
  height, and whether it was created through SDK.
- If supernodes are unavailable and `-require-actions=false`, log the skip and
  continue the rest of the activity. If `-require-actions=true`, fail the run.
- If supernodes are available but a requested target state cannot be reached
  before `-action-readiness-timeout`, record only the highest confirmed on-chain
  state for that action. With `-require-actions=false`, log the downgrade or
  skip and continue. With `-require-actions=true`, fail the run after updating
  the registry with any confirmed action state.
- Pending actions are deterministic once the request-action tx is included.
  Done and approved actions are best-effort because they depend on supernode
  upload/finalization behavior on the live devnet.

## Rerun Semantics

The tool supports two rerun patterns:

- **More activity:** load the registry and submit additional activity for
  existing accounts.
- **More accounts:** append `-num-accounts` new accounts, fund them, and include
  them in the activity mix.

Before submitting activity that may conflict on rerun, query the chain when a
cheap check exists:

- existing delegations
- existing unbonding delegations
- existing redelegations
- current withdraw address
- existing authz grants
- existing feegrant allowances

When a tx fails with a known duplicate or in-progress conflict, record the
existing state only if the chain query confirms it.

## Error Handling

- Missing or empty validator set is fatal.
- Missing funding key is fatal.
- Insufficient funder balance is fatal before account creation activity begins.
- Per-account activity failures are warnings unless all activity fails.
- Action generation failure is non-fatal by default and fatal only with
  `-require-actions=true`.
- Registry writes are atomic: write to a temporary file in the same directory,
  then rename.
- Tx submission retries account-sequence mismatches and waits for inclusion
  before dependent txs.

## Testing

Unit tests should cover:

- key-style detection from version strings
- account name allocation on fresh and existing registries
- registry load/update/save behavior
- evmigration registry envelope compatibility during shared-type refactors
- activity tracking deduplication
- coin parsing and max-account-amount validation
- funder batch planning with explicit increasing sequence numbers
- validator selection and activity planning
- action-state flag parsing
- action timeout handling that records only confirmed on-chain state

Integration smoke tests should cover:

- building `tests-gen-activity`
- dry-run registry planning
- live-chain execution against a local devnet when available

`devnet-tests-build` should build `tests-gen-activity` alongside
`tests_evmigration`.

## Open Implementation Notes

- Prefer moving common primitives in small slices so evmigration remains
  buildable after each refactor step.
- Keep migration-specific record fields in evmigration unless they are truly
  shared.
- Consider `devnet/tests/common` package names such as `chain`, `keys`,
  `registry`, and `activity` if one package becomes too large.
- Implement the funder batcher before enabling per-account parallel activity so
  funding sequence behavior is tested in isolation.
