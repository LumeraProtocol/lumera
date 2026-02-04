#!/bin/bash
# /root/scripts/supernode-setup.sh
set -euo pipefail

# Require MONIKER env (compose already sets it)
if [ -z "${MONIKER:-}" ]; then
	echo "[SN] MONIKER is not set; skipping supernode setup."
	exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
	echo "[SN] jq is missing"
fi

if ! command -v curl >/dev/null 2>&1; then
	echo "[SN] curl is missing"
fi

DAEMON="lumerad"
CHAIN_ID="lumera-devnet-1"
KEYRING_BACKEND="test"
DENOM="ulume"

# In-container standard ports (cosmos-sdk)
LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_RPC_ADDR="http://localhost:${LUMERA_RPC_PORT}"

# Names & paths
KEY_NAME="${MONIKER}_key"
SN_BASEDIR="/root/.supernode"
SN_CONFIG="${SN_BASEDIR}/config.yml"
SN_PORT="${SUPERNODE_PORT:-4444}"
SN_P2P_PORT="${SUPERNODE_P2P_PORT:-4445}"
SN_GATEWAY_PORT="${SUPERNODE_GATEWAY_PORT:-8002}"
SN_LOG="${SN_LOG:-/root/logs/supernode.log}"

SHARED_DIR="/shared"
STATUS_DIR="${SHARED_DIR}/status"
RELEASE_DIR="${SHARED_DIR}/release"

# supernode
SN="supernode-linux-amd64"
SN_ALT="supernode"
SN_BIN_SRC="${RELEASE_DIR}/${SN}"
SN_BIN_SRC_ALT="${RELEASE_DIR}/${SN_ALT}"
SN_BIN_DST="/usr/local/bin/${SN}"
NODE_STATUS_DIR="${STATUS_DIR}/${MONIKER}"
SN_MNEMONIC_FILE="${NODE_STATUS_DIR}/sn_mnemonic"
SN_ADDR_FILE="${NODE_STATUS_DIR}/supernode-address"

IP_ADDR="$(hostname -i | awk '{print $1}')"

# sncli
SNCLI="sncli"
SNCLI_BASEDIR="/root/.sncli"
SNCLI_CFG_SRC="${RELEASE_DIR}/sncli-config.toml"
SNCLI_CFG="${SNCLI_BASEDIR}/config.toml"
SNCLI_BIN_SRC="${RELEASE_DIR}/${SNCLI}"
SNCLI_BIN_DST="/usr/local/bin/${SNCLI}"
SNCLI_MNEMONIC_FILE="${NODE_STATUS_DIR}/sncli_mnemonic"
SNCLI_ADDR_FILE="${NODE_STATUS_DIR}/sncli_address"
SNCLI_FUND_AMOUNT="100000" # in ulume
SNCLI_MIN_AMOUNT=10000
SNCLI_KEY_NAME="sncli-account"

if [[ "$KEY_NAME" == *validator* ]]; then
	SN_KEY_NAME="${KEY_NAME/validator/supernode}"
else
	SN_KEY_NAME="${KEY_NAME}_sn"
fi

run() {
	echo "+ $*"
	"$@"
}

run_capture() {
	echo "+ $*" >&2 # goes to stderr, not captured
	"$@"
}

require_crudini() {
	if ! command -v crudini >/dev/null 2>&1; then
		echo "[SN] ERROR: crudini not found. Please install it (e.g., apt-get update && apt-get install -y crudini) and re-run."
		return 1
	fi
}

# Wait for a transaction to be included in a block
wait_for_tx() {
	local txhash="$1"
	local timeout="${2:-90}"
	local interval="${3:-3}"

	if [[ -z "$txhash" ]]; then
		echo "[SN] wait_for_tx: missing tx hash"
		return 2
	fi

	echo "[SN] Waiting for tx $txhash (up to ${timeout}s) via WebSocket…"
	local wait_args=(q wait-tx "$txhash" --output json --timeout "${timeout}s")
	[[ -n "$LUMERA_RPC_ADDR" ]] && wait_args+=(--node "$LUMERA_RPC_ADDR")

	# Try WebSocket subscription first
	local out rc=0
	out="$($DAEMON "${wait_args[@]}" 2>&1)"
	rc=$?
	if [[ $rc -eq 0 ]] && jq -e . >/dev/null 2>&1 <<<"$out"; then
		local code height gas_used gas_wanted raw_log ts
		code=$(jq -r 'try .code // "null"' <<<"$out")
		height=$(jq -r 'try .height // "0"' <<<"$out")
		gas_used=$(jq -r 'try .gas_used // ""' <<<"$out")
		gas_wanted=$(jq -r 'try .gas_wanted // ""' <<<"$out")
		raw_log=$(jq -r 'try .raw_log // ""' <<<"$out")
		ts=$(jq -r 'try .timestamp // ""' <<<"$out")

		if [[ "$code" == "0" || "$code" == "null" ]]; then
			echo "[SN] Tx $txhash confirmed at height $height (gas $gas_used/$gas_wanted) $ts"
			return 0
		else
			echo "[SN] Tx $txhash FAILED at height $height: code=$code"
			[[ -n "$raw_log" ]] && echo "[SN] raw_log: $raw_log"
			return 1
		fi
	else
		echo "[SN] WebSocket wait failed/timeout; falling back to RPC polling…"
	fi

	# Fallback: poll q tx by hash (works even if indexer is null, once node surfaces it)
	local deadline=$((SECONDS + timeout))
	while ((SECONDS < deadline)); do
		local tx_args=(q tx "$txhash" --output json)
		[[ -n "$NODE" ]] && tx_args+=(--node "$NODE")

		out="$($DAEMON "${tx_args[@]}" 2>&1)" || true

		# If it's valid JSON, try to read fields; otherwise keep waiting on common "not found" cases
		if jq -e . >/dev/null 2>&1 <<<"$out"; then
			local height code codespace raw_log gas_used gas_wanted
			height=$(jq -r 'try .height // "0"' <<<"$out")
			code=$(jq -r 'try .code // "null"' <<<"$out")
			codespace=$(jq -r 'try .codespace // ""' <<<"$out")
			raw_log=$(jq -r 'try .raw_log // ""' <<<"$out")
			gas_used=$(jq -r 'try .gas_used // ""' <<<"$out")
			gas_wanted=$(jq -r 'try .gas_wanted // ""' <<<"$out")

			if [[ "$height" != "0" && "$height" != "null" ]]; then
				if [[ "$code" == "0" || "$code" == "null" ]]; then
					echo "[SN] Tx $txhash confirmed at height $height (gas $gas_used/$gas_wanted)"
					return 0
				else
					echo "[SN] Tx $txhash FAILED at height $height: code=$code codespace=${codespace:-N/A}"
					[[ -n "$raw_log" ]] && echo "[SN] raw_log: $raw_log"
					return 1
				fi
			fi
		else
			# Non-JSON or "not found" cases: keep polling
			# Typical texts: "tx (...) not found", RPC -32603, or empty while indexing.
			:
		fi

		sleep "$interval"
	done

	echo "[SN] Timeout: tx $txhash not found/committed after ${timeout}s."
	echo "[SN] Hints: ensure RPC reachable (set \$NODE), and node is not lagging."
	return 2
}

# Get current block height (integer), 0 if unknown
current_height() {
	curl -sf "${LUMERA_RPC_ADDR}/status" |
		jq -r '.result.sync_info.latest_block_height // "0"' 2>/dev/null |
		awk '{print ($1 ~ /^[0-9]+$/) ? $1 : 0}'
}

# Wait until height >= target (with timeout)
wait_for_height_at_least() {
	local target="$1"
	local retries="${2:-180}" # ~180s
	local delay="${3:-1}"

	echo "[SN] Waiting for block height >= ${target} ..."
	for ((i = 0; i < retries; i++)); do
		local h
		h="$(current_height)"
		if ((h >= target)); then
			echo "[SN] Height is ${h} (>= ${target}) — OK."
			return 0
		fi
		sleep "$delay"
	done
	echo "[SN] Timeout waiting for height >= ${target}."
	return 1
}

# Wait for N new blocks from the current height (default 5)
wait_for_n_blocks() {
	local n="${1:-5}"
	local start
	start="$(current_height)"
	local target=$((start + n))
	# If the chain hasn't started yet (start==0), still use +n (so target=n)
	((target < n)) && target="$n"
	wait_for_height_at_least "$target"
}

wait_for_lumera() {
	local rpc="${LUMERA_RPC_ADDR}/status"
	echo "[SN] Waiting for lumerad RPC at ${rpc}..."
	# Try up to 180s, 1s interval
	for i in $(seq 1 180); do
		if curl -sf "$rpc" >/dev/null 2>&1; then
			echo "[SN] lumerad RPC is up."
			return 0
		fi
		sleep 1
	done
	echo "[SN] lumerad RPC did not become ready in time."
	return 1
}

start_supernode() {
	# Ensure only one supernode process runs
	if pgrep -x ${SN} >/dev/null; then
		echo "[SN] Supernode already running, skipping start."
	else
		echo "[SN] Waiting for at least one new block before starting supernode..."
		wait_for_n_blocks 1 || {
			echo "[SN] Chain not progressing; cannot start supernode."
			return 1
		}
		echo "[SN] Starting supernode..."
		export P2P_USE_EXTERNAL_IP=false
		run ${SN} start -d "$SN_BASEDIR" >"$SN_LOG" 2>&1 &
		echo "[SN] Supernode started on ${SN_ENDPOINT}, logging to $SN_LOG"
	fi
}

stop_supernode_if_running() {
	if pgrep -x ${SN} >/dev/null; then
		echo "[SN] Stopping supernode..."
		run ${SN} stop -d "$SN_BASEDIR" >"$SN_LOG" 2>&1 &
		echo "[SN] Supernode stopped."
	else
		echo "[SN] Supernode is not running."
	fi
}

install_supernode_binary() {
	echo "[SN] Optional install: checking binaries at $SN_BIN_SRC or $SN_BIN_SRC_ALT"

	# 1) Pick source: prefer SN_BIN_SRC, else fallback to SN_BIN_SRC_ALT
	local src=""
	if [ -f "$SN_BIN_SRC" ]; then
		src="$SN_BIN_SRC"
	elif [ -f "$SN_BIN_SRC_ALT" ]; then
		src="$SN_BIN_SRC_ALT"
	else
		echo "[SN] supernode binary not found in either location; skipping."
		exit 0
	fi
	echo "[SN] Using source: $src"

	# 2) Install to fixed destination name: $SN_BIN_DST (/usr/local/bin/supernode-linux-amd64)
	if [ -f "$SN_BIN_DST" ]; then
		if cmp -s "$src" "$SN_BIN_DST"; then
			echo "[SN] supernode binary already installed and up-to-date."
		else
			echo "[SN] supernode binary is outdated; updating."
			run cp -f "$src" "$SN_BIN_DST"
			chmod +x "$SN_BIN_DST"
		fi
	else
		echo "[SN] Installing supernode binary..."
		run cp -f "$src" "$SN_BIN_DST"
		chmod +x "$SN_BIN_DST"
	fi

	# 3) Ensure /usr/local/bin/supernode -> supernode-linux-amd64 symlink
	local link="/usr/local/bin/supernode"
	if [ -e "$link" ] && [ ! -L "$link" ]; then
		echo "[SN] Found regular file at $link; removing to create symlink."
		rm -f "$link"
	fi

	# Create/update symlink; ensure it points to supernode-linux-amd64
	(
		cd /usr/local/bin || exit 1
		if [ -L "supernode" ]; then
			current_target="$(readlink supernode)"
			if [ "$current_target" != "${SN}" ]; then
				echo "[SN] Updating symlink supernode -> ${SN}"
				ln -sfn "${SN}" "supernode"
			else
				echo "[SN] Symlink supernode already points to ${SN}"
			fi
		else
			echo "[SN] Creating symlink supernode -> ${SN}"
			ln -sfn "${SN}" "supernode"
		fi
	)
}

register_supernode() {
	if is_sn_registered_active; then
		echo "[SN] Supernode is already registered and in ACTIVE state; no action needed."
	elif is_sn_blocked_state; then
		echo "[SN] Supernode is in ${SN_LAST_STATE} state; skipping registration."
	else
		echo "[SN] Registering supernode..."
		REG_TX_JSON="$(run_capture $DAEMON tx supernode register-supernode \
			"$VALOPER_ADDR" "$SN_ENDPOINT" "$SN_ADDR" \
			--from "$KEY_NAME" --chain-id "$CHAIN_ID" --keyring-backend "$KEYRING_BACKEND" \
			--gas auto --gas-adjustment 1.3 --fees "5000${DENOM}" -y --output json)"
		REG_TX_HASH="$(echo "$REG_TX_JSON" | jq -r .txhash)"
		if [[ -n "$REG_TX_HASH" && "$REG_TX_HASH" != "null" ]]; then
			wait_for_tx "$REG_TX_HASH" || {
				echo "[SN] Registration tx failed/timeout"
				exit 1
			}
		else
			echo "[SN] Failed to obtain txhash for registration"
			exit 1
		fi
		if is_sn_registered_active; then
			echo "[SN] Supernode registered successfully and is now ACTIVE."
		else
			echo "[SN] Supernode registration failed or not in ACTIVE state."
			exit 1
		fi
	fi
}

configure_supernode_p2p_listen() {
	local ip_addr="$1"
	local config_file="$SN_CONFIG"

	if [ -z "$ip_addr" ]; then
		echo "[SN] No IP address provided!"
		return 1
	fi
	if [ ! -f "$config_file" ]; then
		echo "[SN] config.yml not found at $config_file"
		return 1
	fi

	echo "[SN] Setting p2p.listen_address: ${ip_addr} in $config_file"

	# 1. Remove any existing listen_address lines inside the p2p: block
	sed -i '/^[[:space:]]*p2p:[[:space:]]*$/,/^[^[:space:]]/ { /^[[:space:]]*listen_address:[[:space:]]*/d }' "$config_file"

	# 2. Insert the new listen_address line after the "p2p:" line with 4-space indent
	sed -i '/^[[:space:]]*p2p:[[:space:]]*$/a\    listen_address: '"${ip_addr}" "$config_file"
}

configure_supernode() {
	echo "[SN] Ensuring SN key exists..."
	mkdir -p "$SN_BASEDIR" "${NODE_STATUS_DIR}"
	if [ -f "$SN_MNEMONIC_FILE" ]; then
		if ! run $DAEMON keys show "$SN_KEY_NAME" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1; then
			(cat "$SN_MNEMONIC_FILE") | run $DAEMON keys add "$SN_KEY_NAME" --recover --keyring-backend "$KEYRING_BACKEND" >/dev/null
		fi
	else
		run $DAEMON keys delete "$SN_KEY_NAME" --keyring-backend "$KEYRING_BACKEND" -y || true
		MNEMONIC_JSON="$(run_capture $DAEMON keys add "$SN_KEY_NAME" --keyring-backend "$KEYRING_BACKEND" --output json)"
		echo "[SN] Generated new supernode key: $MNEMONIC_JSON"
		echo "$MNEMONIC_JSON" | jq -r .mnemonic >"$SN_MNEMONIC_FILE"
	fi

	SN_ADDR="$(run_capture $DAEMON keys show "$SN_KEY_NAME" -a --keyring-backend "$KEYRING_BACKEND")"
	echo "[SN] Supernode address: $SN_ADDR"
	echo "$SN_ADDR" >"$SN_ADDR_FILE"
	VAL_ADDR="$(run_capture $DAEMON keys show "$KEY_NAME" -a --keyring-backend "$KEYRING_BACKEND")"
	echo "[SN] Validator address: $VAL_ADDR"
	VALOPER_ADDR="$(run_capture $DAEMON keys show "$KEY_NAME" --bech val -a --keyring-backend "$KEYRING_BACKEND")"
	echo "[SN] Validator operator address: $VALOPER_ADDR"

	GENESIS_ADDR="$(cat ${NODE_STATUS_DIR}/genesis-address)"
	echo "[SN] Genesis address: $GENESIS_ADDR"

	SN_ENDPOINT="${IP_ADDR}:${SN_PORT}"

	echo "[SN] Init config if missing..."
	if [ ! -f "$SN_BASEDIR/config.yml" ]; then
		run ${SN} init -y --force \
			--basedir "$SN_BASEDIR" \
			--keyring-backend "$KEYRING_BACKEND" \
			--key-name "$SN_KEY_NAME" \
			--supernode-addr "$IP_ADDR" \
			--supernode-port "$SN_PORT" \
			--recover \
			--mnemonic "$(cat "$SN_MNEMONIC_FILE")" \
			--lumera-grpc "localhost:${LUMERA_GRPC_PORT}" \
			--chain-id "$CHAIN_ID"

		printf "[SN] Generated config\n%s\n" "$(cat "$SN_CONFIG")"
		configure_supernode_p2p_listen "${IP_ADDR}"
	fi

	echo "[SN] Checking SN balance for $SN_ADDR..."
	BAL_JSON="$(run_capture $DAEMON q bank balances "$SN_ADDR" --output json)"
	echo "[SN] Balance output: $BAL_JSON"
	BAL="$(
		echo "$BAL_JSON" |
			jq -r --arg denom "$DENOM" '
        ([.balances[]? | select(.denom == $denom) | .amount] | first) // "0"
      '
	)"
	echo "[SN] Current SN balance: $BAL"
	# Normalize and compare numerically
	[[ -z "$BAL" ]] && BAL="0"
	if ((BAL < 1000000)); then
		echo "[SN] Funding Supernode account..."
		SEND_TX_JSON="$(run_capture $DAEMON tx bank send "$GENESIS_ADDR" "$SN_ADDR" "10000000${DENOM}" \
			--chain-id "$CHAIN_ID" \
			--keyring-backend "$KEYRING_BACKEND" \
			--gas auto \
			--gas-adjustment 1.3 \
			--fees "3000$DENOM" \
			--output json --yes)"
		echo "[SN] Send tx output: $SEND_TX_JSON"
		SEND_TX_HASH="$(echo "$SEND_TX_JSON" | jq -r .txhash)"
		if [ -n "$SEND_TX_HASH" ] && [ "$SEND_TX_HASH" != "null" ]; then
			if ! wait_for_tx "$SEND_TX_HASH"; then
				echo "[SN] Funding tx failed or not confirmed. Exiting."
				exit 1
			fi
		else
			echo "[SN] Failed to get TXHASH for funding transaction."
			exit 1
		fi
	fi
}

# Returns 0 if registered to SN_ADDR and last state is SUPERNODE_STATE_ACTIVE, else 1
is_sn_registered_active() {
	local info

	echo "[SN] Checking if supernode is registered..."
	info="$(run_capture $DAEMON q supernode get-supernode "$VALOPER_ADDR" --output json)"
	echo "[SN] Supernode info output: $info"

	# Extract the supernode account (empty string if missing)
	local acct
	acct="$(echo "$info" | jq -r '.supernode.supernode_account // ""')"

	# Extract the last state by highest height (empty string if none)
	# sort_by is safe even if heights are strings
	local last_state
	last_state="$(echo "$info" | jq -r '
      (.supernode.states // [])
      | sort_by(.height | tonumber)
      | (last // empty)
      | .state // ""
    ')"

	echo "[SN] Supernode: account='${acct}', last_state='${last_state}'"
	if [[ -n "$acct" && "$acct" == "$SN_ADDR" && "$last_state" == "SUPERNODE_STATE_ACTIVE" ]]; then
		return 0
	fi

	echo "[SN] Status: not active and/or account mismatch"
	return 1
}

# Returns 0 if last state is a non-registrable state, else 1
is_sn_blocked_state() {
	local info
	SN_LAST_STATE=""

	echo "[SN] Checking if supernode is postponed..."
	info="$(run_capture $DAEMON q supernode get-supernode "$VALOPER_ADDR" --output json)"
	echo "[SN] Supernode info output: $info"

	local acct
	acct="$(echo "$info" | jq -r '.supernode.supernode_account // ""')"

	local last_state
	last_state="$(echo "$info" | jq -r '
      (.supernode.states // [])
      | sort_by(.height | tonumber)
      | (last // empty)
      | .state // ""
    ')"

	SN_LAST_STATE="$last_state"
	echo "[SN] Supernode: account='${acct}', last_state='${last_state}'"
	case "$last_state" in
	SUPERNODE_STATE_POSTPONED | SUPERNODE_STATE_DISABLED | SUPERNODE_STATE_STOPPED | SUPERNODE_STATE_PENALIZED)
		return 0
		;;
	*)
		return 1
		;;
	esac
}

install_sncli_binary() {
	echo "[SNCLI] Optional install: checking binaries at $SNCLI_BIN_SRC"
	if [ -f "$SNCLI_BIN_SRC" ]; then
		if [ -f "$SNCLI_BIN_DST" ]; then
			if [ "$SNCLI_BIN_SRC" -nt "$SNCLI_BIN_DST" ] || ! cmp -s "$SNCLI_BIN_SRC" "$SNCLI_BIN_DST"; then
				echo "[SNCLI] sncli binary is outdated; updating."
				run cp -f "$SNCLI_BIN_SRC" "$SNCLI_BIN_DST"
				chmod +x "$SNCLI_BIN_DST"
			else
				echo "[SNCLI] sncli binary already installed and up-to-date."
			fi
		else
			echo "[SNCLI] Installing sncli binary..."
			run cp -f "$SNCLI_BIN_SRC" "$SNCLI_BIN_DST"
			chmod +x "$SNCLI_BIN_DST"
		fi
	else
		echo "[SNCLI] sncli binary not found at $SNCLI_BIN_SRC; skipping."
		return 0
	fi
}

configure_sncli() {
	if [ ! -f "$SNCLI_BIN_DST" ]; then
		echo "[SNCLI] sncli binary not found at $SNCLI_BIN_DST; skipping configuration."
		return 0
	fi

	echo "[SNCLI] Configuring sncli..."
	mkdir -p "$SNCLI_BASEDIR"
	# Start from template if provided; otherwise ensure the file exists
	if [ -f "${SNCLI_CFG_SRC}" ]; then
		echo "[SNCLI] Using template ${SNCLI_CFG_SRC} -> ${SNCLI_CFG}"
		cp -f "${SNCLI_CFG_SRC}" "${SNCLI_CFG}"
	else
		echo "[SNCLI] No sncli-config.toml template found; creating empty config at ${SNCLI_CFG}"
		: >"${SNCLI_CFG}"
	fi

	# Ensure sncli-account key exists
	if [ -f "$SNCLI_MNEMONIC_FILE" ]; then
		if ! run ${DAEMON} keys show "${SNCLI_KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null 2>&1; then
			(cat "$SNCLI_MNEMONIC_FILE") | run $DAEMON keys add "$SNCLI_KEY_NAME" --recover --keyring-backend "$KEYRING_BACKEND" >/dev/null
		fi
	else
		run $DAEMON keys delete "$SNCLI_KEY_NAME" --keyring-backend "$KEYRING_BACKEND" -y || true
		local mn_json
		mn_json="$(run_capture $DAEMON keys add "$SNCLI_KEY_NAME" --keyring-backend "$KEYRING_BACKEND" --output json)"
		echo "[SNCLI] Generated new sncli key ${SNCLI_KEY_NAME}..."
		echo "${mn_json}" | jq -r .mnemonic >"${SNCLI_MNEMONIC_FILE}"
	fi
	local addr
	addr="$(${DAEMON} keys show "${SNCLI_KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
	echo "${addr}" >"${SNCLI_ADDR_FILE}"
	echo "[SNCLI] sncli-account address: ${addr}"

	echo "[SNCLI] Checking ${SNCLI_KEY_NAME} balance for $addr..."
	bal_json="$(run_capture $DAEMON q bank balances "$addr" --output json)"
	echo "[SNCLI] Balance output: $bal_json"
	bal="$(
		echo "$bal_json" |
			jq -r --arg denom "$DENOM" '
        ([.balances[]? | select(.denom == $denom) | .amount] | first) // "0"
      '
	)"
	echo "[SNCLI] Current ${SNCLI_KEY_NAME} balance: $bal"
	# Normalize and compare numerically
	[[ -z "$bal" ]] && bal="0"
	if ((bal < ${SNCLI_MIN_AMOUNT})); then
		echo "[SNCLI] Funding ${SNCLI_KEY_NAME}..."
		send_tx_json="$(run_capture $DAEMON tx bank send "$GENESIS_ADDR" "$addr" "${SNCLI_FUND_AMOUNT}${DENOM}" \
			--chain-id "$CHAIN_ID" \
			--keyring-backend "$KEYRING_BACKEND" \
			--gas auto \
			--gas-adjustment 1.3 \
			--fees "3000${DENOM}" \
			--output json --yes)"
		echo "[SNCLI] Send tx output: $send_tx_json"
		send_tx_hash="$(echo "$send_tx_json" | jq -r .txhash)"
		if [ -n "$send_tx_hash" ] && [ "$send_tx_hash" != "null" ]; then
			if ! wait_for_tx "$send_tx_hash"; then
				echo "[SNCLI] Funding tx failed or not confirmed. Exiting."
				exit 1
			fi
		else
			echo "[SNCLI] Failed to get TXHASH for funding transaction."
			exit 1
		fi
	fi

	# --- [lumera] ---
	crudini --set "${SNCLI_CFG}" lumera grpc_addr "\"localhost:${LUMERA_GRPC_PORT}\""
	crudini --set "${SNCLI_CFG}" lumera chain_id "\"${CHAIN_ID}\""

	# --- [supernode] ---
	if [ -n "${SN_ADDR:-}" ]; then
		crudini --set "${SNCLI_CFG}" supernode address "\"${SN_ADDR}\""
	fi
	crudini --set "${SNCLI_CFG}" supernode grpc_endpoint "\"${IP_ADDR}:${SN_PORT}\""
	crudini --set "${SNCLI_CFG}" supernode p2p_endpoint "\"${IP_ADDR}:${SN_P2P_PORT}\""

	# --- [keyring] ---
	crudini --set "${SNCLI_CFG}" keyring backend "\"${KEYRING_BACKEND}\""
	crudini --set "${SNCLI_CFG}" keyring key_name "\"${SNCLI_KEY_NAME}\""
	crudini --set "${SNCLI_CFG}" keyring local_address "\"$addr\""

}

# ------------------------------- main --------------------------------
require_crudini
stop_supernode_if_running
install_supernode_binary
install_sncli_binary
# Ensure Lumera RPC is up before any chain ops
wait_for_lumera || exit 0 # don't fail the container if chain isn't ready; just skip SN
# Wait for at least 5 blocks
wait_for_height_at_least 5 || {
	echo "[SN] Lumera chain not producing blocks in time; exiting."
	exit 1
}

configure_supernode
register_supernode
configure_sncli
start_supernode
