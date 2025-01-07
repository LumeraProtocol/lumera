#!/usr/bin/env bash
#
# Script:  test_5_validator_jail_dynamic.sh
# Purpose: Demonstrate automatically querying addresses, registering 5 supernodes,
#          stopping one validator to force jailing, and checking that the jailed
#          validator’s supernode is disabled.

set -euo pipefail

#################################################################
# Configuration
#################################################################
CHAIN_ID="pastel-devnet-1"
KEYRING_BACKEND="test"

# We assume you have 5 containers named "pastel-validator1" through "pastel-validator5".
# Adjust these if you use different names or a docker-compose setup.
CONTAINER_PREFIX="pastel-validator"

# For demonstration, we assign each validator a local IP address.
# Change these if your environment differs.
declare -A VALIDATOR_IPS=(
  [1]="192.168.1.1"
  [2]="192.168.1.2"
  [3]="192.168.1.3"
  [4]="192.168.1.4"
  [5]="192.168.1.5"
)

# How long to wait for the chain to produce blocks once a validator is stopped (to ensure jailing).
# Adjust this based on block times and jailing parameters.
SLEEP_FOR_JAIL=60

# Which validator do we stop to test jailing? For example, #2
JAIL_VALIDATOR_NUM=2

# Extra sleep time after restarting container before unjailing
SLEEP_FOR_UNJAIL=90

#################################################################
# Logging Setup
#################################################################
LOG_FILE="./test_5_validator_jail_dynamic.log"
> "$LOG_FILE"

# General log function (info-level).
log() {
  local timestamp
  timestamp="$(date '+%Y-%m-%d %H:%M:%S')"
  echo -e "[$timestamp] [INFO] $*" | tee -a "$LOG_FILE"
}

# Error log function.
error() {
  local timestamp
  timestamp="$(date '+%Y-%m-%d %H:%M:%S')"
  echo -e "[$timestamp] [ERROR] $*" | tee -a "$LOG_FILE" >&2
}

# Function to run commands, show them in logs, and capture output.
run_cmd() {
  # Print the command we are about to run:
  log "Running: $*"
  # Execute the command. 
  # - We use eval so that we can see the entire command as typed.
  # - Pipe both stdout and stderr to tee, which appends to LOG_FILE.
  eval "$*" 2>&1 | tee -a "$LOG_FILE"
}

#################################################################
# Helper Functions
#################################################################

# Query addresses for a given validator # (1..5).
# We store them in global arrays: VAL_ACCOUNT[i], VAL_OPERATOR[i].
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

# Register a validator’s supernode
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

#################################################################
# Main Flow
#################################################################

log "===== Starting test_5_validator_jail_dynamic.sh ====="

# 1) (Optional) Ensure containers are up. We'll assume they're already running.

# 2) Dynamically query addresses for each validator
for i in 1 2 3 4 5; do
  log "Querying addresses for validator ${i} ..."
  query_addresses "$i"
  log "  Validator ${i} Account:  ${VAL_ACCOUNT[$i]}"
  log "  Validator ${i} Operator: ${VAL_OPERATOR[$i]}"
done

# 3) Register all validators as supernodes
for i in 1 2 3 4 5; do
  register_supernode "$i"
  log "Sleeping 3 seconds after registering validator ${i}..."
  sleep 3
done

sleep 3

log "Querying the list of registered supernodes (via pastel-validator1) ..."
run_cmd "docker exec pastel-validator1 pasteld query supernode list-super-nodes"

# 4) Stop one validator to force it to miss blocks and become jailed
local_jail_container="${CONTAINER_PREFIX}${JAIL_VALIDATOR_NUM}"
local_jail_valop="${VAL_OPERATOR[$JAIL_VALIDATOR_NUM]}"

log "Stopping ${local_jail_container} to force jailing ..."
run_cmd "docker stop ${local_jail_container}"

log "Sleeping for ${SLEEP_FOR_JAIL} seconds to allow jailing ..."
sleep "${SLEEP_FOR_JAIL}"

# 5) Check the validator’s staking state
log "Checking if validator ${JAIL_VALIDATOR_NUM} is jailed (via pastel-validator1) ..."
run_cmd "docker exec pastel-validator1 pasteld query staking validator ${local_jail_valop}"

# 6) Check supernode status in the list to see if it’s disabled
log "Checking if validator ${JAIL_VALIDATOR_NUM}'s supernode is disabled ..."
run_cmd "docker exec pastel-validator1 pasteld query supernode get-super-node ${local_jail_valop}"

# 7) Restart the jailed container; it remains jailed until unjailed
log "Restarting container ${local_jail_container} (validator is still jailed) ..."
run_cmd "docker start ${local_jail_container}"

log "Sleeping for ${SLEEP_FOR_UNJAIL} seconds before unjailing ..."
sleep "${SLEEP_FOR_UNJAIL}"

# 8) Unjail the validator
log "Unjailing validator ${JAIL_VALIDATOR_NUM} ..."
run_cmd "docker exec -it pastel-validator2 pasteld tx slashing unjail \
  --from validator2_key \
  --chain-id ${CHAIN_ID} \
  --keyring-backend ${KEYRING_BACKEND} \
  --yes"

sleep 5

# 9) Check supernode status again to see if it’s active
log "Verifying that validator ${JAIL_VALIDATOR_NUM}'s supernode is active again ..."
run_cmd "docker exec pastel-validator1 pasteld query supernode get-super-node ${local_jail_valop}"

log "===== Completed test_5_validator_jail_dynamic.sh ====="
