package keeper

import (
	"github.com/pastelnetwork/pastel/x/claim/types"
)

var _ types.QueryServer = Keeper{}
