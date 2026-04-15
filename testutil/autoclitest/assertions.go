package autoclitest

import (
	"testing"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func AssertServiceMethodsCovered(t *testing.T, desc grpc.ServiceDesc, opts []*autocliv1.RpcCommandOptions, skipped ...string) {
	t.Helper()

	registered := make(map[string]*autocliv1.RpcCommandOptions, len(opts))
	for _, opt := range opts {
		method := opt.GetRpcMethod()
		require.NotEmpty(t, method, "autocli entry should declare an RPC method")
		require.NotContains(t, registered, method, "duplicate AutoCLI entry for %s", method)
		registered[method] = opt
	}

	skip := make(map[string]struct{}, len(skipped))
	for _, method := range skipped {
		skip[method] = struct{}{}
	}

	for _, method := range desc.Methods {
		if _, ok := skip[method.MethodName]; ok {
			continue
		}

		opt, ok := registered[method.MethodName]
		require.True(t, ok, "missing AutoCLI entry for RPC %s", method.MethodName)
		if !opt.GetSkip() {
			require.NotEmpty(t, opt.GetUse(), "AutoCLI entry for %s should define a Use string", method.MethodName)
		}
	}
}
