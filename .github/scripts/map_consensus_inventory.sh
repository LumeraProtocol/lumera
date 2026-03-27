#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

# 1) Find map fields in proto definitions.
proto_maps=$(grep -RIn "map<" proto/lumera || true)

# 2) Find map-bearing generated message hints in module types.
pb_maps=$(grep -RIn "for k := range m\." x/*/v1/types/*.pb.go || true)

# 3) Guard cascade-client-failure path: it must be either reserved in msg path OR canonicalized in keeper path.
reserved_in_msg="false"
if grep -q "EVIDENCE_TYPE_CASCADE_CLIENT_FAILURE" x/audit/v1/keeper/msg_submit_evidence.go; then
  reserved_in_msg="true"
fi

has_canonical_encoder="false"
if grep -q "marshalCascadeClientFailureEvidenceMetadataDeterministic" x/audit/v1/keeper/evidence.go; then
  has_canonical_encoder="true"
fi

if [ "$reserved_in_msg" != "true" ] && [ "$has_canonical_encoder" != "true" ]; then
  echo "ERROR: cascade-client-failure is not reserved in MsgSubmitEvidence and no canonical deterministic encoder was found"
  exit 1
fi

# 4) Determinism coverage checks.
# Keep these paths aligned with committed determinism suites.
for f in \
  tests/integration/bank/deterministic_test.go \
  tests/integration/staking/determinstic_test.go \
  tests/systemtests/supernode_metrics_test.go \
  tests/systemtests/supernode_metrics_staleness_test.go
 do
  if [ ! -f "$f" ]; then
    echo "WARN: missing determinism-related test file: $f"
  fi
 done

# Hard floor: repo must keep at least one deterministic integration test.
deterministic_integration_count=$(find tests/integration -type f -name '*determin*test.go' | wc -l | tr -d ' ')
if [ "$deterministic_integration_count" -lt 1 ]; then
  echo "ERROR: no deterministic integration tests found under tests/integration"
  exit 1
fi

echo "Proto map fields:"
echo "$proto_maps"
echo
echo "Generated map marshal loops:"
echo "$pb_maps"
echo
echo "Map-bearing consensus inventory check passed"
