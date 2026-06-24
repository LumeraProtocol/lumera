package app

import (
	"reflect"
	"testing"

	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	"github.com/stretchr/testify/require"
)

// TestIBCERC20MiddlewareWiring verifies app-level wiring for ERC20 IBC
// middleware across v1 and v2 transfer stacks.
func TestIBCERC20MiddlewareWiring(t *testing.T) {
	app := Setup(t)

	// ERC20 keeper must hold a transfer keeper reference for IBC callbacks.
	erc20KeeperField := reflect.ValueOf(app.Erc20Keeper).FieldByName("transferKeeper")
	require.True(t, erc20KeeperField.IsValid())
	require.False(t, erc20KeeperField.IsNil())

	// IBC-Go transfer keeper should be initialized and wrapped by callbacks stack.
	require.NotNil(t, app.TransferKeeper.GetICS4Wrapper())

	// IBC v1 transfer route exists (outermost middleware is PFM).
	v1TransferModule, ok := app.GetIBCKeeper().PortKeeper.Route(ibctransfertypes.ModuleName)
	require.True(t, ok)
	require.NotNil(t, v1TransferModule)

	// IBC v2 transfer route should be top-level ERC20 middleware wrapper.
	v2TransferModule := app.GetIBCKeeper().ChannelKeeperV2.Router.Route(ibctransfertypes.PortID)
	require.NotNil(t, v2TransferModule)

	v2Type := reflect.TypeOf(v2TransferModule)
	require.Equal(t, "IBCMiddleware", v2Type.Name())
	require.Contains(t, v2Type.PkgPath(), "github.com/cosmos/evm/x/erc20/v2")
}
