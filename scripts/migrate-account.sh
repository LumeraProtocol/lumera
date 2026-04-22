#!/usr/bin/env bash
#
# Migrate a single-signature legacy account to its EVM-compatible counterpart.
# See docs/design/evmigration-scripts-design.md and
# docs/evm-integration/user-guides/migration.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh disable=SC1091
source "${SCRIPT_DIR}/evmigration-common.sh"

main() {
  # Populated in Task 10.
  return 0
}

main "$@"
