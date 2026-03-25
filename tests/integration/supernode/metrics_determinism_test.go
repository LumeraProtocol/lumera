package integration_test

import (
	"testing"

	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func TestMetricsAggregateMarshalDeterministicAcrossMapInsertionOrder(t *testing.T) {
	m1 := &sntypes.MetricsAggregate{
		Metrics: map[string]float64{
			"cpu_usage":  85.5,
			"mem_usage":  63.2,
			"disk_usage": 71.9,
		},
		ReportCount: 10,
		Height:      12345,
	}

	m2 := &sntypes.MetricsAggregate{
		Metrics: map[string]float64{
			"disk_usage": 71.9,
			"mem_usage":  63.2,
			"cpu_usage":  85.5,
		},
		ReportCount: 10,
		Height:      12345,
	}

	b1, err := gogoproto.Marshal(m1)
	require.NoError(t, err)
	b2, err := gogoproto.Marshal(m2)
	require.NoError(t, err)
	require.Equal(t, b1, b2)
}
