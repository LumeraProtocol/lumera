#!/bin/bash
# /root/scripts/network-maker-setup.sh
#
# Network-maker setup and lifecycle script for Lumera devnet.
#
# Network-maker is a multi-account management service used for NFT/scanner
# operations. It runs on a single validator (typically validator_3, controlled
# by validators.json "network-maker" flag). It provides gRPC + HTTP APIs for
# managing accounts, scanning files, and submitting transactions.
#
# Modes (env START_MODE):
#   run   (default)  Install binary, create/fund accounts, configure, and start.
#   wait             Only wait until lumerad RPC + supernode are ready, then exit.
#
# This script is a no-op (exits 0) if:
#   - /shared/release/network-maker binary is missing, OR
#   - validators.json has "network-maker": false (or missing) for this MONIKER
#
# Dependencies (must complete before this script runs):
#   - validator-setup.sh → provides genesis-address file
#   - supernode-setup.sh → provides running supernode endpoint
#
# Environment:
#   MONIKER            - Validator moniker, set by docker-compose
#   START_MODE         - "run" (default) or "wait"
#   NM_GRPC_PORT       - gRPC listen port (default 50051)
#   NM_HTTP_PORT       - HTTP gateway port (default 8080)
#
set -euo pipefail

START_MODE="${START_MODE:-run}"

# ─── Paths & Constants ────────────────────────────────────────────────────────

: "${MONIKER:?MONIKER environment variable must be set}"

SUPERNODE_INSTALL_WAIT_TIMEOUT=300
SHARED_DIR="/shared"
CFG_DIR="${SHARED_DIR}/config"
CFG_CHAIN="${CFG_DIR}/config.json"
CFG_VALS="${CFG_DIR}/validators.json"
RELEASE_DIR="${SHARED_DIR}/release"
STATUS_DIR="${SHARED_DIR}/status"
NODE_STATUS_DIR="${STATUS_DIR}/${MONIKER}"

# Network ports (inside container)
LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_RPC_ADDR="http://localhost:${LUMERA_RPC_PORT}"
SUPERNODE_PORT="${SUPERNODE_PORT:-4444}"
IP_ADDR="$(hostname -i | awk '{print $1}')"
SN_ENDPOINT="${IP_ADDR}:${SUPERNODE_PORT}"
DAEMON="${DAEMON:-lumerad}"
DAEMON_HOME="${DAEMON_HOME:-/root/.lumera}"

# Network-maker binary and config paths
NM="network-maker"
NM_SRC_BIN="${RELEASE_DIR}/${NM}"       # Source: copied from host by configure.sh
NM_DST_BIN="/usr/local/bin/${NM}"       # Destination: installed location
NM_HOME="/root/.${NM}"                  # Runtime home directory
NM_FILES_DIR="/root/nm-files"           # Local scanner directory
NM_FILES_DIR_SHARED="/shared/nm-files"  # Shared scanner directory (across containers)
NM_LOG="${NM_LOG:-/root/logs/network-maker.log}"
NM_TEMPLATE="${RELEASE_DIR}/nm-config.toml"  # Config template from host
NM_CONFIG="${NM_HOME}/config.toml"           # Active config (patched from template)
NM_GRPC_PORT="${NM_GRPC_PORT:-50051}"
NM_HTTP_PORT="${NM_HTTP_PORT:-8080}"

# Account management — network-maker gets its own funded keyring accounts
# separate from the validator and supernode accounts.
NM_KEY_PREFIX="nm-account"
NM_MNEMONIC_FILE_BASE="${NODE_STATUS_DIR}/nm_mnemonic"
NM_ADDR_FILE="${NODE_STATUS_DIR}/nm-address"
GENESIS_ADDR_FILE="${NODE_STATUS_DIR}/genesis-address"  # Written by validator-setup.sh
SN_ADDR_FILE="${NODE_STATUS_DIR}/supernode-address"      # Written by supernode-setup.sh

# Arrays populated by configure_nm_accounts()
declare -a NM_ACCOUNT_KEY_NAMES=()
declare -a NM_ACCOUNT_ADDRESSES=()
declare -a NM_FUND_TX_HASHES=()

mkdir -p "${NODE_STATUS_DIR}" "$(dirname "${NM_LOG}")" "${NM_HOME}"

# ═════════════════════════════════════════════════════════════════════════════
# UTILITY FUNCTIONS
# ═════════════════════════════════════════════════════════════════════════════

run() {
	echo "+ $*" >&2
	"$@"
}

run_capture() {
	echo "+ $*" >&2 # goes to stderr, not captured
	"$@"
}

have() { command -v "$1" >/dev/null 2>&1; }
wait_for_file() { while [ ! -s "$1" ]; do sleep 1; done; }

# Exit with success (0) so the container keeps running even when NM is skipped
fail_soft() {
	echo "[NM] $*"
	exit 0
}

version_ge() {
	printf '%s\n' "$2" "$1" | sort -V | head -n1 | grep -q "^$2$"
}

# Fetch the latest block height from lumerad.
latest_block_height() {
	local status
	status="$(curl -sf "${LUMERA_RPC_ADDR}/status" 2>/dev/null || true)"
	local height
	height="$(jq -r 'try .result.sync_info.latest_block_height // "0"' <<<"${status}")"
	printf "%s" "${height:-0}"
}

wait_for_block_height_increase() {
	local prev_height="$1"
	local timeout="${SUPERNODE_INSTALL_WAIT_TIMEOUT:-300}"
	local elapsed=0

	while ((elapsed < timeout)); do
		local height
		height="$(latest_block_height)"
		if ((height > prev_height)); then
			return 0
		fi
		sleep 1
		((elapsed++))
	done
	echo "[NM] Timeout waiting for new block after height ${prev_height}." >&2
	exit 1
}

# Wait for a tx to be confirmed on-chain. Tries WebSocket-based wait-tx first,
# then falls back to polling `q tx` by hash.
wait_for_tx_confirmation() {
	local txhash="$1"
	if ! ${DAEMON} q wait-tx "${txhash}" --timeout 90s >/dev/null 2>&1; then
		local deadline ok out code height
		deadline=$((SECONDS + 120))
		ok=0
		while ((SECONDS < deadline)); do
			out="$(${DAEMON} q tx "${txhash}" --output json 2>/dev/null || true)"
			if jq -e . >/dev/null 2>&1 <<<"${out}"; then
				code="$(jq -r 'try .code // "0"' <<<"${out}")"
				height="$(jq -r 'try .height // "0"' <<<"${out}")"
				if [ "${height}" != "0" ] && [ "${code}" = "0" ]; then
					ok=1
					break
				fi
			fi
			sleep 5
		done
		[ "${ok}" = "1" ] || {
			echo "[NM] Funding tx ${txhash} failed or not found."
			exit 1
		}
	fi
}

# ─── Read Config ──────────────────────────────────────────────────────────────
have jq || echo "[NM] WARNING: jq is missing; attempting to proceed."

[ -f "${CFG_CHAIN}" ] || {
	echo "[NM] Missing ${CFG_CHAIN}"
	exit 1
}
[ -f "${CFG_VALS}" ] || {
	echo "[NM] Missing ${CFG_VALS}"
	exit 1
}

# Global chain settings from config.json
CHAIN_ID="$(jq -r '.chain.id' "${CFG_CHAIN}")"
DENOM="$(jq -r '.chain.denom.bond' "${CFG_CHAIN}")"
KEYRING_BACKEND="$(jq -r '.daemon.keyring_backend' "${CFG_CHAIN}")"
# Number of NM accounts to create (configurable in config.json → network-maker.max_accounts)
DEFAULT_NM_MAX_ACCOUNTS=1
NM_MAX_ACCOUNTS="${DEFAULT_NM_MAX_ACCOUNTS}"
NM_CFG_MAX_ACCOUNTS="$(jq -r 'try .["network-maker"].max_accounts // ""' "${CFG_CHAIN}")"
if [[ "${NM_CFG_MAX_ACCOUNTS}" =~ ^[0-9]+$ ]]; then
	if [ "${NM_CFG_MAX_ACCOUNTS}" -ge 1 ]; then
		NM_MAX_ACCOUNTS="${NM_CFG_MAX_ACCOUNTS}"
	else
		echo "[NM] max_accounts must be >=1; using default ${DEFAULT_NM_MAX_ACCOUNTS}"
	fi
fi
DEFAULT_NM_ACCOUNT_BALANCE="10000000${DENOM}"
NM_ACCOUNT_BALANCE="$(jq -r 'try .["network-maker"].account_balance // ""' "${CFG_CHAIN}")"
if [ -z "${NM_ACCOUNT_BALANCE}" ] || [ "${NM_ACCOUNT_BALANCE}" = "null" ]; then
	NM_ACCOUNT_BALANCE="${DEFAULT_NM_ACCOUNT_BALANCE}"
fi
if [[ "${NM_ACCOUNT_BALANCE}" =~ ^[0-9]+$ ]]; then
	NM_ACCOUNT_BALANCE="${NM_ACCOUNT_BALANCE}${DENOM}"
fi

# Load this validator's record and check if network-maker is enabled for it
VAL_REC_JSON="$(jq -c --arg m "$MONIKER" '[.[] | select(.moniker==$m)][0]' "${CFG_VALS}")"
[ -n "${VAL_REC_JSON}" ] && [ "${VAL_REC_JSON}" != "null" ] || {
	echo "[NM] Validator moniker ${MONIKER} not found in validators.json"
	exit 1
}

NM_ENABLED="$(echo "${VAL_REC_JSON}" | jq -r 'try .["network-maker"].enabled // .["network-maker"] // "false"')"
NM_GRPC_PORT="$(echo "${VAL_REC_JSON}" | jq -r 'try .["network-maker"].grpc_port // empty')"
NM_HTTP_PORT="$(echo "${VAL_REC_JSON}" | jq -r 'try .["network-maker"].http_port // empty')"
if [ -z "${NM_GRPC_PORT}" ] || [ "${NM_GRPC_PORT}" = "null" ]; then NM_GRPC_PORT="${NM_GRPC_PORT:-50051}"; fi
if [ -z "${NM_HTTP_PORT}" ] || [ "${NM_HTTP_PORT}" = "null" ]; then NM_HTTP_PORT="${NM_HTTP_PORT:-8080}"; fi

# ─── Short-Circuit Checks ─────────────────────────────────────────────────────
# Exit early if NM is not applicable for this validator.
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

# ═════════════════════════════════════════════════════════════════════════════
# PROCESS LIFECYCLE
# ═════════════════════════════════════════════════════════════════════════════

# Start network-maker as a background process (idempotent)
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

# ═════════════════════════════════════════════════════════════════════════════
# CONFIGURATION
# Patch the config template with runtime values (endpoints, accounts, paths).
# The config uses TOML format with INI-style sections edited via crudini.
# ═════════════════════════════════════════════════════════════════════════════

# Add a directory to [scanner].directories in the TOML config.
# Handles missing sections, non-list values, and duplicate prevention.
add_dir_to_scanner() {
	local dir="$1"
	local cfg="$2"

	# Ensure file exists
	[ -f "$cfg" ] || {
		echo "[NM] add_dir_to_scanner: config '$cfg' not found"
		return 1
	}

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
	\[*\]) ;; # looks like a [ ... ]
	*)
		crudini --set "$cfg" scanner directories "[\"$dir\"]"
		return
		;;
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

# Build the active config from the template, then patch in runtime values:
# - Chain connection (gRPC, RPC, chain ID, denom)
# - Network-maker listen addresses (gRPC + HTTP gateway)
# - Keyring settings and account list
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

	# lumera section
	crudini --set "$cfg" lumera grpc_endpoint "\"localhost:${LUMERA_GRPC_PORT}\""
	crudini --set "$cfg" lumera rpc_endpoint "\"$LUMERA_RPC_ADDR\""
	crudini --set "$cfg" lumera chain_id "\"$CHAIN_ID\""
	crudini --set "$cfg" lumera denom "\"$DENOM\""

	# monitor (grpc/http) listeners
	crudini --set "$cfg" network-maker grpc_listen "\"0.0.0.0:${NM_GRPC_PORT}\""
	crudini --set "$cfg" network-maker http_gateway_listen "\"0.0.0.0:${NM_HTTP_PORT}\""

	# keyring section
	crudini --set "$cfg" keyring backend "\"$KEYRING_BACKEND\""
	crudini --set "$cfg" keyring dir "\"${DAEMON_HOME}\""

	update_nm_keyring_accounts "$cfg"
}

# Write [[keyring.accounts]] TOML array entries into the config.
# First strips any existing [[keyring.accounts]] blocks, then appends fresh ones.
update_nm_keyring_accounts() {
	local cfg="$1"
	local total_accounts="${#NM_ACCOUNT_KEY_NAMES[@]}"
	if [ "${total_accounts}" -eq 0 ]; then
		echo "[NM] WARNING: No network-maker accounts available to write into ${cfg}"
		return
	fi

	local tmp_cfg
	tmp_cfg="$(mktemp)"
	awk '
    /^[[:space:]]*\[\[keyring\.accounts\]\]/ { skip=1; next }
    {
      if (skip) {
        if ($0 ~ /^[[:space:]]*\[/) {
          if ($0 ~ /^[[:space:]]*\[\[keyring\.accounts\]\]/) {
            next
          }
          skip=0
        } else {
          next
        }
      }
      print
    }
  ' "${cfg}" >"${tmp_cfg}"
	mv "${tmp_cfg}" "${cfg}"

	local idx
	{
		echo ""
		for idx in "${!NM_ACCOUNT_KEY_NAMES[@]}"; do
			printf '[[keyring.accounts]]\nkey_name = "%s"\naddress  = "%s"\n\n' \
				"${NM_ACCOUNT_KEY_NAMES[$idx]}" "${NM_ACCOUNT_ADDRESSES[$idx]}"
		done
	} >>"${cfg}"

	echo "[NM] Configured ${total_accounts} network-maker account(s) in ${cfg}"
}

# ═════════════════════════════════════════════════════════════════════════════
# CHAIN & SUPERNODE READINESS WAITERS
# Network-maker depends on both lumerad (for tx submission) and supernode
# (for task coordination). Both must be up before NM can start.
# ═════════════════════════════════════════════════════════════════════════════

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

# Wait for supernode to become reachable. Checks both process presence
# (for local endpoints) and TCP port reachability.
wait_for_supernode() {
	local ep="${SN_ENDPOINT}"
	local host="${ep%:*}"
	local port="${ep##*:}"
	local timeout="${SUPERNODE_INSTALL_WAIT_TIMEOUT:-300}"

	echo "[NM] Waiting ${timeout} secs for supernode on ${host}:${port}…"

	# Consider local-only process check if endpoint is on this machine
	local is_local=0
	case "$host" in
	127.0.0.1 | localhost | "$IP_ADDR") is_local=1 ;;
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

# ═════════════════════════════════════════════════════════════════════════════
# BINARY INSTALLATION
# ═════════════════════════════════════════════════════════════════════════════

# Copy NM binary from shared release dir to /usr/local/bin/ (idempotent)
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

# ═════════════════════════════════════════════════════════════════════════════
# ACCOUNT MANAGEMENT
# Create NM_MAX_ACCOUNTS keyring keys (nm-account, nm-account-2, etc.),
# fund each from the validator's genesis account. Keys are persisted via
# mnemonic files in /shared/status/<moniker>/ for recovery across restarts.
# ═════════════════════════════════════════════════════════════════════════════

# Ensure a keyring key exists: recover from mnemonic file, or generate new.
# Returns the bech32 address on stdout.
ensure_nm_key() {
	local key_name="$1"
	local mnemonic_file="$2"

	if run ${DAEMON} keys show "${key_name}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1; then
		echo "[NM] Key ${key_name} already exists." >&2
	else
		if [ -s "${mnemonic_file}" ]; then
			echo "[NM] Recovering ${key_name} from saved mnemonic." >&2
			(cat "${mnemonic_file}") | run ${DAEMON} keys add "${key_name}" --recover --keyring-backend "${KEYRING_BACKEND}" >/dev/null
		else
			echo "[NM] Creating new key ${key_name}…" >&2
			local mnemonic_json
			mnemonic_json="$(run_capture ${DAEMON} keys add "${key_name}" --keyring-backend "${KEYRING_BACKEND}" --output json)"
			echo "${mnemonic_json}" | jq -r .mnemonic >"${mnemonic_file}"
		fi
		sleep 5
	fi

	local addr
	addr="$(run_capture ${DAEMON} keys show "${key_name}" -a --keyring-backend "${KEYRING_BACKEND}")"
	printf "%s" "${addr}"
}

# Fund an NM account if its balance is zero. Returns the txhash on stdout
# (empty string if already funded).
fund_nm_account_if_needed() {
	local key_name="$1"
	local account_addr="$2"
	local genesis_addr="$3"

	local bal_json bal
	bal_json="$(run_capture ${DAEMON} q bank balances "${account_addr}" --output json)"
	bal="$(echo "${bal_json}" | jq -r --arg d "${DENOM}" '([.balances[]? | select(.denom==$d) | .amount] | first) // "0"')"
	[[ -z "${bal}" ]] && bal="0"
	echo "[NM] Current ${key_name} balance: ${bal}${DENOM}" >&2

	if ((bal == 0)); then
		sleep 5
		echo "[NM] Funding ${key_name} with ${NM_ACCOUNT_BALANCE} from genesis address ${genesis_addr}…" >&2
		local send_json txhash
		send_json="$(run_capture ${DAEMON} tx bank send "${genesis_addr}" "${account_addr}" "${NM_ACCOUNT_BALANCE}" \
			--chain-id "${CHAIN_ID}" --keyring-backend "${KEYRING_BACKEND}" \
			--gas auto --gas-adjustment 1.3 --fees "3000${DENOM}" \
			--yes --output json)"
		txhash="$(echo "${send_json}" | jq -r .txhash)"

		if [ -n "${txhash}" ] && [ "${txhash}" != "null" ]; then
			printf "%s" "${txhash}"
		else
			echo "[NM] Could not obtain txhash for funding transaction" >&2
			exit 1
		fi
	else
		echo "[NM] ${key_name} already funded; skipping." >&2
		printf ""
	fi
}

# Fund all NM accounts sequentially. Waits for each block to avoid sequence
# number conflicts (each bank send must land in a different block).
fund_nm_accounts() {
	local genesis_addr="$1"
	local prev_height="$2"
	local total="${#NM_ACCOUNT_KEY_NAMES[@]}"
	local idx key_name account_addr fund_tx

	if [ "${total}" -eq 0 ]; then
		return
	fi

	for idx in $(seq 0 $((total - 1))); do
		key_name="${NM_ACCOUNT_KEY_NAMES[$idx]}"
		account_addr="${NM_ACCOUNT_ADDRESSES[$idx]}"
		fund_tx="$(fund_nm_account_if_needed "${key_name}" "${account_addr}" "${genesis_addr}")"
		if [ -n "${fund_tx}" ]; then
			NM_FUND_TX_HASHES+=("${fund_tx}")
			wait_for_block_height_increase "${prev_height}"
			prev_height="$(latest_block_height)"
		fi
	done

	if [ "${#NM_FUND_TX_HASHES[@]}" -gt 0 ]; then
		wait_for_all_funding_txs
	fi
}

wait_for_all_funding_txs() {
	local txhash
	for txhash in "${NM_FUND_TX_HASHES[@]}"; do
		echo "[NM] Waiting for funding tx ${txhash} to confirm…" >&2
		wait_for_tx_confirmation "${txhash}"
	done
}

# Create all NM accounts (keys + funding). Populates NM_ACCOUNT_KEY_NAMES
# and NM_ACCOUNT_ADDRESSES arrays used by configure_nm() to write config.
configure_nm_accounts() {
	if [ ! -f "${GENESIS_ADDR_FILE}" ]; then
		echo "[NM] ERROR: Missing ${GENESIS_ADDR_FILE} (created by validator-setup)."
		exit 1
	fi

	local genesis_addr
	genesis_addr="$(cat "${GENESIS_ADDR_FILE}")"

	NM_ACCOUNT_KEY_NAMES=()
	NM_ACCOUNT_ADDRESSES=()
	NM_FUND_TX_HASHES=()
	: >"${NM_ADDR_FILE}"

	local idx key_name mnemonic_file account_addr
	for idx in $(seq 1 "${NM_MAX_ACCOUNTS}"); do
		if [ "${idx}" -eq 1 ]; then
			key_name="${NM_KEY_PREFIX}"
			mnemonic_file="${NM_MNEMONIC_FILE_BASE}"
		else
			key_name="${NM_KEY_PREFIX}-${idx}"
			mnemonic_file="${NM_MNEMONIC_FILE_BASE}-${idx}"
		fi

		account_addr="$(ensure_nm_key "${key_name}" "${mnemonic_file}")"
		echo "[NM] ${key_name} address: ${account_addr}"

		NM_ACCOUNT_KEY_NAMES+=("${key_name}")
		NM_ACCOUNT_ADDRESSES+=("${account_addr}")
		printf "%s,%s\n" "${key_name}" "${account_addr}" >>"${NM_ADDR_FILE}"
	done

	local starting_height
	starting_height="$(latest_block_height)"
	fund_nm_accounts "${genesis_addr}" "${starting_height}"

	echo "[NM] Prepared ${#NM_ACCOUNT_KEY_NAMES[@]} network-maker account(s)."
}

# ═════════════════════════════════════════════════════════════════════════════
# MAIN EXECUTION
#
# Execution order:
#   1. Wait mode: just wait for lumerad + supernode, then exit
#   2. Run mode:
#      a. Stop any leftover NM process
#      b. Install binary from shared release dir
#      c. Wait for chain + supernode readiness
#      d. Create/fund NM accounts
#      e. Build config from template
#      f. Start NM process
# ═════════════════════════════════════════════════════════════════════════════

if [ "${START_MODE}" = "wait" ]; then
	wait_for_lumera || exit 1
	wait_for_supernode || exit 1
	exit 0
fi

stop_network_maker_if_running
install_network_maker_binary

# Both chain and supernode must be ready before we can fund accounts or start NM
wait_for_lumera || fail_soft "Chain not ready; skipping NM."
wait_for_supernode || fail_soft "Supernode not ready; skipping NM."

configure_nm_accounts    # Create keys + fund from genesis account
configure_nm             # Build config.toml from template + runtime values
start_network_maker      # Launch NM process in background
