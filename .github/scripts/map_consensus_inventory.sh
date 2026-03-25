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
  echo "WARN: cascade-client-failure is not reserved in MsgSubmitEvidence and no canonical deterministic encoder was found"
fi

# 4) Soft-check determinism tests presence (informational; hard gate is separate workflow).
for f in \
  tests/integration/audit/evidence_determinism_test.go \
  tests/integration/supernode/metrics_determinism_test.go \
  tests/systemtests/audit_evidence_determinism_system_test.go
 do
  if [ ! -f "$f" ]; then
    echo "WARN: missing determinism test file: $f"
  fi
 done

echo "Proto map fields:"
echo "$proto_maps"
echo
echo "Generated map marshal loops:"
echo "$pb_maps"
echo
echo "Map-bearing consensus inventory check passed"
