#!/usr/bin/env bash
# stop.sh — stop devnet services inside a validator container
# Usage:
#   ./stop.sh                    # stop all known services
#   ./stop.sh nm|sn|lumera|nginx
set -euo pipefail

DAEMON="${DAEMON:-lumerad}"
DAEMON_HOME="${DAEMON_HOME:-/root/.lumera}"
SN_BASEDIR="${SN_BASEDIR:-/root/.supernode}"

log() {
  echo "[STOP] $*"
}

stop_nm() {
  local name="network-maker"

  if pgrep -x "${name}" >/dev/null 2>&1; then
    log "Stopping network-maker..."
    pkill -x "${name}" || true
    log "network-maker stop requested."
  else
    log "network-maker is not running."
  fi
}

stop_sn() {
  local stopped=0
  local names=("supernode-linux-amd64" "supernode")

  for name in "${names[@]}"; do
    if pgrep -x "${name}" >/dev/null 2>&1; then
      stopped=1
      log "Stopping supernode (${name})..."
      if command -v "${name}" >/dev/null 2>&1; then
        "${name}" stop -d "${SN_BASEDIR}" >/dev/null 2>&1 || pkill -x "${name}" || true
      else
        pkill -x "${name}" || true
      fi
    fi
  done

  if (( stopped == 0 )); then
    log "Supernode is not running."
  else
    log "Supernode stop requested."
  fi
}

stop_nginx() {
  if pgrep -x nginx >/dev/null 2>&1; then
    log "Stopping nginx..."
    if command -v nginx >/dev/null 2>&1; then
      nginx -s quit >/dev/null 2>&1 || nginx -s stop >/dev/null 2>&1 || pkill -x nginx || true
    else
      pkill -x nginx || true
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

  if pgrep -x "${DAEMON}" >/dev/null 2>&1; then
    log "Stopping ${DAEMON}..."
    pkill -x "${DAEMON}" || true
    log "${DAEMON} stop requested."
  else
    log "${DAEMON} is not running."
  fi
}

stop_all() {
  stop_nm
  stop_sn
  stop_nginx
  stop_lumera
}

usage() {
  echo "Usage: $0 [nm|sn|lumera|nginx|all]" >&2
  exit 1
}

target="${1:-all}"
case "${target}" in
  nm|network-maker)
    stop_nm
    ;;
  sn|supernode)
    stop_sn
    ;;
  nginx|ui)
    stop_nginx
    ;;
  lumera|lumerad|chain)
    stop_lumera
    ;;
  all|"")
    stop_all
    ;;
  *)
    usage
    ;;
esac
