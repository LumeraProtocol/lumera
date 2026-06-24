package crossruntime

import (
	"cosmossdk.io/core/address"
	"github.com/ethereum/go-ethereum/common"
)

// EVMAddrToBech32 converts an EVM hex address to a Bech32 address string
// using the provided address codec.
func EVMAddrToBech32(addrCdc address.Codec, addr common.Address) (string, error) {
	return addrCdc.BytesToString(addr.Bytes())
}

// Bech32ToEVMAddr converts a Bech32 address string to an EVM hex address
// using the provided address codec.
func Bech32ToEVMAddr(addrCdc address.Codec, bech32Addr string) (common.Address, error) {
	bz, err := addrCdc.StringToBytes(bech32Addr)
	if err != nil {
		return common.Address{}, err
	}
	return common.BytesToAddress(bz), nil
}

// AccAddrToEVMAddr converts an SDK AccAddress to an EVM common.Address.
func AccAddrToEVMAddr(addr []byte) common.Address {
	return common.BytesToAddress(addr)
}
