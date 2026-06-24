#!/usr/bin/env bash
# restart.sh — restart devnet services inside a validator container
# Usage:
#   ./restart.sh                   # restart all known services
#   ./restart.sh uploader|sn|lumera|nginx
set -euo pipefail

DAEMON="${DAEMON:-lumerad}"
DAEMON_HOME="${DAEMON_HOME:-/root/.lumera}"
SN_BASEDIR="${SN_BASEDIR:-/root/.supernode}"

LOGS_DIR="${LOGS_DIR:-/root/logs}"
OLD_LOGS_DIR="${OLD_LOGS_DIR:-${LOGS_DIR}/old}"
VALIDATOR_LOG="${VALIDATOR_LOG:-${LOGS_DIR}/validator.log}"
SN_LOG="${SN_LOG:-${LOGS_DIR}/supernode.log}"
NM_LOG="${NM_LOG:-${LOGS_DIR}/lumera-uploader.log}"

SHARED_DIR="/shared"
RELEASE_DIR="${SHARED_DIR}/release"
NM_UI_DIR="${NM_UI_DIR:-${RELEASE_DIR}/uploader-ui}"
NM_UI_PORT="${NM_UI_PORT:-8088}"

LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_RPC_ADDR="${LUMERA_RPC_ADDR:-http://localhost:${LUMERA_RPC_PORT}}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/common.sh"
STOP_SCRIPT="${STOP_SCRIPT:-${SCRIPT_DIR}/stop.sh}"

log() {
	echo "[RESTART] $*"
}

# Match a process by binary basename. Uses pgrep -f with an anchored pattern
# instead of -x because the kernel truncates `comm` to 15 chars
# (TASK_COMM_LEN), which makes -x emit a warning and return no matches for
# longer names like "supernode-linux-amd64".
proc_running() {
	pgrep -f "(^|/)${1}( |$)" >/dev/null 2>&1
}

run_stop() {
	if [ ! -f "${STOP_SCRIPT}" ]; then
		log "stop.sh not found at ${STOP_SCRIPT}"
		exit 1
	fi
	bash "${STOP_SCRIPT}" "$@"
}

ensure_logs_dir() {
	mkdir -p "${LOGS_DIR}" "${OLD_LOGS_DIR}"
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
	log "Archived ${log_file} -> ${target}"
}

start_lumera() {
	local pattern="${DAEMON} start --home ${DAEMON_HOME}"

	if pgrep -f "${pattern}" >/dev/null 2>&1 || proc_running "${DAEMON}"; then
		log "${DAEMON} already running."
		return 0
	fi

	if ! command -v "${DAEMON}" >/dev/null 2>&1; then
		log "Binary ${DAEMON} not found in PATH."
		return 1
	fi

	ensure_logs_dir
	archive_log_file "${VALIDATOR_LOG}"
	mkdir -p "$(dirname "${VALIDATOR_LOG}")" "${DAEMON_HOME}/config"

	CLAIMS_LOCAL="${DAEMON_HOME}/config/claims.csv"
	EXTRA_START_FLAGS="$(lumerad_claims_start_flags "${DAEMON}" "${CLAIMS_LOCAL}")"
	log "Starting ${DAEMON}..."
	# shellcheck disable=SC2086
	"${DAEMON}" start --home "${DAEMON_HOME}" ${EXTRA_START_FLAGS} >"${VALIDATOR_LOG}" 2>&1 &
	log "${DAEMON} start requested; logging to ${VALIDATOR_LOG}"
}

start_supernode() {
	local names=("supernode-linux-amd64" "supernode")
	local running=0

	for name in "${names[@]}"; do
		if proc_running "${name}"; then
			running=1
			break
		fi
	done

	if ((running == 1)); then
		log "Supernode already running."
		return 0
	fi

	local bin=""
	for name in "${names[@]}"; do
		if command -v "${name}" >/dev/null 2>&1; then
			bin="${name}"
			break
		fi
	done

	if [ -z "${bin}" ]; then
		log "Supernode binary not found; skipping start."
		return 0
	fi

	ensure_logs_dir
	archive_log_file "${SN_LOG}"
	mkdir -p "$(dirname "${SN_LOG}")" "${SN_BASEDIR}"

	log "Starting supernode (${bin})..."
	P2P_USE_EXTERNAL_IP=${P2P_USE_EXTERNAL_IP:-false} "${bin}" start -d "${SN_BASEDIR}" >"${SN_LOG}" 2>&1 &
	log "Supernode start requested; logging to ${SN_LOG}"
}

start_uploader() {
	# Try lumera-uploader first, fall back to network-maker
	local name=""
	for candidate in "lumera-uploader" "network-maker"; do
		if proc_running "${candidate}"; then
			log "${candidate} already running."
			return 0
		fi
		if [[ -z "${name}" ]] && command -v "${candidate}" >/dev/null 2>&1; then
			name="${candidate}"
		fi
	done

	if [[ -z "${name}" ]]; then
		log "lumera-uploader binary not found; skipping start."
		return 0
	fi

	ensure_logs_dir
	archive_log_file "${NM_LOG}"
	mkdir -p "$(dirname "${NM_LOG}")"

	log "Starting ${name}..."
	"${name}" >"${NM_LOG}" 2>&1 &
	log "${name} start requested; logging to ${NM_LOG}"
}

start_nginx() {
	if proc_running nginx; then
		log "nginx already running."
		return 0
	fi

	if [ ! -d "${NM_UI_DIR}" ] || [ ! -f "${NM_UI_DIR}/index.html" ]; then
		log "lumera-uploader UI not found at ${NM_UI_DIR}; skipping nginx start."
		return 0
	fi

	mkdir -p /etc/nginx/conf.d
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

	log "Starting nginx to serve lumera-uploader UI on port ${NM_UI_PORT}..."
	nginx
	log "nginx start requested."
}

restart_all() {
	run_stop all
	start_lumera
	start_supernode
	start_uploader
	start_nginx
}

restart_uploader() {
	run_stop uploader
	start_uploader
}

restart_sn() {
	run_stop sn
	start_supernode
}

restart_nginx() {
	run_stop nginx
	start_nginx
}

restart_lumera() {
	run_stop lumera
	start_lumera
}

usage() {
	echo "Usage: $0 [uploader|sn|lumera|nginx|all]" >&2
	exit 1
}

target="${1:-all}"
case "${target}" in
uploader | nm | network-maker | lumera-uploader | ul)
	restart_uploader
	;;
sn | supernode)
	restart_sn
	;;
nginx | ui)
	restart_nginx
	;;
lumera | lumerad | chain)
	restart_lumera
	;;
all | "")
	restart_all
	;;
*)
	usage
	;;
esac
