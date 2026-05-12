#!/usr/bin/env bash
###############################################################################
# LEP-6 Storage-Truth Enforcement — Devnet Integration Tests
#
# Exercises the chain-side LEP-6 lifecycle against the running 5-validator
# Docker devnet. Modeled on devnet/tests/everlight/everlight_test.sh — uses
# `lumerad` CLI inside validator containers for tx/query, with no dependency
# on the off-chain supernode runtime (the test plays the SN role by signing
# as the registered SN identities created by supernode-setup.sh).
#
# Tests:
#   T1  TestLEP6_ParamsAndEpochAnchor
#   T2  TestLEP6_SubmitEpochReport_HappyPath
#   T3  TestLEP6_SubmitStorageRecheckEvidence_UpdatesSuspicionScore
#   T4  TestLEP6_HealOpLifecycle_ClaimVerifyFinalize
#   T5  TestLEP6_RecheckEvidenceRejectsUnauthorizedSubmitter
#   T6  TestLEP6_ClaimHealCompleteRejectsNonexistentOp
#   T7  TestLEP6_HealVerificationRejectsDuplicateVote (inside T4 lifecycle)
#
# Pre-requisites:
#   - Devnet up via `make devnet-up-detach`
#   - Existing registered supernodes, or key-resolvable validator accounts so
#     this script can bootstrap validator-owned supernodes when fewer than 3
#     are registered.
#
# Usage:
#   COMPOSE_FILE=devnet/docker-compose.yml bash devnet/tests/lep6/lep6_test.sh
#
# Environment variables (all optional):
#   COMPOSE_FILE      path to docker-compose.yml (default: devnet/docker-compose.yml)
#   CHAIN_ID          chain id                   (default: lumera-devnet-1)
#   FEES              tx fees                    (default: 5000ulume)
#   GAS               gas limit                  (default: 500000)
#   LEP6_VERBOSE      set to 1 for verbose tx/query JSON dumps
###############################################################################
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
COMPOSE_FILE="${COMPOSE_FILE:-devnet/docker-compose.yml}"
CHAIN_ID="${CHAIN_ID:-lumera-devnet-1}"
KEYRING="test"
DENOM="ulume"
FEES="${FEES:-5000${DENOM}}"
GAS="${GAS:-500000}"
VERBOSE="${LEP6_VERBOSE:-0}"

VALIDATOR_SERVICES=(supernova_validator_1 supernova_validator_2 supernova_validator_3 supernova_validator_4 supernova_validator_5)
PRIMARY_SERVICE="${VALIDATOR_SERVICES[0]}"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
RESULTS=()

# ---------------------------------------------------------------------------
# Result helpers
# ---------------------------------------------------------------------------
pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    RESULTS+=("PASS: $1")
    printf '  ✓ PASS: %s\n' "$1"
}
fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    RESULTS+=("FAIL: $1 — $2")
    printf '  ✗ FAIL: %s — %s\n' "$1" "$2" >&2
}
skip() {
    SKIP_COUNT=$((SKIP_COUNT + 1))
    RESULTS+=("SKIP: $1 — $2")
    printf '  ⊘ SKIP: %s — %s\n' "$1" "$2"
}
section() {
    printf '\n============================================================\n'
    printf '%s\n' "$1"
    printf '============================================================\n'
}
log() {
    printf '    %s\n' "$1"
}
debug() {
    [[ "$VERBOSE" == "1" ]] && printf '    DEBUG: %s\n' "$1" >&2 || true
}

# ---------------------------------------------------------------------------
# Container exec helpers
# ---------------------------------------------------------------------------
lumerad_exec() {
    timeout 30s docker compose -f "$COMPOSE_FILE" exec -T "$PRIMARY_SERVICE" lumerad "$@"
}

lumerad_exec_service() {
    local service="$1"; shift
    timeout 30s docker compose -f "$COMPOSE_FILE" exec -T "$service" lumerad "$@"
}

lumerad_query() {
    lumerad_exec query "$@" --output json 2>/dev/null
}

lumerad_query_service() {
    local service="$1"; shift
    lumerad_exec_service "$service" query "$@" --output json 2>/dev/null
}

lumerad_tx_service() {
    local service="$1"; shift
    lumerad_exec_service "$service" tx "$@" \
        --chain-id "$CHAIN_ID" \
        --keyring-backend "$KEYRING" \
        --fees "$FEES" \
        --gas "$GAS" \
        --broadcast-mode sync \
        --output json \
        --yes 2>/dev/null
}

tx_code_from_json() {
    local code
    code="$(printf '%s' "$1" | jq -er '.code // 0' 2>/dev/null)" || {
        echo "-1"
        return 0
    }
    echo "$code"
}

is_sequence_mismatch() {
    local code raw_log
    code="$(tx_code_from_json "$1")"
    raw_log="$(echo "$1" | jq -r '.raw_log // empty' 2>/dev/null || echo "")"
    [[ "$code" == "32" ]] && [[ "$raw_log" == *"account sequence mismatch"* ]]
}

# Submit tx with sequence-mismatch retry. Returns final tx JSON via stdout.
run_tx_with_retry() {
    local service="$1"; shift
    local attempt result raw_log expected_seq
    local -a args=("$@")

    for attempt in 1 2 3 4 5; do
        result="$(lumerad_tx_service "$service" "${args[@]}")" || true
        if ! printf '%s' "$result" | jq -e . >/dev/null 2>&1; then
            printf '    WARN: non-JSON tx response on %s attempt %s; retrying\n' "$service" "$attempt" >&2
            sleep 3
            continue
        fi
        if ! is_sequence_mismatch "$result"; then
            echo "$result"
            return 0
        fi
        raw_log="$(echo "$result" | jq -r '.raw_log // empty' 2>/dev/null)"
        expected_seq="$(echo "$raw_log" | grep -oE 'expected [0-9]+' | head -1 | awk '{print $2}')"
        printf '    WARN: seq mismatch on %s attempt %s (expected=%s); retrying\n' "$service" "$attempt" "${expected_seq:-?}" >&2
        if [[ -n "$expected_seq" ]] && [[ "$expected_seq" =~ ^[0-9]+$ ]]; then
            local -a filtered=() i=0
            while (( i < ${#args[@]} )); do
                if [[ "${args[$i]}" == "--sequence" ]]; then
                    i=$((i + 2)); continue
                fi
                if [[ "${args[$i]}" == --sequence=* ]]; then
                    i=$((i + 1)); continue
                fi
                filtered+=("${args[$i]}"); i=$((i + 1))
            done
            args=("${filtered[@]}" "--sequence" "$expected_seq")
        fi
        sleep 3
    done

    echo "$result"
    return 0
}

# Wait for tx to be included AND succeed (code=0). Echo final tx-query JSON.
# Returns 0 on success, 1 on timeout, 2 on inclusion-but-failure (with details).
wait_for_tx() {
    local txhash="$1" deadline result code log_msg attempt
    deadline=$(( $(date +%s) + 30 ))
    for attempt in {1..15}; do
        (( $(date +%s) >= deadline )) && break
        result="$(lumerad_query tx "$txhash" 2>/dev/null || true)"
        if [[ -n "$result" ]] && echo "$result" | jq -e '.txhash' >/dev/null 2>&1; then
            code="$(echo "$result" | jq -r '.code // 0' 2>/dev/null)"
            if [[ "$code" == "0" ]]; then
                echo "$result"
                return 0
            fi
            log_msg="$(echo "$result" | jq -r '.raw_log // empty' 2>/dev/null | head -c 300)"
            # Keep stdout as pure JSON: wait_for_tx is often used inside command
            # substitution and stdout log lines make the caller's jq parsing see
            # an empty raw_log even though the tx response contains the reason.
            printf '    tx %s included but FAILED: code=%s log=%s\n' "$txhash" "$code" "$log_msg" >&2
            echo "$result"
            return 2
        fi
        sleep 2
    done
    return 1
}


# Submit a tx that is expected to be rejected by CheckTx or DeliverTx.
# Returns 0 only when the chain rejects the tx for the expected reason; returns 1
# if the tx is accepted or rejected for an unrelated/missing reason.
expect_tx_rejected_with_retry() {
    local test_name="$1" service="$2" expected_substring="$3"; shift 3
    local result code txhash rc raw_log inclusion

    result="$(run_tx_with_retry "$service" "$@")" || true
    code="$(tx_code_from_json "$result")"
    raw_log="$(echo "$result" | jq -r '.raw_log // empty' 2>/dev/null | head -c 300)"
    if [[ "$code" != "0" ]]; then
        if [[ -n "$expected_substring" && "$raw_log" != *"$expected_substring"* ]]; then
            log "$test_name rejected at CheckTx with unexpected reason: code=$code raw=$raw_log expected~=$expected_substring"
            return 1
        fi
        log "$test_name rejected at CheckTx as expected: code=$code raw=$raw_log"
        return 0
    fi

    txhash="$(echo "$result" | jq -r '.txhash // empty' 2>/dev/null)"
    if [[ -z "$txhash" || "$txhash" == "null" ]]; then
        log "$test_name returned code=0 but no txhash; failing because CheckTx success without a txhash is an unexpected CLI/output issue: raw=$raw_log"
        return 1
    fi

    rc=0
    inclusion="$(wait_for_tx "$txhash")" || rc=$?
    if (( rc == 2 )); then
        raw_log="$(echo "$inclusion" | jq -r '.raw_log // empty' 2>/dev/null | head -c 300)"
        if [[ -n "$expected_substring" && "$raw_log" != *"$expected_substring"* ]]; then
            log "$test_name rejected at DeliverTx with unexpected reason: tx=$txhash raw=$raw_log expected~=$expected_substring"
            return 1
        fi
        log "$test_name rejected at DeliverTx as expected (tx=$txhash raw=$raw_log)"
        return 0
    fi
    if (( rc == 1 )); then
        log "$test_name timed out while waiting for expected rejection (tx=$txhash)"
        return 1
    fi

    log "$test_name unexpectedly accepted (tx=$txhash)"
    return 1
}

# ---------------------------------------------------------------------------
# Supernode discovery: maps validator services to their registered SN identities.
# Builds a global parallel-array map: SN_SERVICES, SN_KEYS, SN_ACCOUNTS, SN_VALADDRS.
# Selection rule: SN key is the supernode-key registered for that validator
# (key name from supernode-setup.sh: "${KEY_NAME/validator/supernode}" or
# "${KEY_NAME}_sn"; we resolve via existence check).
# ---------------------------------------------------------------------------
SN_SERVICES=()
SN_KEYS=()
SN_ACCOUNTS=()
SN_VALADDRS=()

resolve_supernode_key_for_service() {
    local service="$1" candidate
    for candidate in \
        "${service/supernova_validator/supernova_supernode}_key" \
        "${service}_sn_key" \
        "${service}_key_sn" \
        "${service}_key" ; do
        if lumerad_exec_service "$service" keys show "$candidate" -a --keyring-backend "$KEYRING" >/dev/null 2>&1; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

key_address_for_service() {
    local service="$1" key="$2"
    lumerad_exec_service "$service" keys show "$key" -a --keyring-backend "$KEYRING" 2>/dev/null | tr -d '\r\n'
}

key_valoper_for_service() {
    local service="$1" key="$2"
    lumerad_exec_service "$service" keys show "$key" --bech val -a --keyring-backend "$KEYRING" 2>/dev/null | tr -d '\r\n'
}

# Register the validator's own key as a supernode if not already registered.
# Used when supernode-setup.sh hasn't run (supernode binary not in /shared/release/).
# Matches the everlight_test.sh:ensure_supernode_registered_for_service pattern.
ensure_supernode_registered_for_service() {
    local service="$1" idx="$2"
    local key acc val ip tx_result tx_code

    key="$(resolve_supernode_key_for_service "$service")" || return 1
    acc="$(key_address_for_service "$service" "$key")"
    val="$(key_valoper_for_service "$service" "$key")"
    [[ -z "$acc" || -z "$val" ]] && return 1

    # Already registered for this validator?
    if lumerad_query supernode get-supernode "$val" >/dev/null 2>&1; then
        local on_chain
        on_chain="$(lumerad_query supernode get-supernode "$val" | jq -r '.supernode.supernode_account // empty' 2>/dev/null)"
        if [[ -n "$on_chain" ]]; then
            log "  $service: already registered (acc=$on_chain val=$val)"
            return 0
        fi
    fi

    # Mirror devnet/generators/docker-compose.go: validators use 172.28.0.(10+idx).
    ip="172.28.0.$((10 + idx))"
    tx_result="$(run_tx_with_retry "$service" supernode register-supernode \
        "$val" "$ip" "$acc" --p2p-port "4445" \
        --from "$key")" || true
    tx_code="$(tx_code_from_json "$tx_result")"
    if [[ "$tx_code" != "0" ]]; then
        log "  $service: register-supernode failed code=$tx_code raw=$(echo "$tx_result" | jq -r '.raw_log // empty' | head -c 200)"
        return 1
    fi
    local txhash
    txhash="$(echo "$tx_result" | jq -r '.txhash // empty' 2>/dev/null)"
    [[ -n "$txhash" ]] && wait_for_tx "$txhash" >/dev/null
    log "  $service: registered (key=$key acc=$acc val=$val)"
    return 0
}

submit_bootstrap_host_report_for_service() {
    local service="$1" key="$2" acc="$3" epoch_id="$4"

    # Missing-report enforcement only requires the supernode to have submitted
    # a report for the enforcement epoch. During bootstrap there may not be an
    # anchored active set yet, so submit a self host report without peer/storage
    # observations; this mirrors a healthy supernode reporting its own host
    # metrics instead of weakening global devnet genesis postponement params.
    if lumerad_query audit epoch-report "$epoch_id" "$acc" >/dev/null 2>&1; then
        log "  $service: bootstrap host report already exists for epoch $epoch_id"
        return 0
    fi

    local result tx_code txhash host_json rc
    host_json="$(host_report_json "PORT_STATE_OPEN")"
    result="$(run_tx_with_retry "$service" \
        audit submit-epoch-report \
        "$epoch_id" "$host_json" \
        --from "$key")" || true
    tx_code="$(tx_code_from_json "$result")"
    if [[ "$tx_code" != "0" ]]; then
        log "  $service: bootstrap submit-epoch-report failed code=$tx_code raw=$(echo "$result" | jq -r '.raw_log // empty' 2>/dev/null | head -c 200)"
        return 1
    fi
    txhash="$(echo "$result" | jq -r '.txhash // empty' 2>/dev/null)"
    if [[ -z "$txhash" || "$txhash" == "null" ]]; then
        log "  $service: bootstrap submit-epoch-report returned no txhash"
        return 1
    fi
    rc=0
    wait_for_tx "$txhash" >/dev/null || rc=$?
    if (( rc != 0 )); then
        return "$rc"
    fi
    log "  $service: submitted bootstrap host report for epoch $epoch_id"
    return 0
}

submit_bootstrap_host_reports() {
    # Optional positional args: account addresses to skip (e.g. accounts the
    # caller is about to submit a real report for in this same epoch). The
    # underlying per-service helper is already idempotent per (epoch, acc) via
    # an existing audit epoch-report check, so the skip list is a defense-in-
    # depth guard against rare races where the caller has signed but the tx
    # has not yet landed when the sweep runs.
    local -a skip_accs=("$@")
    local service key acc submitted=0 failed=0 epoch_id rc list_json skip
    list_json="$(lumerad_query supernode list-supernodes 2>/dev/null || true)"

    # Pin the epoch ONCE up front. Re-reading per service can land submissions
    # in different epochs when the bootstrap loop straddles an epoch boundary
    # (each tx takes seconds), which then collides with the test's own
    # submit-epoch-report at the new epoch as a duplicate report.
    epoch_id="$(audit_current_epoch_id | tr -dc '0-9')"
    [[ -z "$epoch_id" ]] && return 1

    for service in "${VALIDATOR_SERVICES[@]}"; do
        key="$(resolve_supernode_key_for_service "$service" 2>/dev/null || true)"
        [[ -z "$key" ]] && continue
        acc="$(key_address_for_service "$service" "$key")"
        [[ -z "$acc" ]] && continue
        # Only report for accounts that are actually registered; validator_2 is
        # often not registered on fresh devnet, and attempting to report from it
        # can return non-JSON CLI errors while its validator account is absent.
        if ! echo "$list_json" | jq -e --arg acc "$acc" '.supernodes[]? | select(.supernode_account == $acc)' >/dev/null 2>&1; then
            continue
        fi

        # Honor caller-provided skip list.
        local skipped=0
        for skip in "${skip_accs[@]}"; do
            if [[ "$skip" == "$acc" ]]; then
                skipped=1
                break
            fi
        done
        (( skipped == 1 )) && continue

        log "Bootstrap: submitting host report for $service in epoch $epoch_id before enforcement"
        rc=0
        submit_bootstrap_host_report_for_service "$service" "$key" "$acc" "$epoch_id" || rc=$?
        if (( rc == 2 )); then
            # Epoch may have advanced between signing and DeliverTx; retry once
            # against the chain's current epoch instead of failing bootstrap.
            epoch_id="$(audit_current_epoch_id | tr -dc '0-9')" || true
            if [[ -n "$epoch_id" ]]; then
                log "  $service: retrying bootstrap host report in current epoch $epoch_id"
                rc=0
                submit_bootstrap_host_report_for_service "$service" "$key" "$acc" "$epoch_id" || rc=$?
            fi
        fi
        if (( rc == 0 )); then
            submitted=$((submitted + 1))
        else
            failed=$((failed + 1))
        fi
        sleep 1
    done

    log "Bootstrap host reports submitted: $submitted failed: $failed"
    (( submitted >= 3 ))
}

# Bootstrap registration: if no SNs are registered on chain, register each validator
# as a supernode using its validator key. No-op if SNs already registered (e.g. by
# supernode-setup.sh when the supernode binary is bundled in /shared/release/).
bootstrap_register_supernodes_if_needed() {
    local count
    count="$(lumerad_query supernode list-supernodes | jq '.supernodes | length' 2>/dev/null || echo 0)"
    if (( count >= 3 )); then
        log "Bootstrap skipped: $count supernodes already registered"
        submit_bootstrap_host_reports
        return $?
    fi

    log "Bootstrap: registering validators as supernodes (currently $count registered)"
    local idx=0 service
    for service in "${VALIDATOR_SERVICES[@]}"; do
        idx=$((idx + 1))
        ensure_supernode_registered_for_service "$service" "$idx" || true
        sleep 1
    done

    # Verify
    count="$(lumerad_query supernode list-supernodes | jq '.supernodes | length' 2>/dev/null || echo 0)"
    log "Post-bootstrap supernode count: $count"
    if (( count < 3 )); then
        return 1
    fi
    submit_bootstrap_host_reports
    return $?
}

discover_supernodes() {
    section "Discovering registered supernodes"
    local list_json
    list_json="$(lumerad_query supernode list-supernodes)" || {
        printf 'failed to list supernodes\n' >&2
        return 1
    }
    debug "list-supernodes raw: $list_json"

    local total
    total="$(echo "$list_json" | jq '.supernodes | length' 2>/dev/null || echo 0)"
    log "On-chain registered supernodes: $total"

    local service key acc val sn_acc_on_chain
    for service in "${VALIDATOR_SERVICES[@]}"; do
        key="$(resolve_supernode_key_for_service "$service" 2>/dev/null || true)"
        [[ -z "$key" ]] && continue
        acc="$(key_address_for_service "$service" "$key")"
        val="$(key_valoper_for_service "$service" "$key")"
        [[ -z "$acc" || -z "$val" ]] && continue
        # Confirm this account is actually a registered SN on-chain
        sn_acc_on_chain="$(echo "$list_json" | jq -r --arg a "$acc" '.supernodes[]? | select(.supernode_account == $a) | .supernode_account' | head -1)"
        if [[ -z "$sn_acc_on_chain" ]]; then
            log "  $service: key=$key acc=$acc — not yet registered on chain (skipping)"
            continue
        fi
        SN_SERVICES+=("$service")
        SN_KEYS+=("$key")
        SN_ACCOUNTS+=("$acc")
        SN_VALADDRS+=("$val")
        log "  [${#SN_SERVICES[@]}] $service: key=$key acc=$acc val=$val"
    done

    log "Discovered ${#SN_SERVICES[@]} usable supernode signers"
    if (( ${#SN_SERVICES[@]} < 3 )); then
        printf 'need >=3 registered+key-resolvable supernodes; got %d\n' "${#SN_SERVICES[@]}" >&2
        return 1
    fi
    return 0
}

# ---------------------------------------------------------------------------
# Audit query helpers
# ---------------------------------------------------------------------------
audit_current_epoch_id() {
    # Before the first epoch boundary the query returns start/end heights but no
    # explicit epoch_id; chain enforcement treats that as epoch 0.
    lumerad_query audit current-epoch | jq -r '.epoch_id // "0"'
}

audit_current_epoch_anchor() {
    lumerad_query audit current-epoch-anchor
}

audit_assigned_targets() {
    local supernode_acc="$1" epoch_id="$2"
    lumerad_query audit assigned-targets "$supernode_acc" \
        --epoch-id "$epoch_id" --filter-by-epoch-id 2>/dev/null
}

audit_node_suspicion_state() {
    lumerad_query audit node-suspicion-state "$1" 2>/dev/null
}

audit_ticket_deterioration_state() {
    lumerad_query audit ticket-deterioration-state "$1" 2>/dev/null
}

audit_heal_op() {
    lumerad_query audit heal-op "$1" 2>/dev/null
}

audit_heal_ops_by_ticket() {
    lumerad_query audit heal-ops-by-ticket "$1" 2>/dev/null
}

# Wait for the next epoch boundary. Returns 0 on success, 1 on timeout.
wait_for_next_epoch() {
    local current_epoch deadline now last_report_refresh=0 ts
    current_epoch="$(audit_current_epoch_id | tr -dc '0-9')" || return 1
    [[ -z "$current_epoch" ]] && return 1
    deadline=$(( $(date +%s) + 180 ))
    log "Waiting for next epoch (currently at epoch $current_epoch)..."

    # Keep validator-owned bootstrap supernodes healthy while deliberately
    # crossing epoch boundaries. Without fresh host reports, the audit module can
    # correctly postpone them for missing reports; then assigned-targets becomes
    # empty even with divisor=1 and later tests spin waiting for assignments.
    submit_bootstrap_host_reports >/dev/null 2>&1 || true
    last_report_refresh=$(date +%s)

    while (( $(date +%s) < deadline )); do
        now="$(audit_current_epoch_id 2>/dev/null | tr -dc '0-9' || true)"
        if [[ -n "$now" && "$now" != "$current_epoch" ]]; then
            log "Advanced to epoch $now"
            return 0
        fi
        ts=$(date +%s)
        if (( ts - last_report_refresh >= 20 )); then
            submit_bootstrap_host_reports >/dev/null 2>&1 || true
            last_report_refresh=$ts
        fi
        sleep 2
    done
    log "Timed out waiting for epoch > $current_epoch (last observed ${now:-unknown})"
    return 1
}

# Fresh devnets register supernodes after the chain is already producing blocks;
# the audit module only reflects them in the active epoch anchor after the next
# epoch transition. Wait for that materialization before asserting LEP-6 state.
wait_for_active_supernodes() {
    local want="${1:-3}" deadline count
    deadline=$(( $(date +%s) + 240 ))
    while (( $(date +%s) < deadline )); do
        count="$(audit_current_epoch_anchor | jq -r '.anchor.active_supernode_accounts | length' 2>/dev/null || echo 0)"
        if [[ "$count" =~ ^[0-9]+$ ]] && (( count >= want )); then
            log "Active supernodes materialized in epoch anchor: $count"
            return 0
        fi
        log "Waiting for active supernodes in epoch anchor (have ${count:-0}, want $want)"
        submit_bootstrap_host_reports >/dev/null 2>&1 || true
        sleep 5
    done
    return 1
}

# Find a (prober_idx, target_acc) pair from SN_* arrays where the prober's
# assigned-targets in the given epoch is non-empty. Echos "prober_idx target_acc"
# on stdout. Returns 1 if no valid pair exists.
find_prober_target_pair() {
    local epoch_id="$1"
    local i prober_acc at_json target
    for (( i=0; i<${#SN_ACCOUNTS[@]}; i++ )); do
        prober_acc="${SN_ACCOUNTS[$i]}"
        at_json="$(audit_assigned_targets "$prober_acc" "$epoch_id" 2>/dev/null)" || continue
        target="$(echo "$at_json" | jq -r '.target_supernode_accounts[0] // empty' 2>/dev/null)"
        if [[ -n "$target" ]]; then
            echo "$i $target"
            return 0
        fi
    done
    return 1
}

# Find a prober currently assigned to a specific target in the given epoch.
# Echos "prober_idx target_acc" on stdout. Returns 1 if no local signer has the
# target in its assignment for this epoch.
find_prober_for_target() {
    local epoch_id="$1" wanted_target="$2"
    local i prober_acc at_json
    for (( i=0; i<${#SN_ACCOUNTS[@]}; i++ )); do
        prober_acc="${SN_ACCOUNTS[$i]}"
        [[ "$prober_acc" == "$wanted_target" ]] && continue
        at_json="$(audit_assigned_targets "$prober_acc" "$epoch_id" 2>/dev/null)" || continue
        if echo "$at_json" | jq -e --arg t "$wanted_target" '.target_supernode_accounts[]? | select(. == $t)' >/dev/null 2>&1; then
            echo "$i $wanted_target"
            return 0
        fi
    done
    return 1
}

# Find an index in SN_ACCOUNTS whose account is NOT one of the given accounts.
find_other_signer_idx() {
    local i
    for (( i=0; i<${#SN_ACCOUNTS[@]}; i++ )); do
        local acc="${SN_ACCOUNTS[$i]}"
        local skip=0 a
        for a in "$@"; do
            [[ "$acc" == "$a" ]] && skip=1 && break
        done
        if (( skip == 0 )); then
            echo "$i"
            return 0
        fi
    done
    return 1
}

# Find the local devnet signer index for a chain-assigned supernode account.
find_signer_idx_by_account() {
    local wanted="$1" i
    for (( i=0; i<${#SN_ACCOUNTS[@]}; i++ )); do
        if [[ "${SN_ACCOUNTS[$i]}" == "$wanted" ]]; then
            echo "$i"
            return 0
        fi
    done
    return 1
}

# ---------------------------------------------------------------------------
# Payload builders (chain-validated JSON shapes)
# ---------------------------------------------------------------------------

# Required-open-ports for the genesis params we set: [4444, 4445, 8002].
REQUIRED_PORTS_LEN=3

refresh_required_ports_len() {
    local params_json ports_len
    params_json="$(lumerad_query audit params)" || return 1
    ports_len="$(echo "$params_json" | jq -r '.params.required_open_ports | length' 2>/dev/null || echo 0)"
    if [[ -z "$ports_len" || "$ports_len" == "null" || "$ports_len" == "0" ]]; then
        return 1
    fi
    REQUIRED_PORTS_LEN="$ports_len"
    log "required_open_ports length=$REQUIRED_PORTS_LEN"
    return 0
}

# Build host report JSON (chain expects flat object with cpu/mem/disk/ports/failed_actions_count).
host_report_json() {
    local port_state="${1:-PORT_STATE_OPEN}"
    local i states="["
    for (( i=0; i<REQUIRED_PORTS_LEN; i++ )); do
        [[ $i -gt 0 ]] && states+=","
        states+="\"$port_state\""
    done
    states+="]"
    cat <<EOF
{"cpu_usage_percent":1.0,"mem_usage_percent":1.0,"disk_usage_percent":1.0,"inbound_port_states":${states},"failed_actions_count":0}
EOF
}

# Build storage-challenge-observation JSON for one target (matches required_open_ports length).
sc_observation_json() {
    local target_acc="$1" port_state="${2:-PORT_STATE_OPEN}"
    local i states="["
    for (( i=0; i<REQUIRED_PORTS_LEN; i++ )); do
        [[ $i -gt 0 ]] && states+=","
        states+="\"$port_state\""
    done
    states+="]"
    cat <<EOF
{"target_supernode_account":"${target_acc}","port_states":${states}}
EOF
}

# Build storage-proof-result JSON. Default class is INVALID_TRANSCRIPT (score-neutral
# but recheck-eligible — used to seed the transcript KV store).
proof_result_json() {
    local challenger="$1" target="$2" ticket="$3" transcript_hash="$4"
    local bucket_type="${5:-STORAGE_PROOF_BUCKET_TYPE_RECENT}"
    local result_class="${6:-STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT}"
    cat <<EOF
{"target_supernode_account":"${target}","challenger_supernode_account":"${challenger}","ticket_id":"${ticket}","transcript_hash":"${transcript_hash}","bucket_type":"${bucket_type}","result_class":"${result_class}","artifact_class":"STORAGE_PROOF_ARTIFACT_CLASS_INDEX","artifact_key":"seed-artifact-key","artifact_ordinal":0,"artifact_count":8,"derivation_input_hash":"seed-derivation-hash","challenger_signature":"seed-challenger-signature"}
EOF
}

# ---------------------------------------------------------------------------
# Pre-flight environment checks
# ---------------------------------------------------------------------------
preflight() {
    section "Pre-flight checks"

    if ! command -v jq >/dev/null 2>&1; then
        printf 'jq is required\n' >&2
        return 1
    fi

    if ! docker compose -f "$COMPOSE_FILE" ps "$PRIMARY_SERVICE" >/dev/null 2>&1; then
        printf 'devnet not up: cannot reach %s via %s\n' "$PRIMARY_SERVICE" "$COMPOSE_FILE" >&2
        printf 'Start with: make devnet-up-detach\n' >&2
        return 1
    fi

    refresh_required_ports_len || {
        printf 'audit params missing required_open_ports; cannot build host/peer port reports\n' >&2
        return 1
    }

    local node_status
    node_status="$(lumerad_exec status 2>/dev/null || true)"
    if [[ -z "$node_status" ]]; then
        printf 'lumerad not responsive in %s\n' "$PRIMARY_SERVICE" >&2
        return 1
    fi
    local height
    height="$(echo "$node_status" | jq -r '.sync_info.latest_block_height // .SyncInfo.latest_block_height // "0"' 2>/dev/null)"
    log "Chain height: $height"
    if [[ "$height" == "0" ]]; then
        printf 'chain not progressing\n' >&2
        return 1
    fi
    return 0
}

# ---------------------------------------------------------------------------
# T1 — Params + epoch anchor sanity
# ---------------------------------------------------------------------------
test_lep6_params_and_epoch_anchor() {
    section "T1: TestLEP6_ParamsAndEpochAnchor"

    local params_json
    params_json="$(lumerad_query audit params)" || {
        fail "T1.params" "audit params query failed"; return
    }

    local epoch_len divisor mode heal_threshold
    epoch_len="$(echo "$params_json" | jq -r '.params.epoch_length_blocks // empty')"
    divisor="$(echo "$params_json" | jq -r '.params.storage_truth_challenge_target_divisor // empty')"
    mode="$(echo "$params_json" | jq -r '.params.storage_truth_enforcement_mode // empty')"
    heal_threshold="$(echo "$params_json" | jq -r '.params.storage_truth_ticket_deterioration_heal_threshold // empty')"
    log "epoch_length_blocks=$epoch_len  divisor=$divisor  heal_threshold=$heal_threshold  mode=$mode"

    if [[ "$epoch_len" == "20" ]]; then
        pass "T1.epoch_length_blocks == 20"
    else
        fail "T1.epoch_length_blocks" "expected 20, got '$epoch_len'"
    fi

    if [[ "$divisor" =~ ^[0-9]+$ ]] && (( divisor > 0 )); then
        pass "T1.storage_truth_challenge_target_divisor is positive ($divisor)"
    else
        fail "T1.divisor" "expected positive divisor, got '$divisor'"
    fi

    if [[ "$mode" == "STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW" || "$mode" == "STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT" || "$mode" == "STORAGE_TRUTH_ENFORCEMENT_MODE_HARD" ]]; then
        pass "T1.storage_truth_enforcement_mode is valid ($mode)"
    else
        fail "T1.mode" "expected valid enforcement mode, got '$mode'"
    fi

    if [[ "$heal_threshold" =~ ^[0-9]+$ ]] && (( heal_threshold > 0 )); then
        pass "T1.storage_truth_ticket_deterioration_heal_threshold is positive ($heal_threshold)"
    else
        fail "T1.heal_threshold" "expected positive heal threshold, got '$heal_threshold'"
    fi

    local anchor_json active_count
    anchor_json="$(audit_current_epoch_anchor)" || {
        fail "T1.anchor" "current-epoch-anchor query failed"; return
    }
    active_count="$(echo "$anchor_json" | jq -r '.anchor.active_supernode_accounts | length' 2>/dev/null || echo 0)"
    log "anchor.active_supernode_accounts.length=$active_count"
    if (( active_count >= 3 )); then
        pass "T1.active_supernodes >= 3 ($active_count)"
    else
        fail "T1.active_supernodes" "expected >=3, got $active_count"
    fi
}

# ---------------------------------------------------------------------------
# T2 — SubmitEpochReport happy-path (host report + observations + proof results)
#
# Submits a complete epoch report from SN[0] for the current epoch with
# observations covering all assigned targets, asserts tx success, then queries
# storage-challenge-reports for SN[1] to confirm the report contents indexed.
# ---------------------------------------------------------------------------
test_lep6_submit_epoch_report() {
    section "T2: TestLEP6_SubmitEpochReport_HappyPath"

    # Wait for fresh epoch boundary so the submit slot is guaranteed free.
    if ! wait_for_next_epoch; then
        fail "T2.wait_epoch" "could not advance to next epoch"; return
    fi
    local epoch_id
    epoch_id="$(audit_current_epoch_id)"
    log "Using epoch_id=$epoch_id"

    local pair prober_idx
    pair="$(find_prober_target_pair "$epoch_id")" || {
        fail "T2.targets" "no prober has assigned targets in epoch $epoch_id"; return
    }
    read -r prober_idx _ <<<"$pair"
    local prober_service="${SN_SERVICES[$prober_idx]}"
    local prober_key="${SN_KEYS[$prober_idx]}"
    local prober_acc="${SN_ACCOUNTS[$prober_idx]}"

    # Keep the OTHER bootstrap supernodes healthy in this epoch by submitting
    # host-only reports for them now (the prober submits its own full report
    # below; we skip it to avoid duplicate-report rejection). With
    # consecutive_epochs_to_postpone=1 a single missing report postpones an SN
    # at this epoch's end, which would empty the active set for T3+ and break
    # downstream tests. The per-service helper is idempotent against
    # already-existing (epoch, acc) reports.
    submit_bootstrap_host_reports "$prober_acc" >/dev/null 2>&1 || true
    local at_json targets
    at_json="$(audit_assigned_targets "$prober_acc" "$epoch_id")" || {
        fail "T2.assigned_targets" "assigned-targets query failed"; return
    }
    targets=()
    while IFS= read -r tgt; do
        [[ -n "$tgt" ]] && targets+=("$tgt")
    done < <(echo "$at_json" | jq -r '.target_supernode_accounts[]? // empty')

    log "Assigned targets for prober ${prober_acc}: ${#targets[@]}"
    if (( ${#targets[@]} == 0 )); then
        fail "T2.targets" "prober has no assigned targets in epoch $epoch_id"; return
    fi

    # Build observation flags.
    local -a obs_args=()
    local target
    for target in "${targets[@]}"; do
        obs_args+=("--storage-challenge-observations" "$(sc_observation_json "$target")")
    done

    # Submit complete epoch report.
    local host_json
    host_json="$(host_report_json "PORT_STATE_OPEN")"
    local result tx_code txhash
    result="$(run_tx_with_retry "$prober_service" \
        audit submit-epoch-report \
        "$epoch_id" "$host_json" \
        "${obs_args[@]}" \
        --from "$prober_key")" || true
    tx_code="$(tx_code_from_json "$result")"
    debug "submit-epoch-report tx result: ${result:0:300}"
    if [[ "$tx_code" != "0" ]]; then
        fail "T2.submit" "tx failed code=$tx_code raw_log=$(echo "$result" | jq -r '.raw_log // empty' | head -c 200)"; return
    fi
    txhash="$(echo "$result" | jq -r '.txhash // empty' 2>/dev/null)"
    if [[ -z "$txhash" ]]; then
        fail "T2.txhash" "no txhash in tx result"; return
    fi
    if ! wait_for_tx "$txhash" >/dev/null; then
        fail "T2.tx_inclusion" "tx $txhash not included in block within timeout"; return
    fi
    pass "T2.SubmitEpochReport tx included successfully (epoch=$epoch_id)"

    # Verify the report we just submitted appears in the target's
    # storage-challenge-reports listing for this exact epoch. A plain count
    # check can be satisfied by stale reports from earlier test runs.
    local first_target="${targets[0]}"
    local reports_json matching_report_count
    reports_json="$(lumerad_query audit storage-challenge-reports "$first_target" --epoch-id "$epoch_id" --filter-by-epoch-id)" || {
        fail "T2.scr_query" "storage-challenge-reports query failed"; return
    }
    matching_report_count="$(echo "$reports_json" | jq -r --arg reporter "$prober_acc" --argjson epoch "$epoch_id" '[.reports[]? | select((.reporter_supernode_account // .supernode_account) == $reporter and ((.epoch_id | tonumber) == $epoch))] | length' 2>/dev/null || echo 0)"
    log "storage-challenge-reports for $first_target in epoch $epoch_id from $prober_acc: matching_count=$matching_report_count"
    if (( matching_report_count >= 1 )); then
        pass "T2.storage_challenge_reports indexed prober report"
    else
        # Indexing can lag by a block — tolerate one block.
        sleep 6
        reports_json="$(lumerad_query audit storage-challenge-reports "$first_target" --epoch-id "$epoch_id" --filter-by-epoch-id)" || true
        matching_report_count="$(echo "$reports_json" | jq -r --arg reporter "$prober_acc" --argjson epoch "$epoch_id" '[.reports[]? | select((.reporter_supernode_account // .supernode_account) == $reporter and ((.epoch_id | tonumber) == $epoch))] | length' 2>/dev/null || echo 0)"
        if (( matching_report_count >= 1 )); then
            pass "T2.storage_challenge_reports indexed (after retry)"
        else
            fail "T2.scr_count" "expected report for reporter=$prober_acc epoch=$epoch_id, got matching_count=$matching_report_count"
        fi
    fi
}

# ---------------------------------------------------------------------------
# T3 — SubmitStorageRecheckEvidence updates suspicion and ticket scores
#
# Uses divisor=1 so prober has every target. Picks SN[0]=prober, SN[1]=target,
# SN[2]=rechecker. Seeds transcript record via INVALID_TRANSCRIPT proof result,
# then submits RECHECK_CONFIRMED_FAIL evidence; asserts node suspicion=15 and
# ticket deterioration=8 (LEP-6 spec scoring constants).
# ---------------------------------------------------------------------------
test_lep6_submit_storage_recheck_evidence() {
    section "T3: TestLEP6_SubmitStorageRecheckEvidence_UpdatesSuspicionScore"

    if ! wait_for_next_epoch; then
        fail "T3.wait_epoch" "could not advance to next epoch"; return
    fi
    local epoch_id
    epoch_id="$(audit_current_epoch_id)"
    log "Using epoch_id=$epoch_id"

    # Dynamically find a (prober, target) pair where the chain has assigned a
    # target to the prober. With default divisor/postponement params, some
    # epochs may legitimately have no local assigned pair; keep the default
    # params and advance a bounded number of epochs instead of failing on the
    # first empty assignment window.
    local pair pair_attempt
    for pair_attempt in 1 2 3 4 5; do
        if pair="$(find_prober_target_pair "$epoch_id")"; then
            break
        fi
        log "  no prober has assigned targets in epoch $epoch_id; waiting for another epoch"
        submit_bootstrap_host_reports >/dev/null 2>&1 || true
        if ! wait_for_next_epoch; then
            fail "T3.wait_pair_epoch" "could not advance while looking for assigned targets"; return
        fi
        epoch_id="$(audit_current_epoch_id)"
        log "Using epoch_id=$epoch_id"
    done
    if [[ -z "${pair:-}" ]]; then
        fail "T3.no_pair" "no prober has any assigned targets after $pair_attempt attempts"; return
    fi
    local prober_idx target_acc
    read -r prober_idx target_acc <<<"$pair"

    local prober_service="${SN_SERVICES[$prober_idx]}"
    local prober_key="${SN_KEYS[$prober_idx]}"
    local prober_acc="${SN_ACCOUNTS[$prober_idx]}"

    # Pick rechecker: any signer ≠ prober ≠ target.
    local rechecker_idx
    rechecker_idx="$(find_other_signer_idx "$prober_acc" "$target_acc")" || {
        fail "T3.no_rechecker" "could not find a rechecker distinct from prober/target"; return
    }
    local rechecker_service="${SN_SERVICES[$rechecker_idx]}"
    local rechecker_key="${SN_KEYS[$rechecker_idx]}"
    local rechecker_acc="${SN_ACCOUNTS[$rechecker_idx]}"

    # Keep the OTHER bootstrap SNs healthy this epoch (prober submits its own
    # full report below; skip it to avoid duplicate-report rejection).
    submit_bootstrap_host_reports "$prober_acc" >/dev/null 2>&1 || true

    local ticket_id="lep6-devnet-recheck-ticket-${epoch_id}"
    local old_hash="lep6-devnet-old-transcript-${epoch_id}"
    local recheck_hash="lep6-devnet-recheck-transcript-${epoch_id}"

    log "prober=$prober_acc(idx=$prober_idx) target=$target_acc rechecker=$rechecker_acc(idx=$rechecker_idx) ticket=$ticket_id"

    # Read pre-recheck score (NotFound treated as 0).
    local pre_state pre_score
    pre_state="$(audit_node_suspicion_state "$target_acc" 2>/dev/null || true)"
    pre_score="$(echo "$pre_state" | jq -r '.state.suspicion_score // "0"' 2>/dev/null || echo "0")"
    [[ -z "$pre_score" || "$pre_score" == "null" ]] && pre_score="0"
    log "Pre-recheck target suspicion_score=$pre_score"

    # Step 1: prober submits epoch report with INVALID_TRANSCRIPT proof result for target.
    # Must include peer observations covering all assigned targets.
    local at_json
    at_json="$(audit_assigned_targets "$prober_acc" "$epoch_id")" || {
        fail "T3.at_query" "assigned-targets failed"; return
    }
    local -a obs_args=()
    local tgt
    while IFS= read -r tgt; do
        [[ -n "$tgt" ]] && obs_args+=("--storage-challenge-observations" "$(sc_observation_json "$tgt")")
    done < <(echo "$at_json" | jq -r '.target_supernode_accounts[]? // empty')

    if (( ${#obs_args[@]} == 0 )); then
        fail "T3.no_targets" "prober has no assigned targets"; return
    fi

    local pr_json
    pr_json="$(proof_result_json "$prober_acc" "$target_acc" "$ticket_id" "$old_hash" \
        "STORAGE_PROOF_BUCKET_TYPE_RECENT" "STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT")"
    local host_json
    host_json="$(host_report_json "PORT_STATE_OPEN")"

    local seed_result
    seed_result="$(run_tx_with_retry "$prober_service" \
        audit submit-epoch-report \
        "$epoch_id" "$host_json" \
        "${obs_args[@]}" \
        --storage-proof-results "$pr_json" \
        --from "$prober_key")" || true
    local seed_code
    seed_code="$(tx_code_from_json "$seed_result")"
    if [[ "$seed_code" != "0" ]]; then
        fail "T3.seed_check" "seed CheckTx failed code=$seed_code raw=$(echo "$seed_result" | jq -r '.raw_log // empty' | head -c 200)"
        return
    fi
    local seed_txhash
    seed_txhash="$(echo "$seed_result" | jq -r '.txhash // empty' 2>/dev/null)"
    local seed_inclusion
    seed_inclusion=$(wait_for_tx "$seed_txhash" >/dev/null; echo $?)
    if (( seed_inclusion != 0 )); then
        fail "T3.seed_deliver" "seed DeliverTx failed (rc=$seed_inclusion); the prober/target pair was rejected by the chain"
        return
    fi
    pass "T3.seed proof transcript via INVALID_TRANSCRIPT submitted (epoch=$epoch_id)"

    # Step 2: rechecker submits RECHECK_CONFIRMED_FAIL.
    local recheck_result recheck_code
    recheck_result="$(run_tx_with_retry "$rechecker_service" \
        audit submit-storage-recheck-evidence \
        "$epoch_id" "$target_acc" "$ticket_id" \
        --challenged-result-transcript-hash "$old_hash" \
        --recheck-transcript-hash "$recheck_hash" \
        --recheck-result-class recheck-confirmed-fail \
        --from "$rechecker_key")" || true
    recheck_code="$(tx_code_from_json "$recheck_result")"
    if [[ "$recheck_code" != "0" ]]; then
        fail "T3.recheck_check" "recheck CheckTx failed code=$recheck_code raw=$(echo "$recheck_result" | jq -r '.raw_log // empty' | head -c 200)"
        return
    fi
    local recheck_txhash
    recheck_txhash="$(echo "$recheck_result" | jq -r '.txhash // empty' 2>/dev/null)"
    local recheck_inclusion
    recheck_inclusion=$(wait_for_tx "$recheck_txhash" >/dev/null; echo $?)
    if (( recheck_inclusion != 0 )); then
        fail "T3.recheck_deliver" "recheck DeliverTx failed (rc=$recheck_inclusion)"
        return
    fi
    pass "T3.SubmitStorageRecheckEvidence tx included successfully"

    # Step 3: assert node suspicion delta == +15.
    sleep 4
    local post_state post_score
    post_state="$(audit_node_suspicion_state "$target_acc")" || true
    post_score="$(echo "$post_state" | jq -r '.state.suspicion_score // "0"' 2>/dev/null || echo "0")"
    [[ -z "$post_score" || "$post_score" == "null" ]] && post_score="0"
    local delta=$((post_score - pre_score))
    log "Post-recheck target suspicion_score=$post_score (delta=$delta)"
    if (( delta == 15 )); then
        pass "T3.node_suspicion delta == +15 (LEP-6 recheck penalty)"
    else
        fail "T3.suspicion_delta" "expected exactly +15, got delta=$delta (pre=$pre_score post=$post_score)"
    fi

    # Step 4: assert ticket deterioration == 8.
    local ticket_state ticket_score
    ticket_state="$(audit_ticket_deterioration_state "$ticket_id")" || true
    ticket_score="$(echo "$ticket_state" | jq -r '.state.deterioration_score // "0"' 2>/dev/null || echo "0")"
    [[ -z "$ticket_score" || "$ticket_score" == "null" ]] && ticket_score="0"
    log "ticket_id=$ticket_id deterioration_score=$ticket_score"
    if [[ "$ticket_score" == "8" ]]; then
        pass "T3.ticket_deterioration == 8 (LEP-6 spec)"
    else
        fail "T3.ticket_score" "expected 8, got $ticket_score"
    fi
}


# ---------------------------------------------------------------------------
# T5 — Recheck evidence authorization rejects original reporter / target submitters
#
# Seeds a valid proof transcript, then proves the chain rejects recheck evidence
# when the submitter is not independent from the original report or challenged SN.
# ---------------------------------------------------------------------------
test_lep6_recheck_rejects_unauthorized_submitter() {
    section "T5: TestLEP6_RecheckEvidenceRejectsUnauthorizedSubmitter"

    if ! wait_for_next_epoch; then
        fail "T5.wait_epoch" "could not advance to next epoch"; return
    fi
    local epoch_id
    epoch_id="$(audit_current_epoch_id)"
    log "Using epoch_id=$epoch_id"

    local pair pair_attempt
    for pair_attempt in 1 2 3 4 5; do
        if pair="$(find_prober_target_pair "$epoch_id")"; then
            break
        fi
        log "  no prober has assigned targets in epoch $epoch_id; waiting for another epoch"
        submit_bootstrap_host_reports >/dev/null 2>&1 || true
        if ! wait_for_next_epoch; then
            fail "T5.wait_pair_epoch" "could not advance while looking for assigned targets"; return
        fi
        epoch_id="$(audit_current_epoch_id)"
        log "Using epoch_id=$epoch_id"
    done
    if [[ -z "${pair:-}" ]]; then
        fail "T5.no_pair" "no prober has any assigned targets after $pair_attempt attempts"; return
    fi
    local prober_idx target_acc
    read -r prober_idx target_acc <<<"$pair"

    local prober_service="${SN_SERVICES[$prober_idx]}"
    local prober_key="${SN_KEYS[$prober_idx]}"
    local prober_acc="${SN_ACCOUNTS[$prober_idx]}"

    local target_idx=-1 i
    for (( i=0; i<${#SN_ACCOUNTS[@]}; i++ )); do
        if [[ "${SN_ACCOUNTS[$i]}" == "$target_acc" ]]; then
            target_idx=$i
            break
        fi
    done
    if (( target_idx < 0 )); then
        fail "T5.target_key" "target $target_acc is not key-resolvable in this devnet"; return
    fi
    local target_service="${SN_SERVICES[$target_idx]}"
    local target_key="${SN_KEYS[$target_idx]}"

    # Keep OTHER bootstrap SNs healthy this epoch (prober submits below).
    submit_bootstrap_host_reports "$prober_acc" >/dev/null 2>&1 || true

    local ticket_id="lep6-devnet-unauth-recheck-ticket-${epoch_id}"
    local old_hash="lep6-devnet-unauth-old-${epoch_id}"
    local recheck_hash="lep6-devnet-unauth-recheck-${epoch_id}"

    local at_json
    at_json="$(audit_assigned_targets "$prober_acc" "$epoch_id")" || {
        fail "T5.at_query" "assigned-targets failed"; return
    }
    local -a obs_args=()
    local tgt
    while IFS= read -r tgt; do
        [[ -n "$tgt" ]] && obs_args+=("--storage-challenge-observations" "$(sc_observation_json "$tgt")")
    done < <(echo "$at_json" | jq -r '.target_supernode_accounts[]? // empty')
    if (( ${#obs_args[@]} == 0 )); then
        fail "T5.no_targets" "prober has no assigned targets"; return
    fi

    local pr_json host_json seed_result seed_code seed_tx seed_rc
    pr_json="$(proof_result_json "$prober_acc" "$target_acc" "$ticket_id" "$old_hash" \
        "STORAGE_PROOF_BUCKET_TYPE_RECENT" "STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT")"
    host_json="$(host_report_json "PORT_STATE_OPEN")"
    seed_result="$(run_tx_with_retry "$prober_service" \
        audit submit-epoch-report \
        "$epoch_id" "$host_json" \
        "${obs_args[@]}" \
        --storage-proof-results "$pr_json" \
        --from "$prober_key")" || true
    seed_code="$(tx_code_from_json "$seed_result")"
    if [[ "$seed_code" != "0" ]]; then
        fail "T5.seed_check" "seed CheckTx failed code=$seed_code raw=$(echo "$seed_result" | jq -r '.raw_log // empty' | head -c 200)"
        return
    fi
    seed_tx="$(echo "$seed_result" | jq -r '.txhash // empty' 2>/dev/null)"
    seed_rc=$(wait_for_tx "$seed_tx" >/dev/null; echo $?)
    if (( seed_rc != 0 )); then
        fail "T5.seed_deliver" "seed DeliverTx failed (rc=$seed_rc)"
        return
    fi
    pass "T5.seed proof transcript submitted"

    if expect_tx_rejected_with_retry "T5.original_reporter_recheck" "$prober_service" "creator must be independent from the challenged result reporter" \
        audit submit-storage-recheck-evidence \
        "$epoch_id" "$target_acc" "$ticket_id" \
        --challenged-result-transcript-hash "$old_hash" \
        --recheck-transcript-hash "$recheck_hash-prober" \
        --recheck-result-class recheck-confirmed-fail \
        --from "$prober_key"; then
        pass "T5.original reporter cannot submit recheck evidence"
    else
        fail "T5.original_reporter_recheck" "original reporter recheck evidence was accepted"
    fi

    if expect_tx_rejected_with_retry "T5.challenged_target_recheck" "$target_service" "challenged_supernode_account must not equal creator" \
        audit submit-storage-recheck-evidence \
        "$epoch_id" "$target_acc" "$ticket_id" \
        --challenged-result-transcript-hash "$old_hash" \
        --recheck-transcript-hash "$recheck_hash-target" \
        --recheck-result-class recheck-confirmed-fail \
        --from "$target_key"; then
        pass "T5.challenged target cannot submit recheck evidence"
    else
        fail "T5.challenged_target_recheck" "challenged target recheck evidence was accepted"
    fi
}

# ---------------------------------------------------------------------------
# T6 — ClaimHealComplete rejects a nonexistent heal op
#
# Proves the chain does not accept a healer claim for an op id that was never
# scheduled by EndBlock.
# ---------------------------------------------------------------------------
test_lep6_claim_rejects_nonexistent_op() {
    section "T6: TestLEP6_ClaimHealCompleteRejectsNonexistentOp"

    local service="${SN_SERVICES[0]}"
    local key="${SN_KEYS[0]}"
    local fake_op_id="999999999"
    local fake_ticket="lep6-devnet-nonexistent-heal-ticket-$(date +%s)"
    local fake_manifest="lep6-devnet-nonexistent-manifest-${fake_op_id}"

    if expect_tx_rejected_with_retry "T6.nonexistent_heal_claim" "$service" "not found" \
        audit claim-heal-complete \
        "$fake_op_id" "$fake_ticket" "$fake_manifest" \
        --from "$key"; then
        pass "T6.nonexistent heal-op claim rejected"
    else
        fail "T6.nonexistent_heal_claim" "claim for nonexistent heal op was accepted"
    fi
}

# ---------------------------------------------------------------------------
# T4 — Heal-op lifecycle: deterioration threshold → claim → verify (x2)
#
# The chain creates a heal op when ticket deterioration crosses the threshold
# and the scheduleStorageTruthHealOpsAtEpochEnd eligibility predicate is true
# (holder diversity, index failure, or repeated recent failures). The loop below
# uses the live chain threshold and keeps one ticket fixed until both threshold
# and eligibility are satisfied.
#
# If heal-op creation cannot be observed within HEAL_OP_TIMEOUT_SEC, the test
# fails with an actionable diagnostic so the orchestrator can decide whether
# to lower heal_threshold in genesis.
#
# Once the heal op exists: pick a healer ≠ target; submit ClaimHealComplete; then
# pick verifier_count distinct verifiers ≠ healer ≠ target; submit
# SubmitHealVerification(verified=true) from each; assert HealOp transitions
# to the terminal successful HEAL_OP_STATUS_VERIFIED state.
# ---------------------------------------------------------------------------
HEAL_OP_TIMEOUT_SEC=600

test_lep6_heal_op_lifecycle() {
    section "T4: TestLEP6_HealOpLifecycle_ClaimVerifyFinalize"

    local ticket_id="lep6-devnet-heal-ticket-$(date +%s)"
    local target_acc=""
    log "ticket=$ticket_id"

    local params_json heal_threshold
    params_json="$(lumerad_query audit params)" || {
        fail "T4.params" "audit params query failed"; return
    }
    heal_threshold="$(echo "$params_json" | jq -r '.params.storage_truth_ticket_deterioration_heal_threshold // "0"' 2>/dev/null || echo 0)"
    [[ -z "$heal_threshold" || "$heal_threshold" == "null" ]] && heal_threshold=0
    if (( heal_threshold <= 0 )); then
        fail "T4.heal_threshold_param" "invalid heal threshold '$heal_threshold'"
        return
    fi

    # Drive deterioration to the live chain threshold and scheduling eligibility.
    # Each successful RECHECK_CONFIRMED_FAIL adds +8. Use the same ticket and
    # target across attempts; each epoch, discover a prober that is currently
    # assigned to that fixed target. This models a single deteriorating ticket on
    # one holder without weakening default genesis params.
    local max_attempts=16 attempt=0 cur_ticket_score=0 successful_rechecks=0 heal_eligible=0
    local recent_failure_count=0 distinct_holder_failure_count=0 last_index_failure_epoch=0
    while (( attempt < max_attempts )); do
        if (( cur_ticket_score >= heal_threshold && heal_eligible == 1 )); then
            break
        fi
        attempt=$((attempt + 1))

        if ! wait_for_next_epoch; then
            fail "T4.advance_epoch" "could not advance epoch on attempt $attempt"; return
        fi
        local epoch_id
        epoch_id="$(audit_current_epoch_id)"

        local pair prober_idx_match prober_acc at_json
        if [[ -z "$target_acc" ]]; then
            pair="$(find_prober_target_pair "$epoch_id")" || {
                log "  [attempt $attempt] no assigned prober/target pair this epoch; skipping epoch"
                continue
            }
            read -r prober_idx_match target_acc <<<"$pair"
            log "  [attempt $attempt] selected fixed heal target=$target_acc"
        else
            pair="$(find_prober_for_target "$epoch_id" "$target_acc")" || {
                log "  [attempt $attempt] no prober assigned to fixed target=$target_acc in epoch $epoch_id; skipping epoch"
                continue
            }
            read -r prober_idx_match _ <<<"$pair"
        fi
        prober_acc="${SN_ACCOUNTS[$prober_idx_match]}"
        at_json="$(audit_assigned_targets "$prober_acc" "$epoch_id" 2>/dev/null)" || {
            log "  [attempt $attempt] assigned-target query failed for prober=$prober_acc; skipping epoch"
            continue
        }

        local prober_service="${SN_SERVICES[$prober_idx_match]}"
        local prober_key="${SN_KEYS[$prober_idx_match]}"
        # Pick rechecker ≠ prober ≠ target.
        local rk
        rk="$(find_other_signer_idx "$prober_acc" "$target_acc")" || continue
        local rechecker_service="${SN_SERVICES[$rk]}"
        local rechecker_key="${SN_KEYS[$rk]}"
        local rechecker_acc="${SN_ACCOUNTS[$rk]}"

        # Keep OTHER bootstrap SNs healthy this epoch (prober submits below).
        submit_bootstrap_host_reports "$prober_acc" >/dev/null 2>&1 || true

        local old_hash="heal-old-${epoch_id}-${attempt}"
        local recheck_hash="heal-recheck-${epoch_id}-${attempt}"

        log "[attempt $attempt] epoch=$epoch_id prober=$prober_acc target=$target_acc rechecker=$rechecker_acc"

        # Build observations covering all assigned targets.
        local -a obs_args=() tgt
        while IFS= read -r tgt; do
            [[ -n "$tgt" ]] && obs_args+=("--storage-challenge-observations" "$(sc_observation_json "$tgt")")
        done < <(echo "$at_json" | jq -r '.target_supernode_accounts[]? // empty')

        local pr_json
        pr_json="$(proof_result_json "$prober_acc" "$target_acc" "$ticket_id" "$old_hash" \
            "STORAGE_PROOF_BUCKET_TYPE_RECENT" "STORAGE_PROOF_RESULT_CLASS_INVALID_TRANSCRIPT")"
        local host_json
        host_json="$(host_report_json "PORT_STATE_OPEN")"

        # Seed.
        local seed_result seed_code seed_tx seed_rc
        seed_result="$(run_tx_with_retry "$prober_service" \
            audit submit-epoch-report \
            "$epoch_id" "$host_json" \
            "${obs_args[@]}" \
            --storage-proof-results "$pr_json" \
            --from "$prober_key")" || true
        seed_code="$(tx_code_from_json "$seed_result")"
        if [[ "$seed_code" != "0" ]]; then
            log "  seed CheckTx code=$seed_code raw=$(echo "$seed_result" | jq -r '.raw_log // empty' | head -c 100)"
            continue
        fi
        seed_tx="$(echo "$seed_result" | jq -r '.txhash // empty' 2>/dev/null)"
        seed_rc=$(wait_for_tx "$seed_tx" >/dev/null; echo $?)
        if (( seed_rc != 0 )); then
            log "  seed DeliverTx failed (rc=$seed_rc) — skipping this attempt"
            continue
        fi

        # Recheck.
        local rr_result rr_code rr_tx rr_rc
        rr_result="$(run_tx_with_retry "$rechecker_service" \
            audit submit-storage-recheck-evidence \
            "$epoch_id" "$target_acc" "$ticket_id" \
            --challenged-result-transcript-hash "$old_hash" \
            --recheck-transcript-hash "$recheck_hash" \
            --recheck-result-class recheck-confirmed-fail \
            --from "$rechecker_key")" || true
        rr_code="$(tx_code_from_json "$rr_result")"
        if [[ "$rr_code" != "0" ]]; then
            log "  recheck CheckTx code=$rr_code raw=$(echo "$rr_result" | jq -r '.raw_log // empty' | head -c 100)"
            continue
        fi
        rr_tx="$(echo "$rr_result" | jq -r '.txhash // empty' 2>/dev/null)"
        rr_rc=$(wait_for_tx "$rr_tx" >/dev/null; echo $?)
        if (( rr_rc != 0 )); then
            log "  recheck DeliverTx failed (rc=$rr_rc)"
            continue
        fi
        successful_rechecks=$((successful_rechecks + 1))

        sleep 3
        local ticket_state_json
        ticket_state_json="$(audit_ticket_deterioration_state "$ticket_id" 2>/dev/null || true)"
        cur_ticket_score="$(echo "$ticket_state_json" | jq -r '.state.deterioration_score // "0"' 2>/dev/null || echo 0)"
        recent_failure_count="$(echo "$ticket_state_json" | jq -r '.state.recent_failure_epoch_count // 0' 2>/dev/null || echo 0)"
        distinct_holder_failure_count="$(echo "$ticket_state_json" | jq -r '.state.distinct_holder_failure_count // 0' 2>/dev/null || echo 0)"
        last_index_failure_epoch="$(echo "$ticket_state_json" | jq -r '.state.last_index_failure_epoch // "0"' 2>/dev/null || echo 0)"
        [[ -z "$cur_ticket_score" || "$cur_ticket_score" == "null" ]] && cur_ticket_score=0
        [[ -z "$recent_failure_count" || "$recent_failure_count" == "null" ]] && recent_failure_count=0
        [[ -z "$distinct_holder_failure_count" || "$distinct_holder_failure_count" == "null" ]] && distinct_holder_failure_count=0
        [[ -z "$last_index_failure_epoch" || "$last_index_failure_epoch" == "null" ]] && last_index_failure_epoch=0
        if (( distinct_holder_failure_count >= 2 || last_index_failure_epoch > 0 || recent_failure_count >= 2 )); then
            heal_eligible=1
        else
            heal_eligible=0
        fi
        log "  ticket state: deterioration=$cur_ticket_score recent_failures=$recent_failure_count distinct_holders=$distinct_holder_failure_count last_index_failure_epoch=$last_index_failure_epoch eligible=$heal_eligible (rechecks_successful=$successful_rechecks)"
        if (( cur_ticket_score >= heal_threshold && heal_eligible == 1 )); then
            log "Heal threshold and scheduling eligibility reached (threshold=$heal_threshold)"
            break
        fi
    done

    if (( cur_ticket_score < heal_threshold )); then
        fail "T4.threshold" "ticket deterioration $cur_ticket_score < $heal_threshold after $attempt attempts ($successful_rechecks successful rechecks); heal op cannot be created"
        return
    fi
    pass "T4.deterioration reached heal threshold ($cur_ticket_score >= $heal_threshold)"
    if (( heal_eligible != 1 )); then
        fail "T4.heal_eligibility" "ticket reached threshold but is not scheduling-eligible after $attempt attempts: recent_failures=$recent_failure_count distinct_holders=$distinct_holder_failure_count last_index_failure_epoch=$last_index_failure_epoch"
        return
    fi
    pass "T4.ticket scheduling eligibility reached (recent_failures=$recent_failure_count distinct_holders=$distinct_holder_failure_count last_index_failure_epoch=$last_index_failure_epoch)"

    # Wait for EndBlock to schedule the heal op.
    local heal_op_id="" heal_json deadline
    deadline=$(( $(date +%s) + HEAL_OP_TIMEOUT_SEC ))
    while (( $(date +%s) < deadline )); do
        wait_for_next_epoch || true
        # Keep all bootstrap SNs healthy while we wait for scheduling.
        submit_bootstrap_host_reports >/dev/null 2>&1 || true
        heal_json="$(audit_heal_ops_by_ticket "$ticket_id" 2>/dev/null || true)"
        heal_op_id="$(echo "$heal_json" | jq -r '.heal_ops[0].heal_op_id // empty' 2>/dev/null)"
        if [[ -n "$heal_op_id" && "$heal_op_id" != "null" ]]; then
            log "Heal op scheduled: heal_op_id=$heal_op_id"
            break
        fi
        sleep 5
    done

    if [[ -z "$heal_op_id" || "$heal_op_id" == "null" ]]; then
        fail "T4.heal_op_creation" "no heal op created for ticket $ticket_id within ${HEAL_OP_TIMEOUT_SEC}s"
        return
    fi
    pass "T4.heal_op created (id=$heal_op_id)"

    # Use the chain-assigned healer/verifiers from the scheduled heal op. ClaimHealComplete
    # is authorized only for heal_op.healer_supernode_account; using any non-target
    # signer makes DeliverTx fail with ErrHealOpUnauthorized.
    heal_json="$(audit_heal_op "$heal_op_id")" || { fail "T4.heal_op_query" "heal-op query failed"; return; }
    local healer_acc healer_idx
    healer_acc="$(echo "$heal_json" | jq -r '.heal_op.healer_supernode_account // empty')"
    if [[ -z "$healer_acc" || "$healer_acc" == "null" ]]; then
        fail "T4.no_assigned_healer" "scheduled heal op has no healer account"
        return
    fi
    healer_idx="$(find_signer_idx_by_account "$healer_acc")" || {
        fail "T4.no_healer_key" "assigned healer $healer_acc not found in devnet signer set"; return
    }
    local healer_service="${SN_SERVICES[$healer_idx]}"
    local healer_key="${SN_KEYS[$healer_idx]}"
    local manifest_hash="lep6-heal-manifest-${heal_op_id}"
    log "Using assigned healer idx=$healer_idx account=$healer_acc"

    # Submit ClaimHealComplete.
    local claim_result claim_code claim_tx claim_rc
    claim_result="$(run_tx_with_retry "$healer_service" \
        audit claim-heal-complete \
        "$heal_op_id" "$ticket_id" "$manifest_hash" \
        --from "$healer_key")" || true
    claim_code="$(tx_code_from_json "$claim_result")"
    if [[ "$claim_code" != "0" ]]; then
        fail "T4.claim_check" "claim-heal-complete CheckTx failed code=$claim_code raw=$(echo "$claim_result" | jq -r '.raw_log // empty' | head -c 200)"
        return
    fi
    claim_tx="$(echo "$claim_result" | jq -r '.txhash // empty' 2>/dev/null)"
    claim_rc=$(wait_for_tx "$claim_tx" >/dev/null; echo $?)
    if (( claim_rc != 0 )); then
        fail "T4.claim_deliver" "claim DeliverTx failed (rc=$claim_rc)"
        return
    fi
    pass "T4.ClaimHealComplete tx included successfully"

    # Submit verifications from the chain-assigned verifier set.
    local verifications_needed verifications_done=0 verifier_pos
    verifications_needed="$(echo "$heal_json" | jq -r '.heal_op.verifier_supernode_accounts | length' 2>/dev/null || echo 0)"
    [[ -z "$verifications_needed" || "$verifications_needed" == "null" ]] && verifications_needed=0
    if (( verifications_needed <= 0 )); then
        fail "T4.no_assigned_verifiers" "scheduled heal op has no verifier accounts"
        return
    fi
    for (( verifier_pos=0; verifier_pos<verifications_needed; verifier_pos++ )); do
        local v_acc verifier_idx
        v_acc="$(echo "$heal_json" | jq -r --argjson i "$verifier_pos" '.heal_op.verifier_supernode_accounts[$i] // empty')"
        verifier_idx="$(find_signer_idx_by_account "$v_acc")" || {
            fail "T4.no_verifier_key" "assigned verifier $v_acc not found in devnet signer set"; return
        }
        local v_service="${SN_SERVICES[$verifier_idx]}"
        local v_key="${SN_KEYS[$verifier_idx]}"
        local verif_result verif_code verif_tx verif_rc
        verif_result="$(run_tx_with_retry "$v_service" \
            audit submit-heal-verification \
            "$heal_op_id" true "$manifest_hash" \
            --from "$v_key")" || true
        verif_code="$(tx_code_from_json "$verif_result")"
        if [[ "$verif_code" != "0" ]]; then
            log "  verifier idx=$verifier_idx CheckTx failed code=$verif_code raw=$(echo "$verif_result" | jq -r '.raw_log // empty' | head -c 150)"
            continue
        fi
        verif_tx="$(echo "$verif_result" | jq -r '.txhash // empty' 2>/dev/null)"
        verif_rc=$(wait_for_tx "$verif_tx" >/dev/null; echo $?)
        if (( verif_rc != 0 )); then
            log "  verifier idx=$verifier_idx DeliverTx failed (rc=$verif_rc)"
            continue
        fi
        verifications_done=$((verifications_done + 1))
        log "  verification #$verifications_done from idx=$verifier_idx accepted"

        if (( verifications_done == 1 )); then
            if expect_tx_rejected_with_retry "T7.duplicate_heal_verification" "$v_service" "verification already submitted by creator" \
                audit submit-heal-verification \
                "$heal_op_id" true "$manifest_hash" \
                --from "$v_key"; then
                pass "T7.duplicate heal verification rejected"
            else
                fail "T7.duplicate_heal_verification" "duplicate verifier vote was accepted"
                return
            fi
        fi

        if (( verifications_done >= verifications_needed )); then
            break
        fi
    done

    if (( verifications_done < verifications_needed )); then
        fail "T4.verifications" "got $verifications_done/$verifications_needed verifications"
        return
    fi
    pass "T4.SubmitHealVerification x$verifications_done accepted"

    # Wait for status to transition.
    sleep 4
    local final_json final_status
    final_json="$(audit_heal_op "$heal_op_id")" || { fail "T4.final_query" "heal-op query failed"; return; }
    final_status="$(echo "$final_json" | jq -r '.heal_op.status // empty')"
    log "Final heal_op status: $final_status"
    if [[ "$final_status" == "HEAL_OP_STATUS_VERIFIED" ]]; then
        pass "T4.HealOp reached terminal verified status"
    else
        fail "T4.final_status" "expected HEAL_OP_STATUS_VERIFIED after assigned verifier votes, got '$final_status'"
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    section "LEP-6 Storage-Truth Devnet Tests"
    log "COMPOSE_FILE=$COMPOSE_FILE"
    log "CHAIN_ID=$CHAIN_ID"
    log "PRIMARY_SERVICE=$PRIMARY_SERVICE"

    preflight || exit 1
    bootstrap_register_supernodes_if_needed || {
        printf 'failed to bootstrap supernode registrations\n' >&2
        exit 1
    }
    wait_for_active_supernodes 3 || {
        printf 'registered supernodes did not become active in the epoch anchor\n' >&2
        exit 1
    }
    discover_supernodes || exit 1

    test_lep6_params_and_epoch_anchor
    test_lep6_submit_epoch_report
    test_lep6_submit_storage_recheck_evidence
    test_lep6_recheck_rejects_unauthorized_submitter
    test_lep6_claim_rejects_nonexistent_op
    test_lep6_heal_op_lifecycle

    section "Summary"
    printf 'PASS=%d  FAIL=%d  SKIP=%d\n' "$PASS_COUNT" "$FAIL_COUNT" "$SKIP_COUNT"
    for r in "${RESULTS[@]}"; do
        printf '  %s\n' "$r"
    done
    (( FAIL_COUNT > 0 )) && exit 1
    exit 0
}

main "$@"
