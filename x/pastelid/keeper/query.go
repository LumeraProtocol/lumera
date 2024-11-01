package keeper

import (
	"github.com/pastelnetwork/pasteld/x/pastelid/types"
)

var _ types.QueryServer = Keeper{}
