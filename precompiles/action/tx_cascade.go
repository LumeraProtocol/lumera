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
	// RequestCascadeMethod is the ABI method name for requesting a Cascade action.
	RequestCascadeMethod = "requestCascade"
	// FinalizeCascadeMethod is the ABI method name for finalizing a Cascade action.
	FinalizeCascadeMethod = "finalizeCascade"
)

// RequestCascade creates a new Cascade action from typed ABI parameters.
func (p Precompile) RequestCascade(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 7 {
		return nil, fmt.Errorf("requestCascade: expected 7 args, got %d", len(args))
	}

	dataHash := args[0].(string)
	fileName := args[1].(string)
	rqIdsIc := args[2].(uint64)
	signatures := args[3].(string) // "Base64(rq_ids).creator_signature" textual format
	price := args[4].(*big.Int)
	expirationTime := args[5].(int64)
	fileSizeKbs := args[6].(uint64)

	creator, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	// Build Cascade metadata as JSON — the message server's handler will parse
	// and validate it, then convert to protobuf binary internally.
	metadataJSON, err := json.Marshal(map[string]interface{}{
		"data_hash":  dataHash,
		"file_name":  fileName,
		"rq_ids_ic":  rqIdsIc,
		"signatures": signatures,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal cascade metadata: %w", err)
	}

	priceCoin := sdk.NewCoin("ulume", math.NewIntFromBigInt(price))

	msg := &actiontypes.MsgRequestAction{
		Creator:        creator,
		ActionType:     actiontypes.ActionTypeCascade.String(),
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
		"file_name", fileName,
	)

	resp, err := p.actionMsgSvr.RequestAction(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := p.EmitActionRequested(ctx, stateDB, resp.ActionId, contract.Caller(), uint8(actiontypes.ActionTypeCascade), price); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(resp.ActionId)
}

// FinalizeCascade finalizes a Cascade action with typed result parameters.
func (p Precompile) FinalizeCascade(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("finalizeCascade: expected 2 args, got %d", len(args))
	}

	actionId := args[0].(string)
	rqIdsIds := args[1].([]string)

	caller, err := evmAddrToBech32(p.addrCdc, contract.Caller())
	if err != nil {
		return nil, fmt.Errorf("invalid caller address: %w", err)
	}

	metadataJSON, err := json.Marshal(map[string]interface{}{
		"rq_ids_ids": rqIdsIds,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal cascade finalize metadata: %w", err)
	}

	msg := &actiontypes.MsgFinalizeAction{
		Creator:    caller,
		ActionId:   actionId,
		ActionType: actiontypes.ActionTypeCascade.String(),
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

	// Look up the action to determine whether finalization actually completed.
	// The keeper may return nil (no error) for soft rejections where evidence
	// is recorded instead of failing the tx. Only emit ActionFinalized and
	// report success when the action reached the Done state.
	action, _ := p.actionKeeper.GetActionByID(ctx, actionId)
	finalized := action != nil && action.State == actiontypes.ActionStateDone
	if finalized {
		if err := p.EmitActionFinalized(ctx, stateDB, actionId, contract.Caller(), uint8(action.State)); err != nil {
			return nil, err
		}
	}

	return method.Outputs.Pack(finalized)
}
