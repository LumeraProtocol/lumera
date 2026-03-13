package v1_11_1

import (
	"testing"

	"github.com/stretchr/testify/require"

	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func TestWithMinDiskFreePercentFloor_RaisesWhenBelowFloor(t *testing.T) {
	params := audittypes.DefaultParams()
	params.MinDiskFreePercent = 0

	updated, changed := withMinDiskFreePercentFloor(params, 15)
	require.True(t, changed)
	require.Equal(t, uint32(15), updated.MinDiskFreePercent)
}

func TestWithMinDiskFreePercentFloor_NoChangeAtOrAboveFloor(t *testing.T) {
	testCases := []struct {
		name  string
		value uint32
		floor uint32
	}{
		{name: "equal", value: 15, floor: 15},
		{name: "greater", value: 22, floor: 15},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := audittypes.DefaultParams()
			params.MinDiskFreePercent = tc.value

			updated, changed := withMinDiskFreePercentFloor(params, tc.floor)
			require.False(t, changed)
			require.Equal(t, tc.value, updated.MinDiskFreePercent)
		})
	}
}
