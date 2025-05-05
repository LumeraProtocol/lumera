package keeper

import (
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

var _ types.QueryServer = Keeper{}
