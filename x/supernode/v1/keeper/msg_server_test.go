package keeper_test

import (
	"context"
	"testing"

	keeper2 "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	"github.com/stretchr/testify/require"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
)

func setupMsgServer(t testing.TB) (keeper2.Keeper, types.MsgServer, context.Context) {
	k, ctx := keepertest.SupernodeKeeper(t)
	return k, keeper2.NewMsgServerImpl(k), ctx
}

func TestMsgServer(t *testing.T) {
	k, ms, ctx := setupMsgServer(t)
	require.NotNil(t, ms)
	require.NotNil(t, ctx)
	require.NotEmpty(t, k)
}
