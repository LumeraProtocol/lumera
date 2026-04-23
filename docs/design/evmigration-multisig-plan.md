# evmigration Multisig Destination Pivot — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `x/evmigration` so that a multisig legacy source migrates to a multisig destination with `eth_secp256k1` sub-keys (same K-of-N threshold), replacing the current "multisig → single EOA" implementation on branch `evm`.

**Architecture:** Unified `MigrationProof` proto shape used on both `legacy_proof` and `new_proof` fields. Verifier parameterizes over `SubKeyType` (Cosmos secp256k1 vs eth secp256k1). `MigrateAuth` persists multisig pubkey on the destination `BaseAccount`. CLI four-step flow (`generate-proof-payload` / `sign-proof` / `combine-proof` / `submit-proof`) has each co-signer signing both legacy and new halves in one `sign-proof` invocation.

**Tech Stack:** Cosmos SDK v0.53.6, Cosmos EVM v0.6.0, protobuf via `buf`, Go 1.26.2 (per `go.mod`), `lumerad` CLI, golangci-lint.

**Reference:** Full design at [docs/design/evmigration-multisig-design.md](docs/design/evmigration-multisig-design.md). Devnet spike validated SDK primitives on 2026-04-22 (see design §1).

**Working branch:** `evm` (existing commits stay; this plan adds refactor commits on top).

**Ground rules:**
- Run `make lint` before every commit; must be 0 issues.
- Unit test command: `go test ./x/evmigration/... -v -count=1`.
- Integration test command: `go test -tags='integration test' ./tests/integration/evmigration/... -v -timeout 10m`.
- Never skip git hooks. Never force-push. Never amend a shared commit.
- After each phase, run the **full unit suite** for `x/evmigration` before starting the next phase.

---

## Phase 1 — Proto & Types Refactor

### Task 1: Rename `LegacyProof` → `MigrationProof` in proof.proto; add `SIG_FORMAT_EIP191`

**Files:**
- Modify: `proto/lumera/evmigration/proof.proto`

- [ ] **Step 1: Open the proto file**

Run: `cat proto/lumera/evmigration/proof.proto`

- [ ] **Step 2: Rewrite the content**

Replace the file contents with:

```proto
syntax = "proto3";
package lumera.evmigration;
option go_package = "x/evmigration/types"; // matches existing evmigration protos; do NOT use the full github.com path — buf generates files at module root from this relative form

enum SigFormat {
  SIG_FORMAT_UNSPECIFIED = 0;
  SIG_FORMAT_CLI         = 1;
  SIG_FORMAT_ADR036      = 2;
  SIG_FORMAT_EIP191      = 3;
}

message MigrationProof {
  oneof proof {
    SingleKeyProof single   = 1;
    MultisigProof  multisig = 2;
  }
}

message SingleKeyProof {
  bytes pub_key        = 1;
  bytes signature      = 2;
  SigFormat sig_format = 3;
}

message MultisigProof {
  uint32 threshold               = 1;
  repeated bytes sub_pub_keys    = 2;
  repeated uint32 signer_indices = 3;
  repeated bytes sub_signatures  = 4;
  SigFormat sig_format           = 5;
}
```

- [ ] **Step 3: Commit proto source change**

```bash
git add proto/lumera/evmigration/proof.proto
git commit -m "$(cat <<'EOF'
evmigration(proto): rename LegacyProof to MigrationProof; add SIG_FORMAT_EIP191

Unifies proof shape for both legacy and new sides of the migration
messages. Adds SIG_FORMAT_EIP191 for new-side Keplr/Leap wallet signing.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Update `tx.proto` — remove `new_signature`, add `new_proof`

**Files:**
- Modify: `proto/lumera/evmigration/tx.proto`

- [ ] **Step 1: Locate message definitions**

Run: `grep -n "MsgClaimLegacyAccount\|MsgMigrateValidator\|new_signature\|legacy_proof\|LegacyProof" proto/lumera/evmigration/tx.proto`

- [ ] **Step 2: Rewrite the two message blocks**

Replace `MsgClaimLegacyAccount` and `MsgMigrateValidator` with:

```proto
message MsgClaimLegacyAccount {
  string new_address          = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string legacy_address       = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  MigrationProof legacy_proof = 3 [(gogoproto.nullable) = false];
  MigrationProof new_proof    = 4 [(gogoproto.nullable) = false];
}

message MsgMigrateValidator {
  string new_address          = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string legacy_address       = 2 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  MigrationProof legacy_proof = 3 [(gogoproto.nullable) = false];
  MigrationProof new_proof    = 4 [(gogoproto.nullable) = false];
}
```

Ensure the `import "lumera/evmigration/proof.proto";` statement is present at the top.

- [ ] **Step 3: Commit the tx.proto change**

```bash
git add proto/lumera/evmigration/tx.proto
git commit -m "$(cat <<'EOF'
evmigration(proto): replace new_signature with structured new_proof

new_proof carries a MigrationProof oneof so the destination side can be
single-key or multisig, mirroring the legacy_proof shape. Since the EVM
upgrade has not been deployed, no reserved tags needed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Regenerate protobuf Go code

**Files:**
- Regenerate: `x/evmigration/types/*.pb.go`

- [ ] **Step 1: Run the proto generator**

Run: `make build-proto`
Expected: proto regeneration completes without error; `x/evmigration/types/proof.pb.go` and `tx.pb.go` are updated.

- [ ] **Step 2: Inspect what changed**

Run: `git diff --stat x/evmigration/types/*.pb.go`
Expected: `proof.pb.go` and `tx.pb.go` modified.

- [ ] **Step 3: Attempt compile — expect failures in Go callers**

Run: `go build ./x/evmigration/... 2>&1 | head -40`
Expected: compile errors referencing `LegacyProof`, `LegacyProof_Single`, `LegacyProof_Multisig`, `NewSignature` — these are what Task 4 fixes.

- [ ] **Step 4: Commit regenerated files**

```bash
git add x/evmigration/types/proof.pb.go x/evmigration/types/tx.pb.go
git commit -m "$(cat <<'EOF'
evmigration(proto): regenerate Go code for MigrationProof rename

Known: Go callers in keeper, types, cli do not yet compile against the
new names; they are fixed in the subsequent task.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Sweep-replace `LegacyProof` → `MigrationProof` across Go code

**Files (modify):**
- `x/evmigration/types/proof.go`
- `x/evmigration/types/proof_test.go`
- `x/evmigration/types/types.go`
- `x/evmigration/types/errors.go` (add `ErrInvalidMigrationProof` if named variants exist)
- `x/evmigration/keeper/verify.go`
- `x/evmigration/keeper/verify_test.go`
- `x/evmigration/keeper/msg_server_claim_legacy.go`
- `x/evmigration/keeper/msg_server_claim_legacy_test.go`
- `x/evmigration/keeper/msg_server_migrate_validator.go`
- `x/evmigration/keeper/msg_server_migrate_validator_test.go`
- `x/evmigration/client/cli/tx.go`
- `x/evmigration/client/cli/tx_multisig.go`
- `x/evmigration/client/cli/tx_multisig_internal_test.go`
- `x/evmigration/client/cli/tx_test.go`
- `x/evmigration/client/cli/tx_multisig_test.go`
- `tests/integration/evmigration/` (any file referencing `LegacyProof`)

- [ ] **Step 1: Find all callers**

Run: `grep -rn "LegacyProof\b\|ErrInvalidLegacyProof\|ErrInvalidLegacyPubKey\|ErrInvalidLegacySignature" x/evmigration/ tests/integration/evmigration/ 2>&1 | wc -l`
Expected: a count of the lines needing update (≈80-120).

- [ ] **Step 2: Sweep-replace in Go files**

Run: `grep -rl "LegacyProof\b" x/evmigration/ tests/integration/evmigration/ --include="*.go" | xargs sed -i 's/\bLegacyProof\b/MigrationProof/g'`

- [ ] **Step 2b: Unify error sentinels in `x/evmigration/types/errors.go`**

The existing file has both `ErrInvalidLegacy*` AND `ErrInvalidNew*` variants (see [types/errors.go:19-28](x/evmigration/types/errors.go#L19-L28)). After this refactor the verifier is side-agnostic, so side-specific error names become misleading. Collapse into one neutral taxonomy:

**Existing code map** (from [types/errors.go](x/evmigration/types/errors.go)):

| Code | Existing symbol | Disposition in this refactor |
|------|-----------------|------------------------------|
| 1100 | `ErrInvalidSigner` | kept, unchanged |
| 1101 | `ErrMigrationDisabled` | kept, unchanged |
| 1102 | `ErrMigrationWindowClosed` | kept, unchanged |
| 1103 | `ErrBlockRateLimitExceeded` | kept, unchanged |
| 1104 | `ErrSameAddress` | kept, unchanged |
| 1105 | `ErrAlreadyMigrated` | kept, unchanged |
| 1106 | `ErrNewAddressWasMigrated` | kept, unchanged |
| 1107 | `ErrCannotMigrateModuleAccount` | kept, unchanged |
| 1108 | `ErrUseValidatorMigration` | kept, unchanged |
| 1109 | `ErrLegacyAccountNotFound` | kept, unchanged |
| **1110** | `ErrInvalidLegacyPubKey` | **renamed** → `ErrInvalidMigrationPubKey` (same code) |
| **1111** | `ErrPubKeyAddressMismatch` | **kept** (name is already neutral, same code) |
| **1112** | `ErrInvalidLegacySignature` | **renamed** → `ErrInvalidMigrationSignature` (same code) |
| 1113 | `ErrNotValidator` | **kept unchanged** — do NOT reuse 1113 |
| 1114 | `ErrValidatorUnbonding` | kept, unchanged |
| 1115 | `ErrTooManyDelegators` | kept, unchanged |
| **1116** | `ErrInvalidNewPubKey` | **deleted** (callers use `ErrInvalidMigrationPubKey` at 1110) |
| **1117** | `ErrNewPubKeyAddressMismatch` | **deleted** (callers use `ErrPubKeyAddressMismatch` at 1111) |
| **1118** | `ErrInvalidNewSignature` | **deleted** (callers use `ErrInvalidMigrationSignature` at 1112) |
| 1119 | `ErrNewAddressAlreadyUsed` | kept, unchanged |
| **1120** | `ErrInvalidLegacyProof` | **renamed** → `ErrInvalidMigrationProof` (same code) |

Concrete edit: in `x/evmigration/types/errors.go`, the unified declarations reuse the vacated code slots so `ErrNotValidator`'s code 1113 is untouched:

```go
var (
    // codes 1100-1109 unchanged.

    ErrInvalidMigrationPubKey    = errors.Register(ModuleName, 1110, "invalid public key in migration proof")
    ErrPubKeyAddressMismatch     = errors.Register(ModuleName, 1111, "public key does not derive to claimed address") // unchanged
    ErrInvalidMigrationSignature = errors.Register(ModuleName, 1112, "migration signature verification failed")

    ErrNotValidator              = errors.Register(ModuleName, 1113, "legacy address is not a validator operator") // unchanged
    ErrValidatorUnbonding        = errors.Register(ModuleName, 1114, "validator is unbonding or unbonded; wait for completion") // unchanged
    ErrTooManyDelegators         = errors.Register(ModuleName, 1115, "validator has too many delegators; exceeds max_validator_delegations") // unchanged

    // 1116, 1117, 1118 left intentionally unregistered — reclaimed from the side-specific
    // ErrInvalidNewPubKey / ErrNewPubKeyAddressMismatch / ErrInvalidNewSignature which no
    // longer exist. Do not reuse these codes for new sentinels in this module to avoid
    // confusion with pre-refactor clients (though nothing on chain observes error codes).

    ErrNewAddressAlreadyUsed     = errors.Register(ModuleName, 1119, "new address was already used as a migration destination") // unchanged
    ErrInvalidMigrationProof     = errors.Register(ModuleName, 1120, "invalid migration proof")
)
```

**Delete** from `errors.go`: `ErrInvalidLegacyPubKey` (rename), `ErrInvalidLegacySignature` (rename), `ErrInvalidLegacyProof` (rename), `ErrInvalidNewPubKey` (delete; callers map to 1110), `ErrInvalidNewSignature` (delete; callers map to 1112), `ErrNewPubKeyAddressMismatch` (delete; callers map to 1111). Codes 1116-1118 are left vacant.

- [ ] **Step 2c: Sweep-update callers to the unified names**

Run: `grep -rln "ErrInvalidLegacyProof\|ErrInvalidLegacyPubKey\|ErrInvalidLegacySignature\|ErrInvalidNewPubKey\|ErrInvalidNewSignature\|ErrNewPubKeyAddressMismatch" x/evmigration/ tests/integration/evmigration/ --include="*.go" | xargs sed -i \
  -e 's/ErrInvalidLegacyProof/ErrInvalidMigrationProof/g' \
  -e 's/ErrInvalidLegacyPubKey/ErrInvalidMigrationPubKey/g' \
  -e 's/ErrInvalidLegacySignature/ErrInvalidMigrationSignature/g' \
  -e 's/ErrInvalidNewPubKey/ErrInvalidMigrationPubKey/g' \
  -e 's/ErrInvalidNewSignature/ErrInvalidMigrationSignature/g' \
  -e 's/ErrNewPubKeyAddressMismatch/ErrPubKeyAddressMismatch/g'`

Where Wrap-site messages previously said `"legacy pubkey derives to %s"`, update the text to include a `side-` prefix: `"(legacy side) pubkey derives to %s"` or `"(new side) pubkey derives to %s"` so clients can still tell which half failed.

- [ ] **Step 3: Rebuild**

Run: `go build ./x/evmigration/... 2>&1 | head -40`
Expected: compile succeeds. If errors remain, they're likely in `types.MigrationProof_Single` / `types.MigrationProof_Multisig` oneof types — fix by updating the sed-produced references (the proto-generated type names auto-changed from `LegacyProof_Single` → `MigrationProof_Single`).

- [ ] **Step 4: Run unit tests**

Run: `go test ./x/evmigration/... -v -count=1 2>&1 | tail -30`
Expected: many tests pass; some may fail because they reference `new_signature` (replaced next phase). Note these and keep going.

- [ ] **Step 5: Lint**

Run: `make lint`
Expected: 0 issues from the sweep.

- [ ] **Step 6: Commit**

```bash
git add x/evmigration/ tests/integration/evmigration/
git commit -m "$(cat <<'EOF'
evmigration: rename LegacyProof symbols to MigrationProof across Go

Pure mechanical rename following the proto rename. Tests that depend on
new_signature are not yet updated — that is done as part of the
verifier refactor.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Make `MigrationProof.ValidateBasic` side-aware

**Files:**
- Modify: `x/evmigration/types/proof.go`
- Modify: `x/evmigration/types/proof_test.go`

- [ ] **Step 1: Define `Side` type**

In `x/evmigration/types/proof.go`, add near the top:

```go
// Side identifies which half of a migration a proof is proving.
type Side int

const (
    SideLegacy Side = iota + 1
    SideNew
)
```

- [ ] **Step 2: Write failing tests**

In `x/evmigration/types/proof_test.go`, add:

```go
// SingleKeyProof length rules (per design §4.1): 64-byte Cosmos sig, 65-byte eth sig.

func TestMigrationProof_ValidateBasic_SingleKey_EIP191_RejectedOnLegacySide(t *testing.T) {
    proof := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
        PubKey:    make([]byte, secp256k1.PubKeySize),
        Signature: make([]byte, 64), // correct LEGACY length; EIP191 rejection should still fire
        SigFormat: types.SigFormat_SIG_FORMAT_EIP191,
    }}}
    err := proof.ValidateBasic(types.SideLegacy)
    require.ErrorContains(t, err, "EIP191")
}

func TestMigrationProof_ValidateBasic_SingleKey_EIP191_AcceptedOnNewSide(t *testing.T) {
    proof := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
        PubKey:    make([]byte, secp256k1.PubKeySize),
        Signature: make([]byte, 65), // strict 65 for new side
        SigFormat: types.SigFormat_SIG_FORMAT_EIP191,
    }}}
    require.NoError(t, proof.ValidateBasic(types.SideNew))
}

func TestMigrationProof_ValidateBasic_SingleKey_RejectWrongSigLenPerSide(t *testing.T) {
    // Legacy side: 65-byte sig rejected.
    legacy := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
        PubKey:    make([]byte, secp256k1.PubKeySize),
        Signature: make([]byte, 65),
        SigFormat: types.SigFormat_SIG_FORMAT_CLI,
    }}}
    err := legacy.ValidateBasic(types.SideLegacy)
    require.Error(t, err)
    require.ErrorContains(t, err, "64 bytes")

    // New side: 64-byte sig rejected.
    newSide := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
        PubKey:    make([]byte, secp256k1.PubKeySize),
        Signature: make([]byte, 64),
        SigFormat: types.SigFormat_SIG_FORMAT_CLI,
    }}}
    err = newSide.ValidateBasic(types.SideNew)
    require.Error(t, err)
    require.ErrorContains(t, err, "65 bytes")
}

func TestMigrationProof_ValidateBasic_Multisig_EIP191_Rejected(t *testing.T) {
    proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:     1,
        SubPubKeys:    [][]byte{make([]byte, secp256k1.PubKeySize)},
        SignerIndices: []uint32{0},
        SubSignatures: [][]byte{make([]byte, 64)}, // legacy side would otherwise pass; EIP191 still rejected
        SigFormat:     types.SigFormat_SIG_FORMAT_EIP191,
    }}}
    for _, side := range []types.Side{types.SideLegacy, types.SideNew} {
        err := proof.ValidateBasic(side)
        require.ErrorContains(t, err, "EIP191")
    }
}

func TestMigrationProof_ValidateBasic_Multisig_RejectWrongSubSigLenPerSide(t *testing.T) {
    // Legacy side: any sub-sig of non-64 length rejected.
    legacy := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:     1,
        SubPubKeys:    [][]byte{make([]byte, secp256k1.PubKeySize)},
        SignerIndices: []uint32{0},
        SubSignatures: [][]byte{make([]byte, 65)}, // wrong for legacy
        SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
    }}}
    err := legacy.ValidateBasic(types.SideLegacy)
    require.Error(t, err)
    require.ErrorContains(t, err, "64 bytes")

    // New side: any sub-sig of non-65 length rejected.
    newSide := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:     1,
        SubPubKeys:    [][]byte{make([]byte, secp256k1.PubKeySize)},
        SignerIndices: []uint32{0},
        SubSignatures: [][]byte{make([]byte, 64)}, // wrong for new
        SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
    }}}
    err = newSide.ValidateBasic(types.SideNew)
    require.Error(t, err)
    require.ErrorContains(t, err, "65 bytes")
}
```

- [ ] **Step 3: Run tests — expect fail**

Run: `go test ./x/evmigration/types/... -v -run TestMigrationProof_ValidateBasic -count=1`
Expected: compile error or `ValidateBasic` signature mismatch.

- [ ] **Step 4: Update `ValidateBasic` signature**

In `x/evmigration/types/proof.go`, change signature:

```go
func (p *MigrationProof) ValidateBasic(side Side) error {
    if p == nil {
        return ErrInvalidMigrationProof.Wrap("proof is nil")
    }
    switch inner := p.Proof.(type) {
    case *MigrationProof_Single:
        return inner.Single.validateBasic(side)
    case *MigrationProof_Multisig:
        return inner.Multisig.validateBasic(side)
    default:
        return ErrInvalidMigrationProof.Wrap("no proof set")
    }
}

func (p *SingleKeyProof) validateBasic(side Side) error {
    if len(p.PubKey) != secp256k1.PubKeySize {
        return ErrInvalidMigrationPubKey.Wrapf("expected %d bytes, got %d", secp256k1.PubKeySize, len(p.PubKey))
    }
    // Per-side signature length — see design §4.1 and §4.2 for rationale.
    // Legacy Cosmos secp256k1: 64 bytes (raw R||S). Cosmos keyring has no V convention.
    // New eth_secp256k1:       65 bytes (R||S||V). Every realistic eth signer (Cosmos EVM
    //                          keyring, go-ethereum crypto.Sign, Keplr/Leap personal_sign)
    //                          produces 65 bytes. V is ignored by the verifier but kept on
    //                          the wire for Ethereum-native tooling consistency.
    if side == SideLegacy && len(p.Signature) != 64 {
        return ErrInvalidMigrationSignature.Wrapf("legacy Cosmos secp256k1 signature must be 64 bytes, got %d", len(p.Signature))
    }
    if side == SideNew && len(p.Signature) != 65 {
        return ErrInvalidMigrationSignature.Wrapf("new eth_secp256k1 signature must be 65 bytes (R||S||V), got %d", len(p.Signature))
    }
    if p.SigFormat == SigFormat_SIG_FORMAT_UNSPECIFIED {
        return ErrInvalidMigrationProof.Wrap("sig_format unspecified")
    }
    if p.SigFormat == SigFormat_SIG_FORMAT_EIP191 && side != SideNew {
        return ErrInvalidMigrationProof.Wrap("EIP191 is only valid for new-side single-key proofs")
    }
    return nil
}

func (p *MultisigProof) validateBasic(side Side) error {
    if p.SigFormat == SigFormat_SIG_FORMAT_EIP191 {
        return ErrInvalidMigrationProof.Wrap("EIP191 is not valid for multisig proofs on either side")
    }
    // Preserve existing structural checks (N>=1, threshold bounds, signer_indices
    // exact-K + ascending + in-range, sub_signatures length matches).
    if err := p.validateStructure(); err != nil {
        return err
    }
    // Length-check EVERY sub_pub_key (not just indexed ones), because
    // LegacyAminoPubKey.Address() consumes all N sub-keys during derivation.
    for i, raw := range p.SubPubKeys {
        if len(raw) != secp256k1.PubKeySize {
            return ErrInvalidMigrationPubKey.Wrapf("sub_pub_keys[%d]: expected %d bytes, got %d",
                i, secp256k1.PubKeySize, len(raw))
        }
    }
    // Per-side sub-signature length enforcement (design §4.1 / §4.2):
    // legacy multisig sub-sigs are Cosmos secp256k1 → 64 bytes;
    // new multisig sub-sigs are eth_secp256k1 → 65 bytes (R||S||V).
    expectedSigLen := 64
    sigLabel := "legacy Cosmos secp256k1 sub-signature"
    if side == SideNew {
        expectedSigLen = 65
        sigLabel = "new eth_secp256k1 sub-signature"
    }
    for i, sig := range p.SubSignatures {
        if len(sig) != expectedSigLen {
            return ErrInvalidMigrationSignature.Wrapf("%s[%d]: expected %d bytes, got %d",
                sigLabel, i, expectedSigLen, len(sig))
        }
    }
    return nil
}
```

Extract the existing N/threshold/indices checks into `validateStructure()` if not already separated.

- [ ] **Step 5: Update callers**

Every call site that calls `ValidateBasic()` without a side argument must pass one. Find them:

Run: `grep -rn "MigrationProof.*ValidateBasic\|\.ValidateBasic()" x/evmigration/ | grep -v _test`

Update in:
- `x/evmigration/types/types.go` (**not** `tx.go` — the file is `types.go` at [types/types.go:23](x/evmigration/types/types.go#L23)). `MsgClaimLegacyAccount.ValidateBasic` calls `m.LegacyProof.ValidateBasic(SideLegacy)` and `m.NewProof.ValidateBasic(SideNew)`.
- `x/evmigration/types/types.go` (same for `MsgMigrateValidator`)
- `x/evmigration/keeper/verify.go` (`VerifyMigrationProof` passes the side param)

- [ ] **Step 6: Run the new tests — expect pass**

Run: `go test ./x/evmigration/types/... -v -run TestMigrationProof_ValidateBasic -count=1`
Expected: PASS.

- [ ] **Step 7: Run the full types test suite**

Run: `go test ./x/evmigration/types/... -v -count=1 2>&1 | tail -20`
Expected: all pass.

- [ ] **Step 8: Lint & commit**

```bash
make lint
git add x/evmigration/types/
git commit -m "$(cat <<'EOF'
evmigration(types): make ValidateBasic side-aware; reject EIP191 on legacy and on multisig

- Add Side type (SideLegacy, SideNew).
- SingleKeyProof accepts EIP191 only on SideNew.
- MultisigProof rejects EIP191 on both sides.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2 — Verifier Rewrite

### Task 6: Create shared `sigverify` package; add Cosmos and eth sig primitives

**Files:**
- Create: `x/evmigration/types/sigverify/sigverify.go`
- Create: `x/evmigration/types/sigverify/sigverify_test.go`

**Why a shared package:** the keeper verifier AND the CLI's `combine-proof` must use identical verification logic (per design Finding 2 resolution — combine-proof verifies partials cryptographically before threshold selection). Duplicating the primitives in keeper and CLI risks divergence. Put them in `types/sigverify` so both can import them.

- [ ] **Step 1: Write failing tests**

In `x/evmigration/types/sigverify/sigverify_test.go`:

```go
package sigverify_test

import (
    "bytes"
    "crypto/sha256" // required for the Cosmos-CLI hash path
    "testing"

    "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
    sdk "github.com/cosmos/cosmos-sdk/types"
    ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"
    "github.com/stretchr/testify/require"

    "github.com/LumeraProtocol/lumera/x/evmigration/types"
    "github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

func TestVerifyCosmosSecp256k1_CLI(t *testing.T) {
    priv := secp256k1.GenPrivKey()
    pk := priv.PubKey().(*secp256k1.PubKey)
    payload := []byte("payload-bytes")
    hash := sha256.Sum256(payload)
    sig, err := priv.Sign(hash[:])
    require.NoError(t, err)
    require.NoError(t, sigverify.VerifyCosmosSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, types.SigFormat_SIG_FORMAT_CLI))
}

// Cosmos EVM v0.6.0's ethsecp256k1.PrivKey.Sign returns a 65-byte recoverable
// signature (R||S||V). That is the ONLY accepted wire length on the new side —
// ValidateBasic rejects 64-byte input upfront, and sigverify.VerifyEthSecp256k1
// returns an error on anything but 65.

func TestVerifyEthSecp256k1_CLI_65byte(t *testing.T) {
    priv, err := ethsecp256k1.GenerateKey()
    require.NoError(t, err)
    pk := priv.PubKey().(*ethsecp256k1.PubKey)
    payload := []byte("payload-bytes")
    sig, err := priv.Sign(payload) // ethsecp256k1 Sign does Keccak256 internally
    require.NoError(t, err)
    require.Equal(t, 65, len(sig), "SDK eth sig contract is 65 bytes (R||S||V); got %d", len(sig))
    require.NoError(t, sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, types.SigFormat_SIG_FORMAT_CLI))
}

func TestVerifyEthSecp256k1_EIP191_65byte(t *testing.T) {
    // Round-trip: sign the EIP-191-wrapped payload, verify under EIP-191 format.
    // sigverify slices off the V byte before ECDSA verify.
    priv, err := ethsecp256k1.GenerateKey()
    require.NoError(t, err)
    pk := priv.PubKey().(*ethsecp256k1.PubKey)
    payload := []byte("payload-bytes")
    wrapped := sigverify.EIP191PersonalSignPayload(payload)
    sig, err := priv.Sign(wrapped)
    require.NoError(t, err)
    require.Equal(t, 65, len(sig), "SDK eth sig contract is 65 bytes (R||S||V); got %d", len(sig))
    require.NoError(t, sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, types.SigFormat_SIG_FORMAT_EIP191))
}

func TestVerifyEthSecp256k1_ADR036_65byte(t *testing.T) {
    // Coverage for the third format under the same strict-65 contract.
    priv, err := ethsecp256k1.GenerateKey()
    require.NoError(t, err)
    pk := priv.PubKey().(*ethsecp256k1.PubKey)
    payload := []byte("payload-bytes")
    signerAddr := sdk.AccAddress(pk.Address())
    doc := sigverify.ADR036SignDoc(signerAddr.String(), payload)
    sig, err := priv.Sign(doc)
    require.NoError(t, err)
    require.Equal(t, 65, len(sig))
    require.NoError(t, sigverify.VerifyEthSecp256k1(pk, signerAddr, payload, sig, types.SigFormat_SIG_FORMAT_ADR036))
}

func TestVerifyEthSecp256k1_VByteIgnoredByVerifier(t *testing.T) {
    // The V byte is recovery metadata and NOT used by our ECDSA-verify-under-pubkey
    // path. An eth signature with a wrong V byte should still verify because
    // sigverify slices sig[:64] before calling VerifySignature. Lock this in —
    // a future refactor that "validates" V against the recovered pubkey would be
    // functionally correct but break the documented "V is ignored" contract.
    priv, err := ethsecp256k1.GenerateKey()
    require.NoError(t, err)
    pk := priv.PubKey().(*ethsecp256k1.PubKey)
    payload := []byte("payload-bytes")
    sig, err := priv.Sign(payload)
    require.NoError(t, err)
    // Clobber V with an arbitrary value. R||S is unchanged; verification
    // under the supplied pubkey must still pass.
    tampered := bytes.Clone(sig)
    tampered[64] ^= 0xff
    require.NoError(t, sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, tampered, types.SigFormat_SIG_FORMAT_CLI))
}

func TestVerifyEthSecp256k1_Reject64Byte(t *testing.T) {
    // Strict wire contract: 64-byte input is rejected with a clear error.
    // Regression lock against a future refactor that "helpfully" accepts 64.
    priv, err := ethsecp256k1.GenerateKey()
    require.NoError(t, err)
    pk := priv.PubKey().(*ethsecp256k1.PubKey)
    payload := []byte("payload-bytes")
    sig, err := priv.Sign(payload)
    require.NoError(t, err)
    sig64 := bytes.Clone(sig[:64])
    err = sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig64, types.SigFormat_SIG_FORMAT_CLI)
    require.Error(t, err)
    require.ErrorContains(t, err, "65 bytes")
}

func TestVerifyEthSecp256k1_RejectOtherLengths(t *testing.T) {
    priv, err := ethsecp256k1.GenerateKey()
    require.NoError(t, err)
    pk := priv.PubKey().(*ethsecp256k1.PubKey)
    for _, badLen := range []int{0, 63, 66, 128} {
        err := sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), []byte("x"), make([]byte, badLen), types.SigFormat_SIG_FORMAT_CLI)
        require.Error(t, err, "len=%d should be rejected", badLen)
        require.ErrorContains(t, err, "65 bytes")
    }
}
```

- [ ] **Step 2: Implement the package**

In `x/evmigration/types/sigverify/sigverify.go`:

```go
package sigverify

import (
    "crypto/sha256"
    "encoding/base64"
    "fmt"

    "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
    sdk "github.com/cosmos/cosmos-sdk/types"
    ethsecp256k1 "github.com/cosmos/evm/crypto/ethsecp256k1"

    "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// SubKeyType identifies which curve/hash-convention the verifier should use
// for a single-key or multisig sub-key verification. Exported here (rather
// than in x/evmigration/keeper) so both the keeper's VerifyMigrationProof
// and the CLI's combine-proof can share a single definition.
type SubKeyType int

const (
    SubKeyTypeCosmosSecp256k1 SubKeyType = iota + 1 // legacy-side sub-keys
    SubKeyTypeEthSecp256k1                          // new-side sub-keys
)

// EIP191PersonalSignPayload wraps msg in the EIP-191 "personal_sign" envelope:
//   "\x19Ethereum Signed Message:\n" || decimal(len(msg)) || msg
func EIP191PersonalSignPayload(msg []byte) []byte {
    prefix := fmt.Appendf(nil, "\x19Ethereum Signed Message:\n%d", len(msg))
    return append(prefix, msg...)
}

// ADR036SignDoc builds the canonical ADR-036 sign doc (alphabetically-sorted JSON).
func ADR036SignDoc(signer string, data []byte) []byte {
    return []byte(fmt.Sprintf(
        `{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},`+
            `"memo":"","msgs":[{"type":"sign/MsgSignData","value":`+
            `{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
        base64.StdEncoding.EncodeToString(data), signer,
    ))
}

// VerifyCosmosSecp256k1 checks a single Cosmos secp256k1 signature over the
// migration payload, accepting either CLI (raw SHA256) or ADR-036 (canonical JSON) envelope.
func VerifyCosmosSecp256k1(pk *secp256k1.PubKey, signerAddr sdk.AccAddress, payload, sig []byte, format types.SigFormat) error {
    switch format {
    case types.SigFormat_SIG_FORMAT_CLI:
        hash := sha256.Sum256(payload)
        if pk.VerifySignature(hash[:], sig) {
            return nil
        }
    case types.SigFormat_SIG_FORMAT_ADR036:
        doc := ADR036SignDoc(signerAddr.String(), payload)
        if pk.VerifySignature(doc, sig) {
            return nil
        }
    case types.SigFormat_SIG_FORMAT_EIP191:
        return types.ErrInvalidMigrationProof.Wrap("EIP191 is not valid for Cosmos secp256k1 signatures")
    default:
        return types.ErrInvalidMigrationProof.Wrap("sig_format unspecified")
    }
    return types.ErrInvalidMigrationSignature
}

// VerifyEthSecp256k1 checks a single eth_secp256k1 signature.
//
// Wire contract (design §4.1): eth signatures are strictly 65 bytes (R||S||V).
// This function REJECTS any other length — including the 64-byte form — so a
// caller that skipped ValidateBasic doesn't sneak a malformed sig through.
//
// Verification semantics (design §4.2): direct-verify, NOT ecrecover-and-compare.
//   - Build the format-specific message bytes (CLI = raw payload, ADR-036 =
//     canonical sign-doc, EIP-191 = personal-sign-wrapped payload).
//   - Slice off the V byte (sig[:64]); V is recovery metadata ignored by the
//     verifier and kept on the wire only for Ethereum-native tooling
//     consistency.
//   - Call pk.VerifySignature(msg, sig[:64]) — VerifySignature internally
//     applies Keccak256 and performs ECDSA verify under the supplied pubkey.
//   - The caller (verifySingleKeyProof or verifyMultisigProof) independently
//     asserts that sdk.AccAddress(pk.Address()) == bound_addr, which binds
//     the pubkey to the declared new_address.
func VerifyEthSecp256k1(pk *ethsecp256k1.PubKey, signerAddr sdk.AccAddress, payload, sig []byte, format types.SigFormat) error {
    // Strict wire format: eth signatures are always 65 bytes (R||S||V) per
    // design §4.1. ValidateBasic should have rejected non-65-byte input
    // upstream; this length check is a defense-in-depth belt-and-braces
    // guard for any direct callers that skip ValidateBasic.
    if len(sig) != 65 {
        return types.ErrInvalidMigrationSignature.Wrapf("eth signature must be 65 bytes (R||S||V), got %d", len(sig))
    }
    var msg []byte
    switch format {
    case types.SigFormat_SIG_FORMAT_CLI:
        msg = payload
    case types.SigFormat_SIG_FORMAT_EIP191:
        msg = EIP191PersonalSignPayload(payload)
    case types.SigFormat_SIG_FORMAT_ADR036:
        msg = ADR036SignDoc(signerAddr.String(), payload)
    default:
        return types.ErrInvalidMigrationProof.Wrap("sig_format unspecified")
    }
    // Slice off the V recovery byte — pk.VerifySignature needs R||S only.
    // (V is redundant for verify-under-pubkey; we keep it on the wire for
    // Ethereum tooling compatibility per design §4.1.)
    if pk.VerifySignature(msg, sig[:64]) {
        return nil
    }
    return types.ErrInvalidMigrationSignature
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./x/evmigration/types/sigverify/... -v -count=1`
Expected: all pass.

- [ ] **Step 4: Commit**

```bash
make lint
git add x/evmigration/types/sigverify/
git commit -m "$(cat <<'EOF'
evmigration(sigverify): shared signature primitives for keeper and CLI

Factors VerifyCosmosSecp256k1, VerifyEthSecp256k1, EIP191PersonalSignPayload,
and ADR036SignDoc into x/evmigration/types/sigverify. Both the keeper
verifier and the CLI combine-proof command import these helpers, so
verification logic cannot drift between them.

Per design Finding 5: EIP-191 verification is direct-verify (strip
recovery byte when present, VerifySignature under supplied pubkey), not
ecrecover-and-compare. Rationale: avoids v-byte convention ambiguity
and pubkey-recovery divergence. The address-binding check stays in the
single-key proof verifier as before.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5: Update `x/evmigration/keeper/verify.go` to import from the shared package**

Replace the existing `verifySecp256k1Sig` inline function with `sigverify.VerifyCosmosSecp256k1`; likewise any EIP-191 / ADR-036 helpers. Remove `eip191PersonalSignPayload` / `adr036SignDoc` from `verify.go` — they now live in `sigverify`.

- [ ] **Step 6: Rebuild & test**

Run: `go build ./x/evmigration/... && go test ./x/evmigration/keeper/ -run TestVerify -v -count=1`
Expected: pass.

- [ ] **Step 7: Commit**

```bash
make lint
git add x/evmigration/keeper/verify.go
git commit -m "evmigration(verify): delegate sig primitives to types/sigverify

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Add `VerifyMigrationProof` (keeping old functions alive for now)

**Files:**
- Modify: `x/evmigration/keeper/verify.go`
- Modify: `x/evmigration/keeper/verify_test.go`

**Execution-order note:** this task adds the new `VerifyMigrationProof` function alongside the existing `VerifyLegacyProof` and `VerifyNewSignature`. It does **NOT** delete the old functions — msg-servers still call them. The old functions are removed in **Task 11.5 (cleanup)** after Tasks 10 and 11 update the msg-servers. This preserves the invariant "every commit compiles and lints."

- [ ] **Step 1: Import `SubKeyType` from sigverify**

Don't redefine `SubKeyType` in the keeper — use the one exported from `x/evmigration/types/sigverify` (defined in Task 6). In `verify.go`:

```go
import (
    "github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)
// then reference sigverify.SubKeyType, sigverify.SubKeyTypeCosmosSecp256k1, sigverify.SubKeyTypeEthSecp256k1
```

- [ ] **Step 2: Refactor `verifySingleKeyProof` to be sub-key-type-aware**

Replace:

```go
func verifySingleKeyProof(payload []byte, boundAddr sdk.AccAddress, p *types.SingleKeyProof, keyType sigverify.SubKeyType) error {
    if len(p.PubKey) != secp256k1.PubKeySize {
        return types.ErrInvalidMigrationPubKey.Wrapf("expected %d bytes, got %d", secp256k1.PubKeySize, len(p.PubKey))
    }
    switch keyType {
    case sigverify.SubKeyTypeCosmosSecp256k1:
        pk := &secp256k1.PubKey{Key: p.PubKey}
        if !sdk.AccAddress(pk.Address()).Equals(boundAddr) {
            return types.ErrPubKeyAddressMismatch.Wrapf("pubkey derives to %s, expected %s", sdk.AccAddress(pk.Address()), boundAddr)
        }
        return sigverify.VerifyCosmosSecp256k1(pk, boundAddr, payload, p.Signature, p.SigFormat)
    case sigverify.SubKeyTypeEthSecp256k1:
        pk := &ethsecp256k1.PubKey{Key: p.PubKey}
        if !sdk.AccAddress(pk.Address()).Equals(boundAddr) {
            return types.ErrPubKeyAddressMismatch.Wrapf("pubkey derives to %s, expected %s", sdk.AccAddress(pk.Address()), boundAddr)
        }
        return sigverify.VerifyEthSecp256k1(pk, boundAddr, payload, p.Signature, p.SigFormat)
    default:
        return types.ErrInvalidMigrationProof.Wrap("unknown sub-key type")
    }
}
```

- [ ] **Step 3: Refactor `verifyMultisigProof` similarly**

```go
func verifyMultisigProof(payload []byte, boundAddr sdk.AccAddress, m *types.MultisigProof, keyType sigverify.SubKeyType) error {
    subPubKeys := make([]cryptotypes.PubKey, len(m.SubPubKeys))
    for i, raw := range m.SubPubKeys {
        if len(raw) != secp256k1.PubKeySize {
            return types.ErrInvalidMigrationPubKey.Wrapf("sub_pub_keys[%d]: expected %d bytes, got %d", i, secp256k1.PubKeySize, len(raw))
        }
        switch keyType {
        case sigverify.SubKeyTypeCosmosSecp256k1:
            subPubKeys[i] = &secp256k1.PubKey{Key: raw}
        case sigverify.SubKeyTypeEthSecp256k1:
            subPubKeys[i] = &ethsecp256k1.PubKey{Key: raw}
        default:
            return types.ErrInvalidMigrationProof.Wrap("unknown sub-key type")
        }
    }
    multiPK := kmultisig.NewLegacyAminoPubKey(int(m.Threshold), subPubKeys)
    if !sdk.AccAddress(multiPK.Address()).Equals(boundAddr) {
        return types.ErrPubKeyAddressMismatch.Wrapf("multisig pubkey derives to %s, expected %s", sdk.AccAddress(multiPK.Address()), boundAddr)
    }
    for i, idx := range m.SignerIndices {
        if int(idx) >= len(subPubKeys) {
            return types.ErrInvalidMigrationProof.Wrapf("signer_indices[%d]=%d out of range", i, idx)
        }
        switch pk := subPubKeys[idx].(type) {
        case *secp256k1.PubKey:
            signerAddr := sdk.AccAddress(pk.Address())
            if err := sigverify.VerifyCosmosSecp256k1(pk, signerAddr, payload, m.SubSignatures[i], m.SigFormat); err != nil {
                return types.ErrInvalidMigrationSignature.Wrapf("sub-sig %d (signer %s) invalid: %s", i, signerAddr, err)
            }
        case *ethsecp256k1.PubKey:
            signerAddr := sdk.AccAddress(pk.Address())
            if err := sigverify.VerifyEthSecp256k1(pk, signerAddr, payload, m.SubSignatures[i], m.SigFormat); err != nil {
                return types.ErrInvalidMigrationSignature.Wrapf("sub-sig %d (signer %s) invalid: %s", i, signerAddr, err)
            }
        }
    }
    return nil
}
```

- [ ] **Step 4: Add `VerifyMigrationProof` (alongside the existing `VerifyLegacyProof`)**

```go
func VerifyMigrationProof(
    chainID string, evmChainID uint64, kind string,
    legacyAddr, newAddr, boundAddr sdk.AccAddress,
    proof *types.MigrationProof,
    keyType sigverify.SubKeyType,
) error {
    if proof == nil {
        return types.ErrInvalidMigrationProof.Wrap("proof required")
    }
    side := types.SideLegacy
    if keyType == sigverify.SubKeyTypeEthSecp256k1 {
        side = types.SideNew
    }
    if err := proof.ValidateBasic(side); err != nil {
        return err
    }
    payload := migrationPayload(chainID, evmChainID, kind, legacyAddr, newAddr)
    switch p := proof.Proof.(type) {
    case *types.MigrationProof_Single:
        return verifySingleKeyProof(payload, boundAddr, p.Single, keyType)
    case *types.MigrationProof_Multisig:
        return verifyMultisigProof(payload, boundAddr, p.Multisig, keyType)
    default:
        return types.ErrInvalidMigrationProof.Wrap("no proof set")
    }
}
```

**Do NOT delete `VerifyLegacyProof` or `VerifyNewSignature` yet** — they are still called by the msg-servers. They are removed in Task 11.5 after Tasks 10 and 11 update the callers. If your IDE or `staticcheck` complains about the *new* `VerifyMigrationProof` being apparently dead code, it's because no caller wires to it until Task 10 — ignore until then.

- [ ] **Step 5: Run legacy-side unit tests — expect pass**

Run: `go test ./x/evmigration/keeper/ -run TestVerify -v -count=1 2>&1 | tail -20`
Expected: all existing legacy-multisig + single-key tests pass.

- [ ] **Step 6: Commit**

```bash
make lint
git add x/evmigration/keeper/verify.go x/evmigration/keeper/verify_test.go
git commit -m "$(cat <<'EOF'
evmigration(verify): unify VerifyLegacyProof+VerifyNewSignature into VerifyMigrationProof

Parameterized by sigverify.SubKeyType. Legacy side passes
sigverify.SubKeyTypeCosmosSecp256k1 and boundAddr=legacyAddr; new side
passes sigverify.SubKeyTypeEthSecp256k1 and boundAddr=newAddr.
Per-sub-key signature verification dispatches on concrete type (Cosmos
secp256k1 vs eth secp256k1) to handle the different hash conventions
(SHA256 vs Keccak256).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8 — [DEFERRED cleanup; execute AFTER Task 11]

> **DO NOT RUN THIS TASK IN SEQUENCE.** It deletes functions that Tasks 10 and 11 are in the middle of removing calls to. Running it before Task 10 and Task 11 both land will leave the tree in a state where `msg_server_claim_legacy.go` and `msg_server_migrate_validator.go` fail to compile — violating the plan's "every commit compiles and lints" rule. Come back to this task after Task 11 commits.

**Purpose:** remove the now-unused `VerifyLegacyProof`, `VerifyNewSignature`, and the ECDSA-recovery helpers.

**Files:**
- Modify: `x/evmigration/keeper/verify.go`
- Modify: `x/evmigration/keeper/verify_test.go`

- [ ] **Step 1: Confirm all callers of `VerifyNewSignature` and `VerifyLegacyProof` are gone**

Run: `grep -rn "VerifyNewSignature\|VerifyLegacyProof\|recoverDerivedNewAddresses\|normalizeRecoverySignatures\|findMatchingRecoveredAddress" x/evmigration/`
Expected: output limited to verify.go itself (the definitions and internal tests). If msg-server files show up, Tasks 10 / 11 are incomplete — do NOT proceed.

- [ ] **Step 2: Delete the unused functions and their tests**

From `x/evmigration/keeper/verify.go` remove:
- `VerifyLegacyProof` (superseded by `VerifyMigrationProof`)
- `VerifyNewSignature`, `normalizeRecoverySignatures`, `recoverDerivedNewAddresses`, `findMatchingRecoveredAddress` (ECDSA-recovery path no longer needed)

From `x/evmigration/keeper/verify_test.go` remove any tests that exercised those entry points. Keep the tests that target `VerifyMigrationProof` and the internal per-sub-key helpers.

Remove the import `"github.com/ethereum/go-ethereum/crypto"` if no longer referenced (check with `goimports -l x/evmigration/keeper/verify.go`).

- [ ] **Step 3: Build + lint + full evmigration test suite**

Run: `go build ./x/evmigration/... && go test ./x/evmigration/... -v -count=1 && make lint`
Expected: all green.

- [ ] **Step 4: Commit**

```bash
git add x/evmigration/keeper/verify.go x/evmigration/keeper/verify_test.go
git commit -m "$(cat <<'EOF'
evmigration(verify): remove unused VerifyLegacyProof, VerifyNewSignature, and recovery helpers

All callers now go through VerifyMigrationProof (see Tasks 10 and 11).
The ECDSA-recovery path for the new side is retired — direct-verify
under the supplied eth pubkey is the only supported verification.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Add unit tests for new-side multisig verification

**Files:**
- Modify: `x/evmigration/keeper/verify_test.go`

- [ ] **Step 1: Write end-to-end multisig-new-side tests**

Add to `verify_test.go`:

```go
func TestVerifyMigrationProof_NewSide_Multisig_Valid2of3(t *testing.T) {
    // Three eth_secp256k1 sub-keys.
    privs := [3]*ethsecp256k1.PrivKey{}
    pubs  := [3]cryptotypes.PubKey{}
    rawPubs := make([][]byte, 3)
    for i := range privs {
        p, _ := ethsecp256k1.GenerateKey()
        privs[i] = p
        pubs[i] = p.PubKey()
        rawPubs[i] = pubs[i].Bytes()
    }
    multiPK := kmultisig.NewLegacyAminoPubKey(2, pubs[:])
    newAddr := sdk.AccAddress(multiPK.Address())
    legacyAddr := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))

    payload := migrationPayload("test-chain", 76857769, migrationPayloadKindClaim, legacyAddr, newAddr)

    // Sign with subs 0 and 2.
    sig0, _ := privs[0].Sign(payload)
    sig2, _ := privs[2].Sign(payload)

    proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:     2,
        SubPubKeys:    rawPubs,
        SignerIndices: []uint32{0, 2},
        SubSignatures: [][]byte{sig0, sig2},
        SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
    }}}

    err := VerifyMigrationProof(
        "test-chain", 76857769, migrationPayloadKindClaim,
        legacyAddr, newAddr, newAddr,
        proof, sigverify.SubKeyTypeEthSecp256k1,
    )
    require.NoError(t, err)
}

// TestVerifyMigrationProof_NewSide_Multisig_AmionoAddressMismatch_OnKeyTypeSwap proves that
// when a multisig is constructed with Cosmos secp256k1 sub-keys but verified under
// SubKeyTypeEthSecp256k1, the verifier rejects with ErrPubKeyAddressMismatch.
//
// Why this works as a clean test: kmultisig.LegacyAminoPubKey.Address() is derived from the
// amino-encoded serialization of the LegacyAminoPubKey struct, which INCLUDES each sub-key's
// type-URL (e.g. `/cosmos.crypto.secp256k1.PubKey` vs `/cosmos.evm.crypto.v1.ethsecp256k1.PubKey`).
// So the same raw 33-byte bag produces different amino bytes and therefore different addresses
// depending on which sub-key type is chosen when rebuilding. We bind the proof to the
// Cosmos-built address and ask the verifier to rebuild as eth — the amino serialization
// diverges and the address comparison fires cleanly.
func TestVerifyMigrationProof_NewSide_Multisig_AmionoAddressMismatch_OnKeyTypeSwap(t *testing.T) {
    priv := secp256k1.GenPrivKey()
    pk := priv.PubKey()
    boundAddr := sdk.AccAddress(kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{pk}).Address())

    proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:     1,
        SubPubKeys:    [][]byte{pk.Bytes()}, // 33-byte Cosmos-compressed secp256k1 bag
        SignerIndices: []uint32{0},
        SubSignatures: [][]byte{make([]byte, 64)},
        SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
    }}}

    err := VerifyMigrationProof(
        "test-chain", 76857769, migrationPayloadKindClaim,
        boundAddr, boundAddr, boundAddr,
        proof, sigverify.SubKeyTypeEthSecp256k1, // verifier wraps bytes as eth_secp256k1
    )
    require.Error(t, err)
    require.ErrorIs(t, err, types.ErrPubKeyAddressMismatch,
        "expected address-derivation mismatch (amino bytes diverge on sub-key-type-URL), got: %v", err)
}

// TestVerifyMigrationProof_NewSide_Multisig_SubSigInvalid_UnderCosmosKeyBytes is the companion
// test that covers the case where the address HAPPENS to match (e.g. caller deliberately
// passes the eth-built address), but the sub-signature was produced with a Cosmos secp256k1 key
// and therefore fails under ethsecp256k1.PubKey.VerifySignature (Keccak256 hash mismatch).
// Precise expectation: ErrInvalidMigrationSignature.
func TestVerifyMigrationProof_NewSide_Multisig_SubSigInvalid_UnderCosmosKeyBytes(t *testing.T) {
    priv := secp256k1.GenPrivKey()
    pk := priv.PubKey().(*secp256k1.PubKey)

    // Build the bound address under the ETH interpretation (so the address comparison
    // inside verifyMultisigProof matches) — we're exercising the subsequent sub-sig check.
    ethPK := &ethsecp256k1.PubKey{Key: pk.Bytes()}
    boundAddr := sdk.AccAddress(kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{ethPK}).Address())

    // Produce a valid COSMOS-convention signature (SHA256 over payload).
    payload := migrationPayload("test-chain", 76857769, migrationPayloadKindClaim, boundAddr, boundAddr)
    hash := sha256.Sum256(payload)
    sig, err := priv.Sign(hash[:])
    require.NoError(t, err)

    proof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:     1,
        SubPubKeys:    [][]byte{pk.Bytes()},
        SignerIndices: []uint32{0},
        SubSignatures: [][]byte{sig},
        SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
    }}}

    err = VerifyMigrationProof(
        "test-chain", 76857769, migrationPayloadKindClaim,
        boundAddr, boundAddr, boundAddr,
        proof, sigverify.SubKeyTypeEthSecp256k1,
    )
    require.Error(t, err)
    require.ErrorIs(t, err, types.ErrInvalidMigrationSignature)
}
```

- [ ] **Step 2: Run**

Run: `go test ./x/evmigration/keeper/ -run "TestVerifyMigrationProof_NewSide" -v -count=1`
Expected: PASS for both.

- [ ] **Step 3: Commit**

```bash
make lint
git add x/evmigration/keeper/verify_test.go
git commit -m "evmigration(verify): unit tests for new-side multisig (eth_secp256k1 sub-keys)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 3 — Msg-Server Updates

### Task 10: Wire dual `VerifyMigrationProof` calls in `ClaimLegacyAccount`

**Files:**
- Modify: `x/evmigration/keeper/msg_server_claim_legacy.go`
- Modify: `x/evmigration/keeper/msg_server_claim_legacy_test.go`

- [ ] **Step 1: Replace the verification block**

Replace the existing two `Verify*` calls with:

```go
import "github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"

if err := msg.LegacyProof.ValidateParams(params.MaxMultisigSubKeys); err != nil {
    return nil, err
}
if err := msg.NewProof.ValidateParams(params.MaxMultisigSubKeys); err != nil {
    return nil, err
}
if err := VerifyMigrationProof(
    ctx.ChainID(), lcfg.EVMChainID, migrationPayloadKindClaim,
    legacyAddr, newAddr, legacyAddr,
    &msg.LegacyProof, sigverify.SubKeyTypeCosmosSecp256k1,
); err != nil {
    return nil, err
}
if err := VerifyMigrationProof(
    ctx.ChainID(), lcfg.EVMChainID, migrationPayloadKindClaim,
    legacyAddr, newAddr, newAddr,
    &msg.NewProof, sigverify.SubKeyTypeEthSecp256k1,
); err != nil {
    return nil, err
}
```

- [ ] **Step 2: Update existing tests to construct `NewProof` instead of `NewSignature`**

Grep for tests that set `NewSignature` on `MsgClaimLegacyAccount`:

Run: `grep -n "NewSignature" x/evmigration/keeper/msg_server_claim_legacy_test.go`

For each test, replace the `NewSignature: sig` line with `NewProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{PubKey: newPubKey, Signature: newSig, SigFormat: types.SigFormat_SIG_FORMAT_CLI}}}`.

Helper (add near the top of the test file):

```go
func newSingleKeyProof(pk []byte, sig []byte) types.MigrationProof {
    return types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
        PubKey: pk, Signature: sig, SigFormat: types.SigFormat_SIG_FORMAT_CLI,
    }}}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./x/evmigration/keeper/ -run TestClaimLegacy -v -count=1 2>&1 | tail -30`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
make lint
git add x/evmigration/keeper/msg_server_claim_legacy.go x/evmigration/keeper/msg_server_claim_legacy_test.go
git commit -m "$(cat <<'EOF'
evmigration(msg-server): dual VerifyMigrationProof in ClaimLegacyAccount

Replaces VerifyLegacyProof + VerifyNewSignature with two symmetric
VerifyMigrationProof calls — one sigverify.SubKeyTypeCosmosSecp256k1 bound to
legacyAddr, one sigverify.SubKeyTypeEthSecp256k1 bound to newAddr. Test fixtures
now build MigrationProof{Single} on both sides.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Same for `MigrateValidator`

**Files:**
- Modify: `x/evmigration/keeper/msg_server_migrate_validator.go`
- Modify: `x/evmigration/keeper/msg_server_migrate_validator_test.go`

- [ ] **Step 1-4: Mirror Task 10**

Replicate the Task 10 changes in the validator msg server and its test file. The diff is identical in shape (swap `migrationPayloadKindClaim` for `migrationPayloadKindValidator`). Use the same `newSingleKeyProof` helper.

- [ ] **Step 5: Run full evmigration unit suite**

Run: `go test ./x/evmigration/... -v -count=1 2>&1 | tail -40`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
make lint
git add x/evmigration/keeper/msg_server_migrate_validator.go x/evmigration/keeper/msg_server_migrate_validator_test.go
git commit -m "evmigration(msg-server): dual VerifyMigrationProof in MigrateValidator

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 4 — Multisig Destination Persistence

### Task 12: `MigrateAuth` sets `BaseAccount.PubKey` when destination is multisig

**Files:**
- Modify: `x/evmigration/keeper/migrate_auth.go`
- Modify: `x/evmigration/keeper/msg_server_claim_legacy.go` (updates `migrateAccount` helper signature)
- Modify: `x/evmigration/keeper/msg_server_migrate_validator.go` (equivalent validator path)
- Modify: `x/evmigration/keeper/migrate_test.go`

**Approach:** extend the existing `MigrateAuth(legacyAddr, newAddr) (*VestingInfo, error)` signature to `MigrateAuth(legacyAddr, newAddr, destProof *types.MigrationProof) (*VestingInfo, error)` — no parallel method. Because the keeper's internal helper `migrateAccount(ctx, legacyAddr, newAddr)` currently hides that call from the msg-servers, its signature must also change: `migrateAccount(ctx, legacyAddr, newAddr, destProof *types.MigrationProof) error`.

- [ ] **Step 1: Write failing tests using the existing `initMockFixture` pattern**

The existing `migrate_test.go` tests use `initMockFixture(t)` plus gomock `EXPECT` calls on `accountKeeper` — **not** a direct-use-keeper pattern. Follow that exact convention. See [keeper/migrate_test.go:48](x/evmigration/keeper/migrate_test.go#L48) for `initMockFixture` and [keeper/migrate_test.go:150-166](x/evmigration/keeper/migrate_test.go#L150-L166) for a canonical example.

Add to `migrate_test.go`:

```go
// TestMigrateAuth_MultisigDestination_SetsPubKey verifies that when destProof
// carries a multisig, MigrateAuth calls SetPubKey on the new BaseAccount with
// the reconstructed LegacyAminoPubKey (eth sub-keys) before SetAccount.
func TestMigrateAuth_MultisigDestination_SetsPubKey(t *testing.T) {
    f := initMockFixture(t)
    legacy := testAccAddr()

    // Build a 2-of-3 eth multisig and derive the new address from it.
    subPubs := make([][]byte, 3)
    subPubKeys := make([]cryptotypes.PubKey, 3)
    for i := 0; i < 3; i++ {
        p, err := ethsecp256k1.GenerateKey()
        require.NoError(t, err)
        subPubKeys[i] = p.PubKey()
        subPubs[i] = subPubKeys[i].Bytes()
    }
    multiPK := kmultisig.NewLegacyAminoPubKey(2, subPubKeys)
    newAddr := sdk.AccAddress(multiPK.Address())

    baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
    freshNewAcc := authtypes.NewBaseAccountWithAddress(newAddr)

    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(baseAcc)
    f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
    f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(freshNewAcc)
    // Key assertion: SetAccount is called with an account whose PubKey is the
    // reconstructed multisig LegacyAminoPubKey over the eth sub-keys.
    f.accountKeeper.EXPECT().
        SetAccount(gomock.Any(), gomock.AssignableToTypeOf(&authtypes.BaseAccount{})).
        Do(func(_ sdk.Context, acc sdk.AccountI) {
            require.NotNil(t, acc.GetPubKey(), "BaseAccount.PubKey must be set for multisig destination")
            require.Equal(t, multiPK.Address(), acc.GetPubKey().Address())
        })

    destProof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:  2,
        SubPubKeys: subPubs,
    }}}

    _, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, destProof)
    require.NoError(t, err)
}

// TestMigrateAuth_MultisigDestination_AddressMismatch asserts the Finding 4
// defensive check: if a caller passes destProof whose reconstructed multisig
// address does not match newAddr, MigrateAuth refuses WITHOUT mutating state
// and returns ErrPubKeyAddressMismatch. Since the check runs up front,
// gomock expects NO accountKeeper calls — not RemoveAccount, not GetAccount,
// not NewAccountWithAddress, not SetAccount. The absence of EXPECT() calls
// combined with strict gomock makes any keeper access fail the test.
func TestMigrateAuth_MultisigDestination_AddressMismatch(t *testing.T) {
    f := initMockFixture(t)
    legacy := testAccAddr()

    // Build a real multisig, then deliberately supply a *different* newAddr.
    subPubs := make([][]byte, 3)
    subPubKeys := make([]cryptotypes.PubKey, 3)
    for i := 0; i < 3; i++ {
        p, err := ethsecp256k1.GenerateKey()
        require.NoError(t, err)
        subPubKeys[i] = p.PubKey()
        subPubs[i] = subPubKeys[i].Bytes()
    }
    wrongNewAddr := testAccAddr() // NOT the multisig's derived address

    // No accountKeeper EXPECT() — the validation must reject before any
    // call into the keeper. Strict gomock will fail the test on any call.

    destProof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:  2,
        SubPubKeys: subPubs,
    }}}

    _, err := f.keeper.MigrateAuth(f.ctx, legacy, wrongNewAddr, destProof)
    require.Error(t, err)
    require.ErrorIs(t, err, types.ErrPubKeyAddressMismatch)
}

// TestMigrateAuth_PreExistingVestingDestination_Rejected asserts the type-safety
// check: migrations to a destination that is already a vesting account (any
// variant) are rejected, independent of pubkey or destProof shape. Otherwise
// FinalizeVestingAccount would silently extract the BaseAccount core and
// clobber the pre-existing vesting schedule.
func TestMigrateAuth_PreExistingVestingDestination_Rejected(t *testing.T) {
    f := initMockFixture(t)
    legacy := testAccAddr()
    newAddr := testAccAddr()

    // Pre-existing ContinuousVestingAccount at newAddr.
    baseAcc := authtypes.NewBaseAccountWithAddress(newAddr)
    origVesting := sdk.NewCoins(sdk.NewInt64Coin("ulume", 1000))
    bva, err := vestingtypes.NewBaseVestingAccount(baseAcc, origVesting, 2_000_000)
    require.NoError(t, err)
    existingVesting := vestingtypes.NewContinuousVestingAccountRaw(bva, 1_000_000)

    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingVesting)
    // No further EXPECT() calls — the type-safety check must reject before any
    // legacy-account lookup or state mutation.

    _, err = f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil) // single-key destProof=nil
    require.Error(t, err)
    require.ErrorContains(t, err, "non-BaseAccount")
    require.ErrorContains(t, err, "ContinuousVestingAccount")
}

// TestMigrateAuth_PreExistingModuleDestination_Rejected asserts that a module
// account cannot be used as a migration destination — regardless of side or
// destProof shape. Preserves parity with the existing legacy-side module-account
// rejection in preChecks.
func TestMigrateAuth_PreExistingModuleDestination_Rejected(t *testing.T) {
    f := initMockFixture(t)
    legacy := testAccAddr()
    newAddr := sdk.AccAddress(authtypes.NewModuleAddress("distribution"))

    // Build a real ModuleAccount at newAddr.
    moduleAcc := authtypes.NewEmptyModuleAccount("distribution", authtypes.Minter, authtypes.Burner)

    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(moduleAcc)
    // No further EXPECT() calls — the type-safety check must reject before any
    // legacy-account lookup or state mutation.

    _, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)
    require.Error(t, err)
    require.ErrorIs(t, err, types.ErrCannotMigrateModuleAccount)
}

// TestMigrateAuth_MultisigDestination_PreExistingMismatchedPubKey_Rejected asserts
// that if newAddr already has a BaseAccount with a *different* non-nil pubkey
// (e.g., someone funded the target address with their own EOA pre-migration),
// MigrateAuth refuses to silently overwrite it with the reconstructed multisig
// pubkey. This is the SDK 0.53.6 SetPubKey safety guard (review #12 Finding 1).
func TestMigrateAuth_MultisigDestination_PreExistingMismatchedPubKey_Rejected(t *testing.T) {
    f := initMockFixture(t)
    legacy := testAccAddr()

    // Build a real multisig so the address-derivation check passes.
    subPubs := make([][]byte, 3)
    subPubKeys := make([]cryptotypes.PubKey, 3)
    for i := 0; i < 3; i++ {
        p, err := ethsecp256k1.GenerateKey()
        require.NoError(t, err)
        subPubKeys[i] = p.PubKey()
        subPubs[i] = subPubKeys[i].Bytes()
    }
    multiPK := kmultisig.NewLegacyAminoPubKey(2, subPubKeys)
    newAddr := sdk.AccAddress(multiPK.Address())

    // Pre-existing account at newAddr with a *different* eth pubkey.
    otherPriv, err := ethsecp256k1.GenerateKey()
    require.NoError(t, err)
    existingAcc := authtypes.NewBaseAccountWithAddress(newAddr)
    require.NoError(t, existingAcc.SetPubKey(otherPriv.PubKey()))

    // Under the refactored MigrateAuth (review #14 Finding 1) the pubkey-match
    // check runs PRE-mutation, so only the newAddr probe is expected. GetAccount(legacy)
    // and RemoveAccount must NOT be called — otherwise we'd leave a partially
    // migrated state on rejection.
    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingAcc)
    // No GetAccount(legacy), no RemoveAccount, no NewAccountWithAddress, no SetAccount.

    destProof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:  2,
        SubPubKeys: subPubs,
    }}}

    _, err = f.keeper.MigrateAuth(f.ctx, legacy, newAddr, destProof)
    require.Error(t, err)
    require.ErrorIs(t, err, types.ErrPubKeyAddressMismatch)
    require.ErrorContains(t, err, "refusing to overwrite")
}

// TestMigrateAuth_MultisigDestination_PreExistingMatchingPubKey_Idempotent asserts
// that if newAddr already has a BaseAccount whose pubkey is byte-equal to the
// reconstructed multisig (e.g., idempotent re-run), MigrateAuth proceeds without
// error and does NOT call SetPubKey again (the pubkey is already correct).
func TestMigrateAuth_MultisigDestination_PreExistingMatchingPubKey_Idempotent(t *testing.T) {
    f := initMockFixture(t)
    legacy := testAccAddr()

    subPubs := make([][]byte, 3)
    subPubKeys := make([]cryptotypes.PubKey, 3)
    for i := 0; i < 3; i++ {
        p, err := ethsecp256k1.GenerateKey()
        require.NoError(t, err)
        subPubKeys[i] = p.PubKey()
        subPubs[i] = subPubKeys[i].Bytes()
    }
    multiPK := kmultisig.NewLegacyAminoPubKey(2, subPubKeys)
    newAddr := sdk.AccAddress(multiPK.Address())

    // Pre-existing account with the CORRECT multisig pubkey already set.
    existingAcc := authtypes.NewBaseAccountWithAddress(newAddr)
    require.NoError(t, existingAcc.SetPubKey(multiPK))

    baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
    // Under the single-GetAccount(newAddr) discipline, the probe is the only
    // newAddr fetch in the whole function — Phase 2 reuses the cached value.
    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(existingAcc)
    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(baseAcc)
    f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
    // No NewAccountWithAddress — existingAcc is reused.
    // No SetPubKey expectation — Phase-1 check C confirmed the pubkey already
    // matches, so Phase 2 skips the write (idempotent re-run).
    f.accountKeeper.EXPECT().
        SetAccount(gomock.Any(), gomock.AssignableToTypeOf(&authtypes.BaseAccount{})).
        Do(func(_ sdk.Context, acc sdk.AccountI) {
            require.Equal(t, multiPK.Address(), acc.GetPubKey().Address())
        })

    destProof := &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold:  2,
        SubPubKeys: subPubs,
    }}}

    _, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, destProof)
    require.NoError(t, err)
}

// TestMigrateAuth_MultisigDestination_MalformedDestProof asserts the ValidateBasic
// up-front check (review #10 Finding 1): a structurally malformed destProof — e.g.
// threshold=0, or a wrong-length sub-pubkey, or threshold > N — must be rejected
// before any accountKeeper call. Otherwise malformed input could reach
// NewLegacyAminoPubKey / SetPubKey and panic inside the crypto stack.
func TestMigrateAuth_MultisigDestination_MalformedDestProof(t *testing.T) {
    f := initMockFixture(t)
    legacy := testAccAddr()
    newAddr := testAccAddr()

    cases := []struct {
        name      string
        destProof *types.MigrationProof
    }{
        {
            name: "threshold=0",
            destProof: &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
                Threshold:     0, // invalid: 1 ≤ threshold ≤ N
                SubPubKeys:    [][]byte{make([]byte, 33), make([]byte, 33)},
                SignerIndices: []uint32{},
                SubSignatures: [][]byte{},
                SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
            }}},
        },
        {
            name: "threshold exceeds N",
            destProof: &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
                Threshold:     5, // invalid: exceeds len(SubPubKeys)=2
                SubPubKeys:    [][]byte{make([]byte, 33), make([]byte, 33)},
                SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
            }}},
        },
        {
            name: "sub_pub_key wrong length",
            destProof: &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
                Threshold:     1,
                SubPubKeys:    [][]byte{make([]byte, 32)}, // 32 bytes, must be 33
                SigFormat:     types.SigFormat_SIG_FORMAT_CLI,
            }}},
        },
        {
            name: "multisig with EIP191",
            destProof: &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
                Threshold:     1,
                SubPubKeys:    [][]byte{make([]byte, 33)},
                SigFormat:     types.SigFormat_SIG_FORMAT_EIP191, // rejected for multisig
            }}},
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // No accountKeeper EXPECT() — validation must reject before any keeper access.
            _, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, tc.destProof)
            require.Error(t, err, "malformed destProof must be rejected")
        })
    }
}

// TestMigrateAuth_SingleKeyDestination_NilPubKey is the regression lock: a
// single-key destProof (or a nil destProof) must leave the new BaseAccount's
// PubKey at nil, matching pre-pivot behavior and every other fresh account.
func TestMigrateAuth_SingleKeyDestination_NilPubKey(t *testing.T) {
    f := initMockFixture(t)
    legacy := testAccAddr()
    newAddr := testAccAddr()

    baseAcc := authtypes.NewBaseAccountWithAddress(legacy)
    freshNewAcc := authtypes.NewBaseAccountWithAddress(newAddr)

    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), legacy).Return(baseAcc)
    f.accountKeeper.EXPECT().RemoveAccount(gomock.Any(), baseAcc)
    f.accountKeeper.EXPECT().GetAccount(gomock.Any(), newAddr).Return(nil)
    f.accountKeeper.EXPECT().NewAccountWithAddress(gomock.Any(), newAddr).Return(freshNewAcc)
    f.accountKeeper.EXPECT().
        SetAccount(gomock.Any(), gomock.AssignableToTypeOf(&authtypes.BaseAccount{})).
        Do(func(_ sdk.Context, acc sdk.AccountI) {
            require.Nil(t, acc.GetPubKey(),
                "single-key destination must have nil BaseAccount.PubKey to match fresh-EVM-account behavior")
        })

    singleProof := &types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
        PubKey: make([]byte, 33),
    }}}

    _, err := f.keeper.MigrateAuth(f.ctx, legacy, newAddr, singleProof)
    require.NoError(t, err)
}
```

- [ ] **Step 1b: Update every existing `MigrateAuth` caller to pass `nil` as the new `destProof` arg**

The arity change to `MigrateAuth(ctx, legacyAddr, newAddr, destProof *types.MigrationProof)` breaks every existing caller. Locate them:

Run: `grep -n "\.MigrateAuth(" x/evmigration/`
Expected callers (in addition to `migrateAccount` helper updated in Step 4): the existing tests at [migrate_test.go:163, :188, :216, :246, :271, :286, :299, :318](x/evmigration/keeper/migrate_test.go#L163) — eight sites that call `f.keeper.MigrateAuth(f.ctx, legacy, newAddr)`. Update each to `f.keeper.MigrateAuth(f.ctx, legacy, newAddr, nil)`.

`nil` is semantically correct for all existing tests: they predate this refactor and test the legacy "no destination-shape persistence" behavior, which the new code preserves when `destProof == nil` or `destProof.GetMultisig() == nil`.

- [ ] **Step 2: Run — expect compile failure on `MigrateAuth`'s new arity**

Run: `go test ./x/evmigration/keeper/ -run TestMigrateAuth -v -count=1`

- [ ] **Step 3: Extend `MigrateAuth` signature**

In `migrate_auth.go`:

```go
func (k Keeper) MigrateAuth(
    ctx context.Context,
    legacyAddr, newAddr sdk.AccAddress,
    destProof *types.MigrationProof,
) (*VestingInfo, error) {
    // -------------------------------------------------------------------------
    // PHASE 1 — ALL PRE-MUTATION CHECKS. No state is written until they pass.
    // Any rejection here leaves the chain state untouched (no partial migration).
    // -------------------------------------------------------------------------

    // Phase-1 check A: stateless proof validation. Cheap; do it first so
    // malformed input doesn't trigger any state reads.
    if destProof != nil {
        if err := destProof.ValidateBasic(types.SideNew); err != nil {
            return nil, err
        }
    }

    // Phase-1 probe: fetch the pre-existing account at newAddr ONCE and cache
    // it. The single GetAccount(newAddr) call is reused during materialization
    // in Phase 2; since legacy removal doesn't affect newAddr, the probe value
    // stays accurate across the mutation boundary.
    existingNewAcc := k.accountKeeper.GetAccount(ctx, newAddr)

    // Phase-1 check B: destination-account type safety (for both single-key
    // AND multisig destinations). See rationale below for why BaseAccount-only.
    if existingNewAcc != nil {
        if _, ok := existingNewAcc.(sdk.ModuleAccountI); ok {
            return nil, types.ErrCannotMigrateModuleAccount.Wrapf(
                "destination %s is a module account; cannot migrate to a module address",
                newAddr,
            )
        }
        if _, ok := existingNewAcc.(*authtypes.BaseAccount); !ok {
            // Covers vesting accounts (Continuous/Delayed/Periodic/PermanentLocked),
            // any future smart-account / contract-account type, and any third-party
            // wrapper type the module hasn't been taught about.
            return nil, types.ErrPubKeyAddressMismatch.Wrapf(
                "destination %s has non-BaseAccount type %T; migration to existing special accounts (vesting, module, etc.) is not supported — choose a fresh destination",
                newAddr, existingNewAcc,
            )
        }
    }

    // Phase-1 check C: multisig-specific reconstruction, address binding, AND
    // pubkey-compatibility on any pre-existing BaseAccount. Each of these runs
    // BEFORE any state mutation so a mismatch cannot leave the chain in a
    // partially-migrated state. (Previously the pubkey-compat check ran after
    // RemoveAccount — review #14 Finding 1.)
    var destMultiPK cryptotypes.PubKey
    if destProof != nil {
        if ms := destProof.GetMultisig(); ms != nil {
            subKeys := make([]cryptotypes.PubKey, len(ms.SubPubKeys))
            for i, raw := range ms.SubPubKeys {
                subKeys[i] = &ethsecp256k1.PubKey{Key: raw}
            }
            multiPK := kmultisig.NewLegacyAminoPubKey(int(ms.Threshold), subKeys)
            if !sdk.AccAddress(multiPK.Address()).Equals(newAddr) {
                return nil, types.ErrPubKeyAddressMismatch.Wrapf(
                    "destination multisig pubkey derives to %s, expected %s",
                    sdk.AccAddress(multiPK.Address()), newAddr,
                )
            }
            // Pubkey-compatibility on cached pre-existing account: if it
            // already has a pubkey, it must match. SDK 0.53.6's
            // BaseAccount.SetPubKey is an unconditional overwrite, so without
            // this check we'd silently replace a different legitimate pubkey
            // during Phase 2. Rejecting PRE-mutation is the important part —
            // if this fires after legacy removal, we'd leave the chain
            // half-migrated.
            if existingNewAcc != nil {
                if existingPK := existingNewAcc.GetPubKey(); existingPK != nil {
                    if !bytes.Equal(existingPK.Bytes(), multiPK.Bytes()) {
                        return nil, types.ErrPubKeyAddressMismatch.Wrapf(
                            "destination account %s already has a different pubkey; refusing to overwrite with reconstructed multisig",
                            newAddr,
                        )
                    }
                    // existingPK == multiPK → idempotent re-run case.
                    // destMultiPK will still be set below; the Phase-2
                    // SetPubKey call gates on "pubkey still nil" so this
                    // idempotency is free.
                }
            }
            destMultiPK = multiPK
        }
    }

    // -------------------------------------------------------------------------
    // PHASE 2 — STATE MUTATION. All pre-mutation checks have passed.
    // -------------------------------------------------------------------------

    // ... existing vesting-capture + legacy-account-removal logic unchanged:
    //     - GetAccount(ctx, legacyAddr) → legacyAcc (or ErrLegacyAccountNotFound)
    //     - reject module-account legacy (ErrCannotMigrateModuleAccount)
    //     - capture *VestingInfo if legacyAcc is a vesting variant
    //     - RemoveAccount(ctx, legacyAcc) — state mutation begins here

    // Materialize newAcc. Reuse the cached probe from Phase 1 — legacy removal
    // does not touch newAddr, so existingNewAcc is still accurate. This keeps
    // GetAccount(ctx, newAddr) to exactly ONE call in the whole function.
    var newAcc sdk.AccountI
    if existingNewAcc != nil {
        newAcc = existingNewAcc
    } else {
        newAcc = k.accountKeeper.NewAccountWithAddress(ctx, newAddr)
    }

    // Apply multisig pubkey. Phase-1 check C already confirmed that if newAcc
    // has a pubkey, it byte-equals destMultiPK. So the only case that needs
    // a SetPubKey call is when the existing pubkey slot is nil (fresh account
    // OR funded-but-never-signed). The else branch (existing == destMultiPK)
    // is the idempotent re-run case and requires no write.
    if destMultiPK != nil && newAcc.GetPubKey() == nil {
        if err := newAcc.SetPubKey(destMultiPK); err != nil {
            return nil, err
        }
    }

    k.accountKeeper.SetAccount(ctx, newAcc)
    return vi, nil
}
```

**Why "fresh or plain BaseAccount only":** the existing `FinalizeVestingAccount` at [migrate_auth.go:95](x/evmigration/keeper/migrate_auth.go#L95) currently handles a non-BaseAccount destination by extracting the BaseAccount core (preserving pubkey, account number, sequence) and rebuilding the destination as a new vesting account with the *legacy's* vesting parameters. That's a silent clobber of any special-type state pre-existing at `newAddr`: a continuous-vesting destination would lose its schedule; a module account would be overwritten (and shouldn't even be a migration target); a future smart-account type would lose its state. Rather than encode per-type clobber/preserve semantics, the simplest correct rule is "migration destinations must be fresh or plain BaseAccount." Users who want to migrate into a special-type address must first convert it (or pick a different destination).

Note: the single-key destination path never calls `SetPubKey` (the ante handler populates the pubkey on the user's first signed tx, which performs its own match check). But the **type-safety check DOES apply to single-key destinations too** — even without `SetPubKey`, the vesting-finalize path can clobber a pre-existing vesting account, and module accounts should never be migration destinations regardless of shape.

- [ ] **Step 4: Extend `migrateAccount` helper signature**

In `msg_server_claim_legacy.go`, find the private helper (around the existing `func (ms msgServer) migrateAccount(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress) error`). Change it to:

```go
func (ms msgServer) migrateAccount(
    ctx sdk.Context,
    legacyAddr, newAddr sdk.AccAddress,
    destProof *types.MigrationProof,
) error {
    // ... existing steps 1-8 unchanged ...

    // Step 3a: Migrate auth account (now receives destProof so multisig destinations get their pubkey set).
    vestingInfo, err := ms.MigrateAuth(ctx, legacyAddr, newAddr, destProof)
    if err != nil {
        return fmt.Errorf("migrate auth: %w", err)
    }

    // ... remaining steps unchanged ...
}
```

- [ ] **Step 5: Update `migrateAccount` callers**

Both `ClaimLegacyAccount` (in `msg_server_claim_legacy.go`) and `MigrateValidator` (in `msg_server_migrate_validator.go` — if it uses a similar helper; if it inlines the logic, update the inlined call site instead) must pass `&msg.NewProof`:

Run: `grep -n "migrateAccount\|MigrateAuth(" x/evmigration/keeper/msg_server_*.go`

Update each call site to pass `&msg.NewProof` as the new last argument. If `msg_server_migrate_validator.go` calls `MigrateAuth` directly (no `migrateAccount` helper), update that call too.

- [ ] **Step 6: Run tests**

Run: `go test ./x/evmigration/keeper/ -run "TestMigrateAuth\|TestClaimLegacy\|TestMigrateValidator" -v -count=1 2>&1 | tail -30`
Expected: all pass — both new `TestMigrateAuth_*` cases and the existing end-to-end tests.

- [ ] **Step 7: Commit**

```bash
make lint
git add x/evmigration/keeper/migrate_auth.go x/evmigration/keeper/migrate_test.go x/evmigration/keeper/msg_server_claim_legacy.go x/evmigration/keeper/msg_server_migrate_validator.go
git commit -m "$(cat <<'EOF'
evmigration(migrate): persist multisig pubkey on destination BaseAccount

MigrateAuth now takes destProof *types.MigrationProof and, when the
destination side is multisig, calls acc.SetPubKey(multiPK) before
SetAccount. This makes the K-of-N shape visible on-chain immediately
(avoids the nil-pubkey "sign any tx first" footgun). Single-key
destinations continue to use nil-pubkey BaseAccount.

Also threads destProof through the msg-server-private migrateAccount
helper (signature change) and both msg-server call sites.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5 — CLI Rewrite

**Context for Phase 5:** the CLI has two one-shot commands (`claim-legacy-account`, `migrate-validator`) and four multi-step commands (`generate-proof-payload`, `sign-proof`, `combine-proof`, `submit-proof`). All of them must build `MigrationProof{Single}` or `MigrationProof{Multisig}` on **both** sides.

**Migration txs are intentionally unsigned at the Cosmos tx layer — chicken-and-egg requirement.** The new EVM address does not yet exist as an on-chain account at the moment a migration tx is submitted (it's materialized *by* the migration), so **no account is available to sign the Cosmos envelope**. Trying to sign with the new key would fail because the account has no `account_number` / `sequence` yet; trying to sign with the legacy key would defeat the point of the migration (the user shouldn't need the legacy key on the chain side — it's already proved control via `legacy_proof`). The design resolves the chicken-and-egg by declaring **zero signers** at the proto level:

- `MsgClaimLegacyAccount` and `MsgMigrateValidator` have **no `cosmos.msg.v1.signer` option** — `GetSigners()` returns empty.
- Authorization is fully embedded in the `legacy_proof` and `new_proof` fields (see the verifier in Task 7).
- The evmigration ante handler waives fees so no fee-payer account is needed.
- Replay protection comes from `MigrationRecords.Has(legacyAddr)` in `preChecks`, not from a sequence number.

Adding a signer to these txs produces the validation error "expected 0, got 1"; see the explanatory comment at [tx.go:260-262](x/evmigration/client/cli/tx.go#L260-L262). `submit-proof` therefore does **not** sign an outer envelope, does **not** take a `--from` broadcaster key, and does **not** use the SDK's generic gas estimator (which would inject a simulated signer).

### Task 13: Retire `signNewMigrationProof` + `MigrationSetNewProof`; route new side through `MigrationProof{Single}`

**Files:**
- Modify: `x/evmigration/client/cli/tx.go` — `migrationProofMsg` interface, `runMigrationTx`, one-shot commands.
- Modify: `x/evmigration/types/types.go` — remove `MigrationSetNewProof(signature []byte)` methods (they write the removed `NewSignature` field).
- Modify: `x/evmigration/client/cli/tx_test.go` — update any test asserting on the old setter.

**Why both files need edits together:** the existing contract between the interface and the type methods is "caller produces `[]byte` signature, calls `msg.MigrationSetNewProof([]byte)`, type writes to `msg.NewSignature`". After the proto refactor (Task 2), `NewSignature` no longer exists and `MigrationSetNewProof([]byte)` has nowhere to store its input. The new contract is "caller produces `types.MigrationProof` via `buildNewSingleProof` and sets `msg.NewProof = …` directly — no interface method needed."

- [ ] **Step 1: Find and remove `signNewMigrationProof` and `MigrationSetNewProof`**

Run: `grep -rn "signNewMigrationProof\|MigrationSetNewProof" x/evmigration/`
Expected hits: the two type methods in [types/types.go:48-50, :78-80](x/evmigration/types/types.go#L48-L50) (one per message), the interface member in [tx.go:221](x/evmigration/client/cli/tx.go#L221), the single caller in [tx.go:234](x/evmigration/client/cli/tx.go#L234), and `signNewMigrationProof` itself at [tx.go:355](x/evmigration/client/cli/tx.go#L355). All must go.

- [ ] **Step 2: Add the new-side proof builder**

Replace the old `signNewMigrationProof([]byte → sig)` with a new-side proof producer:

```go
func buildNewSingleProof(clientCtx client.Context, newKeyName, proofKind, legacyAddress, newAddress string) (types.MigrationProof, error) {
    payload := []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
        clientCtx.ChainID, lcfg.EVMChainID, proofKind, legacyAddress, newAddress))
    sig, pubKey, err := clientCtx.Keyring.Sign(newKeyName, payload, signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON)
    if err != nil {
        return types.MigrationProof{}, err
    }
    ethPK, ok := pubKey.(*evmcryptotypes.PubKey)
    if !ok {
        return types.MigrationProof{}, fmt.Errorf("key %q must use eth_secp256k1, got %T", newKeyName, pubKey)
    }
    return types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
        PubKey:    ethPK.Key,
        Signature: sig,
        SigFormat: types.SigFormat_SIG_FORMAT_CLI,
    }}}, nil
}
```

- [ ] **Step 3: Simplify the `migrationProofMsg` interface and `runMigrationTx`**

In [tx.go:217-222](x/evmigration/client/cli/tx.go#L217-L222), the interface is:

```go
type migrationProofMsg interface {
    sdk.Msg
    MigrationNewAddress() string
    MigrationLegacyAddress() string
    MigrationSetNewProof(signature []byte) // remove
}
```

Drop the `MigrationSetNewProof` member — the interface now exists only to give `runMigrationTx` and `simulateMigrationGas` a generic way to read `legacy_address` / `new_address` for logging and the simulation builder. No runtime proof mutation happens inside `runMigrationTx` anymore.

Then update `runMigrationTx` to stop signing and setting the new proof (the caller has already set it):

```go
func runMigrationTx(cmd *cobra.Command, msg migrationProofMsg) error {
    clientCtx, err := client.GetClientTxContext(cmd)
    if err != nil {
        return err
    }

    // The caller (one-shot claim/migrate-validator OR combine-proof) has fully assembled
    // legacy_proof and new_proof on the message. runMigrationTx no longer derives, sets,
    // or signs proof material — and the message itself declares zero signers, so the
    // outer Cosmos tx envelope is also unsigned. This function just runs ValidateBasic,
    // simulates gas via the migration-specific estimator (which does NOT inject a signer),
    // builds the unsigned tx, and broadcasts.
    if validateBasic, ok := msg.(sdk.HasValidateBasic); ok {
        if err := validateBasic.ValidateBasic(); err != nil {
            return err
        }
    }

    txf, err := clienttx.NewFactoryCLI(clientCtx, cmd.Flags())
    if err != nil {
        return err
    }
    // ... rest of simulate (via simulateMigrationGas, preserved) + BuildUnsignedTx +
    // confirm + broadcast — unchanged from today's behavior at tx.go:263-295 ...
    return nil
}
```

Both the `proofKind` and `broadcasterKeyName` parameters are removed from the signature (they were only needed for `signNewMigrationProof` and the envelope-signing path that does not exist). Update the three call sites at [tx.go:82, tx.go:116, tx_multisig.go:742](x/evmigration/client/cli/tx.go#L82) to drop those args. The kind is still carried on the message itself (`@type`), so nothing is lost.

- [ ] **Step 4: Update `resolveClaimMsg` and `resolveValidatorMsg`**

The existing helpers in `tx.go` build `MsgClaimLegacyAccount` / `MsgMigrateValidator` with only the legacy half populated; `MigrationSetNewProof` was setting the new half later. Rework them to set **both** halves up-front:

```go
func resolveClaimMsg(cmd *cobra.Command, legacyKeyName, newKeyName string) (*types.MsgClaimLegacyAccount, string, error) {
    clientCtx, err := client.GetClientTxContext(cmd)
    if err != nil { return nil, "", err }

    // Build legacy-side single-key proof (existing signLegacyProofFromKeyring path).
    newAddr, legacyAddr, legacyPubKey, legacySig, err := signLegacyProofFromKeyring(clientCtx, legacyKeyName, newKeyName, migrationProofKindClaim)
    if err != nil { return nil, "", err }

    // Build new-side single-key proof up-front.
    newProof, err := buildNewSingleProof(clientCtx, newKeyName, migrationProofKindClaim, legacyAddr, newAddr)
    if err != nil { return nil, "", err }

    msg := &types.MsgClaimLegacyAccount{
        LegacyAddress: legacyAddr,
        NewAddress:    newAddr,
        LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
            PubKey:    legacyPubKey,
            Signature: legacySig,
            SigFormat: types.SigFormat_SIG_FORMAT_CLI,
        }}},
        NewProof: newProof,
    }
    return msg, newKeyName, nil
}
```

Mirror for `resolveValidatorMsg` (same shape, `MigrationProof`/payload kind swapped).

- [ ] **Step 5: Delete `MigrationSetNewProof` methods from `types/types.go`**

At [types/types.go:48-50](x/evmigration/types/types.go#L48-L50) and [types/types.go:78-80](x/evmigration/types/types.go#L78-L80), delete both methods. Keep `MigrationNewAddress()` and `MigrationLegacyAddress()` — the interface still uses those.

- [ ] **Step 6: Build + test**

Run: `go build ./x/evmigration/...`
Run: `go test ./x/evmigration/client/cli/... -v -count=1 2>&1 | tail -30`

Fix any remaining tests that grep for `"new_signature"`, call `MigrationSetNewProof`, or assert on the old interface shape.

- [ ] **Step 7: Commit**

```bash
make lint
git add x/evmigration/client/cli/tx.go x/evmigration/client/cli/tx_test.go x/evmigration/types/types.go
git commit -m "$(cat <<'EOF'
evmigration(cli): one-shot commands build both halves of MigrationProof directly

- Remove MigrationSetNewProof from migrationProofMsg interface and from the
  two message types in types.go.
- Drop the post-tx new-signature mutation in runMigrationTx; callers now
  construct MsgClaimLegacyAccount / MsgMigrateValidator with NewProof
  already populated via the new buildNewSingleProof helper.
- Delete signNewMigrationProof.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 14: `generate-proof-payload` accepts new-side pubkey material

**Files:**
- Modify: `x/evmigration/client/cli/tx_multisig.go`
- Modify: `x/evmigration/client/cli/tx_multisig_test.go` — the existing tests reference types this task removes/renames: `PartialSingle`, `PartialMultisig`, `PartialSubSignature`, and the flat `PartialSigs` slice (see [tx_multisig_test.go:18-62](x/evmigration/client/cli/tx_multisig_test.go#L18-L62)). Without updating this file in the same commit, `go build ./x/evmigration/client/cli/...` fails. A later step in this task migrates those tests to the new `Legacy`/`New` + `PartialLegacySignatures`/`PartialNewSignatures` shape.

- [ ] **Step 1: Identify the command definition**

Run: `grep -n "generate-proof-payload\|cmdGenerateProofPayload\|func.*GenerateProof" x/evmigration/client/cli/tx_multisig.go`

- [ ] **Step 2: Extend with new-side flags**

**Existing flags preserved** (see [tx_multisig.go:32-37](x/evmigration/client/cli/tx_multisig.go#L32-L37)):
- `--legacy <bech32>` — legacy (coin-type 118) source address. Still required.
- `--new <bech32>` — destination address. **Changed semantics:** no longer required; when `--new-key` or `--new-sub-pub-keys` is supplied, the new address is derived from the key material and `--new` becomes optional (used only as a cross-check per Finding 1). **Remove the `_ = cmd.MarkFlagRequired(flagNewAddr)` line at [tx_multisig.go:496](x/evmigration/client/cli/tx_multisig.go#L496)** — otherwise cobra will reject invocations that omit `--new` even when the derived-address workflow supplies it via `--new-key`.
- `--kind claim|validator`, `--out`, `--legacy-key`, `--sig-format`.

**Update `ParseSigFormat` and `SigFormatString` to handle `SIG_FORMAT_EIP191`.** The existing parser at [tx_multisig.go:105-127](x/evmigration/client/cli/tx_multisig.go#L105-L127) only knows about `CLI` and `ADR036`; if we leave it as-is, `validatePartialProof` (Step 3b) and `buildProofFromPartial` (Task 16) will reject every new-side single-key partial whose `sig_format` is `SIG_FORMAT_EIP191` with a cryptic "unknown sig_format" error. Update both functions:

```go
// ParseSigFormat converts the JSON string to a proto enum.
func ParseSigFormat(s string) (types.SigFormat, error) {
    switch s {
    case "SIG_FORMAT_CLI":
        return types.SigFormat_SIG_FORMAT_CLI, nil
    case "SIG_FORMAT_ADR036":
        return types.SigFormat_SIG_FORMAT_ADR036, nil
    case "SIG_FORMAT_EIP191":
        return types.SigFormat_SIG_FORMAT_EIP191, nil
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
    case types.SigFormat_SIG_FORMAT_EIP191:
        return "SIG_FORMAT_EIP191"
    default:
        return "SIG_FORMAT_UNSPECIFIED"
    }
}
```

(Note: this is a CLI-layer helper. The keeper's verifier dispatches on the enum directly via `sigverify.Verify{Cosmos,Eth}Secp256k1`, so no keeper-side parsing change is needed. The scope-validation `validateSideSpec` in Step 3b continues to reject EIP-191 on the legacy side and on multisig — that check happens AFTER `ParseSigFormat` succeeds.)

**New flags added in this task:**

```go
cmd.Flags().String(flagNewKey, "", "Keyring name of the destination-side single-key (must be eth_secp256k1). Mutually exclusive with --new-sub-pub-keys.")
cmd.Flags().StringSlice(flagNewSubPubKeys, nil, "Comma-separated list of destination-side sub-keys. Each entry is either a keyring key name or a base64-encoded 33-byte eth_secp256k1 pubkey.")
cmd.Flags().Uint32(flagNewThreshold, 0, "Threshold K for the destination-side multisig. Required with --new-sub-pub-keys.")
```

Constants near the other flag constants:

```go
flagNewKey         = "new-key"
flagNewSubPubKeys  = "new-sub-pub-keys"
flagNewThreshold   = "new-threshold"
```

Validation rules to add in `RunE`:
- Reject if both `--new-key` and `--new-sub-pub-keys` are supplied (mutually exclusive).
- Reject if neither is supplied (mirror-source rule requires explicit choice).
- Reject if `--new-sub-pub-keys` is supplied but legacy account is single-key (must mirror source shape).
- Reject if `--new-key` is supplied but legacy account is multisig.
- For multisig new side, compute `newAddr = sdk.AccAddress(kmultisig.NewLegacyAminoPubKey(K, ethSubKeys).Address())` and cross-check against the `--new` flag value.
- Reject if any new sub-pubkey matches any legacy sub-pubkey (wrong key reused).

- [ ] **Step 3: Extend the `PartialProof` struct**

In `tx_multisig.go` (or wherever `PartialProof` lives), add:

```go
type PartialProof struct {
    Version     int    `json:"version"`
    Kind        string `json:"kind"`
    LegacyAddress string `json:"legacy_address"` // match existing field name in tx_multisig.go:404
    NewAddress    string `json:"new_address"`
    ChainID     string `json:"chain_id"`
    EVMChainID  uint64 `json:"evm_chain_id"`
    PayloadHex  string `json:"payload_hex"`

    Legacy *SideSpec `json:"legacy,omitempty"`
    New    *SideSpec `json:"new,omitempty"`

    PartialLegacySignatures []PartialSignature `json:"partial_legacy_signatures"`
    PartialNewSignatures    []PartialSignature `json:"partial_new_signatures"`
}

type SideSpec struct {
    // For single-key: PubKey set; Threshold and SubPubKeys empty.
    // For multisig:  Threshold and SubPubKeys set; PubKey empty.
    PubKey      string   `json:"pub_key,omitempty"`
    Threshold   uint32   `json:"threshold,omitempty"`
    SubPubKeys  []string `json:"sub_pub_keys,omitempty"`
    SigFormat   string   `json:"sig_format"`
}

type PartialSignature struct {
    Index     uint32 `json:"index"`
    Signature string `json:"signature"`
}
```

**Bump `partialProofVersion` from `1` to `2`** at its declaration in `tx_multisig.go`. The on-disk layout changes incompatibly (top-level `Single`/`Multisig` → `Legacy`/`New` + split signature arrays); old v1 files must fail loudly with "unsupported partial_proof version 1 (expected 2)" rather than silently parse as v2 and error with confusing side-spec messages. `validatePartialProof` already rejects mismatched versions — bumping the constant is sufficient.

- [ ] **Step 3b: Update the existing helpers that validated the old `Single/Multisig + PartialSigs` schema**

The old `PartialProof` had `Single *SingleSpec`, `Multisig *MultisigSpec`, and one flat `PartialSigs` slice. Four existing helpers in `tx_multisig.go` know about that shape and must be rewritten against the new `Legacy/New + PartialLegacySignatures/PartialNewSignatures` shape. Locate them:

Run: `grep -n "validatePartialProof\|canonicalPayloadBytes\|AssertPartialProofsConsistent\|verifyPartialSignature" x/evmigration/client/cli/tx_multisig.go`

Expected hits (line numbers will shift after edits — these reference the current state):

- `canonicalPayloadBytes` at [tx_multisig.go:151](x/evmigration/client/cli/tx_multisig.go#L151) — **unchanged structurally**, still reads `pp.ChainID`, `pp.EVMChainID`, `pp.Kind`, `pp.LegacyAddress`, `pp.NewAddress`. But verify that renames from Finding 1 (keeping `LegacyAddress`/`NewAddress`) preserve these accesses.

- `validatePartialProof` at [tx_multisig.go:155](x/evmigration/client/cli/tx_multisig.go#L155) — rewrite:

    ```go
    func validatePartialProof(pp *PartialProof) error {
        if pp.Version != partialProofVersion {
            return fmt.Errorf("unsupported partial_proof version %d (expected %d)", pp.Version, partialProofVersion)
        }
        if pp.Kind != migrationProofKindClaim && pp.Kind != migrationProofKindValidator {
            return fmt.Errorf("partial proof has invalid kind %q (expected %q or %q)",
                pp.Kind, migrationProofKindClaim, migrationProofKindValidator)
        }
        if pp.Legacy == nil {
            return fmt.Errorf("partial proof missing 'legacy' side spec")
        }
        if pp.New == nil {
            return fmt.Errorf("partial proof missing 'new' side spec")
        }
        if err := validateSideSpec("legacy", pp.Legacy); err != nil {
            return err
        }
        if err := validateSideSpec("new", pp.New); err != nil {
            return err
        }
        payloadBytes, err := hex.DecodeString(pp.PayloadHex)
        if err != nil {
            return fmt.Errorf("payload_hex: %w", err)
        }
        if !bytes.Equal(payloadBytes, canonicalPayloadBytes(pp)) {
            return fmt.Errorf("payload_hex does not match chain_id/kind/legacy_address/new_address fields")
        }
        return nil
    }

    // validateSideSpec enforces the "either single or multisig, not both / not neither"
    // rule per side, plus the SigFormat constraints the design places on each shape:
    //   - EIP-191 is only valid on single-key new-side proofs (rejected on legacy side
    //     and rejected on multisig on both sides — see design §4.1).
    //   - ADR-036 and CLI formats are valid on both sides and both shapes.
    func validateSideSpec(label string, s *SideSpec) error {
        isSingle := s.PubKey != ""
        isMulti  := s.Threshold > 0 || len(s.SubPubKeys) > 0
        switch {
        case !isSingle && !isMulti:
            return fmt.Errorf("%s side: neither pub_key nor sub_pub_keys set", label)
        case isSingle && isMulti:
            return fmt.Errorf("%s side: both single-key (pub_key) and multisig (threshold/sub_pub_keys) fields are set", label)
        case isMulti && s.Threshold == 0:
            return fmt.Errorf("%s side: multisig has threshold=0", label)
        case isMulti && int(s.Threshold) > len(s.SubPubKeys):
            return fmt.Errorf("%s side: threshold=%d exceeds sub_pub_keys count=%d", label, s.Threshold, len(s.SubPubKeys))
        }
        if s.SigFormat == "" {
            return fmt.Errorf("%s side: sig_format empty", label)
        }
        parsed, err := ParseSigFormat(s.SigFormat)
        if err != nil {
            return fmt.Errorf("%s side: sig_format %q: %w", label, s.SigFormat, err)
        }
        // SIG_FORMAT_EIP191 is only valid for single-key NEW-side proofs. Reject it:
        //   - on legacy side (Cosmos secp256k1 keys never produce EIP-191 sigs)
        //   - on multisig (no wallet implements multisig EIP-191; the verifier
        //     rejects this shape per design §4.1).
        // Catching it here prevents sign-proof from producing a partial that would
        // only fail later during submit-proof's ValidateBasic.
        if parsed == types.SigFormat_SIG_FORMAT_EIP191 {
            if label == "legacy" {
                return fmt.Errorf("%s side: SIG_FORMAT_EIP191 is not valid on the legacy side", label)
            }
            if isMulti {
                return fmt.Errorf("%s side: SIG_FORMAT_EIP191 is not valid for multisig proofs", label)
            }
        }
        return nil
    }
    ```

- `AssertPartialProofsConsistent` at [tx_multisig.go:288](x/evmigration/client/cli/tx_multisig.go#L288) — extend to check BOTH sides of two partial files agree on structure. Full body (no elided pre-existing block):

    ```go
    func AssertPartialProofsConsistent(a, b *PartialProof) error {
        if a.Version != b.Version {
            return fmt.Errorf("version differs: %d vs %d", a.Version, b.Version)
        }
        if a.Kind != b.Kind {
            return fmt.Errorf("kind differs: %q vs %q", a.Kind, b.Kind)
        }
        if a.ChainID != b.ChainID {
            return fmt.Errorf("chain_id differs: %q vs %q", a.ChainID, b.ChainID)
        }
        if a.EVMChainID != b.EVMChainID {
            return fmt.Errorf("evm_chain_id differs: %d vs %d", a.EVMChainID, b.EVMChainID)
        }
        if a.LegacyAddress != b.LegacyAddress {
            return fmt.Errorf("legacy_address differs: %q vs %q", a.LegacyAddress, b.LegacyAddress)
        }
        if a.NewAddress != b.NewAddress {
            return fmt.Errorf("new_address differs: %q vs %q", a.NewAddress, b.NewAddress)
        }
        if a.PayloadHex != b.PayloadHex {
            return fmt.Errorf("payload_hex differs (chain_id/kind/legacy_address/new_address mismatch between files)")
        }
        if err := assertSideSpecsEqual("legacy", a.Legacy, b.Legacy); err != nil {
            return err
        }
        if err := assertSideSpecsEqual("new", a.New, b.New); err != nil {
            return err
        }
        return nil
    }

    func assertSideSpecsEqual(label string, a, b *SideSpec) error {
        if (a == nil) != (b == nil) {
            return fmt.Errorf("%s side spec presence differs between partial files", label)
        }
        if a == nil {
            return nil
        }
        if a.PubKey != b.PubKey {
            return fmt.Errorf("%s side pub_key differs", label)
        }
        if a.Threshold != b.Threshold {
            return fmt.Errorf("%s side threshold differs: %d vs %d", label, a.Threshold, b.Threshold)
        }
        if !slicesEqualString(a.SubPubKeys, b.SubPubKeys) {
            return fmt.Errorf("%s side sub_pub_keys differ", label)
        }
        if a.SigFormat != b.SigFormat {
            return fmt.Errorf("%s side sig_format differs: %q vs %q", label, a.SigFormat, b.SigFormat)
        }
        return nil
    }
    ```

- `verifyPartialSignature` at [tx_multisig.go:179](x/evmigration/client/cli/tx_multisig.go#L179) — **delete**. It only knew how to verify a Cosmos secp256k1 sub-signature. Its callers now go through `sigverify.VerifyCosmosSecp256k1` and `sigverify.VerifyEthSecp256k1` (Task 16's `verifyOne` helper). Remove `verifyPartialSignature` and its callsites; the shared-package versions fully subsume its behavior.

Add/ensure a `slicesEqualString(a, b []string) bool` utility exists (it's one-liner; define at the bottom of `tx_multisig.go` if not already present):

```go
func slicesEqualString(a, b []string) bool {
    if len(a) != len(b) {
        return false
    }
    for i := range a {
        if a[i] != b[i] {
            return false
        }
    }
    return true
}
```

- [ ] **Step 3c: Two-pass loader — version check first, then strict v2 decode**

Design §7 commits to rejecting v1 files with a clear "version mismatch" error. A single-pass `json.Decoder.DisallowUnknownFields` would fire on `single`/`partial_sigs` (v1's top-level fields) BEFORE the loader sees `version`, producing a cryptic "unknown field" error instead of the user-friendly version-mismatch message. Use a **two-pass decode**: first pass reads just `version` with a tolerant decoder and rejects unsupported versions; second pass strict-decodes the full v2 shape. This gives v1 files the clean error the design promises while keeping v2 strictness for drift detection.

```go
import "bytes"

// LoadPartialProof reads a PartialProof JSON file with a two-pass decode:
//   Pass 1: tolerant decode of just the version field. Reject if the file
//           is v1 (shipped briefly on branch `evm`) or any unsupported
//           version with a clear "unsupported partial_proof version N
//           (expected M)" error. This is the error v1-era users see.
//   Pass 2: strict decode of the full v2 shape with DisallowUnknownFields.
//           This catches future-forward drift (fields added by a newer
//           lumerad not known to this binary) with "unknown field" errors.
// Then validatePartialProof runs structural checks on the parsed result.
func LoadPartialProof(path string) (*PartialProof, error) {
    b, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read %s: %w", path, err)
    }

    // Pass 1: version probe. Use ordinary json.Unmarshal here so unknown
    // fields don't interfere with the version read.
    var probe struct {
        Version int `json:"version"`
    }
    if err := json.Unmarshal(b, &probe); err != nil {
        return nil, fmt.Errorf("parse %s: %w", path, err)
    }
    if probe.Version != partialProofVersion {
        return nil, fmt.Errorf("unsupported partial_proof version %d (expected %d)", probe.Version, partialProofVersion)
    }

    // Pass 2: strict decode now that version is confirmed. Unknown fields
    // at this point indicate drift within the v2 lineage — reject with the
    // stdlib's "unknown field" error so a future-forward lumerad's output
    // doesn't get silently truncated on load.
    dec := json.NewDecoder(bytes.NewReader(b))
    dec.DisallowUnknownFields()
    var pp PartialProof
    if err := dec.Decode(&pp); err != nil {
        return nil, fmt.Errorf("parse %s: %w", path, err)
    }
    if err := validatePartialProof(&pp); err != nil {
        return nil, err
    }
    return &pp, nil
}
```

Two regression tests:

```go
func TestLoadPartialProof_V1File_VersionMismatchError(t *testing.T) {
    // Exercises the happy path for the v1 deprecation UX: a v1-shape file
    // produces "unsupported partial_proof version 1 (expected 2)", NOT
    // "unknown field 'single'". Without pass 1, v1 files would fail with
    // the unknown-field error (technically correct but confusing to users
    // upgrading from older lumerad builds).
    raw := []byte(`{"version": 1, "single": {"pub_key_b64": "AAAA"}, "partial_sigs": []}`)
    tmp := filepath.Join(t.TempDir(), "v1.json")
    require.NoError(t, os.WriteFile(tmp, raw, 0o600))

    _, err := cli.LoadPartialProof(tmp)
    require.Error(t, err)
    require.Contains(t, err.Error(), "unsupported partial_proof version 1 (expected 2)")
    require.NotContains(t, err.Error(), "unknown field")
}

func TestLoadPartialProof_V2FileWithFutureField_UnknownFieldError(t *testing.T) {
    // Exercises pass 2's drift detection: a v2 file with a field this
    // binary doesn't know about must fail with "unknown field" rather
    // than silently parse into zero-valued fields.
    raw := []byte(`{"version": 2, "future_field": "something", "kind": "claim"}`)
    tmp := filepath.Join(t.TempDir(), "v2-future.json")
    require.NoError(t, os.WriteFile(tmp, raw, 0o600))

    _, err := cli.LoadPartialProof(tmp)
    require.Error(t, err)
    require.Contains(t, err.Error(), "unknown field")
}
```

- [ ] **Step 3d: Migrate existing `tx_multisig_test.go` fixtures to the new shape**

The external tests at [tx_multisig_test.go:18-62](x/evmigration/client/cli/tx_multisig_test.go#L18-L62) reference the removed types `PartialSingle`, `PartialMultisig`, `PartialSubSignature` and the flat `PartialSigs` slice. Rewrite each fixture. For each construction of the form:

```go
// OLD
pp := &cli.PartialProof{
    // ... common fields ...
    Single: &cli.PartialSingle{PubKeyB64: "AAAA", SigFormat: "SIG_FORMAT_CLI"},
    PartialSigs: []cli.PartialSubSignature{{Index: 0, SignatureB64: "BBBB"}},
}
```

Produce:

```go
// NEW
pp := &cli.PartialProof{
    // ... common fields ...
    Legacy: &cli.SideSpec{PubKey: "AAAA", SigFormat: "SIG_FORMAT_CLI"},
    New:    &cli.SideSpec{PubKey: "CCCC", SigFormat: "SIG_FORMAT_CLI"}, // fresh eth-keyed stand-in; tests should generate a valid pubkey from a deterministic seed where possible
    PartialLegacySignatures: []cli.PartialSignature{{Index: 0, Signature: "BBBB"}},
    PartialNewSignatures:    []cli.PartialSignature{{Index: 0, Signature: "DDDD"}},
}
```

For the multisig construction:

```go
// OLD
a := &cli.PartialProof{
    Multisig: &cli.PartialMultisig{Threshold: 2, SubPubKeysB64: []string{"x", "y", "z"}, SigFormat: "SIG_FORMAT_CLI"},
}
```

Produce:

```go
// NEW
a := &cli.PartialProof{
    Legacy: &cli.SideSpec{Threshold: 2, SubPubKeys: []string{"x", "y", "z"}, SigFormat: "SIG_FORMAT_CLI"},
    New:    &cli.SideSpec{Threshold: 2, SubPubKeys: []string{"X", "Y", "Z"}, SigFormat: "SIG_FORMAT_CLI"},
}
```

For tests that previously checked "single vs multisig mismatch between partial files" (`a := &cli.PartialProof{Single: ...}; b := &cli.PartialProof{Multisig: ...}`), rewrite to exercise the new `assertSideSpecsEqual` rejections — e.g., one file with `Legacy.PubKey` set and another with `Legacy.Threshold`/`Legacy.SubPubKeys` set. The assertion text in `AssertPartialProofsConsistent` changes from the current `'single' vs 'multisig' mismatch` to whatever the Step 3b helper produces (`"legacy side pub_key differs"` etc.) — update test `require.ErrorContains(t, err, ...)` strings accordingly.

Run: `go test ./x/evmigration/client/cli/... -v -count=1 2>&1 | tail -40`
Expected: all pre-existing tests pass under the new fixtures; failing tests point at genuine behavior changes (which Step 3b's new `validateSideSpec` / `assertSideSpecsEqual` messages should account for).

- [ ] **Step 4: Seed `PartialProof.New` from the flags**

Add these helpers (above or near `cmdGenerateProofPayload`):

```go
// b64encodeAll returns base64-encoded strings for every input byte slice,
// preserving order. Used to serialize pubkey byte-slices into PartialProof JSON.
func b64encodeAll(in [][]byte) []string {
    out := make([]string, len(in))
    for i, b := range in {
        out[i] = base64.StdEncoding.EncodeToString(b)
    }
    return out
}

// resolveEthSubKey accepts either a keyring key-name or a base64-encoded
// 33-byte compressed eth_secp256k1 pubkey and returns the raw pubkey bytes.
// Errors if the spec resolves to a non-ethsecp256k1 key.
func resolveEthSubKey(clientCtx client.Context, spec string) ([]byte, error) {
    // Try as keyring name first.
    if rec, err := clientCtx.Keyring.Key(spec); err == nil {
        pk, err := rec.GetPubKey()
        if err != nil {
            return nil, fmt.Errorf("cannot get pubkey for key %q: %w", spec, err)
        }
        ethPK, ok := pk.(*evmcryptotypes.PubKey)
        if !ok {
            return nil, fmt.Errorf("key %q is %T, expected eth_secp256k1", spec, pk)
        }
        return ethPK.Key, nil
    }
    // Try as base64 pubkey.
    raw, err := base64.StdEncoding.DecodeString(spec)
    if err != nil {
        return nil, fmt.Errorf("%q is neither a keyring key nor a base64-encoded pubkey: %w", spec, err)
    }
    if len(raw) != 33 {
        return nil, fmt.Errorf("base64 pubkey %q decodes to %d bytes, expected 33", spec, len(raw))
    }
    return raw, nil
}
```

Implementation in `cmdGenerateProofPayload`. **Crucial ordering (Finding 1):** the final `newAddr` must be derived from `--new-key` or `--new-sub-pub-keys` BEFORE computing `PayloadHex`. The existing command builds `PayloadHex` from `--new` directly; that order is wrong when the user also passes `--new-key`/`--new-sub-pub-keys`, because the signed payload would embed the wrong address. Move the pp construction to after the new-side derivation:

```go
import "github.com/cosmos/cosmos-sdk/crypto/keys/multisig" // aliased as kmultisig in existing code

if newKey != "" && len(newSubKeys) > 0 {
    return fmt.Errorf("pass either --new-key (single-key destination) OR --new-sub-pub-keys (multisig destination), not both")
}

// Derive the authoritative newAddr from the new-side flags.
var (
    derivedNewAddr string
    newSide        *SideSpec
)
switch {
case newKey != "":
    rec, err := clientCtx.Keyring.Key(newKey)
    if err != nil {
        return fmt.Errorf("new key %q not found in keyring: %w", newKey, err)
    }
    pk, err := rec.GetPubKey()
    if err != nil {
        return err
    }
    ethPK, ok := pk.(*evmcryptotypes.PubKey)
    if !ok {
        return fmt.Errorf("key %q is %T, expected eth_secp256k1", newKey, pk)
    }
    derivedNewAddr = sdk.AccAddress(ethPK.Address()).String()
    newSide = &SideSpec{
        PubKey:    base64.StdEncoding.EncodeToString(ethPK.Key),
        SigFormat: types.SigFormat_SIG_FORMAT_CLI.String(),
    }

case len(newSubKeys) > 0:
    if newThreshold == 0 {
        return fmt.Errorf("--new-threshold required when --new-sub-pub-keys is set")
    }
    if int(newThreshold) > len(newSubKeys) {
        return fmt.Errorf("--new-threshold=%d exceeds --new-sub-pub-keys count=%d", newThreshold, len(newSubKeys))
    }
    subBytes := make([][]byte, len(newSubKeys))
    subPubKeys := make([]cryptotypes.PubKey, len(newSubKeys))
    for i, spec := range newSubKeys {
        raw, err := resolveEthSubKey(clientCtx, spec)
        if err != nil {
            return fmt.Errorf("new sub-key %d (%q): %w", i, spec, err)
        }
        subBytes[i] = raw
        subPubKeys[i] = &evmcryptotypes.PubKey{Key: raw}
    }
    multiPK := kmultisig.NewLegacyAminoPubKey(int(newThreshold), subPubKeys)
    derivedNewAddr = sdk.AccAddress(multiPK.Address()).String()
    newSide = &SideSpec{
        Threshold:  uint32(newThreshold),
        SubPubKeys: b64encodeAll(subBytes),
        SigFormat:  types.SigFormat_SIG_FORMAT_CLI.String(),
    }

default:
    return fmt.Errorf("must pass either --new-key (single-key destination) or --new-sub-pub-keys + --new-threshold (multisig destination)")
}

// Cross-check: if the user passed --new explicitly, it must agree with the derived address.
// This catches the foot-gun where --new says one bech32 and --new-key/--new-sub-pub-keys
// derives to another. Silently overriding or leaving payload_hex bound to the wrong
// address would produce unusable partials.
if newStr != "" && newStr != derivedNewAddr {
    return fmt.Errorf("--new=%q disagrees with address derived from --new-key/--new-sub-pub-keys (%q); omit --new or correct the key material", newStr, derivedNewAddr)
}

// Seed the legacy side from the on-chain account pubkey (or --legacy-key for nil-pubkey
// single-sig accounts — see existing branches at tx_multisig.go:414-483). This block
// produces legacySide of type *SideSpec, consuming accPubKey, legacyKey, and sigFmtStr
// (already computed earlier in the command's RunE).
legacySide, err := buildLegacySideSpec(clientCtx, accPubKey, legacyKey, sigFmtStr, legacyAddr)
if err != nil {
    return err
}

// Now assemble PartialProof with the authoritative newAddr embedded in PayloadHex.
pp := &PartialProof{
    Version:                 partialProofVersion,
    Kind:                    kind,
    LegacyAddress:           legacyStr,
    NewAddress:              derivedNewAddr,
    ChainID:                 clientCtx.ChainID,
    EVMChainID:              evmChainID,
    PayloadHex:              hexEncode([]byte(ComputePayload(clientCtx.ChainID, evmChainID, kind, legacyStr, derivedNewAddr))),
    Legacy:                  legacySide,
    New:                     newSide,
    PartialLegacySignatures: []PartialSignature{},
    PartialNewSignatures:    []PartialSignature{},
}

// buildLegacySideSpec mirrors the existing switch at tx_multisig.go:414-483, producing
// a *SideSpec instead of setting the now-removed pp.Single / pp.Multisig. The four
// branches map exactly:
//
//   *secp256k1.PubKey on-chain           -> SideSpec{PubKey: base64(pubkey), SigFormat}
//   *multisig.LegacyAminoPubKey on-chain -> SideSpec{Threshold, SubPubKeys, SigFormat}
//   nil on-chain + --legacy-key          -> SideSpec{PubKey: base64(keyring pubkey), SigFormat}
//   nil on-chain, no --legacy-key        -> error with the "submit any tx first / pass
//                                           --legacy-key" remediation (unchanged from today).
//
// Keep the existing key-type assertions (rejects --legacy-key pointing at an eth key)
// and the --legacy-key-matches-on-chain-pubkey cross-check where applicable.
func buildLegacySideSpec(clientCtx client.Context, accPubKey cryptotypes.PubKey, legacyKeyName, sigFmt string, legacyAddr sdk.AccAddress) (*SideSpec, error) {
    switch pk := accPubKey.(type) {
    case *secp256k1.PubKey:
        if legacyKeyName != "" {
            rec, err := clientCtx.Keyring.Key(legacyKeyName)
            if err != nil {
                return nil, fmt.Errorf("--legacy-key %q not found: %w", legacyKeyName, err)
            }
            kp, err := rec.GetPubKey()
            if err != nil {
                return nil, err
            }
            if !bytes.Equal(kp.Bytes(), pk.Bytes()) {
                return nil, fmt.Errorf("--legacy-key pubkey does not match on-chain pubkey for %s", legacyAddr)
            }
        }
        return &SideSpec{
            PubKey:    base64.StdEncoding.EncodeToString(pk.Bytes()),
            SigFormat: sigFmt,
        }, nil

    case *kmultisig.LegacyAminoPubKey:
        if legacyKeyName != "" {
            return nil, fmt.Errorf("--legacy-key is not applicable to multisig accounts; co-signers sign via sign-proof")
        }
        subs := pk.GetPubKeys()
        subBytes := make([]string, len(subs))
        for i, sub := range subs {
            cpk, ok := sub.(*secp256k1.PubKey)
            if !ok {
                return nil, fmt.Errorf("legacy multisig sub-key %d is %T, expected Cosmos secp256k1", i, sub)
            }
            subBytes[i] = base64.StdEncoding.EncodeToString(cpk.Bytes())
        }
        return &SideSpec{
            Threshold:  uint32(pk.Threshold),
            SubPubKeys: subBytes,
            SigFormat:  sigFmt,
        }, nil

    case nil:
        if legacyKeyName == "" {
            return nil, fmt.Errorf(
                "account at %s has no on-chain pubkey record; pass --legacy-key to seed the pubkey from your keyring (single-sig only), or for a multisig address submit a 1-ulume self-send first",
                legacyAddr,
            )
        }
        rec, err := clientCtx.Keyring.Key(legacyKeyName)
        if err != nil {
            return nil, fmt.Errorf("--legacy-key %q not found: %w", legacyKeyName, err)
        }
        kp, err := rec.GetPubKey()
        if err != nil {
            return nil, err
        }
        cpk, ok := kp.(*secp256k1.PubKey)
        if !ok {
            return nil, fmt.Errorf("--legacy-key is %T, expected Cosmos secp256k1 (eth keys belong on --new-key)", kp)
        }
        derivedAddr := sdk.AccAddress(cpk.Address())
        if !derivedAddr.Equals(legacyAddr) {
            return nil, fmt.Errorf("--legacy-key derives to %s, not the requested --legacy %s", derivedAddr, legacyAddr)
        }
        return &SideSpec{
            PubKey:    base64.StdEncoding.EncodeToString(cpk.Bytes()),
            SigFormat: sigFmt,
        }, nil

    default:
        return nil, fmt.Errorf("legacy account has unsupported pubkey type %T (expected Cosmos secp256k1 or LegacyAminoPubKey)", pk)
    }
}

// Shape-mirroring check: legacy multisig => new must be multisig; legacy single-key => new must be single-key.
if pp.Legacy.PubKey != "" && pp.New.PubKey == "" {
    return fmt.Errorf("legacy account is single-key; --new-sub-pub-keys is not allowed (destination shape must mirror source)")
}
if pp.Legacy.Threshold > 0 && pp.New.Threshold == 0 {
    return fmt.Errorf("legacy account is multisig; --new-key is not allowed (destination shape must mirror source)")
}

// Key-reuse guard: no new eth sub-pubkey may equal any legacy sub-pubkey.
if pp.New.Threshold > 0 && pp.Legacy.Threshold > 0 {
    legacySet := make(map[string]struct{}, len(pp.Legacy.SubPubKeys))
    for _, k := range pp.Legacy.SubPubKeys {
        legacySet[k] = struct{}{}
    }
    for i, k := range pp.New.SubPubKeys {
        if _, reused := legacySet[k]; reused {
            return fmt.Errorf("new sub-pub-key %d reuses a legacy sub-pubkey; generate a fresh eth key per co-signer", i)
        }
    }
}
```

- [ ] **Step 5: Write a CLI unit test**

In `x/evmigration/client/cli/tx_multisig_internal_test.go`:

```go
func TestGenerateProofPayload_MultisigToMultisig_SeedsNewSubKeys(t *testing.T) {
    // Stub on-chain account as a Cosmos multisig
    // Invoke cmdGenerateProofPayload with --new-sub-pub-keys pointing to 3 eth keys
    // Assert: pp.New.SubPubKeys has 3 entries; pp.New.Threshold == expected
}

func TestGenerateProofPayload_RejectsShapeMismatch(t *testing.T) {
    // Legacy is multisig but user passes --new-key → error
    // Legacy is single-key but user passes --new-sub-pub-keys → error
}
```

- [ ] **Step 6: Run and commit**

```bash
go test ./x/evmigration/client/cli/... -v -count=1 -run "TestGenerateProofPayload" 2>&1 | tail -20
make lint
git add x/evmigration/client/cli/tx_multisig.go x/evmigration/client/cli/tx_multisig_internal_test.go
git commit -m "evmigration(cli): generate-proof-payload seeds new-side shape from flags

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 15: `sign-proof` signs both halves in one invocation

**Files:**
- Modify: `x/evmigration/client/cli/tx_multisig.go`
- Modify: `x/evmigration/client/cli/tx_multisig_internal_test.go`

- [ ] **Step 1: Add `--new-key` flag alongside existing `--from`**

The cobra command already accepts `--from` (standard SDK flag). Add:

```go
cmd.Flags().String(flagNewKey, "", "Keyring name of the destination-side sub-key (must be eth_secp256k1)")
```

Require that at least one of `--from` / `--new-key` is supplied.

- [ ] **Step 2: Sign each side — matching the existing per-format convention**

`payload` is shared by both branches — declare it once before the conditionals so the new-only path (no `--from`) also has access. **Critical:** the existing CLI (see [x/evmigration/client/cli/tx_multisig.go:576-588](x/evmigration/client/cli/tx_multisig.go#L576-L588) and [tx.go:209](x/evmigration/client/cli/tx.go#L209)) uses specific signing conventions that the on-chain verifier expects. Do not diverge:

| Side | SigFormat | Pre-hash before `Keyring.Sign`? | `SignMode` |
|------|-----------|---------------------------------|------------|
| legacy (Cosmos secp256k1) | `SIG_FORMAT_CLI` | **Yes** — `sha256(payload)` | `SIGN_MODE_UNSPECIFIED` |
| legacy (Cosmos secp256k1) | `SIG_FORMAT_ADR036` | No — pass canonical JSON doc | `SIGN_MODE_UNSPECIFIED` |
| new (eth_secp256k1) | `SIG_FORMAT_CLI` | No — eth keyring applies Keccak256 internally | `SIGN_MODE_LEGACY_AMINO_JSON` |
| new (eth_secp256k1) | `SIG_FORMAT_ADR036` | No — pass canonical JSON doc | `SIGN_MODE_LEGACY_AMINO_JSON` |
| new (eth_secp256k1) | `SIG_FORMAT_EIP191` | No — pass EIP-191-wrapped envelope | `SIGN_MODE_LEGACY_AMINO_JSON` |

EIP-191 is only valid on single-key new-side proofs; `sign-proof` in the multisig flow never produces it (see design §2 Non-Goals and §4.1).

```go
import (
    "encoding/hex"
    signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
    "github.com/LumeraProtocol/lumera/x/evmigration/types"
    "github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

payload, err := hex.DecodeString(pp.PayloadHex)
if err != nil {
    return fmt.Errorf("invalid payload_hex in partial file: %w", err)
}

if fromKey == "" && newKey == "" {
    return fmt.Errorf("at least one of --from (legacy sub-key) or --new-key (new sub-key) must be supplied")
}

if fromKey != "" {
    idx, err := findSubKeyIndex(clientCtx, fromKey, pp.Legacy, sigverify.SubKeyTypeCosmosSecp256k1)
    if err != nil {
        return fmt.Errorf("--from: %w", err)
    }
    signerAddr, err := deriveSubKeyAddr(clientCtx, fromKey)
    if err != nil {
        return fmt.Errorf("--from: %w", err)
    }
    signInput, err := legacySigningInput(payload, pp.Legacy.SigFormat, signerAddr)
    if err != nil {
        return err
    }
    sig, _, err := clientCtx.Keyring.Sign(fromKey, signInput, signingtypes.SignMode_SIGN_MODE_UNSPECIFIED)
    if err != nil {
        return fmt.Errorf("legacy sign: %w", err)
    }
    pp.PartialLegacySignatures = upsertSig(pp.PartialLegacySignatures, PartialSignature{
        Index:     idx,
        Signature: base64.StdEncoding.EncodeToString(sig),
    })
}

if newKey != "" {
    idx, err := findSubKeyIndex(clientCtx, newKey, pp.New, sigverify.SubKeyTypeEthSecp256k1)
    if err != nil {
        return fmt.Errorf("--new-key: %w", err)
    }
    signerAddr, err := deriveSubKeyAddr(clientCtx, newKey)
    if err != nil {
        return fmt.Errorf("--new-key: %w", err)
    }
    signInput, err := newSigningInput(payload, pp.New.SigFormat, signerAddr)
    if err != nil {
        return err
    }
    sig, _, err := clientCtx.Keyring.Sign(newKey, signInput, signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON)
    if err != nil {
        return fmt.Errorf("new sign: %w", err)
    }
    pp.PartialNewSignatures = upsertSig(pp.PartialNewSignatures, PartialSignature{
        Index:     idx,
        Signature: base64.StdEncoding.EncodeToString(sig),
    })
}
```

Concrete helper implementations (place near the top of `tx_multisig.go`):

```go
// legacySigningInput returns the bytes to pass to Keyring.Sign for a
// Cosmos-secp256k1-side partial, matching what the on-chain verifier's
// sigverify.VerifyCosmosSecp256k1 expects.
func legacySigningInput(payload []byte, format string, signerAddr string) ([]byte, error) {
    switch format {
    case types.SigFormat_SIG_FORMAT_CLI.String():
        h := sha256.Sum256(payload)
        return h[:], nil
    case types.SigFormat_SIG_FORMAT_ADR036.String():
        return sigverify.ADR036SignDoc(signerAddr, payload), nil
    case types.SigFormat_SIG_FORMAT_EIP191.String():
        return nil, fmt.Errorf("SIG_FORMAT_EIP191 is not valid on the legacy side")
    default:
        return nil, fmt.Errorf("unsupported legacy sig_format %q", format)
    }
}

// newSigningInput returns the bytes to pass to Keyring.Sign for an
// eth-secp256k1-side partial, matching what the on-chain verifier's
// sigverify.VerifyEthSecp256k1 expects. Multisig signing never uses EIP-191
// but the helper tolerates it for symmetry with single-key flows.
func newSigningInput(payload []byte, format string, signerAddr string) ([]byte, error) {
    switch format {
    case types.SigFormat_SIG_FORMAT_CLI.String():
        return payload, nil
    case types.SigFormat_SIG_FORMAT_EIP191.String():
        return sigverify.EIP191PersonalSignPayload(payload), nil
    case types.SigFormat_SIG_FORMAT_ADR036.String():
        return sigverify.ADR036SignDoc(signerAddr, payload), nil
    default:
        return nil, fmt.Errorf("unsupported new sig_format %q", format)
    }
}

// findSubKeyIndex looks up keyName in the keyring, matches its pubkey against
// spec.SubPubKeys (for multisig) or spec.PubKey (for single-key), and returns
// the sub-key index. Errors on key not found, mismatch, or key-type mismatch.
func findSubKeyIndex(clientCtx client.Context, keyName string, spec *SideSpec, expected sigverify.SubKeyType) (uint32, error) {
    rec, err := clientCtx.Keyring.Key(keyName)
    if err != nil {
        return 0, fmt.Errorf("key %q not found in keyring: %w", keyName, err)
    }
    pk, err := rec.GetPubKey()
    if err != nil {
        return 0, err
    }
    var keyBytes []byte
    switch expected {
    case sigverify.SubKeyTypeCosmosSecp256k1:
        cpk, ok := pk.(*secp256k1.PubKey)
        if !ok {
            return 0, fmt.Errorf("key %q is %T, expected Cosmos secp256k1", keyName, pk)
        }
        keyBytes = cpk.Bytes()
    case sigverify.SubKeyTypeEthSecp256k1:
        epk, ok := pk.(*evmcryptotypes.PubKey)
        if !ok {
            return 0, fmt.Errorf("key %q is %T, expected eth_secp256k1", keyName, pk)
        }
        keyBytes = epk.Key
    default:
        return 0, fmt.Errorf("unknown expected sub-key type")
    }
    target := base64.StdEncoding.EncodeToString(keyBytes)
    // Single-key side:
    if spec.PubKey != "" {
        if spec.PubKey != target {
            return 0, fmt.Errorf("key %q pubkey does not match partial.PubKey", keyName)
        }
        return 0, nil
    }
    // Multisig side:
    for i, k := range spec.SubPubKeys {
        if k == target {
            return uint32(i), nil
        }
    }
    return 0, fmt.Errorf("key %q pubkey is not a member of partial.SubPubKeys", keyName)
}

// deriveSubKeyAddr returns the bech32 address of keyName from the keyring.
// Returns an error (rather than an empty string) when the keyring lookup or
// address derivation fails: the ADR-036 sign-doc embeds this bech32 as the
// "signer" field, so an empty string would silently produce a partial that
// only fails later during combine-proof verification, with a cryptic
// "signature invalid" message instead of a clear "key not found" error.
func deriveSubKeyAddr(clientCtx client.Context, keyName string) (string, error) {
    rec, err := clientCtx.Keyring.Key(keyName)
    if err != nil {
        return "", fmt.Errorf("cannot look up key %q for signer-address derivation: %w", keyName, err)
    }
    addr, err := rec.GetAddress()
    if err != nil {
        return "", fmt.Errorf("cannot derive address for key %q: %w", keyName, err)
    }
    return addr.String(), nil
}

// upsertSig replaces any entry at the same index, otherwise appends — idempotent.
func upsertSig(existing []PartialSignature, fresh PartialSignature) []PartialSignature {
    filtered := existing[:0]
    for _, p := range existing {
        if p.Index != fresh.Index {
            filtered = append(filtered, p)
        }
    }
    return append(filtered, fresh)
}
```

- [ ] **Step 3: Write tests**

```go
func TestSignProof_SignsBothSides(t *testing.T) { /* --from + --new-key → both partial_*_signatures updated */ }
func TestSignProof_LegacyOnly(t *testing.T)     { /* only --from */ }
func TestSignProof_NewOnly(t *testing.T)        { /* only --new-key */ }
func TestSignProof_Idempotent(t *testing.T)     { /* resigning at same index overwrites */ }
func TestSignProof_WrongKeyType_NewSide(t *testing.T) { /* Cosmos secp256k1 key with --new-key → rejected */ }
```

- [ ] **Step 4: Run and commit**

```bash
go test ./x/evmigration/client/cli/... -v -run "TestSignProof" -count=1
make lint
git add x/evmigration/client/cli/tx_multisig.go x/evmigration/client/cli/tx_multisig_internal_test.go
git commit -m "evmigration(cli): sign-proof signs both legacy and new halves per invocation

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 16: `combine-proof` verifies partials cryptographically, then selects K valid

**Design Finding 2:** the revised design dropped the "verify partials during combine" behavior that the prior single-EOA implementation had. Restore it: verify every merged partial sig under its claimed sub-pubkey; drop invalid entries; select the K valid partials with the lowest ascending indices.

**Files:**
- Modify: `x/evmigration/client/cli/tx_multisig.go`
- Modify: `x/evmigration/client/cli/tx_multisig_internal_test.go`

- [ ] **Step 1: Add a shared `buildProofFromPartial` with verification**

Use existing helpers wherever possible: `ParseSigFormat` already lives at [tx_multisig.go:106](x/evmigration/client/cli/tx_multisig.go#L106). For base64/hex decoding, use stdlib directly — no `mustB64`/`mustHex` panickers. All decode errors must propagate as returned errors so an engineer debugging a malformed partial file sees exactly which field rejected.

```go
import (
    "encoding/base64"
    "encoding/hex"
    "github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

// decodeBase64 wraps base64.StdEncoding.DecodeString with a field-aware error.
// Used inside buildProofFromPartial for pubkey and signature byte-slices.
func decodeBase64(field, in string) ([]byte, error) {
    out, err := base64.StdEncoding.DecodeString(in)
    if err != nil {
        return nil, fmt.Errorf("%s: %w", field, err)
    }
    return out, nil
}

// buildProofFromPartial assembles a MigrationProof from a partial-file side spec
// and the merged partial signatures for that side. It VERIFIES every merged
// partial signature using the appropriate sub-key type (Cosmos on legacy,
// eth on new), DROPS invalid ones with a warning, and selects the K valid
// partials with the lowest ascending indices (canonical order). Returns an
// error if fewer than threshold partials verify.
func buildProofFromPartial(
    side *SideSpec, sigs []PartialSignature, payload []byte,
    keyType sigverify.SubKeyType, sideLabel string, stderr io.Writer,
) (types.MigrationProof, error) {
    format, err := ParseSigFormat(side.SigFormat)
    if err != nil {
        return types.MigrationProof{}, fmt.Errorf("%s side sig_format %q: %w", sideLabel, side.SigFormat, err)
    }

    // Single-key side: design §4.5 says exactly one partial signature at index 0.
    // Reject extras (could mask co-signer confusion) and missing/mislabeled entries.
    if side.PubKey != "" {
        switch {
        case len(sigs) == 0:
            return types.MigrationProof{}, fmt.Errorf("%s side has no partial signature (single-key side expects exactly one at index 0)", sideLabel)
        case len(sigs) > 1:
            return types.MigrationProof{}, fmt.Errorf("%s side has %d partial signatures; single-key side expects exactly one at index 0", sideLabel, len(sigs))
        case sigs[0].Index != 0:
            return types.MigrationProof{}, fmt.Errorf("%s side partial signature has index=%d; single-key side expects index=0", sideLabel, sigs[0].Index)
        }
        pkBytes, err := decodeBase64(sideLabel+".pub_key", side.PubKey)
        if err != nil {
            return types.MigrationProof{}, err
        }
        // Length-check BEFORE verifyOne, because secp256k1.PubKey.Address()
        // (called inside verifyOne) panics on wrong-length Key bytes.
        // The keeper's ValidateBasic performs this check on-chain, but the
        // CLI's buildProofFromPartial path doesn't go through ValidateBasic.
        if len(pkBytes) != secp256k1.PubKeySize {
            return types.MigrationProof{}, fmt.Errorf("%s side single-key pub_key: expected %d bytes, got %d", sideLabel, secp256k1.PubKeySize, len(pkBytes))
        }
        sigBytes, err := decodeBase64(fmt.Sprintf("%s.partial_signatures[0].signature", sideLabel), sigs[0].Signature)
        if err != nil {
            return types.MigrationProof{}, err
        }
        if err := verifyOne(keyType, pkBytes, payload, sigBytes, format); err != nil {
            return types.MigrationProof{}, fmt.Errorf("%s side single-key partial signature invalid: %w", sideLabel, err)
        }
        return types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
            PubKey: pkBytes, Signature: sigBytes, SigFormat: format,
        }}}, nil
    }

    // Multisig: verify every merged partial, collect valid indices in ascending order.
    subPubs := make([][]byte, len(side.SubPubKeys))
    for i, k := range side.SubPubKeys {
        raw, err := decodeBase64(fmt.Sprintf("%s.sub_pub_keys[%d]", sideLabel, i), k)
        if err != nil {
            return types.MigrationProof{}, err
        }
        // Length-check EVERY sub-pubkey (not just the ones referenced by signer_indices),
        // because LegacyAminoPubKey address derivation consumes all N of them and the
        // concrete PubKey structs (both secp256k1 and ethsecp256k1) panic on Address()
        // when Key is the wrong length.
        if len(raw) != secp256k1.PubKeySize {
            return types.MigrationProof{}, fmt.Errorf("%s side sub_pub_keys[%d]: expected %d bytes, got %d", sideLabel, i, secp256k1.PubKeySize, len(raw))
        }
        subPubs[i] = raw
    }

    // Sort merged sigs by index so canonical ordering is deterministic.
    sort.Slice(sigs, func(i, j int) bool { return sigs[i].Index < sigs[j].Index })

    validIdxs := make([]uint32, 0, len(sigs))
    validSigs := make([][]byte, 0, len(sigs))
    for _, ps := range sigs {
        if int(ps.Index) >= len(subPubs) {
            fmt.Fprintf(stderr, "WARN %s side: dropping partial at index %d (out of range for N=%d)\n", sideLabel, ps.Index, len(subPubs))
            continue
        }
        sigBytes, err := decodeBase64(fmt.Sprintf("%s.partial_signatures[index=%d].signature", sideLabel, ps.Index), ps.Signature)
        if err != nil {
            fmt.Fprintf(stderr, "WARN %s side: dropping partial at index %d (base64 decode error): %s\n", sideLabel, ps.Index, err)
            continue
        }
        if err := verifyOne(keyType, subPubs[ps.Index], payload, sigBytes, format); err != nil {
            fmt.Fprintf(stderr, "WARN %s side: dropping partial at index %d: %s\n", sideLabel, ps.Index, err)
            continue
        }
        validIdxs = append(validIdxs, ps.Index)
        validSigs = append(validSigs, sigBytes)
    }

    if uint32(len(validIdxs)) < side.Threshold {
        return types.MigrationProof{}, fmt.Errorf("need %d valid partial signatures on %s side, have %d",
            side.Threshold, sideLabel, len(validIdxs))
    }

    // Select the first K valid ones (lowest indices, already ascending).
    validIdxs = validIdxs[:int(side.Threshold)]
    validSigs = validSigs[:int(side.Threshold)]

    return types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
        Threshold: side.Threshold, SubPubKeys: subPubs,
        SignerIndices: validIdxs, SubSignatures: validSigs, SigFormat: format,
    }}}, nil
}

func verifyOne(keyType sigverify.SubKeyType, pubKeyBytes, payload, sig []byte, format types.SigFormat) error {
    switch keyType {
    case sigverify.SubKeyTypeCosmosSecp256k1:
        pk := &secp256k1.PubKey{Key: pubKeyBytes}
        return sigverify.VerifyCosmosSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, format)
    case sigverify.SubKeyTypeEthSecp256k1:
        pk := &ethsecp256k1.PubKey{Key: pubKeyBytes}
        return sigverify.VerifyEthSecp256k1(pk, sdk.AccAddress(pk.Address()), payload, sig, format)
    default:
        return fmt.Errorf("unknown sub-key type")
    }
}
```

Note: `sigverify.SubKeyType` and its constants are exported from the shared package created in Task 6 (see that task's Step 2). The keeper `verify.go` uses the same constants via the `sigverify.` prefix.

- [ ] **Step 2: Wire `combine-proof` to call `buildProofFromPartial` for each side**

```go
payload, err := hex.DecodeString(pp.PayloadHex)
if err != nil {
    return fmt.Errorf("invalid payload_hex in partial file: %w", err)
}

legacyProof, err := buildProofFromPartial(
    pp.Legacy, pp.PartialLegacySignatures, payload,
    sigverify.SubKeyTypeCosmosSecp256k1, "legacy", cmd.ErrOrStderr(),
)
if err != nil { return err }

newProof, err := buildProofFromPartial(
    pp.New, pp.PartialNewSignatures, payload,
    sigverify.SubKeyTypeEthSecp256k1, "new", cmd.ErrOrStderr(),
)
if err != nil { return err }

var msg sdk.Msg
switch pp.Kind {
case "claim":
    msg = &types.MsgClaimLegacyAccount{LegacyAddress: pp.LegacyAddress, NewAddress: pp.NewAddress, LegacyProof: legacyProof, NewProof: newProof}
case "validator":
    msg = &types.MsgMigrateValidator{LegacyAddress: pp.LegacyAddress, NewAddress: pp.NewAddress, LegacyProof: legacyProof, NewProof: newProof}
default:
    return fmt.Errorf("unsupported kind %q", pp.Kind)
}
```

- [ ] **Step 3: Tests**

```go
func TestCombineProof_BelowThresholdRejected_Legacy(t *testing.T)    {}
func TestCombineProof_BelowThresholdRejected_New(t *testing.T)       {}
func TestCombineProof_BothSidesThreshold_Valid(t *testing.T)         {}
func TestCombineProof_MismatchedPayloadsRejected(t *testing.T)       {}
func TestCombineProof_MultiFile(t *testing.T)                        {}

// Finding 2 coverage: invalid partials dropped, valid ones at higher indices still meet threshold.
func TestCombineProof_DropsInvalidPartial_SelectsValidHigherIndex(t *testing.T) {
    // Build a 2-of-3 fixture. Sub-signer 0 produces a CORRUPTED partial (wrong payload).
    // Sub-signers 1 and 2 produce valid partials.
    // combine-proof must: warn about index 0, drop it, assemble {1, 2} in ascending order, succeed.
    // Assert the resulting MultisigProof.SignerIndices == [1, 2] (NOT [0, 1]).
}

func TestCombineProof_AllPartialsInvalid_BelowThresholdError(t *testing.T) {
    // All partials corrupted. Expect "need K valid partial signatures on <side>, have 0".
}

func TestCombineProof_PartialOutOfRangeIndex_Dropped(t *testing.T) {
    // A partial claims index=99 for a 3-sub-key multisig. Dropped with warning.
}

// Finding 5 coverage: single-key side must be exactly one entry at index 0.
func TestCombineProof_SingleKeySide_Extras_Rejected(t *testing.T) {
    // Single-key side has 2 partial signatures (e.g., same key signed twice under
    // different payload variants). combine-proof must reject with
    // "<side> side has 2 partial signatures; single-key side expects exactly one at index 0".
}

func TestCombineProof_SingleKeySide_WrongIndex_Rejected(t *testing.T) {
    // Single-key side has one partial but at index 1 (caller mistake — the
    // signer shouldn't have specified a non-zero index for single-key).
    // combine-proof must reject with
    // "<side> side partial signature has index=1; single-key side expects index=0".
}

func TestCombineProof_SingleKeySide_Missing_Rejected(t *testing.T) {
    // Single-key side has zero partial signatures. Reject with
    // "<side> side has no partial signature (single-key side expects exactly one at index 0)".
}
```

- [ ] **Step 4: Run and commit**

Run: `go test ./x/evmigration/client/cli/... -v -run "TestCombineProof" -count=1`
Expected: all pass including the new `DropsInvalidPartial` case.

```bash
make lint
git add x/evmigration/client/cli/
git commit -m "$(cat <<'EOF'
evmigration(cli): combine-proof verifies partials cryptographically (Finding 2)

Before threshold selection, every merged partial signature is verified
under its claimed sub-pubkey using the shared types/sigverify helpers
(identical to the keeper's verification path — no CLI/keeper drift).
Invalid partials are dropped with a warning; valid partials with the
lowest ascending indices are selected. Prevents a stale or corrupted
low-index partial from poisoning a combined tx when other valid
partials exist at higher indices.

Restores the behavior the pre-revision single-EOA combine-proof had.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 17: `submit-proof` just broadcasts

**Files:**
- Modify: `x/evmigration/client/cli/tx_multisig.go`

- [ ] **Step 1: Remove new-signature signing logic from `cmdSubmitProof`**

The existing `submit-proof` calls `runMigrationTx` → `signNewMigrationProof`. In the new model, the tx in `<tx.json>` has its application-level proofs (`legacy_proof` and `new_proof`) fully assembled at `combine-proof` time. The command should:

1. Load the tx from the file.
2. Validate the message (`ValidateBasic`).
3. Simulate gas via `simulateMigrationGas` (the migration-specific estimator that does not inject a signer — see the comment at [tx.go:260-262](x/evmigration/client/cli/tx.go#L260-L262)).
4. `BuildUnsignedTx` — **the tx stays unsigned at the Cosmos layer**. The proto messages have no `cosmos.msg.v1.signer`, so `GetSigners()` returns empty; adding any envelope signature yields "expected 0, got 1". There is no broadcaster `--from` key.
5. Broadcast via `clientCtx.BroadcastTx`.

Remove `cmd.Flags().String(flagTxTimeout, ...)` only if it was bound to the signer path; tx-timeout (the `timeout_height` on the unsigned tx body) is still useful. Do **not** re-introduce `AddTxFlagsToCmd`'s `--from` expectation in a way that requires a funded key — the submit is file-driven and funded-account-free.

Reuse the existing SDK helpers `txf.WithGas(simRes.GasInfo.GasUsed * adjustment)`, `txf.BuildUnsignedTx`, `clientCtx.BroadcastTx`. Do **not** call `tx.Sign(...)` — the existing `runMigrationTx` path at [tx.go:275](x/evmigration/client/cli/tx.go#L275) already goes `BuildUnsignedTx` → `confirmMigrationTx` → `BroadcastTx`, skipping envelope signing entirely. Mirror that path.

- [ ] **Step 2: Test**

```go
func TestSubmitProof_FullyAssembled_Broadcasts(t *testing.T) {
    // Given a fully-assembled MigrationProof on both sides, submit-proof should
    // just run ValidateBasic + BuildUnsignedTx + broadcast — NO envelope signing.
    // Regression lock: if a future refactor accidentally introduces AddTxFlagsToCmd-style
    // --from-based signing, this test should fail with the chain's "expected 0, got 1" error.
}

func TestSubmitProof_RejectsFromFlagOrEnvelopeSig(t *testing.T) {
    // Optional hardening: if the command still accepts --from from some upstream flag
    // binding, passing a real funded key must either be silently ignored OR produce a
    // clear "migration txs are unsigned at the Cosmos layer; --from is not applicable" error.
}
```

- [ ] **Step 3: Commit**

```bash
make lint
git add x/evmigration/client/cli/tx_multisig.go x/evmigration/client/cli/tx_multisig_internal_test.go
git commit -m "evmigration(cli): submit-proof broadcasts pre-assembled migration tx

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 18: End-to-end CLI test (four-step multisig→multisig)

**Files:**
- Modify: `x/evmigration/client/cli/tx_multisig_test.go` (network-backed test)

- [ ] **Step 1: Write the test**

```go
func TestCLI_MultisigToMultisig_EndToEnd(t *testing.T) {
    // 1. Start a network with a pre-seeded Cosmos multisig legacy account.
    // 2. Pre-create 3 eth_secp256k1 keys on the test keyring.
    // 3. Run generate-proof-payload with --new-sub-pub-keys and --new-threshold.
    //    Assert the output file contains both pp.Legacy and pp.New populated.
    // 4. Run sign-proof --from legacy-sub-1 --new-key eth-sub-1.
    // 5. Run sign-proof --from legacy-sub-2 --new-key eth-sub-2.
    // 6. Run combine-proof → assert the tx json has MigrationProof{Multisig} on both sides.
    // 7. Run submit-proof; wait for height.
    // 8. Query MigrationRecord; assert delegations re-keyed, balance moved.
    // 9. Query new BaseAccount; assert PubKey is the reconstructed LegacyAminoPubKey over the eth sub-keys.
}
```

- [ ] **Step 2: Run and commit**

```bash
go test ./x/evmigration/client/cli/... -v -run "TestCLI_MultisigToMultisig_EndToEnd" -count=1 -timeout 5m
make lint
git add x/evmigration/client/cli/tx_multisig_test.go
git commit -m "evmigration(cli): end-to-end multisig→multisig network test

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 6 — Integration Tests

### Task 19: `TestMsgClaimLegacyAccount_MultisigToMultisig`

**Files:**
- Modify: `tests/integration/evmigration/migration_test.go`
- Create: `tests/integration/evmigration/multisig_helpers.go` (if not present)

- [ ] **Step 1: Add helper functions**

```go
// tests/integration/evmigration/multisig_helpers.go
func buildLegacyMultisig(t *testing.T, N, K int) (addr sdk.AccAddress, subPrivs []*secp256k1.PrivKey, pk *kmultisig.LegacyAminoPubKey) { ... }
func buildNewMultisig(t *testing.T, N, K int) (addr sdk.AccAddress, subPrivs []*ethsecp256k1.PrivKey, pk *kmultisig.LegacyAminoPubKey) { ... }
func signLegacyMultisigProof(payload []byte, subPrivs []*secp256k1.PrivKey, signerIdxs []int, format types.SigFormat) *types.MultisigProof { ... }
func signNewMultisigProof(payload []byte, subPrivs []*ethsecp256k1.PrivKey, signerIdxs []int, format types.SigFormat) *types.MultisigProof { ... }
```

- [ ] **Step 2: Write the integration test**

```go
//go:build integration && test

func TestMsgClaimLegacyAccount_MultisigToMultisig(t *testing.T) {
    app, ctx := setupApp(t) // existing helper

    legacyAddr, legacyPrivs, _ := buildLegacyMultisig(t, 3, 2)
    newAddr, _, newPK := buildNewMultisig(t, 3, 2)

    // Fund legacy.
    fundAccount(t, app, ctx, legacyAddr, sdk.NewCoins(sdk.NewCoin("ulume", sdk.NewInt(1_000_000_000))))
    // Register legacy multisig pubkey on chain (simulate a prior tx).
    ensureLegacyPubKeyOnChain(t, app, ctx, legacyAddr, /* multisig pubkey */)

    payload := []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:claim:%s:%s",
        ctx.ChainID(), lcfg.EVMChainID, legacyAddr, newAddr))

    legacyProof := signLegacyMultisigProof(payload, legacyPrivs, []int{0, 2}, types.SigFormat_SIG_FORMAT_CLI)
    newProof    := signNewMultisigProof(payload, /* new privs */, []int{0, 2}, types.SigFormat_SIG_FORMAT_CLI)

    msg := &types.MsgClaimLegacyAccount{
        LegacyAddress: legacyAddr.String(),
        NewAddress:    newAddr.String(),
        LegacyProof:   types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: legacyProof}},
        NewProof:      types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: newProof}},
    }

    _, err := msgSrv.ClaimLegacyAccount(ctx, msg)
    require.NoError(t, err)

    // Assertions
    rec, err := keeper.MigrationRecords.Get(ctx, legacyAddr.String())
    require.NoError(t, err)
    require.Equal(t, newAddr.String(), rec.NewAddress)

    newAcc := app.AccountKeeper.GetAccount(ctx, newAddr)
    require.NotNil(t, newAcc.GetPubKey())
    require.Equal(t, newPK.Address(), newAcc.GetPubKey().Address())

    balance := app.BankKeeper.GetBalance(ctx, newAddr, "ulume")
    require.Equal(t, int64(1_000_000_000), balance.Amount.Int64())
}
```

- [ ] **Step 3: Run and commit**

```bash
go test -tags='integration test' ./tests/integration/evmigration/... -v -run TestMsgClaimLegacyAccount_MultisigToMultisig -count=1 -timeout 5m
make lint
git add tests/integration/evmigration/
git commit -m "evmigration(integration): multisig→multisig claim-legacy end-to-end

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 20: `TestMsgMigrateValidator_MultisigToMultisig` with post-migration `MsgEditValidator`

**Files:**
- Modify: `tests/integration/evmigration/migration_test.go`

- [ ] **Step 1: Write the test**

Reuses the helpers from Task 19. Steps:

1. Seed a validator whose operator is a 2-of-3 Cosmos multisig (via `MsgCreateValidator` from the multisig bech32; follow the devnet spike pattern).
2. Seed delegations to that validator from unrelated delegators.
3. Run `MsgMigrateValidator` with a multisig-to-multisig proof.
4. Assert the new operator address is the multisig-of-eth bech32; validator record is re-keyed; delegations re-keyed; supernode records re-keyed.
5. **Follow-on assertion**: sign `MsgEditValidator` from the new multisig (using 2 of the 3 eth sub-keys) and submit. Expect the moniker to update, as demonstrated in the devnet spike.

- [ ] **Step 2: Run and commit**

```bash
go test -tags='integration test' ./tests/integration/evmigration/... -v -run TestMsgMigrateValidator_MultisigToMultisig -count=1 -timeout 5m
make lint
git add tests/integration/evmigration/
git commit -m "evmigration(integration): multisig→multisig validator migration + post-migration MsgEditValidator

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 21: Additional regression & edge-case tests

**Files:**
- Modify: `tests/integration/evmigration/migration_test.go`

- [ ] **Step 1: Write the remaining tests**

```go
func TestMsgClaimLegacyAccount_SingleKeyToSingleKey_Regression(t *testing.T) { /* ensure prior behavior still works */ }
func TestMsgClaimLegacyAccount_MultisigVesting_ToMultisig(t *testing.T)      { /* continuous vesting preserved */ }
func TestMsgClaimLegacyAccount_Multisig_WrongThreshold(t *testing.T)         { /* K-1 legacy sigs OR K-1 new sigs rejected */ }
func TestMsgClaimLegacyAccount_Multisig_ReplayRejected(t *testing.T)         {}
func TestMsgClaimLegacyAccount_Multisig_ADR036_BothSides(t *testing.T)       {}
```

- [ ] **Step 2: Run and commit**

```bash
go test -tags='integration test' ./tests/integration/evmigration/... -v -count=1 -timeout 10m
make lint
git add tests/integration/evmigration/
git commit -m "evmigration(integration): multisig regression + vesting + replay + ADR036 coverage

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 7 — Devnet Tests

### Task 22: Update `devnet/tests/evmigration/multisig_keys.go`

**Files:**
- Modify: `devnet/tests/evmigration/multisig_keys.go`

- [ ] **Step 1: Extend fixture seeding**

- Pre-create 3 fresh `eth_secp256k1` keys per multisig fixture (for new-side sub-keys).
- Register the legacy multisig's `LegacyAminoPubKey` on-chain by signing a 1-ulume self-send before the test runs.
- Expose helpers `getLegacyMultisigKeys(idx)` and `getNewMultisigKeys(idx)` returning the key-name triples for CLI invocations.

- [ ] **Step 2: Build and commit**

```bash
cd devnet && go build ./... && cd -
git add devnet/tests/evmigration/multisig_keys.go
git commit -m "evmigration(devnet): seed eth_secp256k1 sub-keys for new-side multisig

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 23: Update `multisig_test.go` and `multisig_validator_test.go`

**Files:**
- Modify: `devnet/tests/evmigration/multisig_test.go`
- Modify: `devnet/tests/evmigration/multisig_validator_test.go`

- [ ] **Step 1: Rewrite the end-to-end flow**

Each test runs the four CLI commands against a devnet container (likely `lumera-supernova_validator_3` or equivalent):

1. `lumerad tx evmigration generate-proof-payload --legacy <A> --new-sub-pub-keys <k1,k2,k3> --new-threshold 2 --chain-id <id> --out /tmp/pp.json`
2. `lumerad tx evmigration sign-proof /tmp/pp.json --from legacy-sub-1 --new-key eth-sub-1 --out /tmp/pp-1.json`
3. Repeat for signer 2. (Optionally signer 3 as idempotency check.)
4. `lumerad tx evmigration combine-proof /tmp/pp-1.json /tmp/pp-2.json --out /tmp/tx.json`
5. `lumerad tx evmigration submit-proof /tmp/tx.json --chain-id <id>` (no `--from` — migration txs are unsigned; see Task 17 / design §4.5)

Assertions:
- `MigrationRecord` is set.
- New multisig bech32 has balance moved.
- `lumerad query auth account <newAddr>` returns `BaseAccount.PubKey` as `LegacyAminoPubKey` with 3 `ethsecp256k1.PubKey` sub-keys.
- Delegations re-keyed.
- Replay of `submit-proof` rejected (`ErrAlreadyMigrated`).

For `multisig_validator_test.go`, add the post-migration `MsgEditValidator` from the new multisig-of-eth operator. The devnet spike demonstrates this works; the test codifies it.

- [ ] **Step 2: Run and commit**

```bash
# Devnet test command — adapt to your existing devnet harness:
cd devnet && go test ./tests/evmigration/... -v -count=1 -timeout 15m && cd -
git add devnet/tests/evmigration/multisig_test.go devnet/tests/evmigration/multisig_validator_test.go
git commit -m "evmigration(devnet): multisig→multisig full flow incl. post-migration MsgEditValidator

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 24: Update `multisig_estimate_test.go`

**Files:**
- Modify: `devnet/tests/evmigration/multisig_estimate_test.go`

- [ ] **Step 1: Update expectations**

The `MigrationEstimate` response now carries `is_multisig`, `threshold`, `num_signers`. Expand the test to:

- Supported 2-of-3 multisig → `would_succeed=true`, correct threshold/num_signers.
- Multisig with N > `MaxMultisigSubKeys` → `would_succeed=false`, size-cap reason.
- Non-secp256k1 sub-key → rejected.
- Nested multisig → rejected.

- [ ] **Step 2: Run and commit**

```bash
cd devnet && go test ./tests/evmigration/... -v -run TestMigrationEstimate_Multisig -count=1 && cd -
git add devnet/tests/evmigration/multisig_estimate_test.go
git commit -m "evmigration(devnet): multisig MigrationEstimate coverage

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Phase 8 — Documentation

### Task 25: Update evm-integration docs (incl. critical supernode-migration.md rewrite)

**Files:**
- Modify: `docs/evm-integration/evmigration.md` (check exact filename under `docs/evm-integration/`; might be top-level `evmigration.md` plus `evmigration/portal-ui.md`)
- Modify: `docs/evm-integration/tests.md` — new rows under evmigration for multisig-to-multisig tests
- Modify: `docs/evm-integration/evmigration/portal-ui.md` — portal-UI implications for constructing the new multisig shape
- Modify: `docs/evm-integration/unit-evmigration.md`, `docs/evm-integration/integration-evmigration.md` — coverage summaries
- **Critical** — Modify: `docs/evm-integration/user-guides/supernode-migration.md` — described in detail below (Finding 3)

- [ ] **Step 1: Find all relevant docs**

Run: `find docs/evm-integration -type f -name "*.md" | xargs grep -l "evmigration\|legacy.*multisig\|legacy_pub_key\|legacy_signature" 2>/dev/null`

- [ ] **Step 2: Add a "Multisig account migration" section to the general `evmigration.md`**

Content to include:

- Mirror-source rule (single → single EOA; multisig → multisig-of-eth).
- Sub-key type constraints: legacy Cosmos secp256k1; new eth_secp256k1.
- Four-step CLI walkthrough with concrete commands.
- `PartialProof` JSON schema reference.
- Gotchas: co-signer must hold both their legacy and new keys; nil-pubkey legacy accounts require a pre-migration tx.

- **Migration order — FAQ (applies to every multisig migration type: balance-holding multisig via `MsgClaimLegacyAccount`, validator operator multisig via `MsgMigrateValidator`, and multisig-operated supernodes).** Verbatim Markdown to include:

    ```markdown
    ## Migration order — FAQ

    **Q: Do we need to migrate the multisig before its individual co-signers migrate their personal accounts? Or after?**

    A: **Any order works, including interleaved.** This holds uniformly for every multisig migration scenario — a balance-holding multisig, a validator operator multisig, and a multisig-operated supernode. Sub-signer and multisig migrations are mutually independent because:

    - The multisig's `LegacyAminoPubKey` — containing every sub-signer's 33-byte compressed pubkey and the threshold — is stored inline on the *multisig's* own `BaseAccount.PubKey`. Removing a sub-signer's individual account from x/auth (via their personal migration) does not touch this record.
    - Signing is an offline private-key operation. Each co-signer's `lumerad tx evmigration sign-proof --from <legacy-sub-key>` produces a signature from their local keyring. The keyring's private key exists independently of any chain state, so it continues to work after the sub-signer's personal account has been migrated.
    - The on-chain verifier reconstructs the multisig from pubkey bytes in the proof and verifies each sub-signature against the claimed sub-pubkey. It never consults x/auth about the sub-signers' individual account existence.

    **Precondition (unchanged):** the multisig's own `LegacyAminoPubKey` must already be on-chain — i.e., the multisig must have signed at least one transaction in the past. If the multisig received funds but never signed anything, submit any 1-ulume self-send from the multisig first so its pubkey gets recorded. This precondition is independent of sub-signer migration state.

    **Non-migrating sub-signers:** if a co-signer chooses never to migrate their own personal account, the multisig migration still succeeds as long as K of N co-signers participate in the sign-proof ceremony.

    **Implication for planning:** operators can migrate in whatever order is operationally simplest — e.g., every co-signer migrates their personal account on their own schedule, and the multisig migration happens whenever K of N can coordinate. There is no chain-level ordering constraint.

    This property applies to all three migration message types:
    - `MsgClaimLegacyAccount` (balance-holding multisig)
    - `MsgMigrateValidator` (validator operator multisig — `x/staking` delegations, `x/distribution` state, `x/supernode` records all key on the multisig bech32, not sub-signers)
    - `MsgClaimLegacyAccount` / `MsgMigrateValidator` for supernode-operator multisigs (the cleanup flow described in the supernode user guide keys on the multisig's on-chain pubkey, set by `MigrateAuth` per design §4.6)
    ```

- [ ] **Step 3: Update `tests.md`**

Add rows under "Unit Tests" and "Integration Tests" for every test added in Phases 2, 6, 7.

- [ ] **Step 4: Rewrite `docs/evm-integration/user-guides/supernode-migration.md` §Multisig (Finding 3)**

The existing multisig section (around [line 304](docs/evm-integration/user-guides/supernode-migration.md#L304)) describes the pre-revision single-EOA flow. It must be rewritten to match the dual-side protocol. Specific content changes:

1. **Rewrite step 1** — "Recover the new EVM key in the supernode keyring" → "Generate **N fresh `eth_secp256k1` sub-keys** on the supernode host (or have co-signers provide their eth pubkeys)". Example:

    ```bash
    # Each co-signer runs (on their own machine if preferred):
    lumerad keys add <op-name>-eth-<N> --key-type eth_secp256k1 \
      --keyring-backend <backend>

    # Coordinator derives the new multisig address from the three eth pubkeys:
    lumerad keys add <op-name>-msig-new \
      --multisig <op-name>-eth-1,<op-name>-eth-2,<op-name>-eth-3 \
      --multisig-threshold 2 \
      --keyring-backend <backend>

    # Query to confirm:
    lumerad keys show <op-name>-msig-new --address
    # lumera1...   <-- this is your new_address
    ```

2. **Rewrite step 2** — keep the "ensure legacy multisig pubkey is on-chain" guidance (it's unchanged).

3. **Rewrite step 3 (generate-proof-payload)** — change `--new <single-eth-address>` to `--new <multisig-derived-bech32> --new-sub-pub-keys k1,k2,k3 --new-threshold 2`. Keep `--legacy` and `--chain-id`. Example:

    ```bash
    lumerad tx evmigration generate-proof-payload \
      --legacy <multisig-legacy-address> \
      --new <new-multisig-address-from-step-1> \
      --new-sub-pub-keys <op-name>-eth-1,<op-name>-eth-2,<op-name>-eth-3 \
      --new-threshold 2 \
      --kind claim \
      --chain-id <chain-id> \
      --out proof.json
    ```

4. **Rewrite step 4 (sign-proof)** — each co-signer now passes BOTH `--from <legacy-sub-key>` AND `--new-key <eth-sub-key>`. Example:

    ```bash
    lumerad tx evmigration sign-proof proof.json \
      --from <my-legacy-sub-key-name> \
      --new-key <my-eth-sub-key-name> \
      --keyring-backend <backend> \
      --chain-id <chain-id> \
      --out my-partial.json
    ```

    Call out: idempotent resign replaces the co-signer's prior entries on both sides, never duplicates.

5. **Rewrite step 5 (combine-proof)** — same command shape as before, but the doc should explicitly note that `combine-proof` now verifies every merged partial signature on both legacy and new sides, drops invalid entries with a stderr warning, and selects the K valid partials with the lowest ascending indices on each side. Update the "skip invalid entries, select the first K valid" language to say "on each side independently".

6. **Rewrite step 6 (submit-proof)** — the existing doc says "broadcast using the new EVM key as the transaction signer". This is wrong under the revised design. Rewrite to: "`submit-proof` broadcasts the pre-assembled tx **without signing at the Cosmos layer**. Migration messages declare zero signers (authorization is fully embedded in `legacy_proof` and `new_proof`), fees are waived by the evmigration ante handler, and replay is prevented by the keeper's `MigrationRecords.Has(legacyAddr)` check. There is no `--from` broadcaster key, no fee-payer, no envelope signature — `submit-proof` just loads `tx.json`, runs `ValidateBasic`, simulates gas via the migration-specific estimator, builds an unsigned tx, and broadcasts." Example:

    ```bash
    lumerad tx evmigration submit-proof tx.json \
      --from <some-funded-ops-key> \
      --chain-id <chain-id> \
      --keyring-backend <backend>
    ```

7. **Update the daemon's error-message template** (at the top of §Multisig). The existing template shows fabricated commands that don't match any real `lumerad` command shape (uses `assemble-proof`, stdout redirects, etc.). Rewrite the template to show the real four-step sequence using the real flag names (`--legacy`, `--new`, `--new-sub-pub-keys`, `--new-threshold`, `--new-key`, `--from`). The `submit-proof` step takes **no** `--from` — migration txs are unsigned at the Cosmos layer because the new EVM account doesn't exist yet (chicken-and-egg); `--chain-id` is the only flag besides the file path.

8. **Add a section** "Why the new operator is not an EVM-addressable address" — references Non-Goal §2 of the design doc: the new operator is a Cosmos SDK multisig bech32, not an Ethereum 20-byte address. It can perform ALL Cosmos-side validator and supernode operations but cannot originate `MsgEthereumTx`. Supernode operators who want EVM DeFi with their operator rewards should configure a separate withdraw address (single EOA) via `MsgSetWithdrawAddress`.

9. **Add a section** "Post-migration cleanup" — the daemon's idempotent cleanup path detects the on-chain multisig `BaseAccount.PubKey` (set by `MigrateAuth` per design §4.6), so cleanup works without the supernode needing to "know" that the new operator is a multisig. No workflow change from the operator's side beyond restart.

10. **Cross-reference the universal migration-order FAQ** — the order-independence property applies uniformly to all multisig migrations (balance-holding, validator, supernode), so the FAQ itself lives in the general `evmigration.md` per Step 2. In `supernode-migration.md`, add a short pointer at the end of the §Multisig section:

    ```markdown
    ### Migration order relative to sub-signer personal migrations

    Supernode operators whose operator key is a multisig often ask whether they need to coordinate their personal account migrations with the multisig's migration ceremony. They do not: sub-signer and multisig migrations are mutually independent. See the "Migration order — FAQ" in [evmigration.md](../evmigration.md#migration-order--faq) for the full explanation; the short version is that any order works, including interleaved, and a sub-signer's personal migration never affects the multisig's ability to migrate later.
    ```

    This keeps the supernode guide focused on supernode-specific ceremony and avoids duplicating content that applies universally.

- [ ] **Step 5: Commit**

```bash
git add docs/evm-integration/
git commit -m "$(cat <<'EOF'
docs(evmigration): describe multisig→multisig migration flow

- Mirror-source rule for destination shape.
- Four-step CLI walkthrough with dual-side signing.
- PartialProof v1 JSON schema.
- Portal-UI implications for multisig construction.
- Test coverage updates.
- Supernode user guide: rewrite multisig section for the dual-side
  protocol (new eth sub-keys, --new-key signatures, multisig-derived
  new_address, Cosmos-SDK-not-EVM scoping, unsigned-at-the-Cosmos-layer rationale).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Final Verification

- [ ] **Step 1: Full unit test suite**

Run: `go test ./x/evmigration/... -v -count=1 2>&1 | tail -20`
Expected: all pass.

- [ ] **Step 2: Full integration test suite**

Run: `go test -tags='integration test' ./tests/integration/evmigration/... -v -count=1 -timeout 15m 2>&1 | tail -30`
Expected: all pass.

- [ ] **Step 3: Full EVM integration tests (regression)**

Run: `go test -tags='integration test' ./tests/integration/evm/... -v -count=1 -timeout 20m 2>&1 | tail -20`
Expected: no new failures relative to baseline on `evm` branch.

- [ ] **Step 4: Lint**

Run: `make lint`
Expected: 0 issues.

- [ ] **Step 5: Devnet smoke test**

Run the devnet fixture end-to-end (via the existing `make devnet-*` targets) and execute the multisig and validator tests from Phase 7.

---

## Self-Review

1. **Spec coverage:** every section in `docs/design/evmigration-multisig-design.md` maps to a task (§4.1→T1-T3; §4.2→T6-T8; §4.3→T5; §4.5→T13-T18; §4.6→T12; §4.7→T10-T11; §5→T9,T18-T24; §7 risks are mitigated by test coverage). ✓

2. **Placeholder scan — honest accounting:**
    - **Production-code blocks are concrete** (no `{ ... }` bodies) in: Tasks 1-8 (proto, sigverify package, ValidateBasic, VerifyMigrationProof, VerifyCosmosSecp256k1, VerifyEthSecp256k1), Task 12 (MigrateAuth + migrateAccount), Task 14 helpers (`b64encodeAll`, `resolveEthSubKey`, mutual-exclusion/shape-mirroring/key-reuse guards), Task 15 helpers (`legacySigningInput`, `newSigningInput`, `findSubKeyIndex`, `deriveSubKeyAddr`, `upsertSig`), Task 16 `buildProofFromPartial` with per-partial verification and K-selection.
    - **Test skeletons are deliberately sketched**, not fully concrete: most `TestCombineProof_*` / `TestSignProof_*` / `TestGenerateProofPayload_*` entries are single-line signatures with intent comments. A subset (the ones doing work that needs precise setup — `TestMigrateAuth_*`, `TestVerifyMigrationProof_NewSide_*`, `TestCombineProof_DropsInvalidPartial_*`) is fully written. The sketched tests rely on pattern-matching against the concrete ones — an engineer executing the plan should write them in the same shape (`initMockFixture` for keeper tests; network fixture for CLI tests) with table-style variation.
    - **Helpers in Task 19 (`buildLegacyMultisig`, `signLegacyMultisigProof`, etc.) are signatures only**. The implementations are straightforward (deterministic key material + SHA256/Keccak256 signing via the `sigverify` helpers), but the concrete bodies live at implementation time, not plan time.
    - **No TBD / TODO / "implement later" / "similar to Task N" markers** remain in the plan — the looseness above is test-stub-only and flagged here rather than hidden.

3. **Type consistency:** `MigrationProof`, `sigverify.SubKeyType{CosmosSecp256k1,EthSecp256k1}`, `Side{Legacy,New}`, `PartialProof`, `SideSpec`, `PartialSignature` — used consistently across tasks. Function names (`VerifyMigrationProof`, `verifySingleKeyProof`, `verifyMultisigProof`, `sigverify.VerifyCosmosSecp256k1`, `sigverify.VerifyEthSecp256k1`, `buildNewSingleProof`, `buildProofFromPartial`, `legacySigningInput`, `newSigningInput`, `findSubKeyIndex`, `upsertSig`) referenced consistently. Flag names match the existing CLI: `--legacy`, `--new`, `--legacy-key`; new flags `--new-key`, `--new-sub-pub-keys`, `--new-threshold` introduced in Task 14. ✓

4. **Commit-compilability:** every commit in every task leaves `go build ./... && make lint` green. Task 8 (delete unused helpers) explicitly flagged DEFERRED — must run after Task 11 to maintain this invariant. ✓
