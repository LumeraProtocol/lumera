package openrpc

import (
	"sync"

	evmmempool "github.com/cosmos/evm/mempool"
	evmrpc "github.com/cosmos/evm/rpc"
	"github.com/cosmos/evm/rpc/stream"
	servertypes "github.com/cosmos/evm/server/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
)

var (
	registerOnce sync.Once
	registerErr  error
)

// RegisterJSONRPCNamespace registers the `rpc_discover` method in the JSON-RPC server.
func RegisterJSONRPCNamespace() error {
	registerOnce.Do(func() {
		registerErr = evmrpc.RegisterAPINamespace(Namespace, func(
			_ *server.Context,
			_ client.Context,
			_ *stream.RPCStream,
			_ bool,
			_ servertypes.EVMTxIndexer,
			_ *evmmempool.ExperimentalEVMMempool,
		) []gethrpc.API {
			return []gethrpc.API{
				{
					Namespace: Namespace,
					Version:   apiVersion,
					Service:   API{},
					Public:    true,
				},
			}
		})
	})

	return registerErr
}

// EnsureNamespaceEnabled appends the OpenRPC discovery namespace to a namespace list.
func EnsureNamespaceEnabled(namespaces []string) []string {
	for _, ns := range namespaces {
		if ns == Namespace {
			return namespaces
		}
	}
	return append(append([]string(nil), namespaces...), Namespace)
}
