#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  WORKDIR="$(mktemp -d)"
  mkdir -p "$WORKDIR"
  CLAIMS_FILE="$WORKDIR/claims.csv"
  printf 'address,balance\n' > "$CLAIMS_FILE"
}

teardown() {
  rm -rf "$WORKDIR"
}

write_lumerad_shim() {
  local help_text="$1"
  SHIM="$WORKDIR/lumerad"
  cat > "$SHIM" <<EOF
#!/usr/bin/env bash
if [ "\$1" = "start" ] && [ "\$2" = "--help" ]; then
  cat <<'HELP'
$help_text
HELP
  exit 0
fi
exit 1
EOF
  chmod +x "$SHIM"
}

@test "claims start flags use only claims-path for older binaries" {
  write_lumerad_shim "      --claims-path string   Path to claims.csv file"

  run bash -c 'source "$1"; lumerad_claims_start_flags "$2" "$3"' _ \
    "$REPO_ROOT/devnet/scripts/common.sh" "$SHIM" "$CLAIMS_FILE"

  [ "$status" -eq 0 ]
  [ "$output" = "--claims-path=$CLAIMS_FILE" ]
}

@test "claims start flags force claim loading for newer binaries" {
  write_lumerad_shim "      --skip-claims-check
      --claims-path string   Path to claims.csv file"

  run bash -c 'source "$1"; lumerad_claims_start_flags "$2" "$3"' _ \
    "$REPO_ROOT/devnet/scripts/common.sh" "$SHIM" "$CLAIMS_FILE"

  [ "$status" -eq 0 ]
  [ "$output" = "--skip-claims-check=false --claims-path=$CLAIMS_FILE" ]
}

@test "claims start flags are empty when claims file is absent" {
  write_lumerad_shim "      --claims-path string   Path to claims.csv file"

  run bash -c 'source "$1"; lumerad_claims_start_flags "$2" "$3"' _ \
    "$REPO_ROOT/devnet/scripts/common.sh" "$SHIM" "$WORKDIR/missing.csv"

  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}
