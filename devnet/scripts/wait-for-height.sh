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
CONSECUTIVE_PENDING_POLLS=0
MAX_FAILURES_BEFORE_LOG_CHECK="${MAX_FAILURES_BEFORE_LOG_CHECK:-3}"

detect_upgrade_halt() {
	local logs
	logs="$(docker compose -f "${COMPOSE_FILE}" logs --tail=50 "${SERVICE}" 2>/dev/null || true)"
	if echo "${logs}" | grep -qE "UPGRADE.*NEEDED.*height.*${TARGET_HEIGHT}|UPGRADE.*NEEDED at height: ${TARGET_HEIGHT}"; then
		return 0
	fi
	return 1
}

echo -n "Waiting for height >= ${TARGET_HEIGHT} (service=${SERVICE}, timeout=${TIMEOUT_SECONDS}s): "

LAST_HEIGHT=""
while ((SECONDS < deadline)); do
	height="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
		lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // "0"' 2>/dev/null || echo "0")"

	if [[ "$height" =~ ^[0-9]+$ ]] && ((height >= TARGET_HEIGHT)); then
		echo "${height} ✓"
		exit 0
	fi

	CONSECUTIVE_PENDING_POLLS=$((CONSECUTIVE_PENDING_POLLS + 1))
	if ((CONSECUTIVE_PENDING_POLLS >= MAX_FAILURES_BEFORE_LOG_CHECK)) && detect_upgrade_halt; then
		echo ""
		echo "Node halted for upgrade at height ${TARGET_HEIGHT} (detected from container logs)."
		exit 0
	fi

	if [[ "$height" != "$LAST_HEIGHT" && "$height" =~ ^[0-9]+$ && "$height" != "0" ]]; then
		echo -n "${height}-"
		LAST_HEIGHT="$height"
	fi
	sleep "${INTERVAL}"
done

echo ""
echo "Timeout waiting for height ${TARGET_HEIGHT}." >&2
exit 1
