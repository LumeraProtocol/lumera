#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "Usage: $0 <release-name> <upgrade-height> <binaries-dir>"
  exit 1
fi

RELEASE_NAME="$1"
UPGRADE_HEIGHT="$2"
BINARIES_DIR="$3"

if ! [[ "${UPGRADE_HEIGHT}" =~ ^[0-9]+$ ]]; then
  echo "Upgrade height must be a positive integer. Got: ${UPGRADE_HEIGHT}" >&2
  exit 1
fi

if [[ ! -d "${BINARIES_DIR}" ]]; then
  echo "Binaries directory not found: ${BINARIES_DIR}" >&2
  exit 1
fi
BINARIES_DIR="$(cd "${BINARIES_DIR}" && pwd)"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEVNET_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DEVNET_ROOT}/docker-compose.yml"

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  echo "docker-compose.yml not found at ${COMPOSE_FILE}" >&2
  exit 1
fi

echo "Submitting software upgrade proposal for ${RELEASE_NAME} at height ${UPGRADE_HEIGHT}..."
"${SCRIPT_DIR}/submit_upgrade_proposal.sh" "${RELEASE_NAME}" "${UPGRADE_HEIGHT}"
"${SCRIPT_DIR}/submit_upgrade_proposal.sh" "${RELEASE_NAME}"

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
"${SCRIPT_DIR}/vote_all.sh" "${PROPOSAL_ID}"

echo "Waiting for chain to reach height ${UPGRADE_HEIGHT}..."
"${SCRIPT_DIR}/wait_for_height.sh" "${UPGRADE_HEIGHT}"

echo "Upgrading binaries from ${BINARIES_DIR}..."
"${SCRIPT_DIR}/upgrade_binaries.sh" "${BINARIES_DIR}"

echo "Upgrade to ${RELEASE_NAME} initiated successfully."
