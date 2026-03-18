//go:build integration
// +build integration

package contracts_test

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	testaccounts "github.com/LumeraProtocol/lumera/testutil/accounts"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	evmprogram "github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestERC20ApproveAllowanceTransferFrom deploys a minimal ERC20 contract and
// exercises the approve → allowance → transferFrom flow between two accounts.
//
// The contract implements: balanceOf, approve, allowance, transfer, transferFrom.
// This validates that standard ERC20 DeFi primitives work correctly on
// Lumera's EVM stack.
func TestERC20ApproveAllowanceTransferFrom(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-erc20-flows", 600)
	node.StartAndWaitRPC()
	defer node.Stop()

	// Wait for the first block before sending any transactions.
	node.WaitForBlockNumberAtLeast(t, 1, 30*time.Second)

	ownerAddr := testaccounts.MustAccountAddressFromTestKeyInfo(t, node.KeyInfo())
	ownerKey := evmtest.MustDerivePrivateKey(t, node.KeyInfo().Mnemonic)

	// Generate a spender account and fund it.
	spenderKey, spenderAddr := testaccounts.MustGenerateEthKey(t)
	fundAccount(t, node, spenderAddr)

	gasPrice := node.MustGetGasPriceWithRetry(t, 20*time.Second)

	// 1) Deploy minimal ERC20 contract with initial supply to owner.
	initialSupply := big.NewInt(1_000_000)
	deployCode := minimalERC20CreationCode(ownerAddr, initialSupply)
	nonce := node.MustGetPendingNonceWithRetry(t, ownerAddr.Hex(), 20*time.Second)
	deployHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: ownerKey,
		Nonce:      nonce,
		To:         nil,
		Value:      big.NewInt(0),
		Gas:        1_000_000,
		GasPrice:   gasPrice,
		Data:       deployCode,
	})
	deployReceipt := node.WaitForReceipt(t, deployHash, 45*time.Second)
	assertReceiptBasics(t, deployReceipt)
	contractAddr := evmtest.MustStringField(t, deployReceipt, "contractAddress")

	// 2) Check owner's initial balance.
	ownerBal := erc20BalanceOf(t, node, contractAddr, ownerAddr)
	if ownerBal.Cmp(initialSupply) != 0 {
		t.Fatalf("owner balance: got %s want %s", ownerBal, initialSupply)
	}

	// 3) Owner approves spender for 500 tokens.
	approveAmount := big.NewInt(500)
	approveData := erc20ApprovePacked(spenderAddr, approveAmount)
	nonce = node.MustGetPendingNonceWithRetry(t, ownerAddr.Hex(), 20*time.Second)
	approveHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: ownerKey,
		Nonce:      nonce,
		To:         ptrAddr(mustHexAddress(t, contractAddr)),
		Value:      big.NewInt(0),
		Gas:        200_000,
		GasPrice:   gasPrice,
		Data:       approveData,
	})
	approveReceipt := node.WaitForReceipt(t, approveHash, 45*time.Second)
	assertReceiptBasics(t, approveReceipt)

	// 4) Check allowance.
	allowance := erc20Allowance(t, node, contractAddr, ownerAddr, spenderAddr)
	if allowance.Cmp(approveAmount) != 0 {
		t.Fatalf("allowance: got %s want %s", allowance, approveAmount)
	}

	// 5) Spender calls transferFrom to move 200 tokens from owner to spender.
	transferAmount := big.NewInt(200)
	transferFromData := erc20TransferFromPacked(ownerAddr, spenderAddr, transferAmount)
	spenderNonce := node.MustGetPendingNonceWithRetry(t, spenderAddr.Hex(), 20*time.Second)
	transferHash := node.SendLegacyTxWithParams(t, evmtest.LegacyTxParams{
		PrivateKey: spenderKey,
		Nonce:      spenderNonce,
		To:         ptrAddr(mustHexAddress(t, contractAddr)),
		Value:      big.NewInt(0),
		Gas:        200_000,
		GasPrice:   gasPrice,
		Data:       transferFromData,
	})
	transferReceipt := node.WaitForReceipt(t, transferHash, 45*time.Second)
	assertReceiptBasics(t, transferReceipt)

	// 6) Verify balances after transferFrom.
	ownerBalAfter := erc20BalanceOf(t, node, contractAddr, ownerAddr)
	spenderBalAfter := erc20BalanceOf(t, node, contractAddr, spenderAddr)
	expectedOwner := new(big.Int).Sub(initialSupply, transferAmount)
	if ownerBalAfter.Cmp(expectedOwner) != 0 {
		t.Fatalf("owner balance after: got %s want %s", ownerBalAfter, expectedOwner)
	}
	if spenderBalAfter.Cmp(transferAmount) != 0 {
		t.Fatalf("spender balance after: got %s want %s", spenderBalAfter, transferAmount)
	}

	// 7) Verify allowance decreased.
	allowanceAfter := erc20Allowance(t, node, contractAddr, ownerAddr, spenderAddr)
	expectedAllowance := new(big.Int).Sub(approveAmount, transferAmount)
	if allowanceAfter.Cmp(expectedAllowance) != 0 {
		t.Fatalf("allowance after: got %s want %s", allowanceAfter, expectedAllowance)
	}

	t.Logf("ERC20 flow complete: approve=%s, transferFrom=%s, remainingAllowance=%s",
		approveAmount, transferAmount, allowanceAfter)
}

// ---------------------------------------------------------------------------
// ERC20 ABI helpers (manual encoding to avoid importing Solidity ABI tools)
// ---------------------------------------------------------------------------

// ERC20 method selectors (keccak256 of function signatures, first 4 bytes).
var (
	balanceOfSelector    = crypto.Keccak256([]byte("balanceOf(address)"))[:4]
	approveSelector      = crypto.Keccak256([]byte("approve(address,uint256)"))[:4]
	allowanceSelector    = crypto.Keccak256([]byte("allowance(address,address)"))[:4]
	transferSelector     = crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
	transferFromSelector = crypto.Keccak256([]byte("transferFrom(address,address,uint256)"))[:4]
)

func erc20BalanceOf(t *testing.T, node *evmtest.Node, contract string, account common.Address) *big.Int {
	t.Helper()
	data := append(balanceOfSelector, common.LeftPadBytes(account.Bytes(), 32)...)
	var resultHex string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   contract,
			"data": "0x" + hex.EncodeToString(data),
		},
		"latest",
	}, &resultHex)
	return decodeBigFromHex(t, resultHex)
}

func erc20Allowance(t *testing.T, node *evmtest.Node, contract string, owner, spender common.Address) *big.Int {
	t.Helper()
	data := append(allowanceSelector, common.LeftPadBytes(owner.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(spender.Bytes(), 32)...)
	var resultHex string
	node.MustJSONRPC(t, "eth_call", []any{
		map[string]any{
			"to":   contract,
			"data": "0x" + hex.EncodeToString(data),
		},
		"latest",
	}, &resultHex)
	return decodeBigFromHex(t, resultHex)
}

func erc20ApprovePacked(spender common.Address, amount *big.Int) []byte {
	data := append(approveSelector, common.LeftPadBytes(spender.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(amount.Bytes(), 32)...)
	return data
}

func erc20TransferFromPacked(from, to common.Address, amount *big.Int) []byte {
	data := append(transferFromSelector, common.LeftPadBytes(from.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(to.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(amount.Bytes(), 32)...)
	return data
}

func decodeBigFromHex(t *testing.T, hexStr string) *big.Int {
	t.Helper()
	trimmed := strings.TrimPrefix(strings.TrimSpace(hexStr), "0x")
	if trimmed == "" {
		return big.NewInt(0)
	}
	val, ok := new(big.Int).SetString(trimmed, 16)
	if !ok {
		t.Fatalf("decode big from hex %q failed", hexStr)
	}
	return val
}

func ptrAddr(a common.Address) *common.Address {
	return &a
}

// fundAccount sends native funds to an Ethereum address via bank send.
func fundAccount(t *testing.T, node *evmtest.Node, addr common.Address) {
	t.Helper()

	accCodec := addresscodec.NewBech32Codec(lcfg.Bech32AccountAddressPrefix)
	bech32Addr, err := accCodec.BytesToString(addr.Bytes())
	if err != nil {
		t.Fatalf("encode bech32: %v", err)
	}

	amount := big.NewInt(10_000_000_000_000) // Enough for fees.
	fundAccountViaBankSend(t, node, bech32Addr, amount)
}

// fundAccountViaBankSend sends native funds to a bech32 recipient.
func fundAccountViaBankSend(t *testing.T, node *evmtest.Node, recipient string, amount *big.Int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	output, err := evmtest.RunCommand(
		ctx,
		node.RepoRoot(),
		node.BinPath(),
		"tx", "bank", "send", "validator", recipient, amount.String()+lcfg.ChainDenom,
		"--home", node.HomeDir(),
		"--keyring-backend", "test",
		"--chain-id", node.ChainID(),
		"--node", node.CometRPCURL(),
		"--broadcast-mode", "sync",
		"--gas", "200000",
		"--fees", "1000"+lcfg.ChainDenom,
		"--yes",
		"--output", "json",
		"--log_no_color",
	)
	if err != nil {
		t.Fatalf("bank send to %s: %v\n%s", recipient, err, output)
	}

	// Wait for tx to be included in a block.
	time.Sleep(3 * time.Second)
	node.WaitForBlockNumberAtLeast(t, node.MustGetBlockNumber(t)+1, 20*time.Second)
}

// ---------------------------------------------------------------------------
// Minimal ERC20 bytecode generator
// ---------------------------------------------------------------------------
//
// This generates a minimal but functionally complete ERC20 contract using raw
// EVM bytecode. The contract supports:
//   - balanceOf(address)     → slot: keccak256(addr . slot0)
//   - approve(address,uint)  → slot: keccak256(spender . keccak256(owner . slot1))
//   - allowance(addr,addr)   → same slot as approve
//   - transfer(addr,uint)    → updates balances
//   - transferFrom(addr,addr,uint) → updates balances + decrements allowance
//
// Storage layout:
//   - Slot 0 base: balances mapping (balances[addr] = keccak256(addr . 0))
//   - Slot 1 base: allowances mapping (allowances[owner][spender] = keccak256(spender . keccak256(owner . 1)))

func minimalERC20CreationCode(initialHolder common.Address, supply *big.Int) []byte {
	// For test simplicity, we deploy a Solidity-like contract by building
	// raw init code that:
	// 1. Sets balances[initialHolder] = supply in storage
	// 2. Returns the runtime dispatcher bytecode
	//
	// The runtime uses function selector dispatch (first 4 bytes of calldata).

	// Build the runtime bytecode.
	runtime := buildERC20RuntimeBytecode()

	// Init code: store initial balance, then return runtime.
	// keccak256(abi.encode(initialHolder, 0)) = storage slot for balances[initialHolder]
	slotKey := balanceSlotKey(initialHolder)

	init := evmprogram.New()
	// Store supply at balanceSlot.
	init.Push(supply.Bytes())
	init.Push(slotKey)
	init.Op(vm.SSTORE)
	// Return runtime via CODECOPY.
	init.ReturnViaCodeCopy(runtime)

	return init.Bytes()
}

func balanceSlotKey(addr common.Address) []byte {
	// keccak256(abi.encodePacked(address, uint256(0)))
	data := append(common.LeftPadBytes(addr.Bytes(), 32), common.LeftPadBytes([]byte{0}, 32)...)
	return crypto.Keccak256(data)
}

func allowanceSlotKey(owner, spender common.Address) []byte {
	// keccak256(spender . keccak256(owner . 1))
	ownerData := append(common.LeftPadBytes(owner.Bytes(), 32), common.LeftPadBytes([]byte{1}, 32)...)
	innerHash := crypto.Keccak256(ownerData)
	outerData := append(common.LeftPadBytes(spender.Bytes(), 32), innerHash...)
	return crypto.Keccak256(outerData)
}

// buildERC20RuntimeBytecode creates a minimal ERC20 runtime using evmprogram
// that dispatches based on function selector. It implements:
//   - balanceOf(address)                    → 0x70a08231
//   - approve(address,uint256)              → 0x095ea7b3
//   - allowance(address,address)            → 0xdd62ed3e
//   - transfer(address,uint256)             → 0xa9059cbb
//   - transferFrom(address,address,uint256) → 0x23b872dd
//
// Storage layout (Solidity-compatible):
//   - Slot 0 base: balances mapping  — balances[addr]  = keccak256(addr . 0)
//   - Slot 1 base: allowances mapping — allowances[owner][spender] = keccak256(spender . keccak256(owner . 1))
//
// Memory scratch area: [0:64] is used for keccak256 hashing throughout.
func buildERC20RuntimeBytecode() []byte {
	rt := evmprogram.New()
	var revertPatches []int // forward-reference positions to the shared revert block

	// ── Dispatcher: extract 4-byte selector from calldata ─────────────
	// calldataload(0) >> 224 isolates the first 4 bytes as a uint256.
	rt.Push(0).Op(vm.CALLDATALOAD).Push(0xe0).Op(vm.SHR)

	// Branch to each function (forward jumps, patched after bodies are built).
	balOfPatch := selectorBranch(rt, balanceOfSelector)
	approvePatch := selectorBranch(rt, approveSelector)
	allowPatch := selectorBranch(rt, allowanceSelector)
	xferPatch := selectorBranch(rt, transferSelector)
	xferFromPatch := selectorBranch(rt, transferFromSelector)

	// Fallback: no matching selector → revert.
	rt.Op(vm.POP) // discard selector
	rt.Push(0).Push(0).Op(vm.REVERT)

	// ── balanceOf(address) ────────────────────────────────────────────
	// Returns balances[addr] where addr = calldata[4:36].
	patchJumpDest(rt, balOfPatch)
	rt.Op(vm.POP) // discard selector
	// Compute balance slot: keccak256(addr . 0).
	rt.Push(4).Op(vm.CALLDATALOAD) // addr
	rt.Push(0).Op(vm.MSTORE)       // mem[0:32] = addr
	rt.Push(0)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = 0
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	rt.Op(vm.SLOAD)          // balance
	rt.Push(0).Op(vm.MSTORE) // mem[0:32] = balance
	rt.Return(0, 32)

	// ── approve(address spender, uint256 amount) ──────────────────────
	// Sets allowances[CALLER][spender] = amount, returns true.
	patchJumpDest(rt, approvePatch)
	rt.Op(vm.POP) // discard selector
	// Step 1: innerHash = keccak256(CALLER . 1).
	rt.Op(vm.CALLER)
	rt.Push(0).Op(vm.MSTORE)  // mem[0:32] = CALLER (owner)
	rt.Push(1)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = 1
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	// Step 2: slot = keccak256(spender . innerHash).
	rt.Push(32).Op(vm.MSTORE)      // mem[32:64] = innerHash
	rt.Push(4).Op(vm.CALLDATALOAD) // spender
	rt.Push(0).Op(vm.MSTORE)       // mem[0:32] = spender
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	// Store amount at slot.                     stack: [slot]
	rt.Push(36).Op(vm.CALLDATALOAD) //           stack: [slot, amount]
	rt.Op(vm.SWAP1)                 //           stack: [amount, slot]
	rt.Op(vm.SSTORE)                // SSTORE(slot, amount)
	// Return true.
	rt.Push(1).Push(0).Op(vm.MSTORE)
	rt.Return(0, 32)

	// ── allowance(address owner, address spender) ─────────────────────
	// Returns allowances[owner][spender].
	patchJumpDest(rt, allowPatch)
	rt.Op(vm.POP) // discard selector
	// Step 1: innerHash = keccak256(owner . 1).
	rt.Push(4).Op(vm.CALLDATALOAD) // owner
	rt.Push(0).Op(vm.MSTORE)       // mem[0:32] = owner
	rt.Push(1)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = 1
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	// Step 2: slot = keccak256(spender . innerHash).
	rt.Push(32).Op(vm.MSTORE)       // mem[32:64] = innerHash
	rt.Push(36).Op(vm.CALLDATALOAD) // spender
	rt.Push(0).Op(vm.MSTORE)        // mem[0:32] = spender
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	rt.Op(vm.SLOAD)          // allowance value
	rt.Push(0).Op(vm.MSTORE) // mem[0:32] = value
	rt.Return(0, 32)

	// ── transfer(address to, uint256 amount) ──────────────────────────
	// Debits CALLER, credits to, returns true.
	patchJumpDest(rt, xferPatch)
	rt.Op(vm.POP) // discard selector

	rt.Push(36).Op(vm.CALLDATALOAD) // stack: [amount]

	// Compute sender slot: keccak256(CALLER . 0).
	rt.Op(vm.CALLER)
	rt.Push(0).Op(vm.MSTORE)  // mem[0:32] = CALLER
	rt.Push(0)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = 0
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	//                                           stack: [amount, senderSlot]
	rt.Op(vm.DUP1, vm.SLOAD) //                 stack: [amount, senderSlot, senderBal]

	// Underflow check: revert if senderBal < amount.
	rt.Op(vm.DUP3) //                           stack: [.., senderBal, amount]
	rt.Op(vm.DUP2) //                           stack: [.., senderBal, amount, senderBal]
	revertPatches = append(revertPatches, revertIfTopLT(rt))
	//                                           stack: [amount, senderSlot, senderBal]

	// newSenderBal = senderBal - amount.
	rt.Op(vm.DUP3)  //                          stack: [.., senderBal, amount]
	rt.Op(vm.SWAP1)  //                         stack: [.., amount, senderBal]
	rt.Op(vm.SUB)    //                         stack: [amount, senderSlot, senderBal-amount]
	rt.Op(vm.SWAP1)  //                         stack: [amount, newBal, senderSlot]
	rt.Op(vm.SSTORE) //                         stack: [amount]

	// Compute recipient slot: keccak256(to . 0).
	rt.Push(4).Op(vm.CALLDATALOAD)
	rt.Push(0).Op(vm.MSTORE)  // mem[0:32] = to
	rt.Push(0)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = 0
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	//                                           stack: [amount, toSlot]
	rt.Op(vm.DUP1, vm.SLOAD) //                 stack: [amount, toSlot, toBal]
	rt.Op(vm.DUP3)            //                stack: [amount, toSlot, toBal, amount]
	rt.Op(vm.ADD)              //               stack: [amount, toSlot, newToBal]
	rt.Op(vm.SWAP1)            //               stack: [amount, newToBal, toSlot]
	rt.Op(vm.SSTORE)           //               stack: [amount]
	rt.Op(vm.POP)              //               stack: []

	// Return true.
	rt.Push(1).Push(0).Op(vm.MSTORE)
	rt.Return(0, 32)

	// ── transferFrom(address from, address to, uint256 amount) ────────
	// Checks allowance, debits from, credits to, returns true.
	patchJumpDest(rt, xferFromPatch)
	rt.Op(vm.POP) // discard selector

	rt.Push(68).Op(vm.CALLDATALOAD) // stack: [amount]

	// --- Check & debit allowance ---
	// Compute allowance slot: keccak256(CALLER . keccak256(from . 1)).
	// Here CALLER = spender (msg.sender of transferFrom).
	rt.Push(4).Op(vm.CALLDATALOAD) // from (owner)
	rt.Push(0).Op(vm.MSTORE)       // mem[0:32] = from
	rt.Push(1)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = 1
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = innerHash
	rt.Op(vm.CALLER)
	rt.Push(0).Op(vm.MSTORE) // mem[0:32] = CALLER (spender)
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	//                                           stack: [amount, allowSlot]
	rt.Op(vm.DUP1, vm.SLOAD) //                 stack: [amount, allowSlot, allowVal]

	// Revert if allowVal < amount.
	rt.Op(vm.DUP3)
	rt.Op(vm.DUP2)
	revertPatches = append(revertPatches, revertIfTopLT(rt))
	//                                           stack: [amount, allowSlot, allowVal]

	// newAllowance = allowVal - amount.
	rt.Op(vm.DUP3, vm.SWAP1, vm.SUB) //         stack: [amount, allowSlot, newAllow]
	rt.Op(vm.SWAP1)                    //        stack: [amount, newAllow, allowSlot]
	rt.Op(vm.SSTORE)                   //        stack: [amount]

	// --- Debit from's balance ---
	// Compute balance slot: keccak256(from . 0).
	rt.Push(4).Op(vm.CALLDATALOAD)
	rt.Push(0).Op(vm.MSTORE)  // mem[0:32] = from
	rt.Push(0)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = 0
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	//                                           stack: [amount, fromSlot]
	rt.Op(vm.DUP1, vm.SLOAD) //                 stack: [amount, fromSlot, fromBal]

	// Revert if fromBal < amount.
	rt.Op(vm.DUP3, vm.DUP2)
	revertPatches = append(revertPatches, revertIfTopLT(rt))
	//                                           stack: [amount, fromSlot, fromBal]

	// newFromBal = fromBal - amount.
	rt.Op(vm.DUP3, vm.SWAP1, vm.SUB)
	rt.Op(vm.SWAP1, vm.SSTORE) //               stack: [amount]

	// --- Credit to's balance ---
	// Compute balance slot: keccak256(to . 0).
	rt.Push(36).Op(vm.CALLDATALOAD)
	rt.Push(0).Op(vm.MSTORE)  // mem[0:32] = to
	rt.Push(0)
	rt.Push(32).Op(vm.MSTORE) // mem[32:64] = 0
	rt.Push(64).Push(0).Op(vm.KECCAK256)
	//                                           stack: [amount, toSlot]
	rt.Op(vm.DUP1, vm.SLOAD) //                 stack: [amount, toSlot, toBal]
	rt.Op(vm.DUP3, vm.ADD)    //                stack: [amount, toSlot, newToBal]
	rt.Op(vm.SWAP1, vm.SSTORE) //               stack: [amount]
	rt.Op(vm.POP)               //              stack: []

	// Return true.
	rt.Push(1).Push(0).Op(vm.MSTORE)
	rt.Return(0, 32)

	// ── Shared revert block ──────────────────────────────────────────
	_, revertDest := rt.Jumpdest()
	rt.Push(0).Push(0).Op(vm.REVERT)

	// Patch all forward references to the revert block.
	code := rt.Bytes()
	for _, pos := range revertPatches {
		code[pos] = byte(revertDest >> 8)
		code[pos+1] = byte(revertDest)
	}

	return code
}

// selectorBranch emits a dispatcher branch: DUP1 PUSH4 <sel> EQ PUSH2 <placeholder> JUMPI.
// Returns the byte offset of the PUSH2 data to be patched with the real jump destination.
func selectorBranch(p *evmprogram.Program, selector []byte) int {
	p.Op(vm.DUP1)
	p.Push(selector)
	p.Op(vm.EQ)
	p.Op(vm.PUSH2)
	pos := p.Size()
	p.Append([]byte{0, 0}) // placeholder for 2-byte destination
	p.Op(vm.JUMPI)
	return pos
}

// patchJumpDest adds a JUMPDEST and patches the forward-reference at patchPos.
func patchJumpDest(p *evmprogram.Program, patchPos int) {
	_, dest := p.Jumpdest()
	code := p.Bytes()
	code[patchPos] = byte(dest >> 8)
	code[patchPos+1] = byte(dest)
}

// revertIfTopLT emits: LT PUSH2 <placeholder> JUMPI.
// Expects stack [a (top), b] — jumps to the shared revert block if a < b.
// Returns the byte offset to be patched with the revert destination.
func revertIfTopLT(p *evmprogram.Program) int {
	p.Op(vm.LT)
	p.Op(vm.PUSH2)
	pos := p.Size()
	p.Append([]byte{0, 0})
	p.Op(vm.JUMPI)
	return pos
}
