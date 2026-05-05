#!/usr/bin/env bash
# stop.sh — stop devnet services inside a validator container
# Usage:
#   ./stop.sh                    # stop all known services
#   ./stop.sh uploader|sn|lumera|nginx
set -euo pipefail

DAEMON="${DAEMON:-lumerad}"
DAEMON_HOME="${DAEMON_HOME:-/root/.lumera}"
SN_BASEDIR="${SN_BASEDIR:-/root/.supernode}"

log() {
	echo "[STOP] $*"
}

# Match a process by binary basename. Uses pgrep/pkill -f with an anchored
# pattern instead of -x because the kernel truncates `comm` to 15 chars
# (TASK_COMM_LEN), which makes -x emit a warning and return no matches for
# longer names like "supernode-linux-amd64".
proc_running() {
	pgrep -f "(^|/)${1}( |$)" >/dev/null 2>&1
}

proc_kill() {
	pkill -f "(^|/)${1}( |$)" || true
}

stop_uploader() {
	local stopped=0
	# Handle both new (lumera-uploader) and old (network-maker) binary names
	for name in "lumera-uploader" "network-maker"; do
		if proc_running "${name}"; then
			log "Stopping ${name}..."
			proc_kill "${name}"
			log "${name} stop requested."
			stopped=1
		fi
	done
	if ((stopped == 0)); then
		log "lumera-uploader is not running."
	fi
}

stop_sn() {
	local stopped=0
	local names=("supernode-linux-amd64" "supernode")

	for name in "${names[@]}"; do
		if proc_running "${name}"; then
			stopped=1
			log "Stopping supernode (${name})..."
			if command -v "${name}" >/dev/null 2>&1; then
				"${name}" stop -d "${SN_BASEDIR}" >/dev/null 2>&1 || proc_kill "${name}"
			else
				proc_kill "${name}"
			fi
		fi
	done

	if ((stopped == 0)); then
		log "Supernode is not running."
	else
		log "Supernode stop requested."
	fi
}

stop_nginx() {
	if proc_running nginx; then
		log "Stopping nginx..."
		if command -v nginx >/dev/null 2>&1; then
			nginx -s quit >/dev/null 2>&1 || nginx -s stop >/dev/null 2>&1 || proc_kill nginx
		else
			proc_kill nginx
		fi
		log "nginx stop requested."
	else
		log "nginx is not running."
	fi
}

stop_lumera() {
	local pattern="${DAEMON} start --home ${DAEMON_HOME}"

	if pgrep -f "${pattern}" >/dev/null 2>&1; then
		log "Stopping ${DAEMON}..."
		pkill -f "${pattern}" || true
		log "${DAEMON} stop requested."
		return
	fi

	if proc_running "${DAEMON}"; then
		log "Stopping ${DAEMON}..."
		proc_kill "${DAEMON}"
		log "${DAEMON} stop requested."
	else
		log "${DAEMON} is not running."
	fi
}

stop_all() {
	stop_uploader
	stop_sn
	stop_nginx
	stop_lumera
}

usage() {
	echo "Usage: $0 [uploader|sn|lumera|nginx|all]" >&2
	exit 1
}

target="${1:-all}"
case "${target}" in
uploader | nm | network-maker | lumera-uploader | ul)
	stop_uploader
	;;
sn | supernode)
	stop_sn
	;;
nginx | ui)
	stop_nginx
	;;
lumera | lumerad | chain)
	stop_lumera
	;;
all | "")
	stop_all
	;;
*)
	usage
	;;
esac
