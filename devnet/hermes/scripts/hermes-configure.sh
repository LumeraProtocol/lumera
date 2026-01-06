#!/usr/bin/env bash
set -euo pipefail

: "${HERMES_CONFIG_PATH:=${HERMES_CONFIG:-/root/.hermes/config.toml}}"
: "${HERMES_TEMPLATE_PATH:=/root/scripts/hermes-config-template.toml}"

ENTRY_LOG_FILE="${ENTRY_LOG_FILE:-/root/logs/entrypoint.log}"
LOG_PREFIX="[hermes-configure]"

log_info() {
  local msg="$1"
  local line
  line=$(printf '[%s] %s %s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "${LOG_PREFIX}" "${msg}")
  printf '%s\n' "${line}"
  printf '%s\n' "${line}" >> "${ENTRY_LOG_FILE}"
}

fmt_cmd() {
  local out="" arg
  for arg in "$@"; do
    if [ -z "${out}" ]; then
      out=$(printf '%q' "${arg}")
    else
      out="${out} $(printf '%q' "${arg}")"
    fi
  done
  printf '%s' "${out}"
}

log_cmd_start() {
  log_info "CMD start: $(fmt_cmd "$@")"
}

log_cmd_result() {
  local rc="$1"
  shift
  if [ "${rc}" -eq 0 ]; then
    log_info "CMD success (rc=${rc}): $(fmt_cmd "$@")"
  else
    log_info "CMD failure (rc=${rc}): $(fmt_cmd "$@")"
  fi
}

log_cmd_output() {
  local label="$1"
  local payload="$2"
  local count=0
  if [ -z "${payload}" ]; then
    return 0
  fi
  while IFS= read -r line; do
    log_info "${label}: ${line}"
    count=$((count + 1))
    if [ "${count}" -ge 40 ]; then
      log_info "${label}: ... (truncated after 40 lines)"
      break
    fi
  done <<< "${payload}"
}

ran() {
  local cmd=("$@")
  log_cmd_start "${cmd[@]}"
  "${cmd[@]}"
  local rc=$?
  log_cmd_result "${rc}" "${cmd[@]}"
  return "${rc}"
}

ran_capture() {
  local cmd=("$@")
  log_cmd_start "${cmd[@]}"
  local output rc
  if output=$("${cmd[@]}" 2>&1); then
    rc=0
  else
    rc=$?
  fi
  log_cmd_output "CMD output" "${output}"
  log_cmd_result "${rc}" "${cmd[@]}"
  printf '%s' "${output}"
  return "${rc}"
}

: "${LUMERA_CHAIN_ID:=lumera-devnet-1}"
: "${LUMERA_RPC_ADDR:=http://supernova_validator_1:26657}"
: "${LUMERA_GRPC_ADDR:=http://supernova_validator_1:9090}"
: "${LUMERA_WS_ADDR:=ws://supernova_validator_1:26657/websocket}"
: "${LUMERA_ACCOUNT_PREFIX:=lumera}"
: "${LUMERA_BOND_DENOM:=ulume}"
: "${SIMD_CHAIN_ID:=hermes-simd-1}"
: "${SIMD_DENOM:=stake}"
: "${SIMD_RPC_PORT:=26657}"
: "${SIMD_GRPC_PORT:=9090}"
: "${HERMES_KEY_NAME:=relayer}"
: "${HERMES_MAX_GAS:=1000000}"

CONFIG_DIR="$(dirname "${HERMES_CONFIG_PATH}")"
ran mkdir -p "${CONFIG_DIR}"

if [ ! -f "${HERMES_CONFIG_PATH}" ]; then
  ran cp "${HERMES_TEMPLATE_PATH}" "${HERMES_CONFIG_PATH}"
fi

ensure_mode_enabled() {
  local section="$1"
  local value="$2"
  if ! ran python3 - "$HERMES_CONFIG_PATH" "$section" "$value" <<'PY'
import pathlib
import re
import sys

path = pathlib.Path(sys.argv[1])
section = sys.argv[2]
value = sys.argv[3]

lines = path.read_text().splitlines()
out = []
in_section = False
replaced = False

for line in lines:
    if re.match(r'^\s*\[', line):
        if in_section and not replaced:
            out.append(f'enabled = {value}')
            replaced = True
        in_section = line.strip() == f'[{section}]'
        out.append(line)
        continue
    if in_section and re.match(r'^\s*enabled\s*=', line):
        out.append(f'enabled = {value}')
        replaced = True
        continue
    out.append(line)

if in_section and not replaced:
    out.append(f'enabled = {value}')

path.write_text('\n'.join(out) + '\n')
PY
  then
    log_info "Failed to enforce ${section}.enabled=${value}"
    return 1
  fi
  log_info "Ensured ${section}.enabled=${value}"
}

ensure_mode_enabled "mode.channels" "true"
ensure_mode_enabled "mode.connections" "true"

append_chain() {
  local chain_id="$1"
  local block="$2"
  log_info "Updating Hermes chain entry for ${chain_id}"
  if ! ran python3 - "$HERMES_CONFIG_PATH" "$chain_id" <<'PY'
import pathlib, re, sys
config_path = pathlib.Path(sys.argv[1])
chain_id = sys.argv[2]
text = config_path.read_text()
parts = text.split('[[chains]]')
if len(parts) == 1:
    sys.exit(0)
head = parts[0]
blocks = ['[[chains]]' + part for part in parts[1:]]
pattern = re.compile(r"^\s*id\s*=\s*['\"]{}['\"]\s*$".format(re.escape(chain_id)), re.MULTILINE)
kept = [blk for blk in blocks if not pattern.search(blk)]
config_path.write_text(head + ''.join(kept))
PY
  then
    log_info "Failed to normalise configuration for chain ${chain_id}"
    return 1
  fi
  if printf "\n%s\n" "${block}" >> "${HERMES_CONFIG_PATH}"; then
    log_info "Appended configuration block for ${chain_id}"
  else
    log_info "Failed to append configuration block for ${chain_id}"
    return 1
  fi
}

lumera_block=$(cat <<EOF
[[chains]]
id = '${LUMERA_CHAIN_ID}'
type = 'CosmosSdk'
rpc_addr = '${LUMERA_RPC_ADDR}'
grpc_addr = '${LUMERA_GRPC_ADDR}'
event_source = { mode = 'push', url = '${LUMERA_WS_ADDR}' }
rpc_timeout = '10s'
account_prefix = '${LUMERA_ACCOUNT_PREFIX}'
key_name = '${HERMES_KEY_NAME}'
store_prefix = 'ibc'
memo_prefix = ''
gas_price = { price = 0.025, denom = '${LUMERA_BOND_DENOM}' }
max_gas = ${HERMES_MAX_GAS}
clock_drift = '5s'
trusting_period = '14days'
trust_threshold = '1/3'
EOF
)

simd_block=$(cat <<EOF
[[chains]]
id = '${SIMD_CHAIN_ID}'
type = 'CosmosSdk'
rpc_addr = 'http://127.0.0.1:${SIMD_RPC_PORT}'
grpc_addr = 'http://127.0.0.1:${SIMD_GRPC_PORT}'
event_source = { mode = 'push', url = 'ws://127.0.0.1:${SIMD_RPC_PORT}/websocket' }
rpc_timeout = '10s'
account_prefix = 'cosmos'
key_name = '${HERMES_KEY_NAME}'
store_prefix = 'ibc'
memo_prefix = ''
gas_price = { price = 0.025, denom = '${SIMD_DENOM}' }
max_gas = ${HERMES_MAX_GAS}
clock_drift = '5s'
trusting_period = '14days'
trust_threshold = '1/3'
EOF
)

append_chain "${LUMERA_CHAIN_ID}" "${lumera_block}"
append_chain "${SIMD_CHAIN_ID}" "${simd_block}"
