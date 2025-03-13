package keeper

import (
	"github.com/LumeraProtocol/lumera/x/action/types"
)

var _ types.QueryServer = &Keeper{}
