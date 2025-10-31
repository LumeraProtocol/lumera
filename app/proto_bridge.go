package app

import (
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	supernodetypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	"github.com/LumeraProtocol/lumera/internal/protobridge"
)

func init() {
	// Cosmos SDK enums used in query parameters.
	protobridge.RegisterEnum("cosmos.gov.v1beta1.ProposalStatus", govtypes.ProposalStatus_value)
	protobridge.RegisterEnum("cosmos.gov.v1beta1.VoteOption", govtypes.VoteOption_value)

	// Lumera module enums.
	protobridge.RegisterEnum("lumera.action.v1.ActionType", actiontypes.ActionType_value)
	protobridge.RegisterEnum("lumera.action.v1.ActionState", actiontypes.ActionState_value)
	protobridge.RegisterEnum("lumera.supernode.v1.SuperNodeState", supernodetypes.SuperNodeState_value)
}
