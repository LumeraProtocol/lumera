#!/usr/bin/env bash
###############################################################################
# Everlight Phase 1 -- Devnet Integration Tests
#
# Automates the executable subset of S10-S15-eval-scenarios.md scenarios 1, 6,
# 7, 8 and performs opportunistic live checks for scenario 2/8 prerequisites.
# Scenarios 2-5, 9, and 10 still require richer registered-supernode / upgrade
# setup than this smoke test can assume.
#
# NOTE: Everlight logic is embedded in x/supernode. All CLI queries use
#       `lumerad query supernode ...` and `lumerad tx supernode ...`.
#
# Usage:
#   COMPOSE_FILE=devnet/docker-compose.yml SERVICE=supernova_validator_1 bash devnet/tests/everlight/everlight_test.sh
#
# Environment variables:
#   COMPOSE_FILE  -- path to docker-compose.yml (default: devnet/docker-compose.yml)
#   SERVICE       -- validator service name     (default: supernova_validator_1)
###############################################################################
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
COMPOSE_FILE="${COMPOSE_FILE:-devnet/docker-compose.yml}"
SERVICE="${SERVICE:-supernova_validator_1}"
CHAIN_ID="lumera-devnet-1"
KEYRING="test"
DENOM="ulume"
FEES="5000${DENOM}"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
RESULTS=()

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Run lumerad inside the validator container.
lumerad_exec() {
    docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE" lumerad "$@"
}

lumerad_exec_service() {
    local service="$1"
    shift
    docker compose -f "$COMPOSE_FILE" exec -T "$service" lumerad "$@"
}

# Run lumerad query and return JSON.
lumerad_query() {
    lumerad_exec query "$@" --output json 2>/dev/null
}

supernode_metrics_rest() {
    local validator="$1"
    docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE" \
        curl -s -X GET "http://localhost:1317/LumeraProtocol/lumera/supernode/v1/metrics/${validator}" \
        -H "accept: application/json" 2>/dev/null
}

# Run lumerad tx and return JSON (broadcast sync).
lumerad_tx() {
    lumerad_exec tx "$@" \
        --chain-id "$CHAIN_ID" \
        --keyring-backend "$KEYRING" \
        --fees "$FEES" \
        --broadcast-mode sync \
        --output json \
        --yes 2>/dev/null
}

lumerad_tx_service() {
    local service="$1"
    shift
    lumerad_exec_service "$service" tx "$@" \
        --chain-id "$CHAIN_ID" \
        --keyring-backend "$KEYRING" \
        --fees "$FEES" \
        --broadcast-mode sync \
        --output json \
        --yes 2>/dev/null
}

tx_code_from_json() {
    local json="$1"
    echo "$json" | jq -r '.code // "0"' 2>/dev/null || echo "0"
}

is_sequence_mismatch() {
    local json="$1"
    local code raw_log
    code="$(tx_code_from_json "$json")"
    raw_log="$(echo "$json" | jq -r '.raw_log // empty' 2>/dev/null || echo "")"
    [[ "$code" == "32" ]] && [[ "$raw_log" == *"account sequence mismatch"* ]]
}

run_tx_with_retry() {
    local service="$1"
    shift
    local attempt result

    for attempt in 1 2 3; do
        result="$(lumerad_tx_service "$service" "$@")" || true
        if ! is_sequence_mismatch "$result"; then
            echo "$result"
            return 0
        fi
        echo "    WARN: sequence mismatch on $service tx attempt $attempt, retrying..." >&2
        sleep 2
    done

    echo "$result"
    return 0
}

service_key_name() {
    echo "${SERVICE}_key"
}

get_first_validator_address() {
    local list_json
    list_json="$(lumerad_query supernode list-supernodes)" || return 1
    echo "$list_json" | jq -r '.supernodes[]?.validator_address // empty' | head -n 1
}

service_account_address() {
    local key_name
    key_name="$(service_key_name)"
    lumerad_exec keys show "$key_name" -a --keyring-backend "$KEYRING" 2>/dev/null | tr -d '\r\n'
}

service_supernode_key_name() {
    echo "${SERVICE/supernova_validator/supernova_supernode}_key"
}

service_supernode_account_address() {
    local key_name
    key_name="$(service_supernode_key_name)"
    lumerad_exec keys show "$key_name" -a --keyring-backend "$KEYRING" 2>/dev/null | tr -d '\r\n'
}

get_service_supernode() {
    local account="$1"
    local list_json
    list_json="$(lumerad_query supernode list-supernodes)" || return 1
    echo "$list_json" | jq -c --arg account "$account" '.supernodes[]? | select(.supernode_account == $account)' 2>/dev/null | head -n 1
}

get_service_validator_address() {
    local account sn_json
    account="$(service_supernode_account_address)" || return 1
    [[ -n "$account" ]] || return 1
    sn_json="$(get_service_supernode "$account")" || return 1
    echo "$sn_json" | jq -r '.validator_address // empty' 2>/dev/null
}

wait_for_supernode_state() {
    local validator="$1" expected="$2" timeout_s="${3:-30}"
    local deadline=$((SECONDS + timeout_s))
    local last_state=""

    while (( SECONDS < deadline )); do
        local sn_json
        sn_json="$(lumerad_query supernode get-supernode "$validator")" || true
        last_state="$(echo "$sn_json" | jq -r '.supernode.states[-1].state // empty' 2>/dev/null)"
        if [[ "$last_state" == "$expected" ]]; then
            return 0
        fi
        sleep 2
    done

    echo "$last_state"
    return 1
}

coin_amount() {
    local json="$1" denom="$2"
    echo "$json" | jq -r --arg denom "$denom" '
        [
            (.balance // [])[]
            | select(.denom == $denom)
            | (.amount | tonumber)
        ] | add // 0
    ' 2>/dev/null
}

# Record a PASS result.
pass() {
    local name="$1"
    PASS_COUNT=$((PASS_COUNT + 1))
    RESULTS+=("PASS  $name")
    echo "  PASS: $name"
}

# Record a FAIL result.
fail() {
    local name="$1"
    shift
    FAIL_COUNT=$((FAIL_COUNT + 1))
    RESULTS+=("FAIL  $name -- $*")
    echo "  FAIL: $name -- $*"
}

# Record a SKIP result.
skip() {
    local name="$1"
    shift
    SKIP_COUNT=$((SKIP_COUNT + 1))
    RESULTS+=("SKIP  $name -- $*")
    echo "  SKIP: $name -- $*"
}

# Assert that a jq expression applied to JSON produces a truthy result.
# Usage: assert_jq "$json" '<jq filter>' "test name"
assert_jq() {
    local json="$1" filter="$2" name="$3"
    if echo "$json" | jq -e "$filter" >/dev/null 2>&1; then
        pass "$name"
    else
        local actual
        actual="$(echo "$json" | jq -r "$filter" 2>/dev/null || echo '<jq error>')"
        fail "$name" "filter=$filter actual=$actual"
    fi
}

# Assert a string is non-empty.
assert_nonempty() {
    local value="$1" name="$2"
    if [[ -n "$value" && "$value" != "null" ]]; then
        pass "$name"
    else
        fail "$name" "expected non-empty value"
    fi
}

# ---------------------------------------------------------------------------
# Scenario 1: Module Bootstrap (F14, F18)
# ---------------------------------------------------------------------------
scenario_1_module_bootstrap() {
    echo ""
    echo "=== Scenario 1: Module Bootstrap (F14, F18) ==="

    # 1a. Query supernode params — reward_distribution must be present
    local params
    params="$(lumerad_query supernode params)" || true
    if [[ -z "$params" ]]; then
        fail "S1.1 supernode params query" "query returned empty"
        return
    fi
    assert_jq "$params" '.params.reward_distribution | length > 0' "S1.1 reward_distribution non-empty"
    assert_jq "$params" '.params.reward_distribution.payment_period_blocks != null' "S1.1a payment_period_blocks present"
    assert_jq "$params" '.params.reward_distribution.registration_fee_share_bps != null' "S1.1b registration_fee_share_bps present"

    # 1b. Query supernode pool-state
    local pool
    pool="$(lumerad_query supernode pool-state)" || true
    if [[ -z "$pool" ]]; then
        fail "S1.2 pool-state query" "query returned empty"
    else
        assert_jq "$pool" '. | length > 0' "S1.2 pool-state returns data"
    fi

    # 1c. Query auth module-account supernode
    local modacct
    modacct="$(lumerad_query auth module-account supernode)" || true
    if [[ -z "$modacct" ]]; then
        fail "S1.3 supernode module account" "query returned empty"
    else
        assert_jq "$modacct" '.account != null' "S1.3 supernode module account exists"

        local module_addr key_name sender_addr before_pool send_amount tx_result tx_code pool_after before_amt after_amt
        module_addr="$(echo "$modacct" | jq -r '
            .account.value.address //
            .account.base_account.address //
            .account.value.base_account.address //
            .account.address //
            empty' 2>/dev/null)"
        key_name="$(service_key_name)"
        sender_addr="$(lumerad_exec keys show "$key_name" -a --keyring-backend "$KEYRING" 2>/dev/null | tr -d '\r\n')" || true

        echo "    DEBUG: module_addr=$module_addr"
        echo "    DEBUG: sender_addr=$sender_addr"

        if [[ -n "$module_addr" && -n "$sender_addr" ]]; then
            before_pool="$pool"
            before_amt="$(coin_amount "$before_pool" "$DENOM")"
            echo "    DEBUG: before_pool=$before_pool"
            echo "    DEBUG: before_amt=$before_amt"

            send_amount="10000${DENOM}"
            echo "    DEBUG: sending $send_amount from $sender_addr to $module_addr"
            tx_result="$(run_tx_with_retry "$SERVICE" bank send "$sender_addr" "$module_addr" "$send_amount" --from "$key_name")" || true
            tx_code="$(tx_code_from_json "$tx_result")"
            echo "    DEBUG: tx_result=${tx_result:0:300}"
            echo "    DEBUG: tx_code=$tx_code"

            if [[ "$tx_code" == "0" ]]; then
                local txhash
                txhash="$(echo "$tx_result" | jq -r '.txhash // empty' 2>/dev/null)"
                echo "    DEBUG: txhash=$txhash"
                if [[ -n "$txhash" ]]; then
                    sleep 6
                    local tx_check
                    tx_check="$(lumerad_query tx "$txhash" 2>/dev/null)" || true
                    local exec_code exec_log
                    exec_code="$(echo "$tx_check" | jq -r '.code // "0"' 2>/dev/null || echo "0")"
                    exec_log="$(echo "$tx_check" | jq -r '.raw_log // .log // empty' 2>/dev/null || echo "")"
                    echo "    DEBUG: tx exec_code=$exec_code"
                    echo "    DEBUG: tx exec_log=${exec_log:0:300}"
                    if [[ "$exec_code" != "0" ]]; then
                        fail "S1.3a fund supernode module account tx accepted" "tx failed at execution: code=$exec_code log=$exec_log"
                        return
                    fi
                fi
                pass "S1.3a fund supernode module account tx accepted"

                pool_after="$(lumerad_query supernode pool-state)" || true
                echo "    DEBUG: pool_after=$pool_after"
                if [[ -n "$pool_after" ]]; then
                    after_amt="$(coin_amount "$pool_after" "$DENOM")"
                    echo "    DEBUG: after_amt=$after_amt"
                    if [[ -n "$before_amt" && -n "$after_amt" ]] && (( after_amt >= before_amt + 10000 )); then
                        pass "S1.3b pool balance increased after funding"
                    else
                        fail "S1.3b pool balance increased after funding" "before=$before_amt after=$after_amt"
                    fi
                else
                    fail "S1.3b pool balance increased after funding" "post-funding pool-state query returned empty"
                fi
            else
                fail "S1.3a fund supernode module account tx accepted" "code=$tx_code output=${tx_result:0:300}"
            fi
        else
            skip "S1.3a/S1.3b fund supernode module account" "module_addr='${module_addr:-<empty>}' sender_addr='${sender_addr:-<empty>}' key_name='$key_name'"
        fi
    fi

    # 1d. Verify max_storage_usage_percent is set (drives STORAGE_FULL transitions).
    assert_jq "$params" '.params.max_storage_usage_percent != null' \
        "S1.4 max_storage_usage_percent present in supernode params"
}

# ---------------------------------------------------------------------------
# Scenario 2: STORAGE_FULL State Transition (F12, F13)
# ---------------------------------------------------------------------------
scenario_2_storage_full_transition() {
    echo ""
    echo "=== Scenario 2: STORAGE_FULL State Transition (F12, F13) ==="

    local service_addr
    service_addr="$(service_supernode_account_address)" || true
    if [[ -z "$service_addr" ]]; then
        skip "S2.1 resolve service account" "could not resolve $(service_supernode_key_name)"
        return
    fi

    local sn_json
    sn_json="$(get_service_supernode "$service_addr")" || true
    if [[ -z "$sn_json" ]]; then
        skip "S2.1 resolve service supernode" "no supernode found for account $service_addr"
        return
    fi

    local validator_addr supernode_account current_state
    validator_addr="$(echo "$sn_json" | jq -r '.validator_address // empty' 2>/dev/null)"
    supernode_account="$(echo "$sn_json" | jq -r '.supernode_account // empty' 2>/dev/null)"
    current_state="$(echo "$sn_json" | jq -r '.states[-1].state // empty' 2>/dev/null)"

    if [[ -z "$validator_addr" || -z "$supernode_account" ]]; then
        skip "S2.1 resolve service supernode" "missing validator or supernode account in query response"
        return
    fi
    if [[ "$current_state" == "SUPERNODE_STATE_STORAGE_FULL" ]]; then
        skip "S2.1 resolve service supernode" "service supernode is already in STORAGE_FULL"
        return
    fi
    pass "S2.1 resolved service supernode (validator=$validator_addr state=${current_state:-unknown})"

    local params max_usage target_usage ports_json metrics_json
    params="$(lumerad_query supernode params)" || true
    if [[ -z "$params" ]]; then
        fail "S2.2 supernode params query" "query returned empty"
        return
    fi

    max_usage="$(echo "$params" | jq -r '.params.max_storage_usage_percent // empty' 2>/dev/null)"
    if [[ -z "$max_usage" || ! "$max_usage" =~ ^[0-9]+$ ]]; then
        fail "S2.2 supernode params query" "invalid max_storage_usage_percent=$max_usage"
        return
    fi

    target_usage=$(( max_usage + 1 ))
    if (( target_usage > 100 )); then
        skip "S2.2 disk-full report threshold" "max_storage_usage_percent=$max_usage leaves no valid >threshold test value"
        return
    fi

    ports_json="$(echo "$params" | jq -c '.params.required_open_ports // [] | map({port: ., state: "PORT_STATE_OPEN"})' 2>/dev/null)"
    if [[ -z "$ports_json" || "$ports_json" == "null" ]]; then
        ports_json="[]"
    fi

    metrics_json="$(jq -cn \
        --argjson usage "$target_usage" \
        --argjson ports "$ports_json" \
        '{
            version_major: 2,
            version_minor: 0,
            version_patch: 0,
            cpu_cores_total: 16,
            cpu_usage_percent: 25,
            mem_total_gb: 64,
            mem_usage_percent: 40,
            mem_free_gb: 32,
            disk_total_gb: 2000,
            disk_usage_percent: $usage,
            disk_free_gb: 50,
            uptime_seconds: 3600,
            peers_count: 10,
            cascade_kademlia_db_bytes: 2147483648,
            open_ports: $ports
        }')"

    local key_name tx_result tx_code txhash tx_check exec_code
    key_name="$(service_supernode_key_name)"
    tx_result="$(run_tx_with_retry "$SERVICE" supernode report-supernode-metrics \
        --validator-address "$validator_addr" \
        --metrics "$metrics_json" \
        --from "$key_name")" || true
    tx_code="$(tx_code_from_json "$tx_result")"

    if [[ "$tx_code" != "0" ]]; then
        fail "S2.3 report disk-full metrics tx accepted" "code=$tx_code output=${tx_result:0:300}"
        return
    fi

    txhash="$(echo "$tx_result" | jq -r '.txhash // empty' 2>/dev/null)"
    if [[ -z "$txhash" ]]; then
        fail "S2.3 report disk-full metrics tx accepted" "missing txhash output=${tx_result:0:300}"
        return
    fi

    sleep 6
    tx_check="$(lumerad_query tx "$txhash")" || true
    exec_code="$(echo "$tx_check" | jq -r '.code // "0"' 2>/dev/null || echo "0")"
    if [[ "$exec_code" != "0" ]]; then
        fail "S2.3 report disk-full metrics tx accepted" "tx execution failed code=$exec_code"
        return
    fi
    pass "S2.3 report disk-full metrics tx accepted"

    local metrics_state reported_usage
    metrics_state="$(supernode_metrics_rest "$validator_addr")" || true
    if [[ -n "$metrics_state" ]] && [[ "$(echo "$metrics_state" | jq -r '.code // empty' 2>/dev/null)" != "5" ]]; then
        reported_usage="$(echo "$metrics_state" | jq -r '.metrics_state.metrics.disk_usage_percent // empty' 2>/dev/null)"
        if [[ "$reported_usage" == "$target_usage" ]]; then
            pass "S2.4 reported disk usage stored on-chain (value=$reported_usage)"
        else
            fail "S2.4 reported disk usage stored on-chain" "expected=$target_usage actual=$reported_usage"
        fi
    else
        fail "S2.4 reported disk usage stored on-chain" "metrics query returned empty"
    fi

    local observed_state
    if observed_state="$(wait_for_supernode_state "$validator_addr" "SUPERNODE_STATE_STORAGE_FULL" 30)"; then
        pass "S2.5 supernode transitions to STORAGE_FULL after disk-full report"
    else
        fail "S2.5 supernode transitions to STORAGE_FULL after disk-full report" "final_state=$observed_state"
    fi
}

# ---------------------------------------------------------------------------
# Scenario 6: Registration Fee Share (F16)
# ---------------------------------------------------------------------------
scenario_6_registration_fee_share() {
    echo ""
    echo "=== Scenario 6: Registration Fee Share (F16) ==="

    local params
    params="$(lumerad_query supernode params)" || true
    if [[ -z "$params" ]]; then
        fail "S6.1 supernode params query" "query returned empty"
        return
    fi

    local bps
    bps="$(echo "$params" | jq -r '.params.reward_distribution.registration_fee_share_bps // empty')"
    assert_nonempty "$bps" "S6.1 registration_fee_share_bps is set"

    if [[ -n "$bps" && "$bps" != "null" ]]; then
        if [[ "$bps" =~ ^[0-9]+$ ]] && (( bps > 0 )); then
            pass "S6.2 registration_fee_share_bps > 0 (value=$bps)"
        else
            fail "S6.2 registration_fee_share_bps > 0" "got: $bps"
        fi
    fi
}

# ---------------------------------------------------------------------------
# Scenario 7: Governance (F11, F14)
# ---------------------------------------------------------------------------
scenario_7_governance() {
    echo ""
    echo "=== Scenario 7: Governance (F11, F14) ==="

    # 7a. Query defaults
    local params
    params="$(lumerad_query supernode params)" || true
    if [[ -z "$params" ]]; then
        fail "S7.1 default params returned" "query returned empty"
        return
    fi
    assert_jq "$params" '.params.reward_distribution.payment_period_blocks != null' "S7.1 default params returned"

    # 7b. Submit a governance proposal to update supernode params, vote, wait,
    #     then verify the param change took effect.
    local key_name="${SERVICE}_key"
    local sender_addr
    sender_addr="$(lumerad_exec keys show "$key_name" -a --keyring-backend "$KEYRING" 2>/dev/null | tr -d '\r\n')"

    if [[ -z "$sender_addr" ]]; then
        fail "S7.2 gov proposal submit" "could not resolve key $key_name"
        return
    fi

    # Resolve the gov module authority address.
    local gov_acct gov_addr
    gov_acct="$(lumerad_query auth module-account gov)" || true
    gov_addr="$(echo "$gov_acct" | jq -r '
        .account.value.address //
        .account.base_account.address //
        .account.value.base_account.address //
        .account.address //
        empty' 2>/dev/null)"

    if [[ -z "$gov_addr" ]]; then
        fail "S7.2 gov proposal submit" "could not resolve gov module address"
        return
    fi
    echo "    DEBUG: gov_addr=$gov_addr"

    # Read the current payment_period_blocks so we can change it.
    local orig_ppb
    orig_ppb="$(echo "$params" | jq -r '.params.reward_distribution.payment_period_blocks // "100"')"
    local new_ppb=$(( orig_ppb + 50 ))
    echo "    DEBUG: orig_ppb=$orig_ppb new_ppb=$new_ppb"

    # Build a full set of current params with the one field changed.
    local current_params updated_params
    current_params="$(echo "$params" | jq '.params')"
    updated_params="$(echo "$current_params" | jq --arg ppb "$new_ppb" '.reward_distribution.payment_period_blocks = $ppb')"

    # Write the proposal JSON into the container.
    local proposal_file="/tmp/sn_param_proposal.json"
    docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE" bash -c "cat > $proposal_file" <<PROPEOF
{
    "messages": [{
        "@type": "/lumera.supernode.v1.MsgUpdateParams",
        "authority": "$gov_addr",
        "params": $updated_params
    }],
    "deposit": "1000000000${DENOM}",
    "metadata": "",
    "title": "Update Supernode Params (devnet test)",
    "summary": "Automated devnet test: change payment_period_blocks from $orig_ppb to $new_ppb"
}
PROPEOF

    # Submit the proposal.
    local submit_result submit_code
    submit_result="$(run_tx_with_retry "$SERVICE" gov submit-proposal "$proposal_file" --from "$key_name")" || true
    submit_code="$(echo "$submit_result" | jq -r '.code // empty' 2>/dev/null || echo "")"
    echo "    DEBUG: submit_result=${submit_result:0:400}"

    if [[ -n "$submit_code" && "$submit_code" != "0" ]]; then
        fail "S7.2 gov proposal submit" "tx code=$submit_code"
        return
    fi

    # Wait for submission tx to land.
    local submit_txhash
    submit_txhash="$(echo "$submit_result" | jq -r '.txhash // empty' 2>/dev/null)"
    if [[ -n "$submit_txhash" ]]; then
        sleep 6
        local submit_check submit_exec_code
        submit_check="$(lumerad_query tx "$submit_txhash")" || true
        submit_exec_code="$(echo "$submit_check" | jq -r '.code // "0"' 2>/dev/null || echo "0")"
        if [[ "$submit_exec_code" != "0" ]]; then
            fail "S7.2 gov proposal submit" "tx execution failed code=$submit_exec_code"
            return
        fi
    fi
    pass "S7.2 gov proposal submitted"

    # Find the proposal ID (last proposal from this depositor).
    local proposals_json proposal_id
    proposals_json="$(lumerad_query gov proposals --depositor "$sender_addr")" || true
    proposal_id="$(echo "$proposals_json" | jq -r '.proposals[-1].id // empty' 2>/dev/null)"

    if [[ -z "$proposal_id" ]]; then
        fail "S7.3 gov proposal found" "could not find proposal from depositor $sender_addr"
        return
    fi
    echo "    DEBUG: proposal_id=$proposal_id"
    pass "S7.3 gov proposal found (id=$proposal_id)"

    # Vote yes from multiple validators to meet quorum (33.4%).
    # With 5 equal-weight validators, we need at least 2 votes (40%).
    local vote_ok=true
    for voter_svc in supernova_validator_1 supernova_validator_2; do
        local voter_key="${voter_svc}_key"
        local vote_result vote_code
        vote_result="$(run_tx_with_retry "$voter_svc" gov vote "$proposal_id" yes --from "$voter_key")" || true
        vote_code="$(echo "$vote_result" | jq -r '.code // empty' 2>/dev/null || echo "")"
        echo "    DEBUG: vote from $voter_svc code=$vote_code txhash=$(echo "$vote_result" | jq -r '.txhash // empty' 2>/dev/null)"

        if [[ -n "$vote_code" && "$vote_code" != "0" ]]; then
            echo "    WARN: vote from $voter_svc failed with code=$vote_code"
            vote_ok=false
        fi
        sleep 3
    done

    if $vote_ok; then
        pass "S7.4 gov votes accepted (2 validators)"
    else
        fail "S7.4 gov votes accepted" "one or more votes failed"
        return
    fi

    # Wait for vote txs to land.
    sleep 6

    # Wait for the voting period to end and proposal to pass.
    # Devnet voting_period is 30s; poll for up to 60s.
    echo "    Waiting for voting period to end..."
    local deadline=$((SECONDS + 60))
    local prop_status=""
    while (( SECONDS < deadline )); do
        sleep 5
        local prop_json
        prop_json="$(lumerad_query gov proposal "$proposal_id")" || true
        prop_status="$(echo "$prop_json" | jq -r '.proposal.status // empty' 2>/dev/null)"
        echo "    DEBUG: proposal status=$prop_status"
        if [[ "$prop_status" == "PROPOSAL_STATUS_PASSED" ]]; then
            break
        fi
        if [[ "$prop_status" == "PROPOSAL_STATUS_REJECTED" || "$prop_status" == "PROPOSAL_STATUS_FAILED" ]]; then
            break
        fi
    done

    if [[ "$prop_status" == "PROPOSAL_STATUS_PASSED" ]]; then
        pass "S7.5 gov proposal passed"
    else
        fail "S7.5 gov proposal passed" "final status=$prop_status"
        return
    fi

    # Verify the param was actually updated.
    local new_params new_ppb_actual
    new_params="$(lumerad_query supernode params)" || true
    new_ppb_actual="$(echo "$new_params" | jq -r '.params.reward_distribution.payment_period_blocks // empty')"
    echo "    DEBUG: expected=$new_ppb actual=$new_ppb_actual"

    if [[ "$new_ppb_actual" == "$new_ppb" ]]; then
        pass "S7.6 param updated via governance (payment_period_blocks: $orig_ppb -> $new_ppb)"
    else
        fail "S7.6 param updated via governance" "expected=$new_ppb actual=$new_ppb_actual"
    fi
}

# ---------------------------------------------------------------------------
# Scenario 8: Proto Compatibility (F10, F11)
# ---------------------------------------------------------------------------
scenario_8_proto_compatibility() {
    echo ""
    echo "=== Scenario 8: Proto Compatibility (F10, F11) ==="

    # 8a. Query supernode params for current STORAGE_FULL behavior.
    local snparams
    snparams="$(lumerad_query supernode params)" || true
    if [[ -z "$snparams" ]]; then
        fail "S8.1 supernode params" "query returned empty"
    else
        local max_usage
        max_usage="$(echo "$snparams" | jq -r '.params.max_storage_usage_percent // "0"')"
        if [[ "$max_usage" =~ ^[0-9]+$ ]] && (( max_usage > 0 )); then
            pass "S8.1 max_storage_usage_percent in supernode params (value=$max_usage)"
        else
            fail "S8.1 max_storage_usage_percent in supernode params" "unexpected value=$max_usage"
        fi
    fi

    local target_validator
    target_validator="$(get_service_validator_address)" || true
    if [[ -z "$target_validator" ]]; then
        target_validator="$(get_first_validator_address)" || true
    fi
    if [[ -n "$target_validator" ]]; then
        local sn metrics
        sn="$(lumerad_query supernode get-supernode "$target_validator")" || true
        if [[ -n "$sn" ]]; then
            assert_jq "$sn" '.supernode.validator_address != null' \
                "S8.1a supernode query returns validator record"
            assert_jq "$sn" '.supernode.states | length > 0' \
                "S8.1b supernode query exposes state history"
        else
            fail "S8.1a supernode query returns validator record" "query returned empty for $target_validator"
        fi

        metrics="$(supernode_metrics_rest "$target_validator")" || true
        if [[ -n "$metrics" ]] && [[ "$(echo "$metrics" | jq -r '.code // empty' 2>/dev/null)" != "5" ]]; then
            assert_jq "$metrics" '.metrics_state.metrics.cascade_kademlia_db_bytes != null' \
                "S8.1c cascade_kademlia_db_bytes present in metrics query"
        else
            skip "S8.1c cascade_kademlia_db_bytes present in metrics query" "no metrics found for $target_validator"
        fi
    else
        skip "S8.1a/S8.1c live supernode proto checks" "no registered supernode found on devnet"
    fi

}

# ---------------------------------------------------------------------------
# Scenarios 2-5, 9, 10: Stubs (require registered supernodes)
# ---------------------------------------------------------------------------
scenario_stubs() {
    echo ""
    echo "=== Scenarios requiring registered supernodes (stubbed) ==="

    # Scenario 3: Periodic Distribution -- Happy Path (F15)
    # Requires: 2+ registered supernodes with varying cascade_kademlia_db_bytes,
    # funded Everlight pool, small payment_period_blocks. Needs supernode
    # registration and multiple block advancement.
    skip "S3 periodic distribution happy path" "requires multiple registered supernodes with metrics"

    # Scenario 4: Distribution Edge Cases (F15)
    # Requires: configurable supernodes with varying states and metric values.
    skip "S4 distribution edge cases" "requires configurable supernodes"

    # Scenario 5: Anti-Gaming Guardrails (F15)
    # Requires: supernodes across multiple distribution periods with varying
    # metrics to test ramp-up, growth cap, and smoothing.
    skip "S5 anti-gaming guardrails" "requires multi-period supernode metrics history"

    # Scenario 9: Upgrade Handler Idempotency (F18)
    # Requires: chain started from pre-Everlight genesis with existing
    # supernodes, then upgraded to v1.15.0. Needs the upgrade devnet flow.
    skip "S9 upgrade handler idempotency" "requires pre-Everlight genesis and upgrade flow"

    # Scenario 10: Full Lifecycle (Cross-Feature)
    # Requires: full supernode registration, funding, action submission, and
    # multi-period distribution. End-to-end flow that combines all features.
    skip "S10 full lifecycle" "requires full supernode lifecycle setup"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    echo "============================================================"
    echo "  Everlight Phase 1 -- Devnet Integration Tests"
    echo "============================================================"
    echo "  COMPOSE_FILE: $COMPOSE_FILE"
    echo "  SERVICE:      $SERVICE"
    echo "  CHAIN_ID:     $CHAIN_ID"
    echo "============================================================"

    # Verify the service is reachable
    echo ""
    echo "--- Checking devnet connectivity ---"
    local status
    status="$(lumerad_exec status 2>/dev/null | jq -r '.sync_info.latest_block_height // empty' 2>/dev/null)" || true
    if [[ -z "$status" ]]; then
        echo "FATAL: Cannot reach $SERVICE via docker compose. Is devnet running?"
        echo "       Run: make devnet-up-detach"
        exit 1
    fi
    echo "  Connected. Current block height: $status"

    # Wait for chain to stabilize — early blocks may still be processing
    # genesis transactions which can cause sequence mismatches.
    if (( status < 10 )); then
        echo "  Waiting for chain to stabilize (height < 10)..."
        sleep 10
    fi

    scenario_1_module_bootstrap
    scenario_6_registration_fee_share
    scenario_7_governance
    scenario_8_proto_compatibility
    scenario_2_storage_full_transition
    scenario_stubs

    # Summary
    echo ""
    echo "============================================================"
    echo "  RESULTS SUMMARY"
    echo "============================================================"
    for r in "${RESULTS[@]}"; do
        echo "  $r"
    done
    echo "------------------------------------------------------------"
    echo "  PASS: $PASS_COUNT   FAIL: $FAIL_COUNT   SKIP: $SKIP_COUNT"
    echo "============================================================"

    if (( FAIL_COUNT > 0 )); then
        exit 1
    fi
    exit 0
}

main "$@"
