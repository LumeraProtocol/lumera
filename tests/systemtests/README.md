# Testing

Test framework for system tests. 
Starts and interacts with a (multi node) blockchain in Go.
Supports:
* CLI
* Servers
* Events
* RPC

Uses:
* testify
* gjson
* sjson

Server and client side are executed on the host machine.

## Developer
### Test strategy
System tests cover the full stack via CLI and a running (multi node) network. They are more expensive (in terms of time/CPU) 
to run compared to unit or integration tests. 
Therefore, we focus on the **critical path** and do not cover every condition.

#### These tests USES BINARY from the go path. You can build a new one with `ignite chain build`

### Execute a single test
```sh
cd tests/systemtests
go test -tags=system_test -v . -test.run=TestSupernodeRegistrationFailures
```

### Execute all system tests
```sh
cd tests/systemtests
go test -tags=system_test -v . 
```

Test CLI parameters:
* `-verbose` verbose output
* `-rebuild` rebuild artifacts
* `-wait-time` duration - time to wait for chain events (default 30s)
* `-nodes-count` int - number of nodes in the cluster (default 4)
* `-block-time` duration - target block creation time (default 1s)
* `-binary` string - executable binary for server/client side (default "lumerad")
* `-bech32` string - bech32 prefix to be used with addresses (default "lumera")

# Port ranges
With *n* nodes:
* `26657` - `26657+n` - RPC
* `1317` - `1317+n` - API
* `9090` - `9090+n` - GRPC
* `16656` - `16656+n` - P2P

For example Node *3* listens on `26660` for RPC calls

## Resources

* [gjson query syntax](https://github.com/tidwall/gjson#path-syntax)

## Acknowledgments

The initial code was taken from wasmd.