# EVM Migration Devnet Tests

This directory contains the source code for the `tests_evmigration` binary — a devnet testing tool for the `x/evmigration` module.

For the full guide (modes, Makefile targets, upgrade walkthrough, and module coverage), see:

**[docs/devnet-evmigration-tests.md](../../docs/devnet-evmigration-tests.md)**

## Multisig mode

Exercises the four-step multisig CLI flow (`generate-proof-payload` → `sign-proof` → `combine-proof` → `submit-proof`) against a freshly-seeded 2-of-3 secp256k1 multisig legacy account. The mode creates three signer keys, assembles the multisig composite key, funds it from `--funder`, issues a 1-ulume self-send to register the multisig pubkey on-chain, then runs the migration and verifies the resulting on-chain record and balance transfer. Run with:

```sh
tests_evmigration -mode=multisig -bin=lumerad -rpc=tcp://localhost:26657 \
                  -chain-id=lumera-devnet-1 -funder=validator0
```
