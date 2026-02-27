# LEP-5 — Cascade Availability Commitment (Testing Guide)

> **Full design:** [LEP-5 Technical Specification](../new-feature-lep5.md)

LEP-5 introduces a **Cascade Availability Commitment** system that requires a finalizing SuperNode to prove possession of actual file data via BLAKE3-based Merkle proofs, closing the vulnerability where a malicious SuperNode could finalize actions and collect fees without ever storing anything.

---

## 1. Prepare the Devnet

```bash
# Build the chain binary and devnet artefacts
make devnet-build-default

# Compile the devnet Go test binaries
make devnet-tests-build

# Start the network (foreground — logs stream to terminal)
make devnet-up
```

Wait until all validators have produced blocks and supernodes have registered (watch the log output for `supernode-setup.sh` completion).

---

## 2. Verify Supernodes Are ACTIVE

From inside any validator container (or from the host if ports are forwarded):

```bash
docker exec lumera-supernova_validator_1 lumerad q supernode list-supernodes
```

All supernodes listed must show `state: "ACTIVE"` before running the LEP-5 tests.

---

## 3. Run the LEP-5 Tests

### 3.1 Happy-Path Test (valid Merkle proofs → action reaches DONE)

```bash
LUMERA_PRINT_MERKLE_TREE=true \
LUMERA_RPC_ADDR=http://localhost:26667 \
LUMERA_GRPC_ADDR=localhost:9090 \
  go test -v -run TestLEP5CascadeAvailabilityCommitment -timeout 20m \
  ./devnet/tests/validator/
```

### 3.2 Failure-Path Test (corrupt proofs → finalization rejected, action stays PENDING)

```bash
LUMERA_PRINT_MERKLE_TREE=true \
LUMERA_RPC_ADDR=http://localhost:26667 \
LUMERA_GRPC_ADDR=localhost:9090 \
  go test -v -run TestLEP5CascadeAvailabilityCommitmentFailure -timeout 20m \
  ./devnet/tests/validator/
```

### 3.3 Query Action Metadata (inspect on-chain state for any action)

```bash
LUMERA_GRPC_ADDR=localhost:9090 \
LUMERA_ACTION_ID=1 \
  go test -v -run TestLEP5QueryActionMetadata -timeout 2m \
  ./devnet/tests/validator/
```

---

## 4. Environment Variables Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `LUMERA_RPC_ADDR` | auto-probed | CometBFT RPC endpoint |
| `LUMERA_GRPC_ADDR` | `localhost:9090` | gRPC endpoint |
| `LUMERA_CHAIN_ID` | `lumera-devnet-1` | Chain ID |
| `LUMERA_DENOM` | `ulume` | Fee denomination |
| `LUMERA_PRINT_MERKLE_TREE` | *(unset)* | Set to `true` to print an ASCII Merkle tree diagram with challenge highlights |
| `LUMERA_ACTION_ID` | `1` | Action ID for the `TestLEP5QueryActionMetadata` helper |
| `LUMERA_SUPERNODE_MNEMONIC_FILE` | auto-probed | Path to the supernode mnemonic file |

---

## 5. What the Tests Verify

| Test | Verifies |
|------|----------|
| `TestLEP5CascadeAvailabilityCommitment` | Register action with `AvailabilityCommitment` → finalize with valid chunk Merkle proofs → action reaches **DONE** |
| `TestLEP5CascadeAvailabilityCommitmentFailure` | Register action → finalize with **corrupt** leaf hashes → tx rejected on-chain → action stays **PENDING** |
| `TestLEP5QueryActionMetadata` | Query and pretty-print full `CascadeMetadata` for a given action ID |
