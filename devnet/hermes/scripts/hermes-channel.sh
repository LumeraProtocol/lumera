#!/usr/bin/env bash
set -euo pipefail

: "${HERMES_CONFIG_PATH:=${HERMES_CONFIG:-/root/.hermes/config.toml}}"
: "${LUMERA_CHAIN_ID:=lumera-devnet-1}"
: "${SIMD_CHAIN_ID:=hermes-simd-1}"
: "${HERMES_KEY_NAME:=hermes-relayer}"
: "${RELAYER_MNEMONIC_FILE:=/shared/hermes/hermes-relayer.mnemonic}"
: "${LUMERA_MNEMONIC_FILE:=${RELAYER_MNEMONIC_FILE}}"
: "${SIMD_MNEMONIC_FILE:=${RELAYER_MNEMONIC_FILE}}"
: "${HERMES_STATUS_DIR:=/shared/status/hermes}"

ENTRY_LOG_FILE="${ENTRY_LOG_FILE:-/root/logs/entrypoint.log}"
LOG_PREFIX="[channel-setup]"

# Keep only the JSON part of each line (drop logger prefixes like "[ts] ... CMD output: { ... }")
extract_json() {
  # Strip anything before the first JSON brace/bracket, then:
  #  - keep objects: lines starting with '{'
  #  - keep arrays that start with an object: lines starting with '[{'
  #  - drop timestamp lines like "[2025-10-14T...]" which are NOT JSON
  sed 's/^[^{[]*//' | awk '
    /^[[:space:]]*{/ { print; next }
    /^[[:space:]]*\[\{/ { print; next }
  '
}

# Read Hermes output line-by-line and return the FIRST line that contains `"result":`
# as a single JSON object. If none found, return {}.
first_result_object() {
  local line
  line="$(extract_json | grep -m1 -E '"result"[[:space:]]*:' || true)"
  if [ -n "$line" ]; then
    printf '%s\n' "$line"
  else
    echo "{}"
  fi
}

# Return a normalized **array** from `.result`:
# - if .result is an array  -> return it
# - if .result is an object -> wrap it as [ .result ]
# - otherwise               -> []
result_items() {
  # Slurp all JSON docs, pick the LAST object that has "result",
  # then normalize result to an array: array -> itself; object -> [object]; else -> [].
  extract_json | jq -cs '
    def norm(x):
      if (x|type)=="array" then x
      elif (x|type)=="object" then [x]
      else [] end;

    # last object that has a "result" key
    ( [ .[] | select(type=="object" and has("result")) ]
      | if length>0 then .[length-1].result else [] end
    ) | norm(.)
  ' 2>/dev/null || echo "[]"
}

log_info() {
 local msg="$1"
  local ts line
  ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  line="[$ts] ${LOG_PREFIX} ${msg}"

  # 1) show on console even inside $( ... ): send to STDERR
  printf '%s\n' "$line" >&2

  # 2) optionally also write to a file if set
  if [ -n "${ENTRY_LOG_FILE:-}" ]; then
    # create dir if needed; ignore errors but don't crash
    mkdir -p "$(dirname -- "$ENTRY_LOG_FILE")" 2>/dev/null || true
    printf '%s\n' "$line" >> "$ENTRY_LOG_FILE" 2>/dev/null || true
  fi
}

fmt_cmd() {
  local out="" arg
  for arg in "$@"; do
    if [ -z "${out}" ]; then
      out=$(printf '%q' "${arg}")
    else
      out="${out} $(printf '%q' "${arg}")"
    fi
  done
  printf '%s' "${out}"
}

log_cmd_start() {
  log_info "CMD start: $(fmt_cmd "$@")"
}

log_cmd_result() {
  local rc="$1"
  shift
  if [ "${rc}" -eq 0 ]; then
    log_info "CMD success (rc=${rc}): $(fmt_cmd "$@")"
  else
    log_info "CMD failure (rc=${rc}): $(fmt_cmd "$@")"
  fi
}

log_cmd_output() {
  local label="$1"
  local payload="$2"
  local count=0
  if [ -z "${payload}" ]; then
    return 0
  fi
  while IFS= read -r line; do
    log_info "${label}: ${line}"
    count=$((count + 1))
    if [ "${count}" -ge 200 ]; then
      log_info "${label}: ... (truncated after 200 lines)"
      break
    fi
  done <<< "${payload}"
}

# Log when a Hermes JSON query returns zero items
log_query_empty() {
  local label="$1"
  local json_payload="$2"
  local count
  count="$(printf '%s\n' "${json_payload}" \
    | result_items \
    | jq 'length' 2>/dev/null || echo 0)"
  if [ "${count}" -eq 0 ]; then
    log "${label}: query returned 0 items"
  fi
}

ran() {
  local cmd=("$@")
  log_cmd_start "${cmd[@]}"
  "${cmd[@]}"
  local rc=$?
  log_cmd_result "${rc}" "${cmd[@]}"
  return "${rc}"
}

ran_capture() {
  local cmd=("$@")
  log_cmd_start "${cmd[@]}"
  local output rc
  if output=$("${cmd[@]}" 2>&1); then
    rc=0
  else
    rc=$?
  fi
  log_cmd_output "CMD output" "${output}"
  log_cmd_result "${rc}" "${cmd[@]}"
  printf '%s' "${output}"
  return "${rc}"
}

mkdir -p "${HERMES_STATUS_DIR}"
CHANNEL_INFO_FILE="${HERMES_STATUS_DIR}/channel_transfer.json"

log() {
  log_info "$1"
}

log_cmd() {
  log_cmd_start "$@"
}

run() {
  ran "$@"
}

run_capture() {
  ran_capture "$@"
}

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    log "jq is required but not installed in this container"
    exit 1
  fi
}

ensure_client() {
  local host_chain="$1"
  local reference_chain="$2"
  local client_id=""
  local query_json

  log "Querying existing clients on chain ${host_chain} referencing ${reference_chain}"
  query_json="$(run_capture hermes --json query clients --host-chain "${host_chain}" || true)"
  log_query_empty "clients (${host_chain})" "${query_json}"
  client_id="$(printf '%s\n' "${query_json}" \
    | result_items \
    | jq -r --arg ref "${reference_chain}" \
      'map(select(.chain_id==$ref)) | .[0].client_id // empty' || true)"
  if [ -n "${client_id}" ] && [ "${client_id}" != "null" ]; then
    log "Reusing existing client ${client_id} on ${host_chain} (counterparty ${reference_chain})"
    printf '%s\n' "${client_id}"
    return 0
  fi
  log "No matching client on ${host_chain} for counterparty ${reference_chain}; will create a new client"

  local create_json
  create_json="$(run_capture hermes --json create client --host-chain "${host_chain}" --reference-chain "${reference_chain}")"
  local result_line
  result_line="$(printf '%s\n' "${create_json}")"
  client_id="$(printf '%s' "${result_line}" \
    | result_items \
    | jq -r '.[0].CreateClient.client_id // empty' 2>/dev/null || true)"
  if [ -z "${client_id}" ] || [ "${client_id}" = "null" ]; then
    log "Failed to create client on ${host_chain} referencing ${reference_chain}"
    printf '%s\n' "${create_json}" | first_result_object | jq . >&2 || true
    exit 1
  fi
  log "Created client ${client_id} on ${host_chain} referencing ${reference_chain}"
  printf '%s\n' "${client_id}"
}

ensure_connection() {
  local a_chain="$1"
  local b_chain="$2"
  local a_client="$3"
  local b_client="$4"
  local connection_id=""

  local connections_json
  connections_json="$(run_capture hermes --json query connections --chain "${a_chain}" --counterparty-chain "${b_chain}" || true)"
  log_query_empty "connections (${a_chain})" "${connections_json}"
  connection_id="$(printf '%s\n' "${connections_json}" \
  | result_items \
  | jq -r '.[0] // empty' || true)"

  if [ -n "${connection_id}" ] && [ "${connection_id}" != "null" ]; then
    log "Reusing existing connection ${connection_id} on ${a_chain}"
    printf '%s\n' "${connection_id}"
    return 0
  fi

  log "No matching connection on ${a_chain} for clients (${a_client} <-> ${b_client}); will create a new connection"

  local create_conn_json
  create_conn_json="$(run_capture hermes --json create connection \
      --a-chain "${a_chain}" \
      --b-chain "${b_chain}" \
      --delay 0)"
  local conn_result_line
  conn_result_line="$(printf '%s\n' "${create_conn_json}")"
  connection_id="$(printf '%s' "${conn_result_line}" \
    | first_result_object \
    | jq -r --arg a_client "${a_client}" --arg b_client "${b_client}" \
       'select(.result.a_side.client_id==$a_client and .result.b_side.client_id==$b_client)
       | (.result.a_side.connection_id // .result.b_side.connection_id // empty)' \
       2>/dev/null || true)"

  if [ -z "${connection_id}" ] || [ "${connection_id}" = "null" ]; then
    local conn_err
    conn_err="$(printf '%s\n' "${conn_result_line}" \
      | first_result_object \
      | jq -r '.result // empty' || true)"
    log "Failed to create connection between ${a_chain} and ${b_chain}"
    if [ -n "${conn_err}" ] && [ "${conn_err}" != "null" ]; then
      log "Hermes error: ${conn_err}"
    fi
    printf '%s\n' "${create_conn_json}" >&2
    exit 1
  fi
  log "Created connection ${connection_id} (${a_chain} <-> ${b_chain})"
  printf '%s\n' "${connection_id}"
}

if command -v hermes >/dev/null 2>&1; then
  HERMES_VERSION_OUTPUT="$(hermes version 2>&1 || true)"
  log "Hermes CLI detected: ${HERMES_VERSION_OUTPUT}"
else
  log "Hermes CLI not found in PATH"
  exit 1
fi

log "Using Hermes config: ${HERMES_CONFIG_PATH}"
log "Lumera chain: ${LUMERA_CHAIN_ID}, SIMD chain: ${SIMD_CHAIN_ID}"

require_jq

if [ ! -s "${LUMERA_MNEMONIC_FILE}" ]; then
  log "Lumera mnemonic file ${LUMERA_MNEMONIC_FILE} missing"
  exit 1
fi

if [ ! -s "${SIMD_MNEMONIC_FILE}" ]; then
  log "SIMD mnemonic file ${SIMD_MNEMONIC_FILE} missing"
  exit 1
fi

if ! OUT="$(run_capture hermes keys add \
     --chain "${LUMERA_CHAIN_ID}" \
     --key-name "${HERMES_KEY_NAME}" \
     --mnemonic-file "${LUMERA_MNEMONIC_FILE}" \
     --overwrite 2>&1)"; then
  log "Failed to import Lumera key: ${OUT}"
  exit 1
fi
log "Imported Lumera key ${HERMES_KEY_NAME}"
lumera_relayer_addr="$(run_capture hermes --json keys list --chain "${LUMERA_CHAIN_ID}" \
  | jq -r ".result[]? | select(.name==\"${HERMES_KEY_NAME}\") | .address" || true)"
if [ -n "${lumera_relayer_addr}" ] && [ "${lumera_relayer_addr}" != "null" ]; then
  log "Lumera relayer address: ${lumera_relayer_addr}"
fi

if ! OUT="$(run_capture hermes keys add \
     --chain "${SIMD_CHAIN_ID}" \
     --key-name "${HERMES_KEY_NAME}" \
     --mnemonic-file "${SIMD_MNEMONIC_FILE}" \
     --overwrite 2>&1)"; then
  log "Failed to import SIMD key: ${OUT}"
  exit 1
fi
log "Imported SIMD key ${HERMES_KEY_NAME}"
simd_relayer_addr="$(run_capture hermes --json keys list --chain "${SIMD_CHAIN_ID}" \
  | jq -r ".result[]? | select(.name==\"${HERMES_KEY_NAME}\") | .address" || true)"
if [ -n "${simd_relayer_addr}" ] && [ "${simd_relayer_addr}" != "null" ]; then
  log "SIMD relayer address: ${simd_relayer_addr}"
fi

existing=""
log "Querying existing transfer channels on ${LUMERA_CHAIN_ID}"
channels_json="$(run_capture hermes --json query channels --chain "${LUMERA_CHAIN_ID}" --counterparty-chain "${SIMD_CHAIN_ID}" || true)"
log_query_empty "channels (${LUMERA_CHAIN_ID})" "${channels_json}"
existing="$(printf '%s\n' "${channels_json}" \
  | result_items \
  | jq -r 'map(select(.port_id=="transfer")) | .[0].channel_id // empty' || true)"
if [ -z "${existing}" ]; then
  log "No existing 'transfer' channel from ${LUMERA_CHAIN_ID} to ${SIMD_CHAIN_ID} found; will create"
fi

if [ -n "${existing}" ]; then
  log "Channel already exists: ${existing}"
  cat <<EOF >"${CHANNEL_INFO_FILE}"
{
  "channel_id": "${existing}",
  "port_id": "transfer",
  "counterparty_chain_id": "${SIMD_CHAIN_ID}",
  "last_updated": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF
  log "Wrote channel metadata to ${CHANNEL_INFO_FILE}"
  exit 0
fi

a_client_id="$(ensure_client "${LUMERA_CHAIN_ID}" "${SIMD_CHAIN_ID}")"
b_client_id="$(ensure_client "${SIMD_CHAIN_ID}" "${LUMERA_CHAIN_ID}")"
connection_id="$(ensure_connection "${LUMERA_CHAIN_ID}" "${SIMD_CHAIN_ID}" "${a_client_id}" "${b_client_id}")"

log "Creating transfer channel between ${LUMERA_CHAIN_ID} and ${SIMD_CHAIN_ID} using connection ${connection_id}"
if chan_out="$(run_capture hermes --json create channel \
     --a-chain "${LUMERA_CHAIN_ID}" \
     --a-connection "${connection_id}" \
     --a-port transfer --b-port transfer \
     --order unordered)"; then
     log "Hermes channel creation command completed"
else
  log "Channel creation command failed (non-zero exit code)"
  printf '%s\n' "$chan_out" >&2
  exit 1
fi

log "Hermes create channel raw output:" && echo "${chan_out}" || true
status_ok="$(printf '%s\n' "$chan_out" \
  | extract_json | grep -m1 -E '"result"[[:space:]]*:' \
  | jq -r '.status // "success"' 2>/dev/null || echo "error")"

if [ "$status_ok" != "success" ]; then
  log "Hermes returned non-success JSON status: $status_ok"
  printf '%s\n' "$chan_out" >&2
  exit 1
fi

new_channel_id="$(printf '%s\n' "$chan_out" \
  | first_result_object \
  | jq -r '.result.a_side.channel_id // .result.b_side.channel_id // empty' \
    2>/dev/null || true)"

if [ -z "${new_channel_id:-}" ]; then
  log "Unable to extract channel id from command output; polling existing channels..."
  for attempt in $(seq 1 10); do
    log "Polling attempt ${attempt}/10 on ${LUMERA_CHAIN_ID}"
    new_channel_id="$(run_capture hermes --json query channels --chain "${LUMERA_CHAIN_ID}" --counterparty-chain "${SIMD_CHAIN_ID}" \
      | result_items \
      | jq -r 'map(select(.port_id=="transfer")) |.[0].channel_id // empty' \
          2>/dev/null || true)"
    if [ -n "${new_channel_id}" ]; then
      break
    fi
    log "Polling attempt ${attempt}/10 on ${SIMD_CHAIN_ID}"
    new_channel_id="$(
      run_capture hermes --json query channels --chain "${SIMD_CHAIN_ID}" --counterparty-chain "${LUMERA_CHAIN_ID}" \
      | result_items \
      | jq -r 'map(select(.port_id=="transfer")) | .[0].channel_id // empty' \
          2>/dev/null || true
    )"
    if [ -n "${new_channel_id}" ]; then
      break
    fi
    sleep 3
  done
fi

if [ -z "${new_channel_id:-}" ]; then
  log "Channel creation command failed: could not determine new channel id"
  exit 1
fi
log "New channel detected: ${new_channel_id}"

cat <<EOF >"${CHANNEL_INFO_FILE}"
{
  "a_chain_id": "${LUMERA_CHAIN_ID}",
  "b_chain_id": "${SIMD_CHAIN_ID}",
  "a_client_id": "${a_client_id}",
  "b_client_id": "${b_client_id}",
  "channel_id": "${new_channel_id}",
  "port_id": "transfer",
  "last_updated": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF

log "Channel ${new_channel_id} created successfully; metadata stored at ${CHANNEL_INFO_FILE}"
