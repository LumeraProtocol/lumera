#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <target_height>"
  exit 1
fi

TARGET_HEIGHT="$1"
if ! [[ "$TARGET_HEIGHT" =~ ^[0-9]+$ ]]; then
  echo "Target height must be a positive integer. Got: $TARGET_HEIGHT" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEVNET_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DEVNET_ROOT}/docker-compose.yml"
SERVICE="${SERVICE_NAME:-supernova_validator_1}"
INTERVAL="${INTERVAL:-5}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-600}"

deadline=$((SECONDS + TIMEOUT_SECONDS))

echo "Waiting for block height >= ${TARGET_HEIGHT} (service=${SERVICE}, timeout=${TIMEOUT_SECONDS}s)..."

while (( SECONDS < deadline )); do
  height="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
    lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // "0"' 2>/dev/null || echo "0")"

  if [[ "$height" =~ ^[0-9]+$ ]] && (( height >= TARGET_HEIGHT )); then
    echo "Reached height ${height}."
    exit 0
  fi

  echo "Current height ${height}."
  sleep "${INTERVAL}"
done

echo "Timeout waiting for height ${TARGET_HEIGHT}." >&2
exit 1
