package types

import (
	"testing"

	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
)

func TestMetricsAggregateMarshalDeterministicForMap(t *testing.T) {
	m := &MetricsAggregate{
		Metrics: map[string]float64{
			"cpu":  85.5,
			"mem":  63.2,
			"disk": 71.9,
		},
		ReportCount: 10,
		Height:      12345,
	}

	first, err := gogoproto.Marshal(m)
	require.NoError(t, err)

	for i := 0; i < 40; i++ {
		got, err := gogoproto.Marshal(m)
		require.NoError(t, err)
		require.Equal(t, first, got)
	}
}
