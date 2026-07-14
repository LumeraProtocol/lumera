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
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/common.sh"
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

# duration_to_seconds converts a Go duration string as returned by the chain's
# gov params (e.g. "10m0s", "600s", "1h30m", "5m") into whole seconds. The chain
# returns durations in Go format, NOT plain seconds, so naive numeric stripping
# would read "10m0s" as 10. Unparseable input yields 0.
duration_to_seconds() {
	local d="${1:-}" total=0
	[[ "${d}" =~ ([0-9]+)h ]] && total=$(( total + BASH_REMATCH[1] * 3600 ))
	[[ "${d}" =~ ([0-9]+)m ]] && total=$(( total + BASH_REMATCH[1] * 60 ))
	[[ "${d}" =~ ([0-9]+)s ]] && total=$(( total + BASH_REMATCH[1] ))
	echo "${total}"
}

RUNNING_VERSION="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
	lumerad version 2>/dev/null | head -n 1 | tr -d '\r' || true)"
RUNNING_VERSION="$(normalize_version "${RUNNING_VERSION}")"
EXPECTED_VERSION="$(normalize_version "${RELEASE_NAME}")"

if [[ -n "${RUNNING_VERSION}" && "${RUNNING_VERSION}" == "${EXPECTED_VERSION}" ]]; then
	echo "Node is already running version ${RUNNING_VERSION}. Upgrade to ${RELEASE_NAME} already complete."
	exit 0
fi
if [[ -n "${RUNNING_VERSION}" ]] && versions_match "${EXPECTED_VERSION}" "${RUNNING_VERSION}"; then
	echo "Node is already running compatible version ${RUNNING_VERSION}. Upgrade to ${RELEASE_NAME} already complete."
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

	# The upgrade plan height MUST be reached only AFTER the proposal passes
	# (i.e. after the voting period ends). A fixed offset shorter than the voting
	# period makes the chain sail past the plan height while still voting, after
	# which x/upgrade cannot schedule an upgrade in the past — the upgrade simply
	# never executes. So derive the offset from the live voting period and the
	# measured block time, with a safety buffer.
	VOTING_DUR="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
		lumerad query gov params --output json 2>/dev/null \
		| jq -r '(.params.voting_period // .voting_params.voting_period // "600s")' 2>/dev/null)"
	VOTING_SECS="$(duration_to_seconds "${VOTING_DUR}")"
	[[ "${VOTING_SECS}" =~ ^[0-9]+$ ]] && (( VOTING_SECS > 0 )) || VOTING_SECS=600
	echo "Chain voting period: ${VOTING_DUR} (= ${VOTING_SECS}s)"
	# Measure seconds-per-block over a short sample.
	sleep 6
	H2="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
		lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // empty' 2>/dev/null || true)"
	[[ "${H2}" =~ ^[0-9]+$ ]] || H2="${CURRENT_HEIGHT}"
	DELTA=$(( H2 - CURRENT_HEIGHT )); (( DELTA >= 1 )) || DELTA=1
	BLOCK_SECS=$(( 6 / DELTA )); (( BLOCK_SECS >= 1 )) || BLOCK_SECS=1
	VOTING_BLOCKS=$(( VOTING_SECS / BLOCK_SECS ))
	# offset = voting period in blocks + buffer (default 60 blocks of headroom)
	DYNAMIC_OFFSET=$(( VOTING_BLOCKS + ${UPGRADE_HEIGHT_BUFFER_BLOCKS:-60} ))
	(( DYNAMIC_OFFSET > AUTO_HEIGHT_OFFSET )) && AUTO_HEIGHT_OFFSET="${DYNAMIC_OFFSET}"
	UPGRADE_HEIGHT=$(( H2 + AUTO_HEIGHT_OFFSET ))
	echo "Voting ~${VOTING_SECS}s (~${VOTING_BLOCKS} blocks @ ${BLOCK_SECS}s/blk); height ${H2}; scheduling upgrade at ${UPGRADE_HEIGHT} (offset ${AUTO_HEIGHT_OFFSET})."
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
    | map(select(
        any(.messages[]?; (.value.plan.name // .plan.name // empty) == $name)
        and ((.status == "PROPOSAL_STATUS_VOTING_PERIOD")
             or (.status == "PROPOSAL_STATUS_DEPOSIT_PERIOD")
             or (.status == "PROPOSAL_STATUS_PASSED"))
      ))
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
	PROPOSAL_HEIGHT="$(echo "${PROPOSAL_JSON}" | jq -r '.proposal.messages[]? | (.value.plan.height // .plan.height // empty)' 2>/dev/null | head -n 1 || true)"
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

# Wait for the chain to HALT for this upgrade. The upgrade fires at the plan
# height's begin-block, so the chain commits only up to (UPGRADE_HEIGHT - 1) and
# then panics "UPGRADE NEEDED" — it never commits UPGRADE_HEIGHT itself. So we
# poll for the halt marker (primary signal) rather than for height >=
# UPGRADE_HEIGHT (which would never be reached and would time out). We also break
# if the chain sails PAST the height without halting, leaving the guard below to
# refuse the swap.
echo "Waiting for the ${RELEASE_NAME} upgrade halt at height ${UPGRADE_HEIGHT}..."
UPGRADE_WAIT_TIMEOUT="${UPGRADE_WAIT_TIMEOUT:-1800}"
waited=0
while true; do
	if detect_upgrade_halt; then
		echo "✅ Chain halted for ${RELEASE_NAME} upgrade (committed ~$((UPGRADE_HEIGHT - 1)))."
		break
	fi
	CURRENT_HEIGHT_NOW="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
		lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // empty' 2>/dev/null || true)"
	if [[ "${CURRENT_HEIGHT_NOW}" =~ ^[0-9]+$ ]]; then
		if ((CURRENT_HEIGHT_NOW >= UPGRADE_HEIGHT)); then
			echo "⚠️  Chain at ${CURRENT_HEIGHT_NOW} passed ${UPGRADE_HEIGHT} without halting; proceeding to guard."
			break
		fi
		echo "  height ${CURRENT_HEIGHT_NOW}/${UPGRADE_HEIGHT} (waited ${waited}s)"
	else
		echo "  RPC unreachable; re-checking for upgrade-halt marker..."
	fi
	if ((waited >= UPGRADE_WAIT_TIMEOUT)); then
		echo "Timed out after ${UPGRADE_WAIT_TIMEOUT}s waiting for the ${RELEASE_NAME} upgrade halt." >&2
		break
	fi
	sleep 5
	waited=$((waited + 5))
done

# Guard: only swap binaries once the chain has actually HALTED for THIS upgrade.
# If the chain is still producing blocks past the upgrade height, the proposal
# did not pass / the upgrade did not execute. Swapping the binary now would start
# the new release on un-upgraded state and panic, e.g.:
#   "version of store feemarket mismatch ... new stores should be added using
#    StoreUpgrades"
# which crash-loops every validator. Refuse rather than brick the devnet.
if ! detect_upgrade_halt; then
	LIVE_HEIGHT="$(docker compose -f "${COMPOSE_FILE}" exec -T "${SERVICE}" \
		lumerad status 2>/dev/null | jq -r '.sync_info.latest_block_height // empty' 2>/dev/null || true)"
	if [[ "${LIVE_HEIGHT}" =~ ^[0-9]+$ ]]; then
		echo "ERROR: chain is still producing blocks (height ${LIVE_HEIGHT}) and has NOT halted for the ${RELEASE_NAME} upgrade." >&2
		echo "       The upgrade did not execute — the proposal likely failed to pass (quorum/threshold)." >&2
		echo "       Inspect: lumerad query gov proposal ${PROPOSAL_ID}" >&2
		echo "       Refusing to swap binaries; running ${RELEASE_NAME} on un-upgraded state would crash all nodes." >&2
		exit 1
	fi
	echo "⚠️  No upgrade-halt marker found and RPC is unreachable; assuming a genuine halt and proceeding." >&2
fi

echo "Upgrading binaries from ${BINARIES_DIR}..."
"${SCRIPT_DIR}/upgrade-binaries.sh" "${BINARIES_DIR}" "${RELEASE_NAME}"

echo "Upgrade to ${RELEASE_NAME} initiated successfully."
