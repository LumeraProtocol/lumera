package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateJSONRPCNamespacePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		chainID    string
		namespaces []string
		wantErr    string
	}{
		{
			name:       "mainnet rejects forbidden namespaces",
			chainID:    "lumera-mainnet-1",
			namespaces: []string{"eth", "debug", "personal", "admin", "rpc"},
			wantErr:    `["debug" "personal" "admin"]`,
		},
		{
			name:       "mainnet allows public namespaces",
			chainID:    "lumera-mainnet-1",
			namespaces: []string{"eth", "net", "web3", "rpc"},
		},
		{
			name:       "testnet allows debug namespaces",
			chainID:    "lumera-testnet-2",
			namespaces: []string{"eth", "debug", "personal", "admin"},
		},
		{
			name:       "devnet allows debug namespaces",
			chainID:    "lumera-devnet-3",
			namespaces: []string{"eth", "debug", "personal", "admin"},
		},
		{
			name:       "mainnet normalizes duplicates and casing",
			chainID:    "lumera-mainnet-1",
			namespaces: []string{"ETH", " Debug ", "debug", "PERSONAL"},
			wantErr:    `["debug" "personal"]`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateJSONRPCNamespacePolicy(tt.chainID, tt.namespaces)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, tt.chainID)
		})
	}
}
