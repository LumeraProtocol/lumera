package keeper

import (
	"github.com/pastelnetwork/pastel/x/pastelid/types"
)

var _ types.QueryServer = Keeper{}
