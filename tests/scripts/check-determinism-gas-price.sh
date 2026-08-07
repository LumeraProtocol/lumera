#!/usr/bin/env bash
# Local reproduction of the CI determinism-pipeline bank-send fee step.
#
# The CI job failed with:
#   gas prices too low, got: 0.002500000000000000ulume
#   required: 0.010986328125000000ulume
#
# because it used a flat `--fees 500ulume` (= 0.0025ulume/gas at 200k gas),
# which was fine at the old 0.0025 default base fee but is below the global
# minimum once the default was raised to 0.0125.
#
# This script proves the *derivation* used in the workflow fix resolves to a
# value that clears the floor, without needing the whole 6-node testnet.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

GAS_PRICES="$(grep -oP 'FeeMarketDefaultBaseFee\s*=\s*"\K[0-9.]+' config/evm.go)ulume"
if [ -z "${GAS_PRICES%ulume}" ]; then
  echo "FAIL: could not derive FeeMarketDefaultBaseFee from config/evm.go" >&2
  exit 1
fi

DERIVED="${GAS_PRICES%ulume}"
echo "derived GAS_PRICES = ${GAS_PRICES}"

# The observed required floor at the raised base fee, from the CI failure.
REQUIRED="0.010986328125"
OLD_FLAT_PER_GAS="0.0025"   # 500ulume / 200000 gas

python3 - "$DERIVED" "$REQUIRED" "$OLD_FLAT_PER_GAS" <<'PY'
import sys
derived, required, old = (float(x) for x in sys.argv[1:4])
print(f"derived   : {derived}")
print(f"required  : {required}")
print(f"old flat  : {old}")
ok = derived > required
print(f"derived clears floor : {ok}")
print(f"old flat cleared     : {old > required}")
if not ok:
    raise SystemExit("FAIL: derived gas price does not clear the observed floor")
if old > required:
    raise SystemExit("FAIL: control is vacuous — the old value should NOT clear the floor")
print("PASS: fix clears the floor and the regression control still reproduces")
PY
