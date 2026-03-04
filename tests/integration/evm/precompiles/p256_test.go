//go:build integration
// +build integration

package precompiles_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"testing"
	"time"

	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	p256precompile "github.com/cosmos/evm/precompiles/p256"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

// TestP256PrecompileVerifyViaEthCall verifies secp256r1 signature validation
// behavior for valid and invalid public keys through the static p256 precompile.
func testP256PrecompileVerifyViaEthCall(t *testing.T, node *evmtest.Node) {
	t.Helper()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	msgHash := sha256.Sum256([]byte("lumera-p256-precompile"))

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate p256 key: %v", err)
	}
	r, s, err := ecdsa.Sign(rand.Reader, priv, msgHash[:])
	if err != nil {
		t.Fatalf("sign message: %v", err)
	}

	validInput := make([]byte, p256precompile.VerifyInputLength)
	copy(validInput[0:32], msgHash[:])
	r.FillBytes(validInput[32:64])
	s.FillBytes(validInput[64:96])
	priv.X.FillBytes(validInput[96:128])
	priv.Y.FillBytes(validInput[128:160])

	validResult := mustEthCallPrecompile(t, node, evmtypes.P256PrecompileAddress, validInput)
	wantTrue := common.LeftPadBytes(big.NewInt(1).Bytes(), 32)
	if !bytes.Equal(validResult, wantTrue) {
		t.Fatalf("expected valid p256 verification result %x, got %x", wantTrue, validResult)
	}

	otherPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate second p256 key: %v", err)
	}
	invalidInput := append([]byte(nil), validInput...)
	otherPriv.X.FillBytes(invalidInput[96:128])
	otherPriv.Y.FillBytes(invalidInput[128:160])

	invalidResult := mustEthCallPrecompile(t, node, evmtypes.P256PrecompileAddress, invalidInput)
	if len(invalidResult) != 0 {
		t.Fatalf("expected empty result for invalid p256 signature, got %x", invalidResult)
	}
}
