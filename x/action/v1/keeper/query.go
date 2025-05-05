package keeper

import (
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
)

var _ types.QueryServer = &Keeper{}
