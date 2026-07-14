#!/usr/bin/env bash
###################################################################################
# Copyright 2026 The Lumera Protocol
#
# Migration shell script for multisig legacy accounts (regular and validator).
#
# Multisig migration helper. Dispatches on the first positional argument to
# one of four subcommand functions wrapping lumerad tx evmigration
# {generate-proof-payload, sign-proof, combine-proof, submit-proof}.
# See docs/design/evmigration-multisig-scripts-design.md.

set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./evmigration-common.sh disable=SC1091
source "${SCRIPT_DIR}/evmigration-common.sh"

_mms_usage() {
  cat >&2 <<'USAGE'
Usage: migrate-multisig.sh <subcommand> [args...]

Subcommands:
  generate   Coordinator: produce proof.json (wraps generate-proof-payload)
  sign       Co-signer: append partial signature (wraps sign-proof)
  combine    Coordinator: merge partials into tx.json (wraps combine-proof)
  submit     Coordinator: broadcast + verify (wraps submit-proof)

Run `migrate-multisig.sh <subcommand> --help` for subcommand-specific flags.
Mainnet RPC example: https://rpc.lumera.io:443
USAGE
}

_mms_generate() {
  local legacy="" new="" kind="" chain_id="${LUMERA_CHAIN_ID:-${CHAIN_ID:-}}" node="${LUMERA_NODE:-tcp://localhost:26657}" out="proof.json"
  local sig_format="" binary="lumerad"
  local new_sub_pub_keys="" new_threshold="" legacy_key=""
  local new_key=""
  local keyring_backend="test" keyring_dir="" home_dir=""
  while (( $# > 0 )); do
    case "$1" in
      --legacy)           _require_value "$1" "$#" "${2-}"; legacy="$2"; shift 2 ;;
      --new)              _require_value "$1" "$#" "${2-}"; new="$2"; shift 2 ;;
      --kind)             log_error "generate: --kind is no longer supported; the script infers claim vs validator from chain state"; exit 1 ;;
      --chain-id)         _require_value "$1" "$#" "${2-}"; chain_id="$2"; shift 2 ;;
      --node)             _require_value "$1" "$#" "${2-}"; node="$2"; shift 2 ;;
      --out)              _require_value "$1" "$#" "${2-}"; out="$2"; shift 2 ;;
      --sig-format)       _require_value "$1" "$#" "${2-}"; sig_format="$2"; shift 2 ;;
      --new-key)          _require_value "$1" "$#" "${2-}"; new_key="$2"; shift 2 ;;
      --new-sub-pub-keys) _require_value "$1" "$#" "${2-}"; new_sub_pub_keys="$2"; shift 2 ;;
      --new-threshold)    _require_value "$1" "$#" "${2-}"; new_threshold="$2"; shift 2 ;;
      --legacy-key)       _require_value "$1" "$#" "${2-}"; legacy_key="$2"; shift 2 ;;
      --binary)           _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      --keyring-backend)  _require_value "$1" "$#" "${2-}"; keyring_backend="$2"; shift 2 ;;
      --keyring-dir)      _require_value "$1" "$#" "${2-}"; keyring_dir="$2"; shift 2 ;;
      --home)             _require_value "$1" "$#" "${2-}"; home_dir="$2"; shift 2 ;;
      -h|--help)
        cat >&2 <<'G_USAGE'
Usage: migrate-multisig.sh generate --legacy <multisig-key-or-addr> \
  (--new-key <evm-multisig-key> | --new-sub-pub-keys <k1,k2,...>) \
  [--chain-id <id>] [--out <path>] \
  [--new-threshold <K>]              Defaults to the on-chain legacy multisig threshold
  [--new <new-multisig-addr>]         Cross-checks the address derived from destination key material
  [--node <url>]                      RPC endpoint (default $LUMERA_NODE or tcp://localhost:26657;
                                      mainnet example: https://rpc.lumera.io:443)
  [--sig-format SIG_FORMAT_CLI|SIG_FORMAT_ADR036]
  [--keyring-backend <b>] [--keyring-dir <dir>] [--home <dir>]
  [--binary <path>]

--new-key is the easiest path when you already created the destination EVM
multisig key locally; the script reads its embedded eth_secp256k1 signer pubkeys.
That means --new-sub-pub-keys is inferred from the local keyring when --new-key
exists there.
--new-sub-pub-keys remains available for explicit key names or base64 compressed
33-byte eth_secp256k1 pubkeys. The chain exposes legacy multisig signers, but a
fresh destination address has no on-chain account record, so destination EVM
signer pubkeys must come from local keyring material or explicit pubkeys.
--out defaults to proof.json.
--chain-id is optional when $LUMERA_CHAIN_ID or $CHAIN_ID is set, or when it can
be auto-detected from the RPC endpoint.
The migration kind is always inferred from chain state: validator accounts use
validator migration, all other multisig accounts use claim migration.

The EVM chain ID is fixed by the binary (lcfg.EVMChainID) and not
user-configurable — the keeper always verifies against that constant.
G_USAGE
        exit 0 ;;
      *) log_error "unknown flag: $1"; exit 1 ;;
    esac
  done

  # Required-flag validation. --new is optional (cross-check only).
  if [[ -z "$legacy" ]]; then
    log_error "generate: --legacy is required"
    exit 1
  fi
  if [[ -n "$new_key" && -n "$new_sub_pub_keys" ]]; then
    log_error "generate: --new-key and --new-sub-pub-keys are mutually exclusive"
    exit 1
  fi
  if [[ -z "$new_key" && -z "$new_sub_pub_keys" ]]; then
    log_error "generate: pass --new-key <evm-multisig-key> or --new-sub-pub-keys <k1,k2,...>"
    log_error "the chain only stores legacy multisig pubkeys; a fresh destination has no on-chain EVM signer pubkeys"
    exit 1
  fi
  # Wire up globals used by common helpers
  # shellcheck disable=SC2034  # consumed by lumerad_q and auth_pubkey_type helpers
  BIN="$binary"
  # shellcheck disable=SC2034
  NODE="$node"
  # shellcheck disable=SC2034
  CHAIN_ID="$chain_id"
  # shellcheck disable=SC2034  # lumerad_q passes keyring flags through
  KEYRING_BACKEND="$keyring_backend"
  # shellcheck disable=SC2034
  KEYRING_DIR="$keyring_dir"
  # shellcheck disable=SC2034
  HOME_DIR="$home_dir"

  log_info "[generate 1/5] validating prerequisites and connecting to RPC $node"
  require_multisig_binary
  require_jq
  resolve_chain_id
  chain_id="$CHAIN_ID"
  if [[ -z "$chain_id" ]]; then
    log_error "generate: chain ID is required; pass --chain-id, set \$LUMERA_CHAIN_ID / \$CHAIN_ID, or use a reachable RPC endpoint for auto-detection"
    exit 1
  fi

  log_info "[generate 2/5] resolving legacy and destination multisig addresses"
  [[ "$keyring_backend" == "file" ]] && log_info "the encrypted keyring may prompt for its passphrase; input is hidden while typing"
  local legacy_input="$legacy"
  if [[ "$legacy" != lumera1* ]]; then
    legacy=$(resolve_address "$legacy_input")
    log_info "legacy multisig key $(legacy_value "$legacy_input") -> address $(legacy_value "$legacy")"
  fi

  if [[ -n "$new_key" ]]; then
    local key_threshold key_addr
    key_threshold=$(key_multisig_threshold "$new_key")
    if [[ -z "$key_threshold" || ! "$key_threshold" =~ ^[0-9]+$ || "$key_threshold" == "0" ]]; then
      log_error "could not read multisig threshold from destination key $(new_value "$new_key")"
      exit 1
    fi
    if [[ -n "$new_threshold" && "$new_threshold" != "$key_threshold" ]]; then
      log_error "--new-threshold=$new_threshold does not match destination key $(new_value "$new_key") threshold=$key_threshold"
      exit 1
    fi
    new_threshold="$key_threshold"
    new_sub_pub_keys=$(key_multisig_sub_pub_keys_csv "$new_key")
    key_addr=$(resolve_address "$new_key")
    if [[ -n "$new" && "$new" != "$key_addr" ]]; then
      log_error "--new $(new_value "$new") does not match destination key $(new_value "$new_key") address $(new_value "$key_addr")"
      exit 1
    fi
    new="$key_addr"
    log_info "using destination EVM multisig key $(new_value "$new_key") -> address $(new_value "$new")"
  fi

  log_info "[generate 3/5] checking the on-chain multisig shape and signer order"
  # Check on-chain pubkey BEFORE estimate so a nil-pubkey multisig gets
  # the exit-8 "seed the pubkey first" remediation, not a confusing
  # downstream error.
  local pk_type legacy_pubkey_json legacy_sub_pub_keys_json
  pk_type=$(auth_pubkey_type "$legacy")
  case "$pk_type" in
    none)
      log_error "multisig pubkey is not seeded on-chain for $(legacy_value "$legacy")"
      log_error "submit any transaction from the multisig account first, then retry"
      exit 8 ;;
    single-sig)
      log_error "legacy account $(legacy_value "$legacy") is single-sig; use migrate-account.sh or migrate-validator.sh"
      exit 3 ;;
    multisig) ;;
    *) log_error "unexpected pubkey type for $(legacy_value "$legacy"): $pk_type"; exit 2 ;;
  esac
  local legacy_threshold legacy_subkey_count
  legacy_pubkey_json=$(auth_multisig_pubkey_json "$legacy")
  legacy_threshold=$(jq -r '.threshold // empty' <<<"$legacy_pubkey_json")
  legacy_subkey_count=$(jq -r '.public_keys | length' <<<"$legacy_pubkey_json")
  legacy_sub_pub_keys_json=$(jq -c '[.public_keys[]?.key]' <<<"$legacy_pubkey_json")
  if [[ -z "$legacy_threshold" || ! "$legacy_threshold" =~ ^[0-9]+$ || "$legacy_threshold" == "0" ]]; then
    log_error "could not read legacy multisig threshold from chain for $(legacy_value "$legacy")"
    exit 2
  fi
  if [[ -z "$legacy_subkey_count" || ! "$legacy_subkey_count" =~ ^[0-9]+$ || "$legacy_subkey_count" == "0" ]]; then
    log_error "could not read legacy multisig signer pubkeys from chain for $(legacy_value "$legacy")"
    exit 2
  fi
  if [[ -z "$new_threshold" ]]; then
    new_threshold="$legacy_threshold"
    log_info "using on-chain legacy multisig threshold for new multisig: ${new_threshold}-of-${legacy_subkey_count}"
  fi

  log_info "[generate 4/5] checking migration eligibility and destination freshness"
  # Pull estimate — provides is_validator, would_succeed, is_multisig confirmation.
  local estimate
  estimate=$(preflight_estimate "$legacy")
  assert_multisig "$estimate"

  local detected_kind
  if [[ "$(jq -r '.is_validator' <<<"$estimate")" == "true" ]]; then
    detected_kind="validator"
  else
    detected_kind="claim"
  fi
  if [[ -z "$kind" ]]; then
    kind="$detected_kind"
    log_info "auto-detected multisig migration kind: $kind"
  fi

  # Design §3.1: catch already-migrated / already-used destinations
  # and doomed ceremonies BEFORE co-signers spend time on partials.
  # --new is optional (the CLI derives and returns it from --new-sub-pub-keys);
  # only probe the destination-side checks when the operator supplied it.
  assert_not_migrated "$legacy" "${new:-}"
  if [[ -n "$new" ]]; then
    assert_new_address_unused "$new"
    assert_destination_fresh "$new"
  fi
  assert_estimate_succeeds "$estimate"

  # generate-proof-payload needs keyring access to resolve --new-sub-pub-keys
  # entries given as local key names (vs. base64 pubkeys). Pass keyring flags
  # through directly rather than via lumerad_tx, since this command takes no
  # --from/--fee/--gas.
  local args=(tx evmigration generate-proof-payload
    --legacy "$legacy"
    --kind "$kind"
    --out "$out"
    --new-sub-pub-keys "$new_sub_pub_keys"
    --new-threshold "$new_threshold")
  [[ -n "$new"           ]] && args+=(--new "$new")
  [[ -n "$sig_format"    ]] && args+=(--sig-format "$sig_format")
  [[ -n "$legacy_key"    ]] && args+=(--legacy-key "$legacy_key")
  args+=(--keyring-backend "$keyring_backend")
  [[ -n "$keyring_dir" ]] && args+=(--keyring-dir "$keyring_dir")
  [[ -n "$home_dir"    ]] && args+=(--home "$home_dir")
  args+=(--node "$node" --chain-id "$chain_id" --output json)

  log_info "[generate 5/5] writing proof template to $out"
  log_info "  legacy: $(legacy_value "$legacy")"
  [[ -n "$new" ]] && log_info "  new:    $(new_value "$new")"
  "$BIN" "${args[@]}"
  if [[ -f "$out" ]]; then
    local proof_json proof_new_threshold proof_new_keys_json
    proof_json=$(read_proof_file "$out")
    proof_new_threshold=$(jq -r '.new.threshold' <<<"$proof_json")
    proof_new_keys_json=$(jq -c '.new.sub_pub_keys' <<<"$proof_json")
    log_signer_order_json_array "On-chain legacy signer order" "$legacy_threshold" "$legacy_sub_pub_keys_json"
    log_signer_order_json_array "Destination signer order" "$proof_new_threshold" "$proof_new_keys_json"
    log_info "Signing instruction: co-signers should sign the same signer index on both legacy and new sides"
    log_info "For same-mnemonic migrations: recover each EVM sub-key from the matching legacy mnemonic, then build the destination multisig with --nosort in the legacy order above"
  fi
  log_info "generate complete — no transaction was broadcast"
  log_info "NEXT: securely distribute $out to at least K co-signers; each runs 'migrate-multisig.sh sign'"
}
_mms_sign() {
  local input="" from="" new_key="" chain_id="${LUMERA_CHAIN_ID:-${CHAIN_ID:-}}" out="" binary="lumerad"
  local node="${LUMERA_NODE:-tcp://localhost:26657}" keyring_backend="test" keyring_dir="" home_dir=""
  local positional=()
  while (( $# > 0 )); do
    case "$1" in
      --from)             _require_value "$1" "$#" "${2-}"; from="$2"; shift 2 ;;
      --new-key)          _require_value "$1" "$#" "${2-}"; new_key="$2"; shift 2 ;;
      --chain-id)         _require_value "$1" "$#" "${2-}"; chain_id="$2"; shift 2 ;;
      --node)             _require_value "$1" "$#" "${2-}"; node="$2"; shift 2 ;;
      --out)              _require_value "$1" "$#" "${2-}"; out="$2"; shift 2 ;;
      --binary)           _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      --keyring-backend)  _require_value "$1" "$#" "${2-}"; keyring_backend="$2"; shift 2 ;;
      --keyring-dir)      _require_value "$1" "$#" "${2-}"; keyring_dir="$2"; shift 2 ;;
      --home)             _require_value "$1" "$#" "${2-}"; home_dir="$2"; shift 2 ;;
      -h|--help)
        cat >&2 <<'S_USAGE'
Usage: migrate-multisig.sh sign <proof-or-partial.json> \
  [--from <my-legacy-sub-key>] [--new-key <my-eth-sub-key>] \
  [--chain-id <id>] --out <partial.json> \
  [--node <url>] [--keyring-backend <b>] [--keyring-dir <dir>] [--home <dir>] [--binary <path>]

Purpose:
  A co-signer reads proof.json, signs the side(s) for keys they control, and
  writes a partial JSON file to return to the coordinator.

Required:
  <proof-or-partial.json>       Proof template or an existing partial file.
  --out <partial.json>          Output file for this signer.
  --from <legacy-sub-key>       Legacy Cosmos secp256k1 sub-key to sign the
                                legacy side.
  --new-key <eth-sub-key>       New eth_secp256k1 sub-key to sign the EVM side.

At least one of --from (Cosmos secp256k1 sub-key for the legacy side) or
--new-key (eth_secp256k1 sub-key for the new side) must be supplied. A
co-signer who holds both sub-keys passes both flags to sign both sides in
one invocation; re-running is idempotent (replaces the prior entry at the
same index).

Validation before signing:
  - input file is a multisig proof/partial and has a valid payload hash
  - --from is a Cosmos secp256k1 key listed in legacy.sub_pub_keys
  - --new-key is an eth_secp256k1 key listed in new.sub_pub_keys

--chain-id is optional when $LUMERA_CHAIN_ID or $CHAIN_ID is set, or when it can
be auto-detected from the RPC endpoint.

Examples:
  migrate-multisig.sh sign proof.json --from alice-legacy --new-key alice-evm --out partial-alice.json
  migrate-multisig.sh sign proof.json --from alice-legacy --out partial-legacy-alice.json
  migrate-multisig.sh sign proof.json --new-key alice-evm --out partial-new-alice.json
S_USAGE
        exit 0 ;;
      --*) log_error "unknown flag: $1"; exit 1 ;;
      *)   positional+=("$1"); shift ;;
    esac
  done

  if (( ${#positional[@]} != 1 )); then
    log_error "sign: expected exactly one positional argument (<proof-or-partial.json>)"
    exit 1
  fi
  input="${positional[0]}"

  if [[ -z "$from" && -z "$new_key" ]]; then
    log_error "sign: at least one of --from or --new-key is required"
    exit 1
  fi
  if [[ -z "$out" ]]; then
    log_error "sign: --out is required"
    exit 1
  fi

  # shellcheck disable=SC2034
  BIN="$binary"
  # shellcheck disable=SC2034
  NODE="$node"
  # shellcheck disable=SC2034
  CHAIN_ID="$chain_id"
  # shellcheck disable=SC2034
  KEYRING_BACKEND="$keyring_backend"
  # shellcheck disable=SC2034
  KEYRING_DIR="$keyring_dir"
  # shellcheck disable=SC2034
  HOME_DIR="$home_dir"

  log_info "[sign 1/4] validating prerequisites and chain ID"
  require_multisig_binary
  require_jq
  resolve_chain_id
  chain_id="$CHAIN_ID"
  if [[ -z "$chain_id" ]]; then
    log_error "sign: chain ID is required; pass --chain-id or set \$LUMERA_CHAIN_ID / \$CHAIN_ID"
    exit 1
  fi

  log_info "[sign 2/4] validating proof file $input"
  # Parse + validate the input proof/partial. read_proof_file rejects
  # single-key-on-either-side (exit 3), bad payload_hex (exit 9), missing
  # fields (exit 9), and structural issues (exit 9).
  local pjson
  pjson=$(read_proof_file "$input")

  log_info "[sign 3/4] matching your local key(s) to signer positions"
  [[ "$keyring_backend" == "file" ]] && log_info "the encrypted keyring may prompt for its passphrase; input is hidden while typing"
  local from_idx="" new_idx=""
  if [[ -n "$from" ]]; then
    local from_pubkey listed
    from_pubkey=$(key_pubkey_b64 "$from")
    listed=$(jq -r '.legacy.sub_pub_keys[]' <<<"$pjson")
    if ! grep -qFx "$from_pubkey" <<<"$listed"; then
      log_error "--from '$(legacy_value "$from")' pubkey is not among legacy.sub_pub_keys in $input"
      exit 1
    fi
    from_idx=$(jq -r --arg pk "$from_pubkey" '.legacy.sub_pub_keys | index($pk) // empty' <<<"$pjson")
    log_info "legacy signer index $from_idx: $(legacy_value "$from")"
  fi
  if [[ -n "$new_key" ]]; then
    local new_pubkey listed_new
    new_pubkey=$(key_pubkey_b64 "$new_key")
    listed_new=$(jq -r '.new.sub_pub_keys[]' <<<"$pjson")
    if ! grep -qFx "$new_pubkey" <<<"$listed_new"; then
      log_error "--new-key '$(new_value "$new_key")' pubkey is not among new.sub_pub_keys in $input"
      exit 1
    fi
    new_idx=$(jq -r --arg pk "$new_pubkey" '.new.sub_pub_keys | index($pk) // empty' <<<"$pjson")
    log_info "new signer index $new_idx: $(new_value "$new_key")"
  fi
  log_info "Signing instruction: sign the same signer index on both legacy and new sides; combine only counts matching indices"

  # Multisig-to-multisig signer-index alignment pre-check. When a co-signer
  # passes BOTH --from and --new-key, the two keys must occupy the same signer
  # position in their respective multisigs (mirror-source rule). The on-chain
  # consensus check enforces this, and `lumerad sign-proof` rejects mismatches
  # with a terse error. We catch it earlier with an actionable remediation so
  # the operator doesn't have to round-trip through cryptic CLI output.
  #
  # Root cause when this fires: the destination EVM multisig was built without
  # --nosort, so cosmos-sdk sorted its eth_secp256k1 sub-pubkeys by bytes —
  # and that byte-order differs from the legacy secp256k1 byte-order. Result:
  # member N's legacy key and EVM key land at different indices.
  if [[ -n "$from_idx" && -n "$new_idx" && "$from_idx" != "$new_idx" ]]; then
    local legacy_threshold
    legacy_threshold=$(jq -r '.legacy.threshold // .new.threshold // "K"' <<<"$pjson")
    log_error "signer-index mismatch: legacy key '$(legacy_value "$from")' is at index $from_idx,"
    log_error "  but new key '$(new_value "$new_key")' is at index $new_idx in $input"
    log_error ""
    log_error "Multisig migration requires the same signer position on both sides."
    log_error "The destination EVM multisig was almost certainly built WITHOUT --nosort,"
    log_error "so its sub-pub-keys were re-sorted by bytes and no longer mirror the legacy"
    log_error "member order."
    log_error ""
    log_error "Remediation — rebuild the destination multisig in legacy member order:"
    log_error "  1. Get legacy member order from chain:"
    log_error "     lumerad query auth account <legacy-multisig-address> -o json \\"
    log_error "       | jq -r '.account.value.pub_key.public_keys[].key'"
    log_error "  2. Identify each member's eth_secp256k1 sub-key (in the SAME order)."
    log_error "  3. Recreate the destination key WITH --nosort:"
    log_error "       lumerad keys delete <new-multisig-key> -y    # if it already exists"
    log_error "       lumerad keys add <new-multisig-key> \\"
    log_error "         --multisig=<eth-sub-1>,<eth-sub-2>,...,<eth-sub-N> \\"
    log_error "         --multisig-threshold=${legacy_threshold} \\"
    log_error "         --nosort"
    log_error "  4. Re-run 'migrate-multisig.sh generate' with the rebuilt key, then redistribute"
    log_error "     the fresh proof.json to co-signers."
    exit 11
  fi

  # Pass through to lumerad tx evmigration sign-proof. At least one of
  # --from / --new-key is set (checked above).
  local args=(tx evmigration sign-proof "$input"
    --chain-id "$chain_id"
    --out "$out"
    --keyring-backend "$keyring_backend")
  [[ -n "$from"        ]] && args+=(--from "$from")
  [[ -n "$new_key"     ]] && args+=(--new-key "$new_key")
  [[ -n "$keyring_dir" ]] && args+=(--keyring-dir "$keyring_dir")
  [[ -n "$home_dir"    ]] && args+=(--home "$home_dir")

  local sides=()
  [[ -n "$from"    ]] && sides+=("legacy($(legacy_value "$from"))")
  [[ -n "$new_key" ]] && sides+=("new($(new_value "$new_key"))")
  log_info "[sign 4/4] signing $input: ${sides[*]}"
  [[ "$keyring_backend" == "file" ]] && log_info "enter the keyring passphrase when prompted; signing does not broadcast"
  "$BIN" "${args[@]}"
  log_info "sign complete — partial written to $out; no transaction was broadcast"
  log_info "NEXT: return $out to the coordinator"
}
_mms_combine() {
  local out="" binary="lumerad"
  local node="${LUMERA_NODE:-tcp://localhost:26657}"
  local positional=()
  while (( $# > 0 )); do
    case "$1" in
      --out)     _require_value "$1" "$#" "${2-}"; out="$2"; shift 2 ;;
      --node)    _require_value "$1" "$#" "${2-}"; node="$2"; shift 2 ;;
      --binary)  _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      -h|--help)
        cat >&2 <<'C_USAGE'
Usage: migrate-multisig.sh combine <partial1.json> <partial2.json> [...] --out <tx.json> \
  [--node <url>] [--binary <path>]

Purpose:
  The coordinator merges co-signer partial JSON files into a final tx.json
  that can be submitted with `migrate-multisig.sh submit`.

Required:
  <partial*.json>       One or more partial files returned by co-signers.
  --out <tx.json>       Output transaction file.

Optional:
  --node <url>          RPC endpoint for gas simulation (default $LUMERA_NODE or
                        tcp://localhost:26657). combine now simulates gas (--gas auto)
                        and therefore requires connectivity to a node; the coordinator
                        already needs a node for the subsequent submit step.

Validation before combine:
  - all partials are valid multisig proof/partial files
  - all partials agree on chain ID, legacy address, new address, kind,
    payload, thresholds, signature format, and sub-pub-key lists
  - each side has at least K valid signer entries
  - the same signer indices meet quorum on both legacy and new sides
  - `lumerad tx evmigration combine-proof` accepts the partial signatures

Exit codes:
  4   quorum is not met, or quorum exists per-side but not by matching signer index
  9   partial files are malformed or disagree on immutable proof fields

Example:
  migrate-multisig.sh combine partial-alice.json partial-bob.json --out tx.json
C_USAGE
        exit 0 ;;
      --*) log_error "unknown flag: $1"; exit 1 ;;
      *)   positional+=("$1"); shift ;;
    esac
  done

  if (( ${#positional[@]} < 1 )); then
    log_error "combine: at least one partial file required"
    exit 1
  fi
  if [[ -z "$out" ]]; then
    log_error "combine: --out is required"
    exit 1
  fi

  # shellcheck disable=SC2034
  BIN="$binary"
  # shellcheck disable=SC2034
  NODE="$node"

  log_info "[combine 1/3] validating prerequisites and RPC connectivity"
  require_multisig_binary
  require_jq
  resolve_chain_id

  log_info "[combine 2/3] validating ${#positional[@]} partial file(s) and checking matching-index quorum"
  # Per-file + cross-file consistency check. summarize_partials prints the
  # K-of-N entry-presence matrix to stderr and exits 9 on cross-file
  # disagreement (it calls read_proof_file internally, which rejects
  # single-key proofs with exit 3 and bad payload_hex with exit 9).
  # Returns 0 iff distinct signer indices >= threshold.
  if ! summarize_partials "${positional[@]}"; then
    exit 4
  fi

  # Pass through to lumerad combine-proof. If lumerad reports fewer valid
  # signatures than the threshold, map its exit to exit 4.
  log_info "[combine 3/3] combining signatures and simulating gas through $node"
  log_info "gas simulation may take a while for validators with many delegations"
  local args=(tx evmigration combine-proof "${positional[@]}" --out "$out" \
    --node "$node" --chain-id "$CHAIN_ID" \
    --gas auto --gas-adjustment "$MIGRATION_GAS_ADJUSTMENT")
  local combine_out combine_rc=0
  combine_out=$("$BIN" "${args[@]}" 2>&1) || combine_rc=$?
  printf '%s\n' "$combine_out" >&2
  if [[ "$combine_out" == *"need "*"valid partial signatures"* ]]; then
    exit 4
  fi
  if (( combine_rc != 0 )); then
    exit "$combine_rc"
  fi
  log_info "combine complete — transaction written to $out; nothing was broadcast"
  log_info "NEXT: review the summary, then run 'migrate-multisig.sh submit $out'"
}
_mms_submit() {
  local input="" chain_id="${LUMERA_CHAIN_ID:-${CHAIN_ID:-}}" node="${LUMERA_NODE:-tcp://localhost:26657}" binary="lumerad"
  local keyring_backend="test" keyring_dir="" home_dir=""
  local yes=0 dry_run=0 node_stopped=0
  local positional=()
  while (( $# > 0 )); do
    case "$1" in
      --chain-id)         _require_value "$1" "$#" "${2-}"; chain_id="$2"; shift 2 ;;
      --node)             _require_value "$1" "$#" "${2-}"; node="$2"; shift 2 ;;
      --binary)           _require_value "$1" "$#" "${2-}"; binary="$2"; shift 2 ;;
      --keyring-backend)  _require_value "$1" "$#" "${2-}"; keyring_backend="$2"; shift 2 ;;
      --keyring-dir)      _require_value "$1" "$#" "${2-}"; keyring_dir="$2"; shift 2 ;;
      --home)             _require_value "$1" "$#" "${2-}"; home_dir="$2"; shift 2 ;;
      --yes|-y)           yes=1; shift ;;
      --dry-run)          dry_run=1; shift ;;
      --i-have-stopped-the-node) node_stopped=1; shift ;;
      -h|--help)
        cat >&2 <<'SU_USAGE'
Usage: migrate-multisig.sh submit <tx.json> \
  [--chain-id <id>] \
  [--node <url>] \
  [--keyring-backend <b>] [--keyring-dir <dir>] [--home <dir>] \
  [--yes] [--dry-run] [--i-have-stopped-the-node] [--binary <path>]

Purpose:
  The coordinator broadcasts the combined multisig migration tx and verifies
  chain state after inclusion.

Required:
  <tx.json>                         Transaction file produced by combine.
  --i-have-stopped-the-node          Required for validator migrations in
                                    non-interactive runs; confirms the
                                    validator node is stopped before broadcast.

Safety checks before broadcast:
  - tx.json is a multisig-to-multisig migration tx
  - legacy address has no migration record
  - new address has no migration records
  - new address does not already exist on-chain
  - fresh migration-estimate still succeeds

Mainnet RPC example: https://rpc.lumera.io:443
--chain-id is optional when $LUMERA_CHAIN_ID or $CHAIN_ID is set, or when it can
be auto-detected from the RPC endpoint.

submit-proof does not sign at the Cosmos tx layer — migration messages
declare zero signers and fees are waived by the evmigration ante handler.
There is no --from / --fee / --gas-prices.

--dry-run performs all checks and stops before broadcast.
--yes skips the final broadcast prompt, but does not replace
--i-have-stopped-the-node for validator migrations.

Examples:
  migrate-multisig.sh submit tx.json
  migrate-multisig.sh submit tx.json --yes
  migrate-multisig.sh submit tx.json --i-have-stopped-the-node
SU_USAGE
        exit 0 ;;
      --*) log_error "unknown flag: $1"; exit 1 ;;
      *)   positional+=("$1"); shift ;;
    esac
  done

  if (( ${#positional[@]} != 1 )); then
    log_error "submit: expected exactly one positional argument (<tx.json>)"
    exit 1
  fi
  input="${positional[0]}"

  # shellcheck disable=SC2034
  BIN="$binary"
  # shellcheck disable=SC2034
  NODE="$node"
  # shellcheck disable=SC2034
  CHAIN_ID="$chain_id"
  # shellcheck disable=SC2034
  KEYRING_BACKEND="$keyring_backend"
  # shellcheck disable=SC2034
  KEYRING_DIR="$keyring_dir"
  # shellcheck disable=SC2034
  HOME_DIR="$home_dir"
  # shellcheck disable=SC2034
  YES="$yes"
  # shellcheck disable=SC2034
  DRY_RUN="$dry_run"

  local submit_mode="LIVE (will ask before broadcast)"
  (( dry_run == 1 )) && submit_mode="DRY RUN (no broadcast)"
  log_info "multisig submit mode: $submit_mode"
  log_info "RPC endpoint: $node (HTTP-only endpoints are supported)"
  log_info "[1/6] validating local prerequisites and resolving chain ID"
  require_multisig_binary
  require_jq
  resolve_chain_id
  chain_id="$CHAIN_ID"
  if [[ -z "$chain_id" ]]; then
    log_error "submit: chain ID is required; pass --chain-id, set \$LUMERA_CHAIN_ID / \$CHAIN_ID, or use a reachable RPC endpoint for auto-detection"
    exit 1
  fi

  log_info "[2/6] validating the combined multisig transaction file"
  # Parse + validate tx.json. Rejects non-multisig→multisig (exit 3),
  # missing fields / malformed (exit 9). Emits compact summary JSON.
  local tx_meta
  tx_meta=$(read_migration_tx_file "$input")
  local legacy new kind threshold num_signers new_threshold new_num_signers
  legacy=$(jq -r '.legacy_address' <<<"$tx_meta")
  new=$(jq -r '.new_address' <<<"$tx_meta")
  kind=$(jq -r '.kind' <<<"$tx_meta")
  threshold=$(jq -r '.threshold' <<<"$tx_meta")
  num_signers=$(jq -r '.num_signers' <<<"$tx_meta")
  new_threshold=$(jq -r '.new_threshold' <<<"$tx_meta")
  new_num_signers=$(jq -r '.new_num_signers' <<<"$tx_meta")

  log_info "[3/6] checking migration history and destination freshness"
  assert_not_migrated "$legacy" "$new"
  assert_new_address_unused "$new"
  assert_destination_fresh "$new"

  # Fresh estimate — catches ceremony-duration chain-state drift.
  log_info "[4/6] requesting a fresh migration estimate"
  local estimate
  estimate=$(preflight_estimate "$legacy")
  assert_multisig "$estimate"
  assert_estimate_succeeds "$estimate"

  log_info "[5/6] recording the legacy balance for post-migration verification"
  local snap
  snap=$(snapshot_bank_balances "$legacy")

  # Confirmation banner
  {
    printf '\n==== Multisig migration submit ====\n'
    printf '  Kind:        %s\n' "$kind"
    printf '  Legacy msig: %s-of-%s\n' "$threshold" "$num_signers"
    printf '  New msig:    %s-of-%s (eth sub-keys)\n' "$new_threshold" "$new_num_signers"
    printf '  Legacy:      %s\n' "$(legacy_value "$legacy")"
    printf '  New:         %s\n' "$(new_value "$new")"
    printf '===================================\n\n'
  } >&2

  # Validator kind needs separate downtime acknowledgement.
  if [[ "$kind" == "validator" ]]; then
    cat >&2 <<'BANNER'
================================================================
WARNING — VALIDATOR MIGRATION
Your validator will miss blocks and may be jailed during
migration. The node MUST be stopped before broadcasting this tx.
================================================================
BANNER
    if (( node_stopped != 1 )); then
      if [[ ! -t 0 ]]; then
        log_error "validator downtime not acknowledged and no TTY available"
        log_error "re-run with --i-have-stopped-the-node to confirm non-interactively"
        exit 10
      fi
      local reply=""
      printf 'Type "yes" to confirm the node is stopped: ' >&2
      read -r reply || true
      if [[ "$reply" != "yes" ]]; then
        log_error "validator downtime not acknowledged"
        exit 10
      fi
    fi
  fi

  # Skip the interactive prompt in --dry-run; nothing destructive will happen.
  if (( DRY_RUN == 1 )); then
    log_info "[6/6] dry-run complete: all pre-flight checks passed; no transaction was broadcast"
    return 0
  fi
  log_info "[6/6] requesting confirmation and broadcasting over RPC"
  confirm "Proceed with broadcast?"

  # submit-proof does not take --from; authorization is in the proof bytes.
  # We still pass keyring flags so the SDK's NewFactoryCLI can construct a
  # keyring-less context without erroring, and --output json for parsing.
  _read_migration_tx_timeout_flags
  local args=(tx evmigration submit-proof "$input"
    --chain-id "$chain_id"
    --node "$node"
    --keyring-backend "$keyring_backend"
    --output json)
  args+=("${_MIGRATION_TX_TIMEOUT_FLAGS[@]}")
  [[ -n "$keyring_dir" ]] && args+=(--keyring-dir "$keyring_dir")
  [[ -n "$home_dir"    ]] && args+=(--home "$home_dir")
  (( yes == 1 )) && args+=(-y)

  # Capture stdout, stderr, and exit code separately so a partial RPC failure
  # (e.g. lumerad prints `EOF` to stderr after the CometBFT RPC stream drops)
  # doesn't kill the script under `set -e` with no diagnostic. The tx may or
  # may not have entered the mempool; before erroring out, probe chain state
  # for the migration record so the operator gets a definitive answer instead
  # of a bare `EOF` line.
  local broadcast_json broadcast_err broadcast_rc=0 tx_hash
  local stderr_file
  stderr_file=$(mktemp)
  broadcast_json=$("$BIN" "${args[@]}" 2>"$stderr_file") || broadcast_rc=$?
  broadcast_err=$(<"$stderr_file")
  rm -f "$stderr_file"
  if (( broadcast_rc != 0 )); then
    recover_submit_after_broadcast_error "$broadcast_rc" "$broadcast_err" "$legacy" "$new" "$snap"
    return 0
  fi
  tx_hash=$(assert_broadcast_accepted "$broadcast_json")

  log_info "broadcast tx $tx_hash; waiting for inclusion..."
  # rc=2 means indexation timeout (tx may still land); fall through to
  # verify_migration which checks authoritative chain state. Only fatal on rc=1.
  local wait_rc=0
  wait_for_tx "$tx_hash" || wait_rc=$?
  if (( wait_rc == 1 )); then
    exit 1
  fi
  verify_migration "$legacy" "$new" "$snap"
  show_migration_summary "$legacy" "$new"

  log_info "migration complete"
  log_info "  legacy: $(legacy_value "$legacy")"
  log_info "  new:    $(new_value "$new")"
  log_info "  tx:     $tx_hash"
  if [[ "$kind" == "validator" ]]; then
    log_info "NEXT: restart the validator node immediately and monitor missed-block/jail status"
  else
    log_info "NEXT: distribute the confirmed destination address and use the destination multisig for future transactions"
  fi
}

recover_submit_after_broadcast_error() {
  local broadcast_rc="$1" broadcast_err="$2" legacy="$3" new="$4" snap="$5"
  local timeout="${LUMERA_TX_WAIT_TIMEOUT:-90}"
  local started=$SECONDS
  local logged_wait=0

  log_error "broadcast command failed (rc=$broadcast_rc)"
  [[ -n "$broadcast_err" ]] && log_error "lumerad stderr: $broadcast_err"
  log_error "checking chain state — the tx may still have landed:"
  log_error "  $BIN query evmigration migration-record $(legacy_value "$legacy") --node $NODE"

  while true; do
    local rec_json rec_legacy rec_new rec_height
    if ! rec_json=$(lumerad_q_capture evmigration migration-record "$legacy"); then
      log_error "could not query migration-record for $(legacy_value "$legacy"); cannot determine whether the tx landed"
      log_lumerad_err
      log_error "do not re-run submit until the migration record is checked manually"
      exit 7
    fi

    rec_legacy=$(jq -r '.record.legacy_address // empty' <<<"$rec_json")
    if [[ -n "$rec_legacy" ]]; then
      rec_new=$(jq -r '.record.new_address // "<missing>"' <<<"$rec_json")
      rec_height=$(jq -r '.record.migration_height // "<unknown>"' <<<"$rec_json")
      if [[ "$rec_new" == "$new" ]]; then
        log_info "on-chain migration record found for $(legacy_value "$legacy") -> $(new_value "$new") at height $rec_height"
        log_info "broadcast appears to have succeeded despite the RPC error above; running post-broadcast verification"
        verify_migration "$legacy" "$new" "$snap"
        show_migration_summary "$legacy" "$new"
        log_info "migration complete"
        return 0
      fi
      log_error "migration record found for $(legacy_value "$legacy"), but it points to a DIFFERENT destination:"
      log_error "  on-chain destination:  $(new_value "$rec_new") (migrated at height $rec_height)"
      log_error "  destination you asked: $(new_value "$new")"
      log_error "do not re-run submit until the destination mismatch is resolved"
      exit 7
    fi

    if (( SECONDS - started >= timeout )); then
      break
    fi
    if (( logged_wait == 0 )); then
      log_info "no migration record visible yet; polling for up to ${timeout}s before giving retry guidance"
      logged_wait=1
    fi
    sleep 1
  done

  log_error "no migration record found for $(legacy_value "$legacy") after ${timeout}s — tx did not land within the wait window; safe to re-run submit"
  exit 2
}

main() {
  if (( $# == 0 )); then
    _mms_usage
    exit 1
  fi
  local subcmd="$1"
  shift
  case "$subcmd" in
    generate)   _mms_generate "$@" ;;
    sign)       _mms_sign "$@" ;;
    combine)    _mms_combine "$@" ;;
    submit)     _mms_submit "$@" ;;
    -h|--help)  _mms_usage; exit 0 ;;
    *)          _mms_usage; exit 1 ;;
  esac
}

main "$@"
