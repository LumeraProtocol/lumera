package action

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
	// EventTypeActionRequested is emitted when a new action is created.
	EventTypeActionRequested = "ActionRequested"
	// EventTypeActionFinalized is emitted when a supernode finalizes an action.
	EventTypeActionFinalized = "ActionFinalized"
	// EventTypeActionApproved is emitted when the creator approves an action.
	EventTypeActionApproved = "ActionApproved"
)

// EmitActionRequested emits an ActionRequested EVM log.
func (p Precompile) EmitActionRequested(
	ctx sdk.Context,
	stateDB vm.StateDB,
	actionId string,
	creator common.Address,
	actionType uint8,
	price *big.Int,
) error {
	event := p.Events[EventTypeActionRequested]

	topics := make([]common.Hash, 3)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(actionId)
	if err != nil {
		return err
	}
	topics[2], err = cmn.MakeTopic(creator)
	if err != nil {
		return err
	}

	// Pack non-indexed data: actionType (uint8) + price (uint256)
	var data []byte
	data = append(data, cmn.PackNum(reflect.ValueOf(new(big.Int).SetUint64(uint64(actionType))))...)
	data = append(data, cmn.PackNum(reflect.ValueOf(price))...)

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitActionFinalized emits an ActionFinalized EVM log.
func (p Precompile) EmitActionFinalized(
	ctx sdk.Context,
	stateDB vm.StateDB,
	actionId string,
	superNode common.Address,
	newState uint8,
) error {
	event := p.Events[EventTypeActionFinalized]

	topics := make([]common.Hash, 3)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(actionId)
	if err != nil {
		return err
	}
	topics[2], err = cmn.MakeTopic(superNode)
	if err != nil {
		return err
	}

	// Pack non-indexed data: newState (uint8)
	data := cmn.PackNum(reflect.ValueOf(new(big.Int).SetUint64(uint64(newState))))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitActionApproved emits an ActionApproved EVM log.
func (p Precompile) EmitActionApproved(
	ctx sdk.Context,
	stateDB vm.StateDB,
	actionId string,
	creator common.Address,
) error {
	event := p.Events[EventTypeActionApproved]

	topics := make([]common.Hash, 3)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(actionId)
	if err != nil {
		return err
	}
	topics[2], err = cmn.MakeTopic(creator)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        nil,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}
