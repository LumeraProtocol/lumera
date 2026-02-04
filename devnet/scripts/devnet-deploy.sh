#!/usr/bin/env bash
set -e

DEVNET_ROOT="/tmp/lumera-devnet"
mkdir -p "$DEVNET_ROOT/shared"
if [ -f external_genesis.json ]; then
	cp external_genesis.json "$DEVNET_ROOT/shared/external_genesis.json"
else
	echo "No external genesis file found."
	return 1
fi
if [ -f claims.csv ]; then
	cp claims.csv "$DEVNET_ROOT/shared/claims.csv"
else
	echo "No claims file found."
	return 2
fi
if [ -f "supernode" ]; then
	mkdir -p "$DEVNET_ROOT/shared/release"
	cp "supernode" "$DEVNET_ROOT/shared/release/supernode"
else
	echo "No supernode binary found."
fi
docker compose build
