package wasm

import (
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// EventTypeWasmExecuted is emitted when a CosmWasm contract is successfully executed.
	EventTypeWasmExecuted = "WasmExecuted"
)

// EmitWasmExecuted emits a WasmExecuted EVM log.
func (p Precompile) EmitWasmExecuted(
	ctx sdk.Context,
	stateDB vm.StateDB,
	caller common.Address,
	contractAddr string,
	response []byte,
) error {
	event := p.Events[EventTypeWasmExecuted]

	topics := make([]common.Hash, 2)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(caller)
	if err != nil {
		return err
	}

	// Pack non-indexed data: contractAddr (string) + response (bytes)
	data, err := p.ABI.Events[EventTypeWasmExecuted].Inputs.NonIndexed().Pack(contractAddr, response)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}
