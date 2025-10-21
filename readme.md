# Lumera
[![Release Workflow](https://github.com/LumeraProtocol/lumera/actions/workflows/release.yml/badge.svg)](https://github.com/LumeraProtocol/lumera/actions/workflows/release.yml)

# lumera
**lumera** is a blockchain built using Cosmos SDK and Tendermint and created with [Ignite CLI](https://ignite.com/cli).

## Get started

### Install Ignite CLI

```bash
curl https://get.ignite.com/cli! | bash
```


### Build

```bash
ignite chain build
```

> **Note:** You can still build directly with go: `go build cmd`, but it won't build protobuf files.

**Note2:** You might get error during build:
```
Cosmos SDK's version is: v0.50.12

✘ Cannot build app:                                                          
                                                                           
error while running command go mod tidy: go: cannot find "go1.24.1" in PATH
: exit status 1
```
Lumera project doesn't specify toolchain, but it seems `Ignite` sometime might still require it. Do this:
```cmd
go install golang.org/dl/go1.24.1@latest
go1.24.1 download
export GOTOOLCHAIN=auto
```


### Initialize

You only need to run this command once.
```bash
lumerad init my-node
```

### Get latest `genesis.json`

```bash
https://github.com/LumeraProtocol/lumera-networks
```

### Get seeds

```bash
https://github.com/LumeraProtocol/lumera-networks
```

### Start

```
lumerad start
```

## Learn more

- [Ignite CLI](https://ignite.com/cli)
- [Tutorials](https://docs.ignite.com/guide)
- [Ignite CLI docs](https://docs.ignite.com)
- [Cosmos SDK docs](https://docs.cosmos.network)
- [Developer Chat](https://discord.gg/ignite)
