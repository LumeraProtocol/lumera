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

if [[ "${REQUESTED_HEIGHT}" == "auto-height" ]]; then
  echo "Auto height requested. Determining current chain height from ${SERVICE}..."
  CURRENT_HEIGHT="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
    lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // empty' 2>/dev/null || true)"

  if ! [[ "${CURRENT_HEIGHT}" =~ ^[0-9]+$ ]]; then
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

if [[ ! -d "${BINARIES_DIR}" ]]; then
  echo "Binaries directory not found: ${BINARIES_DIR}" >&2
  exit 1
fi
BINARIES_DIR="$(cd "${BINARIES_DIR}" && pwd)"

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

echo "Casting votes for all validators..."
"${SCRIPT_DIR}/vote-all.sh" "${PROPOSAL_ID}"

echo "Waiting for chain to reach height ${UPGRADE_HEIGHT}..."
"${SCRIPT_DIR}/wait-for-height.sh" "${UPGRADE_HEIGHT}"

echo "Upgrading binaries from ${BINARIES_DIR}..."
"${SCRIPT_DIR}/upgrade-binaries.sh" "${BINARIES_DIR}"

echo "Upgrade to ${RELEASE_NAME} initiated successfully."
