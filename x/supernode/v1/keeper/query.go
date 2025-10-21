package keeper

import (
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	
	k types.SupernodeKeeper
}

var _ types.QueryServer = queryServer{}

// NewQueryServerImpl returns an implementation of the QueryServer interface
// for the provided Keeper.
func NewQueryServerImpl(k types.SupernodeKeeper) types.QueryServer {
	return queryServer{
		UnimplementedQueryServer: types.UnimplementedQueryServer{},
		k: k,
	}
}
