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
SLEEP_FOR_JAIL=120

# Which validator do we stop to test jailing? For example, #2
JAIL_VALIDATOR_NUM=2

#################################################################
# Helper Functions
#################################################################

# Log to console and optionally tee to a file if you like.
log() {
  echo -e "[LOG] $*"
}

# Query addresses for a given validator # (1..5).
# We store them in global arrays: VAL_ACCOUNT[i], VAL_OPERATOR[i].
declare -A VAL_ACCOUNT
declare -A VAL_OPERATOR

query_addresses() {
  local i="$1"
  local container="${CONTAINER_PREFIX}${i}"

  # Get the “account address” in bech32 (e.g., pastel1xxxx)
  VAL_ACCOUNT["$i"]="$(
    docker exec "$container" pasteld keys show validator${i}_key \
      --keyring-backend "$KEYRING_BACKEND" -a
  )"

  # Get the “operator address” in bech32 (e.g., pastelvaloper1xxxx)
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

  log "Registering supernode on $container [valoper=$valop, account=$valacct, IP=$ip]"
  docker exec "$container" pasteld tx supernode register-supernode \
    "$valop" \
    "$ip" \
    "1.0" \
    "$valacct" \
    --from "validator${i}_key" \
    --keyring-backend "$KEYRING_BACKEND" \
    --chain-id "$CHAIN_ID" \
    --yes
}

#################################################################
# Main Flow
#################################################################

# 1) (Optional) Ensure containers are up (e.g., `docker-compose up -d`).
#    We'll assume they're already running.

# 2) Dynamically query addresses for each validator
for i in 1 2 3 4 5; do
  log "Querying addresses for validator$i..."
  query_addresses "$i"
  log "  Account  : ${VAL_ACCOUNT[$i]}"
  log "  Operator : ${VAL_OPERATOR[$i]}"
done

# 3) Register all validators as supernodes
for i in 1 2 3 4 5; do
  register_supernode "$i"
  # Sleep briefly so the transactions are included in separate blocks
  sleep 1
done

# 4) Check that they are all registered as supernodes (from validator1’s node, for example)
log "Querying supernode list after registration (from pastel-validator1):"
docker exec pastel-validator1 pasteld query supernode list-super-nodes

# 5) Stop one validator to force it to miss blocks and become jailed
local_jail_container="${CONTAINER_PREFIX}${JAIL_VALIDATOR_NUM}"
local_jail_valop="${VAL_OPERATOR[$JAIL_VALIDATOR_NUM]}"

log "Stopping $local_jail_container so it misses blocks..."
docker stop "$local_jail_container"

log "Sleeping for $SLEEP_FOR_JAIL seconds to allow jailing..."
sleep "$SLEEP_FOR_JAIL"

# 6) Query the validator’s staking state from a running node (validator1)
log "Checking if validator${JAIL_VALIDATOR_NUM} is jailed..."
docker exec pastel-validator1 pasteld query staking validator "$local_jail_valop"

# 7) Check supernode status in the list to see if it’s disabled
log "Checking supernode list to see if validator${JAIL_VALIDATOR_NUM} supernode is disabled..."
docker exec pastel-validator1 pasteld query supernode list-super-nodes

# 8) (Optional) Restart the container; it remains jailed unless unjailed
# log "Restarting $local_jail_container (still jailed until unjailed)..."
# docker start "$local_jail_container"

# 9) (Optional) Unjail if desired (example not automatically done here).
#    e.g.:
  #  docker exec -it pastel-validator2 pasteld tx slashing unjail \
  #     --from validator2_key \
  #     --chain-id "$CHAIN_ID" \
  #     --keyring-backend "$KEYRING_BACKEND" \
  #     --yes

log "All done!"

