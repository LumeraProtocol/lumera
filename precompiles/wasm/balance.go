package wasm

import (
	cmn "github.com/cosmos/evm/precompiles/common"
)

// NewBalanceHandlerFactory returns the balance handler factory for the wasm
// precompile. It intentionally returns nil: the wasm precompile must install
// NO balance handler.
//
// Invariant: the EVM StateDB must never mirror bank balance events emitted
// by a CosmWasm contract execution. Every balance event a wasm contract can
// produce is a native bank movement of its own funds (dispatched from the
// 32-byte contract address); the native bank keeper is the single source of
// truth for those movements.
//
// Replaying them is doubly broken (see the regression tests in
// balance_test.go):
//
//  1. The upstream BalanceHandler maps event addresses into EVM accounts via
//     common.BytesToAddress, silently truncating the 32-byte wasm contract
//     address. The truncated alias has no EVM balance, so SubBalance wraps
//     uint256 (x/vm/statedb has no insufficient-balance guard), and the
//     SetBalance reconciliation at commit mints ~2^256 extended-denom units
//     into the alias account.
//  2. The matching coin_received replay credits the recipient in the StateDB
//     journal on top of the native bank credit; SetBalance reconciles the
//     journal target against a bank view that does not include the in-flight
//     native transfer and mints the amount a second time (observed: fixture
//     sent 111, recipient got 222, plus a giant alias mint).
//
// Filtering only non-20-byte addresses is NOT sufficient (case 2 persists),
// so the correct minimal fix is to not mirror at all. This is the single
// production wiring point for the invariant: NewPrecompile calls this
// helper, and the regression tests build the replay through it, so
// re-installing a handler fails the tests.
func NewBalanceHandlerFactory() *cmn.BalanceHandlerFactory {
	return nil
}
