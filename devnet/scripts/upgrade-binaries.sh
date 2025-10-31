#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <binaries-dir>" >&2
  exit 1
fi

BINARIES_DIR="$1"
if [[ ! -d "${BINARIES_DIR}" ]]; then
  echo "Binaries directory not found: ${BINARIES_DIR}" >&2
  exit 1
fi
BINARIES_DIR="$(cd "${BINARIES_DIR}" && pwd)"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEVNET_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DEVNET_ROOT}/docker-compose.yml"

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  echo "docker-compose.yml not found at ${COMPOSE_FILE}" >&2
  exit 1
fi

DEVNET_RUNTIME_DIR="${DEVNET_DIR:-/tmp/lumera-devnet-1}"
RELEASE_DIR="${DEVNET_RUNTIME_DIR}/shared/release"

echo "Stopping devnet containers..."
docker compose -f "${COMPOSE_FILE}" stop

echo "Copying binaries from ${BINARIES_DIR} to ${RELEASE_DIR}..."
mkdir -p "${RELEASE_DIR}"
shopt -s nullglob
copied=0
for file in "${BINARIES_DIR}"/*; do
  if [[ -f "${file}" ]]; then
    cp -Sf "${file}" "${RELEASE_DIR}/"
    copied=1
  fi
done
shopt -u nullglob

if [[ "${copied}" -eq 0 ]]; then
  echo "No files were copied from ${BINARIES_DIR}" >&2
  exit 1
fi

if [[ -f "${RELEASE_DIR}/lumerad" ]]; then
  chmod +x "${RELEASE_DIR}/lumerad"
fi

echo "Restarting devnet containers..."
START_MODE=run docker compose -f "${COMPOSE_FILE}" up -d

echo "Binaries upgrade complete using ${BINARIES_DIR}."
