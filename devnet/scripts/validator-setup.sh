#!/bin/bash
# /root/scripts/validator-setup.sh
#
# Validator initialization and genesis coordination script for Lumera devnet.
#
# This script runs inside each validator Docker container and orchestrates
# the distributed genesis ceremony across all validators. The flow differs
# based on whether this node is the PRIMARY or a SECONDARY validator:
#
# PRIMARY validator flow:
#   1. Initialize chain (`lumerad init`)
#   2. Copy external genesis template, normalize denoms
#   3. Create own key + genesis account
#   4. Create governance key + genesis account
#   5. Create Hermes relayer key + genesis account
#   6. Publish initial genesis to /shared/ and signal readiness
#   7. Wait for all secondaries to publish their node IDs and gentx files
#   8. Collect secondary genesis accounts and gentx into genesis
#   9. Run own gentx + collect-gentxs to finalize genesis
#  10. Publish final genesis and persistent peers list
#
# SECONDARY validator flow:
#   1. Wait for primary's "genesis_accounts_ready" signal
#   2. Initialize chain, copy initial genesis from primary
#   3. Create own key + genesis account
#   4. Generate gentx and publish to /shared/gentx/
#   5. Publish node ID for peer discovery
#   6. Wait for final genesis from primary, copy it locally
#
# Coordination mechanism:
#   All validators share a Docker volume mounted at /shared/. Coordination
#   uses file-based flags (polled with wait_for_file) and flock for
#   concurrent writes. The primary creates the genesis and waits for
#   secondaries; secondaries wait for the primary.
#
# Environment:
#   MONIKER            - Validator moniker (e.g. "supernova_validator_1"), set by docker-compose
#   LUMERA_API_PORT    - REST API port (default 1317)
#   LUMERA_GRPC_PORT   - gRPC port (default 9090)
#   LUMERA_RPC_PORT    - CometBFT RPC port (default 26657)
#
set -euo pipefail

# ─── Prerequisites ────────────────────────────────────────────────────────────

# Require MONIKER env (compose already sets it)
: "${MONIKER:?MONIKER environment variable must be set}"
echo "[SETUP] Setting up validator $MONIKER"

# ─── Shared Volume Paths ─────────────────────────────────────────────────────
# All validators mount /shared/ from the host. This directory is the sole
# coordination channel between containers during genesis setup.

DEFAULT_P2P_PORT=26656
SHARED_DIR="/shared"
CFG_DIR="${SHARED_DIR}/config"
CFG_CHAIN="${CFG_DIR}/config.json"          # Global chain config (chain ID, denoms, mnemonics)
CFG_VALS="${CFG_DIR}/validators.json"       # Per-validator specs (ports, stakes, monikers)
CLAIMS_SHARED="${CFG_DIR}/claims.csv"       # Token claim records (optional)
GENESIS_SHARED="${CFG_DIR}/genesis.json"    # Initial genesis (after primary adds accounts, before gentx)
FINAL_GENESIS_SHARED="${CFG_DIR}/final_genesis.json"  # Final genesis (after collect-gentxs)
EXTERNAL_GENESIS="${CFG_DIR}/external_genesis.json"   # Template genesis from host
PEERS_SHARED="${CFG_DIR}/persistent_peers.txt"        # Peer list built by primary
GENTX_DIR="${CFG_DIR}/gentx"               # Shared directory for gentx exchange
ADDR_DIR="${SHARED_DIR}/addresses"         # Secondary validators publish their addresses here
STATUS_DIR="${SHARED_DIR}/status"
RELEASE_DIR="${SHARED_DIR}/release"

# Coordination flags — empty files whose existence signals a phase is complete
GENESIS_READY_FLAG="${STATUS_DIR}/genesis_accounts_ready"  # Primary: initial genesis ready
SETUP_COMPLETE_FLAG="${STATUS_DIR}/setup_complete"         # Primary: all setup done

# Per-node status directory (node ID, addresses, keys, flags)
NODE_STATUS_DIR="${STATUS_DIR}/${MONIKER}"
NODE_SETUP_COMPLETE_FLAG="${NODE_STATUS_DIR}/setup_complete"
GENESIS_MNEMONIC_FILE="${NODE_STATUS_DIR}/genesis-address-mnemonic"
LOCKS_DIR="${STATUS_DIR}/locks"

# ─── Hermes IBC Relayer ──────────────────────────────────────────────────────
# The Hermes relayer needs a funded account in genesis to relay IBC packets.
# Its mnemonic is shared via /shared/hermes/ so the Hermes container can
# import it on startup.

HERMES_SHARED_DIR="${SHARED_DIR}/hermes"
HERMES_STATUS_DIR="${STATUS_DIR}/hermes"
HERMES_RELAYER_KEY="${HERMES_RELAYER_KEY:-hermes-relayer}"
HERMES_RELAYER_FILE_NAME="${HERMES_RELAYER_KEY}"
if [[ "${HERMES_RELAYER_FILE_NAME}" != lumera-* ]]; then
	HERMES_RELAYER_FILE_NAME="lumera-${HERMES_RELAYER_FILE_NAME}"
fi
HERMES_RELAYER_MNEMONIC_FILE="${HERMES_SHARED_DIR}/${HERMES_RELAYER_FILE_NAME}.mnemonic"
HERMES_RELAYER_ADDR_FILE="${HERMES_SHARED_DIR}/${HERMES_RELAYER_FILE_NAME}.address"
HERMES_RELAYER_GENESIS_AMOUNT="${HERMES_RELAYER_GENESIS_AMOUNT:-10000000}" # in bond denom units

# ─── Read Chain Config ────────────────────────────────────────────────────────
# All chain parameters are read from config.json (placed on /shared/ by the
# host-side `make devnet-build-*` target). This avoids hardcoding values.

if [ ! command -v jq ] >/dev/null 2>&1; then
	echo "[CONFIGURE] jq is missing"
fi
if [ ! -f "${CFG_CHAIN}" ]; then
	echo "[SETUP] Missing ${CFG_CHAIN}"
	exit 1
fi
if [ ! -f "${CFG_VALS}" ]; then
	echo "[SETUP] Missing ${CFG_VALS}"
	exit 1
fi

CHAIN_ID="$(jq -r '.chain.id' "${CFG_CHAIN}")"
DENOM="$(jq -r '.chain.denom.bond' "${CFG_CHAIN}")"
KEYRING_BACKEND="$(jq -r '.daemon.keyring_backend' "${CFG_CHAIN}")"
DAEMON="$(jq -r '.daemon.binary' "${CFG_CHAIN}")"
DAEMON_HOME_BASE="$(jq -r '.paths.base.container' "${CFG_CHAIN}")"
DAEMON_DIR="$(jq -r '.paths.directories.daemon' "${CFG_CHAIN}")"

if [ -z "${CHAIN_ID}" ] || [ -z "${DENOM}" ] || [ -z "${KEYRING_BACKEND}" ] ||
	[ -z "${DAEMON}" ] || [ -z "${DAEMON_HOME_BASE}" ] || [ -z "${DAEMON_DIR}" ]; then
	echo "[SETUP] Invalid config.json (missing required fields)"
	exit 1
fi

# ─── Local Paths (inside container) ──────────────────────────────────────────

DAEMON_HOME="${DAEMON_HOME_BASE}/${DAEMON_DIR}"
echo "[SETUP] DAEMON_HOME is $DAEMON_HOME"

CONFIG_TOML="${DAEMON_HOME}/config/config.toml"   # CometBFT config (RPC, P2P, peers)
APP_TOML="${DAEMON_HOME}/config/app.toml"         # Cosmos SDK app config (API, gRPC, JSON-RPC, gas)
GENESIS_LOCAL="${DAEMON_HOME}/config/genesis.json" # This node's local copy of genesis
CLAIMS_LOCAL="${DAEMON_HOME}/config/claims.csv"
GENTX_LOCAL_DIR="${DAEMON_HOME}/config/gentx"      # Local gentx staging directory

mkdir -p "${NODE_STATUS_DIR}" "${STATUS_DIR}"
mkdir -p "${LOCKS_DIR}"

# ─── Load This Validator's Record ─────────────────────────────────────────────
# Each validator's config (key name, stake, balance, ports) comes from its
# entry in validators.json, matched by MONIKER.

VAL_REC_JSON="$(jq -c --arg m "$MONIKER" '[.[] | select(.moniker==$m)][0]' "${CFG_VALS}")"
if [ -z "${VAL_REC_JSON}" ] || [ "${VAL_REC_JSON}" = "null" ]; then
	echo "[SETUP] Validator with moniker=${MONIKER} not found in validators.json"
	exit 1
fi

KEY_NAME="$(echo "${VAL_REC_JSON}" | jq -r '.key_name')"
STAKE_AMOUNT="$(echo "${VAL_REC_JSON}" | jq -r '.initial_distribution.validator_stake')"
ACCOUNT_BAL="$(echo "${VAL_REC_JSON}" | jq -r '.initial_distribution.account_balance')"
P2P_HOST_PORT="$(echo "${VAL_REC_JSON}" | jq --arg port "${DEFAULT_P2P_PORT}" -r '.port // $port')"
VAL_INDEX="$(jq -r --arg m "${MONIKER}" 'map(.moniker) | index($m) // -1' "${CFG_VALS}")"
# Load pre-configured mnemonic for deterministic addresses across devnet rebuilds.
# If absent, a new key will be generated in init_if_needed().
GENESIS_ACCOUNT_MNEMONIC=""
if [ "${VAL_INDEX}" != "-1" ]; then
	GENESIS_ACCOUNT_MNEMONIC="$(jq -r --argjson idx "${VAL_INDEX}" '.["genesis-account-mnemonics"][$idx] // empty' "${CFG_CHAIN}")"
fi

# ─── Primary Election ────────────────────────────────────────────────────────
# Exactly one validator is the "primary" — it creates the genesis and
# coordinates the ceremony. Prefer the one with .primary==true in
# validators.json; fall back to the first entry.
PRIMARY_NAME="$(jq -r '
  (map(select(.primary==true)) | if length>0 then .[0].moniker else empty end)
  // (.[0].moniker)
' "${CFG_VALS}")"
IS_PRIMARY="0"
[ "${MONIKER}" = "${PRIMARY_NAME}" ] && IS_PRIMARY="1"

echo "[SETUP] MONIKER=${MONIKER} KEY_NAME=${KEY_NAME} PRIMARY=${IS_PRIMARY} CHAIN_ID=${CHAIN_ID}"
mkdir -p "${DAEMON_HOME}/config"

# ═════════════════════════════════════════════════════════════════════════════
# UTILITY FUNCTIONS
# ═════════════════════════════════════════════════════════════════════════════

# Log and execute a command (output goes to stdout)
run() {
	echo "+ $*"
	"$@"
}

# Log a command to stderr (so stdout can be captured by the caller)
run_capture() {
	echo "+ $*" >&2 # goes to stderr, not captured
	"$@"
}

# Delete and re-import a key from mnemonic (destructive — always replaces)
recover_key_from_mnemonic() {
	local key_name="$1"
	local mnemonic="$2"
	run ${DAEMON} keys delete "${key_name}" --keyring-backend "${KEYRING_BACKEND}" -y >/dev/null 2>&1 || true
	printf '%s\n' "${mnemonic}" | run ${DAEMON} keys add "${key_name}" --recover --keyring-backend "${KEYRING_BACKEND}" >/dev/null
}

# ─── File Locking ─────────────────────────────────────────────────────────────
# Multiple containers write to /shared/ concurrently. These helpers use flock
# to serialize writes and prevent partial/corrupt files (e.g., gentx, addresses,
# Hermes mnemonic). Falls back to no-lock if flock is unavailable.

# Execute a command while holding an exclusive file lock
with_lock() {
	local name="$1"
	shift
	local lock_file="${LOCKS_DIR}/${name}.lock"
	mkdir -p "${LOCKS_DIR}"
	if ! command -v flock >/dev/null 2>&1; then
		"$@"
		return
	fi
	{
		flock -x 200
		"$@"
	} 200>"${lock_file}"
}

# Atomically write a value to a file under lock
write_with_lock() {
	local lock_name="$1"
	local dest="$2"
	local value="$3"
	with_lock "${lock_name}" bash -c 'printf "%s\n" "$1" > "$2"' _ "${value}" "${dest}"
}

# Execute a copy (or any command) under lock
copy_with_lock() {
	local lock_name="$1"
	shift
	with_lock "${lock_name}" "$@"
}

verify_gentx_file() {
	local file="$1"
	if [ ! -f "${file}" ]; then
		echo "[SETUP] ERROR: gentx file ${file} not found"
		return 1
	fi
	return 0
}

# ─── Node Discovery ───────────────────────────────────────────────────────────
# Each validator publishes its CometBFT node ID and P2P port to the shared
# status directory. The primary waits for all node IDs before building the
# persistent_peers list.

# Write this node's P2P port and CometBFT node ID to /shared/status/<moniker>/
write_node_markers() {
	local nodeid
	# write fixed container P2P port
	echo "${DEFAULT_P2P_PORT}" >"${NODE_STATUS_DIR}/port"

	if [ -f "${CONFIG_TOML}" ]; then
		# Cosmos SDK 0.53+ exposes CometBFT commands under "comet";
		# keep a tendermint fallback for older binaries.
		nodeid="$(${DAEMON} comet show-node-id 2>/dev/null || ${DAEMON} tendermint show-node-id 2>/dev/null || true)"
		[ -n "${nodeid}" ] && echo "${nodeid}" >"${NODE_STATUS_DIR}/nodeid"
	fi

	echo "[SETUP] status files in ${NODE_STATUS_DIR}:"
	ls -l "${NODE_STATUS_DIR}" || true
}

# Build the persistent_peers.txt file from all validators' published node IDs.
# Uses Docker-compose service names (== moniker) as hostnames to avoid IP churn.
# Format: <node-id>@<service-name>:<p2p-port>
build_persistent_peers() {
	: >"${PEERS_SHARED}"
	while IFS= read -r other; do
		[ -z "${other}" ] && continue
		[ "${other}" = "${MONIKER}" ] && continue
		local od="${STATUS_DIR}/${other}"
		# Use service DNS name (compose service == moniker) to avoid IP churn.
		if [ -s "${od}/nodeid" ] && [ -s "${od}/port" ]; then
			echo "$(cat "${od}/nodeid")@${other}:$(cat "${od}/port")" >>"${PEERS_SHARED}"
		fi
	done < <(jq -r '.[].moniker' "${CFG_VALS}")
	echo "[SETUP] persistent_peers:"
	cat "${PEERS_SHARED}" || true
}

# Inject persistent_peers and private_peer_ids into config.toml.
# Private peers are needed because Docker-internal IPs are non-routable;
# CometBFT would otherwise refuse to dial them.
apply_persistent_peers() {
	if [ -f "${PEERS_SHARED}" ] && [ -f "${CONFIG_TOML}" ]; then
		local peers
		peers="$(paste -sd, "${PEERS_SHARED}" || true)"
		if [ -n "${peers}" ]; then
			sed -i -E "s|^persistent_peers *=.*$|persistent_peers = \"${peers}\"|g" "${CONFIG_TOML}"
			echo "[SETUP] Applied persistent_peers to ${CONFIG_TOML}"
		fi

		# Treat all validators as private peers so CometBFT accepts their non-routable addresses.
		local peer_ids
		peer_ids="$(cut -d@ -f1 "${PEERS_SHARED}" | paste -sd, || true)"
		if [ -n "${peer_ids}" ]; then
			sed -i -E "s|^private_peer_ids *=.*$|private_peer_ids = \"${peer_ids}\"|g" "${CONFIG_TOML}"
			echo "[SETUP] Applied private_peer_ids to ${CONFIG_TOML}"
		fi
	fi
}

# ─── Node Configuration ───────────────────────────────────────────────────────
# Update app.toml and config.toml with API/gRPC/JSON-RPC settings from
# config.json. Uses crudini for INI-style TOML editing.

configure_node_config() {
	local api_port="${LUMERA_API_PORT:-1317}"
	local grpc_port="${LUMERA_GRPC_PORT:-9090}"
	local rpc_port="${LUMERA_RPC_PORT:-26657}"
	local api_enable_unsafe_cors jsonrpc_enable jsonrpc_address jsonrpc_ws_address jsonrpc_api jsonrpc_enable_indexer

	api_enable_unsafe_cors="$(jq -r '.api.enable_unsafe_cors // true' "${CFG_CHAIN}")"
	jsonrpc_enable="$(jq -r '.["json-rpc"].enable // true' "${CFG_CHAIN}")"
	jsonrpc_address="$(jq -r '.["json-rpc"].address // "0.0.0.0:8545"' "${CFG_CHAIN}")"
	jsonrpc_ws_address="$(jq -r '.["json-rpc"].ws_address // "0.0.0.0:8546"' "${CFG_CHAIN}")"
	jsonrpc_api="$(jq -r '.["json-rpc"].api // "web3,eth,personal,net,txpool,debug,rpc"' "${CFG_CHAIN}")"
	jsonrpc_enable_indexer="$(jq -r '.["json-rpc"].enable_indexer // true' "${CFG_CHAIN}")"
	jsonrpc_api="${jsonrpc_api// /}"
	if [[ ",${jsonrpc_api}," != *",rpc,"* ]]; then
		jsonrpc_api="${jsonrpc_api},rpc"
	fi

	if ! command -v crudini >/dev/null 2>&1; then
		echo "[SETUP] ERROR: crudini not found; cannot update configs"
		exit 1
	fi

	if [ -f "${APP_TOML}" ]; then
		run crudini --set "${APP_TOML}" '' minimum-gas-prices "\"0.0025ulume\""
		run crudini --set "${APP_TOML}" api enable "true"
		run crudini --set "${APP_TOML}" api swagger "true"
		run crudini --set "${APP_TOML}" api address "\"tcp://0.0.0.0:${api_port}\""
		# Required for browser-extension clients (MetaMask) that send non-simple
		# headers like x-metamask-clientid on JSON-RPC requests.
		run crudini --set "${APP_TOML}" api enabled-unsafe-cors "${api_enable_unsafe_cors}"
		run crudini --set "${APP_TOML}" grpc enable "true"
		run crudini --set "${APP_TOML}" grpc address "\"0.0.0.0:${grpc_port}\""
		run crudini --set "${APP_TOML}" grpc-web enable "true"
		run crudini --set "${APP_TOML}" json-rpc enable "${jsonrpc_enable}"
		run crudini --set "${APP_TOML}" json-rpc address "\"${jsonrpc_address}\""
		run crudini --set "${APP_TOML}" json-rpc ws-address "\"${jsonrpc_ws_address}\""
		run crudini --set "${APP_TOML}" json-rpc api "\"${jsonrpc_api}\""
		run crudini --set "${APP_TOML}" json-rpc enable-indexer "${jsonrpc_enable_indexer}"
		echo "[SETUP] Updated ${APP_TOML} with API/GRPC configuration."
	else
		echo "[SETUP] WARNING: ${APP_TOML} not found; skipping app.toml update"
	fi

	if [ -f "${CONFIG_TOML}" ]; then
		run crudini --set "${CONFIG_TOML}" rpc laddr "\"tcp://0.0.0.0:${rpc_port}\""
		echo "[SETUP] Updated ${CONFIG_TOML} RPC configuration."
	else
		echo "[SETUP] WARNING: ${CONFIG_TOML} not found; skipping config.toml update"
	fi
}

# ─── Hermes Relayer Account ────────────────────────────────────────────────────
# Create (or recover) a keyring key for the IBC Hermes relayer, add it as a
# genesis account with funds, and publish its mnemonic to /shared/hermes/ so
# the Hermes container can import it. Called by both primary and secondaries
# to ensure the account exists in each node's local genesis (needed because
# secondaries also call add-genesis-account before sending gentx to primary).
ensure_hermes_relayer_account() {
	echo "[SETUP] Ensuring Hermes relayer account..."
	mkdir -p "${HERMES_SHARED_DIR}" "${HERMES_STATUS_DIR}"

	local mnemonic=""
	if [ -s "${HERMES_RELAYER_MNEMONIC_FILE}" ]; then
		mnemonic="$(cat "${HERMES_RELAYER_MNEMONIC_FILE}")"
	fi

	local relayer_addr
	relayer_addr="$(run_capture ${DAEMON} keys show "${HERMES_RELAYER_KEY}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
	relayer_addr="$(printf '%s' "${relayer_addr}" | tr -d '\r\n')"
	if [ -z "${relayer_addr}" ]; then
		if [ -n "${mnemonic}" ]; then
			printf '%s\n' "${mnemonic}" | run ${DAEMON} keys add "${HERMES_RELAYER_KEY}" --recover --keyring-backend "${KEYRING_BACKEND}" >/dev/null
		else
			local key_json
			key_json="$(run_capture ${DAEMON} keys add "${HERMES_RELAYER_KEY}" --keyring-backend "${KEYRING_BACKEND}" --output json)"
			mnemonic="$(printf '%s' "${key_json}" | jq -r '.mnemonic // empty' 2>/dev/null || true)"
		fi
	fi

	if [ -n "${mnemonic}" ]; then
		write_with_lock "hermes-mnemonic" "${HERMES_RELAYER_MNEMONIC_FILE}" "${mnemonic}"
	fi

	relayer_addr="$(run_capture ${DAEMON} keys show "${HERMES_RELAYER_KEY}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
	relayer_addr="$(printf '%s' "${relayer_addr}" | tr -d '\r\n')"
	if [ -z "${relayer_addr}" ]; then
		echo "[SETUP] ERROR: Unable to obtain Hermes relayer address"
		exit 1
	fi
	write_with_lock "hermes-addr" "${HERMES_RELAYER_ADDR_FILE}" "${relayer_addr}"

	local need_add=1
	if [ -f "${GENESIS_LOCAL}" ]; then
		if jq -e --arg addr "${relayer_addr}" '.app_state.bank.balances[]? | select(.address==$addr)' "${GENESIS_LOCAL}" >/dev/null 2>&1; then
			need_add=0
		fi
	fi

	if [ "${need_add}" -eq 1 ]; then
		echo "[SETUP] Adding Hermes relayer genesis balance: ${HERMES_RELAYER_GENESIS_AMOUNT}${DENOM}"
		set +e
		run ${DAEMON} genesis add-genesis-account "${relayer_addr}" "${HERMES_RELAYER_GENESIS_AMOUNT}${DENOM}"
		local status=$?
		set -e
		if [ ${status} -ne 0 ]; then
			echo "[SETUP] Failed to add Hermes relayer genesis account."
			exit ${status}
		fi
	else
		echo "[SETUP] Hermes relayer genesis account already present."
	fi
}

# Block until a file exists and is non-empty (coordination primitive)
wait_for_file() {
	while [ ! -s "$1" ]; do
		sleep 1
	done
}

# ═════════════════════════════════════════════════════════════════════════════
# CHAIN INITIALIZATION
# Initialize the node's data directory and create/recover the validator key.
# Idempotent — skips init if genesis.json already exists.
# ═════════════════════════════════════════════════════════════════════════════

# Initialize lumerad and ensure the validator key exists.
# Key recovery priority:
#   1. Pre-configured mnemonic from config.json (deterministic across rebuilds)
#   2. Existing key in keyring (survives container restart via volume mount)
#   3. Generate a fresh key (first run with no config)
init_if_needed() {
	if [ -f "${GENESIS_LOCAL}" ]; then
		echo "[SETUP] ${MONIKER} already initialized (genesis exists)."
	else
		echo "[SETUP] Initializing ${MONIKER}..."
		run ${DAEMON} init "${MONIKER}" --chain-id "${CHAIN_ID}" --overwrite
		# Set default client output to JSON for scripting-friendly parsing.
		sed -i 's/^output = .*/output = "json"/' "${DAEMON_HOME}/config/client.toml"
	fi

	# Ensure validator key exists. If a mnemonic is configured for this validator
	# index in config.json, always recover from it to keep addresses deterministic.
	local addr mnemonic key_json
	if [ -n "${GENESIS_ACCOUNT_MNEMONIC}" ]; then
		recover_key_from_mnemonic "${KEY_NAME}" "${GENESIS_ACCOUNT_MNEMONIC}"
		addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
		addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
		printf '%s\n' "${GENESIS_ACCOUNT_MNEMONIC}" >"${GENESIS_MNEMONIC_FILE}"
		echo "[SETUP] Recovered ${KEY_NAME} from configured genesis mnemonic (validator index ${VAL_INDEX})"
	else
		addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
		addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
		if [ -z "${addr}" ]; then
			key_json="$(run_capture ${DAEMON} keys add "${KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" --output json)"
			addr="$(printf '%s' "${key_json}" | jq -r '.address // empty' 2>/dev/null || true)"
			addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
			mnemonic="$(printf '%s' "${key_json}" | jq -r '.mnemonic // empty' 2>/dev/null || true)"
			if [ -n "${mnemonic}" ]; then
				printf '%s\n' "${mnemonic}" >"${GENESIS_MNEMONIC_FILE}"
				echo "[SETUP] Wrote validator mnemonic to ${GENESIS_MNEMONIC_FILE}"
			else
				echo "[SETUP] WARNING: mnemonic is empty for ${KEY_NAME}; ${GENESIS_MNEMONIC_FILE} was not written"
			fi
		else
			echo "[SETUP] Key ${KEY_NAME} already exists with address ${addr}"
			if [ ! -s "${GENESIS_MNEMONIC_FILE}" ]; then
				echo "[SETUP] WARNING: ${GENESIS_MNEMONIC_FILE} is missing; mnemonic cannot be reconstructed for existing key"
			fi
		fi
	fi

	if [ -z "${addr}" ]; then
		addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
		addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
	fi
}

# ═════════════════════════════════════════════════════════════════════════════
# PRIMARY VALIDATOR SETUP
#
# The primary validator orchestrates the genesis ceremony:
#   1. Init + copy external genesis template
#   2. Normalize denoms across staking/mint/crisis/gov modules
#   3. Create genesis accounts (own + governance + Hermes relayer)
#   4. Publish initial genesis → signal "genesis_accounts_ready"
#   5. Wait for all secondaries to publish node IDs + gentx files
#   6. Collect secondary accounts + gentx → run collect-gentxs
#   7. Publish final genesis + persistent peers
#   8. Signal "setup_complete"
# ═════════════════════════════════════════════════════════════════════════════

primary_validator_setup() {
	init_if_needed
	configure_node_config

	# External genesis is the starting template — contains module defaults,
	# chain params, and any pre-existing accounts. Must be provided by the host.
	if [ ! -f "${EXTERNAL_GENESIS}" ]; then
		echo "ERROR: ${EXTERNAL_GENESIS} not found. Provide existing genesis."
		exit 1
	fi
	cp "${EXTERNAL_GENESIS}" "${GENESIS_LOCAL}"
	[ -f "${CLAIMS_SHARED}" ] && cp "${CLAIMS_SHARED}" "${CLAIMS_LOCAL}"

	# Normalize denoms across all modules that reference the bond denom.
	# The external genesis may use a different denom — force consistency.
	tmp="${DAEMON_HOME}/config/tmp_genesis.json"
	cat "${GENESIS_LOCAL}" | jq \
		--arg denom "${DENOM}" '
      .app_state.staking.params.bond_denom = $denom
      | .app_state.mint.params.mint_denom = $denom
      | .app_state.crisis.constant_fee.denom = $denom
      | .app_state.gov.params.min_deposit[0].denom = $denom
      | .app_state.gov.params.expedited_min_deposit[0].denom = $denom
    ' >"${tmp}"
	mv "${tmp}" "${GENESIS_LOCAL}"

	# Add primary validator’s own genesis account with configured balance
	echo "[SETUP] Creating key/account for ${KEY_NAME}..."
	addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
	addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
	if [ -z "${addr}" ]; then
		echo "[SETUP] ERROR: Unable to obtain address for ${KEY_NAME}"
		exit 1
	fi
	run ${DAEMON} genesis add-genesis-account "${addr}" "${ACCOUNT_BAL}"
	printf '%s\n' "${addr}" >"${NODE_STATUS_DIR}/genesis-address"

	# Create a governance key — used to submit upgrade proposals and vote.
	# Gets a large genesis balance (1T ulume) so it can cover proposal deposits.
	local gov_addr
	gov_addr="$(run_capture ${DAEMON} keys show governance_key -a --keyring-backend "${KEYRING_BACKEND}" 2>/dev/null || true)"
	gov_addr="$(printf '%s' "${gov_addr}" | tr -d '\r\n')"
	if [ -z "${gov_addr}" ]; then
		run ${DAEMON} keys add governance_key --keyring-backend "${KEYRING_BACKEND}" >/dev/null
		gov_addr="$(run_capture ${DAEMON} keys show governance_key -a --keyring-backend "${KEYRING_BACKEND}")"
		gov_addr="$(printf '%s' "${gov_addr}" | tr -d '\r\n')"
	fi
	if [ -z "${gov_addr}" ]; then
		echo "[SETUP] ERROR: Unable to obtain governance key address"
		exit 1
	fi
	printf '%s\n' "${gov_addr}" >${SHARED_DIR}/governance_address
	run ${DAEMON} genesis add-genesis-account "${gov_addr}" "1000000000000${DENOM}"

	ensure_hermes_relayer_account

	# ── Phase gate: signal secondaries that initial genesis is ready ──
	# Secondaries block on this flag before copying genesis and creating their
	# own keys + gentx. The initial genesis has primary + governance + Hermes
	# accounts but not yet the secondary accounts or any gentx.
	cp "${GENESIS_LOCAL}" "${GENESIS_SHARED}"
	mkdir -p "${GENTX_DIR}" "${ADDR_DIR}"
	echo "true" >"${GENESIS_READY_FLAG}"

	# Publish own node ID for peer discovery before waiting
	write_node_markers

	# Wait for all secondary validators to publish their CometBFT node IDs.
	# Each secondary writes to /shared/status/<moniker>/nodeid after init.
	total="$(jq -r 'length' "${CFG_VALS}")"
	echo "[SETUP] Waiting for other node IDs/IPs..."
	while true; do
		found=0
		while IFS= read -r other; do
			[ "${other}" = "${MONIKER}" ] && continue
			od="${STATUS_DIR}/${other}"
			[[ -s "${od}/nodeid" ]] && found=$((found + 1))
		done < <(jq -r '.[].moniker' "${CFG_VALS}")
		[ "${found}" -ge $((total - 1)) ] && break
		sleep 1
	done

	# ── Collect secondary accounts ──
	# Secondaries publish their address + balance to /shared/addresses/<addr>.
	# The primary adds each as a genesis account so they appear in final genesis.
	echo "[SETUP] Collecting addresses & gentx from secondaries..."
	if compgen -G "${ADDR_DIR}/*" >/dev/null; then
		while IFS= read -r file; do
			[ -f "$file" ] || continue
			bal="$(cat "$file")"
			addr="$(basename "$file")"
			run ${DAEMON} genesis add-genesis-account "${addr}" "${bal}"
		done < <(find ${ADDR_DIR} -type f)
	fi

	# ── Generate primary's own gentx ──
	# gentx = "genesis transaction" that self-delegates STAKE_AMOUNT to this
	# validator. Each validator creates one; primary collects them all.
	run ${DAEMON} genesis gentx "${KEY_NAME}" "${STAKE_AMOUNT}" \
		--chain-id "${CHAIN_ID}" \
		--keyring-backend "${KEYRING_BACKEND}"

	for file in "${GENTX_LOCAL_DIR}"/gentx-*.json; do
		[ -f "${file}" ] || continue
		verify_gentx_file "${file}" || exit 1
	done

	# ── Collect secondary gentx files ──
	# Copy all gentx-*.json from /shared/gentx/ into the local gentx dir,
	# then run collect-gentxs to merge them all into the genesis.
	mkdir -p "${GENTX_LOCAL_DIR}"
	if compgen -G "${GENTX_DIR}/*.json" >/dev/null; then
		copy_with_lock "gentx" bash -c 'cp "$1"/*.json "$2"/' _ "${GENTX_DIR}" "${GENTX_LOCAL_DIR}" || true
		for file in "${GENTX_LOCAL_DIR}"/gentx-*.json; do
			[ -f "${file}" ] || continue
			verify_gentx_file "${file}" || exit 1
		done
	fi
	run ${DAEMON} genesis collect-gentxs

	# ── Publish final genesis + peers ──
	# This is the authoritative genesis that all validators will use.
	# Secondaries are waiting on FINAL_GENESIS_SHARED before starting lumerad.
	cp "${GENESIS_LOCAL}" "${FINAL_GENESIS_SHARED}"
	echo "[SETUP] Final genesis published to ${FINAL_GENESIS_SHARED}"

	# Build peer list from all node IDs and inject into config.toml
	build_persistent_peers
	apply_persistent_peers

	# Signal all validators that setup is complete — start.sh waits on this
	echo "true" >"${SETUP_COMPLETE_FLAG}"
	echo "true" >"${NODE_SETUP_COMPLETE_FLAG}"
	echo "[SETUP] Primary setup complete."
}

# ═════════════════════════════════════════════════════════════════════════════
# SECONDARY VALIDATOR SETUP
#
# Secondary validators wait for the primary, then:
#   1. Copy initial genesis from primary (has primary + governance accounts)
#   2. Create own key + add own genesis account
#   3. Generate gentx and publish to /shared/gentx/ for primary to collect
#   4. Publish node ID + address for peer discovery
#   5. Wait for primary's final genesis (with all gentx merged)
#   6. Copy final genesis and apply persistent peers
# ═════════════════════════════════════════════════════════════════════════════

secondary_validator_setup() {
	# Block until primary has created initial genesis with accounts
	echo "[SETUP] Waiting for primary genesis_accounts_ready..."
	wait_for_file "${GENESIS_READY_FLAG}"
	wait_for_file "${GENESIS_SHARED}"

	init_if_needed
	configure_node_config

	# copy initial genesis/claims
	cp "${GENESIS_SHARED}" "${GENESIS_LOCAL}"
	[ -f "${CLAIMS_SHARED}" ] && cp "${CLAIMS_SHARED}" "${CLAIMS_LOCAL}"

	# Create key (if not already present) and add own genesis account.
	# The genesis account must be added to the LOCAL genesis copy so that
	# gentx validation passes. The primary also gets the account via /shared/addresses/.
	addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
	addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
	if [ -z "${addr}" ]; then
		if [ -n "${GENESIS_ACCOUNT_MNEMONIC}" ]; then
			recover_key_from_mnemonic "${KEY_NAME}" "${GENESIS_ACCOUNT_MNEMONIC}"
		else
			run ${DAEMON} keys add "${KEY_NAME}" --keyring-backend "${KEYRING_BACKEND}" >/dev/null
		fi
	fi
	addr="$(run_capture ${DAEMON} keys show "${KEY_NAME}" -a --keyring-backend "${KEYRING_BACKEND}")"
	addr="$(printf '%s' "${addr}" | tr -d '\r\n')"
	if [ -z "${addr}" ]; then
		echo "[SETUP] ERROR: Unable to obtain address for ${KEY_NAME}"
		exit 1
	fi
	run ${DAEMON} genesis add-genesis-account "${addr}" "${ACCOUNT_BAL}"
	ensure_hermes_relayer_account

	mkdir -p "${GENTX_LOCAL_DIR}" "${GENTX_DIR}" "${ADDR_DIR}"

	if compgen -G "${GENTX_LOCAL_DIR}/gentx-*.json" >/dev/null; then
		echo "[SETUP] gentx already exists in ${GENTX_LOCAL_DIR}, skipping generation"
	else
		run ${DAEMON} genesis gentx "${KEY_NAME}" "${STAKE_AMOUNT}" \
			--chain-id "${CHAIN_ID}" --keyring-backend "${KEYRING_BACKEND}"
	fi

	local gentx_file
	gentx_file="$(find "${GENTX_LOCAL_DIR}" -maxdepth 1 -type f -name 'gentx-*.json' -print | head -n1)"
	if [ -z "${gentx_file}" ]; then
		echo "[SETUP] ERROR: gentx generation failed for ${KEY_NAME} (no file produced)"
		exit 1
	fi
	verify_gentx_file "${gentx_file}" || exit 1

	# Publish gentx + address to /shared/ for primary to collect.
	# The address file is named by the address itself; its content is the balance.
	copy_with_lock "gentx" cp "${gentx_file}" "${GENTX_DIR}/${MONIKER}_gentx.json"
	write_with_lock "addresses" "${ADDR_DIR}/${addr}" "${ACCOUNT_BAL}"
	printf '%s\n' "${addr}" >"${NODE_STATUS_DIR}/genesis-address"

	# write own markers for peer discovery
	write_node_markers

	# wait for persistent_peers and apply
	wait_for_file "${PEERS_SHARED}"
	apply_persistent_peers

	# wait for final genesis
	echo "[SETUP] Waiting for final genesis from primary..."
	wait_for_file "${FINAL_GENESIS_SHARED}"
	cp "${FINAL_GENESIS_SHARED}" "${GENESIS_LOCAL}"
	wait_for_file "${SETUP_COMPLETE_FLAG}"

	echo "[SETUP] Secondary setup complete."
	echo "true" >"${NODE_SETUP_COMPLETE_FLAG}"
}

# ═════════════════════════════════════════════════════════════════════════════
# MAIN — dispatch to primary or secondary setup based on election result
# ═════════════════════════════════════════════════════════════════════════════

if [ "${IS_PRIMARY}" = "1" ]; then
	primary_validator_setup
else
	secondary_validator_setup
fi
