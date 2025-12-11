// x/supernode/types/super_node_test.go
package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/stretchr/testify/require"
)

func TestSuperNodeValidation(t *testing.T) {
	valAddr := sdk.ValAddress([]byte("validator"))
	accAddr := sdk.AccAddress(valAddr)

	testCases := []struct {
		name        string
		supernode   types.SuperNode
		expectError bool
		errorType   error
	}{
		{
			name: "valid supernode",
			supernode: types.SuperNode{
				ValidatorAddress: valAddr.String(),
				SupernodeAccount: accAddr.String(),
				Evidence:         []*types.Evidence{},
				Note:             "1.0.0",
				LegacyMetrics: &types.MetricsAggregate{
					Metrics:     make(map[string]float64),
					ReportCount: 0,
				},
				States: []*types.SuperNodeStateRecord{
					{
						State:  types.SuperNodeStateActive,
						Height: 1,
					},
				},
				PrevIpAddresses: []*types.IPAddressHistory{
					{
						Address: "192.168.1.1",
						Height:  1,
					},
				},
				P2PPort: "26657",
			},
			expectError: false,
		},
		{
			name: "invalid validator address",
			supernode: types.SuperNode{
				SupernodeAccount: accAddr.String(),
				ValidatorAddress: "invalid",
				Note:             "1.0.0",
			},
			expectError: true,
		},
		{
			name: "empty ip address",
			supernode: types.SuperNode{
				ValidatorAddress: valAddr.String(),
				SupernodeAccount: accAddr.String(),
				Note:             "1.0.0",
				States: []*types.SuperNodeStateRecord{
					{
						State:  types.SuperNodeStateActive,
						Height: 1,
					},
				},
			},
			expectError: true,
			errorType:   types.ErrEmptyIPAddress,
		},
		{
			name: "unspecified state",
			supernode: types.SuperNode{
				SupernodeAccount: accAddr.String(),
				ValidatorAddress: valAddr.String(),
				Note:             "1.0.0",
			},
			expectError: true,
			errorType:   types.ErrInvalidSuperNodeState,
		},
		{
			name: "empty note",
			supernode: types.SuperNode{
				SupernodeAccount: accAddr.String(),
				ValidatorAddress: valAddr.String(),
				Note:             "",
				States: []*types.SuperNodeStateRecord{
					{
						State:  types.SuperNodeStateActive,
						Height: 1,
					},
				},
				PrevIpAddresses: []*types.IPAddressHistory{
					{
						Address: "192.168.1.1",
						Height:  1,
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty p2p-address",
			supernode: types.SuperNode{
				ValidatorAddress: valAddr.String(),
				SupernodeAccount: accAddr.String(),
				Evidence:         []*types.Evidence{},
				Note:             "1.0.0",
				LegacyMetrics: &types.MetricsAggregate{
					Metrics:     make(map[string]float64),
					ReportCount: 0,
				},
				States: []*types.SuperNodeStateRecord{
					{
						State:  types.SuperNodeStateActive,
						Height: 1,
					},
				},
				PrevIpAddresses: []*types.IPAddressHistory{
					{
						Address: "192.168.1.1",
						Height:  1,
					},
				},
				P2PPort: "",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.supernode.Validate()
			if tc.expectError {
				require.Error(t, err)
				if tc.errorType != nil {
					require.ErrorIs(t, err, tc.errorType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
