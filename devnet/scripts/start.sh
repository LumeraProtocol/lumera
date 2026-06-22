#!/usr/bin/env bash
# --------------------------------------------------------------------------------------------------
# start.sh — Entrypoint for Lumera devnet validators & supernode
#
# MODES (set via START_MODE environment variable):
#   auto  (default)  If setup_complete flag is missing, launches supernode-setup.sh & validator-setup.sh
#                    scripts in the background. Then waits for setup_complete, starts lumerad,
#                    and tails logs.
#
#   bootstrap        Runs setup scripts supernode-setup.sh & validator-setup.sh in the foreground.
#                    Exits when setup_complete is created. Does NOT start lumerad.
#
#   run              Waits for setup_complete, starts lumerad, and tails logs.
#
#   wait  (optional) Wait for setup_complete and exit.
#
# DOCKER COMPOSE:
#   - Image ENTRYPOINT should be: ["/bin/bash", "/root/scripts/start.sh"] (as in Dockerfile).
#     # One-time network bootstrap (creates final genesis & setup_complete, exits)
#     START_MODE=bootstrap docker compose up --build --abort-on-container-exit
#
#     # Steady state: start validators using finalized genesis
#     START_MODE=run       docker compose up -d
# --------------------------------------------------------------------------------------------------
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/common.sh"

START_MODE="${START_MODE:-auto}"

SHARED_DIR="/shared"
CFG_DIR="${SHARED_DIR}/config"
CFG_CHAIN="${CFG_DIR}/config.json"
CFG_VALS="${CFG_DIR}/validators.json"
RELEASE_DIR="${SHARED_DIR}/release"
NM_UI_DIR="${RELEASE_DIR}/uploader-ui"
STATUS_DIR="${SHARED_DIR}/status"
SETUP_COMPLETE="${STATUS_DIR}/setup_complete"
SN="supernode-linux-amd64"
# Detect which uploader binary is in the release dir
if [ -f "${RELEASE_DIR}/lumera-uploader" ]; then
	NM="lumera-uploader"
elif [ -f "${RELEASE_DIR}/network-maker" ]; then
	NM="network-maker"
else
	NM="lumera-uploader"
fi
LUMERAD="lumerad"
LUMERA_SRC_BIN="${RELEASE_DIR}/${LUMERAD}"
LUMERA_DST_BIN="/usr/local/bin/${LUMERAD}"
WASMVM_SRC_LIB="${RELEASE_DIR}/libwasmvm.x86_64.so"
WASMVM_DST_LIB="/usr/lib/libwasmvm.x86_64.so"

DAEMON="${DAEMON:-lumerad}"
DAEMON_HOME="${DAEMON_HOME:-/root/.lumera}"

SCRIPTS_DIR="/root/scripts"
LOGS_DIR="/root/logs"
OLD_LOGS_DIR="${LOGS_DIR}/old"
VALIDATOR_LOG="${LOGS_DIR}/validator.log"
SUPERNODE_LOG="${LOGS_DIR}/supernode.log"
VALIDATOR_SETUP_OUT="${LOGS_DIR}/validator-setup.out"
SUPERNODE_SETUP_OUT="${LOGS_DIR}/supernode-setup.out"
UPLOADER_SETUP_OUT="${LOGS_DIR}/lumera-uploader-setup.out"
TEST_ACCOUNTS_SETUP_OUT="${LOGS_DIR}/test-accounts-setup.out"
NM_UI_PORT="${NM_UI_PORT:-8088}"

LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
LUMERA_RPC_ADDR="http://localhost:${LUMERA_RPC_PORT}"

mkdir -p "${LOGS_DIR}" "${OLD_LOGS_DIR}" "${DAEMON_HOME}/config" "${STATUS_DIR}"

# Require MONIKER env (compose already sets it)
: "${MONIKER:?MONIKER environment variable must be set}"
echo "[BOOT] ${MONIKER}: start.sh (mode=${START_MODE})"

NODE_STATUS_DIR="${STATUS_DIR}/${MONIKER}"
NODE_SETUP_COMPLETE="${NODE_STATUS_DIR}/setup_complete"
mkdir -p "${NODE_STATUS_DIR}"

if ! command -v jq >/dev/null 2>&1; then
	echo "[BOOT] jq is missing"
fi

if [ ! -f "${CFG_CHAIN}" ]; then
	echo "[BOOT] Missing ${CFG_CHAIN}"
	exit 1
fi

if [ ! -f "${CFG_VALS}" ]; then
	echo "[BOOT] Missing ${CFG_VALS}"
	exit 1
fi

PRIMARY_MONIKER="$(jq -r '
  (map(select(.primary==true)) | if length>0 then .[0].moniker else empty end)
  // (.[0].moniker)
' "${CFG_VALS}")"

if [ -z "${PRIMARY_MONIKER}" ] || [ "${PRIMARY_MONIKER}" = "null" ]; then
	echo "[BOOT] Unable to determine primary validator from ${CFG_VALS}"
	exit 1
fi

PRIMARY_STARTED_FLAG="${STATUS_DIR}/${PRIMARY_MONIKER}/lumerad_started"

wait_for_flag() {
	local f="$1"
	until [ -s "${f}" ]; do sleep 1; done
}

inject_nm_ui_env() {
	local api_base="${VITE_API_BASE:-}"
	[ -z "${api_base}" ] && return 0
	[ -d "${NM_UI_DIR}" ] || return 0

	local files
	files="$(grep -rl "http://127.0.0.1:8080" "${NM_UI_DIR}" || true)"
	if [ -z "${files}" ]; then
		echo "[BOOT] ${NM} UI: no API base placeholder found to inject."
		return 0
	fi

	local escaped_base="${api_base//\//\\/}"
	escaped_base="${escaped_base//&/\\&}"
	echo "[BOOT] ${NM} UI: injecting API base ${api_base}"
	# Replace default API base baked into the static bundle with runtime value
	while IFS= read -r f; do
		sed -i "s|http://127.0.0.1:8080|${escaped_base}|g" "$f"
	done <<<"${files}"
}

start_nm_ui_if_present() {
	if [ ! -d "${NM_UI_DIR}" ] || [ ! -f "${NM_UI_DIR}/index.html" ]; then
		echo "[BOOT] ${NM} UI not found at ${NM_UI_DIR}; skipping nginx"
		return
	fi

	inject_nm_ui_env

	cat >/etc/nginx/conf.d/lumera-uploader-ui.conf <<EOF
server {
    listen ${NM_UI_PORT};
    server_name _;
    root ${NM_UI_DIR};
    index index.html;
    location / {
        try_files \$uri \$uri/ /index.html;
    }
}
EOF

	if pgrep -x nginx >/dev/null 2>&1; then
		echo "[BOOT] nginx already running; skipping start for ${NM} UI."
		return
	fi

	echo "[BOOT] Starting nginx to serve ${NM} UI on port ${NM_UI_PORT}"
	nginx
}

archive_log_file() {
	local log_file="$1"
	local ts base target suffix=1

	[ -f "${log_file}" ] || return 0
	[ -s "${log_file}" ] || return 0

	ts="$(date '+%Y%m%d_%H_%M')"
	base="$(basename "${log_file}")"
	target="${OLD_LOGS_DIR}/${ts}.${base}"

	while [ -e "${target}" ]; do
		target="${OLD_LOGS_DIR}/${ts}.${suffix}.${base}"
		suffix=$((suffix + 1))
	done

	mv "${log_file}" "${target}"
	echo "[BOOT] Archived ${log_file} -> ${target}"
}

archive_existing_logs() {
	archive_log_file "${VALIDATOR_LOG}"
	archive_log_file "${SUPERNODE_LOG}"
	archive_log_file "${VALIDATOR_SETUP_OUT}"
	archive_log_file "${SUPERNODE_SETUP_OUT}"
	archive_log_file "${UPLOADER_SETUP_OUT}"
	archive_log_file "${TEST_ACCOUNTS_SETUP_OUT}"
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

	echo "[BOOT] Waiting for block height >= ${target} ..."
	for ((i = 0; i < retries; i++)); do
		local h
		h="$(current_height)"
		if ((h >= target)); then
			echo "[BOOT] Height is ${h} (>= ${target}) — OK."
			return 0
		fi
		sleep "$delay"
	done
	echo "[BOOT] Timeout waiting for height >= ${target}."
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

launch_supernode_setup() {
	# Start optional supernode setup only in auto/run modes after init is done.
	if [ -x "${SCRIPTS_DIR}/supernode-setup.sh" ] && [ -f "${RELEASE_DIR}/${SN}" ]; then
		echo "[BOOT] ${MONIKER}: Launching Supernode setup in background..."
		export LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
		export LUMERA_GRPC_PORT="${LUMERA_GRPC_PORT:-9090}"
		nohup bash "${SCRIPTS_DIR}/supernode-setup.sh" >"${SUPERNODE_SETUP_OUT}" 2>&1 &
	fi
}

wait_for_validator_setup() {
	echo "[BOOT] ${MONIKER}: Waiting for validator setup to complete..."
	wait_for_flag "${SETUP_COMPLETE}"
	wait_for_flag "${NODE_SETUP_COMPLETE}"
	echo "[BOOT] ${MONIKER}: validator setup complete."
}

install_wasm_lib() {
	if [ -f "${WASMVM_SRC_LIB}" ]; then
		if [ -f "${WASMVM_DST_LIB}" ] && cmp -s "${WASMVM_SRC_LIB}" "${WASMVM_DST_LIB}"; then
			echo "[BOOT] libwasmvm.x86_64.so already up to date at ${WASMVM_DST_LIB}"
			return
		fi
		echo "[BOOT] Installing libwasmvm.x86_64.so to ${WASMVM_DST_LIB}"
		run cp -f "${WASMVM_SRC_LIB}" "${WASMVM_DST_LIB}"
		run chmod 755 "${WASMVM_DST_LIB}"
	else
		echo "[BOOT] ${WASMVM_SRC_LIB} not found, assuming libwasmvm.x86_64.so is already installed"
	fi
}

install_lumerad_binary() {
	run cp -f "${LUMERA_SRC_BIN}" "${LUMERA_DST_BIN}"
	run chmod +x "${LUMERA_DST_BIN}"
}

install_or_update_lumerad() {
	if [ ! -f "${LUMERA_DST_BIN}" ]; then
		if [ -f "${LUMERA_SRC_BIN}" ]; then
			echo "[BOOT] ${LUMERAD} binary not found at ${LUMERA_DST_BIN}, installing..."
			install_lumerad_binary
		else
			echo "[BOOT] ${LUMERA_SRC_BIN} not found, assuming ${LUMERAD} is already installed"
		fi
	else
		run lumerad version || true
		if [ -f "${LUMERA_SRC_BIN}" ]; then
			if cmp -s "${LUMERA_SRC_BIN}" "${LUMERA_DST_BIN}"; then
				echo "[BOOT] ${LUMERAD} binary already up to date at ${LUMERA_DST_BIN}"
			else
				echo "[BOOT] Updating ${LUMERAD} binary at ${LUMERA_DST_BIN}"
				install_lumerad_binary
			fi
		else
			echo "[BOOT] ${LUMERA_SRC_BIN} not found, assuming ${LUMERAD} is already installed"
		fi
	fi
	install_wasm_lib
	run lumerad version || true
}

launch_validator_setup() {
	install_or_update_lumerad
	if [ ! -s "${NODE_SETUP_COMPLETE}" ] && [ -x "${SCRIPTS_DIR}/validator-setup.sh" ]; then
		echo "[BOOT] ${MONIKER}: launching validator-setup in background..."
		nohup bash "${SCRIPTS_DIR}/validator-setup.sh" >"${VALIDATOR_SETUP_OUT}" 2>&1 &
	fi
}

launch_uploader_setup() {
	if [ -x "${SCRIPTS_DIR}/lumera-uploader-setup.sh" ] && [ -f "${RELEASE_DIR}/${NM}" ]; then
		echo "[BOOT] ${MONIKER}: Launching Lumera Uploader setup in background..."
		nohup bash "${SCRIPTS_DIR}/lumera-uploader-setup.sh" >"${UPLOADER_SETUP_OUT}" 2>&1 &
	fi
}

launch_test_accounts_setup() {
	# Only fire if the validators.json entry for this node has a non-empty
	# test_accounts block with count > 0. Fund and creation run against the
	# live chain so this is launched in background after start_lumera.
	if [ ! -x "${SCRIPTS_DIR}/test-accounts-setup.sh" ]; then
		return
	fi
	local count
	count="$(jq -r --arg m "${MONIKER}" '
		[.[] | select(.moniker==$m)][0] | try .test_accounts.count // 0
	' "${CFG_VALS}" 2>/dev/null || echo 0)"
	if ! [[ "${count}" =~ ^[0-9]+$ ]] || [ "${count}" -eq 0 ]; then
		return
	fi
	echo "[BOOT] ${MONIKER}: Launching test-accounts setup in background (count=${count})..."
	nohup bash "${SCRIPTS_DIR}/test-accounts-setup.sh" >"${TEST_ACCOUNTS_SETUP_OUT}" 2>&1 &
}

start_lumera() {
	if [ "${MONIKER}" != "${PRIMARY_MONIKER}" ]; then
		echo "[BOOT] ${MONIKER}: Waiting for primary (${PRIMARY_MONIKER}) to start lumerad..."
		wait_for_flag "${PRIMARY_STARTED_FLAG}"
	fi

	echo "[BOOT] ${MONIKER}: Starting lumerad..."
	CLAIMS_LOCAL="${DAEMON_HOME}/config/claims.csv"
	EXTRA_START_FLAGS="$(lumerad_claims_start_flags "${DAEMON}" "${CLAIMS_LOCAL}")"
	if [ -n "${EXTRA_START_FLAGS}" ]; then
		echo "[BOOT] ${MONIKER}: Claims CSV found, loading claim records at genesis"
	fi
	# The EVM JSON-RPC metrics server is enabled by the --metrics start flag and
	# only exists on EVM-enabled builds; pre-EVM lumerad does not know the flag.
	if lumera_supports_evm; then
		local enable_metrics
		enable_metrics="$(jq -r '.["json-rpc"].enable_metrics // true' "${CFG_CHAIN}" 2>/dev/null || echo true)"
		if [ "${enable_metrics}" = "true" ]; then
			EXTRA_START_FLAGS="${EXTRA_START_FLAGS} --metrics"
			echo "[BOOT] ${MONIKER}: EVM build detected, enabling JSON-RPC metrics (--metrics)"
		fi
	fi
	echo "+ ${DAEMON} start --home ${DAEMON_HOME} ${EXTRA_START_FLAGS}"
	# shellcheck disable=SC2086
	"${DAEMON}" start --home "${DAEMON_HOME}" ${EXTRA_START_FLAGS} >"${VALIDATOR_LOG}" 2>&1 &
	LUMERAD_PID=$!
	echo "[BOOT] ${MONIKER}: lumerad started, pid=${LUMERAD_PID}"

	if [ "${MONIKER}" = "${PRIMARY_MONIKER}" ]; then
		mkdir -p "$(dirname "${PRIMARY_STARTED_FLAG}")"
		printf 'started\n' >"${PRIMARY_STARTED_FLAG}"
		echo "[BOOT] ${MONIKER}: Marked primary lumerad as started."
	fi
}

tail_logs() {
	touch "${VALIDATOR_LOG}" "${SUPERNODE_LOG}" "${SUPERNODE_SETUP_OUT}" "${VALIDATOR_SETUP_OUT}" "${UPLOADER_SETUP_OUT}" "${TEST_ACCOUNTS_SETUP_OUT}"
	tail -F "${VALIDATOR_LOG}" "${SUPERNODE_LOG}" "${SUPERNODE_SETUP_OUT}" "${VALIDATOR_SETUP_OUT}" "${UPLOADER_SETUP_OUT}" "${TEST_ACCOUNTS_SETUP_OUT}" &
	TAIL_PID=$!
}

# Wait on the lumerad process and propagate its exit code as the container's
# exit code. If lumerad dies (crash, SIGKILL on host that matches `pkill -f
# 'lumerad start'`, OOM, etc.) PID 1 exits too. The docker-compose
# `restart: unless-stopped` policy handles recovery on any container exit while
# the propagated code keeps crash/kill status visible to docker / observability.
#
# History: 2026-06-02 — a host `pkill -9 -f 'lumerad start'` matched lumerad
# inside the 5 validator containers. PID 1 was bash + tail -F, so containers
# stayed "Up" for 6 days while chain was dead. See PR description.
wait_for_lumera() {
	if [ -z "${LUMERAD_PID:-}" ]; then
		echo "[BOOT] ${MONIKER}: lumerad pid not set; cannot supervise."
		# Fall back to old tail-forever behaviour rather than exit 0 silently.
		wait "${TAIL_PID:-}" 2>/dev/null || true
		return 0
	fi
	# `wait <pid>` returns the exit status of that process. We deliberately do
	# NOT use `set -e` here so we can capture the code.
	set +e
	wait "${LUMERAD_PID}"
	local rc=$?
	set -e
	echo "[BOOT] ${MONIKER}: lumerad exited rc=${rc} — terminating container so docker restart policy can recover."
	if [ -n "${TAIL_PID:-}" ]; then
		kill "${TAIL_PID}" 2>/dev/null || true
	fi
	# Sleep briefly so the last log lines are flushed by tail -F before we exit.
	sleep 1
	exit "${rc}"
}

run_auto_flow() {
	archive_existing_logs
	launch_uploader_setup
	launch_supernode_setup
	launch_validator_setup
	wait_for_validator_setup
	start_lumera
	launch_test_accounts_setup
	start_nm_ui_if_present
	tail_logs
	wait_for_lumera
}

case "${START_MODE}" in
auto | "")
	run_auto_flow
	;;

bootstrap)
	archive_existing_logs
	launch_uploader_setup
	launch_supernode_setup
	launch_validator_setup
	wait_for_validator_setup
	exit 0
	;;

run)
	archive_existing_logs
	wait_for_validator_setup
	wait_for_n_blocks 3 || {
		echo "[SN] Lumera chain not producing blocks in time; exiting."
		exit 1
	}
	start_lumera
	launch_test_accounts_setup
	start_nm_ui_if_present
	tail_logs
	wait_for_lumera
	;;

wait)
	wait_for_validator_setup
	exit 0
	;;

*)
	echo "[BOOT] Unknown START_MODE='${START_MODE}', defaulting to auto."
	run_auto_flow
	;;
esac
