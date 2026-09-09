#!/usr/bin/env bash
set -euo pipefail

# Run govulncheck while allowing only explicitly reviewed baseline symbol-level
# findings that currently have no upstream fixed version. Any new reachable vuln
# ID, or any allowed vuln once a fixed version appears, fails the job.
module="${1:-.}"
shift || true

allowed_ids=(
  # golang.org/x/crypto/openpgp is unmaintained and pulled through the Cosmos SDK
  # keyring stack. There is no fixed version in the Go vuln DB yet; keep this
  # explicit so a future replacement/fix can remove the exception.
  GO-2026-5932

  # github.com/shamaton/msgpack/v2 DoS advisory is pulled through the Cosmos/EVM
  # dependency graph. No fixed version is published in the Go vuln DB yet.
  GO-2026-4740

  # github.com/pion/dtls/v2 random nonce/AES-GCM advisory is pulled through the
  # Cosmos/EVM dependency graph. No fixed version is published yet.
  GO-2026-4479

  # github.com/cosmos/evm v0.6.0 partial precompile state-write advisory has no
  # fixed version in the Go vuln DB yet. Upgrade when Cosmos EVM publishes a fixed path.
  GO-2025-3684

  # github.com/cosmos/cosmos-sdk advisory is present on current master and has no
  # fixed version in the Go vuln DB yet. Keep it explicit and remove once upstream fixes.
  GO-2024-2584
)

workdir="${RUNNER_TEMP:-/tmp}/govulncheck-${module//\//_}"
mkdir -p "$workdir"
out="$workdir/output.json"
text="$workdir/output.txt"
err="$workdir/output.err"

if [[ ! -d "$module" ]]; then
  echo "::error::govulncheck module path does not exist: $module" >&2
  exit 1
fi

set +e
(
  cd "$module"
  govulncheck -format=json "$@" ./... > "$out" 2> "$err"
)
status=$?
set -e

if (( status > 1 )); then
  cat "$err" >&2 || true
  echo "::error::govulncheck failed before producing vulnerability results (exit $status)" >&2
  exit "$status"
fi

# Keep a human-readable copy in the job log.
(
  cd "$module"
  govulncheck "$@" ./...
) > "$text" 2>&1 || true
cat "$text"

python3 - "$out" "${allowed_ids[@]}" <<'PY'
import json
import sys

path = sys.argv[1]
allowed = set(sys.argv[2:])
with open(path, encoding="utf-8") as fh:
    data = fh.read()

if not data.strip():
    print("::error::govulncheck produced empty JSON output", file=sys.stderr)
    sys.exit(1)

decoder = json.JSONDecoder()
pos = 0
seen = set()
fixed_versions = {}
while pos < len(data):
    while pos < len(data) and data[pos].isspace():
        pos += 1
    if pos >= len(data):
        break
    try:
        event, pos = decoder.raw_decode(data, pos)
    except json.JSONDecodeError as exc:
        print(f"::error::govulncheck produced invalid JSON at byte {exc.pos}: {exc.msg}", file=sys.stderr)
        sys.exit(1)
    finding = event.get("finding")
    if not isinstance(finding, dict):
        continue
    trace = finding.get("trace") or []
    # govulncheck emits module/package findings too; its default blocking policy
    # is symbol reachability. Symbol findings include at least one called
    # function in the trace, while module/package findings do not.
    if not any(isinstance(frame, dict) and frame.get("function") for frame in trace):
        continue
    osv = finding.get("osv")
    if isinstance(osv, str) and osv:
        seen.add(osv)
        fixed = finding.get("fixed_version")
        if fixed:
            fixed_versions[osv] = fixed

unexpected = sorted(seen - allowed)
actionable_allowed = sorted(v for v in seen & allowed if fixed_versions.get(v))
if unexpected:
    print("::error::govulncheck found non-baselined reachable vulnerability IDs: " + ", ".join(unexpected))
    sys.exit(1)
if actionable_allowed:
    print("::error::govulncheck allowlist contains vulnerability IDs that now have fixes: " + ", ".join(f"{v} fixed in {fixed_versions[v]}" for v in actionable_allowed))
    sys.exit(1)

if seen:
    print("govulncheck reachable baseline-only findings: " + ", ".join(sorted(seen)))
else:
    print("govulncheck found no reachable vulnerabilities")
PY
