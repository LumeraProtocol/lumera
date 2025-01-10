package keeper

import (
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

var _ types.QueryServer = Keeper{}
