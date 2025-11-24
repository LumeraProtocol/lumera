package v1_8_0

import (
	storetypes "cosmossdk.io/store/types"
	pfmtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward/types"
)

// UpgradeName is the on-chain name used for this upgrade.
const UpgradeName = "v1.8.0"

var StoreUpgrades = storetypes.StoreUpgrades{
	Added: []string{
		pfmtypes.StoreKey, // added Packet Forwarding Middleware (PFM) store key
	},
	Deleted: []string{
		"nft", // deleted NFT module store key
	},
}
