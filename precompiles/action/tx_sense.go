package action

import (
	"encoding/json"
	"fmt"
	"math/big"

	"cosmossdk.io/math"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

const (
	// RequestSenseMethod is the ABI method name for requesting a Sense action.
	RequestSenseMethod = "requestSense"
	// FinalizeSenseMethod is the ABI method name for finalizing a Sense action.
	FinalizeSenseMethod = "finalizeSense"
)

// RequestSense creates a new Sense action from typed ABI parameters.
func (p Precompile) RequestSense(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 5 {
		return nil, fmt.Errorf("requestSense: expected 5 args, got %d", len(args))
	}

	dataHash := args[0].(string)
	ddAndFingerprintsIc := args[1].(uint64)
	price := args[2].(*big.Int)
	expirationTime := args[3].(int64)
	fileSizeKbs := args[4].(uint64)

	creator, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	metadataJSON, err := json.Marshal(map[string]interface{}{
		"data_hash":              dataHash,
		"dd_and_fingerprints_ic": ddAndFingerprintsIc,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal sense metadata: %w", err)
	}

	priceCoin := sdk.NewCoin("ulume", math.NewIntFromBigInt(price))

	msg := &actiontypes.MsgRequestAction{
		Creator:        creator,
		ActionType:     actiontypes.ActionTypeSense.String(),
		Metadata:       string(metadataJSON),
		Price:          priceCoin.String(),
		ExpirationTime: fmt.Sprintf("%d", expirationTime),
		FileSizeKbs:    fmt.Sprintf("%d", fileSizeKbs),
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"creator", creator,
		"data_hash", dataHash,
	)

	resp, err := p.actionMsgSvr.RequestAction(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := p.EmitActionRequested(ctx, stateDB, resp.ActionId, contract.Caller(), uint8(actiontypes.ActionTypeSense), price); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(resp.ActionId)
}

// FinalizeSense finalizes a Sense action with typed result parameters.
func (p Precompile) FinalizeSense(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("finalizeSense: expected 3 args, got %d", len(args))
	}

	actionId := args[0].(string)
	ddAndFingerprintsIds := args[1].([]string)
	signatures := args[2].(string)

	caller, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	metadataJSON, err := json.Marshal(map[string]interface{}{
		"dd_and_fingerprints_ids": ddAndFingerprintsIds,
		"signatures":             signatures,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal sense finalize metadata: %w", err)
	}

	msg := &actiontypes.MsgFinalizeAction{
		Creator:    caller,
		ActionId:   actionId,
		ActionType: actiontypes.ActionTypeSense.String(),
		Metadata:   string(metadataJSON),
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"caller", caller,
		"action_id", actionId,
	)

	if _, err := p.actionMsgSvr.FinalizeAction(ctx, msg); err != nil {
		return nil, err
	}

	action, _ := p.actionKeeper.GetActionByID(ctx, actionId)
	var newState uint8
	if action != nil {
		newState = uint8(action.State)
	}

	if err := p.EmitActionFinalized(ctx, stateDB, actionId, contract.Caller(), newState); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
