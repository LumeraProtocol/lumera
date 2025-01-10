#!/usr/bin/env bash

set -euo pipefail

# Configuration
CHAIN_ID="pastel-devnet-1"
KEYRING_BACKEND="test"
CONTAINER_PREFIX="pastel-validator"

# IP address mapping
declare -A VALIDATOR_IPS=(
  [1]="192.168.1.1"
  [2]="192.168.1.2"
  [3]="192.168.1.3"
  [4]="192.168.1.4"
  [5]="192.168.1.5"
)

# Logging Setup
LOG_FILE="./setup_supernodes.log"
> "$LOG_FILE"

# General log function
log() {
  local timestamp
  timestamp="$(date '+%Y-%m-%d %H:%M:%S')"
  echo -e "[$timestamp] [INFO] $*" | tee -a "$LOG_FILE"
}

# Function to run commands and log output
run_cmd() {
  log "Running: $*"
  eval "$*" 2>&1 | tee -a "$LOG_FILE"
}

# Helper Functions

# Store validator addresses in global arrays
declare -A VAL_ACCOUNT
declare -A VAL_OPERATOR

query_addresses() {
  local i="$1"
  local container="${CONTAINER_PREFIX}${i}"

  VAL_ACCOUNT["$i"]="$(
    docker exec "$container" pasteld keys show validator${i}_key \
      --keyring-backend "$KEYRING_BACKEND" -a
  )"

  VAL_OPERATOR["$i"]="$(
    docker exec "$container" pasteld keys show validator${i}_key \
      --keyring-backend "$KEYRING_BACKEND" --bech val -a
  )"
}

register_supernode() {
  local i="$1"
  local container="${CONTAINER_PREFIX}${i}"
  local valop="${VAL_OPERATOR[$i]}"
  local valacct="${VAL_ACCOUNT[$i]}"
  local ip="${VALIDATOR_IPS[$i]}"

  log "Registering supernode on ${container} (ValOp: ${valop}, Account: ${valacct}, IP: ${ip})"
  run_cmd "docker exec ${container} pasteld tx supernode register-supernode \
    ${valop} \
    ${ip} \
    1.0 \
    ${valacct} \
    --from validator${i}_key \
    --keyring-backend ${KEYRING_BACKEND} \
    --chain-id ${CHAIN_ID} \
    --yes"
}

# Main Flow

log "===== Starting Supernode Registration Script ====="

# Query addresses for all validators
for i in {1..5}; do
  log "Querying addresses for validator ${i} ..."
  query_addresses "$i"
  log "  Validator ${i} Account:  ${VAL_ACCOUNT[$i]}"
  log "  Validator ${i} Operator: ${VAL_OPERATOR[$i]}"
done

# Register all validators as supernodes
for i in {1..5}; do
  register_supernode "$i"
  log "Sleeping 3 seconds after registering validator ${i}..."
  sleep 3
done

log "Querying the list of registered supernodes (via pastel-validator1) ..."
run_cmd "docker exec pastel-validator1 pasteld query supernode list-super-nodes"

log "===== Supernode Registration Complete ====="