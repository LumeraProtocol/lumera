package crossruntime

import (
	"bytes"
	"testing"

	coreaddress "cosmossdk.io/core/address"
	sdkaddress "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/ethereum/go-ethereum/common"
)

func newTestCodec() coreaddress.Codec {
	return sdkaddress.NewBech32Codec("lumera")
}

func TestEVMAddrToBech32_Roundtrip(t *testing.T) {
	addrCdc := newTestCodec()
	evmAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	bech32, err := EVMAddrToBech32(addrCdc, evmAddr)
	if err != nil {
		t.Fatalf("EVMAddrToBech32 failed: %v", err)
	}
	if bech32 == "" {
		t.Fatal("expected non-empty bech32 address")
	}

	// Roundtrip back
	recovered, err := Bech32ToEVMAddr(addrCdc, bech32)
	if err != nil {
		t.Fatalf("Bech32ToEVMAddr failed: %v", err)
	}
	if recovered != evmAddr {
		t.Fatalf("roundtrip mismatch: got %s, want %s", recovered.Hex(), evmAddr.Hex())
	}
}

func TestBech32ToEVMAddr_InvalidBech32(t *testing.T) {
	addrCdc := newTestCodec()
	_, err := Bech32ToEVMAddr(addrCdc, "notabech32address")
	if err == nil {
		t.Fatal("expected error for invalid bech32 address")
	}
}

func TestBech32ToEVMAddr_WrongPrefix(t *testing.T) {
	// Use a cosmos-prefixed address with a lumera codec
	addrCdc := newTestCodec()
	cosmosCodec := sdkaddress.NewBech32Codec("cosmos")
	evmAddr := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	cosmosBech32, err := EVMAddrToBech32(cosmosCodec, evmAddr)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = Bech32ToEVMAddr(addrCdc, cosmosBech32)
	if err == nil {
		t.Fatal("expected error for wrong bech32 prefix")
	}
}

func TestAccAddrToEVMAddr_20Bytes(t *testing.T) {
	input := bytes.Repeat([]byte{0xab}, 20)
	result := AccAddrToEVMAddr(input)
	if result != common.BytesToAddress(input) {
		t.Fatalf("mismatch: got %s", result.Hex())
	}
}

func TestAccAddrToEVMAddr_ShorterThan20Bytes(t *testing.T) {
	input := []byte{0x01, 0x02, 0x03}
	result := AccAddrToEVMAddr(input)
	// common.BytesToAddress left-pads short inputs
	expected := common.BytesToAddress(input)
	if result != expected {
		t.Fatalf("short input: got %s, want %s", result.Hex(), expected.Hex())
	}
}

func TestEVMAddrToBech32_ZeroAddress(t *testing.T) {
	addrCdc := newTestCodec()
	zero := common.Address{}

	bech32, err := EVMAddrToBech32(addrCdc, zero)
	if err != nil {
		t.Fatalf("zero address should encode: %v", err)
	}
	if bech32 == "" {
		t.Fatal("expected non-empty bech32 for zero address")
	}

	recovered, err := Bech32ToEVMAddr(addrCdc, bech32)
	if err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
	if recovered != zero {
		t.Fatalf("roundtrip mismatch for zero address: got %s", recovered.Hex())
	}
}
