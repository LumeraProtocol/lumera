# evmigration Multisig Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement multisig-account migration support in `x/evmigration` so that flat Cosmos SDK multisig accounts (secp256k1 sub-keys only) can migrate via `MsgClaimLegacyAccount` and `MsgMigrateValidator`.

**Architecture:** Replace flat `legacy_pub_key`/`legacy_signature` fields with a structured `LegacyProof` proto oneof (`SingleKeyProof | MultisigProof`). Extend the verifier to reconstruct the multisig `LegacyAminoPubKey` and verify sub-signatures. Add a four-step offline CLI flow (`generate-proof-payload` → `sign-proof` → `combine-proof` → `submit-proof`) modeled on SDK's `tx multisign`.

**Tech Stack:** Go 1.26.1, Cosmos SDK v0.53.6, protoc via `buf`, `github.com/stretchr/testify/require`, `go.uber.org/mock`, `github.com/cosmos/cosmos-sdk/crypto/keys/multisig` (`kmultisig`), `github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1`.

**Design spec reference:** [docs/design/2026-04-18-evmigration-multisig-design.md](../design/2026-04-18-evmigration-multisig-design.md)

**Build/test reference commands:**
- Build: `make build` → produces `build/lumerad`
- Proto regen: `make build-proto`
- Lint: `make lint` (must pass with 0 issues)
- Unit tests: `go test ./x/evmigration/... -v`
- Integration tests: `go test -tags='test' ./tests/integration/evmigration/... -v -timeout 10m`
- Devnet: `make devnet-new` then run devnet tests

---

## Phase 1 — Proto Foundation

### Task 1: Create `proof.proto`

**Files:**
- Create: `proto/lumera/evmigration/proof.proto`

- [ ] **Step 1: Create the proof.proto file**

Write `proto/lumera/evmigration/proof.proto`:

```proto
syntax = "proto3";
package lumera.evmigration;

option go_package = "x/evmigration/types";

// SigFormat enumerates accepted signing envelopes for legacy-side signatures.
enum SigFormat {
  SIG_FORMAT_UNSPECIFIED = 0;
  SIG_FORMAT_CLI         = 1; // Sign(SHA256(payload)) via keyring
  SIG_FORMAT_ADR036      = 2; // ADR-036 signArbitrary canonical JSON
}

// LegacyProof authenticates a legacy-account holder.
// Exactly one oneof case must be set.
message LegacyProof {
  oneof proof {
    SingleKeyProof single   = 1;
    MultisigProof  multisig = 2;
  }
}

// SingleKeyProof is a single compressed secp256k1 key + signature.
message SingleKeyProof {
  // 33-byte compressed secp256k1 public key.
  bytes pub_key        = 1;
  // 64-byte raw secp256k1 signature (CLI) or canonical ADR-036 signature.
  bytes signature      = 2;
  SigFormat sig_format = 3;
}

// MultisigProof is a flat K-of-N multisig with all sub-keys secp256k1.
message MultisigProof {
  // threshold is K: the minimum number of valid sub-signatures required.
  uint32 threshold = 1;
  // sub_pub_keys lists all N sub-keys in original ordering, 33 bytes each.
  repeated bytes sub_pub_keys = 2;
  // signer_indices lists exactly K distinct indices into sub_pub_keys, strictly ascending.
  repeated uint32 signer_indices = 3;
  // sub_signatures are in the same order as signer_indices.
  repeated bytes sub_signatures = 4;
  SigFormat sig_format = 5;
}
```

- [ ] **Step 2: Run buf lint to confirm it parses**

Run: `cd proto && buf lint`
Expected: no errors for `lumera/evmigration/proof.proto`.

- [ ] **Step 3: Commit**

```bash
git add proto/lumera/evmigration/proof.proto
git commit -m "evmigration: add proof.proto with LegacyProof oneof"
```

---

### Task 2: Update `tx.proto`, `params.proto`, `query.proto`

**Files:**
- Modify: `proto/lumera/evmigration/tx.proto`
- Modify: `proto/lumera/evmigration/params.proto`
- Modify: `proto/lumera/evmigration/query.proto`

- [ ] **Step 1: Update tx.proto — add proof.proto import and replace flat fields**

Edit `proto/lumera/evmigration/tx.proto`. Add at top after existing imports:

```proto
import "lumera/evmigration/proof.proto";
```

Replace `message MsgClaimLegacyAccount { ... }` with:

```proto
// MsgClaimLegacyAccount migrates on-chain state from legacy_address to new_address.
message MsgClaimLegacyAccount {
  string new_address    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string legacy_address = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  // legacy_proof authenticates the legacy key holder's consent.
  LegacyProof legacy_proof = 3 [(gogoproto.nullable) = false];
  // new_signature: eth_secp256k1 signature over
  //   Keccak256("lumera-evm-migration:<chain_id>:<evm_chain_id>:claim:<legacy_address>:<new_address>")
  // proving the destination key holder consents to receive migrated state.
  // Also accepts EIP-191 personal_sign signatures (Keplr/Leap wallet path).
  bytes new_signature = 5;
  reserved 4;
  reserved "legacy_pub_key", "legacy_signature";
}
```

Replace `message MsgMigrateValidator { ... }` with the same shape:

```proto
message MsgMigrateValidator {
  string new_address    = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string legacy_address = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  LegacyProof legacy_proof = 3 [(gogoproto.nullable) = false];
  bytes new_signature = 5;
  reserved 4;
  reserved "legacy_pub_key", "legacy_signature";
}
```

- [ ] **Step 2: Update params.proto — add max_multisig_sub_keys**

Edit `proto/lumera/evmigration/params.proto`. Add after `max_validator_delegations`:

```proto
  // max_multisig_sub_keys caps the number of sub-keys in a multisig legacy
  // account's MultisigProof. Bounds per-tx verification cost.
  // Default: 20.
  uint32 max_multisig_sub_keys = 5;
```

- [ ] **Step 3: Update query.proto — add multisig fields to LegacyAccountInfo and MigrationEstimateResponse**

Edit `proto/lumera/evmigration/query.proto`. Append to `LegacyAccountInfo`:

```proto
  // is_multisig is true when the account's on-chain pubkey is a flat Cosmos
  // multisig of secp256k1 sub-keys.
  bool   is_multisig = 5;
  // threshold is K for K-of-N multisig (0 when !is_multisig).
  uint32 threshold   = 6;
  // num_signers is N for K-of-N multisig (0 when !is_multisig).
  uint32 num_signers = 7;
```

Append to `QueryMigrationEstimateResponse`:

```proto
  // is_multisig is true when the account's on-chain pubkey is a flat Cosmos
  // multisig of secp256k1 sub-keys.
  bool   is_multisig = 16;
  // threshold is K for K-of-N multisig (0 when !is_multisig).
  uint32 threshold   = 17;
  // num_signers is N for K-of-N multisig (0 when !is_multisig).
  uint32 num_signers = 18;
```

- [ ] **Step 4: Run buf lint and breaking check**

Run: `cd proto && buf lint`
Expected: no errors.

Run: `cd proto && buf breaking --against '.git#branch=master' || true`
Expected: breaking changes reported for tx.proto (expected — pre-EVM-upgrade, no on-chain consumers). Document this in the commit message.

- [ ] **Step 5: Commit**

```bash
git add proto/lumera/evmigration/
git commit -m "evmigration: update tx/params/query protos for multisig support

- MsgClaimLegacyAccount/MsgMigrateValidator: replace legacy_pub_key and
  legacy_signature with LegacyProof oneof (field 3).
- Params: add max_multisig_sub_keys (field 5, default 20).
- LegacyAccountInfo: add is_multisig / threshold / num_signers (fields 5-7).
- QueryMigrationEstimateResponse: add is_multisig / threshold / num_signers
  (fields 16-18).

Breaking change is intentional; module is pre-EVM-upgrade so no on-chain
messages are in flight."
```

---

### Task 3: Regenerate protobuf Go code

**Files:**
- Modified (via codegen): `x/evmigration/types/tx.pb.go`
- Modified (via codegen): `x/evmigration/types/params.pb.go`
- Modified (via codegen): `x/evmigration/types/query.pb.go`
- Created (via codegen): `x/evmigration/types/proof.pb.go`

- [ ] **Step 1: Run protobuf code generation**

Run: `make build-proto`
Expected: No errors. New file `x/evmigration/types/proof.pb.go` appears; `tx.pb.go`, `params.pb.go`, `query.pb.go` are updated.

- [ ] **Step 2: Verify generated types compile**

Run: `go build ./x/evmigration/types/...`
Expected: Compilation may fail at this point because existing code (verify.go, types.go, CLI) still references `LegacyPubKey` / `LegacySignature`. That's fine — those are fixed in later tasks. The `types` package alone should compile.

Confirm with: `go vet ./x/evmigration/types/`
Expected: No errors specific to types package itself (unrelated compile errors in dependent packages can be ignored).

- [ ] **Step 3: Commit regenerated code**

```bash
git add x/evmigration/types/*.pb.go
git commit -m "evmigration: regenerate protobuf Go code for multisig protos"
```

---

## Phase 2 — Type-Level Validation

### Task 4: Create `types/proof.go` with ValidateBasic helpers

**Files:**
- Create: `x/evmigration/types/proof.go`
- Create: `x/evmigration/types/proof_test.go`
- Modify: `x/evmigration/types/errors.go` (add `ErrInvalidLegacyProof`)

- [ ] **Step 1: Add ErrInvalidLegacyProof to errors.go**

Edit `x/evmigration/types/errors.go`. Add to the error registrations (after the existing `ErrNewAddressAlreadyUsed` at code 1119):

```go
ErrInvalidLegacyProof = errors.Register(ModuleName, 1120, "invalid legacy proof")
```

(The existing file uses `errors.Register` — not `errorsmod.Register` — as imported from `cosmossdk.io/errors`. Keep that alias.)

- [ ] **Step 2: Write failing tests for SingleKeyProof.ValidateBasic**

Create `x/evmigration/types/proof_test.go`:

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

func TestSingleKeyProof_ValidateBasic(t *testing.T) {
	validPK := make([]byte, 33)
	validSig := make([]byte, 64)

	cases := []struct {
		name    string
		proof   *types.SingleKeyProof
		wantErr string
	}{
		{
			name:    "valid",
			proof:   &types.SingleKeyProof{PubKey: validPK, Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI},
			wantErr: "",
		},
		{
			name:    "wrong pubkey length",
			proof:   &types.SingleKeyProof{PubKey: make([]byte, 32), Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI},
			wantErr: "must be 33 bytes",
		},
		{
			name:    "empty signature",
			proof:   &types.SingleKeyProof{PubKey: validPK, Signature: nil, SigFormat: types.SigFormat_SIG_FORMAT_CLI},
			wantErr: "signature required",
		},
		{
			name:    "unspecified sig format",
			proof:   &types.SingleKeyProof{PubKey: validPK, Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_UNSPECIFIED},
			wantErr: "sig_format required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := types.SingleKeyProofValidateBasic(tc.proof)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestMultisigProof_ValidateBasic(t *testing.T) {
	makeKeys := func(n int) [][]byte {
		keys := make([][]byte, n)
		for i := range keys {
			keys[i] = make([]byte, 33)
			keys[i][0] = byte(i + 1) // distinct bytes per key
		}
		return keys
	}
	validSig := make([]byte, 64)

	cases := []struct {
		name    string
		proof   *types.MultisigProof
		wantErr string
	}{
		{
			name: "valid 2-of-3",
			proof: &types.MultisigProof{
				Threshold:      2,
				SubPubKeys:     makeKeys(3),
				SignerIndices:  []uint32{0, 2},
				SubSignatures:  [][]byte{validSig, validSig},
				SigFormat:      types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "",
		},
		{
			name: "empty sub_pub_keys",
			proof: &types.MultisigProof{
				Threshold:     1,
				SubPubKeys:    nil,
				SignerIndices: []uint32{0},
				SubSignatures: [][]byte{validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "sub_pub_keys empty",
		},
		{
			name: "threshold zero",
			proof: &types.MultisigProof{
				Threshold:     0,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{},
				SubSignatures: [][]byte{},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "invalid threshold",
		},
		{
			name: "threshold exceeds N",
			proof: &types.MultisigProof{
				Threshold:     4,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 1, 2},
				SubSignatures: [][]byte{validSig, validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "invalid threshold",
		},
		{
			name: "too few signer_indices",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0},
				SubSignatures: [][]byte{validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "expected exactly K=2 signer_indices",
		},
		{
			name: "too many signer_indices",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 1, 2},
				SubSignatures: [][]byte{validSig, validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "expected exactly K=2 signer_indices",
		},
		{
			name: "sub_signatures length mismatch",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 1},
				SubSignatures: [][]byte{validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "sub_signatures length mismatch",
		},
		{
			name: "indices not ascending",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{2, 0},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "strictly ascending",
		},
		{
			name: "indices duplicate",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{1, 1},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "strictly ascending",
		},
		{
			name: "index out of range",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 5},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: ">= N=3",
		},
		{
			name: "sub pubkey wrong length",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    [][]byte{make([]byte, 33), make([]byte, 32), make([]byte, 33)},
				SignerIndices: []uint32{0, 1},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			},
			wantErr: "must be 33 bytes",
		},
		{
			name: "unspecified sig format",
			proof: &types.MultisigProof{
				Threshold:     2,
				SubPubKeys:    makeKeys(3),
				SignerIndices: []uint32{0, 1},
				SubSignatures: [][]byte{validSig, validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_UNSPECIFIED,
			},
			wantErr: "sig_format required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := types.MultisigProofValidateBasic(tc.proof)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestMultisigProof_ValidateParams_SizeCap(t *testing.T) {
	makeKeys := func(n int) [][]byte {
		keys := make([][]byte, n)
		for i := range keys {
			keys[i] = make([]byte, 33)
			keys[i][0] = byte(i + 1)
		}
		return keys
	}
	validSig := make([]byte, 64)

	proof := &types.MultisigProof{
		Threshold:     1,
		SubPubKeys:    makeKeys(21),
		SignerIndices: []uint32{0},
		SubSignatures: [][]byte{validSig},
		SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
	}
	err := types.MultisigProofValidateParams(proof, 20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds max 20")

	// Within cap
	proof.SubPubKeys = makeKeys(20)
	require.NoError(t, types.MultisigProofValidateParams(proof, 20))
}

func TestLegacyProof_ValidateBasic_Dispatch(t *testing.T) {
	validPK := make([]byte, 33)
	validSig := make([]byte, 64)

	t.Run("single", func(t *testing.T) {
		p := &types.LegacyProof{
			Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
				PubKey: validPK, Signature: validSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
			}},
		}
		require.NoError(t, p.ValidateBasic())
	})
	t.Run("multisig", func(t *testing.T) {
		subKeys := [][]byte{make([]byte, 33), make([]byte, 33)}
		subKeys[0][0] = 1
		subKeys[1][0] = 2
		p := &types.LegacyProof{
			Proof: &types.LegacyProof_Multisig{Multisig: &types.MultisigProof{
				Threshold:     1,
				SubPubKeys:    subKeys,
				SignerIndices: []uint32{0},
				SubSignatures: [][]byte{validSig},
				SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
			}},
		}
		require.NoError(t, p.ValidateBasic())
	})
	t.Run("neither set", func(t *testing.T) {
		p := &types.LegacyProof{}
		err := p.ValidateBasic()
		require.Error(t, err)
		require.Contains(t, err.Error(), "oneof not set")
	})
	t.Run("nil proof", func(t *testing.T) {
		var p *types.LegacyProof
		err := p.ValidateBasic()
		require.Error(t, err)
		require.Contains(t, err.Error(), "legacy_proof required")
	})
}
```

- [ ] **Step 3: Run tests — verify they fail**

Run: `go test ./x/evmigration/types/ -run 'TestSingleKeyProof|TestMultisigProof|TestLegacyProof_ValidateBasic_Dispatch' -v`
Expected: FAIL with "undefined: SingleKeyProofValidateBasic" (or similar — the helpers don't exist yet).

- [ ] **Step 4: Implement the validators in types/proof.go**

Create `x/evmigration/types/proof.go`:

```go
package types

import (
	errorsmod "cosmossdk.io/errors"
)

// ValidateBasic performs stateless validation of a LegacyProof.
// Governance-controlled limits (MaxMultisigSubKeys) are checked via
// ValidateParams, called from the msg server after loading params.
func (p *LegacyProof) ValidateBasic() error {
	if p == nil {
		return ErrInvalidLegacyProof.Wrap("legacy_proof required")
	}
	switch inner := p.Proof.(type) {
	case *LegacyProof_Single:
		return SingleKeyProofValidateBasic(inner.Single)
	case *LegacyProof_Multisig:
		return MultisigProofValidateBasic(inner.Multisig)
	default:
		return ErrInvalidLegacyProof.Wrap("legacy_proof oneof not set")
	}
}

// ValidateParams performs param-dependent validation. Must be called by the
// msg server after Params are loaded from state.
func (p *LegacyProof) ValidateParams(maxSubKeys uint32) error {
	if p == nil {
		return ErrInvalidLegacyProof.Wrap("legacy_proof required")
	}
	if m, ok := p.Proof.(*LegacyProof_Multisig); ok {
		return MultisigProofValidateParams(m.Multisig, maxSubKeys)
	}
	return nil
}

// SingleKeyProofValidateBasic validates a SingleKeyProof's static invariants.
func SingleKeyProofValidateBasic(s *SingleKeyProof) error {
	if s == nil {
		return ErrInvalidLegacyProof.Wrap("single proof nil")
	}
	if len(s.PubKey) != 33 {
		return ErrInvalidLegacyPubKey.Wrap("pub_key must be 33 bytes")
	}
	if len(s.Signature) == 0 {
		return ErrInvalidLegacySignature.Wrap("signature required")
	}
	if s.SigFormat == SigFormat_SIG_FORMAT_UNSPECIFIED {
		return ErrInvalidLegacyProof.Wrap("sig_format required")
	}
	return nil
}

// MultisigProofValidateBasic validates a MultisigProof's static invariants
// (length, ordering, indices). Size cap is enforced separately by
// MultisigProofValidateParams.
func MultisigProofValidateBasic(m *MultisigProof) error {
	if m == nil {
		return ErrInvalidLegacyProof.Wrap("multisig proof nil")
	}
	n := uint32(len(m.SubPubKeys))
	if n == 0 {
		return ErrInvalidLegacyProof.Wrap("sub_pub_keys empty")
	}
	if m.Threshold < 1 || m.Threshold > n {
		return errorsmod.Wrapf(ErrInvalidLegacyProof, "invalid threshold K=%d for N=%d", m.Threshold, n)
	}
	if uint32(len(m.SignerIndices)) != m.Threshold {
		return errorsmod.Wrapf(ErrInvalidLegacyProof,
			"expected exactly K=%d signer_indices, got %d", m.Threshold, len(m.SignerIndices))
	}
	if len(m.SubSignatures) != len(m.SignerIndices) {
		return ErrInvalidLegacyProof.Wrap("sub_signatures length mismatch")
	}
	for i := 1; i < len(m.SignerIndices); i++ {
		if m.SignerIndices[i] <= m.SignerIndices[i-1] {
			return ErrInvalidLegacyProof.Wrap("signer_indices must be strictly ascending")
		}
	}
	for i, idx := range m.SignerIndices {
		if idx >= n {
			return errorsmod.Wrapf(ErrInvalidLegacyProof,
				"signer_indices[%d]=%d >= N=%d", i, idx, n)
		}
	}
	for i, k := range m.SubPubKeys {
		if len(k) != 33 {
			return errorsmod.Wrapf(ErrInvalidLegacyPubKey,
				"sub_pub_keys[%d] must be 33 bytes", i)
		}
	}
	if m.SigFormat == SigFormat_SIG_FORMAT_UNSPECIFIED {
		return ErrInvalidLegacyProof.Wrap("sig_format required")
	}
	return nil
}

// MultisigProofValidateParams enforces the governance-adjustable size cap.
func MultisigProofValidateParams(m *MultisigProof, maxSubKeys uint32) error {
	if m == nil {
		return nil
	}
	if uint32(len(m.SubPubKeys)) > maxSubKeys {
		return errorsmod.Wrapf(ErrInvalidLegacyProof,
			"multisig N=%d exceeds max %d", len(m.SubPubKeys), maxSubKeys)
	}
	return nil
}
```

- [ ] **Step 5: Run tests — verify they pass**

Run: `go test ./x/evmigration/types/ -run 'TestSingleKeyProof|TestMultisigProof|TestLegacyProof_ValidateBasic_Dispatch' -v`
Expected: All subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add x/evmigration/types/proof.go x/evmigration/types/proof_test.go x/evmigration/types/errors.go
git commit -m "evmigration: add LegacyProof ValidateBasic/ValidateParams helpers

Stateless ValidateBasic enforces format invariants (lengths, ordering,
threshold bounds). Param-dependent ValidateParams enforces
MaxMultisigSubKeys cap and is called from msg server after params load.

Tests cover 12 MultisigProof rejection cases + 4 SingleKeyProof cases +
LegacyProof dispatch."
```

---

### Task 5: Update Msg ValidateBasic in `types.go`

**Files:**
- Modify: `x/evmigration/types/types.go`

- [ ] **Step 1: Replace MsgClaimLegacyAccount.ValidateBasic and MsgMigrateValidator.ValidateBasic**

Edit `x/evmigration/types/types.go`. Replace the body of `MsgClaimLegacyAccount.ValidateBasic`:

```go
func (msg *MsgClaimLegacyAccount) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.NewAddress); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid new_address (%s)", err)
	}
	if _, err := sdk.AccAddressFromBech32(msg.LegacyAddress); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid legacy_address (%s)", err)
	}
	if msg.NewAddress == msg.LegacyAddress {
		return ErrSameAddress
	}
	if err := msg.LegacyProof.ValidateBasic(); err != nil {
		return err
	}
	if len(msg.NewSignature) == 0 {
		return ErrInvalidNewSignature.Wrap("new_signature is required")
	}
	return nil
}
```

Replace `MsgMigrateValidator.ValidateBasic` with the identical body (same field paths).

- [ ] **Step 2: Run types tests**

Run: `go test ./x/evmigration/types/ -v`
Expected: Existing tests still PASS. New failures, if any, are in tests that reference the removed `LegacyPubKey` / `LegacySignature` fields — those are fixed in subsequent tasks.

- [ ] **Step 3: Commit**

```bash
git add x/evmigration/types/types.go
git commit -m "evmigration: msg ValidateBasic delegates to LegacyProof.ValidateBasic"
```

---

### Task 6: Update Params with MaxMultisigSubKeys

**Files:**
- Modify: `x/evmigration/types/params.go`

- [ ] **Step 1: Add default constant and Params.Validate check**

Edit `x/evmigration/types/params.go`. Add default constant block:

```go
// DefaultMaxMultisigSubKeys caps the number of sub-keys a multisig legacy account
// may have when migrating. Bounds per-tx verification cost.
var DefaultMaxMultisigSubKeys uint32 = 20
```

Update the `NewParams` signature to accept the new param:

```go
func NewParams(
	enableMigration bool,
	migrationEndTime int64,
	maxMigrationsPerBlock uint64,
	maxValidatorDelegations uint64,
	maxMultisigSubKeys uint32,
) Params {
	return Params{
		EnableMigration:         enableMigration,
		MigrationEndTime:        migrationEndTime,
		MaxMigrationsPerBlock:   maxMigrationsPerBlock,
		MaxValidatorDelegations: maxValidatorDelegations,
		MaxMultisigSubKeys:      maxMultisigSubKeys,
	}
}
```

Update `DefaultParams`:

```go
func DefaultParams() Params {
	return NewParams(
		DefaultEnableMigration,
		DefaultMigrationEndTime,
		DefaultMaxMigrationsPerBlock,
		DefaultMaxValidatorDelegations,
		DefaultMaxMultisigSubKeys,
	)
}
```

Add to the end of `Validate()` (before `return nil`):

```go
	if p.MaxMultisigSubKeys == 0 {
		return fmt.Errorf("max_multisig_sub_keys must be positive")
	}
```

- [ ] **Step 2: Write test for new param default and Validate**

Add to `x/evmigration/types/types_test.go` (or create `params_test.go` if none exists):

```go
func TestParams_MaxMultisigSubKeys(t *testing.T) {
	p := types.DefaultParams()
	require.Equal(t, uint32(20), p.MaxMultisigSubKeys)
	require.NoError(t, p.Validate())

	p.MaxMultisigSubKeys = 0
	require.ErrorContains(t, p.Validate(), "max_multisig_sub_keys must be positive")
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./x/evmigration/types/ -run TestParams -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add x/evmigration/types/params.go x/evmigration/types/types_test.go
git commit -m "evmigration: add MaxMultisigSubKeys param (default 20)"
```

---

## Phase 3 — Verifier Refactor

### Task 7: Add shared `verifySecp256k1Sig` helper

**Files:**
- Modify: `x/evmigration/keeper/verify.go`

- [ ] **Step 1: Add the shared helper function**

Edit `x/evmigration/keeper/verify.go`. Add this new function (place it after `adr036SignDoc`):

```go
// verifySecp256k1Sig checks a single secp256k1 signature over the migration
// payload, accepting either the CLI (raw SHA256) or ADR-036 (canonical JSON)
// envelope as indicated by format. signerAddr must be the bech32 address
// derived from pk — for single-key proofs this is legacyAddr, and for
// multisig proofs it is the individual sub-signer's address.
func verifySecp256k1Sig(pk *secp256k1.PubKey, signerAddr sdk.AccAddress, payload, sig []byte, format types.SigFormat) error {
	switch format {
	case types.SigFormat_SIG_FORMAT_CLI:
		hash := sha256.Sum256(payload)
		if pk.VerifySignature(hash[:], sig) {
			return nil
		}
	case types.SigFormat_SIG_FORMAT_ADR036:
		doc := adr036SignDoc(signerAddr.String(), payload)
		if pk.VerifySignature(doc, sig) {
			return nil
		}
	default:
		return types.ErrInvalidLegacyProof.Wrap("sig_format unspecified")
	}
	return types.ErrInvalidLegacySignature
}
```

- [ ] **Step 2: Run compile check**

Run: `go build ./x/evmigration/keeper/...`
Expected: May fail on existing references to `LegacyPubKey` / `LegacySignature` — those are fixed in task 10. The new helper itself should compile.

Run: `go vet ./x/evmigration/keeper/` and confirm the helper is the only new symbol.

- [ ] **Step 3: Commit**

```bash
git add x/evmigration/keeper/verify.go
git commit -m "evmigration: add shared verifySecp256k1Sig helper

Consolidates CLI (SHA256) vs ADR-036 (canonical JSON) dispatch so
single-key and multisig paths share one implementation."
```

---

### Task 8: Implement `verifySingleKeyProof`

**Files:**
- Modify: `x/evmigration/keeper/verify.go`

- [ ] **Step 1: Add verifySingleKeyProof**

Edit `x/evmigration/keeper/verify.go`. Add:

```go
// verifySingleKeyProof validates a SingleKeyProof against the migration payload.
func verifySingleKeyProof(payload []byte, legacyAddr sdk.AccAddress, p *types.SingleKeyProof) error {
	if len(p.PubKey) != secp256k1.PubKeySize {
		return types.ErrInvalidLegacyPubKey.Wrapf("expected %d bytes, got %d", secp256k1.PubKeySize, len(p.PubKey))
	}
	pk := &secp256k1.PubKey{Key: p.PubKey}
	derived := sdk.AccAddress(pk.Address())
	if !derived.Equals(legacyAddr) {
		return types.ErrPubKeyAddressMismatch.Wrapf(
			"pubkey derives to %s, expected %s", derived, legacyAddr)
	}
	return verifySecp256k1Sig(pk, legacyAddr, payload, p.Signature, p.SigFormat)
}
```

- [ ] **Step 2: Commit intermediate progress**

```bash
git add x/evmigration/keeper/verify.go
git commit -m "evmigration: add verifySingleKeyProof"
```

---

### Task 9: Implement `verifyMultisigProof`

**Files:**
- Modify: `x/evmigration/keeper/verify.go`

- [ ] **Step 1: Add import for kmultisig and cryptotypes**

Edit `x/evmigration/keeper/verify.go` imports:

```go
import (
	// ...existing imports...
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)
```

- [ ] **Step 2: Add verifyMultisigProof**

Add to `x/evmigration/keeper/verify.go`:

```go
// verifyMultisigProof validates a MultisigProof against the migration payload.
// Reconstructs the LegacyAminoPubKey from sub-keys + threshold, confirms it
// derives to legacyAddr, then verifies each sub-signature against its
// claimed sub-key.
func verifyMultisigProof(payload []byte, legacyAddr sdk.AccAddress, m *types.MultisigProof) error {
	subPubKeys := make([]cryptotypes.PubKey, len(m.SubPubKeys))
	for i, raw := range m.SubPubKeys {
		if len(raw) != secp256k1.PubKeySize {
			return types.ErrInvalidLegacyPubKey.Wrapf("sub_pub_keys[%d]: expected %d bytes, got %d",
				i, secp256k1.PubKeySize, len(raw))
		}
		subPubKeys[i] = &secp256k1.PubKey{Key: raw}
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(int(m.Threshold), subPubKeys)
	derived := sdk.AccAddress(multiPK.Address())
	if !derived.Equals(legacyAddr) {
		return types.ErrPubKeyAddressMismatch.Wrapf(
			"multisig pubkey derives to %s, expected %s", derived, legacyAddr)
	}
	for i, idx := range m.SignerIndices {
		if int(idx) >= len(subPubKeys) {
			return types.ErrInvalidLegacyProof.Wrapf(
				"signer_indices[%d]=%d out of range", i, idx)
		}
		signerPK, ok := subPubKeys[idx].(*secp256k1.PubKey)
		if !ok {
			return types.ErrInvalidLegacyPubKey.Wrap("sub-key not secp256k1 (should be unreachable)")
		}
		signerAddr := sdk.AccAddress(signerPK.Address())
		if err := verifySecp256k1Sig(signerPK, signerAddr, payload, m.SubSignatures[i], m.SigFormat); err != nil {
			return types.ErrInvalidLegacySignature.Wrapf(
				"sub-sig %d (signer %s) invalid: %s", i, signerAddr, err)
		}
	}
	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add x/evmigration/keeper/verify.go
git commit -m "evmigration: add verifyMultisigProof

Reconstructs kmultisig.LegacyAminoPubKey from sub-keys + threshold,
verifies address derivation, verifies each sub-signature against its
claimed sub-key using the shared verifySecp256k1Sig helper."
```

---

### Task 10: Replace `VerifyLegacySignature` with `VerifyLegacyProof`; update callers

**Files:**
- Modify: `x/evmigration/keeper/verify.go`
- Modify: `x/evmigration/keeper/msg_server_claim_legacy.go`
- Modify: `x/evmigration/keeper/msg_server_migrate_validator.go`
- Modify: `x/evmigration/keeper/verify_test.go` (update existing tests that used `VerifyLegacySignature`)

- [ ] **Step 1: Add VerifyLegacyProof top-level dispatcher**

Edit `x/evmigration/keeper/verify.go`. Add, and delete the old `VerifyLegacySignature`:

```go
// VerifyLegacyProof verifies a migration proof against the canonical payload.
// Replaces the previous VerifyLegacySignature; the new shape accommodates both
// single-key and multisig legacy accounts via the LegacyProof oneof.
func VerifyLegacyProof(
	chainID string, evmChainID uint64, kind string,
	legacyAddr, newAddr sdk.AccAddress,
	proof *types.LegacyProof,
) error {
	payload := migrationPayload(chainID, evmChainID, kind, legacyAddr, newAddr)
	switch p := proof.Proof.(type) {
	case *types.LegacyProof_Single:
		return verifySingleKeyProof(payload, legacyAddr, p.Single)
	case *types.LegacyProof_Multisig:
		return verifyMultisigProof(payload, legacyAddr, p.Multisig)
	default:
		return types.ErrInvalidLegacyProof.Wrap("no proof set")
	}
}
```

Delete the old `VerifyLegacySignature` function in its entirety.

- [ ] **Step 2: Update msg_server_claim_legacy.go**

Edit `x/evmigration/keeper/msg_server_claim_legacy.go`. Replace lines 42-45 with:

```go
	// Enforce governance-adjustable multisig cap before crypto work.
	params, err := ms.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	if err := msg.LegacyProof.ValidateParams(params.MaxMultisigSubKeys); err != nil {
		return nil, err
	}

	// Verify both embedded proofs before touching state.
	if err := VerifyLegacyProof(ctx.ChainID(), lcfg.EVMChainID, migrationPayloadKindClaim, legacyAddr, newAddr, &msg.LegacyProof); err != nil {
		return nil, err
	}
```

Remove the duplicate `params` fetch earlier in `preChecks` if it becomes redundant — but keep `preChecks` intact; do not reshuffle unrelated logic.

- [ ] **Step 3: Update msg_server_migrate_validator.go identically**

Apply the same edit pattern in `x/evmigration/keeper/msg_server_migrate_validator.go`, substituting `migrationPayloadKindValidator`.

- [ ] **Step 4: Update existing verify_test.go to use the new signature**

Edit `x/evmigration/keeper/verify_test.go`. Existing tests call `keeper.VerifyLegacySignature(...)` with `(pubKeyBytes, sig)` — rewrite each call site to construct a `&types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{PubKey: ..., Signature: ..., SigFormat: types.SigFormat_SIG_FORMAT_CLI}}}` and call `keeper.VerifyLegacyProof(...)` with it.

Replacement example for an existing test like:

```go
err := keeper.VerifyLegacySignature(testChainID, lcfg.EVMChainID, "claim", legacyAddr, newAddr, pubKey, sig)
```

becomes:

```go
proof := &types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
	PubKey: pubKey, Signature: sig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
}}}
err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, "claim", legacyAddr, newAddr, proof)
```

Apply the same transformation to every `VerifyLegacySignature` call in the file.

Also update any test that invokes ADR-036 to set `SigFormat: types.SigFormat_SIG_FORMAT_ADR036`.

- [ ] **Step 5: Run verify tests**

Run: `go test ./x/evmigration/keeper/ -run TestVerifyLegacy -v`
Expected: All existing single-key tests PASS with the new signature.

- [ ] **Step 6: Commit**

```bash
git add x/evmigration/keeper/verify.go x/evmigration/keeper/msg_server_claim_legacy.go x/evmigration/keeper/msg_server_migrate_validator.go x/evmigration/keeper/verify_test.go
git commit -m "evmigration: replace VerifyLegacySignature with VerifyLegacyProof

Top-level verifier dispatches on the LegacyProof oneof to either
verifySingleKeyProof or verifyMultisigProof. Msg servers call
LegacyProof.ValidateParams(MaxMultisigSubKeys) before crypto work.
All existing single-key tests pass under the new call shape."
```

---

### Task 11: Add multisig test cases to verify_test.go

**Files:**
- Modify: `x/evmigration/keeper/verify_test.go`

- [ ] **Step 1: Add multisig test helpers**

Edit `x/evmigration/keeper/verify_test.go`. Append:

```go
// makeMultisigAccount creates N secp256k1 sub-keys and the resulting
// LegacyAminoPubKey for a K-of-N multisig.
func makeMultisigAccount(t *testing.T, threshold, n int) (*kmultisig.LegacyAminoPubKey, []*secp256k1.PrivKey, sdk.AccAddress) {
	t.Helper()
	privKeys := make([]*secp256k1.PrivKey, n)
	pubKeys := make([]cryptotypes.PubKey, n)
	for i := 0; i < n; i++ {
		privKeys[i] = secp256k1.GenPrivKey()
		pubKeys[i] = privKeys[i].PubKey()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(threshold, pubKeys)
	addr := sdk.AccAddress(multiPK.Address())
	return multiPK, privKeys, addr
}

func buildMultisigProof(t *testing.T, kind string, multiPK *kmultisig.LegacyAminoPubKey, privKeys []*secp256k1.PrivKey, signerIdxs []int, legacyAddr, newAddr sdk.AccAddress, format types.SigFormat) *types.LegacyProof {
	t.Helper()
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		testChainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())
	hash := sha256.Sum256([]byte(payload))

	subPubKeys := make([][]byte, len(multiPK.GetPubKeys()))
	for i, pk := range multiPK.GetPubKeys() {
		subPubKeys[i] = pk.Bytes()
	}

	indices := make([]uint32, len(signerIdxs))
	sigs := make([][]byte, len(signerIdxs))
	for i, idx := range signerIdxs {
		indices[i] = uint32(idx)
		if format == types.SigFormat_SIG_FORMAT_ADR036 {
			// ADR-036 signer is the sub-key's individual bech32.
			signerAddr := sdk.AccAddress(privKeys[idx].PubKey().Address())
			doc := fmt.Appendf(nil, `{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},"memo":"","msgs":[{"type":"sign/MsgSignData","value":{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
				base64.StdEncoding.EncodeToString([]byte(payload)), signerAddr.String())
			sig, err := privKeys[idx].Sign(doc)
			require.NoError(t, err)
			sigs[i] = sig
			continue
		}
		sig, err := privKeys[idx].Sign(hash[:])
		require.NoError(t, err)
		sigs[i] = sig
	}
	return &types.LegacyProof{Proof: &types.LegacyProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     uint32(multiPK.Threshold),
		SubPubKeys:    subPubKeys,
		SignerIndices: indices,
		SubSignatures: sigs,
		SigFormat:     format,
	}}}
}
```

Add these imports if not already present:

```go
import (
	// ...existing...
	"encoding/base64"
	"sort"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
)
```

- [ ] **Step 2: Write failing multisig tests**

Add to `verify_test.go`:

```go
func TestVerifyLegacyProof_Multisig_Valid_CLI(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, "claim", multiPK, privs, []int{0, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	require.NoError(t, proof.ValidateBasic())
	require.NoError(t, keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, "claim", legacyAddr, newAddr, proof))
}

func TestVerifyLegacyProof_Multisig_Valid_ADR036(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, "claim", multiPK, privs, []int{1, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_ADR036)
	require.NoError(t, proof.ValidateBasic())
	require.NoError(t, keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, "claim", legacyAddr, newAddr, proof))
}

func TestVerifyLegacyProof_Multisig_1of1(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 1, 1)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, "claim", multiPK, privs, []int{0}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	require.NoError(t, keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, "claim", legacyAddr, newAddr, proof))
}

func TestVerifyLegacyProof_Multisig_WrongAddress(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, "claim", multiPK, privs, []int{0, 1}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)

	// Claim a different legacy address.
	bogusAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, "claim", bogusAddr, newAddr, proof)
	require.ErrorContains(t, err, "multisig pubkey derives to")
}

func TestVerifyLegacyProof_Multisig_InvalidSubSig(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 2, 3)
	_, newAddr := testNewMigrationAccount(t)
	proof := buildMultisigProof(t, "claim", multiPK, privs, []int{0, 1}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	// Corrupt the second sub-signature.
	proof.GetMultisig().SubSignatures[1][0] ^= 0xFF
	err := keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, "claim", legacyAddr, newAddr, proof)
	require.ErrorContains(t, err, "sub-sig 1")
}

func TestVerifyLegacyProof_Multisig_MaxBoundary(t *testing.T) {
	multiPK, privs, legacyAddr := makeMultisigAccount(t, 20, 20)
	_, newAddr := testNewMigrationAccount(t)
	signerIdxs := make([]int, 20)
	for i := range signerIdxs {
		signerIdxs[i] = i
	}
	proof := buildMultisigProof(t, "claim", multiPK, privs, signerIdxs, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	require.NoError(t, proof.ValidateBasic())
	require.NoError(t, proof.ValidateParams(20))
	require.NoError(t, keeper.VerifyLegacyProof(testChainID, lcfg.EVMChainID, "claim", legacyAddr, newAddr, proof))

	// Same proof should fail the param cap when MaxMultisigSubKeys=19.
	require.ErrorContains(t, proof.ValidateParams(19), "exceeds max 19")
}
```

Sort signerIdxs ascending in `buildMultisigProof` before building — add at top of the function:

```go
sort.Ints(signerIdxs)
```

- [ ] **Step 3: Run tests**

Run: `go test ./x/evmigration/keeper/ -run 'TestVerifyLegacyProof_Multisig' -v`
Expected: All six tests PASS.

- [ ] **Step 4: Commit**

```bash
git add x/evmigration/keeper/verify_test.go
git commit -m "evmigration: add multisig verifier tests

Covers valid CLI + ADR-036, 1-of-1 edge, wrong-address rejection,
corrupted sub-signature rejection, N=20 boundary, and param cap."
```

---

## Phase 4 — Query & Legacy-Account Detection

### Task 12: Update `isLegacyPubKey` and `remainingLegacyAccountStatus`

**Files:**
- Modify: `x/evmigration/keeper/query.go`

- [ ] **Step 1: Add isLegacyPubKey helper and update remainingLegacyAccountStatus**

Edit `x/evmigration/keeper/query.go`. Add at package level:

```go
// isLegacyPubKey reports whether pk is a key type migratable by the
// evmigration module: either a plain secp256k1.PubKey or a flat multisig
// where every sub-key is secp256k1.
func isLegacyPubKey(pk cryptotypes.PubKey) bool {
	switch key := pk.(type) {
	case *secp256k1.PubKey:
		return true
	case *kmultisig.LegacyAminoPubKey:
		for _, sub := range key.GetPubKeys() {
			if _, ok := sub.(*secp256k1.PubKey); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}
```

Add imports:

```go
cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
```

Replace the pubkey-type check in `remainingLegacyAccountStatus`. Current:

```go
pk := acc.GetPubKey()
if pk != nil {
	if _, ok := pk.(*secp256k1.PubKey); !ok {
		return status, false
	}
}
```

becomes:

```go
pk := acc.GetPubKey()
if pk != nil && !isLegacyPubKey(pk) {
	return status, false
}
```

- [ ] **Step 2: Run existing query tests**

Run: `go test ./x/evmigration/keeper/ -run 'TestLegacyAccounts|TestMigrationStats' -v`
Expected: existing single-key tests still PASS.

- [ ] **Step 3: Commit**

```bash
git add x/evmigration/keeper/query.go
git commit -m "evmigration: include multisig accounts in legacy account detection

isLegacyPubKey now accepts kmultisig.LegacyAminoPubKey whose sub-keys are
all secp256k1. Nil-pubkey accounts remain supported for single-key
migration (multisig nil-pubkey is out of scope — documented)."
```

---

### Task 13: Populate `LegacyAccountInfo` multisig fields

**Files:**
- Modify: `x/evmigration/keeper/query.go`

- [ ] **Step 1: Extend LegacyAccounts to set is_multisig/threshold/num_signers**

Edit `x/evmigration/keeper/query.go`. Inside `LegacyAccounts` where `info := types.LegacyAccountInfo{...}` is populated, add after the existing field assignments:

```go
			if pk := acc.GetPubKey(); pk != nil {
				if ms, ok := pk.(*kmultisig.LegacyAminoPubKey); ok {
					info.IsMultisig = true
					info.Threshold = uint32(ms.Threshold)
					info.NumSigners = uint32(len(ms.GetPubKeys()))
				}
			}
```

- [ ] **Step 2: Write failing test**

Add to `x/evmigration/keeper/query_test.go`:

```go
func TestLegacyAccounts_Multisig(t *testing.T) {
	k, ctx := keepertest.EvmigrationKeeper(t)
	accKeeper := k.AccountKeeper() // if available, else rely on the testutil setup method
	// Create a 2-of-3 multisig account with a funded balance.
	privs := make([]*secp256k1.PrivKey, 3)
	pubs := make([]cryptotypes.PubKey, 3)
	for i := 0; i < 3; i++ {
		privs[i] = secp256k1.GenPrivKey()
		pubs[i] = privs[i].PubKey()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(2, pubs)
	addr := sdk.AccAddress(multiPK.Address())
	acc := accKeeper.NewAccountWithAddress(ctx, addr)
	require.NoError(t, acc.SetPubKey(multiPK))
	accKeeper.SetAccount(ctx, acc)
	// Fund it (exact bank API depends on testutil; for illustration use GetBankKeeper() if available).
	require.NoError(t, k.BankKeeper().MintCoins(ctx, "mint", sdk.NewCoins(sdk.NewCoin("ulume", sdk.NewInt(1000)))))
	require.NoError(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, "mint", addr, sdk.NewCoins(sdk.NewCoin("ulume", sdk.NewInt(1000)))))

	resp, err := keeper.NewQueryServerImpl(k).LegacyAccounts(ctx, &types.QueryLegacyAccountsRequest{})
	require.NoError(t, err)
	var found *types.LegacyAccountInfo
	for i := range resp.Accounts {
		if resp.Accounts[i].Address == addr.String() {
			found = &resp.Accounts[i]
			break
		}
	}
	require.NotNil(t, found, "multisig account must be in legacy list")
	require.True(t, found.IsMultisig)
	require.Equal(t, uint32(2), found.Threshold)
	require.Equal(t, uint32(3), found.NumSigners)
}
```

Note: the exact helpers for keeper construction live in `testutil/keeper/evmigration.go`. If that testutil does not expose the bank keeper, add a helper there first:

```go
// testutil/keeper/evmigration.go — add if missing
func (k *Keeper) BankKeeper() bankkeeper.Keeper { return k.bankKeeper }
```

- [ ] **Step 3: Run tests and verify**

Run: `go test ./x/evmigration/keeper/ -run TestLegacyAccounts_Multisig -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add x/evmigration/keeper/query.go x/evmigration/keeper/query_test.go
git commit -m "evmigration: LegacyAccounts populates multisig fields"
```

---

### Task 14: Extend `MigrationEstimate` with multisig feasibility

**Files:**
- Modify: `x/evmigration/keeper/query.go`

- [ ] **Step 1: Add multisig feasibility branch**

Edit `x/evmigration/keeper/query.go` `MigrationEstimate`. Immediately before the final `return resp, nil`, add:

```go
	// Multisig feasibility preflight.
	params, _ := qs.k.Params.Get(ctx)
	if acc := qs.k.accountKeeper.GetAccount(ctx, addr); acc != nil {
		if pk := acc.GetPubKey(); pk != nil {
			if ms, ok := pk.(*kmultisig.LegacyAminoPubKey); ok {
				resp.IsMultisig = true
				resp.Threshold = uint32(ms.Threshold)
				resp.NumSigners = uint32(len(ms.GetPubKeys()))

				// Reject nested / non-secp256k1 sub-keys.
				for _, sub := range ms.GetPubKeys() {
					if _, ok := sub.(*secp256k1.PubKey); !ok {
						resp.WouldSucceed = false
						resp.RejectionReason = "multisig contains non-secp256k1 sub-key (unsupported)"
						break
					}
				}
				if resp.WouldSucceed && resp.NumSigners > params.MaxMultisigSubKeys {
					resp.WouldSucceed = false
					resp.RejectionReason = fmt.Sprintf("multisig has %d sub-keys; max is %d",
						resp.NumSigners, params.MaxMultisigSubKeys)
				}
			}
		}
		// Nil pubkey: cannot distinguish single-key vs multisig; detection
		// deferred to the CLI's generate-proof-payload command.
	}
```

Add import `"fmt"` if not already.

- [ ] **Step 2: Write failing tests**

Add to `query_test.go`:

```go
func TestMigrationEstimate_Multisig_Supported(t *testing.T) {
	k, ctx := keepertest.EvmigrationKeeper(t)
	accKeeper := k.AccountKeeper()
	pubs := make([]cryptotypes.PubKey, 3)
	for i := 0; i < 3; i++ {
		pubs[i] = secp256k1.GenPrivKey().PubKey()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(2, pubs)
	addr := sdk.AccAddress(multiPK.Address())
	acc := accKeeper.NewAccountWithAddress(ctx, addr)
	require.NoError(t, acc.SetPubKey(multiPK))
	accKeeper.SetAccount(ctx, acc)

	resp, err := keeper.NewQueryServerImpl(k).MigrationEstimate(ctx, &types.QueryMigrationEstimateRequest{LegacyAddress: addr.String()})
	require.NoError(t, err)
	require.True(t, resp.IsMultisig)
	require.Equal(t, uint32(2), resp.Threshold)
	require.Equal(t, uint32(3), resp.NumSigners)
	require.True(t, resp.WouldSucceed)
}

func TestMigrationEstimate_Multisig_TooManySubKeys(t *testing.T) {
	k, ctx := keepertest.EvmigrationKeeper(t)
	accKeeper := k.AccountKeeper()
	// Build N=21 > default cap 20.
	pubs := make([]cryptotypes.PubKey, 21)
	for i := 0; i < 21; i++ {
		pubs[i] = secp256k1.GenPrivKey().PubKey()
	}
	multiPK := kmultisig.NewLegacyAminoPubKey(1, pubs)
	addr := sdk.AccAddress(multiPK.Address())
	acc := accKeeper.NewAccountWithAddress(ctx, addr)
	require.NoError(t, acc.SetPubKey(multiPK))
	accKeeper.SetAccount(ctx, acc)

	resp, err := keeper.NewQueryServerImpl(k).MigrationEstimate(ctx, &types.QueryMigrationEstimateRequest{LegacyAddress: addr.String()})
	require.NoError(t, err)
	require.True(t, resp.IsMultisig)
	require.False(t, resp.WouldSucceed)
	require.Contains(t, resp.RejectionReason, "max is 20")
}

func TestMigrationEstimate_Multisig_NonSecp256k1SubKey(t *testing.T) {
	k, ctx := keepertest.EvmigrationKeeper(t)
	accKeeper := k.AccountKeeper()
	// Build a multisig where one sub-key is ed25519.
	sec := secp256k1.GenPrivKey().PubKey()
	ed := ed25519.GenPrivKey().PubKey()
	multiPK := kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{sec, ed})
	addr := sdk.AccAddress(multiPK.Address())
	acc := accKeeper.NewAccountWithAddress(ctx, addr)
	require.NoError(t, acc.SetPubKey(multiPK))
	accKeeper.SetAccount(ctx, acc)

	resp, err := keeper.NewQueryServerImpl(k).MigrationEstimate(ctx, &types.QueryMigrationEstimateRequest{LegacyAddress: addr.String()})
	require.NoError(t, err)
	require.True(t, resp.IsMultisig)
	require.False(t, resp.WouldSucceed)
	require.Contains(t, resp.RejectionReason, "non-secp256k1")
}
```

Add import:

```go
"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
```

- [ ] **Step 3: Run tests**

Run: `go test ./x/evmigration/keeper/ -run 'TestMigrationEstimate_Multisig' -v`
Expected: 3 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add x/evmigration/keeper/query.go x/evmigration/keeper/query_test.go
git commit -m "evmigration: MigrationEstimate surfaces multisig feasibility

Rejects nested/non-secp256k1 sub-keys and over-cap N with descriptive
RejectionReason. Nil-pubkey case intentionally not flagged (documented
in design spec section 4.4.1)."
```

---

## Phase 5 — Update Existing One-Shot CLI Commands

### Task 15: Build `LegacyProof{Single:...}` in existing CLI commands

**Files:**
- Modify: `x/evmigration/client/cli/tx.go`

- [ ] **Step 1: Update resolveClaimMsg and resolveValidatorMsg to build LegacyProof**

Edit `x/evmigration/client/cli/tx.go` `resolveClaimMsg`. Replace the return construction:

```go
	return &types.MsgClaimLegacyAccount{
		NewAddress:    newAddr,
		LegacyAddress: legacyAddr,
		LegacyProof: types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
			PubKey:    pubKey,
			Signature: sig,
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}, newKeyName, nil
```

Same transformation for `resolveValidatorMsg`:

```go
	return &types.MsgMigrateValidator{
		NewAddress:    newAddr,
		LegacyAddress: legacyAddr,
		LegacyProof: types.LegacyProof{Proof: &types.LegacyProof_Single{Single: &types.SingleKeyProof{
			PubKey:    pubKey,
			Signature: sig,
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
	}, newKeyName, nil
```

- [ ] **Step 2: Run CLI tests**

Run: `go test ./x/evmigration/client/cli/ -v`
Expected: existing tests that exercise the old flat fields fail. Update each failing test to inspect `msg.LegacyProof.GetSingle().PubKey` instead of `msg.LegacyPubKey`, and `.GetSingle().Signature` instead of `.LegacySignature`.

- [ ] **Step 3: Run full build**

Run: `go build ./x/evmigration/... ./app/...`
Expected: everything builds.

- [ ] **Step 4: Commit**

```bash
git add x/evmigration/client/cli/tx.go x/evmigration/client/cli/tx_test.go
git commit -m "evmigration: one-shot CLI commands build LegacyProof{Single}"
```

---

### Task 16: Mark AutoCLI descriptors `Skip:true`

**Files:**
- Modify: `x/evmigration/module/autocli.go`

- [ ] **Step 1: Skip the two tx descriptors**

Edit `x/evmigration/module/autocli.go`. Replace the `ClaimLegacyAccount` and `MigrateValidator` RpcCommandOptions entries with:

```go
{
	RpcMethod: "ClaimLegacyAccount",
	Skip:      true, // custom hand-written command in x/evmigration/client/cli/tx.go
},
{
	RpcMethod: "MigrateValidator",
	Skip:      true, // custom hand-written command in x/evmigration/client/cli/tx.go
},
```

Remove the now-obsolete PositionalArgs lines for both.

- [ ] **Step 2: Build binary and verify the commands still exist**

Run: `make build`
Then: `./build/lumerad tx evmigration --help`
Expected: `claim-legacy-account` and `migrate-validator` subcommands are listed (from the hand-written GetTxCmd).

- [ ] **Step 3: Commit**

```bash
git add x/evmigration/module/autocli.go
git commit -m "evmigration: skip AutoCLI for claim-legacy-account / migrate-validator

Both messages carry a LegacyProof oneof which AutoCLI cannot render as
positional args. Rely entirely on the hand-written commands in
x/evmigration/client/cli/tx.go and tx_multisig.go."
```

---

## Phase 6 — New CLI Multi-Step Flow

### Task 17: `PartialProof` JSON schema and shared CLI helpers

**Files:**
- Create: `x/evmigration/client/cli/tx_multisig.go`

- [ ] **Step 1: Create the PartialProof types and file I/O helpers**

Create `x/evmigration/client/cli/tx_multisig.go`:

```go
package cli

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// partialProofVersion is the current on-disk format version.
const partialProofVersion = 1

// PartialProof is a coordination artifact passed between co-signers.
// It is never stored on-chain and does not need to round-trip via proto.
type PartialProof struct {
	Version       int                  `json:"version"`
	Kind          string               `json:"kind"` // "claim" | "validator"
	LegacyAddress string               `json:"legacy_address"`
	NewAddress    string               `json:"new_address"`
	ChainID       string               `json:"chain_id"`
	EVMChainID    uint64               `json:"evm_chain_id"`
	PayloadHex    string               `json:"payload_hex"`
	Single        *PartialSingle       `json:"single,omitempty"`
	Multisig      *PartialMultisig     `json:"multisig,omitempty"`
	PartialSigs   []PartialSubSignature `json:"partial_signatures"`
}

type PartialSingle struct {
	PubKeyB64 string `json:"pub_key_b64"`
	SigFormat string `json:"sig_format"` // "SIG_FORMAT_CLI" | "SIG_FORMAT_ADR036"
}

type PartialMultisig struct {
	Threshold    uint32   `json:"threshold"`
	SubPubKeysB64 []string `json:"sub_pub_keys_b64"`
	SigFormat    string   `json:"sig_format"`
}

type PartialSubSignature struct {
	Index        uint32 `json:"index"`
	SignatureB64 string `json:"signature_b64"`
}

// MarshalIndent writes JSON with 2-space indent for human-readable review.
func (pp *PartialProof) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(pp, "", "  ")
}

// LoadPartialProof reads a PartialProof JSON file and validates its version.
func LoadPartialProof(path string) (*PartialProof, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var pp PartialProof
	if err := json.Unmarshal(b, &pp); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if pp.Version != partialProofVersion {
		return nil, fmt.Errorf("unsupported partial_proof version %d (expected %d)", pp.Version, partialProofVersion)
	}
	if pp.Single == nil && pp.Multisig == nil {
		return nil, fmt.Errorf("partial proof has neither 'single' nor 'multisig' section")
	}
	if pp.Single != nil && pp.Multisig != nil {
		return nil, fmt.Errorf("partial proof has both 'single' and 'multisig' sections")
	}
	return &pp, nil
}

// SavePartialProof writes a PartialProof to disk with 0600 mode (contains no
// secrets, but conservative for cold-wallet environments).
func SavePartialProof(path string, pp *PartialProof) error {
	b, err := pp.MarshalIndent()
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ParseSigFormat converts the JSON string to a proto enum.
func ParseSigFormat(s string) (types.SigFormat, error) {
	switch s {
	case "SIG_FORMAT_CLI":
		return types.SigFormat_SIG_FORMAT_CLI, nil
	case "SIG_FORMAT_ADR036":
		return types.SigFormat_SIG_FORMAT_ADR036, nil
	default:
		return types.SigFormat_SIG_FORMAT_UNSPECIFIED, fmt.Errorf("unknown sig_format %q", s)
	}
}

// SigFormatString is the inverse of ParseSigFormat.
func SigFormatString(f types.SigFormat) string {
	switch f {
	case types.SigFormat_SIG_FORMAT_CLI:
		return "SIG_FORMAT_CLI"
	case types.SigFormat_SIG_FORMAT_ADR036:
		return "SIG_FORMAT_ADR036"
	default:
		return "SIG_FORMAT_UNSPECIFIED"
	}
}

// decodeSubPubKeys decodes the base64 sub-pubkeys from a PartialMultisig.
func decodeSubPubKeys(ms *PartialMultisig) ([][]byte, error) {
	out := make([][]byte, len(ms.SubPubKeysB64))
	for i, s := range ms.SubPubKeysB64 {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("sub_pub_keys_b64[%d]: %w", i, err)
		}
		if len(b) != secp256k1.PubKeySize {
			return nil, fmt.Errorf("sub_pub_keys_b64[%d]: expected %d bytes, got %d",
				i, secp256k1.PubKeySize, len(b))
		}
		out[i] = b
	}
	return out, nil
}

// assembleMultisigProof merges partial sub-signatures into a MultisigProof.
// Signatures are deduplicated by index (last write wins). If fewer than
// threshold valid entries are present, returns an error.
func assembleMultisigProof(ms *PartialMultisig, partials []PartialSubSignature) (*types.MultisigProof, error) {
	sigFmt, err := ParseSigFormat(ms.SigFormat)
	if err != nil {
		return nil, err
	}
	subs, err := decodeSubPubKeys(ms)
	if err != nil {
		return nil, err
	}
	byIdx := map[uint32][]byte{}
	for _, p := range partials {
		if int(p.Index) >= len(subs) {
			return nil, fmt.Errorf("partial signature index %d out of range (N=%d)", p.Index, len(subs))
		}
		sig, err := base64.StdEncoding.DecodeString(p.SignatureB64)
		if err != nil {
			return nil, fmt.Errorf("partial signature %d: %w", p.Index, err)
		}
		byIdx[p.Index] = sig
	}
	if uint32(len(byIdx)) < ms.Threshold {
		return nil, fmt.Errorf("need %d partial signatures, have %d", ms.Threshold, len(byIdx))
	}
	indices := make([]uint32, 0, len(byIdx))
	for idx := range byIdx {
		indices = append(indices, idx)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	indices = indices[:ms.Threshold]
	sigs := make([][]byte, len(indices))
	for i, idx := range indices {
		sigs[i] = byIdx[idx]
	}
	return &types.MultisigProof{
		Threshold:     ms.Threshold,
		SubPubKeys:    subs,
		SignerIndices: indices,
		SubSignatures: sigs,
		SigFormat:     sigFmt,
	}, nil
}

// assembleSingleProof builds a SingleKeyProof from a single-entry partial.
func assembleSingleProof(ss *PartialSingle, partials []PartialSubSignature) (*types.SingleKeyProof, error) {
	sigFmt, err := ParseSigFormat(ss.SigFormat)
	if err != nil {
		return nil, err
	}
	pub, err := base64.StdEncoding.DecodeString(ss.PubKeyB64)
	if err != nil {
		return nil, fmt.Errorf("pub_key_b64: %w", err)
	}
	if len(partials) < 1 {
		return nil, fmt.Errorf("need 1 partial signature for single-key proof")
	}
	// Pick the last one (idempotent re-sign) with index 0.
	var sigB64 string
	for _, p := range partials {
		if p.Index != 0 {
			return nil, fmt.Errorf("single-key proof must have index=0, got %d", p.Index)
		}
		sigB64 = p.SignatureB64
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("signature_b64: %w", err)
	}
	return &types.SingleKeyProof{PubKey: pub, Signature: sig, SigFormat: sigFmt}, nil
}

// ComputePayload builds the canonical migration payload bytes. Exported for
// testing (see tx_multisig_test.go).
func ComputePayload(chainID string, evmChainID uint64, kind, legacyAddr, newAddr string) string {
	return fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s", chainID, evmChainID, kind, legacyAddr, newAddr)
}

// hexEncode encodes payload bytes to hex for the PartialProof.PayloadHex field.
func hexEncode(b []byte) string { return hex.EncodeToString(b) }

// unused-but-imported guard (signing mode used by sign-proof in Task 19)
var _ = signingtypes.SignMode_SIGN_MODE_UNSPECIFIED
```

- [ ] **Step 2: Verify file compiles**

Run: `go build ./x/evmigration/client/cli/`
Expected: compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add x/evmigration/client/cli/tx_multisig.go
git commit -m "evmigration: add PartialProof JSON helpers for multi-step CLI

PartialProof is a coordination artifact (never on-chain) carrying the
payload, pubkey material, and accumulated sub-signatures across the
generate/sign/combine/submit offline flow."
```

---

### Task 18: `generate-proof-payload` command

**Files:**
- Modify: `x/evmigration/client/cli/tx.go`
- Modify: `x/evmigration/client/cli/tx_multisig.go`
- Create: `x/evmigration/client/cli/tx_multisig_test.go`

- [ ] **Step 1: Add command registration**

Edit `x/evmigration/client/cli/tx.go`. In `GetTxCmd()`, add the four new commands:

```go
	evmigrationTxCmd.AddCommand(
		cmdClaimLegacyAccount(),
		cmdMigrateValidator(),
		cmdGenerateProofPayload(),
		cmdSignProof(),
		cmdCombineProof(),
		cmdSubmitProof(),
	)
```

- [ ] **Step 2: Implement cmdGenerateProofPayload**

Append to `x/evmigration/client/cli/tx_multisig.go`:

```go
const (
	flagLegacyAddr   = "legacy"
	flagNewAddr      = "new"
	flagKind         = "kind"
	flagEVMChainID   = "evm-chain-id"
	flagOut          = "out"
	flagLegacyKey    = "legacy-key"
	flagSigFormat    = "sig-format"
)

func cmdGenerateProofPayload() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-proof-payload",
		Short: "Generate a PartialProof template for offline multi-party signing",
		Long: `Generate an unsigned PartialProof JSON file for offline multi-party
coordination. For multisig accounts the sub-pubkeys and threshold are
read from the on-chain account record. For nil-pubkey single-key accounts,
pass --legacy-key to seed the pubkey from your local keyring.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			legacyStr, _ := cmd.Flags().GetString(flagLegacyAddr)
			newStr, _ := cmd.Flags().GetString(flagNewAddr)
			kind, _ := cmd.Flags().GetString(flagKind)
			evmChainID, _ := cmd.Flags().GetUint64(flagEVMChainID)
			out, _ := cmd.Flags().GetString(flagOut)
			legacyKey, _ := cmd.Flags().GetString(flagLegacyKey)
			sigFmtStr, _ := cmd.Flags().GetString(flagSigFormat)

			if kind != "claim" && kind != "validator" {
				return fmt.Errorf("--kind must be 'claim' or 'validator'")
			}
			if _, err := ParseSigFormat(sigFmtStr); err != nil {
				return err
			}
			if evmChainID == 0 {
				evmChainID = lcfg.EVMChainID
			}

			legacyAddr, err := sdk.AccAddressFromBech32(legacyStr)
			if err != nil {
				return fmt.Errorf("--legacy: %w", err)
			}
			if _, err := sdk.AccAddressFromBech32(newStr); err != nil {
				return fmt.Errorf("--new: %w", err)
			}

			// Query account and branch on pubkey shape.
			accResp, err := fetchAccount(clientCtx, legacyAddr)
			if err != nil {
				return err
			}
			pp := &PartialProof{
				Version:       partialProofVersion,
				Kind:          kind,
				LegacyAddress: legacyStr,
				NewAddress:    newStr,
				ChainID:       clientCtx.ChainID,
				EVMChainID:    evmChainID,
				PayloadHex:    hexEncode([]byte(ComputePayload(clientCtx.ChainID, evmChainID, kind, legacyStr, newStr))),
				PartialSigs:   []PartialSubSignature{},
			}

			switch pk := accResp.(type) {
			case *secp256k1.PubKey:
				if legacyKey != "" {
					// Optional validation.
					rec, err := clientCtx.Keyring.Key(legacyKey)
					if err != nil {
						return fmt.Errorf("--legacy-key %q not found: %w", legacyKey, err)
					}
					kp, err := rec.GetPubKey()
					if err != nil {
						return err
					}
					if !bytes.Equal(kp.Bytes(), pk.Bytes()) {
						return fmt.Errorf("--legacy-key pubkey does not match on-chain pubkey")
					}
				}
				pp.Single = &PartialSingle{
					PubKeyB64: base64.StdEncoding.EncodeToString(pk.Bytes()),
					SigFormat: sigFmtStr,
				}
			case *kmultisig.LegacyAminoPubKey:
				if legacyKey != "" {
					return fmt.Errorf("--legacy-key is not applicable for multisig accounts")
				}
				subs := make([]string, len(pk.GetPubKeys()))
				for i, k := range pk.GetPubKeys() {
					subs[i] = base64.StdEncoding.EncodeToString(k.Bytes())
				}
				pp.Multisig = &PartialMultisig{
					Threshold:     uint32(pk.Threshold),
					SubPubKeysB64: subs,
					SigFormat:     sigFmtStr,
				}
			case nil:
				// Nil pubkey: require --legacy-key and ONLY single-key path.
				if legacyKey == "" {
					return fmt.Errorf("account at %s has no on-chain pubkey record; pass --legacy-key to seed the pubkey from your keyring (single-sig only), or for a multisig address submit a 1-ulume self-send first", legacyAddr)
				}
				rec, err := clientCtx.Keyring.Key(legacyKey)
				if err != nil {
					return fmt.Errorf("--legacy-key %q not found: %w", legacyKey, err)
				}
				kp, err := rec.GetPubKey()
				if err != nil {
					return err
				}
				secp, ok := kp.(*secp256k1.PubKey)
				if !ok {
					return fmt.Errorf("--legacy-key %q is not secp256k1 (got %T)", legacyKey, kp)
				}
				if !sdk.AccAddress(secp.Address()).Equals(legacyAddr) {
					return fmt.Errorf("--legacy-key derives to %s, expected %s",
						sdk.AccAddress(secp.Address()), legacyAddr)
				}
				pp.Single = &PartialSingle{
					PubKeyB64: base64.StdEncoding.EncodeToString(secp.Bytes()),
					SigFormat: sigFmtStr,
				}
			default:
				return fmt.Errorf("unsupported pubkey type %T", pk)
			}

			if out == "" {
				// Write to stdout.
				b, err := pp.MarshalIndent()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return err
			}
			return SavePartialProof(out, pp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	cmd.Flags().String(flagLegacyAddr, "", "Legacy (coin-type 118) bech32 address to migrate from")
	cmd.Flags().String(flagNewAddr, "", "New (coin-type 60) bech32 destination address")
	cmd.Flags().String(flagKind, "claim", "'claim' for account migration or 'validator' for operator migration")
	cmd.Flags().Uint64(flagEVMChainID, 0, "EVM chain ID (defaults to lcfg.EVMChainID)")
	cmd.Flags().String(flagOut, "", "Output file path; if empty, writes JSON to stdout")
	cmd.Flags().String(flagLegacyKey, "", "Local keyring key name to seed pubkey for nil-pubkey single-sig accounts")
	cmd.Flags().String(flagSigFormat, "SIG_FORMAT_CLI", "Signing envelope: SIG_FORMAT_CLI or SIG_FORMAT_ADR036")
	_ = cmd.MarkFlagRequired(flagLegacyAddr)
	_ = cmd.MarkFlagRequired(flagNewAddr)
	return cmd
}

// fetchAccount queries the on-chain account and returns its pubkey (may be nil).
func fetchAccount(clientCtx client.Context, addr sdk.AccAddress) (cryptotypes.PubKey, error) {
	accRetriever := authtypes.AccountRetriever{}
	acc, err := accRetriever.GetAccount(clientCtx, addr)
	if err != nil {
		return nil, fmt.Errorf("query account %s: %w", addr, err)
	}
	return acc.GetPubKey(), nil // may be nil
}
```

Update imports in `tx_multisig.go`:

```go
import (
	// existing...
	"bytes"
	"github.com/cosmos/cosmos-sdk/client/flags"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/spf13/cobra"
	lcfg "github.com/LumeraProtocol/lumera/config"
)
```

- [ ] **Step 3: Write a unit test for payload construction (does not require network)**

Create `x/evmigration/client/cli/tx_multisig_test.go`:

```go
package cli_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/client/cli"
)

func TestComputePayload_StableFormat(t *testing.T) {
	got := "lumera-evm-migration:lumera-test-1:76857769:claim:lumera1abc:lumera1xyz"
	require.Equal(t, got, cli.ComputePayload("lumera-test-1", 76857769, "claim", "lumera1abc", "lumera1xyz"))
}

func TestPartialProof_RoundTrip(t *testing.T) {
	pp := &cli.PartialProof{
		Version:       1,
		Kind:          "claim",
		LegacyAddress: "lumera1abc",
		NewAddress:    "lumera1xyz",
		ChainID:       "lumera-test-1",
		EVMChainID:    76857769,
		PayloadHex:    hex.EncodeToString([]byte("p")),
		Single: &cli.PartialSingle{
			PubKeyB64: "AAAA",
			SigFormat: "SIG_FORMAT_CLI",
		},
		PartialSigs: []cli.PartialSubSignature{{Index: 0, SignatureB64: "BBBB"}},
	}
	b, err := pp.MarshalIndent()
	require.NoError(t, err)
	require.Contains(t, string(b), "SIG_FORMAT_CLI")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./x/evmigration/client/cli/ -run 'TestComputePayload_StableFormat|TestPartialProof_RoundTrip' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add x/evmigration/client/cli/tx_multisig.go x/evmigration/client/cli/tx_multisig_test.go x/evmigration/client/cli/tx.go
git commit -m "evmigration: add 'generate-proof-payload' CLI command

Queries on-chain account and produces a PartialProof JSON template.
Handles four quadrants of (pubkey-present × single/multisig × with/without
--legacy-key) with explicit errors for invalid combinations."
```

---

### Task 19: `sign-proof` command

**Files:**
- Modify: `x/evmigration/client/cli/tx_multisig.go`

- [ ] **Step 1: Implement cmdSignProof**

Append to `tx_multisig.go`:

```go
func cmdSignProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign-proof <partial-proof.json>",
		Short: "Append your sub-signature to a PartialProof file",
		Long: `Read a PartialProof JSON, match --from key against the proof's sub-keys
to determine its index, sign the canonical payload, and append a
PartialSubSignature entry. Re-signing with the same key overwrites the
previous entry at that index.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			fromKey := clientCtx.FromName
			out, _ := cmd.Flags().GetString(flagOut)
			if out == "" {
				out = args[0]
			}

			pp, err := LoadPartialProof(args[0])
			if err != nil {
				return err
			}

			rec, err := clientCtx.Keyring.Key(fromKey)
			if err != nil {
				return fmt.Errorf("--from key %q not found: %w", fromKey, err)
			}
			kp, err := rec.GetPubKey()
			if err != nil {
				return err
			}
			secp, ok := kp.(*secp256k1.PubKey)
			if !ok {
				return fmt.Errorf("--from key %q is not secp256k1 (got %T)", fromKey, kp)
			}

			payloadBytes, err := hex.DecodeString(pp.PayloadHex)
			if err != nil {
				return fmt.Errorf("decode payload_hex: %w", err)
			}

			// Figure out which index this key matches.
			var idx uint32
			var found bool
			switch {
			case pp.Single != nil:
				pub, err := base64.StdEncoding.DecodeString(pp.Single.PubKeyB64)
				if err != nil {
					return err
				}
				if !bytes.Equal(pub, secp.Bytes()) {
					return fmt.Errorf("--from key does not match single proof's pubkey")
				}
				idx = 0
				found = true
			case pp.Multisig != nil:
				for i, s := range pp.Multisig.SubPubKeysB64 {
					b, err := base64.StdEncoding.DecodeString(s)
					if err != nil {
						return err
					}
					if bytes.Equal(b, secp.Bytes()) {
						idx = uint32(i)
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("--from key is not a member of the multisig")
				}
			}
			if !found {
				return fmt.Errorf("partial proof has neither single nor multisig; cannot sign")
			}

			// Determine the signing format.
			var sigFmtStr string
			if pp.Single != nil {
				sigFmtStr = pp.Single.SigFormat
			} else {
				sigFmtStr = pp.Multisig.SigFormat
			}

			// Produce the signature.
			var sig []byte
			switch sigFmtStr {
			case "SIG_FORMAT_CLI":
				hash := sha256.Sum256(payloadBytes)
				sig, _, err = clientCtx.Keyring.Sign(fromKey, hash[:], signingtypes.SignMode_SIGN_MODE_UNSPECIFIED)
			case "SIG_FORMAT_ADR036":
				signerAddr := sdk.AccAddress(secp.Address()).String()
				doc := []byte(fmt.Sprintf(`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},"memo":"","msgs":[{"type":"sign/MsgSignData","value":{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
					base64.StdEncoding.EncodeToString(payloadBytes), signerAddr))
				sig, _, err = clientCtx.Keyring.Sign(fromKey, doc, signingtypes.SignMode_SIGN_MODE_UNSPECIFIED)
			default:
				return fmt.Errorf("unsupported sig_format %q", sigFmtStr)
			}
			if err != nil {
				return fmt.Errorf("sign: %w", err)
			}

			// Upsert (idempotent for re-sign): remove existing entry with the same index, then append.
			filtered := pp.PartialSigs[:0]
			for _, p := range pp.PartialSigs {
				if p.Index != idx {
					filtered = append(filtered, p)
				}
			}
			pp.PartialSigs = append(filtered, PartialSubSignature{
				Index:        idx,
				SignatureB64: base64.StdEncoding.EncodeToString(sig),
			})

			return SavePartialProof(out, pp)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagOut, "", "Write to this path instead of overwriting the input file")
	return cmd
}
```

Add imports:

```go
"crypto/sha256"
```

- [ ] **Step 2: Commit (tested via devnet/integration tests later)**

```bash
git add x/evmigration/client/cli/tx_multisig.go
git commit -m "evmigration: add 'sign-proof' CLI command

Reads a PartialProof JSON, matches --from key against the proof's
sub-keys, signs the payload in the proof's declared format, and
upserts the signature (idempotent for re-sign)."
```

---

### Task 20: `combine-proof` command

**Files:**
- Modify: `x/evmigration/client/cli/tx_multisig.go`

- [ ] **Step 1: Implement cmdCombineProof**

Append to `tx_multisig.go`:

```go
func cmdCombineProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "combine-proof <partial1.json> [<partial2.json> ...]",
		Short: "Merge partial proofs into an unsigned tx JSON",
		Long: `Combine one or more PartialProof files into a fully-assembled
unsigned tx JSON (MsgClaimLegacyAccount or MsgMigrateValidator without
new_signature, which is added by submit-proof).

All input files must agree on legacy_address, new_address, chain_id,
evm_chain_id, kind, sig_format, threshold, and sub_pub_keys (for
multisig). Partial signatures are deduplicated by index; if the same
signer appears in multiple files, the last occurrence wins.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, _ := cmd.Flags().GetString(flagOut)
			if out == "" {
				return fmt.Errorf("--out is required")
			}

			merged, err := LoadPartialProof(args[0])
			if err != nil {
				return err
			}
			for _, p := range args[1:] {
				other, err := LoadPartialProof(p)
				if err != nil {
					return err
				}
				if err := AssertPartialProofsConsistent(merged, other); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
				// Upsert each of `other`'s partial sigs into `merged`.
				for _, ps := range other.PartialSigs {
					filtered := merged.PartialSigs[:0]
					for _, m := range merged.PartialSigs {
						if m.Index != ps.Index {
							filtered = append(filtered, m)
						}
					}
					merged.PartialSigs = append(filtered, ps)
				}
			}

			// Build the LegacyProof from accumulated partials.
			var legacyProof types.LegacyProof
			switch {
			case merged.Single != nil:
				sp, err := assembleSingleProof(merged.Single, merged.PartialSigs)
				if err != nil {
					return err
				}
				legacyProof = types.LegacyProof{Proof: &types.LegacyProof_Single{Single: sp}}
			case merged.Multisig != nil:
				mp, err := assembleMultisigProof(merged.Multisig, merged.PartialSigs)
				if err != nil {
					return err
				}
				legacyProof = types.LegacyProof{Proof: &types.LegacyProof_Multisig{Multisig: mp}}
			}

			if err := legacyProof.ValidateBasic(); err != nil {
				return fmt.Errorf("assembled proof fails ValidateBasic: %w", err)
			}

			// Write unsigned tx JSON (new_signature empty).
			var unsignedMsg sdk.Msg
			switch merged.Kind {
			case "claim":
				unsignedMsg = &types.MsgClaimLegacyAccount{
					NewAddress:    merged.NewAddress,
					LegacyAddress: merged.LegacyAddress,
					LegacyProof:   legacyProof,
				}
			case "validator":
				unsignedMsg = &types.MsgMigrateValidator{
					NewAddress:    merged.NewAddress,
					LegacyAddress: merged.LegacyAddress,
					LegacyProof:   legacyProof,
				}
			default:
				return fmt.Errorf("unknown kind %q", merged.Kind)
			}

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			txb := clientCtx.TxConfig.NewTxBuilder()
			if err := txb.SetMsgs(unsignedMsg); err != nil {
				return err
			}
			bytes, err := clientCtx.TxConfig.TxJSONEncoder()(txb.GetTx())
			if err != nil {
				return err
			}
			return os.WriteFile(out, bytes, 0o600)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagOut, "", "Output unsigned tx JSON path (required)")
	return cmd
}

// AssertPartialProofsConsistent verifies two PartialProof files agree on
// every field that would change the assembled tx identity. Exported so
// tx_multisig_test.go can exercise it directly.
func AssertPartialProofsConsistent(a, b *PartialProof) error {
	if a.Kind != b.Kind {
		return fmt.Errorf("kind mismatch: %q vs %q", a.Kind, b.Kind)
	}
	if a.LegacyAddress != b.LegacyAddress {
		return fmt.Errorf("legacy_address mismatch: %s vs %s", a.LegacyAddress, b.LegacyAddress)
	}
	if a.NewAddress != b.NewAddress {
		return fmt.Errorf("new_address mismatch: %s vs %s", a.NewAddress, b.NewAddress)
	}
	if a.ChainID != b.ChainID {
		return fmt.Errorf("chain_id mismatch: %s vs %s", a.ChainID, b.ChainID)
	}
	if a.EVMChainID != b.EVMChainID {
		return fmt.Errorf("evm_chain_id mismatch: %d vs %d", a.EVMChainID, b.EVMChainID)
	}
	if (a.Single == nil) != (b.Single == nil) {
		return fmt.Errorf("proof-kind mismatch: one has 'single', the other does not")
	}
	if (a.Multisig == nil) != (b.Multisig == nil) {
		return fmt.Errorf("proof-kind mismatch: one has 'multisig', the other does not")
	}
	if a.Single != nil {
		if a.Single.PubKeyB64 != b.Single.PubKeyB64 {
			return fmt.Errorf("single.pub_key_b64 mismatch")
		}
		if a.Single.SigFormat != b.Single.SigFormat {
			return fmt.Errorf("sig_format mismatch: %s vs %s", a.Single.SigFormat, b.Single.SigFormat)
		}
	}
	if a.Multisig != nil {
		if a.Multisig.Threshold != b.Multisig.Threshold {
			return fmt.Errorf("threshold mismatch: %d vs %d", a.Multisig.Threshold, b.Multisig.Threshold)
		}
		if a.Multisig.SigFormat != b.Multisig.SigFormat {
			return fmt.Errorf("sig_format mismatch: %s vs %s", a.Multisig.SigFormat, b.Multisig.SigFormat)
		}
		if len(a.Multisig.SubPubKeysB64) != len(b.Multisig.SubPubKeysB64) {
			return fmt.Errorf("num sub_pub_keys mismatch: %d vs %d", len(a.Multisig.SubPubKeysB64), len(b.Multisig.SubPubKeysB64))
		}
		for i := range a.Multisig.SubPubKeysB64 {
			if a.Multisig.SubPubKeysB64[i] != b.Multisig.SubPubKeysB64[i] {
				return fmt.Errorf("sub_pub_keys_b64[%d] mismatch", i)
			}
		}
	}
	return nil
}
```

- [ ] **Step 2: Write unit tests for AssertPartialProofsConsistent**

Append to `x/evmigration/client/cli/tx_multisig_test.go`:

```go
func TestAssertPartialProofsConsistent_Matching(t *testing.T) {
	a := &cli.PartialProof{
		Version: 1, Kind: "claim", LegacyAddress: "A", NewAddress: "B", ChainID: "c", EVMChainID: 1,
		Multisig: &cli.PartialMultisig{Threshold: 2, SubPubKeysB64: []string{"x", "y", "z"}, SigFormat: "SIG_FORMAT_CLI"},
	}
	b := *a
	require.NoError(t, cli.AssertPartialProofsConsistent(a, &b))
}

func TestAssertPartialProofsConsistent_ChainIDMismatch(t *testing.T) {
	a := &cli.PartialProof{ChainID: "c1", Multisig: &cli.PartialMultisig{}}
	b := &cli.PartialProof{ChainID: "c2", Multisig: &cli.PartialMultisig{}}
	err := cli.AssertPartialProofsConsistent(a, b)
	require.ErrorContains(t, err, "chain_id mismatch")
}

func TestAssertPartialProofsConsistent_ProofKindMismatch(t *testing.T) {
	a := &cli.PartialProof{Single: &cli.PartialSingle{}}
	b := &cli.PartialProof{Multisig: &cli.PartialMultisig{}}
	err := cli.AssertPartialProofsConsistent(a, b)
	require.ErrorContains(t, err, "proof-kind mismatch")
}

func TestCombineProof_MergeDedupByIndex(t *testing.T) {
	pp := &cli.PartialProof{
		PartialSigs: []cli.PartialSubSignature{{Index: 0, SignatureB64: "old"}},
	}
	other := &cli.PartialProof{
		PartialSigs: []cli.PartialSubSignature{{Index: 0, SignatureB64: "new"}},
	}
	// Emulate the merge loop.
	for _, ps := range other.PartialSigs {
		filtered := pp.PartialSigs[:0]
		for _, m := range pp.PartialSigs {
			if m.Index != ps.Index {
				filtered = append(filtered, m)
			}
		}
		pp.PartialSigs = append(filtered, ps)
	}
	require.Len(t, pp.PartialSigs, 1)
	require.Equal(t, "new", pp.PartialSigs[0].SignatureB64)
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./x/evmigration/client/cli/ -run 'TestAssertPartialProofsConsistent|TestCombineProof_Merge' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add x/evmigration/client/cli/tx_multisig.go x/evmigration/client/cli/tx_multisig_test.go
git commit -m "evmigration: add 'combine-proof' CLI command

Accepts one or more PartialProof files, validates cross-file consistency
on address/chain/sig-format fields, merges partial signatures with
index-based deduplication, and writes a fully-assembled unsigned tx JSON."
```

---

### Task 21: `submit-proof` command

**Files:**
- Modify: `x/evmigration/client/cli/tx_multisig.go`

- [ ] **Step 1: Implement cmdSubmitProof**

Append to `tx_multisig.go`:

```go
func cmdSubmitProof() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-proof <tx.json>",
		Short: "Sign new_signature with --from eth key, simulate gas, broadcast",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			tx, err := clientCtx.TxConfig.TxJSONDecoder()(b)
			if err != nil {
				return err
			}
			msgs := tx.GetMsgs()
			if len(msgs) != 1 {
				return fmt.Errorf("expected exactly 1 msg, got %d", len(msgs))
			}
			mpm, ok := msgs[0].(migrationProofMsg)
			if !ok {
				return fmt.Errorf("unexpected msg type %T", msgs[0])
			}

			// Figure out the kind from the msg type.
			var kind string
			switch msgs[0].(type) {
			case *types.MsgClaimLegacyAccount:
				kind = migrationProofKindClaim
			case *types.MsgMigrateValidator:
				kind = migrationProofKindValidator
			default:
				return fmt.Errorf("unexpected msg type %T", msgs[0])
			}

			return runMigrationTx(cmd, mpm, kind, clientCtx.FromName)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagTxTimeout, defaultTxTimeout, "How long to wait for the transaction to be included in a block")
	return cmd
}
```

- [ ] **Step 2: Build binary and verify the new subcommands appear**

Run: `make build`
Run: `./build/lumerad tx evmigration --help`
Expected output contains `generate-proof-payload`, `sign-proof`, `combine-proof`, `submit-proof`.

- [ ] **Step 3: Commit**

```bash
git add x/evmigration/client/cli/tx_multisig.go
git commit -m "evmigration: add 'submit-proof' CLI command

Completes the four-step offline flow: reads an unsigned tx JSON,
signs new_signature with --from (eth key), simulates gas, broadcasts."
```

---

## Phase 7 — Integration Tests

### Task 22: Integration test helpers

**Files:**
- Create: `tests/integration/evmigration/multisig_helpers.go`

- [ ] **Step 1: Add helpers**

Create `tests/integration/evmigration/multisig_helpers.go`:

```go
//go:build test

package evmigration_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// BuildMultisigLegacyAccount creates an in-memory K-of-N multisig of
// secp256k1 sub-keys and returns the multisig pubkey, sub-private-keys,
// and the derived bech32 address.
func BuildMultisigLegacyAccount(t *testing.T, k, n int) (*multisig.LegacyAminoPubKey, []*secp256k1.PrivKey, sdk.AccAddress) {
	t.Helper()
	privs := make([]*secp256k1.PrivKey, n)
	pubs := make([]cryptotypes.PubKey, n)
	for i := 0; i < n; i++ {
		privs[i] = secp256k1.GenPrivKey()
		pubs[i] = privs[i].PubKey()
	}
	multiPK := multisig.NewLegacyAminoPubKey(k, pubs)
	return multiPK, privs, sdk.AccAddress(multiPK.Address())
}

// SignMultisigProof builds a MultisigProof signed by the K sub-keys at
// signerIdxs. format selects CLI (SHA256) or ADR-036 (canonical JSON).
func SignMultisigProof(
	t *testing.T,
	chainID string,
	kind string,
	multiPK *multisig.LegacyAminoPubKey,
	privs []*secp256k1.PrivKey,
	signerIdxs []int,
	legacyAddr, newAddr sdk.AccAddress,
	format types.SigFormat,
) *types.LegacyProof {
	t.Helper()
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		chainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())

	sort.Ints(signerIdxs)
	indices := make([]uint32, len(signerIdxs))
	sigs := make([][]byte, len(signerIdxs))
	for i, idx := range signerIdxs {
		indices[i] = uint32(idx)
		if format == types.SigFormat_SIG_FORMAT_ADR036 {
			signerAddr := sdk.AccAddress(privs[idx].PubKey().Address()).String()
			doc := []byte(fmt.Sprintf(`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},"memo":"","msgs":[{"type":"sign/MsgSignData","value":{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
				base64.StdEncoding.EncodeToString([]byte(payload)), signerAddr))
			sig, err := privs[idx].Sign(doc)
			require.NoError(t, err)
			sigs[i] = sig
			continue
		}
		hash := sha256.Sum256([]byte(payload))
		sig, err := privs[idx].Sign(hash[:])
		require.NoError(t, err)
		sigs[i] = sig
	}

	subPubKeys := make([][]byte, len(multiPK.GetPubKeys()))
	for i, p := range multiPK.GetPubKeys() {
		subPubKeys[i] = p.Bytes()
	}
	return &types.LegacyProof{Proof: &types.LegacyProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     uint32(multiPK.Threshold),
		SubPubKeys:    subPubKeys,
		SignerIndices: indices,
		SubSignatures: sigs,
		SigFormat:     format,
	}}}
}
```

- [ ] **Step 2: Commit**

```bash
git add tests/integration/evmigration/multisig_helpers.go
git commit -m "evmigration: add integration test helpers for multisig"
```

---

### Task 23: Integration tests — core multisig scenarios

**Files:**
- Modify: `tests/integration/evmigration/migration_test.go`

- [ ] **Step 1: Add integration tests for balance, validator, threshold, replay**

Append to `tests/integration/evmigration/migration_test.go` (within the `_test` package and `//go:build test` tag):

```go
func TestIntegration_MsgClaimLegacyAccount_Multisig(t *testing.T) {
	s := setupChain(t) // existing test harness

	multiPK, privs, legacyAddr := BuildMultisigLegacyAccount(t, 2, 3)
	s.setPubKeyOnChain(legacyAddr, multiPK)
	s.fund(legacyAddr, 1000)

	newPriv, newAddr := s.genEthKey()

	proof := SignMultisigProof(t, s.chainID, "claim", multiPK, privs, []int{0, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	newSig := s.signNewProof(newPriv, "claim", legacyAddr, newAddr)

	msg := &types.MsgClaimLegacyAccount{
		NewAddress:    newAddr.String(),
		LegacyAddress: legacyAddr.String(),
		LegacyProof:   *proof,
		NewSignature:  newSig,
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	require.NoError(t, err)

	require.Equal(t, int64(0), s.balance(legacyAddr, "ulume").Int64())
	require.Equal(t, int64(1000), s.balance(newAddr, "ulume").Int64())
	require.True(t, s.hasMigrationRecord(legacyAddr.String()))
}

func TestIntegration_MsgClaimLegacyAccount_Multisig_WrongThreshold(t *testing.T) {
	s := setupChain(t)
	multiPK, privs, legacyAddr := BuildMultisigLegacyAccount(t, 2, 3)
	s.setPubKeyOnChain(legacyAddr, multiPK)
	s.fund(legacyAddr, 1000)
	newPriv, newAddr := s.genEthKey()

	// Only sign with 1 sub-key (K-1).
	proof := SignMultisigProof(t, s.chainID, "claim", multiPK, privs, []int{0}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	// Manually lower Threshold in the proof to try and bypass the check.
	// The proof's stated threshold will not match the on-chain multisig.
	proof.GetMultisig().Threshold = 1
	newSig := s.signNewProof(newPriv, "claim", legacyAddr, newAddr)

	msg := &types.MsgClaimLegacyAccount{
		NewAddress: newAddr.String(), LegacyAddress: legacyAddr.String(),
		LegacyProof: *proof, NewSignature: newSig,
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	require.ErrorContains(t, err, "multisig pubkey derives to")
}

func TestIntegration_MsgClaimLegacyAccount_Multisig_Replay(t *testing.T) {
	s := setupChain(t)
	multiPK, privs, legacyAddr := BuildMultisigLegacyAccount(t, 2, 3)
	s.setPubKeyOnChain(legacyAddr, multiPK)
	s.fund(legacyAddr, 1000)
	newPriv, newAddr := s.genEthKey()
	proof := SignMultisigProof(t, s.chainID, "claim", multiPK, privs, []int{0, 1}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	newSig := s.signNewProof(newPriv, "claim", legacyAddr, newAddr)

	msg := &types.MsgClaimLegacyAccount{
		NewAddress: newAddr.String(), LegacyAddress: legacyAddr.String(),
		LegacyProof: *proof, NewSignature: newSig,
	}
	_, err := s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	require.NoError(t, err)

	// Replay.
	_, err = s.msgServer.ClaimLegacyAccount(s.ctx, msg)
	require.ErrorContains(t, err, "already migrated")
}

func TestIntegration_MsgMigrateValidator_Multisig(t *testing.T) {
	s := setupChain(t)
	multiPK, privs, legacyAddr := BuildMultisigLegacyAccount(t, 2, 3)
	s.setPubKeyOnChain(legacyAddr, multiPK)
	s.fund(legacyAddr, 10_000_000)
	s.registerValidator(legacyAddr, 5_000_000) // helper that stakes and registers
	newPriv, newAddr := s.genEthKey()

	proof := SignMultisigProof(t, s.chainID, "validator", multiPK, privs, []int{0, 2}, legacyAddr, newAddr, types.SigFormat_SIG_FORMAT_CLI)
	newSig := s.signNewProof(newPriv, "validator", legacyAddr, newAddr)
	_, err := s.msgServer.MigrateValidator(s.ctx, &types.MsgMigrateValidator{
		NewAddress: newAddr.String(), LegacyAddress: legacyAddr.String(),
		LegacyProof: *proof, NewSignature: newSig,
	})
	require.NoError(t, err)

	// Validator record must now live at the new address.
	_, err = s.stakingKeeper.GetValidator(s.ctx, sdk.ValAddress(newAddr))
	require.NoError(t, err)
	_, err = s.stakingKeeper.GetValidator(s.ctx, sdk.ValAddress(legacyAddr))
	require.Error(t, err, "legacy validator record must be gone")
}
```

Note: `setupChain`, `s.fund`, `s.genEthKey`, `s.signNewProof`, `s.registerValidator`, `s.hasMigrationRecord`, `s.balance`, `s.setPubKeyOnChain`, `s.msgServer`, `s.stakingKeeper`, `s.ctx` are existing test-harness hooks in `migration_test.go`. If any is missing, add a thin wrapper following the style of the existing single-key tests.

- [ ] **Step 2: Run integration tests**

Run: `go test -tags='test' ./tests/integration/evmigration/... -run TestIntegration_MsgClaimLegacyAccount_Multisig -v -timeout 10m`
Expected: 4 tests PASS.

Run: `go test -tags='test' ./tests/integration/evmigration/... -run TestIntegration_MsgMigrateValidator_Multisig -v -timeout 10m`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/evmigration/migration_test.go
git commit -m "evmigration: integration tests for multisig migration

Covers: 2-of-3 balance migration, K-1 threshold rejection, replay
rejection, and multisig validator operator migration."
```

---

## Phase 8 — Devnet Tests

### Task 24: Devnet multisig fixture

**Files:**
- Create: `devnet/tests/evmigration/multisig_keys.go`

- [ ] **Step 1: Create the multisig fixture**

Create `devnet/tests/evmigration/multisig_keys.go`:

```go
package evmigration

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// MultisigFixture describes a 2-of-3 secp256k1 multisig seeded into the devnet
// for migration testing. All three sub-keys are added to the shared test
// keyring; the multisig is registered in the keyring too so that addresses
// derive identically on both host and inside containers.
type MultisigFixture struct {
	Name         string   // keyring name of the multisig account
	Address      string   // bech32 address
	MemberNames  []string // keyring names of the three member keys
	Threshold    int
	BalanceULume int64
}

// SeedMultisigFixture creates the three member keys, the multisig, funds it,
// and issues one self-send so that the multisig's pubkey is registered on-chain
// (non-nil — required precondition for migration).
func SeedMultisigFixture(t *testing.T, ctx context.Context) *MultisigFixture {
	t.Helper()
	names := []string{"multisig-signer-1", "multisig-signer-2", "multisig-signer-3"}
	for _, n := range names {
		runLumerad(t, ctx, "keys", "add", n, "--keyring-backend", "test", "--coin-type", "118", "--algo", "secp256k1")
	}
	runLumerad(t, ctx, "keys", "add", "multisig-account",
		"--multisig", strings.Join(names, ","),
		"--multisig-threshold", "2",
		"--keyring-backend", "test")
	addr := queryKeyAddr(t, ctx, "multisig-account")

	// Fund 1,000,000 ulume.
	runLumerad(t, ctx, "tx", "bank", "send", "alice", addr, "1000000ulume",
		"--keyring-backend", "test", "--chain-id", "lumera-devnet", "-y")
	waitForNextBlock(t, ctx)

	// Issue a trivial self-send from the multisig to record its pubkey on-chain.
	// This uses the SDK's built-in multisign flow (unrelated to evmigration).
	generateMultisigSelfSend(t, ctx, addr, names)

	return &MultisigFixture{
		Name:         "multisig-account",
		Address:      addr,
		MemberNames:  names,
		Threshold:    2,
		BalanceULume: 1000000 - 1, // 1 ulume spent on self-send, minus gas ignored here
	}
}

// Helpers that assume the devnet makefile conventions and existing shell scripts.
func runLumerad(t *testing.T, ctx context.Context, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "lumerad", args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "lumerad %s: %s", strings.Join(args, " "), string(out))
	return string(out)
}

func queryKeyAddr(t *testing.T, ctx context.Context, name string) string {
	t.Helper()
	out := runLumerad(t, ctx, "keys", "show", name, "-a", "--keyring-backend", "test")
	return strings.TrimSpace(out)
}

func waitForNextBlock(t *testing.T, ctx context.Context) {
	t.Helper()
	// Implementation hook — devnet scripts typically expose a block-wait helper.
	runLumerad(t, ctx, "q", "block")
}

func generateMultisigSelfSend(t *testing.T, ctx context.Context, addr string, memberNames []string) {
	t.Helper()
	// 1. Build unsigned tx
	unsigned := fmt.Sprintf("/tmp/multisig-selfsend-unsigned-%s.json", addr)
	runLumerad(t, ctx, "tx", "bank", "send", addr, addr, "1ulume",
		"--generate-only", "--chain-id", "lumera-devnet",
		"--from", addr, "--output-document", unsigned)
	// 2. Each member signs
	part := make([]string, len(memberNames))
	for i, m := range memberNames {
		part[i] = fmt.Sprintf("/tmp/multisig-selfsend-%s-%s.sig", m, addr)
		runLumerad(t, ctx, "tx", "sign", unsigned, "--from", m, "--multisig", addr,
			"--chain-id", "lumera-devnet", "--output-document", part[i], "--keyring-backend", "test")
	}
	// 3. Combine
	combined := fmt.Sprintf("/tmp/multisig-selfsend-combined-%s.json", addr)
	args := []string{"tx", "multisign", unsigned, "multisig-account"}
	args = append(args, part...)
	args = append(args, "--chain-id", "lumera-devnet", "--keyring-backend", "test", "--output-document", combined)
	runLumerad(t, ctx, args...)
	// 4. Broadcast
	runLumerad(t, ctx, "tx", "broadcast", combined)
	waitForNextBlock(t, ctx)
}
```

- [ ] **Step 2: Commit**

```bash
git add devnet/tests/evmigration/multisig_keys.go
git commit -m "evmigration devnet: add multisig fixture

Seeds a 2-of-3 secp256k1 multisig, funds it, and issues a self-send to
register the pubkey on-chain (the documented precondition for migration)."
```

---

### Task 25: Devnet test — multisig claim (both flow variants)

**Files:**
- Create: `devnet/tests/evmigration/multisig_test.go`

- [ ] **Step 1: Create the test**

Create `devnet/tests/evmigration/multisig_test.go`:

```go
package evmigration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDevnet_MultisigClaim_SeparateMachine exercises the four-step CLI flow
// as if each signer were on a different machine: one partial JSON per signer,
// merged via combine-proof.
func TestDevnet_MultisigClaim_SeparateMachine(t *testing.T) {
	if os.Getenv("LUMERA_DEVNET") != "1" {
		t.Skip("devnet not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	mf := SeedMultisigFixture(t, ctx)
	// Generate a new eth key for the destination.
	runLumerad(t, ctx, "keys", "add", "multisig-new", "--algo", "eth_secp256k1", "--coin-type", "60", "--keyring-backend", "test")
	newAddr := queryKeyAddr(t, ctx, "multisig-new")

	dir := t.TempDir()
	tmpl := filepath.Join(dir, "proof.json")
	runLumerad(t, ctx, "tx", "evmigration", "generate-proof-payload",
		"--legacy", mf.Address, "--new", newAddr, "--kind", "claim",
		"--chain-id", "lumera-devnet", "--out", tmpl)

	// Each signer produces their own partial file.
	sig1 := filepath.Join(dir, "signer1.json")
	sig2 := filepath.Join(dir, "signer2.json")
	runCopy(t, tmpl, sig1)
	runCopy(t, tmpl, sig2)
	runLumerad(t, ctx, "tx", "evmigration", "sign-proof", sig1, "--from", mf.MemberNames[0], "--keyring-backend", "test")
	runLumerad(t, ctx, "tx", "evmigration", "sign-proof", sig2, "--from", mf.MemberNames[2], "--keyring-backend", "test")

	txJSON := filepath.Join(dir, "tx.json")
	runLumerad(t, ctx, "tx", "evmigration", "combine-proof", sig1, sig2, "--out", txJSON, "--chain-id", "lumera-devnet")

	out := runLumerad(t, ctx, "tx", "evmigration", "submit-proof", txJSON,
		"--from", "multisig-new", "--chain-id", "lumera-devnet", "--keyring-backend", "test", "-y")
	require.Contains(t, out, "txhash:")

	// Verify migration record exists.
	waitForNextBlock(t, ctx)
	rec := runLumerad(t, ctx, "q", "evmigration", "migration-record", mf.Address)
	require.Contains(t, rec, newAddr)

	// Replay must fail.
	_, err := exec.CommandContext(ctx, "lumerad", "tx", "evmigration", "submit-proof", txJSON,
		"--from", "multisig-new", "--chain-id", "lumera-devnet", "--keyring-backend", "test", "-y").CombinedOutput()
	require.Error(t, err)
}

// TestDevnet_MultisigClaim_SharedFile exercises the same flow with a single
// file mutated in place across sign-proof invocations.
func TestDevnet_MultisigClaim_SharedFile(t *testing.T) {
	if os.Getenv("LUMERA_DEVNET") != "1" {
		t.Skip("devnet not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	mf := SeedMultisigFixture(t, ctx)
	runLumerad(t, ctx, "keys", "add", "multisig-new-shared", "--algo", "eth_secp256k1", "--coin-type", "60", "--keyring-backend", "test")
	newAddr := queryKeyAddr(t, ctx, "multisig-new-shared")

	dir := t.TempDir()
	proof := filepath.Join(dir, "proof.json")
	runLumerad(t, ctx, "tx", "evmigration", "generate-proof-payload",
		"--legacy", mf.Address, "--new", newAddr, "--kind", "claim",
		"--chain-id", "lumera-devnet", "--out", proof)
	runLumerad(t, ctx, "tx", "evmigration", "sign-proof", proof, "--from", mf.MemberNames[0], "--keyring-backend", "test")
	runLumerad(t, ctx, "tx", "evmigration", "sign-proof", proof, "--from", mf.MemberNames[1], "--keyring-backend", "test")

	txJSON := filepath.Join(dir, "tx.json")
	runLumerad(t, ctx, "tx", "evmigration", "combine-proof", proof, "--out", txJSON, "--chain-id", "lumera-devnet")
	out := runLumerad(t, ctx, "tx", "evmigration", "submit-proof", txJSON,
		"--from", "multisig-new-shared", "--chain-id", "lumera-devnet", "--keyring-backend", "test", "-y")
	require.Contains(t, out, "txhash:")
}

func runCopy(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, b, 0o600))
}
```

Add `"os/exec"` to imports.

- [ ] **Step 2: Run devnet test**

Run: `make devnet-new` (takes several minutes to bootstrap)
Run: `LUMERA_DEVNET=1 go test ./devnet/tests/evmigration/ -run TestDevnet_MultisigClaim -v -timeout 15m`
Expected: Both tests PASS.

- [ ] **Step 3: Commit**

```bash
git add devnet/tests/evmigration/multisig_test.go
git commit -m "evmigration devnet: multisig claim tests (separate + shared file)"
```

---

### Task 26: Devnet test — multisig validator operator

**Files:**
- Create: `devnet/tests/evmigration/multisig_validator_test.go`

- [ ] **Step 1: Add the validator test**

Create `devnet/tests/evmigration/multisig_validator_test.go`:

```go
package evmigration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDevnet_MultisigValidator verifies that a validator operator whose
// operator key is a multisig can migrate.
func TestDevnet_MultisigValidator(t *testing.T) {
	if os.Getenv("LUMERA_DEVNET") != "1" {
		t.Skip("devnet not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	mf := SeedMultisigFixture(t, ctx)
	// Promote the multisig address to a validator. The specific devnet helper
	// depends on local scripts; the simplest is: the devnet fund helper registers
	// validators at init time. For this test assume a SeedMultisigValidator
	// helper exists in the same file pattern.
	SeedMultisigValidator(t, ctx, mf)

	runLumerad(t, ctx, "keys", "add", "multisig-val-new", "--algo", "eth_secp256k1", "--coin-type", "60", "--keyring-backend", "test")
	newAddr := queryKeyAddr(t, ctx, "multisig-val-new")

	dir := t.TempDir()
	proof := filepath.Join(dir, "proof.json")
	runLumerad(t, ctx, "tx", "evmigration", "generate-proof-payload",
		"--legacy", mf.Address, "--new", newAddr, "--kind", "validator",
		"--chain-id", "lumera-devnet", "--out", proof)
	runLumerad(t, ctx, "tx", "evmigration", "sign-proof", proof, "--from", mf.MemberNames[0], "--keyring-backend", "test")
	runLumerad(t, ctx, "tx", "evmigration", "sign-proof", proof, "--from", mf.MemberNames[2], "--keyring-backend", "test")

	txJSON := filepath.Join(dir, "tx.json")
	runLumerad(t, ctx, "tx", "evmigration", "combine-proof", proof, "--out", txJSON, "--chain-id", "lumera-devnet")
	out := runLumerad(t, ctx, "tx", "evmigration", "submit-proof", txJSON,
		"--from", "multisig-val-new", "--chain-id", "lumera-devnet", "--keyring-backend", "test", "-y")
	require.Contains(t, out, "txhash:")

	// Verify validator record now at newAddr.
	valq := runLumerad(t, ctx, "q", "staking", "validator", newAddr)
	require.Contains(t, valq, "operator_address")

	// Old validator record should be gone (query returns not-found).
	_, err := exec.CommandContext(ctx, "lumerad", "q", "staking", "validator", mf.Address).CombinedOutput()
	require.Error(t, err, "legacy validator record should be deleted")
}

// SeedMultisigValidator stakes the multisig and registers it as a validator.
// Implementation relies on existing devnet helpers.
func SeedMultisigValidator(t *testing.T, ctx context.Context, mf *MultisigFixture) {
	// Implementation: build create-validator tx, sign via multisig flow,
	// broadcast. Pattern follows generateMultisigSelfSend.
}
```

- [ ] **Step 2: Commit**

```bash
git add devnet/tests/evmigration/multisig_validator_test.go
git commit -m "evmigration devnet: multisig validator operator migration test"
```

---

### Task 27: Devnet test — MigrationEstimate preflight

**Files:**
- Create: `devnet/tests/evmigration/multisig_estimate_test.go`

- [ ] **Step 1: Add preflight query test**

Create `devnet/tests/evmigration/multisig_estimate_test.go`:

```go
package evmigration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDevnet_MigrationEstimate_Multisig_Supported(t *testing.T) {
	if os.Getenv("LUMERA_DEVNET") != "1" {
		t.Skip()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mf := SeedMultisigFixture(t, ctx)
	out := runLumerad(t, ctx, "q", "evmigration", "migration-estimate", mf.Address, "--output", "json")

	var est struct {
		IsMultisig   bool   `json:"is_multisig"`
		Threshold    uint32 `json:"threshold"`
		NumSigners   uint32 `json:"num_signers"`
		WouldSucceed bool   `json:"would_succeed"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &est))
	require.True(t, est.IsMultisig)
	require.Equal(t, uint32(2), est.Threshold)
	require.Equal(t, uint32(3), est.NumSigners)
	require.True(t, est.WouldSucceed)
}

func TestDevnet_MigrationEstimate_Multisig_OverCap(t *testing.T) {
	if os.Getenv("LUMERA_DEVNET") != "1" {
		t.Skip()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Build a 21-signer multisig — exceeds default MaxMultisigSubKeys=20.
	addr := seedLargeMultisig(t, ctx, 21)

	out := runLumerad(t, ctx, "q", "evmigration", "migration-estimate", addr, "--output", "json")
	var est struct {
		WouldSucceed    bool   `json:"would_succeed"`
		RejectionReason string `json:"rejection_reason"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &est))
	require.False(t, est.WouldSucceed)
	require.Contains(t, est.RejectionReason, "max is 20")
}

// seedLargeMultisig creates an N-signer 1-of-N multisig (threshold=1 keeps the
// signing flow short — the test only needs the multisig to exist with N sub-keys,
// not for every sub-signer to actually sign). Funds it and registers its pubkey
// on-chain via a 1-ulume self-send.
func seedLargeMultisig(t *testing.T, ctx context.Context, n int) string {
	t.Helper()
	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = fmt.Sprintf("large-multisig-signer-%d", i)
		runLumerad(t, ctx, "keys", "add", names[i],
			"--keyring-backend", "test", "--coin-type", "118", "--algo", "secp256k1")
	}
	accName := fmt.Sprintf("large-multisig-n%d", n)
	runLumerad(t, ctx, "keys", "add", accName,
		"--multisig", strings.Join(names, ","),
		"--multisig-threshold", "1",
		"--keyring-backend", "test")
	addr := queryKeyAddr(t, ctx, accName)

	// Fund so the account exists in bank state.
	runLumerad(t, ctx, "tx", "bank", "send", "alice", addr, "1000ulume",
		"--keyring-backend", "test", "--chain-id", "lumera-devnet", "-y")
	waitForNextBlock(t, ctx)

	// Register pubkey on-chain via self-send.
	// Only the first member needs to sign (threshold=1).
	unsigned := fmt.Sprintf("/tmp/large-multisig-selfsend-%s.json", addr)
	runLumerad(t, ctx, "tx", "bank", "send", addr, addr, "1ulume",
		"--generate-only", "--chain-id", "lumera-devnet",
		"--from", addr, "--output-document", unsigned)
	partial := fmt.Sprintf("/tmp/large-multisig-partial-%s.sig", addr)
	runLumerad(t, ctx, "tx", "sign", unsigned, "--from", names[0], "--multisig", addr,
		"--chain-id", "lumera-devnet", "--output-document", partial, "--keyring-backend", "test")
	combined := fmt.Sprintf("/tmp/large-multisig-combined-%s.json", addr)
	runLumerad(t, ctx, "tx", "multisign", unsigned, accName, partial,
		"--chain-id", "lumera-devnet", "--keyring-backend", "test", "--output-document", combined)
	runLumerad(t, ctx, "tx", "broadcast", combined)
	waitForNextBlock(t, ctx)

	return addr
}
```

Implement `seedLargeMultisig` following the pattern from `SeedMultisigFixture`.

- [ ] **Step 2: Commit**

```bash
git add devnet/tests/evmigration/multisig_estimate_test.go
git commit -m "evmigration devnet: MigrationEstimate multisig preflight tests"
```

---

## Phase 9 — Documentation & Final Verification

### Task 28: Update EVM integration docs

**Files:**
- Modify: `docs/evm-integration/tests.md`
- Modify: `docs/evm-integration/evmigration/portal-ui.md`
- Modify or create: `docs/evm-integration/evmigration.md`
- Modify: `docs/evm-integration/unit-evmigration.md`
- Modify: `docs/evm-integration/integration-evmigration.md`

- [ ] **Step 1: Update tests.md with new test rows**

In `docs/evm-integration/tests.md`, under the evmigration section, add rows for:

| Test | Path | Description |
|------|------|-------------|
| `TestVerifyLegacyProof_Multisig_*` | `x/evmigration/keeper/verify_test.go` | Multisig verifier cases: valid CLI/ADR-036, 1-of-1, wrong address, invalid sub-sig, N=20 boundary, param cap |
| `TestMigrationEstimate_Multisig_*` | `x/evmigration/keeper/query_test.go` | Preflight: supported, N > cap, non-secp256k1 sub-key |
| `TestLegacyAccounts_Multisig` | `x/evmigration/keeper/query_test.go` | LegacyAccounts populates is_multisig/threshold/num_signers |
| `TestIntegration_MsgClaimLegacyAccount_Multisig*` | `tests/integration/evmigration/migration_test.go` | 2-of-3 E2E migration, wrong-threshold, replay |
| `TestIntegration_MsgMigrateValidator_Multisig` | `tests/integration/evmigration/migration_test.go` | Validator operator migration |
| `TestDevnet_MultisigClaim_*` | `devnet/tests/evmigration/multisig_test.go` | CLI four-step flow: separate files + shared file |
| `TestDevnet_MigrationEstimate_Multisig_*` | `devnet/tests/evmigration/multisig_estimate_test.go` | Devnet preflight for supported and over-cap |

- [ ] **Step 2: Update portal-ui.md**

In `docs/evm-integration/evmigration/portal-ui.md`, replace the documentation of flat `legacy_pub_key` / `legacy_signature` fields with documentation of the new `legacy_proof` oneof. Include both shapes:

```markdown
## MsgClaimLegacyAccount fields

| Field | Type | Description |
|-------|------|-------------|
| `new_address` | string (bech32) | Destination coin-type 60 account |
| `legacy_address` | string (bech32) | Source coin-type 118 account |
| `legacy_proof` | `LegacyProof` | Proof of legacy key ownership (see below) |
| `new_signature` | bytes | eth_secp256k1 signature from the destination key |

### LegacyProof (oneof)

Exactly one of the following must be set:

#### Single-key (`single`)

```json
{
  "single": {
    "pub_key": "<base64 of 33-byte compressed secp256k1>",
    "signature": "<base64 of signature>",
    "sig_format": "SIG_FORMAT_CLI" // or "SIG_FORMAT_ADR036"
  }
}
```

#### Multisig (`multisig`)

```json
{
  "multisig": {
    "threshold": 2,
    "sub_pub_keys": ["<base64>", "<base64>", "<base64>"],
    "signer_indices": [0, 2],
    "sub_signatures": ["<base64>", "<base64>"],
    "sig_format": "SIG_FORMAT_CLI"
  }
}
```

Multisig proofs must satisfy:
- Every sub-key is a 33-byte compressed secp256k1 pubkey (no nested multisig, no non-secp256k1 sub-keys).
- `signer_indices.length == threshold` (exact-K, not "at least K").
- `signer_indices` is strictly ascending.
- `sub_signatures.length == signer_indices.length`, in the same order.

For the multi-step offline signing flow (coordination across co-signers on different machines), see [evmigration.md](./evmigration.md#multisig-migration).
```

- [ ] **Step 3: Create or update evmigration.md with the multisig flow**

In `docs/evm-integration/evmigration.md` (create if missing), add a "Multisig migration" section:

```markdown
## Multisig migration

Multisig-controlled legacy accounts use a four-step CLI flow that mirrors
the SDK's `tx multisign` pattern:

```
# Coordinator: build the template
lumerad tx evmigration generate-proof-payload \
    --legacy <multisig-bech32> --new <new-eth-bech32> --kind claim \
    --chain-id lumera-devnet --out proof.json

# Distribute copies of proof.json to each co-signer.

# Each co-signer on their own machine:
lumerad tx evmigration sign-proof proof.json \
    --from <my-sub-key> --keyring-backend test \
    --out my-partial.json

# Coordinator collects the partials and merges:
lumerad tx evmigration combine-proof \
    alice-partial.json bob-partial.json \
    --out tx.json --chain-id lumera-devnet

# Coordinator (or any party holding the destination eth key) broadcasts:
lumerad tx evmigration submit-proof tx.json \
    --from <new-eth-key> --chain-id lumera-devnet --keyring-backend test -y
```

### Preconditions

- The multisig's on-chain pubkey must be non-nil. A multisig that has
  received funds but never signed a transaction must first send any signed
  tx (the smallest is `lumerad tx bank send <multisig> <multisig> 1ulume`
  via the standard SDK multisign flow) to register its pubkey.
- All sub-keys must be `secp256k1` (not ed25519, sr25519, or eth_secp256k1).
- Nested multisig (a sub-key that is itself a multisig) is not supported.
- `N ≤ params.MaxMultisigSubKeys` (default 20; governance-adjustable).

### Destination

The destination address is always a single `eth_secp256k1` EOA.
Multisig holders wanting ongoing multisig custody on the EVM side should
deploy a Gnosis Safe (or similar) at a separate address after migration.

### Nil-pubkey single-key fallback

The four-step flow also supports single-sig nil-pubkey accounts via
`generate-proof-payload --legacy-key <name>`, which seeds the `pub_key`
from the local keyring after verifying the derived address equals
`--legacy`.
```

- [ ] **Step 4: Update unit-evmigration.md and integration-evmigration.md coverage summaries**

Add bullet points under each file's "multisig" subsection listing the new test functions (exhaustive list from Tasks 11, 13, 14, 23).

- [ ] **Step 5: Commit**

```bash
git add docs/evm-integration/
git commit -m "docs: document evmigration multisig support

- tests.md: list new multisig unit/integration/devnet tests.
- portal-ui.md: describe legacy_proof oneof (single + multisig shapes).
- evmigration.md: four-step CLI flow walkthrough with preconditions.
- unit/integration coverage summaries updated."
```

---

### Task 29: Final lint + full test run

**Files:**
- No code changes; CI-style verification.

- [ ] **Step 1: Run lint**

Run: `make lint`
Expected: exits 0 with no issues.

- [ ] **Step 2: Run unit tests**

Run: `go test ./x/evmigration/... -v`
Expected: all PASS.

- [ ] **Step 3: Run integration tests**

Run: `go test -tags='test' ./tests/integration/evmigration/... -v -timeout 15m`
Expected: all PASS.

- [ ] **Step 4: Run build**

Run: `make build`
Expected: `build/lumerad` produced.

Run: `./build/lumerad tx evmigration --help`
Expected: lists `claim-legacy-account`, `migrate-validator`, `generate-proof-payload`, `sign-proof`, `combine-proof`, `submit-proof`.

- [ ] **Step 5: (Optional) Run devnet tests**

Run: `make devnet-new`
Run: `LUMERA_DEVNET=1 go test ./devnet/tests/evmigration/ -run TestDevnet_Multisig -v -timeout 20m`
Expected: all three devnet tests PASS (claim separate-machine, claim shared-file, validator).

- [ ] **Step 6: Verify no uncommitted changes**

Run: `git status`
Expected: `nothing to commit, working tree clean`.

---

## Spec Coverage Self-Review

Cross-checking this plan against [the spec](../design/2026-04-18-evmigration-multisig-design.md):

| Spec section | Task(s) |
|---|---|
| 4.1 proof.proto | Task 1 |
| 4.1 tx.proto / params.proto / query.proto changes | Task 2 |
| 4.1 .pb.go regen | Task 3 |
| 4.2 verifySecp256k1Sig | Task 7 |
| 4.2 verifySingleKeyProof | Task 8 |
| 4.2 verifyMultisigProof | Task 9 |
| 4.2 VerifyLegacyProof + msg server updates | Task 10 |
| 4.2 verifier tests (12 cases) | Task 11 |
| 4.3 two-tier ValidateBasic | Task 4 |
| 4.3 Msg.ValidateBasic delegation | Task 5 |
| 4.3 MaxMultisigSubKeys param | Task 6 |
| 4.4 isLegacyPubKey + query.go | Task 12 |
| 4.4 LegacyAccountInfo multisig fields | Task 13 |
| 4.4.1 MigrationEstimate preflight | Task 14 |
| 4.5 CLI one-shot commands build LegacyProof{Single} | Task 15 |
| 4.5 AutoCLI skip | Task 16 |
| 4.5 generate-proof-payload | Tasks 17, 18 |
| 4.5 sign-proof | Task 19 |
| 4.5 combine-proof | Task 20 |
| 4.5 submit-proof | Task 21 |
| 5.2 integration helpers + tests | Tasks 22, 23 |
| 5.4 devnet multisig_keys.go | Task 24 |
| 5.4 multisig_test.go (both flow variants) | Task 25 |
| 5.4 multisig_validator_test.go | Task 26 |
| 5.4 multisig_estimate_test.go | Task 27 |
| 5.5 documentation updates | Task 28 |
| 6 rollout / final verification | Task 29 |

All spec sections have covering tasks.

## Execution Handoff

**Plan complete and saved to [docs/plan/2026-04-18-evmigration-multisig-plan.md](2026-04-18-evmigration-multisig-plan.md). Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Isolates context-window pressure from the large test-code blocks.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints for review.

**Which approach?**
