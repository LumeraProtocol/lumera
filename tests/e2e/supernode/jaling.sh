#!/usr/bin/env bash

set -euo pipefail

# Step 1: Basic Configuration
echo "Step 1: Setting up configuration..."
CHAIN_ID="lumera-devnet-1"
KEYRING_BACKEND="test"
VALIDATOR_NUM=5
VALIDATOR_CONTAINER="lumera-validator${VALIDATOR_NUM}"
SLEEP_FOR_JAIL=60
SLEEP_FOR_UNJAIL=90

# Create a timestamped log file
LOG_FILE="jail_test_$(date +%Y%m%d_%H%M%S).log"
touch "$LOG_FILE"

# Step 2: Setup Logging Function
log() {
	local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
	echo "$msg" | tee -a "$LOG_FILE"
}

log_cmd() {
	local cmd_output
	log "Executing: $1"
	cmd_output=$(eval "$1" 2>&1)
	echo "$cmd_output" | tee -a "$LOG_FILE"
	echo "----------------------------------------" | tee -a "$LOG_FILE"
}

log "Starting jail test for validator ${VALIDATOR_NUM}"

# Step 3: Get Validator Addresses
log "Step 3: Getting validator addresses..."

VALIDATOR_ACCOUNT=$(docker exec "$VALIDATOR_CONTAINER" lumerad keys show validator${VALIDATOR_NUM}_key \
	--keyring-backend "$KEYRING_BACKEND" -a)
log "Validator Account: $VALIDATOR_ACCOUNT"

VALIDATOR_OPERATOR=$(docker exec "$VALIDATOR_CONTAINER" lumerad keys show validator${VALIDATOR_NUM}_key \
	--keyring-backend "$KEYRING_BACKEND" --bech val -a)
log "Validator Operator: $VALIDATOR_OPERATOR"

# Step 4: Check Initial Status
log "Step 4: Checking initial validator status..."
log_cmd "docker exec lumera-validator1 lumerad query staking validator $VALIDATOR_OPERATOR"

log "Checking initial supernode status..."
log_cmd "docker exec lumera-validator1 lumerad query supernode get-supernode $VALIDATOR_OPERATOR"

# Step 5: Stop Validator to Force Jailing
log "Step 5: Stopping validator container to force jailing..."
log_cmd "docker stop $VALIDATOR_CONTAINER"

log "Sleeping for ${SLEEP_FOR_JAIL} seconds to allow jailing..."
sleep "${SLEEP_FOR_JAIL}"

# Step 6: Check Status After Jailing
log "Step 6: Checking validator status after jailing..."
log_cmd "docker exec lumera-validator1 lumerad query staking validator $VALIDATOR_OPERATOR"

log "Checking supernode status after jailing..."
log_cmd "docker exec lumera-validator1 lumerad query supernode get-supernode $VALIDATOR_OPERATOR"

# Step 7: Restart Validator
log "Step 7: Restarting validator container..."
log_cmd "docker start $VALIDATOR_CONTAINER"

log "Sleeping for ${SLEEP_FOR_UNJAIL} seconds before unjailing..."
sleep "${SLEEP_FOR_UNJAIL}"

# Step 8: Unjail Validator
log "Step 8: Unjailing validator..."
log_cmd "docker exec $VALIDATOR_CONTAINER lumerad tx slashing unjail \
    --from validator${VALIDATOR_NUM}_key \
    --keyring-backend $KEYRING_BACKEND \
    --chain-id $CHAIN_ID \
    --gas auto \
    --gas-adjustment 1.5 \
    --yes"

# Wait for transaction to be processed
log "Waiting 10 seconds for transaction to be processed..."
sleep 10

# Step 9: Final Status Check
log "Step 9: Performing final status check..."
log "Checking final validator status..."
log_cmd "docker exec lumera-validator1 lumerad query staking validator $VALIDATOR_OPERATOR"

log "Checking final supernode status..."
log_cmd "docker exec lumera-validator1 lumerad query supernode get-supernode $VALIDATOR_OPERATOR"

# Step 10: Complete
log "Test completed successfully. All output has been saved to $LOG_FILE"
