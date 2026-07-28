# Supernode Operator EVM Migration Guide

**Last updated**: 2026-06-16
**Applies to**: operators running a Lumera supernode against an EVM-enabled chain (post-EVM upgrade)
**Prerequisite reading**: [migration.md](migration.md) for the chain-level mechanics of legacy → EVM account migration

> **Mandatory operations gate:** Use the [EVM Migration Operator Runbook](operator-migration-runbook.md) for executable/keyring provenance, no-echo destination staging, config backup, concrete systemd/Docker/Kubernetes stop/restart evidence, the irreversible/retry boundary, and sanitized evidence requirements. This guide supplies the supernode-specific finalization steps.
>
> **Command context used below:** populate these values from the approved manifest and runbook discovery before any chain query. `LUMERAD` must be the exact absolute `artifacts.chain_executable.release_path`. Each explicit query below receives the supported home, keyring, chain, and node flags; keys commands receive only home and keyring flags. Do not use a universal wrapper. SuperNode and `sncli` operations must likewise use the independently pinned `artifacts.supernode_executable` and `artifacts.sncli_executable` records, never PATH lookup.
>
> ```bash
> LUMERAD=/absolute/path/from/approved/artifacts.chain_executable.release_path
> LUMERA_HOME=/absolute/discovered/lumera/home
> KEYRING_BACKEND=<test|file|os>
> KEYRING_DIR=/absolute/discovered/keyring/location
> CHAIN_ID=<approved-chain-id>
> TRUSTED_RPC=<trusted-rpc>
> ```

---

## Approved campaign path

This release campaign is manual. The SuperNode daemon's automatic startup-broadcast path is **NOT APPROVED by this runbook or release**. It has not completed a separate, exact supervisor-specific rehearsal covering preflight, stopped-state proof, the startup irreversible boundary, and ambiguous-outcome query-before-restart behavior. Do not add `evm_key_name` and restart a legacy SuperNode in order to trigger a migration broadcast, and do not treat daemon idempotence as permission to retry startup after an ambiguous result.

The approved single-signature procedure is:

1. Discover the SuperNode supervisor, service identity, absolute base/config/keyring paths, and exact manifest-pinned SuperNode, `sncli`, chain executable, and account-helper paths through the [Operator Runbook](operator-migration-runbook.md).
2. Use the manifest-approved PR-2 no-echo operation to pre-stage and prove the coin-type-60 destination key. If that dependency is blocked, stop.
3. Back up the discovered absolute SuperNode config, stop the workload through its discovered supervisor, and prove no duplicate process/container/pod can use the legacy key.
4. Run the post-stop dry-run and the live `migrate-account.sh` one-shot exactly once through [Operator Runbook §6](operator-migration-runbook.md#6-re-run-dry-run-verify-destination-broadcast-once). For a validator-bound SuperNode identity, follow [validator-migration.md](validator-migration.md) instead.
5. Query the migration record before any retry. Only after the record proves the expected destination, update the discovered absolute `config.yml` to the destination identity/key as required by the release, restart exactly one supervised workload for local cleanup, and run the authenticated `sncli` acceptance check in [Operator Runbook §8](operator-migration-runbook.md#8-finalize-restart-and-verify).

A Portal/Keplr migration may also create the on-chain record, but service finalization still uses the same discovered absolute paths, stopped-state proof, destination-address match, supervised restart, and authenticated verification. It is not an automatic-broadcast branch.

## SuperNode-specific configuration contract

The destination key must be `eth_secp256k1` at coin type 60 under a new name. Preserve the legacy `key_name` and `identity` until the one-shot migration is proven on-chain. After proof, apply the release-matched local finalization to the discovered absolute `config.yml`; never edit `~/.supernode/config.yml` under the interactive user or use a bare SuperNode key command.

Expected cleanup logs include an existing migration record, a matching destination address, and local configuration/key cleanup. Any destination mismatch is a hard stop: recover the exact key that controls the record's `new_address` or escalate; never broadcast again.

## Step 4 — Verify

Query the on-chain migration record:

```bash
sudo -u <supernode-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" evmigration migration-record <legacy-address>
```

The response should show `new_address` matching your EVM key's address. Also confirm the supernode's on-chain registration points at the new address:

```bash
# get-supernode takes the VALOPER address (lumeravaloper1…), not the account
# address. Convert your new account address with:
#   sudo -u <supernode-service-user> "$LUMERAD" keys --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" show <new-key> -a --bech val
sudo -u <supernode-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" supernode get-supernode <new-valoper-address>
```

Finally, confirm `config.yml` reflects the switch:

```bash
grep -E "key_name|identity|evm_key_name" /absolute/discovered/supernode/config.yml
```

You should see `key_name: <evm-key-name>`, `identity: <new-evm-address>`, and no `evm_key_name` line.

---

## Portal-created migration record

If Portal/Keplr creates the migration record, use [migration.md → Method 1](migration.md#method-1-portal--keplr-recommended) for the wallet flow, but use this guide's approved campaign path for the service. Prove the SuperNode stopped through its discovered supervisor, verify the record with the manifest-pinned chain executable, pre-stage the exact destination key through PR-2, and confirm its address matches the record. Then apply local finalization to the discovered absolute config and restart through the supervisor **only for cleanup**. A mismatch or ambiguous record is a hard stop.

---

## Troubleshooting

### `evm_key_name "<name>" is not an eth_secp256k1 key`

The destination key has the wrong algorithm. Remove it only through the manifest-approved key operation and repeat the approved PR-2 no-echo prestage with `eth_secp256k1`; do not use a bare recovery shortcut.

### `simulation failed: rpc error: ... invalid length: tx parse error`

The supernode binary is older than the chain's `x/evmigration` proto schema. Upgrade to a supernode build that includes the `LegacyProof` refactor (single-sig sends `LegacyProof{Single: SingleKeyProof{…}}` instead of the retired flat `legacy_pub_key`/`legacy_signature` fields).

### `connected Lumera chain does not have EVM support`

The chain hasn't run the EVM upgrade yet. This supernode binary is post-EVM-only — run the older pre-EVM binary, or wait for the chain upgrade.

### `migration record exists on-chain but new address mismatch`

Someone completed migration with a different EVM key than the one now in your `evm_key_name` config. Either:

- Use the EVM key that actually signed the original migration (re-recover it with the mnemonic that was used), or
- Investigate whether the on-chain `new_address` is correct — it's the authoritative record.

---

## FAQ

**Q: Do I have to migrate on day 1 of the EVM upgrade?**
No — unless governance sets a deadline via the `migration_end_time` param. In practice you migrate when you upgrade the binary, since the new binary is EVM-only.

**Q: Will my supernode lose its ranking / history across the migration?**
No. The migration re-keys the on-chain record: your supernode registration, evidence history, and metrics carry over under the new address. `x/evmigration` transfers all referenced state atomically.

**Q: My supernode runs as both a validator operator and a supernode. Do I migrate twice?**
No — a single `MsgMigrateValidator` re-keys both the validator operator record and the supernode record bound to it. See [validator-migration.md](validator-migration.md) for the validator-specific walkthrough (including the maintenance window and the `max_validator_delegations` check); the supernode side happens as a side-effect of that tx.

**Q: Can I roll back if the migration fails mid-flight?**
No. Before broadcast, correct inputs and repeat the dry-run. After a tx hash or ambiguous failure, query the tx and migration record and escalate before any retry. Restart only after the record proves the expected destination, and only for supervised local cleanup; never restart to retry automatic broadcast.

---

## Multisig supernode accounts

This section only applies if your on-chain supernode operator account is a flat K-of-N multisig (`LegacyAminoPubKey`). If your supernode was set up normally with a single-sig key, **you don't need this section** — follow steps 1–4 above.

The new operator account is **also** a K-of-N multisig, constructed from `eth_secp256k1` sub-keys (see the [mirror-source rule](../evmigration/main.md#multisig-account-migration) in `evmigration/main.md`). The ceremony described below produces that new multisig, builds a dual-side proof, and broadcasts it.

> **Consensus invariants (multisig).** The chain rejects a multisig supernode-operator migration tx at `ValidateBasic` if any of these is violated:
>
> - **Shape + K/N mirror.** K-of-N legacy → K-of-N new, same K and same N (`ErrMirrorSourceMismatch`).
> - **Matching `signer_indices`.** The same K signer positions approve both halves.
> - **Sub-key uniqueness.** No duplicate entries in either side's `sub_pub_keys` list.
> - **Zero-signer submit.** `submit-proof` takes no `--from`, no fee flags, no envelope signature.
>
> Full reference: [legacy-migration.md § Consensus invariants](../evmigration/legacy-migration.md#consensus-invariants).

### Why automatic migration is refused

The daemon's refusal directs operators to the offline multisig ceremony. That diagnostic is not an approved command source: do not copy any abbreviated command embedded in daemon output. Use the release manifest and [Operator Runbook](operator-migration-runbook.md) to pin the exact absolute `lumerad` path and every home/keyring/chain/node input, then follow the proof semantics below without bypassing that execution context.

### Multisig flow overview

You complete the 4-step offline ceremony with `lumerad`, then restart the supernode — the daemon detects the on-chain migration record and finishes local cleanup through its idempotent path.

#### Step 1 — Generate N fresh `eth_secp256k1` sub-keys and derive the new multisig

Each co-signer generates a destination-side eth sub-key in the exact keyring established by the operator runbook. The coordinator collects the N public keys and derives the destination composite with the same K, N, and member order as the legacy multisig. Use the manifest-pinned absolute `lumerad` path and explicit home, keyring backend, and keyring location for every key operation; do not use a bare `lumerad keys` shortcut.

> **`--nosort` is required, and member order must mirror the legacy side.** The coordinator must inspect the legacy multisig through the same manifest-pinned binary and trusted node, then list destination members in that exact order so signer indices match on both sides.

This replaces the old single-EOA "recover the new EVM key" step: the destination is a multisig derived from fresh eth sub-keys, not an EOA recovered from a mnemonic.

Set `evm_key_name` in the supernode's `config.yml` to the name of the new multisig key (`<op-name>-msig-new` in the example above) — the daemon will detect this during the post-migration restart and run cleanup.

#### Step 2 — Ensure the multisig's pubkey is on-chain

If the multisig has received funds but never signed a transaction, its `LegacyAminoPubKey` is nil on-chain and proof generation fails. Complete the required pubkey-registration transaction through the runbook's manifest-pinned execution context, then query the account with the same absolute binary, explicit home/keyring context, chain ID, and trusted node. The response must show all N legacy sub-keys.

#### Step 3 — Coordinator generates the proof payload template

The coordinator generates `proof.json` with the manifest-pinned absolute chain executable, explicit home/keyring backend/keyring location, exact chain ID, trusted node, legacy address, destination public keys in legacy order, matching threshold, and `claim` proof kind. Do not copy an abbreviated proof-generation command from this guide.

- Destination sub-public-keys may be key names in that explicit keyring or base64 compressed 33-byte `eth_secp256k1` public keys.
- The destination threshold is required and must match the legacy K.
- Use `claim` for `MsgClaimLegacyAccount`; a validator operator must instead follow the validator guide.
- The exact chain ID is embedded in every signature.

Distribute the resulting `proof.json` to all co-signers through the approved custody channel.

#### Step 4 — Each co-signer signs both sides in one invocation

Every participating co-signer must hold both their legacy Cosmos sub-key and destination-side eth sub-key at matching indices. Each signs both sides with the manifest-pinned absolute chain executable, their explicit home/keyring backend/keyring location, the same chain ID and trusted node, and a distinct partial-output path. Do not use a bare `sign-proof` example that can silently select another keyring. Re-signing replaces that participant's prior entries rather than duplicating them; return partials through the approved custody channel.

#### Step 5 — Coordinator combines partials

The coordinator combines the reviewed partial files with the same manifest-pinned absolute executable and explicit home/keyring/chain/node context, writing a single `tx.json`. Do not use an abbreviated combine command from this guide.

`combine-proof` rejects the set if any two partials disagree on `chain_id`, `evm_chain_id`, `legacy_address`, `new_address`, `payload_hex`, proof kind, or either side's `sub_pub_keys` list. It verifies every merged partial on both legacy and new sides, drops invalid entries with a stderr warning, then **intersects** the valid signer-index sets across the two sides and selects the first K indices present on BOTH. This is what guarantees `legacy_proof.signer_indices == new_proof.signer_indices`, the consensus-level mirror-source rule. A one-sided partial (e.g. a co-signer who signed only the legacy half because they lost their eth sub-key) does not contribute toward quorum unless another co-signer supplied the new-side signature at the same index. If the intersection has fewer than K entries, combine-proof errors with `need <K> valid partial signatures signed on BOTH sides at matching indices, have <N>` and writes nothing.

#### Step 6 — Coordinator submits the pre-assembled tx

Submission is the irreversible boundary. Return to [Operator Runbook §6](operator-migration-runbook.md#6-re-run-dry-run-verify-destination-broadcast-once), prove the relevant workload stopped, and submit `tx.json` exactly once with the manifest-pinned absolute chain executable, explicit home/keyring backend/keyring location, exact chain ID, and trusted node used throughout the ceremony. `submit-proof` has no `--from`, fee, or envelope-signature flags. After a tx hash or ambiguous transport failure, apply the runbook's query-before-retry rule; verify the migration record through its manifest-pinned query form.

#### Step 7 — Restart the supernode (local cleanup only)

The daemon detects the on-chain migration record, confirms its `new_address` matches the multisig bech32 derived from the `evm_key_name` you configured in Step 1, skips the broadcast step (idempotent), rewrites `config.yml` (`key_name` → new multisig key name, `identity` → new multisig bech32, clears `evm_key_name`), and deletes the old legacy multisig composite from the keyring.

Expected logs on the cleanup restart:

```text
INFO  EVM module detected on chain
WARN  Legacy secp256k1 key detected — EVM account migration required
INFO  Account already migrated on-chain, skipping broadcast
INFO  New address confirmed as registered supernode
INFO  EVM migration complete — legacy key removed, config updated
```

### Why the new operator is not EVM-addressable

The new operator account is a Cosmos SDK multisig bech32 derived from `kmultisig.NewLegacyAminoPubKey` over N `eth_secp256k1` sub-keys. It is **not** an Ethereum 20-byte address. This is a non-goal, not a limitation:

- The new operator can perform **all** Cosmos-side operations required for supernode life-cycle: `MsgEditSupernode`, validator edits (if applicable), `x/staking` delegations, `x/distribution` withdrawals, `x/authz` grants, and IBC transfers. Every supernode-relevant workflow continues to work.
- The new operator **cannot** originate `MsgEthereumTx` — multisig bech32 addresses are not valid senders for EVM transactions, and there is no way to produce a single ECDSA signature that authenticates K-of-N.

Operators who want EVM DeFi access for rewards may configure a separate single-EOA withdraw address after migration. This is a new signed transaction, not part of the migration ceremony; execute it only through the same manifest-pinned absolute chain binary and explicit home/keyring backend/keyring location/chain ID/trusted node context, following the platform's complete multisig signing and broadcast procedure. No abbreviated command is provided here.

### Post-migration cleanup

The daemon's idempotent cleanup path detects the on-chain multisig `BaseAccount.PubKey` (set by `MigrateAuth`) and treats it as the canonical record of "the operator has migrated". No workflow change is required from the operator beyond the restart in Step 7 — the daemon does not need to "know" that the new operator is a multisig; it simply confirms that the on-chain `new_address` matches the address derived locally from `evm_key_name` and runs cleanup.

### Migration order relative to sub-signer personal migrations

Supernode operators whose operator key is a multisig often ask whether they need to coordinate their personal account migrations with the multisig's migration ceremony. They do not: sub-signer and multisig migrations are mutually independent. See the "Migration order — FAQ" in [evmigration/main.md](../evmigration/main.md#migration-order--faq) for the full explanation; the short version is that any order works, including interleaved, and a sub-signer's personal migration never affects the multisig's ability to migrate later.

### Multisig troubleshooting

**`sub-sig 0 (signer lumera1…) invalid: legacy signature verification failed`** — one of the partial signatures didn't verify under its declared sub-pub-key. Most common causes:

- `--chain-id` differed between `generate-proof-payload` and what the chain uses (the chain-id is embedded in the signed payload).
- A co-signer edited `proof.json` between `generate-proof-payload` and `sign-proof`.
- Wrong legacy sub-key used by a signer (`--from` pointed at a key that isn't one of the legacy multisig members), or wrong destination sub-key (`--new-key` pointed at a key not in `--new-sub-pub-keys`).

Regenerate `proof.json` with the correct `--chain-id`, have the affected signer re-run `sign-proof`, then re-combine.

**`sub-sig N (signer lumera1…) invalid: new signature verification failed`** — symmetric failure on the destination side. Typically the signer used the wrong `--new-key` (not the eth sub-key they claimed during `generate-proof-payload`) or their eth sub-key isn't actually one of the entries in `--new-sub-pub-keys`. Fix the `--new-key` value and re-run `sign-proof` for that signer.

**The multisig account was migrated but the supernode still starts the automatic flow** — check that the on-chain record's `new_address` exactly matches the multisig bech32 of the `evm_key_name` configured in the supernode keyring. If they differ, the daemon won't detect the already-migrated state and will try to broadcast fresh. Align `evm_key_name` with the multisig key that was actually used during the offline ceremony.

**What if I only have K−1 of the sub-keys available on the legacy side?** — you can't complete migration. The K-of-N threshold is enforced by the keeper (`need <K> valid partial signatures, have <N>`). Recover the missing legacy sub-key(s) from their mnemonics, or coordinate with the actual holders.

**What if only K−1 co-signers have provided eth sub-keys for the destination side?** — same situation, symmetric: you need K valid new-side partials. Have the missing co-signer(s) generate their eth sub-key (`sudo -u <supernode-service-user> "$LUMERAD" keys --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" add ... --key-type eth_secp256k1`), rebuild `proof.json` via `generate-proof-payload` with the full `--new-sub-pub-keys` list, and re-sign.

**`legacy key "..." is signer index N, but new key "..." is signer index M; multisig migration requires the same signer position to approve both halves`** — raised by `sign-proof` when a co-signer passes both `--from` and `--new-key` in one call but the two keys occupy *different* positions in their respective multisigs. Each co-signer must hold the **same signer index** on the legacy and new sides (the consensus mirror-source rule requires `legacy_proof.signer_indices == new_proof.signer_indices`). The usual root cause is a destination multisig built without `--nosort` (so `keys add` sorted the sub-keys) or with a member order that doesn't mirror the legacy `public_keys`. Recreate `<op-name>-msig-new` with `--nosort`, listing the eth sub-keys in the same member order as the legacy multisig (`sudo -u <supernode-service-user> "$LUMERAD" query --home "$LUMERA_HOME" --keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KEYRING_DIR" --chain-id "$CHAIN_ID" --node "$TRUSTED_RPC" auth account <multisig-legacy-address>`), then regenerate `proof.json` and re-sign.

**The supernode's embedded error message says `assemble-proof` but the CLI has `combine-proof`. Which is correct?** — the CLI command is `combine-proof`. Any older embedded error message in the supernode binary is stale; use this guide's commands.

---

## Related documentation

- [migration.md](migration.md) — chain-level end-user migration guide (Portal + Keplr, shell scripts, raw CLI)
- [migration-scripts.md](migration-scripts.md) — reference for the bundled `migrate-account.sh` / `migrate-validator.sh` shell helpers (flags, exit codes, troubleshooting)
- [validator-migration.md](validator-migration.md) — validator operator migration guide (maintenance window, `max_validator_delegations` check, consensus key handling)
- [legacy-migration.md](../evmigration/legacy-migration.md) — `x/evmigration` module architecture, proto shapes, keeper logic, and the full reference for the offline proof flow
- [node-evm-config-guide.md](node-evm-config-guide.md) — post-upgrade `app.toml` / RPC configuration for full nodes and validators
