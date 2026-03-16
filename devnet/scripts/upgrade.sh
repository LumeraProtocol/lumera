#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
	echo "Usage: $0 <release-name> <upgrade-height|auto-height> <binaries-dir>"
	exit 1
fi

RELEASE_NAME="$1"
REQUESTED_HEIGHT="$2"
BINARIES_DIR="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEVNET_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DEVNET_ROOT}/docker-compose.yml"
SERVICE="${SERVICE_NAME:-supernova_validator_1}"
AUTO_HEIGHT_OFFSET=100

if [[ ! -f "${COMPOSE_FILE}" ]]; then
	echo "docker-compose.yml not found at ${COMPOSE_FILE}" >&2
	exit 1
fi

if [[ ! -d "${BINARIES_DIR}" ]]; then
	echo "Binaries directory not found: ${BINARIES_DIR}" >&2
	exit 1
fi
BINARIES_DIR="$(cd "${BINARIES_DIR}" && pwd)"

# Detect if chain is already halted for this upgrade (re-run scenario).
# When the upgrade height is reached, nodes panic and stop serving RPC,
# so lumerad status fails. Check docker logs for the halt message.
detect_upgrade_halt() {
	local logs
	logs="$(docker compose -f "${COMPOSE_FILE}" logs --tail=100 "${SERVICE}" 2>/dev/null || true)"
	if echo "${logs}" | grep -qE "UPGRADE.*\"${RELEASE_NAME}\".*NEEDED"; then
		return 0
	fi
	return 1
}

# Check if the node is already running the target version (upgrade already completed).
normalize_version() {
	local v="${1:-}"
	v="${v#"${v%%[![:space:]]*}"}"
	v="${v%"${v##*[![:space:]]}"}"
	v="${v#v}"
	printf '%s\n' "${v}"
}

RUNNING_VERSION="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
	lumerad version 2>/dev/null | head -n 1 | tr -d '\r' || true)"
RUNNING_VERSION="$(normalize_version "${RUNNING_VERSION}")"
EXPECTED_VERSION="$(normalize_version "${RELEASE_NAME}")"

if [[ -n "${RUNNING_VERSION}" && "${RUNNING_VERSION}" == "${EXPECTED_VERSION}" ]]; then
	echo "Node is already running version ${RUNNING_VERSION}. Upgrade to ${RELEASE_NAME} already complete."
	exit 0
fi

if [[ "${REQUESTED_HEIGHT}" == "auto-height" ]]; then
	echo "Auto height requested. Determining current chain height from ${SERVICE}..."
	CURRENT_HEIGHT="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
		lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // empty' 2>/dev/null || true)"

	if ! [[ "${CURRENT_HEIGHT}" =~ ^[0-9]+$ ]]; then
		# Chain is not responding — check if it halted for our upgrade
		if detect_upgrade_halt; then
			echo "Chain is already halted for ${RELEASE_NAME} upgrade. Skipping to binary upgrade..."
			"${SCRIPT_DIR}/upgrade-binaries.sh" "${BINARIES_DIR}" "${RELEASE_NAME}"
			echo "Upgrade to ${RELEASE_NAME} initiated successfully."
			exit 0
		fi
		echo "Failed to determine current block height for service ${SERVICE}." >&2
		exit 1
	fi

	UPGRADE_HEIGHT=$((CURRENT_HEIGHT + AUTO_HEIGHT_OFFSET))
	echo "Current height is ${CURRENT_HEIGHT}. Scheduling upgrade at height ${UPGRADE_HEIGHT}."
else
	UPGRADE_HEIGHT="${REQUESTED_HEIGHT}"
fi

if ! [[ "${UPGRADE_HEIGHT}" =~ ^[0-9]+$ ]]; then
	echo "Upgrade height must be a positive integer. Got: ${UPGRADE_HEIGHT}" >&2
	exit 1
fi

echo "Submitting software upgrade proposal for ${RELEASE_NAME} at height ${UPGRADE_HEIGHT}..."
"${SCRIPT_DIR}/submit-upgrade-proposal.sh" "${RELEASE_NAME}" "${UPGRADE_HEIGHT}"
"${SCRIPT_DIR}/submit-upgrade-proposal.sh" "${RELEASE_NAME}"

echo "Retrieving proposal ID..."
PROPOSAL_ID="$(docker compose -f "${COMPOSE_FILE}" exec -T supernova_validator_1 \
	lumerad query gov proposals --output json | jq -r --arg name "${RELEASE_NAME}" '
    .proposals
    | map(select(.messages[]?.value.plan.name == $name))
    | sort_by(.id | tonumber)
    | last
    | .id // empty
  ')"

if [[ -z "${PROPOSAL_ID}" ]]; then
	echo "Failed to determine proposal ID for ${RELEASE_NAME}" >&2
	exit 1
fi

echo "Found proposal ID: ${PROPOSAL_ID}"

# Determine proposal status and planned height
PROPOSAL_JSON="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
	lumerad query gov proposal "${PROPOSAL_ID}" --output json 2>/dev/null || true)"
PROPOSAL_STATUS=""
PROPOSAL_HEIGHT=""
if [[ -n "${PROPOSAL_JSON}" ]]; then
	PROPOSAL_STATUS="$(echo "${PROPOSAL_JSON}" | jq -r '.proposal.status // .status // empty' 2>/dev/null || true)"
	PROPOSAL_HEIGHT="$(echo "${PROPOSAL_JSON}" | jq -r '.proposal.messages[]?.value.plan.height // empty' 2>/dev/null | head -n 1 || true)"
fi

if [[ -n "${PROPOSAL_HEIGHT}" && "${PROPOSAL_HEIGHT}" =~ ^[0-9]+$ ]]; then
	if [[ "${PROPOSAL_HEIGHT}" != "${UPGRADE_HEIGHT}" ]]; then
		echo "⚠️  Proposal height (${PROPOSAL_HEIGHT}) differs from requested height (${UPGRADE_HEIGHT})."
		echo "Using proposal height for wait/upgrade."
		UPGRADE_HEIGHT="${PROPOSAL_HEIGHT}"
	fi
else
	echo "⚠️  Could not determine proposal height; continuing with ${UPGRADE_HEIGHT}."
fi

if [[ "${PROPOSAL_STATUS}" == "PROPOSAL_STATUS_VOTING_PERIOD" ]]; then
	echo "Casting votes for all validators..."
	"${SCRIPT_DIR}/vote-all.sh" "${PROPOSAL_ID}"
else
	echo "ℹ️  Skipping voting; proposal status is ${PROPOSAL_STATUS:-unknown}."
fi

echo "Waiting for chain to reach height ${UPGRADE_HEIGHT}..."
CURRENT_HEIGHT_NOW="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
	lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // empty' 2>/dev/null || true)"
if [[ "${CURRENT_HEIGHT_NOW}" =~ ^[0-9]+$ ]] && ((CURRENT_HEIGHT_NOW >= UPGRADE_HEIGHT)); then
	echo "ℹ️  Current height ${CURRENT_HEIGHT_NOW} is already at or above upgrade height ${UPGRADE_HEIGHT}; skipping wait."
elif ! [[ "${CURRENT_HEIGHT_NOW}" =~ ^[0-9]+$ ]] && detect_upgrade_halt; then
	echo "ℹ️  Chain is already halted for ${RELEASE_NAME} upgrade; skipping wait."
else
	"${SCRIPT_DIR}/wait-for-height.sh" "${UPGRADE_HEIGHT}"
fi

echo "Upgrading binaries from ${BINARIES_DIR}..."
"${SCRIPT_DIR}/upgrade-binaries.sh" "${BINARIES_DIR}" "${RELEASE_NAME}"

echo "Upgrade to ${RELEASE_NAME} initiated successfully."
