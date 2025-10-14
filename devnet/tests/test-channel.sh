#!/usr/bin/env bash
set -euo pipefail

: "${CHANNEL_INFO_FILE:=/shared/status/hermes/channel_transfer.json}"
: "${PORT_ID:=transfer}"

log() {
  printf '[ibc-test] %s\n' "$1"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "Missing required command: $1"
    exit 1
  fi
}

require_cmd jq
require_cmd lumerad

if [ ! -s "${CHANNEL_INFO_FILE}" ]; then
  log "Channel info file ${CHANNEL_INFO_FILE} missing or empty"
  exit 1
fi

CHANNEL_ID="$(jq -r '.channel_id // empty' "${CHANNEL_INFO_FILE}")"
COUNTERPARTY_CHAIN="$(jq -r '.b_chain_id // empty' "${CHANNEL_INFO_FILE}")"

if [ -z "${CHANNEL_ID}" ]; then
  log "channel_id not found in ${CHANNEL_INFO_FILE}"
  exit 1
fi

log "Testing channel ${PORT_ID}/${CHANNEL_ID} (counterparty ${COUNTERPARTY_CHAIN:-unknown})"

# Query all channels (array) and pick the exact match by port+channel
CHANNELS_JSON="$(lumerad q ibc channel channels --output json)"
log "${CHANNELS_JSON}"
MATCH_JSON="$(printf '%s\n' "${CHANNELS_JSON}" \
  | jq -c --arg p "${PORT_ID}" --arg ch "${CHANNEL_ID}" \
      '.channels[]? | select(.port_id==$p and .channel_id==$ch)')"

if [ -z "${MATCH_JSON}" ]; then
  log "Channel ${PORT_ID}/${CHANNEL_ID} not found in 'lumerad q ibc channel channels' output"
  exit 1
fi

STATE="$(printf '%s\n' "${MATCH_JSON}" | jq -r '.state // empty')"

if [ "${STATE}" != "STATE_OPEN" ]; then
  log "Channel state is ${STATE}, expected STATE_OPEN"
  exit 1
fi
log "Channel state: ${STATE}"

CONNECTION_ID="$(printf '%s\n' "${MATCH_JSON}" | jq -r '.connection_hops[0]')"
if [ -z "${CONNECTION_ID}" ] || [ "${CONNECTION_ID}" = "null" ]; then
  log "Could not determine connection id from channel query"
  exit 1
fi
log "Connection ID: ${CONNECTION_ID}"

# Fetch the connection (singular subcommand)
CONNECTION_JSON="$(lumerad q ibc connection connections --output json)"
log "${CONNECTION_JSON}"
MATCH_JSON="$(printf '%s\n' "${CONNECTION_JSON}" \
    | jq -c --arg id "${CONNECTION_ID}" '
      .connections // [] 
      | ( map(select(.id==$id)) 
          + (if length==0 then map(select(.state=="STATE_OPEN")) else [] end)
        ) 
      | .[0] // empty
    '
)"

if [ -z "${MATCH_JSON}" ]; then
  log "Connection ${CONNECTION_ID} not found in 'lumerad q ibc connection connections' output"
  exit 1
fi

CONNECTION_STATE="$(printf '%s\n' "${MATCH_JSON}" | jq -r '.state // empty')"
CLIENT_ID="$(printf '%s\n' "${MATCH_JSON}" | jq -r '.client_id // empty')"
COUNTERPARTY_CLIENT_ID="$(printf '%s\n' "${MATCH_JSON}" | jq -r '.counterparty.client_id // empty')"
COUNTERPARTY_CONN_ID="$(printf '%s\n' "${MATCH_JSON}" | jq -r '.counterparty.connection_id // empty')"

log "Connection ${CONNECTION_ID} state: ${CONNECTION_STATE}"
log "Client ID: ${CLIENT_ID}, Counterparty: client=${COUNTERPARTY_CLIENT_ID}, connection=${COUNTERPARTY_CONN_ID}"

if [ "${CONNECTION_STATE}" != "STATE_OPEN" ]; then
  log "Connection state is ${CONNECTION_STATE}, expected STATE_OPEN"
  exit 1
fi

CLIENT_STATUS_JSON="$(lumerad q ibc client status "${CLIENT_ID}" --output json)"
log "${CLIENT_STATUS_JSON}"
CLIENT_STATUS="$(printf '%s\n' "${CLIENT_STATUS_JSON}" | jq -r '.status // empty')"

if [ "${CLIENT_STATUS}" != "Active" ]; then
  log "Client ${CLIENT_ID} status is ${CLIENT_STATUS:-unknown}, expected Active"
  exit 1
fi
log "Client status: ${CLIENT_STATUS}"

log "IBC channel ${PORT_ID}/${CHANNEL_ID} is open and healthy."

# --- Client-state for this channel (sanity + consistency with connection) ---
CS_JSON="$(lumerad q ibc channel client-state "${PORT_ID}" "${CHANNEL_ID}" --output json)"
log "${CS_JSON}"

# Extract fields robustly across SDK versions
CS_CLIENT_ID="$(printf '%s\n' "${CS_JSON}" \
  | jq -r '.identified_client_state.client_id // .client_id // empty')"

CS_TYPE="$(printf '%s\n' "${CS_JSON}" \
  | jq -r '.identified_client_state.client_state."@type" // .client_state."@type" // empty')"

CS_LATEST_HEIGHT="$(printf '%s\n' "${CS_JSON}" \
  | jq -r '(
        .identified_client_state.client_state.latest_height.revision_height
     // .client_state.latest_height.revision_height
     // "0"
  )')"

if [ -z "${CS_CLIENT_ID}" ] || [ "${CS_CLIENT_ID}" = "null" ]; then
  log "client-state query returned no client_id"
  exit 1
fi

# Must match the client we read from the connection earlier
if [ "${CS_CLIENT_ID}" != "${CLIENT_ID}" ]; then
  log "client-state client_id mismatch: got ${CS_CLIENT_ID}, expected ${CLIENT_ID}"
  exit 1
fi

# Basic liveness: latest height should be a positive integer
if ! [ "${CS_LATEST_HEIGHT}" -gt 0 ] 2>/dev/null; then
  log "client-state latest_height is not positive: ${CS_LATEST_HEIGHT}"
  exit 1
fi

# Optional: type sanity (if present)
if [ -n "${CS_TYPE}" ] && [ "${CS_TYPE}" != "null" ]; then
  log "client-state type: ${CS_TYPE}"
fi

log "Channel client-state OK (client_id=${CS_CLIENT_ID}, latest_height=${CS_LATEST_HEIGHT})."