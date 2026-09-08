//go:build integration
// +build integration

package evmtest

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsTransientBlockGasLimitError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unrelated rejection", err: errors.New("nonce too low"), want: false},
		{name: "direct rejection", err: errors.New("exceeds block gas limit"), want: true},
		{name: "wrapped RPC rejection", err: fmt.Errorf("broadcast failed: %w", errors.New("exceeds block gas limit: internal")), want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isTransientBlockGasLimitError(tc.err); got != tc.want {
				t.Fatalf("isTransientBlockGasLimitError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
