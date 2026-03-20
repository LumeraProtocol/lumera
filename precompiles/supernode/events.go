package supernode

import (
	"math/big"
	"reflect"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// EventTypeSupernodeRegistered is emitted when a supernode is registered.
	EventTypeSupernodeRegistered = "SupernodeRegistered"
	// EventTypeSupernodeDeregistered is emitted when a supernode is deregistered.
	EventTypeSupernodeDeregistered = "SupernodeDeregistered"
	// EventTypeSupernodeStateChanged is emitted when a supernode changes state (start/stop).
	EventTypeSupernodeStateChanged = "SupernodeStateChanged"
)

// EmitSupernodeRegistered emits a SupernodeRegistered EVM log.
func (p Precompile) EmitSupernodeRegistered(
	ctx sdk.Context,
	stateDB vm.StateDB,
	validatorAddress string,
	creator common.Address,
	newState uint8,
) error {
	event := p.Events[EventTypeSupernodeRegistered]

	topics := make([]common.Hash, 3)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(validatorAddress)
	if err != nil {
		return err
	}
	topics[2], err = cmn.MakeTopic(creator)
	if err != nil {
		return err
	}

	data := cmn.PackNum(reflect.ValueOf(new(big.Int).SetUint64(uint64(newState))))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitSupernodeDeregistered emits a SupernodeDeregistered EVM log.
func (p Precompile) EmitSupernodeDeregistered(
	ctx sdk.Context,
	stateDB vm.StateDB,
	validatorAddress string,
	creator common.Address,
	oldState uint8,
) error {
	event := p.Events[EventTypeSupernodeDeregistered]

	topics := make([]common.Hash, 3)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(validatorAddress)
	if err != nil {
		return err
	}
	topics[2], err = cmn.MakeTopic(creator)
	if err != nil {
		return err
	}

	data := cmn.PackNum(reflect.ValueOf(new(big.Int).SetUint64(uint64(oldState))))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitSupernodeStateChanged emits a SupernodeStateChanged EVM log.
func (p Precompile) EmitSupernodeStateChanged(
	ctx sdk.Context,
	stateDB vm.StateDB,
	validatorAddress string,
	creator common.Address,
	newState uint8,
) error {
	event := p.Events[EventTypeSupernodeStateChanged]

	topics := make([]common.Hash, 3)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(validatorAddress)
	if err != nil {
		return err
	}
	topics[2], err = cmn.MakeTopic(creator)
	if err != nil {
		return err
	}

	data := cmn.PackNum(reflect.ValueOf(new(big.Int).SetUint64(uint64(newState))))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}
