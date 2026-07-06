#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

DEVNET_CHAIN_ID="lumera-devnet-1"
DEVNET_RPC="https://rpc.pastel.network"
DEVNET_GRPC="grpc.pastel.network"

TESTNET_CHAIN_ID="lumera-testnet-2"
#TESTNET_RPC="https://lumera-testnet-rpc.polkachu.com:443"
TESTNET_RPC="https://rpc.testnet.lumera.io:443"
TESTNET_GRPC="grpc.testnet.lumera.io:443"

MAINNET_CHAIN_ID="lumera-mainnet-1"
MAINNET_RPC="https://rpc.lumera.io:443"
MAINNET_GRPC="grpc.lumera.io:443"

DEFAULT_CHAIN_ID="$TESTNET_CHAIN_ID"
DEFAULT_RPC="$TESTNET_RPC"
DEFAULT_GRPC="$TESTNET_GRPC"
DEFAULT_CURRENT_CAP="2500"

CHAIN_ID="${LUMERA_CHAIN_ID:-$DEFAULT_CHAIN_ID}"
NODE="${LUMERA_NODE:-${LUMERA_RPC:-$DEFAULT_RPC}}"
GRPC="${LUMERA_GRPC:-$DEFAULT_GRPC}"
BIN="${LUMERA_BINARY:-lumerad}"
GRPCURL="${LUMERA_GRPCURL_BINARY:-grpcurl}"
LIMIT="${LUMERA_QUERY_LIMIT:-1000}"
BUFFER_PERCENT="${LUMERA_BUFFER_PERCENT:-30}"
JSON_OUTPUT=0
ALLOW_PARTIAL=0
GRPC_INSECURE="${LUMERA_GRPC_INSECURE:-0}"
GRPC_MAX_TIME="${LUMERA_GRPC_MAX_TIME:-30}"

# These track whether CHAIN_ID/NODE/GRPC were explicitly provided, so that
# apply_network_defaults() only fills endpoints the caller left unset. A value
# supplied via LUMERA_* env vars (seeded above) counts as explicit: env vars are
# documented as "overrides", so passing --network alongside them must NOT clobber
# them. An explicit --node/--grpc/--chain-id flag also sets these (see parsing).
CHAIN_ID_FLAG_SET=0
NODE_FLAG_SET=0
GRPC_FLAG_SET=0
if [[ -n "${LUMERA_CHAIN_ID:-}" ]]; then CHAIN_ID_FLAG_SET=1; fi
if [[ -n "${LUMERA_NODE:-}" || -n "${LUMERA_RPC:-}" ]]; then NODE_FLAG_SET=1; fi
if [[ -n "${LUMERA_GRPC:-}" ]]; then GRPC_FLAG_SET=1; fi

usage() {
  cat <<'USAGE'
Usage:
  scripts/chain-helper.sh <command> [flags]

Commands:
  max-validator-delegations
      Calculate the highest validator migration object count and recommend a
      max_validator_delegations value.
  stats
      Show chain-wide counts: total accounts, total/jailed validators, and
      total supernodes grouped by their current state.

Common flags:
  --network <name>          Use default endpoints for devnet, testnet, or mainnet
  --node, --rpc <url>        Tendermint RPC endpoint
                             default: selected network RPC
  --grpc <host:port>        gRPC endpoint for commands that need it
                             default: selected network gRPC
  --chain-id <id>           Chain ID; default: selected network chain ID
  --binary <path>           lumerad binary; default: lumerad
  --grpcurl <path>          grpcurl binary; default: grpcurl
  --grpc-insecure           use plaintext gRPC for grpcurl queries
  --limit <n>               query pagination limit; default: 1000
  --json                    emit machine-readable JSON
  -h, --help                show this help

max-validator-delegations flags:
  --buffer-percent <n>      integer percent buffer for suggested_cap; default: 30
  --allow-partial           allow staking-only fallback if evmigration
                             migration-estimate is unavailable

Environment overrides:
  LUMERA_CHAIN_ID, LUMERA_NODE or LUMERA_RPC, LUMERA_GRPC, LUMERA_BINARY,
  LUMERA_GRPCURL_BINARY, LUMERA_GRPC_INSECURE, LUMERA_GRPC_MAX_TIME,
  LUMERA_QUERY_LIMIT, LUMERA_BUFFER_PERCENT
USAGE
}

die() {
  local code="$1"
  shift
  printf 'ERROR: %s\n' "$*" >&2
  exit "$code"
}

warn() {
  if [[ "$JSON_OUTPUT" -eq 0 ]]; then
    printf 'WARN: %s\n' "$*" >&2
  fi
}

progress() {
  if [[ "$JSON_OUTPUT" -eq 0 ]]; then
    printf 'INFO: %s\n' "$*" >&2
  fi
}

require_value() {
  local flag="$1"
  local value="${2:-}"
  if [[ -z "$value" || "$value" == --* ]]; then
    die 1 "$flag requires a value"
  fi
}

require_uint() {
  local name="$1"
  local value="$2"
  if [[ ! "$value" =~ ^[0-9]+$ ]]; then
    die 1 "$name must be a non-negative integer"
  fi
}

require_tools() {
  command -v jq >/dev/null 2>&1 || die 1 "jq is required"
  command -v "$BIN" >/dev/null 2>&1 || die 1 "binary not found or not executable: $BIN"
}

require_grpcurl() {
  command -v "$GRPCURL" >/dev/null 2>&1 || return 1
}

apply_network_defaults() {
  local network="$1" chain_id rpc grpc
  case "$network" in
    devnet)
      chain_id="$DEVNET_CHAIN_ID"
      rpc="$DEVNET_RPC"
      grpc="$DEVNET_GRPC"
      ;;
    testnet)
      chain_id="$TESTNET_CHAIN_ID"
      rpc="$TESTNET_RPC"
      grpc="$TESTNET_GRPC"
      ;;
    mainnet)
      chain_id="$MAINNET_CHAIN_ID"
      rpc="$MAINNET_RPC"
      grpc="$MAINNET_GRPC"
      ;;
    *)
      die 1 "unknown network '$network'; expected devnet, testnet, or mainnet"
      ;;
  esac

  if [[ "$CHAIN_ID_FLAG_SET" -eq 0 ]]; then
    CHAIN_ID="$chain_id"
  fi
  if [[ "$NODE_FLAG_SET" -eq 0 ]]; then
    NODE="$rpc"
  fi
  if [[ "$GRPC_FLAG_SET" -eq 0 ]]; then
    GRPC="$grpc"
  fi
}

lumerad_query() {
  "$BIN" query "$@" \
    --node "$NODE" \
    --chain-id "$CHAIN_ID" \
    --output json
}

validators_json() {
  lumerad_query staking validators --page-limit "$LIMIT"
}

valoper_to_account() {
  local valoper="$1"
  "$BIN" debug addr "$valoper" | awk -F': ' '/^Bech32 Acc:/ { print $2; exit }'
}

estimate_counts_tsv() {
  local account="$1"
  lumerad_query evmigration migration-estimate "$account" | jq -r '
    def n($x): (($x // 0) | tonumber);
    (.estimate? // .) as $e
    | [
        n($e.val_delegation_count),
        n($e.val_unbonding_count),
        n($e.val_redelegation_count),
        (n($e.val_delegation_count) + n($e.val_unbonding_count) + n($e.val_redelegation_count))
      ]
    | @tsv
  '
}

# staking_count_total <subcommand> <address>
# Returns the exact total number of records for a staking sub-query using the
# node-side count_total. Counting (.records | length) on a single page silently
# truncates any validator that has more delegations than --page-limit (e.g. a
# validator with 1593 delegations reads as 1000 under the default limit), which
# is unsafe for sizing max_validator_delegations.
staking_count_total() {
  local subcmd="$1"
  local addr="$2"
  local out total page_len
  out="$(lumerad_query staking "$subcmd" "$addr" --page-limit 1 --page-count-total)" ||
    die 3 "staking $subcmd query failed for $addr"
  total="$(jq -r '(.pagination.total // "") | tostring' <<<"$out")"
  if [[ "$total" =~ ^[0-9]+$ ]]; then
    printf '%s\n' "$total"
    return
  fi
  # Nodes omit pagination.total for an empty result set, so an empty page means
  # zero records. A non-empty page without a total means count_total is not
  # supported and we would silently undercount, so fail loudly instead.
  page_len="$(jq -r '
    (.delegation_responses // .delegations
     // .unbonding_responses // .unbonding_delegations // []) | length
  ' <<<"$out")"
  if [[ "$page_len" == "0" ]]; then
    printf '0\n'
  else
    die 3 "staking $subcmd for $addr returned no pagination.total; node may not support --page-count-total"
  fi
}

delegations_to_count() {
  staking_count_total delegations-to "$1"
}

unbonding_from_count() {
  staking_count_total unbonding-delegations-from "$1"
}

grpcurl_call() {
  local data="$1"
  shift

  local args=(-max-time "$GRPC_MAX_TIME")
  if [[ "$GRPC_INSECURE" == "1" || "$GRPC_INSECURE" == "true" ]]; then
    args+=(-plaintext)
  fi
  "$GRPCURL" "${args[@]}" -d "$data" "$GRPC" "$@"
}

redelegations_from_src_page() {
  local valoper="$1"
  local page_key="${2:-}"
  local request

  request="$(jq -n \
    --arg src "$valoper" \
    --arg limit "$LIMIT" \
    --arg key "$page_key" '
      {
        srcValidatorAddr: $src,
        pagination: {
          limit: $limit
        }
      }
      | if $key != "" then .pagination.key = $key else . end
    ')"

  grpcurl_call "$request" cosmos.staking.v1beta1.Query/Redelegations
}

current_validator_cap() {
  local params cap
  params="$(lumerad_query evmigration params 2>/dev/null || true)"
  cap="$(jq -r '(.params.max_validator_delegations // empty)' <<<"$params" 2>/dev/null || true)"
  if [[ -n "$cap" && "$cap" =~ ^[0-9]+$ ]]; then
    printf '%s\n' "$cap"
  else
    printf '%s\n' "$DEFAULT_CURRENT_CAP"
  fi
}

rows_to_json() {
  if [[ "$#" -eq 0 ]]; then
    printf '[]\n'
    return
  fi

  printf '%s\n' "$@" | jq -R -s '
    split("\n")
    | map(select(length > 0))
    | map(split("\t") | {
        operator_address: .[0],
        account_address: .[1],
        moniker: .[2],
        val_delegation_count: (.[3] | tonumber),
        val_unbonding_count: (.[4] | tonumber),
        val_redelegation_count: (.[5] | tonumber),
        total: (.[6] | tonumber),
        exact: (.[7] == "true")
      })
  '
}

emit_result_json() {
  local mode="$1"
  local exact="$2"
  local current_cap="$3"
  local max_observed="$4"
  local suggested_cap="$5"
  local warnings_json="$6"
  shift 6
  local validators
  validators="$(rows_to_json "$@")"

  jq -n \
    --arg command "max-validator-delegations" \
    --arg chain_id "$CHAIN_ID" \
    --arg rpc "$NODE" \
    --arg grpc "$GRPC" \
    --arg mode "$mode" \
    --argjson exact "$exact" \
    --argjson buffer_percent "$BUFFER_PERCENT" \
    --argjson current_cap "$current_cap" \
    --argjson max_observed "$max_observed" \
    --argjson suggested_cap "$suggested_cap" \
    --argjson validators "$validators" \
    --argjson warnings "$warnings_json" \
    '{
      command: $command,
      chain_id: $chain_id,
      rpc: $rpc,
      grpc: $grpc,
      mode: $mode,
      exact: $exact,
      buffer_percent: $buffer_percent,
      current_cap: $current_cap,
      max_observed: $max_observed,
      suggested_cap: $suggested_cap,
      validators: $validators,
      warnings: $warnings
    }'
}

emit_result_human() {
  local mode="$1"
  local exact="$2"
  local current_cap="$3"
  local max_observed="$4"
  local suggested_cap="$5"
  local warning="$6"
  shift 6

  printf '%-4s %-48s %-24s %8s %8s %8s %8s\n' \
    "rank" "validator" "moniker" "deleg" "unbond" "redel" "total"

  local rank=1 row operator _account moniker delegations unbondings redelegations total _exact_row
  for row in "$@"; do
    IFS=$'\t' read -r operator _account moniker delegations unbondings redelegations total _exact_row <<<"$row"
    printf '%-4s %-48s %-24s %8s %8s %8s %8s\n' \
      "$rank" "$operator" "${moniker:0:24}" "$delegations" "$unbondings" "$redelegations" "$total"
    rank=$((rank + 1))
  done

  printf '\n'
  printf 'command: max-validator-delegations\n'
  printf 'chain_id: %s\n' "$CHAIN_ID"
  printf 'rpc: %s\n' "$NODE"
  printf 'grpc: %s\n' "$GRPC"
  printf 'mode: %s\n' "$mode"
  printf 'exact: %s\n' "$exact"
  printf 'current_cap: %s\n' "$current_cap"
  printf 'max_observed: %s\n' "$max_observed"
  printf 'buffer_percent: %s\n' "$BUFFER_PERCENT"
  printf 'suggested_cap: %s\n' "$suggested_cap"
  if [[ -n "$warning" ]]; then
    printf 'warning: %s\n' "$warning"
  fi
}

parse_max_validator_flags() {
  while (($#)); do
    case "$1" in
      --network)
        require_value "$1" "${2:-}"
        apply_network_defaults "$2"
        shift 2
        ;;
      --node|--rpc)
        require_value "$1" "${2:-}"
        NODE="$2"
        NODE_FLAG_SET=1
        shift 2
        ;;
      --grpc)
        require_value "$1" "${2:-}"
        GRPC="$2"
        GRPC_FLAG_SET=1
        shift 2
        ;;
      --chain-id)
        require_value "$1" "${2:-}"
        CHAIN_ID="$2"
        CHAIN_ID_FLAG_SET=1
        shift 2
        ;;
      --binary)
        require_value "$1" "${2:-}"
        BIN="$2"
        shift 2
        ;;
      --grpcurl)
        require_value "$1" "${2:-}"
        GRPCURL="$2"
        shift 2
        ;;
      --grpc-insecure)
        GRPC_INSECURE=1
        shift
        ;;
      --limit)
        require_value "$1" "${2:-}"
        require_uint "--limit" "$2"
        LIMIT="$2"
        shift 2
        ;;
      --buffer-percent)
        require_value "$1" "${2:-}"
        require_uint "--buffer-percent" "$2"
        BUFFER_PERCENT="$2"
        shift 2
        ;;
      --allow-partial)
        ALLOW_PARTIAL=1
        shift
        ;;
      --json)
        JSON_OUTPUT=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die 1 "unknown flag for max-validator-delegations: $1"
        ;;
    esac
  done
}

collect_validator_records() {
  local data="$1"
  jq -r '
    (.validators // [])
    | .[]
    | [(.operator_address // ""), (.description.moniker // "")]
    | @tsv
  ' <<<"$data"
}

collect_redelegation_counts_by_validator() {
  local -n counts_ref="$1"
  shift

  local total_records="$#"
  local record operator _account moniker page_key page rows src dst next_key index=0 source_count
  for record in "$@"; do
    index=$((index + 1))
    IFS=$'\t' read -r operator _account moniker <<<"$record"
    page_key=""
    source_count=0

    while :; do
      page="$(redelegations_from_src_page "$operator" "$page_key")" ||
        die 3 "grpcurl redelegation scan failed for source validator $operator"

      mapfile -t rows < <(jq -r '
        (.redelegationResponses // .redelegation_responses // [])
        | .[]
        | (.redelegation // {}) as $red
        | [
            ($red.validatorSrcAddress // $red.validator_src_address // ""),
            ($red.validatorDstAddress // $red.validator_dst_address // "")
          ]
        | @tsv
      ' <<<"$page")

      for row in "${rows[@]}"; do
        IFS=$'\t' read -r src dst <<<"$row"
        if [[ -n "$src" ]]; then
          counts_ref["$src"]=$((${counts_ref["$src"]:-0} + 1))
          if [[ "$src" == "$operator" ]]; then
            source_count=$((source_count + 1))
          fi
        fi
        if [[ -n "$dst" && "$dst" != "$src" ]]; then
          counts_ref["$dst"]=$((${counts_ref["$dst"]:-0} + 1))
        fi
      done

      next_key="$(jq -r '(.pagination.nextKey // .pagination.next_key // "")' <<<"$page")"
      if [[ -z "$next_key" || "$next_key" == "null" ]]; then
        break
      fi
      page_key="$next_key"
    done
    progress "scanned redelegations $index/$total_records: ${moniker:-$operator} count=$source_count"
  done
}

max_validator_delegations() {
  parse_max_validator_flags "$@"
  require_uint "LUMERA_QUERY_LIMIT" "$LIMIT"
  require_uint "LUMERA_BUFFER_PERCENT" "$BUFFER_PERCENT"
  require_tools

  local validator_data
  validator_data="$(validators_json)"

  local raw_records=()
  mapfile -t raw_records < <(collect_validator_records "$validator_data")
  if [[ "${#raw_records[@]}" -eq 0 ]]; then
    die 2 "no validators returned from staking validators query"
  fi
  progress "found ${#raw_records[@]} validators"

  local records=()
  local raw operator moniker account
  for raw in "${raw_records[@]}"; do
    IFS=$'\t' read -r operator moniker <<<"$raw"
    if [[ -z "$operator" ]]; then
      die 2 "staking validators query returned a validator without operator_address"
    fi
    account="$(valoper_to_account "$operator")"
    if [[ -z "$account" ]]; then
      die 2 "could not convert validator operator address to account address: $operator"
    fi
    records+=("$operator"$'\t'"$account"$'\t'"$moniker")
  done

  local first_account probe_counts
  IFS=$'\t' read -r _ first_account _ <<<"${records[0]}"

  local mode="evmigration-estimate"
  local exact="true"
  local warning=""
  local warnings_json="[]"
  if ! probe_counts="$(estimate_counts_tsv "$first_account" 2>/dev/null)"; then
    if require_grpcurl; then
      mode="staking-pre-evm"
      exact="true"
      progress "evmigration migration-estimate unavailable; using pre-EVM staking + gRPC redelegation scan"
    elif [[ "$ALLOW_PARTIAL" -eq 0 ]]; then
      die 3 "evmigration migration-estimate is unavailable on $NODE and grpcurl is unavailable for the pre-EVM redelegation scan. Install grpcurl or pass --grpcurl <path>; pass --allow-partial only for a staking-only estimate that is not safe for the final parameter."
    else
      mode="staking-partial"
      exact="false"
      warning="staking-partial mode excludes redelegations where this validator is the source or destination; it is not safe for final max_validator_delegations sizing."
      warnings_json="$(jq -n --arg warning "$warning" '[$warning]')"
      warn "$warning"
    fi
  fi

  local rows=()
  local record delegations unbondings redelegations total counts
  if [[ "$mode" == "evmigration-estimate" ]]; then
    progress "using evmigration migration-estimate query"
    local total_records="${#records[@]}" index=0
    for record in "${records[@]}"; do
      index=$((index + 1))
      IFS=$'\t' read -r operator account moniker <<<"$record"
      progress "estimating validator $index/$total_records: ${moniker:-$operator}"
      if [[ "$account" == "$first_account" ]]; then
        counts="$probe_counts"
      else
        counts="$(estimate_counts_tsv "$account")" || die 3 "evmigration migration-estimate failed for $account ($operator)"
      fi
      IFS=$'\t' read -r delegations unbondings redelegations total <<<"$counts"
      rows+=("$operator"$'\t'"$account"$'\t'"$moniker"$'\t'"$delegations"$'\t'"$unbondings"$'\t'"$redelegations"$'\t'"$total"$'\t'"true")
    done
  elif [[ "$mode" == "staking-pre-evm" ]]; then
    declare -A redelegation_counts=()
    collect_redelegation_counts_by_validator redelegation_counts "${records[@]}"

    local total_records="${#records[@]}" index=0
    for record in "${records[@]}"; do
      index=$((index + 1))
      IFS=$'\t' read -r operator account moniker <<<"$record"
      delegations="$(delegations_to_count "$operator")"
      unbondings="$(unbonding_from_count "$operator")"
      redelegations="${redelegation_counts["$operator"]:-0}"
      total=$((delegations + unbondings + redelegations))
      progress "counting staking records $index/$total_records: ${moniker:-$operator} count=$total (deleg=$delegations unbond=$unbondings redel=$redelegations)"
      rows+=("$operator"$'\t'"$account"$'\t'"$moniker"$'\t'"$delegations"$'\t'"$unbondings"$'\t'"$redelegations"$'\t'"$total"$'\t'"true")
    done
  else
    local total_records="${#records[@]}" index=0
    for record in "${records[@]}"; do
      index=$((index + 1))
      IFS=$'\t' read -r operator account moniker <<<"$record"
      delegations="$(delegations_to_count "$operator")"
      unbondings="$(unbonding_from_count "$operator")"
      redelegations="0"
      total=$((delegations + unbondings + redelegations))
      progress "counting partial staking records $index/$total_records: ${moniker:-$operator} count=$total (deleg=$delegations unbond=$unbondings)"
      rows+=("$operator"$'\t'"$account"$'\t'"$moniker"$'\t'"$delegations"$'\t'"$unbondings"$'\t'"$redelegations"$'\t'"$total"$'\t'"false")
    done
  fi

  local sorted_rows=()
  mapfile -t sorted_rows < <(printf '%s\n' "${rows[@]}" | sort -t $'\t' -k7,7nr)

  local max_observed=0
  if [[ "${#sorted_rows[@]}" -gt 0 ]]; then
    local top_row
    top_row="${sorted_rows[0]}"
    IFS=$'\t' read -r _ _ _ _ _ _ max_observed _ <<<"$top_row"
  fi

  local suggested_cap
  suggested_cap=$(((max_observed * (100 + BUFFER_PERCENT) + 99) / 100))

  local current_cap
  current_cap="$(current_validator_cap)"

  if [[ "$JSON_OUTPUT" -eq 1 ]]; then
    emit_result_json "$mode" "$exact" "$current_cap" "$max_observed" "$suggested_cap" "$warnings_json" "${sorted_rows[@]}"
  else
    emit_result_human "$mode" "$exact" "$current_cap" "$max_observed" "$suggested_cap" "$warning" "${sorted_rows[@]}"
  fi
}

# fetch_all <array_field> <query args...>
# Pages through a lumerad query, following pagination.next_key, and prints a
# single JSON array containing every element of the named top-level array
# field (e.g. "validators", "supernodes"). Avoids page-limit truncation when a
# query returns more records than a single page.
fetch_all() {
  local field="$1"
  shift
  local page_key="" page next acc='[]'
  while :; do
    if [[ -n "$page_key" ]]; then
      page="$(lumerad_query "$@" --page-limit "$LIMIT" --page-key "$page_key")" ||
        die 3 "paginated query failed: $*"
    else
      page="$(lumerad_query "$@" --page-limit "$LIMIT")" ||
        die 3 "paginated query failed: $*"
    fi
    acc="$(jq -c --argjson acc "$acc" --arg f "$field" '$acc + (.[$f] // [])' <<<"$page")"
    next="$(jq -r '(.pagination.next_key // .pagination.nextKey // "")' <<<"$page")"
    if [[ -z "$next" || "$next" == "null" ]]; then
      break
    fi
    page_key="$next"
  done
  printf '%s\n' "$acc"
}

parse_stats_flags() {
  while (($#)); do
    case "$1" in
      --network)
        require_value "$1" "${2:-}"
        apply_network_defaults "$2"
        shift 2
        ;;
      --node|--rpc)
        require_value "$1" "${2:-}"
        NODE="$2"
        NODE_FLAG_SET=1
        shift 2
        ;;
      --grpc)
        require_value "$1" "${2:-}"
        GRPC="$2"
        GRPC_FLAG_SET=1
        shift 2
        ;;
      --chain-id)
        require_value "$1" "${2:-}"
        CHAIN_ID="$2"
        CHAIN_ID_FLAG_SET=1
        shift 2
        ;;
      --binary)
        require_value "$1" "${2:-}"
        BIN="$2"
        shift 2
        ;;
      --limit)
        require_value "$1" "${2:-}"
        require_uint "--limit" "$2"
        LIMIT="$2"
        shift 2
        ;;
      --json)
        JSON_OUTPUT=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die 1 "unknown flag for stats: $1"
        ;;
    esac
  done
}

stats() {
  parse_stats_flags "$@"
  require_uint "LUMERA_QUERY_LIMIT" "$LIMIT"
  require_tools

  progress "querying total accounts"
  local accounts
  accounts="$(lumerad_query auth accounts --page-limit 1 --page-count-total |
    jq -r '(.pagination.total // "") | tostring')"
  [[ "$accounts" =~ ^[0-9]+$ ]] ||
    die 3 "auth accounts returned no pagination.total; node may not support --page-count-total"

  progress "querying validators"
  local validators_json validators_total validators_jailed
  validators_json="$(fetch_all validators staking validators)"
  validators_total="$(jq -r 'length' <<<"$validators_json")"
  validators_jailed="$(jq -r '[.[] | select(.jailed == true)] | length' <<<"$validators_json")"

  progress "querying supernodes"
  local supernodes_json supernodes_total supernode_states_json
  supernodes_json="$(fetch_all supernodes supernode list-supernodes)"
  supernodes_total="$(jq -r 'length' <<<"$supernodes_json")"
  # A supernode's current status is the highest-height entry in its state history.
  supernode_states_json="$(jq -c '
    [ .[]
      | (.states // [])
      | if length > 0 then (max_by(.height | tonumber).state) else "SUPERNODE_STATE_UNSPECIFIED" end
    ]
    | sort_by(.)
    | group_by(.)
    | map({state: .[0], count: length})
    | sort_by(-.count)
  ' <<<"$supernodes_json")"

  if [[ "$JSON_OUTPUT" -eq 1 ]]; then
    jq -n \
      --arg command "stats" \
      --arg chain_id "$CHAIN_ID" \
      --arg rpc "$NODE" \
      --argjson accounts "$accounts" \
      --argjson validators_total "$validators_total" \
      --argjson validators_jailed "$validators_jailed" \
      --argjson supernodes_total "$supernodes_total" \
      --argjson supernode_states "$supernode_states_json" \
      '{
        command: $command,
        chain_id: $chain_id,
        rpc: $rpc,
        accounts: { total: $accounts },
        validators: {
          total: $validators_total,
          jailed: $validators_jailed,
          not_jailed: ($validators_total - $validators_jailed)
        },
        supernodes: {
          total: $supernodes_total,
          by_state: $supernode_states
        }
      }'
  else
    printf 'command: stats\n'
    printf 'chain_id: %s\n' "$CHAIN_ID"
    printf 'rpc: %s\n' "$NODE"
    printf '\n'
    printf 'accounts:\n'
    printf '  total: %s\n' "$accounts"
    printf 'validators:\n'
    printf '  total:      %s\n' "$validators_total"
    printf '  jailed:     %s\n' "$validators_jailed"
    printf '  not_jailed: %s\n' "$((validators_total - validators_jailed))"
    printf 'supernodes:\n'
    printf '  total: %s\n' "$supernodes_total"
    jq -r '.[] | "  \(.state): \(.count)"' <<<"$supernode_states_json"
  fi
}

main() {
  if [[ "$#" -eq 0 ]]; then
    usage
    exit 1
  fi

  case "$1" in
    max-validator-delegations)
      shift
      max_validator_delegations "$@"
      ;;
    stats)
      shift
      stats "$@"
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      die 1 "unknown command: $1"
      ;;
  esac
}

main "$@"
