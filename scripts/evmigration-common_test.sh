#!/usr/bin/env bash
# Unit tests for pure helpers in evmigration-common.sh. No network/lumerad.
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Source only the function definitions (the lib is a source-safe library).
# shellcheck source=./evmigration-common.sh disable=SC1091
source "${DIR}/evmigration-common.sh"
# The library sets -e; disable it for test function calls that may return non-zero.
set +e

fail=0
check() { # check <label> <got> <want>
  if [[ "$2" == "$3" ]]; then echo "ok: $1"; else echo "FAIL: $1 — got '$2' want '$3'"; fail=1; fi
}

check "base+per-record 0"    "$(migration_gas_for_records 0)"    "200000"
check "base+per-record 1597" "$(migration_gas_for_records 1597)" "11379000"
check "base+per-record 2500" "$(migration_gas_for_records 2500)" "17700000"

# gas_exceeds_block_limit: returns 0 (true) only when over a positive limit.
gas_exceeds_block_limit 30000000 25000000; check "30M>25M true" "$?" "0"
gas_exceeds_block_limit 11379000 25000000; check "11M>25M false" "$?" "1"
gas_exceeds_block_limit 99999999 -1;       check "unlimited(-1) false" "$?" "1"
gas_exceeds_block_limit 99999999 "";       check "empty limit false" "$?" "1"

exit "$fail"
