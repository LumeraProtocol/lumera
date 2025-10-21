package keeper

import (
	"github.com/LumeraProtocol/lumera/x/claim/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	
	k Keeper
}

var _ types.QueryServer = queryServer{}

// NewQueryServerImpl returns an implementation of the QueryServer interface
// for the provided Keeper.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{
		UnimplementedQueryServer: types.UnimplementedQueryServer{},
		k: k,
	}
}
