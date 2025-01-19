package keeper

import (
	"github.com/LumeraProtocol/lumera/x/lumeraid/types"
)

var _ types.QueryServer = Keeper{}
