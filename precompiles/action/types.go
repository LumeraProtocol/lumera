package action

import (
	"fmt"
	"math/big"

	"cosmossdk.io/core/address"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"

	actionkeeper "github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

// ActionInfo is the ABI-compatible struct returned by query methods.
// Field names and types must match the ABI definition exactly.
type ActionInfo struct {
	ActionId       string           `abi:"actionId"`
	Creator        common.Address   `abi:"creator"`
	ActionType     uint8            `abi:"actionType"`
	State          uint8            `abi:"state"`
	Metadata       string           `abi:"metadata"`
	Price          *big.Int         `abi:"price"`
	ExpirationTime int64            `abi:"expirationTime"`
	BlockHeight    int64            `abi:"blockHeight"`
	SuperNodes     []common.Address `abi:"superNodes"`
}

// actionToABIInfo converts a keeper Action to the ABI-compatible ActionInfo struct.
func actionToABIInfo(addrCdc address.Codec, action *actiontypes.Action, keeper *actionkeeper.Keeper) (ActionInfo, error) {
	creatorAddr, err := bech32ToEVMAddr(addrCdc, action.Creator)
	if err != nil {
		return ActionInfo{}, fmt.Errorf("convert creator address: %w", err)
	}

	price, err := parsePriceToBigInt(action.Price)
	if err != nil {
		return ActionInfo{}, fmt.Errorf("parse price: %w", err)
	}

	superNodes := make([]common.Address, 0, len(action.SuperNodes))
	for _, sn := range action.SuperNodes {
		addr, err := bech32ToEVMAddr(addrCdc, sn)
		if err != nil {
			continue // skip invalid addresses
		}
		superNodes = append(superNodes, addr)
	}

	// Convert protobuf metadata to JSON for the EVM caller
	metadataJSON := ""
	if len(action.Metadata) > 0 && keeper != nil {
		registry := keeper.GetActionRegistry()
		handler, err := registry.GetHandler(action.ActionType)
		if err == nil {
			jsonBz, err := handler.ConvertProtobufToJSON(action.Metadata)
			if err == nil {
				metadataJSON = string(jsonBz)
			}
		}
	}

	return ActionInfo{
		ActionId:       action.ActionID,
		Creator:        creatorAddr,
		ActionType:     uint8(action.ActionType),
		State:          uint8(action.State),
		Metadata:       metadataJSON,
		Price:          price,
		ExpirationTime: action.ExpirationTime,
		BlockHeight:    action.BlockHeight,
		SuperNodes:     superNodes,
	}, nil
}

// evmAddrToBech32 converts an EVM hex address to a Bech32 address string.
func evmAddrToBech32(addrCdc address.Codec, addr common.Address) (string, error) {
	return addrCdc.BytesToString(addr.Bytes())
}

// bech32ToEVMAddr converts a Bech32 address string to an EVM hex address.
func bech32ToEVMAddr(addrCdc address.Codec, bech32Addr string) (common.Address, error) {
	bz, err := addrCdc.StringToBytes(bech32Addr)
	if err != nil {
		return common.Address{}, err
	}
	return common.BytesToAddress(bz), nil
}

// parsePriceToBigInt extracts the amount from a Cosmos coin string like "10000ulume".
func parsePriceToBigInt(priceStr string) (*big.Int, error) {
	coin, err := sdk.ParseCoinNormalized(priceStr)
	if err != nil {
		// If parsing fails, try as a plain number
		n, ok := new(big.Int).SetString(priceStr, 10)
		if !ok {
			return big.NewInt(0), nil
		}
		return n, nil
	}
	return coin.Amount.BigInt(), nil
}
