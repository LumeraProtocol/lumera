# Gen Activity Tool

`tests-gen-activity` generates realistic user-account activity against a live
Lumera devnet chain. It creates test accounts, funds them from a local keyring
funder, submits bank/staking/authz/feegrant/distribution activity, optionally
creates CASCADE actions through registered Supernodes, and stores everything in
a rerunnable JSON registry.

The tool is intended for devnet and test-chain use only. Generated account
mnemonics are written to the registry so later runs can reuse the same accounts.

## Build

Build the devnet test binaries from the repo root:

```bash
make devnet-tests-build
```

This creates:

```bash
devnet/bin/tests-gen-activity
```

When a devnet is built or rebuilt, `devnet/scripts/configure.sh` copies the
binary into the shared release directory:

```bash
/shared/release/tests-gen-activity
```

For an already-running devnet, copy the freshly built binary into a validator
container:

```bash
docker cp devnet/bin/tests-gen-activity \
  lumera-supernova_validator_1:/usr/local/bin/tests-gen-activity

docker exec lumera-supernova_validator_1 \
  chmod +x /usr/local/bin/tests-gen-activity
```

## Preconditions

Run the tool inside a validator container that has:

- Local `lumerad` RPC and gRPC access.
- A funded key in the local keyring.
- Access to a writable registry path, usually under `/shared/status/`.
- Registered Supernodes if CASCADE action generation is enabled.

Quick health check:

```bash
docker exec lumera-supernova_validator_1 sh -lc '
  lumerad status 2>/dev/null | jq -r "{height:.sync_info.latest_block_height, catching_up:.sync_info.catching_up}"
  lumerad q supernode list-supernodes --output json |
    jq -r "(.supernodes // []) as $sns | \"supernodes=\($sns|length)\""
'
```

Common funder on local devnet:

```bash
governance_key
```

Check the funder balance:

```bash
docker exec lumera-supernova_validator_1 sh -lc '
  addr=$(lumerad keys show governance_key -a --keyring-backend test)
  lumerad q bank balance "$addr" ulume --output json
'
```

## Registry

The registry is a gen-activity-owned JSON file. It records:

- Chain ID, funder key/address, current key style, and validator set.
- Generated account names, addresses, mnemonics, and key style.
- Funding status.
- Activity records: delegations, unbondings, redelegations, bank sends, authz
  grants, feegrants, withdraw-address changes, and CASCADE actions.

Recommended path:

```bash
/shared/status/gen-activity/accounts.json
```

Create the directory before the first run:

```bash
docker exec lumera-supernova_validator_1 \
  mkdir -p /shared/status/gen-activity
```

## Key Style

The tool detects the running `lumerad` version and chooses account key style:

| Chain runtime | Generated key style |
| --- | --- |
| Pre-EVM (`< v1.20.0`) | Cosmos `secp256k1`, coin type `118` |
| EVM-enabled (`>= v1.20.0`) | `eth_secp256k1`, coin type `60` |

Existing accounts keep the key style recorded in the registry, so a registry can
be reused across reruns without rewriting old account metadata.

## Basic Usage

The examples below run inside `lumera-supernova_validator_1`.

Shared arguments:

```bash
run_gen_activity() {
  tests-gen-activity \
    -bin lumerad \
    -rpc tcp://localhost:26657 \
    -grpc localhost:9090 \
    -chain-id lumera-devnet-1 \
    -funding-key governance_key \
    -accounts /shared/status/gen-activity/accounts.json \
    -account-prefix gen \
    -max-account-amount 10000000ulume \
    -funding-batch-size 10 \
    -parallelism 5 \
    "$@"
}
```

### Fresh Registry

Create up to 10 accounts, fund them, generate activity, and try CASCADE actions:

```bash
run_gen_activity -num-accounts 10
```

On a fresh registry, `-num-accounts` is the target total. If the registry already
has fewer accounts and neither rerun flag is set, the tool tops up the deficit.

### Add New Accounts

Always add `-num-accounts` more accounts to an existing registry:

```bash
run_gen_activity -add-accounts -num-accounts 10
```

Names continue from the highest existing index for the prefix, for example
`gen-0011`, `gen-0012`, and so on.

### Add Activity For Existing Accounts

Generate more activity for accounts already in the registry without adding new
accounts:

```bash
run_gen_activity -activity-existing -num-accounts 0
```

This mode skips account generation and funding for already-funded accounts.

### Account Activity Only

Disable CASCADE actions when testing only bank/staking/authz/feegrant activity:

```bash
run_gen_activity -activity-existing -num-accounts 0 -actions=false
```

### CASCADE Action Smoke Test

Create one pending action for an existing funded account:

```bash
run_gen_activity \
  -activity-existing \
  -num-accounts 0 \
  -actions=true \
  -action-states pending \
  -max-actions-per-run 1 \
  -action-readiness-timeout 30s
```

Action generation is non-fatal by default. If Supernodes are active but not yet
ready as CASCADE peers, the tool logs a warning and skips actions. Use
`-require-actions=true` when a missing action should fail the run.

## Useful Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-funding-key` | required | Local keyring key that funds generated accounts |
| `-accounts` | `devnet/tests/gen-activity/accounts.json` | Registry JSON path |
| `-num-accounts` | `10` | Fresh target count, or number to add with `-add-accounts` |
| `-add-accounts` | `false` | Add new accounts to an existing registry |
| `-activity-existing` | `false` | Generate more activity for existing funded accounts |
| `-max-account-amount` | `10000000ulume` | Upper bound for each generated account funding transfer |
| `-funding-batch-size` | `10` | Number of funder transfers to pipeline before waiting |
| `-parallelism` | `5` | Concurrent account workers; each account signs sequentially |
| `-actions` | `true` | Enable CASCADE action generation |
| `-require-actions` | `false` | Fail if action generation cannot run |
| `-action-states` | `pending,done,approved` | Target CASCADE states to create |
| `-max-actions-per-run` | `3` | Maximum CASCADE actions per run |
| `-action-readiness-timeout` | `180s` | Time to wait for action-capable Supernodes |

## Inspect Results

Summarize the registry:

```bash
docker exec lumera-supernova_validator_1 sh -lc '
  jq "{
    accounts:(.accounts|length),
    funded:([.accounts[]|select(.funded==true)]|length),
    mnemonics:([.accounts[]|select((.mnemonic//\"\") != \"\")]|length),
    validators:(.validators|length),
    actions:([.accounts[].actions[]?]|length),
    delegations:([.accounts[].delegations[]?]|length),
    unbondings:([.accounts[].unbondings[]?]|length),
    redelegations:([.accounts[].redelegations[]?]|length),
    bank_sends:([.accounts[].bank_sends[]?]|length),
    authz_grants:([.accounts[].authz_grants[]?]|length),
    feegrants:([.accounts[].feegrants[]?]|length),
    withdraw_addresses:([.accounts[].withdraw_addresses[]?]|length)
  }" /shared/status/gen-activity/accounts.json
'
```

List generated accounts:

```bash
docker exec lumera-supernova_validator_1 sh -lc '
  jq -r ".accounts[] |
    [.name,.address,.funded,.key_style,((.actions//[])|length)] | @tsv" \
    /shared/status/gen-activity/accounts.json
'
```

Check balances:

```bash
docker exec lumera-supernova_validator_1 sh -lc '
  for addr in $(jq -r ".accounts[].address" /shared/status/gen-activity/accounts.json); do
    printf "%s " "$addr"
    lumerad q bank balance "$addr" ulume --output json | jq -r ".balance.amount // \"0\""
  done
'
```

## Troubleshooting

### `no CASCADE-eligible supernodes ready`

Fresh devnets can report five ACTIVE Supernodes before they are ready as CASCADE
peers. The tool waits up to `-action-readiness-timeout` and then skips actions
unless `-require-actions=true`.

Options:

- Wait a few more blocks and rerun with `-activity-existing -num-accounts 0`.
- Use `-actions=false` when testing only account activity.
- Increase `-action-readiness-timeout`.

### `redelegation to this validator already in progress`

Redelegations have chain-level cooldown rules. On reruns, a planned redelegation
can still collide with an in-progress one. The tool treats per-activity failures
as non-fatal, logs the warning, and continues.

### Account sequence errors

Funding transfers are submitted from a single funder with explicit increasing
sequence numbers. Account activity runs in parallel across accounts, but each
account signs its own txs sequentially. If the chain is slow or a prior run was
interrupted, rerun the same command; the registry is saved after key generation,
after funding, after account activity, and after action activity.

### Registry parse or schema error

The tool will not overwrite an invalid registry. Move the broken file aside or
choose a new `-accounts` path.
