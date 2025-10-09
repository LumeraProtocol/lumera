#!/bin/bash
# /root/scripts/network-maker-setup.sh
#
# Modes (env START_MODE):
#   run   (default)  Perform optional install, configure, fund nm-account if needed, and start network-maker.
#   wait              Only wait until lumerad RPC is ready AND supernode is up, then exit 0.
#
# This script is a no-op if:
# - /shared/release/network-maker is missing, OR
# - validators.json has "network-maker": false (or missing) for this MONIKER.
#
set -euo pipefail

START_MODE="${START_MODE:-run}"

# ----- env / paths -----
: "${MONIKER:?MONIKER environment variable must be set}"

SUPERNODE_INSTALL_WAIT_TIMEOUT=300
SHARED_DIR="/shared"
CFG_DIR="${SHARED_DIR}/config"
CFG_CHAIN="${CFG_DIR}/config.json"
CFG_VALS="${CFG_DIR}/validators.json"
RELEASE_DIR="${SHARED_DIR}/release"
STATUS_DIR="${SHARED_DIR}/status"
NODE_STATUS_DIR="${STATUS_DIR}/${MONIKER}"

# In-container standard ports (cosmos-sdk)
LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_RPC_ADDR="http://localhost:${LUMERA_RPC_PORT}"
SUPERNODE_PORT="${SUPERNODE_PORT:-4444}"
IP_ADDR="$(hostname -i | awk '{print $1}')"
SN_ENDPOINT="${IP_ADDR}:${SUPERNODE_PORT}"
DAEMON="${DAEMON:-lumerad}"
DAEMON_HOME="${DAEMON_HOME:-/root/.lumera}"

NM="network-maker"
NM_SRC_BIN="${RELEASE_DIR}/${NM}"
NM_DST_BIN="/usr/local/bin/${NM}"
NM_HOME="/root/.${NM}"
NM_FILES_DIR="/root/nm-files"
NM_FILES_DIR_SHARED="/shared/nm-files"
NM_LOG="${NM_LOG:-/root/logs/network-maker.log}"
NM_TEMPLATE="${RELEASE_DIR}/nm-config.toml"   # Your template in /shared/release (you said it's attached as config.toml)
NM_CONFIG="${NM_HOME}/config.toml"

NM_KEY_NAME="nm-account"
MNEMONIC_FILE="${NODE_STATUS_DIR}/nm_mnemonic"
NM_ADDR_FILE="${NODE_STATUS_DIR}/nm-address"
GENESIS_ADDR_FILE="${NODE_STATUS_DIR}/genesis-address"
SN_ADDR_FILE="${NODE_STATUS_DIR}/supernode-address"

mkdir -p "${NODE_STATUS_DIR}" "$(dirname "${NM_LOG}")" "${NM_HOME}"

# ----- tiny helpers -----
run() {
  echo "+ $*"
  "$@"
}

run_capture() {
  echo "+ $*" >&2   # goes to stderr, not captured
  "$@"
}

have() { command -v "$1" >/dev/null 2>&1; }
wait_for_file() { while [ ! -s "$1" ]; do sleep 1; done; }

fail_soft() { echo "[NM] $*"; exit 0; }   # exit 0 so container keeps running

# ----- prerequisites / config reads -----
have jq || echo "[NM] WARNING: jq is missing; attempting to proceed."

[ -f "${CFG_CHAIN}" ] || { echo "[NM] Missing ${CFG_CHAIN}"; exit 1; }
[ -f "${CFG_VALS}" ]  || { echo "[NM] Missing ${CFG_VALS}";  exit 1; }

# Pull global chain settings
CHAIN_ID="$(jq -r '.chain.id' "${CFG_CHAIN}")"
DENOM="$(jq -r '.chain.denom.bond' "${CFG_CHAIN}")"
KEYRING_BACKEND="$(jq -r '.daemon.keyring_backend' "${CFG_CHAIN}")"

# Pull this validator record + node ports + optional NM flag
VAL_REC_JSON="$(jq -c --arg m "$MONIKER" '[.[] | select(.moniker==$m)][0]' "${CFG_VALS}")"
[ -n "${VAL_REC_JSON}" ] && [ "${VAL_REC_JSON}" != "null" ] || { echo "[NM] Validator moniker ${MONIKER} not found in validators.json"; exit 1; }

NM_ENABLED="$(echo "${VAL_REC_JSON}" | jq -r 'try .["network-maker"] // "false"')"

# ----- short-circuits -----
if [ "${START_MODE}" = "wait" ]; then
  # Just wait until both lumerad RPC and supernode are reachable, then exit 0.
  :
else
  # In run mode, skip entirely if prereqs say "not applicable".
  if [ ! -f "${NM_SRC_BIN}" ]; then
    fail_soft "network-maker binary not found at ${NM_SRC_BIN}; skipping."
  fi
  if [ "${NM_ENABLED}" != "true" ]; then
    fail_soft "validators.json has \"network-maker\": false (or missing) for ${MONIKER}; skipping."
  fi
fi

# ----- start network-maker (idempotent) -----
start_network_maker() {
  if pgrep -x ${NM} >/dev/null 2>&1; then
    echo "[NM] network-maker already running; skipping start."
  else
    echo "[NM] Starting network-maker…"
    # If your binary uses a subcommand like "start", adjust below accordingly.
    run ${NM} >"${NM_LOG}" 2>&1 &
    echo "[NM] network-maker started; logging to ${NM_LOG}"
  fi
}

stop_network_maker_if_running() {
  if pgrep -x ${NM} >/dev/null 2>&1; then
    echo "[NM] Stopping network-maker…"
    pkill -x ${NM}
    echo "[NM] network-maker stopped."
  else
    echo "[NM] network-maker is not running."
  fi
}

# ----- waiters -----
# Add one directory to [scanner].directories in a TOML-ish/INI file using crudini.
# - Creates [scanner] if missing
# - Creates directories if missing -> ["<dir>"]
# - If exists: inserts "<dir>" once (no duplicates), preserving existing entries
add_dir_to_scanner() {
  local dir="$1"
  local cfg="$2"

  # Ensure file exists
  [ -f "$cfg" ] || { echo "[NM] add_dir_to_scanner: config '$cfg' not found"; return 1; }

  # Read current value (empty if not set)
  local current
  if ! current="$(crudini --get "$cfg" scanner directories 2>/dev/null)"; then
    current=""
  fi

  # If not present, set to ["dir"]
  if [ -z "$current" ]; then
    crudini --set "$cfg" scanner directories "[\"$dir\"]"
    return
  fi

  # If present but not a bracketed list, overwrite safely
  case "$current" in
    \[*\]) ;;  # looks like a [ ... ]
    *) crudini --set "$cfg" scanner directories "[\"$dir\"]"; return ;;
  esac

  # Extract inner list between the brackets
  local inner="${current#[}"
  inner="${inner%]}"

  # Normalize spaces around commas (optional; keeps things tidy)
  inner="$(printf '%s' "$inner" | sed 's/[[:space:]]*,[[:space:]]*/, /g;s/^[[:space:]]*//;s/[[:space:]]*$//')"

  # If already contains the dir (quoted), do nothing
  if printf '%s' "$inner" | grep -F -q "\"$dir\""; then
    return
  fi

  # Build new list: prepend by default
  local new_inner
  if [ -z "$inner" ]; then
    new_inner="\"$dir\""
  else
    new_inner="\"$dir\", $inner"
  fi

  crudini --set "$cfg" scanner directories "[${new_inner}]"
}

# Configure network-maker options
configure_nm() {
  local cfg="$NM_CONFIG"

  # ----- write config from template and patch values -----
  if [ ! -f "${NM_TEMPLATE}" ]; then
    echo "[NM] ERROR: Missing NM template: ${NM_TEMPLATE}"
    exit 1
  fi

  cp -f "${NM_TEMPLATE}" "$cfg"

  mkdir -p "${NM_FILES_DIR}" "${NM_FILES_DIR_SHARED}"
  add_dir_to_scanner "${NM_FILES_DIR}" "$cfg"
  add_dir_to_scanner "${NM_FILES_DIR_SHARED}" "$cfg"
  chmod a+w "${NM_FILES_DIR_SHARED}"

  echo "[NM] Scanner directories are configured to include: ${NM_FILES_DIR}, ${NM_FILES_DIR_SHARED}"

  echo "[NM] Configuring network-maker: $cfg"

  nm_address=""
  # read addresses from status dir
  if [ -f "${NM_ADDR_FILE}" ]; then
    nm_address=$(cat ${NM_ADDR_FILE})
  fi

  # lumera section
  crudini --set "$cfg" lumera grpc_addr     "\"localhost:${LUMERA_GRPC_PORT}\""
  crudini --set "$cfg" lumera rpchttp_addr  "\"$LUMERA_RPC_ADDR\""
  crudini --set "$cfg" lumera chain_id      "\"$CHAIN_ID\""
  crudini --set "$cfg" lumera denom         "\"$DENOM\""

  # keyring section
  crudini --set "$cfg" keyring backend      "\"$KEYRING_BACKEND\""
  [ -n "$nm_address" ] && crudini --set "$cfg" keyring local_address "\"$nm_address\""
}

# Wait for lumerad RPC to become available
wait_for_lumera() {
  echo "[NM] Waiting for lumerad RPC at ${LUMERA_RPC_ADDR}..."
  for i in $(seq 1 180); do
    if curl -sf "${LUMERA_RPC_ADDR}/status" >/dev/null 2>&1; then
      echo "[NM] lumerad RPC is up."
      return 0
    fi
    sleep 1
  done
  echo "[NM] lumerad RPC did not become ready in time."
  return 1
}

# Wait for supernode to become available
wait_for_supernode() {
  local ep="${SN_ENDPOINT}"
  local host="${ep%:*}"
  local port="${ep##*:}"
  local timeout="${SUPERNODE_INSTALL_WAIT_TIMEOUT:-300}"

  echo "[NM] Waiting ${timeout} secs for supernode on ${host}:${port}…"

  # Consider local-only process check if endpoint is on this machine
  local is_local=0
  case "$host" in
    127.0.0.1|localhost|"$IP_ADDR") is_local=1 ;;
  esac

  for i in $(seq 1 "$timeout"); do
    # If local endpoint, also accept presence of the process
    if [ "$is_local" -eq 1 ] && pgrep -x supernode >/dev/null 2>&1; then
      echo "[NM] supernode process detected."
      return 0
    fi

    # TCP check
    if (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; then
      exec 3>&-
      echo "[NM] supernode port ${port} at ${host} is reachable."
      return 0
    fi

    sleep 1
  done

  echo "[NM] supernode did not become ready in time (${timeout}s) at ${host}:${port}."
  return 1
}

# ----- optional network-maker install -----
install_network_maker_binary() {
  if [ ! -f "${NM_DST_BIN}" ]; then
    echo "[NM] Installing ${NM} binary..."
    run cp -f "${NM_SRC_BIN}" "${NM_DST_BIN}"
    run chmod +x "${NM_DST_BIN}"
  else
    if cmp -s "${NM_SRC_BIN}" "${NM_DST_BIN}"; then
      echo "[NM] ${NM} binary already up-to-date at ${NM_DST_BIN}; skipping install."
    else
      echo "[NM] Updating ${NM} binary at ${NM_DST_BIN}..."
      run cp -f "${NM_SRC_BIN}" "${NM_DST_BIN}"
      run chmod +x "${NM_DST_BIN}"
    fi
  fi
}

configure_nm_account() {
  # ----- create/fund nm-account if needed -----
  if [ ! -f "${GENESIS_ADDR_FILE}" ]; then
    echo "[NM] ERROR: Missing ${GENESIS_ADDR_FILE} (created by validator-setup)."
    exit 1
  fi
  GENESIS_ADDR="$(cat "${GENESIS_ADDR_FILE}")"

  # Ensure key exists; prefer recovering from stored mnemonic
  if run ${DAEMON} keys show "${NM_KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1; then
    echo "[NM] Key ${NM_KEY_NAME} already exists."
  else
    if [ -s "${MNEMONIC_FILE}" ]; then
      echo "[NM] Recovering ${NM_KEY_NAME} from saved mnemonic."
      (cat "${MNEMONIC_FILE}") | run ${DAEMON} keys add "${NM_KEY_NAME}" --recover --keyring-backend "${KEYRING_BACKEND}" >/dev/null
    else
      echo "[NM] Creating new key ${NM_KEY_NAME}…"
      MNEMONIC_JSON="$(run_capture ${DAEMON} keys add "${NM_KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" --output json)"
      echo "${MNEMONIC_JSON}" | jq -r .mnemonic > "${MNEMONIC_FILE}"
    fi
    sleep 5
  fi

  NM_ADDR="$(run_capture ${DAEMON} keys show "${NM_KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
  echo "${NM_ADDR}" > "${NM_ADDR_FILE}"
  echo "[NM] nm-account address: ${NM_ADDR}"

  # Check balance; if 0, fund from genesis
  BAL_JSON="$(run_capture ${DAEMON} q bank balances "${NM_ADDR}" --output json)"
  BAL="$(echo "${BAL_JSON}" | jq -r --arg d "${DENOM}" '([.balances[]? | select(.denom==$d) | .amount] | first) // "0"')"
  [[ -z "${BAL}" ]] && BAL="0"
  echo "[NM] Current nm-account balance: ${BAL}${DENOM}"

  if (( BAL == 0 )); then
    # need this sleep to avoid "account sequence mismatch" error
    sleep 5

    echo "[NM] Funding nm-account from genesis address ${GENESIS_ADDR}…"
    SEND_JSON="$(run_capture ${DAEMON} tx bank send "${GENESIS_ADDR}" "${NM_ADDR}" "10000000${DENOM}" \
        --chain-id "${CHAIN_ID}" --keyring-backend "${KEYRING_BACKEND}" \
        --gas auto --gas-adjustment 1.3 --fees "3000${DENOM}" \
        --yes --output json)"
    TXHASH="$(echo "${SEND_JSON}" | jq -r .txhash)"

    if [ -n "${TXHASH}" ] && [ "${TXHASH}" != "null" ]; then
      echo "[NM] Waiting for funding tx ${TXHASH} to confirm…"

      sleep 51
      # Prefer WebSocket wait if available in your CLI; fallback to polling q tx
      if ! ${DAEMON} q wait-tx "${TXHASH}" --timeout 90s >/dev/null 2>&1; then
        # Poll fallback
        deadline=$((SECONDS+120))
        ok=0
        while (( SECONDS < deadline )); do
          out="$(${DAEMON} q tx "${TXHASH}" --output json 2>/dev/null || true)"
          if jq -e . >/dev/null 2>&1 <<<"$out"; then
            code="$(jq -r 'try .code // "0"' <<<"$out")"
            height="$(jq -r 'try .height // "0"' <<<"$out")"
            if [ "${height}" != "0" ] && [ "${code}" = "0" ]; then ok=1; break; fi
          fi
          sleep 5
        done
        [ "${ok}" = "1" ] || { echo "[NM] Funding tx failed or not found."; exit 1; }
      fi
    else
      echo "[NM] Could not obtain txhash for funding transaction"; exit 1
    fi
  else
    echo "[NM] nm-account already funded; skipping."
  fi
}


# If in wait mode, just wait and exit
if [ "${START_MODE}" = "wait" ]; then
  wait_for_lumera || exit 1
  wait_for_supernode || exit 1
  exit 0
fi

stop_network_maker_if_running
install_network_maker_binary
# ----- wait for chain & supernode readiness before config/funding/start -----
wait_for_lumera || fail_soft "Chain not ready; skipping NM."
wait_for_supernode || fail_soft "Supernode not ready; skipping NM."

configure_nm_account
configure_nm

start_network_maker
