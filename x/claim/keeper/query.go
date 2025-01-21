package keeper

import (
	"github.com/LumeraProtocol/lumera/x/claim/types"
)

var _ types.QueryServer = Keeper{}
