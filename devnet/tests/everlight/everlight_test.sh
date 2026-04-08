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

# Run lumerad query and return JSON.
lumerad_query() {
    lumerad_exec query "$@" --output json 2>/dev/null
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

service_key_name() {
    echo "${SERVICE}_key"
}

get_first_validator_address() {
    local list_json
    list_json="$(lumerad_query supernode list-super-nodes)" || return 1
    echo "$list_json" | jq -r '.supernode[]?.validator_address // empty' | head -n 1
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

    # 1c. Query auth module-account everlight
    local modacct
    modacct="$(lumerad_query auth module-account everlight)" || true
    if [[ -z "$modacct" ]]; then
        fail "S1.3 everlight pool account" "query returned empty"
    else
        assert_jq "$modacct" '.account != null' "S1.3 everlight pool account exists"

        local module_addr key_name sender_addr before_pool send_amount tx_result tx_code pool_after before_amt after_amt
        module_addr="$(echo "$modacct" | jq -r '.account.base_account.address // .account.value.base_account.address // empty' 2>/dev/null)"
        key_name="$(service_key_name)"
        sender_addr="$(lumerad_exec keys show "$key_name" -a --keyring-backend "$KEYRING" 2>/dev/null | tr -d '\r\n')" || true

        if [[ -n "$module_addr" && -n "$sender_addr" ]]; then
            before_pool="$pool"
            before_amt="$(coin_amount "$before_pool" "$DENOM")"
            send_amount="10000${DENOM}"
            tx_result="$(lumerad_tx bank send "$sender_addr" "$module_addr" "$send_amount" --from "$key_name")" || true
            tx_code="$(echo "$tx_result" | jq -r '.code // "0"' 2>/dev/null || echo "0")"

            if [[ "$tx_code" == "0" ]]; then
                pass "S1.3a fund everlight module account tx accepted"
                sleep 2
                pool_after="$(lumerad_query supernode pool-state)" || true
                if [[ -n "$pool_after" ]]; then
                    after_amt="$(coin_amount "$pool_after" "$DENOM")"
                    if [[ -n "$before_amt" && -n "$after_amt" ]] && (( after_amt >= before_amt + 10000 )); then
                        pass "S1.3b pool balance increased after funding"
                    else
                        fail "S1.3b pool balance increased after funding" "before=$before_amt after=$after_amt"
                    fi
                else
                    fail "S1.3b pool balance increased after funding" "post-funding pool-state query returned empty"
                fi
            else
                fail "S1.3a fund everlight module account tx accepted" "code=$tx_code output=${tx_result:0:200}"
            fi
        else
            skip "S1.3a/S1.3b fund everlight module account" "could not resolve module address or sender key"
        fi
    fi

    # 1d. Query supernode params for cascade_kademlia_db_max_bytes
    # (already fetched above, reuse $params)
    assert_jq "$params" '.params.cascade_kademlia_db_max_bytes != null' \
        "S1.4 cascade_kademlia_db_max_bytes present in supernode params"
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

    # 7b. Submit MsgUpdateParams from a non-authority address (should fail)
    # Use the validator's own key (not governance authority) to send MsgUpdateParams.
    local key_name="${SERVICE}_key"
    local sender_addr
    sender_addr="$(lumerad_exec keys show "$key_name" -a --keyring-backend "$KEYRING" 2>/dev/null | tr -d '\r\n')"

    if [[ -z "$sender_addr" ]]; then
        fail "S7.2 non-authority rejection" "could not resolve key $key_name"
        return
    fi

    # Build a minimal MsgUpdateParams JSON and attempt to submit it as a
    # generic tx. We expect rejection because the sender is not the module
    # governance authority.
    local tmpfile="/tmp/supernode_bad_params.json"
    local current_params
    current_params="$(echo "$params" | jq '.params')"

    # Write the message JSON into the container
    docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE" bash -c "cat > $tmpfile" <<PARAMEOF
{
    "@type": "/lumera.supernode.v1.MsgUpdateParams",
    "authority": "$sender_addr",
    "params": $current_params
}
PARAMEOF

    local tx_result
    tx_result="$(lumerad_exec tx supernode update-params "$tmpfile" \
        --from "$key_name" \
        --chain-id "$CHAIN_ID" \
        --keyring-backend "$KEYRING" \
        --fees "$FEES" \
        --broadcast-mode sync \
        --output json \
        --yes 2>&1)" || true

    # The tx may fail at broadcast (code != 0) or at execution.
    # Check if it was rejected.
    local code
    code="$(echo "$tx_result" | jq -r '.code // empty' 2>/dev/null || echo "")"

    if [[ -n "$code" && "$code" != "0" ]]; then
        pass "S7.2 non-authority MsgUpdateParams rejected (code=$code)"
    else
        # If broadcast succeeded with code 0, we need to wait for the tx and
        # check its execution result.
        local txhash
        txhash="$(echo "$tx_result" | jq -r '.txhash // empty' 2>/dev/null || echo "")"
        if [[ -n "$txhash" ]]; then
            sleep 3
            local wait_result
            wait_result="$(lumerad_query tx "$txhash" 2>/dev/null)" || true
            local exec_code
            exec_code="$(echo "$wait_result" | jq -r '.code // "0"' 2>/dev/null || echo "0")"
            if [[ "$exec_code" != "0" ]]; then
                pass "S7.2 non-authority MsgUpdateParams rejected at execution (code=$exec_code)"
            else
                # Check if the error was in the initial output (some versions
                # return an error string rather than JSON)
                if echo "$tx_result" | grep -qi "unauthorized\|authority\|permission\|invalid"; then
                    pass "S7.2 non-authority MsgUpdateParams rejected (error in output)"
                else
                    fail "S7.2 non-authority MsgUpdateParams rejected" "tx appeared to succeed"
                fi
            fi
        else
            # No JSON response; check raw output for rejection
            if echo "$tx_result" | grep -qi "unauthorized\|authority\|permission\|invalid"; then
                pass "S7.2 non-authority MsgUpdateParams rejected (error in output)"
            else
                fail "S7.2 non-authority MsgUpdateParams rejected" "unexpected response: ${tx_result:0:200}"
            fi
        fi
    fi
}

# ---------------------------------------------------------------------------
# Scenario 8: Proto Compatibility (F10, F11)
# ---------------------------------------------------------------------------
scenario_8_proto_compatibility() {
    echo ""
    echo "=== Scenario 8: Proto Compatibility (F10, F11) ==="

    # 8a. Query supernode params for cascade_kademlia_db_max_bytes
    local snparams
    snparams="$(lumerad_query supernode params)" || true
    if [[ -z "$snparams" ]]; then
        fail "S8.1 supernode params" "query returned empty"
    else
        assert_jq "$snparams" '.params.cascade_kademlia_db_max_bytes != null' \
            "S8.1 cascade_kademlia_db_max_bytes in supernode params"
    fi

    local first_validator
    first_validator="$(get_first_validator_address)" || true
    if [[ -n "$first_validator" ]]; then
        local sn metrics
        sn="$(lumerad_query supernode get-super-node "$first_validator")" || true
        if [[ -n "$sn" ]]; then
            assert_jq "$sn" '.super_node.validator_address != null' \
                "S8.1a supernode query returns validator record"
            assert_jq "$sn" '.super_node.states | length > 0' \
                "S8.1b supernode query exposes state history"
        else
            fail "S8.1a supernode query returns validator record" "query returned empty for $first_validator"
        fi

        metrics="$(lumerad_query supernode get-metrics "$first_validator")" || true
        if [[ -n "$metrics" ]]; then
            assert_jq "$metrics" '.metrics_state.metrics.cascade_kademlia_db_bytes != null' \
                "S8.1c cascade_kademlia_db_bytes present in metrics query"
        else
            skip "S8.1c cascade_kademlia_db_bytes present in metrics query" "no metrics found for $first_validator"
        fi
    else
        skip "S8.1a/S8.1c live supernode proto checks" "no registered supernode found on devnet"
    fi

    # 8b. Export genesis and verify supernode section contains reward_distribution
    local genesis
    genesis="$(lumerad_exec genesis export 2>/dev/null)" || true
    if [[ -z "$genesis" ]]; then
        fail "S8.2 genesis export" "export returned empty"
    else
        local sn_section
        sn_section="$(echo "$genesis" | jq '.app_state.supernode // empty' 2>/dev/null)"
        if [[ -n "$sn_section" && "$sn_section" != "null" ]]; then
            pass "S8.2 supernode section present in genesis export"
            assert_jq "$sn_section" '.params.reward_distribution != null' \
                "S8.2a reward_distribution in exported supernode params"
        else
            fail "S8.2 supernode section present in genesis export" "section missing or null"
        fi
    fi
}

# ---------------------------------------------------------------------------
# Scenarios 2-5, 9, 10: Stubs (require registered supernodes)
# ---------------------------------------------------------------------------
scenario_stubs() {
    echo ""
    echo "=== Scenarios requiring registered supernodes (stubbed) ==="

    # Scenario 2: STORAGE_FULL State Transitions (F12, F13)
    # Requires: registered supernode, setting cascade_kademlia_db_max_bytes via
    # governance, reporting metrics with MsgReportSupernodeMetrics exceeding the
    # threshold. Needs a full supernode registration flow including staking.
    skip "S2 STORAGE_FULL transitions" "requires registered supernode with metrics reporting"

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

    scenario_1_module_bootstrap
    scenario_6_registration_fee_share
    scenario_7_governance
    scenario_8_proto_compatibility
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
