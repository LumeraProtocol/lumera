// x/supernode/types/super_node_test.go
package types_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/pastelnetwork/pastel/x/supernode/types"
	"github.com/stretchr/testify/require"
)

func TestSuperNodeValidation(t *testing.T) {
	currentTime := time.Now()
	valAddr := sdk.ValAddress([]byte("validator"))

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
				IpAddress:        "192.168.1.1",
				State:            types.SuperNodeStateActive,
				Evidence:         []*types.Evidence{},
				LastTimeActive:   currentTime,
				StartedAt:        currentTime,
				Version:          "1.0.0",
				Metrics: &types.MetricsAggregate{
					Metrics:     make(map[string]float64),
					ReportCount: 0,
					LastUpdated: currentTime,
				},
			},
			expectError: false,
		},
		{
			name: "invalid validator address",
			supernode: types.SuperNode{
				ValidatorAddress: "invalid",
				IpAddress:        "192.168.1.1",
				State:            types.SuperNodeStateActive,
				Version:          "1.0.0",
			},
			expectError: true,
		},
		{
			name: "empty ip address",
			supernode: types.SuperNode{
				ValidatorAddress: valAddr.String(),
				IpAddress:        "",
				State:            types.SuperNodeStateActive,
				Version:          "1.0.0",
			},
			expectError: true,
			errorType:   types.ErrEmptyIPAddress,
		},
		{
			name: "unspecified state",
			supernode: types.SuperNode{
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				State:            types.SuperNodeStateUnspecified,
				Version:          "1.0.0",
			},
			expectError: true,
			errorType:   types.ErrInvalidSuperNodeState,
		},
		{
			name: "empty version",
			supernode: types.SuperNode{
				ValidatorAddress: valAddr.String(),
				IpAddress:        "192.168.1.1",
				State:            types.SuperNodeStateActive,
				Version:          "",
			},
			expectError: true,
			errorType:   types.ErrEmptyVersion,
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
