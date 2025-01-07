#!/usr/bin/env bash
set -e

mkdir -p /tmp/pastel-devnet/shared
cp external_genesis.json /tmp/pastel-devnet/shared/external_genesis.json
docker compose build
