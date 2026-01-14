package keeper

import "github.com/LumeraProtocol/lumera/x/audit/v1/types"

type queryServer struct {
	types.UnimplementedQueryServer

	k Keeper
}

var _ types.QueryServer = queryServer{}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{
		UnimplementedQueryServer: types.UnimplementedQueryServer{},
		k:                        k,
	}
}

