package keeper

import (
	"github.com/LumeraProtocol/lumera/x/supernode/types"
)

var _ types.QueryServer = Keeper{}
