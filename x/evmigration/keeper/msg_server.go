package keeper

import (
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// ClaimLegacyAccount is implemented in msg_server_claim_legacy.go.
// MigrateValidator is implemented in msg_server_migrate_validator.go.
