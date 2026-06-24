#!/usr/bin/env bash
#
# apply-evm-ports.sh — make newly-added EVM JSON-RPC host ports take effect on an
# already-running devnet WITHOUT wiping chain data.
#
# Used after an in-place upgrade that crosses the pre-EVM -> EVM boundary: the
# upgraded binary serves EVM JSON-RPC, but the running containers were created
# from a pre-EVM docker-compose.yml (no EVM host ports) and their app.toml still
# binds the metrics/geth servers to 127.0.0.1.
#
# Preconditions:
#   - A freshly generated docker-compose.yml (with the EVM host-port block) has
#     already been copied next to this script's parent dir.
#   - The validators are running an EVM-enabled lumerad.
#
# Steps:
#   1. Reapply EVM metrics/geth bind addresses (0.0.0.0) + enable telemetry in each
#      validator's app.toml. validator-setup.sh writes these only on first setup,
#      so already-set-up nodes need them reapplied for the metrics/geth ports to
#      actually serve.
#   2. Recreate containers (docker compose up -d --no-build) so the new host port
#      mappings take effect. Validator data lives in host bind mounts, so it is
#      preserved across the recreate; start.sh re-adds --metrics on EVM versions.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEVNET_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DEVNET_ROOT}/docker-compose.yml"

if [[ ! -f "${COMPOSE_FILE}" ]]; then
	echo "docker-compose.yml not found at ${COMPOSE_FILE}" >&2
	exit 1
fi

DEVNET_RUNTIME_DIR="${DEVNET_DIR:-/tmp/lumera-devnet-1}"
CFG_CHAIN="${DEVNET_RUNTIME_DIR}/shared/config/config.json"

metrics_addr="0.0.0.0:6065"
geth_addr="0.0.0.0:8100"
if [[ -f "${CFG_CHAIN}" ]]; then
	metrics_addr="$(jq -r '.["json-rpc"].metrics_address // "0.0.0.0:6065"' "${CFG_CHAIN}")"
	geth_addr="$(jq -r '.["json-rpc"].geth_metrics_address // "0.0.0.0:8100"' "${CFG_CHAIN}")"
fi

services="$(docker compose -f "${COMPOSE_FILE}" config --services | grep '^supernova_validator_' || true)"
if [[ -z "${services}" ]]; then
	echo "No validator services found in ${COMPOSE_FILE}" >&2
	exit 1
fi

for svc in ${services}; do
	cid="$(docker compose -f "${COMPOSE_FILE}" ps -q "${svc}" 2>/dev/null || true)"
	if [[ -z "${cid}" ]]; then
		echo "[apply-evm-ports] ${svc}: not running, skipping app.toml reapply"
		continue
	fi
	echo "[apply-evm-ports] ${svc}: setting metrics-address=${metrics_addr}, geth-metrics-address=${geth_addr}, telemetry.enabled=true"
	docker exec "${cid}" crudini --set /root/.lumera/config/app.toml json-rpc metrics-address "\"${metrics_addr}\"" || true
	docker exec "${cid}" crudini --set /root/.lumera/config/app.toml evm geth-metrics-address "\"${geth_addr}\"" || true
	docker exec "${cid}" crudini --set /root/.lumera/config/app.toml telemetry enabled "true" || true
done

echo "[apply-evm-ports] Recreating containers to apply new host port mappings..."
START_MODE=run docker compose -f "${COMPOSE_FILE}" up -d --no-build --remove-orphans
docker compose -f "${COMPOSE_FILE}" ps
