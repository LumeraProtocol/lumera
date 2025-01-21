#!/usr/bin/env bash
set -e

mkdir -p /tmp/lumera-devnet/shared
cp external_genesis.json /tmp/lumera-devnet/shared/external_genesis.json
cp claims.csv /tmp/lumera-devnet/shared/claims.csv
docker compose build
