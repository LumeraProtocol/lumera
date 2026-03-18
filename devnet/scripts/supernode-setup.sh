#!/bin/bash
# /root/scripts/supernode-setup.sh
#
# Supernode setup and lifecycle script for Lumera devnet.
#
# This script runs inside each validator Docker container and handles:
#   1. Installing supernode + sncli binaries from /shared/release/
#   2. Waiting for the Lumera chain to be ready (RPC up, height >= 5)
#   3. Creating/recovering supernode keys (with EVM migration support)
#   4. Initializing supernode config.yml if absent
#   5. Funding the supernode account from the validator's genesis account
#   6. Registering the supernode on-chain (MsgRegisterSupernode)
#   7. Setting up sncli (CLI client) with its own funded account
#   8. Starting the supernode process in the background
#
# Environment:
#   MONIKER            - Validator moniker (e.g. "supernova_validator_1"), set by docker-compose
#   SUPERNODE_PORT     - gRPC listen port (default 4444)
#   SUPERNODE_P2P_PORT - P2P listen port (default 4445)
#   SUPERNODE_GATEWAY_PORT - HTTP gateway port (default 8002)
#   TX_GAS_PRICES      - Override gas price (auto-detected after EVM activation)
#   LUMERA_VERSION     - Optional version hint (binary version takes precedence)
#   LUMERA_FIRST_EVM_VERSION - Chain version that introduced EVM (default v1.12.0)
#
# Coordination:
#   Reads config from /shared/config/{config.json,validators.json}
#   Persists keys/addresses to /shared/status/<moniker>/
#   Reads binaries from /shared/release/
#
set -euo pipefail

# ─── Prerequisites ────────────────────────────────────────────────────────────

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

# ─── Global Constants ─────────────────────────────────────────────────────────

DAEMON="lumerad"
CHAIN_ID="lumera-devnet-1"
KEYRING_BACKEND="test"
DENOM="ulume"
TX_GAS_PRICES="${TX_GAS_PRICES:-0.03ulume}"

# After EVM activation, the feemarket module enforces a minimum global fee in
# its own denom (e.g. aatom/alume).  Query the feemarket params at runtime and
# override TX_GAS_PRICES so bank-send txs satisfy the check.
update_gas_prices_for_evm() {
	local params evm_config base_fee fee_denom
	params="$($DAEMON q feemarket params --output json 2>/dev/null || true)"
	if [[ -z "$params" ]]; then
		return
	fi
	fee_denom="$(echo "$params" | jq -r '.params.fee_denom // empty' 2>/dev/null || true)"
	base_fee="$(echo "$params" | jq -r '.params.base_fee // .params.min_gas_price // empty' 2>/dev/null || true)"
	if [[ -z "$fee_denom" ]]; then
		evm_config="$($DAEMON q evm config --output json 2>/dev/null || true)"
		fee_denom="$(echo "$evm_config" | jq -r '.config.denom // empty' 2>/dev/null || true)"
	fi
	if [[ -n "$fee_denom" && -n "$base_fee" ]]; then
		# Use 2× base fee as gas price to ensure acceptance under fee fluctuation
		local price
		price="$(jq -nr --arg base_fee "$base_fee" '
			($base_fee | tonumber * 2)
			| if . < 0.000001 then 0.000001 else . end
		' 2>/dev/null || true)"
		[[ -z "$price" || "$price" == "null" ]] && price="0.000001"
		TX_GAS_PRICES="${price}${fee_denom}"
		echo "[SN] Feemarket active: using gas price ${TX_GAS_PRICES} (base_fee=${base_fee}${fee_denom})"
	fi
}

# ─── Network Ports (inside container, not host-mapped) ────────────────────────

LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_RPC_ADDR="http://localhost:${LUMERA_RPC_PORT}"

# ─── Paths & Naming ──────────────────────────────────────────────────────────
# KEY_NAME: validator's keyring key, used as --from for on-chain txs
# SN_KEY_NAME: supernode's own keyring key (derived from MONIKER)
KEY_NAME="${MONIKER}_key"
SN_BASEDIR="/root/.supernode"
SN_CONFIG="${SN_BASEDIR}/config.yml"
SN_KEYRING_HOME="${SN_BASEDIR}/keys"
SN_PORT="${SUPERNODE_PORT:-4444}"
SN_P2P_PORT="${SUPERNODE_P2P_PORT:-4445}"
SN_GATEWAY_PORT="${SUPERNODE_GATEWAY_PORT:-8002}"
SN_LOG="${SN_LOG:-/root/logs/supernode.log}"

# Shared volume mounted to all validator containers for cross-node coordination
SHARED_DIR="/shared"
CFG_DIR="${SHARED_DIR}/config"
CFG_CHAIN="${CFG_DIR}/config.json"       # Global chain config (chain ID, mnemonics, EVM version)
CFG_VALS="${CFG_DIR}/validators.json"    # Per-validator specs (ports, stakes, monikers)
STATUS_DIR="${SHARED_DIR}/status"        # Per-validator flags and key material
RELEASE_DIR="${SHARED_DIR}/release"      # Binaries copied from devnet/bin/ on the host

# ─── Supernode Binary Paths ───────────────────────────────────────────────────
# Two possible source names; prefer the platform-specific one
SN="supernode-linux-amd64"
SN_ALT="supernode"
SN_BIN_SRC="${RELEASE_DIR}/${SN}"
SN_BIN_SRC_ALT="${RELEASE_DIR}/${SN_ALT}"
SN_BIN_DST="/usr/local/bin/${SN}"
NODE_STATUS_DIR="${STATUS_DIR}/${MONIKER}"
SN_MNEMONIC_FILE="${NODE_STATUS_DIR}/sn_mnemonic"
SN_ADDR_FILE="${NODE_STATUS_DIR}/supernode-address"

# Container's Docker-network IP (used for P2P listen address and endpoint registration)
IP_ADDR="$(hostname -i | awk '{print $1}')"

# ─── SNCLI (SuperNode CLI Client) Paths ──────────────────────────────────────
# sncli is an optional CLI tool for interacting with the supernode's gRPC API
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
# Loaded later by load_configured_mnemonics() from config.json
SN_CONFIG_MNEMONIC=""
SNCLI_CONFIG_MNEMONIC=""

# Derive supernode key name from validator moniker:
#   "supernova_validator_1_key" → "supernova_supernode_1_key"
if [[ "$KEY_NAME" == *validator* ]]; then
	SN_KEY_NAME="${KEY_NAME/validator/supernode}"
else
	SN_KEY_NAME="${KEY_NAME}_sn"
fi

# HD derivation paths: legacy Cosmos (coin 118) vs EVM-compatible (coin 60)
# The same mnemonic derives different addresses on each path.
# Pre-EVM chains use 118; post-EVM chains use 60 (eth_secp256k1).
SN_LEGACY_HD_PATH="m/44'/118'/0'/0/0"
SN_EVM_HD_PATH="m/44'/60'/0'/0/0"

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

# ═════════════════════════════════════════════════════════════════════════════
# VERSION DETECTION
# Determines the running lumerad version and whether EVM features are active.
# Version comparison is used to decide key types (legacy vs EVM) and gas pricing.
# ═════════════════════════════════════════════════════════════════════════════

# Returns 0 if $1 >= $2 using semver comparison (via sort -V)
version_ge() {
	local current normalized_current normalized_floor
	current="${1:-}"
	normalized_current="$(normalize_version "${current}")"
	normalized_floor="$(normalize_version "${2:-}")"
	printf '%s\n' "${normalized_floor}" "${normalized_current}" | sort -V | head -n1 | grep -q "^${normalized_floor}\$"
}

# Strip leading/trailing whitespace and "v" prefix: "  v1.12.0 " → "1.12.0"
normalize_version() {
	local version="${1:-}"
	version="${version#"${version%%[![:space:]]*}"}"
	version="${version%"${version##*[![:space:]]}"}"
	version="${version#v}"
	printf '%s' "${version}"
}

# Detect the running lumerad version using three sources (in priority order):
#   1. `lumerad version` binary output (most authoritative)
#   2. LUMERA_VERSION env var (set by docker-compose, may be stale after upgrade)
#   3. config.json .chain.version field (fallback)
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

# Return the chain version that first introduced EVM support.
# Used by lumera_supports_evm() to decide whether to set up EVM keys.
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

# Returns 0 if the current lumerad binary version >= the EVM cutover version.
# When true, keys must use eth_secp256k1 (coin 60) instead of secp256k1 (coin 118).
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

# ═════════════════════════════════════════════════════════════════════════════
# KEY MANAGEMENT
# Functions for creating, inspecting, and migrating keyring keys.
#
# Two keyrings are in play:
#   - Default (~/.lumera/): used by lumerad for tx signing
#   - Supernode (~/.supernode/keys/): used by the supernode process itself
#
# Both keyrings need matching keys so the supernode can sign on behalf of
# its registered account. During EVM migration, each keyring gets both a
# legacy (secp256k1) and an EVM (eth_secp256k1) key derived from the same
# mnemonic but different HD paths.
# ═════════════════════════════════════════════════════════════════════════════

# Returns the pubkey @type string (e.g. "/cosmos.crypto.secp256k1.PubKey")
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

# Convenience wrappers: ensure a key of the right type exists in the right keyring.
# "ensure" means: if the key already exists with the correct type, do nothing;
# if it exists with the wrong type, delete and recreate; if missing, create.
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

# Core idempotent key-ensure function.
# Checks if key_name exists in the specified keyring (home_dir) with the
# expected key_type. If it matches, returns early. If it's the wrong type,
# deletes and recreates. If missing, creates from mnemonic.
ensure_key_from_mnemonic_in_home() {
	local home_dir="$1"
	local scope="$2"       # Human-readable label for log messages
	local key_name="$3"
	local mnemonic="$4"
	local key_type="$5"    # "secp256k1" or "eth_secp256k1"
	local hd_path="$6"     # HD derivation path
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

# ═════════════════════════════════════════════════════════════════════════════
# SUPERNODE CONFIG.YML MANIPULATION
# The supernode binary uses a YAML config at ~/.supernode/config.yml.
# These awk-based helpers read/write fields under the "supernode:" block
# without requiring a YAML parser.
# ═════════════════════════════════════════════════════════════════════════════

# Read a value from the "supernode:" block in config.yml.
# Usage: get_supernode_config_value "$SN_CONFIG" "key_name"
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

# Set (or add) a value in the "supernode:" block of config.yml.
# If the key exists, replaces its value; if not, appends it to the block.
# Uses a temp file + mv for atomicity.
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

# ═════════════════════════════════════════════════════════════════════════════
# EVM KEY MIGRATION
#
# When the chain upgrades from pre-EVM (coin 118) to post-EVM (coin 60),
# the supernode's on-chain identity changes addresses. This section prepares
# for that transition by deriving both legacy and EVM keys from the same
# mnemonic and writing the evm_key_name into config.yml.
#
# The supernode binary itself handles the actual on-chain migration
# (MsgClaimLegacyAccount) at runtime. This script just ensures both keys
# exist in both keyrings so the supernode can sign the migration tx.
#
# Key state matrix (what this function sets up):
#   Pre-EVM chain:  key_name=legacy,  evm_key_name=unset     → no migration
#   Post-EVM chain: key_name=legacy,  evm_key_name=<name>_evm → ready to migrate
#   Post-migration: key_name=evm,     evm_key_name=unset     → already migrated
# ═════════════════════════════════════════════════════════════════════════════

# Prepare dual keys (legacy + EVM) if the chain supports EVM and migration
# hasn't happened yet. Idempotent — safe to call on every container restart.
maybe_prepare_supernode_migration() {
	local mnemonic="$1"
	local selected_key_name selected_key_type evm_key_name selected_identity selected_key_address legacy_identity onchain_account

	# Skip if no config or mnemonic available
	if [[ ! -f "$SN_CONFIG" || -z "$mnemonic" ]]; then
		return 0
	fi

	# Skip if chain doesn't support EVM yet
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

	# Case 1: Key is already EVM-type
	if is_evm_pubkey_type "$selected_key_type"; then
		if [[ -n "$onchain_account" && -n "$selected_key_address" && "$onchain_account" != "$selected_key_address" ]]; then
			# Edge case: config key is EVM but the on-chain registration still
			# points to the legacy address. This happens after a container restart
			# mid-migration. Restore the legacy key so migration can complete.
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
		# Already EVM and on-chain account matches — nothing to do
		echo "[SN] Config key ${selected_key_name} is already EVM-compatible (${selected_key_type}); continuing setup."
		return 0
	fi

	# Case 2: Key type is unknown — can't safely migrate
	if ! is_legacy_pubkey_type "$selected_key_type"; then
		echo "[SN] Config key ${selected_key_name} has unknown type ${selected_key_type:-missing}; skipping EVM key derivation."
		return 0
	fi

	# Case 3: Key is legacy — derive the EVM key alongside it
	# Ensure config.yml identity matches the legacy key address (it may have
	# drifted if the config was written with an EVM address from a prior run)
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

	# Create the EVM key in both keyrings (daemon + supernode) and record it
	# in config.yml so the supernode process knows to attempt migration at startup
	evm_key_name="${selected_key_name}_evm"
	echo "[SN] Config key ${selected_key_name} is legacy (${selected_key_type}); deriving ${evm_key_name} from the same mnemonic."
	ensure_supernode_legacy_key_from_mnemonic "$selected_key_name" "$mnemonic"
	ensure_evm_key_from_mnemonic "$evm_key_name" "$mnemonic"
	ensure_supernode_evm_key_from_mnemonic "$evm_key_name" "$mnemonic"
	set_supernode_config_value "$SN_CONFIG" "evm_key_name" "$evm_key_name"
}

# Query the chain for the supernode_account registered under this validator.
# Returns the account address string, or fails if not registered.
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

# ═════════════════════════════════════════════════════════════════════════════
# MNEMONIC LOADING
# Mnemonics can be pre-configured in config.json under "sn-account-mnemonics"
# to ensure deterministic addresses across devnet rebuilds. The array is laid
# out as: [sn_0, sn_1, ..., sn_N, sncli_0, sncli_1, ..., sncli_N] where N
# is the number of validators. Each validator picks its entry by index.
# ═════════════════════════════════════════════════════════════════════════════

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

# crudini is used to edit sncli's TOML config (INI-style section/key/value)
require_crudini() {
	if ! command -v crudini >/dev/null 2>&1; then
		echo "[SN] ERROR: crudini not found. Please install it (e.g., apt-get update && apt-get install -y crudini) and re-run."
		return 1
	fi
}

# ═════════════════════════════════════════════════════════════════════════════
# CHAIN INTERACTION HELPERS
# Functions for waiting on the chain (RPC readiness, block height) and
# confirming transactions. Used during funding and registration.
# ═════════════════════════════════════════════════════════════════════════════

# Wait for a transaction to be included in a block.
# Strategy: first try WebSocket subscription (fast, event-driven), then fall
# back to polling `q tx` by hash every $interval seconds until $timeout.
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

# Block until lumerad's RPC endpoint responds (up to 180 seconds).
# Called early in the main flow to ensure the chain is running before
# attempting any on-chain operations (funding, registration).
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

# ═════════════════════════════════════════════════════════════════════════════
# SUPERNODE PROCESS LIFECYCLE
# Start, stop, and monitor the supernode process. The supernode runs as a
# background process (`supernode start -d <basedir> &`). These functions
# find it by matching the command line via pgrep.
#
# Note: there is no process supervisor (like sn-manager) — if the supernode
# crashes after setup completes, it stays down until the container restarts.
# ═════════════════════════════════════════════════════════════════════════════

supernode_pids() {
	pgrep -f "${SN} start -d ${SN_BASEDIR}" || true
}

supernode_is_running() {
	[[ -n "$(supernode_pids)" ]]
}

wait_for_supernode_exit() {
	local timeout="${1:-15}"
	local deadline=$((SECONDS + timeout))

	while ((SECONDS < deadline)); do
		if ! supernode_is_running; then
			return 0
		fi
		sleep 1
	done

	return 1
}

# Gracefully stop supernode: try `supernode stop` first, then SIGTERM,
# then SIGKILL as a last resort. Each step has a timeout.
stop_supernode_process() {
	if ! supernode_is_running; then
		return 0
	fi

	echo "[SN] Stopping supernode..."
	run ${SN} stop -d "$SN_BASEDIR" >>"$SN_LOG" 2>&1 || true
	if wait_for_supernode_exit 20; then
		echo "[SN] Supernode stopped."
		return 0
	fi

	echo "[SN] Supernode did not stop cleanly; terminating lingering process."
	supernode_pids | xargs -r kill || true
	if wait_for_supernode_exit 10; then
		echo "[SN] Supernode stopped after termination."
		return 0
	fi

	echo "[SN] ERROR: failed to stop supernode."
	return 1
}

start_supernode_process() {
	run ${SN} start -d "$SN_BASEDIR" >"$SN_LOG" 2>&1 &
}

# After starting the supernode on a post-EVM chain, it may perform an
# in-process key migration (MsgClaimLegacyAccount). If it does, we restart
# it so it picks up the new key state cleanly. Detected by checking the log
# for "EVM migration complete" within the first 5 seconds.
restart_supernode_after_evm_migration_if_needed() {
	local configured_key_name

	if ! lumera_supports_evm; then
		return 0
	fi

	configured_key_name="$(get_supernode_config_value "$SN_CONFIG" "key_name")"
	if [[ -z "$configured_key_name" || "$configured_key_name" != *_evm ]]; then
		return 0
	fi

	sleep 5
	if ! grep -q "EVM migration complete" "$SN_LOG" 2>/dev/null; then
		return 0
	fi

	echo "[SN] Supernode completed in-process EVM migration; restarting to refresh runtime key state."
	stop_supernode_process || return 1

	# Update status files and configs with the new post-migration addresses.
	update_addresses_after_evm_migration
	migrate_sncli_account_if_needed

	start_supernode_process
	echo "[SN] Supernode restarted after EVM migration."
}

# After the supernode binary migrates its account on-chain, update the
# persisted address file and sncli config to reflect the new EVM address.
update_addresses_after_evm_migration() {
	local new_sn_addr configured_key_name

	configured_key_name="$(get_supernode_config_value "$SN_CONFIG" "key_name")"
	new_sn_addr="$(daemon_key_address "$configured_key_name" || true)"
	if [[ -z "$new_sn_addr" ]]; then
		echo "[SN] WARN: could not resolve post-migration SN address for ${configured_key_name}"
		return 0
	fi

	# Preserve old address file as -pre-evm, write new address.
	if [[ -f "$SN_ADDR_FILE" ]]; then
		cp -f "$SN_ADDR_FILE" "${SN_ADDR_FILE}-pre-evm"
		echo "[SN] Saved pre-EVM supernode address to ${SN_ADDR_FILE}-pre-evm"
	fi
	echo "$new_sn_addr" >"$SN_ADDR_FILE"
	SN_ADDR="$new_sn_addr"
	echo "[SN] Updated supernode-address: $new_sn_addr"

	# Update sncli config with new supernode address (if sncli is configured).
	if [[ -f "$SNCLI_CFG" ]] && have crudini; then
		crudini --set "${SNCLI_CFG}" supernode address "\"${new_sn_addr}\""
		echo "[SN] Updated sncli config supernode.address: $new_sn_addr"
	fi
}

# Migrate the sncli-account key from legacy (secp256k1) to EVM (eth_secp256k1)
# if the chain supports EVM and the key is still legacy.
migrate_sncli_account_if_needed() {
	if [[ ! -f "$SNCLI_CFG" ]] || ! have crudini; then
		return 0
	fi

	local sncli_key_type sncli_mnemonic old_addr new_addr
	sncli_key_type="$(daemon_key_pubkey_type "$SNCLI_KEY_NAME" || true)"

	# Already EVM — nothing to do.
	if is_evm_pubkey_type "$sncli_key_type"; then
		return 0
	fi

	# Not legacy — unknown, skip.
	if ! is_legacy_pubkey_type "$sncli_key_type"; then
		echo "[SNCLI] Key ${SNCLI_KEY_NAME} has unknown type ${sncli_key_type:-missing}; skipping migration."
		return 0
	fi

	# Need mnemonic to re-derive the key with coin-type 60.
	if [[ ! -f "$SNCLI_MNEMONIC_FILE" ]]; then
		echo "[SNCLI] No mnemonic file for ${SNCLI_KEY_NAME}; cannot migrate to EVM key."
		return 0
	fi
	sncli_mnemonic="$(cat "$SNCLI_MNEMONIC_FILE")"
	if [[ -z "$sncli_mnemonic" ]]; then
		echo "[SNCLI] Empty mnemonic file for ${SNCLI_KEY_NAME}; cannot migrate."
		return 0
	fi

	old_addr="$(daemon_key_address "$SNCLI_KEY_NAME" || true)"
	echo "[SNCLI] Migrating ${SNCLI_KEY_NAME} from legacy to EVM key type..."

	# Save old address, re-create key as EVM type.
	if [[ -f "$SNCLI_ADDR_FILE" ]]; then
		cp -f "$SNCLI_ADDR_FILE" "${SNCLI_ADDR_FILE}-pre-evm"
	fi

	# Delete and re-add with EVM key type.
	$DAEMON keys delete "$SNCLI_KEY_NAME" --keyring-backend "$KEYRING_BACKEND" -y >/dev/null 2>&1 || true
	printf '%s\n' "$sncli_mnemonic" | $DAEMON keys add "$SNCLI_KEY_NAME" \
		--recover \
		--keyring-backend "$KEYRING_BACKEND" \
		--key-type "eth_secp256k1" \
		--hd-path "$SN_EVM_HD_PATH" >/dev/null

	new_addr="$(daemon_key_address "$SNCLI_KEY_NAME" || true)"
	echo "[SNCLI] ${SNCLI_KEY_NAME}: ${old_addr} -> ${new_addr}"

	# Update address file and sncli config.
	echo "$new_addr" >"$SNCLI_ADDR_FILE"
	crudini --set "${SNCLI_CFG}" keyring local_address "\"$new_addr\""
	echo "[SNCLI] Updated sncli config keyring.local_address: $new_addr"

	# Fund the new sncli address if needed (balance is on the old address).
	local bal
	bal="$($DAEMON q bank balances "$new_addr" --output json 2>/dev/null | \
		jq -r --arg denom "$DENOM" '([.balances[]? | select(.denom == $denom) | .amount] | first) // "0"')"
	[[ -z "$bal" ]] && bal="0"
	if ((bal < SNCLI_MIN_AMOUNT)); then
		echo "[SNCLI] Funding migrated ${SNCLI_KEY_NAME} ($new_addr)..."
		local send_json txhash
		send_json="$($DAEMON tx bank send "$GENESIS_ADDR" "$new_addr" "${SNCLI_FUND_AMOUNT}${DENOM}" \
			--chain-id "$CHAIN_ID" \
			--keyring-backend "$KEYRING_BACKEND" \
			--gas auto --gas-adjustment 1.3 \
			--gas-prices "${TX_GAS_PRICES}" \
			--output json --yes 2>/dev/null || true)"
		txhash="$(echo "$send_json" | jq -r '.txhash // empty')"
		if [[ -n "$txhash" ]]; then
			wait_for_tx "$txhash" || echo "[SNCLI] WARN: funding tx may not have confirmed"
		fi
	fi
}

# Extract the numeric suffix from MONIKER (e.g. "supernova_validator_3" → 3)
validator_number() {
	local num
	num="$(echo "${MONIKER}" | grep -oE '[0-9]+$' || true)"
	if [[ -z "$num" ]]; then
		num=1
	fi
	printf '%s' "$num"
}

# If EVM migration is pending (evm_key_name is set), stagger startup so that
# validators don't all migrate simultaneously. Validator N waits (N-1)*5 seconds.
maybe_stagger_for_evm_migration() {
	local evm_key_name val_num delay

	if [[ ! -f "$SN_CONFIG" ]]; then
		return 0
	fi

	evm_key_name="$(get_supernode_config_value "$SN_CONFIG" "evm_key_name")"
	if [[ -z "$evm_key_name" ]]; then
		return 0
	fi

	val_num="$(validator_number)"
	delay=$(( (val_num - 1) * 5 ))
	if ((delay > 0)); then
		echo "[SN] EVM migration pending — staggering startup by ${delay}s (validator ${val_num})."
		sleep "$delay"
	fi
}

# Full supernode start sequence: wait for chain progress, optionally stagger
# for EVM migration, launch process, then check if a post-migration restart
# is needed.
start_supernode() {
	if supernode_is_running; then
		echo "[SN] Supernode already running, skipping start."
	else
		echo "[SN] Waiting for at least one new block before starting supernode..."
		wait_for_n_blocks 1 || {
			echo "[SN] Chain not progressing; cannot start supernode."
			return 1
		}
		maybe_stagger_for_evm_migration
		echo "[SN] Starting supernode..."
		export P2P_USE_EXTERNAL_IP=false
		start_supernode_process
		echo "[SN] Supernode started on ${SN_ENDPOINT}, logging to $SN_LOG"
		restart_supernode_after_evm_migration_if_needed || return 1
	fi
}

stop_supernode_if_running() {
	if supernode_is_running; then
		stop_supernode_process || return 1
	else
		echo "[SN] Supernode is not running."
	fi
}

# ═════════════════════════════════════════════════════════════════════════════
# BINARY INSTALLATION
# Copy supernode and sncli binaries from the shared release directory
# (populated by `make devnet-build-*`) into /usr/local/bin/.
# Both binaries are optional — the script exits cleanly if they're missing.
# ═════════════════════════════════════════════════════════════════════════════

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

# ═════════════════════════════════════════════════════════════════════════════
# ON-CHAIN REGISTRATION
# Submit MsgRegisterSupernode to associate this validator with its supernode
# endpoint and account. Checks current state first to avoid duplicate
# registration or re-registering in a blocked state (postponed/disabled/etc).
# ═════════════════════════════════════════════════════════════════════════════

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

# ═════════════════════════════════════════════════════════════════════════════
# SUPERNODE CONFIGURATION
# Initialize supernode config.yml, create/recover keys, derive addresses,
# set up P2P listen address, handle EVM migration keys, and fund the account.
# ═════════════════════════════════════════════════════════════════════════════

# Patch the p2p.listen_address field in config.yml to use the container's IP.
# Uses sed to manipulate the YAML (no parser available in the container).
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

# Main supernode configuration function. Handles the full key + config + funding
# flow in this order:
#   1. Create or recover the supernode key (three sources: config mnemonic,
#      persisted mnemonic file, or generate new)
#   2. Resolve addresses (supernode, validator, valoper, genesis)
#   3. Initialize config.yml via `supernode init` if it doesn't exist
#   4. Prepare EVM migration keys if applicable
#   5. Fund the supernode account if balance < 1M ulume
configure_supernode() {
	echo "[SN] Ensuring SN key exists..."
	mkdir -p "$SN_BASEDIR" "${NODE_STATUS_DIR}"

	# Key recovery priority:
	#   1. Pre-configured mnemonic from config.json (deterministic across rebuilds)
	#   2. Previously persisted mnemonic file (survives container restart)
	#   3. Generate a fresh key (first run with no config)
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

	# Resolve all addresses needed for registration and funding
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

	# Initialize supernode config.yml on first run. The `supernode init` command
	# creates config.yml, sets up the keyring under ~/.supernode/keys/, and
	# records the key_name, chain_id, and gRPC endpoint.
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

	# Derive EVM keys if the chain has been upgraded past the EVM cutover version
	maybe_prepare_supernode_migration "$(cat "$SN_MNEMONIC_FILE")"

	# Re-read the supernode address from config — migration may have changed
	# which key is active (e.g. from legacy to evm_key_name)
	local configured_sn_key_name
	configured_sn_key_name="$(get_supernode_config_value "$SN_CONFIG" "key_name")"
	if [[ -n "$configured_sn_key_name" ]]; then
		SN_ADDR="$(run_capture $DAEMON keys show "$configured_sn_key_name" -a --keyring-backend "$KEYRING_BACKEND")"
		echo "[SN] Supernode address (${configured_sn_key_name}): $SN_ADDR"
		echo "$SN_ADDR" >"$SN_ADDR_FILE"
	fi

	# Fund the supernode account from the validator's genesis account if balance
	# is below 1M ulume. The supernode needs funds to pay gas for its own txs.
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

# Copy sncli binary from shared release dir (if present) to /usr/local/bin/
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

# ═════════════════════════════════════════════════════════════════════════════
# SNCLI CONFIGURATION
# sncli is an optional CLI client for the supernode's gRPC/P2P API.
# It gets its own keyring key ("sncli-account"), funded separately, and a
# TOML config file with connection endpoints. Uses crudini for INI editing.
# ═════════════════════════════════════════════════════════════════════════════

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

	# Create/recover sncli key (same priority as supernode: config → file → generate)
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

	# Write sncli connection config — points to this container's local endpoints
	# [lumera] section: chain connection
	crudini --set "${SNCLI_CFG}" lumera grpc_addr "\"localhost:${LUMERA_GRPC_PORT}\""
	crudini --set "${SNCLI_CFG}" lumera chain_id "\"${CHAIN_ID}\""

	# [supernode] section: supernode gRPC and P2P addresses
	if [ -n "${SN_ADDR:-}" ]; then
		crudini --set "${SNCLI_CFG}" supernode address "\"${SN_ADDR}\""
	fi
	crudini --set "${SNCLI_CFG}" supernode grpc_endpoint "\"${IP_ADDR}:${SN_PORT}\""
	crudini --set "${SNCLI_CFG}" supernode p2p_endpoint "\"${IP_ADDR}:${SN_P2P_PORT}\""

	# [keyring] section: sncli's own account for signing requests
	crudini --set "${SNCLI_CFG}" keyring backend "\"${KEYRING_BACKEND}\""
	crudini --set "${SNCLI_CFG}" keyring key_name "\"${SNCLI_KEY_NAME}\""
	crudini --set "${SNCLI_CFG}" keyring local_address "\"$addr\""

}

# ═════════════════════════════════════════════════════════════════════════════
# MAIN EXECUTION
#
# Execution order:
#   1. Prerequisites check (crudini)
#   2. Stop any leftover supernode from a prior run
#   3. Install binaries (supernode + sncli) from shared release dir
#   4. Wait for chain readiness (RPC up + height >= 5)
#   5. Detect EVM gas pricing (feemarket module)
#   6. Load pre-configured mnemonics from config.json
#   7. Configure supernode (keys, config.yml, funding)
#   8. Register supernode on-chain (MsgRegisterSupernode)
#   9. Configure sncli (key, config, funding)
#  10. Start supernode process
# ═════════════════════════════════════════════════════════════════════════════

require_crudini
stop_supernode_if_running
install_supernode_binary
install_sncli_binary

# Wait for chain — exit cleanly (don't fail the container) if chain isn't ready
wait_for_lumera || exit 0
# Require at least 5 blocks to ensure genesis is settled and state is queryable
wait_for_height_at_least 5 || {
	echo "[SN] Lumera chain not producing blocks in time; exiting."
	exit 1
}

update_gas_prices_for_evm      # Detect EVM feemarket pricing if active
load_configured_mnemonics      # Load deterministic mnemonics from config.json
configure_supernode            # Keys + config.yml + fund account
register_supernode             # On-chain MsgRegisterSupernode
configure_sncli                # sncli key + config + fund account
start_supernode                # Launch supernode process
