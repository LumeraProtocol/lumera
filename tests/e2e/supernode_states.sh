#!/usr/bin/env bash
#
# Script: test_supernode_state_transitions.sh
# Purpose: Test supernode state transitions through register/deregister/stop/start/jail/unjail
#          and stake modifications around minimum threshold

set -euo pipefail

#################################################################
# Configuration
#################################################################
CHAIN_ID="pastel-devnet-1"
KEYRING_BACKEND="test"
CONTAINER_PREFIX="pastel-validator"

# Test validator number (we'll use validator2 for our tests)
TEST_VALIDATOR_NUM=5

# Sleep durations
SLEEP_FOR_JAIL=60
SLEEP_FOR_UNJAIL=90
SLEEP_BETWEEN_OPS=5

#################################################################
# Logging Setup
#################################################################
LOG_FILE="./test_supernode_state_transitions.log"
> "$LOG_FILE"

log() {
  local timestamp
  timestamp="$(date '+%Y-%m-%d %H:%M:%S')"
  echo -e "[$timestamp] [INFO] $*" | tee -a "$LOG_FILE"
}

error() {
  local timestamp
  timestamp="$(date '+%Y-%m-%d %H:%M:%S')"
  echo -e "[$timestamp] [ERROR] $*" | tee -a "$LOG_FILE" >&2
}

run_cmd() {
  log "Running: $*"
  eval "$*" 2>&1 | tee -a "$LOG_FILE"
}

#################################################################
# Helper Functions
#################################################################

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

check_supernode_status() {
  local i="$1"
  local valop="${VAL_OPERATOR[$i]}"
  
  log "Checking supernode status for validator ${i} (${valop})"
  run_cmd "docker exec pastel-validator1 pasteld query supernode get-super-node ${valop}"
}

#################################################################
# Main Flow
#################################################################

log "===== Starting Supernode State Transitions Test ====="

# 1) Query addresses for test validator
log "Querying addresses for validator ${TEST_VALIDATOR_NUM}"
query_addresses "$TEST_VALIDATOR_NUM"
log "  Validator ${TEST_VALIDATOR_NUM} Account:  ${VAL_ACCOUNT[$TEST_VALIDATOR_NUM]}"
log "  Validator ${TEST_VALIDATOR_NUM} Operator: ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]}"

# 2) Register Supernode
log "Registering supernode for validator ${TEST_VALIDATOR_NUM}"
run_cmd "docker exec ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM} pasteld tx supernode register-supernode \
  ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]} \
  192.168.1.${TEST_VALIDATOR_NUM} \
  1.0 \
  ${VAL_ACCOUNT[$TEST_VALIDATOR_NUM]} \
  --from validator${TEST_VALIDATOR_NUM}_key \
  --keyring-backend ${KEYRING_BACKEND} \
  --chain-id ${CHAIN_ID} \
  --yes"

sleep "$SLEEP_BETWEEN_OPS"
check_supernode_status "$TEST_VALIDATOR_NUM"

# 3) Stop Supernode
log "Stopping supernode for validator ${TEST_VALIDATOR_NUM}"
run_cmd "docker exec ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM} pasteld tx supernode stop-supernode \
  ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]} \
  'maintenance' \
  --from validator${TEST_VALIDATOR_NUM}_key \
  --keyring-backend ${KEYRING_BACKEND} \
  --chain-id ${CHAIN_ID} \
  --yes"

sleep "$SLEEP_BETWEEN_OPS"
check_supernode_status "$TEST_VALIDATOR_NUM"

# 4) Start Supernode
log "Starting supernode for validator ${TEST_VALIDATOR_NUM}"
run_cmd "docker exec ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM} pasteld tx supernode start-supernode \
  ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]} \
  --from validator${TEST_VALIDATOR_NUM}_key \
  --keyring-backend ${KEYRING_BACKEND} \
  --chain-id ${CHAIN_ID} \
  --yes"

sleep "$SLEEP_BETWEEN_OPS"
check_supernode_status "$TEST_VALIDATOR_NUM"

# 5) Reduce stake slightly below minimum required
log "Reducing stake below minimum required (834637515638) for validator ${TEST_VALIDATOR_NUM}"
run_cmd "docker exec ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM} pasteld tx staking unbond \
  ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]} \
  10upsl \
  --from validator${TEST_VALIDATOR_NUM}_key \
  --keyring-backend ${KEYRING_BACKEND} \
  --chain-id ${CHAIN_ID} \
  --yes"

sleep "$SLEEP_BETWEEN_OPS"

log "Checking validator status after reducing stake"
run_cmd "docker exec pastel-validator1 pasteld query staking validator ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]}"
check_supernode_status "$TEST_VALIDATOR_NUM"

# 6) Increase stake slightly above minimum required
log "Increasing stake above minimum required (834637515658) for validator ${TEST_VALIDATOR_NUM}"
run_cmd "docker exec ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM} pasteld tx staking delegate \
  ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]} \
  20upsl \
  --from validator${TEST_VALIDATOR_NUM}_key \
  --keyring-backend ${KEYRING_BACKEND} \
  --chain-id ${CHAIN_ID} \
  --yes"

sleep "$SLEEP_BETWEEN_OPS"

log "Checking validator status after increasing stake"
run_cmd "docker exec pastel-validator1 pasteld query staking validator ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]}"
check_supernode_status "$TEST_VALIDATOR_NUM"

# 7) Force jail by stopping validator
log "Stopping validator ${TEST_VALIDATOR_NUM} container to force jailing"
run_cmd "docker stop ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM}"

log "Sleeping for ${SLEEP_FOR_JAIL} seconds to allow jailing..."
sleep "$SLEEP_FOR_JAIL"

# 8) Verify jailed status and supernode state
log "Checking validator jail status and supernode state"
run_cmd "docker exec pastel-validator1 pasteld query staking validator ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]}"
check_supernode_status "$TEST_VALIDATOR_NUM"

# 9) Restart validator and unjail
log "Restarting validator ${TEST_VALIDATOR_NUM} container"
run_cmd "docker start ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM}"

log "Sleeping for ${SLEEP_FOR_UNJAIL} seconds before unjailing..."
sleep "$SLEEP_FOR_UNJAIL"

log "Unjailing validator ${TEST_VALIDATOR_NUM}"
run_cmd "docker exec ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM} pasteld tx slashing unjail \
  --from validator${TEST_VALIDATOR_NUM}_key \
  --keyring-backend ${KEYRING_BACKEND} \
  --chain-id ${CHAIN_ID} \
  --yes"

sleep "$SLEEP_BETWEEN_OPS"
check_supernode_status "$TEST_VALIDATOR_NUM"

# 10) Deregister Supernode
log "Deregistering supernode for validator ${TEST_VALIDATOR_NUM}"
run_cmd "docker exec ${CONTAINER_PREFIX}${TEST_VALIDATOR_NUM} pasteld tx supernode deregister-supernode \
  ${VAL_OPERATOR[$TEST_VALIDATOR_NUM]} \
  --from validator${TEST_VALIDATOR_NUM}_key \
  --keyring-backend ${KEYRING_BACKEND} \
  --chain-id ${CHAIN_ID} \
  --yes"

sleep "$SLEEP_BETWEEN_OPS"
check_supernode_status "$TEST_VALIDATOR_NUM"

log "===== Completed Supernode State Transitions Test ====="