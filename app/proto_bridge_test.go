package app

import (
	"testing"

	stdproto "github.com/golang/protobuf/proto"
)

func requireEnumValue(t *testing.T, enumName, key string, expected int32) {
	t.Helper()
	valueMap := stdproto.EnumValueMap(enumName)
	if valueMap == nil {
		t.Fatalf("%s enum not registered", enumName)
	}
	if valueMap[key] != expected {
		t.Fatalf("unexpected %s value for %s: got %d want %d", enumName, key, valueMap[key], expected)
	}
}

// TestProtoBridgeRegistersEVMEnums verifies enum bridge registration for Cosmos
// EVM generated enum types used by grpc-gateway/proto-v1 resolution paths.
func TestProtoBridgeRegistersEVMEnums(t *testing.T) {
	requireEnumValue(t, "cosmos.evm.vm.v1.AccessType", "ACCESS_TYPE_PERMISSIONED", 2)
	requireEnumValue(t, "cosmos.evm.erc20.v1.Owner", "OWNER_EXTERNAL", 2)
}

// TestProtoBridgeRegistersCosmosSDKEnums verifies key Cosmos SDK enum mappings
// used by grpc-gateway/proto-v1 enum resolution paths.
func TestProtoBridgeRegistersCosmosSDKEnums(t *testing.T) {
	requireEnumValue(t, "cosmos.gov.v1beta1.ProposalStatus", "PROPOSAL_STATUS_PASSED", 3)
	requireEnumValue(t, "cosmos.gov.v1beta1.VoteOption", "VOTE_OPTION_YES", 1)
	requireEnumValue(t, "cosmos.gov.v1.ProposalStatus", "PROPOSAL_STATUS_PASSED", 3)
	requireEnumValue(t, "cosmos.gov.v1.VoteOption", "VOTE_OPTION_YES", 1)

	requireEnumValue(t, "cosmos.group.v1.VoteOption", "VOTE_OPTION_YES", 1)
	requireEnumValue(t, "cosmos.group.v1.ProposalStatus", "PROPOSAL_STATUS_ACCEPTED", 2)
	requireEnumValue(t, "cosmos.group.v1.ProposalExecutorResult", "PROPOSAL_EXECUTOR_RESULT_SUCCESS", 2)
	requireEnumValue(t, "cosmos.group.v1.Exec", "EXEC_TRY", 1)

	requireEnumValue(t, "cosmos.staking.v1beta1.BondStatus", "BOND_STATUS_BONDED", 3)
}
