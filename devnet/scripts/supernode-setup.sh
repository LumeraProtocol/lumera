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
TX_GAS_PRICES="${TX_GAS_PRICES:-0.03ulume}"

# After EVM activation, the feemarket module enforces a minimum global fee in
# its own denom (e.g. aatom/alume).  Query the feemarket params at runtime and
# override TX_GAS_PRICES so bank-send txs satisfy the check.
update_gas_prices_for_evm() {
	local params base_fee fee_denom
	params="$($DAEMON q feemarket params --output json 2>/dev/null || true)"
	if [[ -z "$params" ]]; then
		return
	fi
	fee_denom="$(echo "$params" | jq -r '.params.fee_denom // empty' 2>/dev/null || true)"
	base_fee="$(echo "$params" | jq -r '.params.base_fee // empty' 2>/dev/null || true)"
	if [[ -n "$fee_denom" && -n "$base_fee" ]]; then
		# Use 2× base fee as gas price to ensure acceptance under fee fluctuation
		local price
		price="$(echo "$base_fee" | awk '{printf "%.0f", $1 * 2}')"
		# Ensure price is at least 1
		[[ "$price" == "0" || -z "$price" ]] && price="1"
		TX_GAS_PRICES="${price}${fee_denom}"
		echo "[SN] Feemarket active: using gas price ${TX_GAS_PRICES} (base_fee=${base_fee}${fee_denom})"
	fi
}

# In-container standard ports (cosmos-sdk)
LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_RPC_ADDR="http://localhost:${LUMERA_RPC_PORT}"

# Names & paths
KEY_NAME="${MONIKER}_key"
SN_BASEDIR="/root/.supernode"
SN_CONFIG="${SN_BASEDIR}/config.yml"
SN_KEYRING_HOME="${SN_BASEDIR}/keys"
SN_PORT="${SUPERNODE_PORT:-4444}"
SN_P2P_PORT="${SUPERNODE_P2P_PORT:-4445}"
SN_GATEWAY_PORT="${SUPERNODE_GATEWAY_PORT:-8002}"
SN_LOG="${SN_LOG:-/root/logs/supernode.log}"

SHARED_DIR="/shared"
CFG_DIR="${SHARED_DIR}/config"
CFG_CHAIN="${CFG_DIR}/config.json"
CFG_VALS="${CFG_DIR}/validators.json"
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
SN_CONFIG_MNEMONIC=""
SNCLI_CONFIG_MNEMONIC=""

if [[ "$KEY_NAME" == *validator* ]]; then
	SN_KEY_NAME="${KEY_NAME/validator/supernode}"
else
	SN_KEY_NAME="${KEY_NAME}_sn"
fi
SN_LEGACY_HD_PATH="m/44'/118'/0'/0/0"
SN_EVM_HD_PATH="m/44'/60'/0'/0/0"

run() {
	echo "+ $*"
	"$@"
}

run_capture() {
	echo "+ $*" >&2 # goes to stderr, not captured
	"$@"
}

recover_key_from_mnemonic() {
	local key_name="$1"
	local mnemonic="$2"
	run ${DAEMON} keys delete "${key_name}" --keyring-backend "${KEYRING_BACKEND}" -y >/dev/null 2>&1 || true
	printf '%s\n' "${mnemonic}" | run ${DAEMON} keys add "${key_name}" --recover --keyring-backend "${KEYRING_BACKEND}" >/dev/null
}

version_ge() {
	local current normalized_current normalized_floor
	current="${1:-}"
	normalized_current="$(normalize_version "${current}")"
	normalized_floor="$(normalize_version "${2:-}")"
	printf '%s\n' "${normalized_floor}" "${normalized_current}" | sort -V | head -n1 | grep -q "^${normalized_floor}\$"
}

normalize_version() {
	local version="${1:-}"
	version="${version#"${version%%[![:space:]]*}"}"
	version="${version%"${version##*[![:space:]]}"}"
	version="${version#v}"
	printf '%s' "${version}"
}

get_lumerad_version() {
	local version=""
	local env_version="${LUMERA_VERSION:-}"
	local config_version=""

	version="$($DAEMON version 2>/dev/null | grep -Eo 'v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?' | head -n1 || true)"
	version="$(normalize_version "${version}")"
	env_version="$(normalize_version "${env_version}")"
	if [[ -n "$version" ]]; then
		if [[ -n "$env_version" && "$env_version" != "null" && "$env_version" != "$version" ]]; then
			echo "[SN] Ignoring stale LUMERA_VERSION=v${env_version}; detected lumerad binary version ${version}." >&2
		fi
		printf '%s' "${version}"
		return 0
	fi

	if [[ -n "$env_version" && "$env_version" != "null" ]]; then
		printf '%s' "${env_version}"
		return 0
	fi

	if [[ -f "${CFG_CHAIN}" ]]; then
		config_version="$(jq -r '.chain.version // empty' "${CFG_CHAIN}" 2>/dev/null || true)"
	fi
	config_version="$(normalize_version "${config_version}")"

	if [[ -n "$config_version" && "$config_version" != "null" ]]; then
		printf '%s' "${config_version}"
		return 0
	fi

	printf '%s' "${version}"
}

get_first_evm_version() {
	local version=""

	if [[ -n "${LUMERA_FIRST_EVM_VERSION:-}" && "${LUMERA_FIRST_EVM_VERSION}" != "null" ]]; then
		version="${LUMERA_FIRST_EVM_VERSION}"
	elif [[ -f "${CFG_CHAIN}" ]]; then
		version="$(jq -r '.chain.evm_from_version // empty' "${CFG_CHAIN}" 2>/dev/null || true)"
	fi

	if [[ -z "$version" || "$version" == "null" ]]; then
		version="v1.12.0"
	fi

	printf '%s' "$(normalize_version "${version}")"
}

lumera_supports_evm() {
	local current_version first_evm_version

	current_version="$(get_lumerad_version)"
	first_evm_version="$(get_first_evm_version)"

	if [[ -z "$current_version" || "$current_version" == "null" ]]; then
		echo "[SN] Unable to determine lumerad version; assuming no EVM migration support."
		return 1
	fi

	if version_ge "$current_version" "$first_evm_version"; then
		echo "[SN] Lumera version v${current_version} has EVM support (cutover v${first_evm_version})."
		return 0
	fi

	echo "[SN] Lumera version v${current_version} is pre-EVM (cutover v${first_evm_version}); skipping EVM key migration setup."
	return 1
}

daemon_key_pubkey_type() {
	daemon_key_pubkey_type_in_home "$1" ""
}

daemon_key_pubkey_type_in_home() {
	local key_name="$1"
	local home_dir="${2:-}"
	local out
	local cmd=($DAEMON keys show "$key_name" --keyring-backend "$KEYRING_BACKEND" --output json)

	if [[ -n "$home_dir" ]]; then
		cmd+=(--home "$home_dir")
	fi

	if ! out="$("${cmd[@]}" 2>/dev/null)"; then
		return 1
	fi

	jq -r '
		.pubkey
		| (if type == "string" then (fromjson? // {}) else . end)
		| .["@type"] // empty
	' <<<"$out"
}

daemon_key_address() {
	daemon_key_address_in_home "$1" ""
}

daemon_key_address_in_home() {
	local key_name="$1"
	local home_dir="${2:-}"
	local cmd=($DAEMON keys show "$key_name" -a --keyring-backend "$KEYRING_BACKEND")

	if [[ -n "$home_dir" ]]; then
		cmd+=(--home "$home_dir")
	fi

	"${cmd[@]}" 2>/dev/null
}

is_legacy_pubkey_type() {
	local pubkey_type="${1:-}"
	[[ -n "$pubkey_type" && "$pubkey_type" == *"secp256k1.PubKey"* && "$pubkey_type" != *"ethsecp256k1"* ]]
}

is_evm_pubkey_type() {
	local pubkey_type="${1:-}"
	[[ -n "$pubkey_type" && "$pubkey_type" == *"ethsecp256k1"* ]]
}

ensure_evm_key_from_mnemonic() {
	ensure_key_from_mnemonic_in_home "" "daemon keyring" "$1" "$2" "eth_secp256k1" "$SN_EVM_HD_PATH"
}

ensure_supernode_evm_key_from_mnemonic() {
	ensure_key_from_mnemonic_in_home "$SN_KEYRING_HOME" "supernode keyring" "$1" "$2" "eth_secp256k1" "$SN_EVM_HD_PATH"
}

ensure_legacy_key_from_mnemonic() {
	ensure_key_from_mnemonic_in_home "" "daemon keyring" "$1" "$2" "secp256k1" "$SN_LEGACY_HD_PATH"
}

ensure_supernode_legacy_key_from_mnemonic() {
	ensure_key_from_mnemonic_in_home "$SN_KEYRING_HOME" "supernode keyring" "$1" "$2" "secp256k1" "$SN_LEGACY_HD_PATH"
}

ensure_key_from_mnemonic_in_home() {
	local home_dir="$1"
	local scope="$2"
	local key_name="$3"
	local mnemonic="$4"
	local key_type="$5"
	local hd_path="$6"
	local current_type=""
	local cmd=($DAEMON keys add "$key_name" \
		--recover \
		--keyring-backend "$KEYRING_BACKEND" \
		--key-type "$key_type" \
		--hd-path "$hd_path")

	if [[ -n "$home_dir" ]]; then
		cmd+=(--home "$home_dir")
	fi

	current_type="$(daemon_key_pubkey_type_in_home "$key_name" "$home_dir" || true)"
	if [[ "$key_type" == "eth_secp256k1" ]] && is_evm_pubkey_type "$current_type"; then
		echo "[SN] ${key_name} already exists in ${scope} as ${current_type}."
		return 0
	fi
	if [[ "$key_type" == "secp256k1" ]] && is_legacy_pubkey_type "$current_type"; then
		echo "[SN] ${key_name} already exists in ${scope} as ${current_type}."
		return 0
	fi

	if [[ -n "$current_type" ]]; then
		echo "[SN] Replacing ${key_name} in ${scope} (${current_type}) with ${key_type} (${hd_path})."
		if [[ -n "$home_dir" ]]; then
			run ${DAEMON} keys delete "${key_name}" --home "${home_dir}" --keyring-backend "${KEYRING_BACKEND}" -y >/dev/null 2>&1 || true
		else
			run ${DAEMON} keys delete "${key_name}" --keyring-backend "${KEYRING_BACKEND}" -y >/dev/null 2>&1 || true
		fi
	else
		echo "[SN] Creating ${key_type} key ${key_name} in ${scope} (${hd_path})."
	fi

	printf '%s\n' "${mnemonic}" | run "${cmd[@]}" >/dev/null
}

get_supernode_config_value() {
	local config_file="$1"
	local key="$2"

	awk -v key="$key" '
		/^supernode:[[:space:]]*$/ { in_block = 1; next }
		in_block && /^[^[:space:]]/ { exit }
		in_block && $1 == key ":" {
			sub(/^[^:]+:[[:space:]]*/, "", $0)
			gsub(/^["'\'']|["'\'']$/, "", $0)
			print $0
			exit
		}
	' "$config_file"
}

set_supernode_config_value() {
	local config_file="$1"
	local key="$2"
	local value="$3"
	local tmp_file

	tmp_file="$(mktemp)"
	awk -v key="$key" -v value="$value" '
		function print_field() {
			printf "    %s: \"%s\"\n", key, value
		}
		BEGIN {
			in_block = 0
			done = 0
		}
		/^supernode:[[:space:]]*$/ {
			in_block = 1
			print
			next
		}
		in_block && /^[^[:space:]]/ {
			if (!done) {
				print_field()
				done = 1
			}
			in_block = 0
		}
		in_block && $0 ~ "^[[:space:]]*" key ":[[:space:]]*" {
			if (!done) {
				print_field()
				done = 1
			}
			next
		}
		{
			print
		}
		END {
			if (in_block && !done) {
				print_field()
			}
		}
	' "$config_file" >"$tmp_file"
	mv "$tmp_file" "$config_file"
}

maybe_prepare_supernode_migration() {
	local mnemonic="$1"
	local selected_key_name selected_key_type evm_key_name selected_identity selected_key_address legacy_identity onchain_account

	if [[ ! -f "$SN_CONFIG" || -z "$mnemonic" ]]; then
		return 0
	fi

	if ! lumera_supports_evm; then
		return 0
	fi

	selected_key_name="$(get_supernode_config_value "$SN_CONFIG" "key_name")"
	if [[ -z "$selected_key_name" ]]; then
		echo "[SN] No supernode.key_name configured in ${SN_CONFIG}; skipping EVM key setup."
		return 0
	fi

	selected_key_type="$(daemon_key_pubkey_type "$selected_key_name" || true)"
	selected_key_address="$(daemon_key_address "$selected_key_name" || true)"
	onchain_account="$(get_registered_supernode_account || true)"
	if is_evm_pubkey_type "$selected_key_type"; then
		if [[ -n "$onchain_account" && -n "$selected_key_address" && "$onchain_account" != "$selected_key_address" ]]; then
			evm_key_name="${selected_key_name}_evm"
			echo "[SN] Config key ${selected_key_name} is already EVM (${selected_key_type}), but validator is still registered with ${onchain_account}; restoring legacy key ${selected_key_name} for migration."
			ensure_legacy_key_from_mnemonic "$selected_key_name" "$mnemonic"
			ensure_supernode_legacy_key_from_mnemonic "$selected_key_name" "$mnemonic"
			ensure_evm_key_from_mnemonic "$evm_key_name" "$mnemonic"
			ensure_supernode_evm_key_from_mnemonic "$evm_key_name" "$mnemonic"
			legacy_identity="$(daemon_key_address "$selected_key_name" || true)"
			if [[ -n "$legacy_identity" ]]; then
				set_supernode_config_value "$SN_CONFIG" "identity" "$legacy_identity"
			fi
			set_supernode_config_value "$SN_CONFIG" "evm_key_name" "$evm_key_name"
			return 0
		fi
		echo "[SN] Config key ${selected_key_name} is already EVM-compatible (${selected_key_type}); continuing setup."
		return 0
	fi

	if ! is_legacy_pubkey_type "$selected_key_type"; then
		echo "[SN] Config key ${selected_key_name} has unknown type ${selected_key_type:-missing}; skipping EVM key derivation."
		return 0
	fi

	legacy_identity="$(daemon_key_address "$selected_key_name" || true)"
	if [[ -n "$legacy_identity" ]]; then
		selected_identity="$(get_supernode_config_value "$SN_CONFIG" "identity")"
		if [[ "$selected_identity" != "$legacy_identity" ]]; then
			echo "[SN] Config identity ${selected_identity:-<empty>} does not match legacy key ${selected_key_name} (${legacy_identity}); restoring pre-migration identity."
		fi
		set_supernode_config_value "$SN_CONFIG" "identity" "$legacy_identity"
	else
		echo "[SN] Unable to resolve address for legacy config key ${selected_key_name}; leaving identity unchanged."
	fi

	evm_key_name="${selected_key_name}_evm"
	echo "[SN] Config key ${selected_key_name} is legacy (${selected_key_type}); deriving ${evm_key_name} from the same mnemonic."
	ensure_supernode_legacy_key_from_mnemonic "$selected_key_name" "$mnemonic"
	ensure_evm_key_from_mnemonic "$evm_key_name" "$mnemonic"
	ensure_supernode_evm_key_from_mnemonic "$evm_key_name" "$mnemonic"
	set_supernode_config_value "$SN_CONFIG" "evm_key_name" "$evm_key_name"
}

get_registered_supernode_account() {
	local info

	if [[ -z "${VALOPER_ADDR:-}" ]]; then
		return 1
	fi

	if ! info="$($DAEMON q supernode get-supernode "$VALOPER_ADDR" --output json 2>/dev/null)"; then
		return 1
	fi

	jq -r '.supernode.supernode_account // empty' <<<"$info"
}

load_configured_mnemonics() {
	if [ ! -f "${CFG_CHAIN}" ] || [ ! -f "${CFG_VALS}" ]; then
		echo "[SN] Missing ${CFG_CHAIN} or ${CFG_VALS}; will generate local supernode keys."
		return 0
	fi

	local val_index val_count
	val_index="$(jq -r --arg m "${MONIKER}" 'map(.moniker) | index($m) // -1' "${CFG_VALS}")"
	if [ "${val_index}" = "-1" ]; then
		echo "[SN] Validator index for ${MONIKER} not found; will generate local supernode keys."
		return 0
	fi

	val_count="$(jq -r 'length' "${CFG_VALS}")"
	SN_CONFIG_MNEMONIC="$(jq -r --argjson idx "${val_index}" '.["sn-account-mnemonics"][$idx] // empty' "${CFG_CHAIN}")"
	SNCLI_CONFIG_MNEMONIC="$(jq -r --argjson idx "${val_index}" --argjson cnt "${val_count}" '.["sn-account-mnemonics"][$idx + $cnt] // empty' "${CFG_CHAIN}")"
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
		[[ -n "${LUMERA_RPC_ADDR:-}" ]] && tx_args+=(--node "$LUMERA_RPC_ADDR")

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
	echo "[SN] Hints: ensure RPC reachable (check \$LUMERA_RPC_ADDR), and node is not lagging."
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
			--gas auto --gas-adjustment 1.5 --gas-prices "${TX_GAS_PRICES}" -y --output json)"
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
	if [ -n "${SN_CONFIG_MNEMONIC}" ]; then
		local bootstrap_sn_key_name
		echo "${SN_CONFIG_MNEMONIC}" >"${SN_MNEMONIC_FILE}"
		bootstrap_sn_key_name="${SN_KEY_NAME}"
		if [ -f "$SN_CONFIG" ]; then
			bootstrap_sn_key_name="$(get_supernode_config_value "$SN_CONFIG" "key_name")"
			[[ -z "$bootstrap_sn_key_name" ]] && bootstrap_sn_key_name="${SN_KEY_NAME}"
		fi
		if run $DAEMON keys show "$bootstrap_sn_key_name" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1; then
			echo "[SN] Preserving existing ${bootstrap_sn_key_name} from configured sn-account-mnemonics entry."
		else
			ensure_legacy_key_from_mnemonic "${bootstrap_sn_key_name}" "${SN_CONFIG_MNEMONIC}"
			echo "[SN] Recovered legacy ${bootstrap_sn_key_name} from configured sn-account-mnemonics entry."
		fi
	elif [ -f "$SN_MNEMONIC_FILE" ]; then
		if ! run $DAEMON keys show "$SN_KEY_NAME" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1; then
			ensure_legacy_key_from_mnemonic "$SN_KEY_NAME" "$(cat "$SN_MNEMONIC_FILE")"
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

	maybe_prepare_supernode_migration "$(cat "$SN_MNEMONIC_FILE")"
	local configured_sn_key_name
	configured_sn_key_name="$(get_supernode_config_value "$SN_CONFIG" "key_name")"
	if [[ -n "$configured_sn_key_name" ]]; then
		SN_ADDR="$(run_capture $DAEMON keys show "$configured_sn_key_name" -a --keyring-backend "$KEYRING_BACKEND")"
		echo "[SN] Supernode address (${configured_sn_key_name}): $SN_ADDR"
		echo "$SN_ADDR" >"$SN_ADDR_FILE"
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
			--gas-prices "${TX_GAS_PRICES}" \
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
	if [[ "$last_state" == "SUPERNODE_STATE_ACTIVE" ]]; then
		if [[ -n "$acct" && "$acct" != "$SN_ADDR" ]]; then
			echo "[SN] Supernode is ACTIVE with on-chain account ${acct}, while local key resolves to ${SN_ADDR}; treating registration as healthy."
		fi
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
	if [ -n "${SNCLI_CONFIG_MNEMONIC}" ]; then
		echo "${SNCLI_CONFIG_MNEMONIC}" >"${SNCLI_MNEMONIC_FILE}"
		recover_key_from_mnemonic "${SNCLI_KEY_NAME}" "${SNCLI_CONFIG_MNEMONIC}"
		echo "[SNCLI] Recovered ${SNCLI_KEY_NAME} from configured sn-account-mnemonics entry."
	elif [ -f "$SNCLI_MNEMONIC_FILE" ]; then
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
			--gas-prices "${TX_GAS_PRICES}" \
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

update_gas_prices_for_evm
load_configured_mnemonics
configure_supernode
register_supernode
configure_sncli
start_supernode
