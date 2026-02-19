package accounts

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// TestKeyInfo mirrors `keys add --output json` fields used in integration tests.
type TestKeyInfo struct {
	Address  string `json:"address"`
	Mnemonic string `json:"mnemonic"`
}

func (k *TestKeyInfo) Normalize() {
	k.Address = strings.TrimSpace(k.Address)
	k.Mnemonic = strings.TrimSpace(k.Mnemonic)
}

func (k TestKeyInfo) Validate() error {
	if k.Address == "" {
		return fmt.Errorf("empty key address")
	}
	if k.Mnemonic == "" {
		return fmt.Errorf("empty mnemonic")
	}
	return nil
}

func MustNormalizeAndValidateTestKeyInfo(t *testing.T, keyInfo *TestKeyInfo) {
	t.Helper()

	keyInfo.Normalize()
	require.NoError(t, keyInfo.Validate())
}

func AccountAddressFromTestKeyInfo(keyInfo TestKeyInfo) (common.Address, error) {
	accAddr, err := sdk.AccAddressFromBech32(keyInfo.Address)
	if err != nil {
		return common.Address{}, err
	}

	return common.BytesToAddress(accAddr.Bytes()), nil
}

func MustAccountAddressFromTestKeyInfo(t *testing.T, keyInfo TestKeyInfo) common.Address {
	t.Helper()

	address, err := AccountAddressFromTestKeyInfo(keyInfo)
	require.NoError(t, err)

	return address
}
