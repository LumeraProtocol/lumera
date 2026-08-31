# Devnet Makefile Commands

All targets are declared in `Makefile.devnet` and exposed through the root `Makefile`. They control the Docker-based devnet end-to-end.

## Build targets

| Command | Description |
| --- | --- |
| `make devnet-build` | Build `lumerad`, copy binaries into `/tmp/<chain-id>/shared/release/`, and run the generators with the active `config.json`/`validators.json`. Accepts `DEVNET_BIN_DIR`, `CONFIG_JSON`, `VALIDATORS_JSON` overrides. |
| `make devnet-build-default` | Run `devnet-build` with the repository default config, validators, genesis template, and claims CSV. |
| `make devnet-build-172` | Build using the `devnet/bin-v1.7.2` bundle and default configs to reproduce the v1.7.2 network. |
| `make devnet-build-191` | Build using the `devnet/bin-v1.9.1` bundle. |
| `make devnet-build-1111` | Build using the `devnet/bin-v1.11.1` bundle. |
| `make devnet-stage-external-version VERSION=v1.12.0` | Stage a devnet locally from `devnet/bin-<VERSION>/` and an external genesis. Skips local Docker image build, disables Hermes through a staged config copy, and does not require a claims CSV unless `EXTERNAL_CLAIMS_FILE` is provided. |
| `make devnet-new-remote-version VERSION=v1.12.0` | Stage locally from downloaded binaries, sync runtime files to `REMOTE_DEVNET_HOST`, and start the devnet there with Docker Compose. |
| `make devnet-tests-build` | Build devnet test binaries (`tests_validator`, `tests_hermes`, `tests_evmigration`) into `devnet/bin/`. |
| `make devnet-tests-lep6` | Run the LEP-6 storage-truth chain-side e2e tests against a running devnet. Requires `make devnet-up-detach`; uses existing registered supernodes or bootstraps validator-owned supernodes from key-resolvable devnet validator accounts when fewer than three are registered. |

## Lifecycle targets

| Command | Description |
| --- | --- |
| `make devnet-up` | Start Docker Compose in the foreground with `START_MODE=auto` so logs stream to the terminal. |
| `make devnet-up-detach` | Start Docker Compose in the background (`docker compose up -d`). |
| `make devnet-down` | Stop the stack and remove containers (`docker compose down --remove-orphans`). |
| `make devnet-stop` | Gracefully stop containers without removing them. |
| `make devnet-start` | Start previously stopped containers with `START_MODE=run`. |
| `make devnet-clean` | Remove `/tmp/<chain-id>/shared`, validator data folders, Hermes volumes, and the generated `docker-compose.yml`. |
| `make devnet-new` | Convenience target: `devnet-down` + `devnet-clean` + `devnet-build-default`. Full teardown and rebuild. |
| `make devnet-new-172` | Clean and rebuild the network using the v1.7.2 binary bundle, then start it. |
| `make devnet-new-191` | Clean and rebuild using v1.9.1 bundle. |
| `make devnet-new-1111` | Clean and rebuild using v1.11.1 bundle. |
| `make devnet-reset` | Clear each validator's `genesis.json` and `priv_validator_key.json`, then restart to rebuild gentx. Preserves other state. |

## Upgrade targets

| Command | Description |
| --- | --- |
| `make devnet-upgrade` | Rebuild binaries (if `DEVNET_BUILD_LUMERA=1`), stop containers, refresh `/shared/release/`, and rerun `configure.sh`. |
| `make devnet-upgrade-binaries` | Copy freshly built `lumerad` and `libwasmvm` into running containers through `devnet/scripts/upgrade-binaries.sh`. |
| `make devnet-upgrade-binaries-default` | Upgrade binaries using the default build output (`build/lumerad`). |
| `make devnet-upgrade-180` | Execute `devnet/scripts/upgrade.sh` for the v1.8.0 release bundle. |
| `make devnet-upgrade-191` | Execute `devnet/scripts/upgrade.sh` for v1.9.1. |
| `make devnet-upgrade-1100` | Execute upgrade for v1.10.0. |
| `make devnet-upgrade-1101` | Execute upgrade for v1.10.1. |
| `make devnet-upgrade-1111` | Execute upgrade for v1.11.1. |
| `make devnet-upgrade-1200` | Execute upgrade for v1.20.0 (EVM upgrade). |
| `make devnet-evm-upgrade` | Full EVM upgrade pipeline: build v1.20.0 binary, submit governance proposal, vote, wait for upgrade height, swap binaries. See [upgrade-testing.md](upgrade-testing.md). |

## Binary management

| Command | Description |
| --- | --- |
| `make devnet-refresh-bin` | Copy `build/lumerad` and `build/libwasmvm.x86_64.so` into `devnet/bin/`. Run after `make build`. |
| `make devnet-download-binaries` | Download versioned binaries from GitHub releases using `download-binaries.sh`. |
| `make devnet-update-scripts` | Copy updated scripts (`start.sh`, `validator-setup.sh`, `supernode-setup.sh`, `lumera-uploader-setup.sh`, plus Hermes scripts) into running containers. |
| `make devnet-deploy-tar` | Package dockerfile, compose file, binaries, configs, claims, and optional genesis into `devnet-deploy.tar.gz` for distribution. |

## EVM migration test targets

These targets run the `tests_evmigration` binary inside the devnet. See [../evm-integration/evmigration/devnet-tests.md](../evm-integration/evmigration/devnet-tests.md) for full documentation.

### Sequential mode

| Command | Description |
| --- | --- |
| `make devnet-evmigration-prepare` | Create legacy accounts and on-chain activity (pre-EVM upgrade) |
| `make devnet-evmigration-estimate` | Estimate migration gas costs |
| `make devnet-evmigration-migrate` | Run `MsgClaimLegacyAccount` for all legacy accounts |
| `make devnet-evmigration-migrate-validator` | Run `MsgMigrateValidator` for validators |
| `make devnet-evmigration-verify` | Verify all state was migrated correctly |
| `make devnet-evmigration-cleanup` | Clean up migration test artifacts |

### Parallel mode

Parallel variants (`devnet-evmigrationp-*`) run the same operations but use concurrent workers for faster execution:

| Command | Description |
| --- | --- |
| `make devnet-evmigrationp-prepare` | Parallel account preparation |
| `make devnet-evmigrationp-estimate` | Parallel gas estimation |
| `make devnet-evmigrationp-migrate` | Parallel account migration |
| `make devnet-evmigrationp-migrate-validator` | Parallel validator migration |
| `make devnet-evmigrationp-migrate-all` | Migrate all accounts + validators in one step |
| `make devnet-evmigrationp-verify` | Parallel verification |
| `make devnet-evmigrationp-cleanup` | Parallel cleanup |
| `make devnet-evmigration-sync-bin` | Sync the `tests_evmigration` binary to the Hermes container |

## Common environment overrides

| Variable | Default | Description |
| --- | --- | --- |
| `DEVNET_BIN_DIR` | `devnet/bin` | Directory containing binaries to copy into the devnet |
| `DEVNET_BUILD_LUMERA` | `1` | Set to `0` to skip building lumerad during `devnet-build` |
| `DEVNET_BUILD_TESTS` | `1` | Set to `0` to skip building devnet test helper binaries during `devnet-build` |
| `DEVNET_DOCKER_BUILD` | `1` | Set to `0` to stage config and compose files without running `docker compose build` |
| `CONFIG_JSON` | `devnet/default-config/config.json` | Path to chain config |
| `VALIDATORS_JSON` | `devnet/default-config/validators.json` | Path to validator specs |
| `EXTERNAL_GENESIS_FILE` | (none) | Path to pre-existing genesis to extend |
| `EXTERNAL_CLAIMS_FILE` | (none) | Optional path to claims CSV. If unset, no `claims.csv` is staged. |
| `REMOTE_DEVNET_HOST` | `lumera-devnet` | SSH host used by `devnet-new-remote-version` |
| `REMOTE_DEVNET_DIR` | `lumera-devnet` | Remote directory that receives the Docker Compose runtime files |
| `REMOTE_DEVNET_STAGE_DIR` | `build/devnet-remote-stage/lumera-devnet-1` | Local workspace-owned staging directory used before syncing to the remote host |
| `REMOTE_EXTERNAL_GENESIS_FILE` | `~/external-genesis/lumera-devnet-1/genesis.json` | Default external genesis used by `devnet-stage-external-version` when `EXTERNAL_GENESIS_FILE` is unset |
| `NOCACHE` | (unset) | Set to `1` to force Docker rebuild without cache |

## Typical workflows

### Fresh devnet from scratch

```bash
make devnet-new
```

### Rebuild after code change

```bash
make build                  # Build lumerad
make devnet-refresh-bin     # Copy into devnet/bin/
make devnet-upgrade-binaries-default  # Swap into running containers
```

### Test a software upgrade

```bash
# Start on old version
make devnet-new-1111

# Upgrade to new version
make devnet-upgrade-1200
```

### Run EVM migration tests

```bash
# 1. Start on pre-EVM version
make devnet-new-1111

# 2. Prepare test state
make devnet-evmigration-prepare

# 3. Upgrade to EVM version
make devnet-evm-upgrade

# 4. Run migration + verify
make devnet-evmigrationp-migrate-all
make devnet-evmigrationp-verify
```
