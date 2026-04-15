#!/usr/bin/env bash

set -euo pipefail

BIN="${1:-./build/lumerad}"

if [[ ! -x "$BIN" ]]; then
  echo "binary not found or not executable: $BIN" >&2
  exit 1
fi

commands=(
  "query action params"
  "query action action"
  "query action get-action-fee"
  "query action list-actions"
  "query action list-actions-by-creator"
  "query action list-actions-by-supernode"
  "query action list-actions-by-block-height"
  "query action list-expired-actions"
  "query action query-action-by-metadata"
  "tx action request-action"
  "tx action finalize-action"
  "tx action approve-action"
  "query audit params"
  "query audit evidence"
  "query audit evidence-by-subject"
  "query audit evidence-by-action"
  "query audit current-epoch"
  "query audit epoch-anchor"
  "query audit current-epoch-anchor"
  "query audit assigned-targets"
  "query audit epoch-report"
  "query audit epoch-reports-by-reporter"
  "query audit storage-challenge-reports"
  "query audit host-reports"
  "tx audit submit-epoch-report"
  "tx audit submit-evidence"
  "query claim params"
  "query claim claim-record"
  "query claim list-claimed"
  "tx claim claim"
  "tx claim delayed-claim"
  "query lumeraid params"
  "query supernode params"
  "query supernode get-supernode"
  "query supernode get-supernode-by-address"
  "query supernode list-supernodes"
  "query supernode get-metrics"
  "query supernode get-top-supernodes-for-block"
  "tx supernode register-supernode"
  "tx supernode deregister-supernode"
  "tx supernode start-supernode"
  "tx supernode stop-supernode"
  "tx supernode update-supernode"
  "tx supernode report-supernode-metrics"
)

for cmd in "${commands[@]}"; do
  read -r -a argv <<<"$cmd"
  echo "==> $BIN ${argv[*]} --help"
  "$BIN" "${argv[@]}" --help >/dev/null
done

echo "CLI help smoke test passed for ${#commands[@]} commands."
