package app

import (
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	grouptypes "github.com/cosmos/cosmos-sdk/x/group"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	supernodetypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	"github.com/LumeraProtocol/lumera/internal/protobridge"
)

func init() {
	// Cosmos SDK enums used in query parameters.
	protobridge.RegisterEnum("cosmos.gov.v1beta1.ProposalStatus", govtypes.ProposalStatus_value)
	protobridge.RegisterEnum("cosmos.gov.v1beta1.VoteOption", govtypes.VoteOption_value)
	protobridge.RegisterEnum("cosmos.gov.v1.ProposalStatus", govtypesv1.ProposalStatus_value)
	protobridge.RegisterEnum("cosmos.gov.v1.VoteOption", govtypesv1.VoteOption_value)
	protobridge.RegisterEnum("cosmos.group.v1.VoteOption", grouptypes.VoteOption_value)
	protobridge.RegisterEnum("cosmos.group.v1.ProposalStatus", grouptypes.ProposalStatus_value)
	protobridge.RegisterEnum("cosmos.group.v1.ProposalExecutorResult", grouptypes.ProposalExecutorResult_value)
	protobridge.RegisterEnum("cosmos.group.v1.Exec", grouptypes.Exec_value)
	protobridge.RegisterEnum("cosmos.staking.v1beta1.BondStatus", stakingtypes.BondStatus_value)

	// Lumera module enums.
	protobridge.RegisterEnum("lumera.action.v1.ActionType", actiontypes.ActionType_value)
	protobridge.RegisterEnum("lumera.action.v1.ActionState", actiontypes.ActionState_value)
	protobridge.RegisterEnum("lumera.supernode.v1.SuperNodeState", supernodetypes.SuperNodeState_value)

	// Cosmos EVM module enums.
	protobridge.RegisterEnum("cosmos.evm.vm.v1.AccessType", vmtypes.AccessType_value)
	protobridge.RegisterEnum("cosmos.evm.erc20.v1.Owner", erc20types.Owner_value)
}
