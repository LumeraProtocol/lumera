#!/usr/bin/env bash
# restart.sh — restart devnet services inside a validator container
# Usage:
#   ./restart.sh                   # restart all known services
#   ./restart.sh nm|sn|lumera|nginx
set -euo pipefail

DAEMON="${DAEMON:-lumerad}"
DAEMON_HOME="${DAEMON_HOME:-/root/.lumera}"
SN_BASEDIR="${SN_BASEDIR:-/root/.supernode}"

LOGS_DIR="${LOGS_DIR:-/root/logs}"
VALIDATOR_LOG="${VALIDATOR_LOG:-${LOGS_DIR}/validator.log}"
SN_LOG="${SN_LOG:-${LOGS_DIR}/supernode.log}"
NM_LOG="${NM_LOG:-${LOGS_DIR}/network-maker.log}"

SHARED_DIR="/shared"
RELEASE_DIR="${SHARED_DIR}/release"
NM_UI_DIR="${NM_UI_DIR:-${RELEASE_DIR}/nm-ui}"
NM_UI_PORT="${NM_UI_PORT:-8088}"

LUMERA_RPC_PORT="${LUMERA_RPC_PORT:-26657}"
LUMERA_RPC_ADDR="${LUMERA_RPC_ADDR:-http://localhost:${LUMERA_RPC_PORT}}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STOP_SCRIPT="${STOP_SCRIPT:-${SCRIPT_DIR}/stop.sh}"

log() {
	echo "[RESTART] $*"
}

run_stop() {
	if [ ! -f "${STOP_SCRIPT}" ]; then
		log "stop.sh not found at ${STOP_SCRIPT}"
		exit 1
	fi
	bash "${STOP_SCRIPT}" "$@"
}

ensure_logs_dir() {
	mkdir -p "${LOGS_DIR}"
}

start_lumera() {
	local pattern="${DAEMON} start --home ${DAEMON_HOME}"

	if pgrep -f "${pattern}" >/dev/null 2>&1 || pgrep -x "${DAEMON}" >/dev/null 2>&1; then
		log "${DAEMON} already running."
		return 0
	fi

	if ! command -v "${DAEMON}" >/dev/null 2>&1; then
		log "Binary ${DAEMON} not found in PATH."
		return 1
	fi

	ensure_logs_dir
	mkdir -p "$(dirname "${VALIDATOR_LOG}")" "${DAEMON_HOME}/config"

	log "Starting ${DAEMON}..."
	"${DAEMON}" start --home "${DAEMON_HOME}" >"${VALIDATOR_LOG}" 2>&1 &
	log "${DAEMON} start requested; logging to ${VALIDATOR_LOG}"
}

start_supernode() {
	local names=("supernode-linux-amd64" "supernode")
	local running=0

	for name in "${names[@]}"; do
		if pgrep -x "${name}" >/dev/null 2>&1; then
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
	mkdir -p "$(dirname "${SN_LOG}")" "${SN_BASEDIR}"

	log "Starting supernode (${bin})..."
	P2P_USE_EXTERNAL_IP=${P2P_USE_EXTERNAL_IP:-false} "${bin}" start -d "${SN_BASEDIR}" >"${SN_LOG}" 2>&1 &
	log "Supernode start requested; logging to ${SN_LOG}"
}

start_network_maker() {
	local name="network-maker"

	if pgrep -x "${name}" >/dev/null 2>&1; then
		log "network-maker already running."
		return 0
	fi

	if ! command -v "${name}" >/dev/null 2>&1; then
		log "network-maker binary not found; skipping start."
		return 0
	fi

	ensure_logs_dir
	mkdir -p "$(dirname "${NM_LOG}")"

	log "Starting network-maker..."
	"${name}" >"${NM_LOG}" 2>&1 &
	log "network-maker start requested; logging to ${NM_LOG}"
}

start_nginx() {
	if pgrep -x nginx >/dev/null 2>&1; then
		log "nginx already running."
		return 0
	fi

	if [ ! -d "${NM_UI_DIR}" ] || [ ! -f "${NM_UI_DIR}/index.html" ]; then
		log "network-maker UI not found at ${NM_UI_DIR}; skipping nginx start."
		return 0
	fi

	mkdir -p /etc/nginx/conf.d
	cat >/etc/nginx/conf.d/network-maker-ui.conf <<EOF
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

	log "Starting nginx to serve network-maker UI on port ${NM_UI_PORT}..."
	nginx
	log "nginx start requested."
}

restart_all() {
	run_stop all
	start_lumera
	start_supernode
	start_network_maker
	start_nginx
}

restart_nm() {
	run_stop nm
	start_network_maker
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
	echo "Usage: $0 [nm|sn|lumera|nginx|all]" >&2
	exit 1
}

target="${1:-all}"
case "${target}" in
nm | network-maker)
	restart_nm
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
