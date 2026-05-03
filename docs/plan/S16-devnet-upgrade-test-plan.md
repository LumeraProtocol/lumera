# Slice S16 — Devnet upgrade-test path for v1.11.1 → v1.12.0

**Status**: planned (awaiting approval)
**Date**: 2026-05-01
**Tag context**: v1.12.0
**Verifies**: F18 (Everlight upgrade handler), AT42, AT43

## Goal

Enable a manually-triggered devnet flow that bootstraps v1.11.1, executes a real
governance upgrade to v1.12.0, then runs the existing Everlight scenarios on the
upgraded chain and asserts upgrade-handler outcomes (AT42 + AT43).

Mirrors the existing pinned-version upgrade pattern in `Makefile.devnet:303-330`
(`devnet-build-191` / `devnet-new-191` / `devnet-upgrade-1100` / `devnet-upgrade-1101`).
Same `devnet-build-XXX` / `devnet-new-XXX` / `devnet-upgrade-XXX` triple, same one-line
`upgrade.sh` invocation. No new orchestration scaffolding is required — the existing
`devnet/scripts/upgrade.sh` already handles proposal submission, deposit, voting via
`vote-all.sh`, height-wait, and binary swap via `upgrade-binaries.sh`.

## In scope

### 1. One-time bootstrap (executed by implementer; not codified in the repo)

Spin up a clean v1.11.1 chain via the existing `devnet-build-191`-style flow but with
binaries built from a fresh checkout of git tag `v1.11.1`. Let it produce a few blocks.
Run `lumerad export` from a validator container. Commit the resulting genesis as:

```
devnet/default-config/devnet-genesis-v1.11.1.json
```

Rationale: the current `devnet/default-config/devnet-genesis.json` already has v1.12.0-only
state baked in (`audit.params.storage_truth_*`, `supernode.params.reward_distribution`,
`supernode.last_distribution_height`) which a v1.11.1 binary will reject. Hand-trimming is
fragile because the audit-module genesis shape is in flux (see uncommitted
`x/audit/v1/module/genesis.go` working-tree changes); exporting from a real v1.11.1 chain
is the safest source.

### 2. Stage `devnet/bin-v1.11.1/`

Contents (mirrors existing `bin-v*` dirs):
- `lumerad`
- `libwasmvm.x86_64.so`
- `tests_validator`
- `tests_hermes`

All built from the v1.11.1 git tag. Document refresh procedure in `devnet/Readme.md`.

### 3. `Makefile.devnet` additions

Append in the existing pinned-version block at `:303-330`, mirroring the 1101 pattern
exactly:

```make
.PHONY: devnet-build-1111 _check-devnet-1111-cfg devnet-new-1111 devnet-upgrade-1120 devnet-tests-everlight-upgrade

DEFAULT_GENESIS_FILE_V1111 := devnet/default-config/devnet-genesis-v1.11.1.json

devnet-build-1111:
	@$(MAKE) devnet-build \
		DEVNET_BUILD_LUMERA=0 \
		DEVNET_BIN_DIR=devnet/bin-v1.11.1 \
		CONFIG_JSON=config/config-no-hermes.json \
		EXTERNAL_GENESIS_FILE="$$(realpath $(DEFAULT_GENESIS_FILE_V1111))" \
		EXTERNAL_CLAIMS_FILE="$$(realpath $(DEFAULT_CLAIMS_FILE))"

_check-devnet-1111-cfg:
	@[ -f "$$(realpath $(DEFAULT_GENESIS_FILE_V1111))" ] || (echo "Missing v1.11.1 genesis"; exit 1)
	@[ -f "$$(realpath $(DEFAULT_CLAIMS_FILE))" ] || (echo "Missing claims file"; exit 1)
	@[ -d devnet/bin-v1.11.1 ] || (echo "Missing devnet/bin-v1.11.1; see devnet/Readme.md"; exit 1)

devnet-new-1111:
	$(MAKE) devnet-down
	$(MAKE) devnet-clean
	$(MAKE) devnet-build-1111
	sleep 10
	$(MAKE) devnet-up-detach

devnet-upgrade-1120:
	@cd devnet/scripts && ./upgrade.sh v1.12.0 auto-height ../bin

devnet-tests-everlight-upgrade:
	$(MAKE) devnet-new-1111
	@# wait for v1.11.1 to produce blocks, snapshot pre-upgrade state
	@bash devnet/tests/everlight/_pre_upgrade_snapshot.sh
	$(MAKE) devnet-upgrade-1120
	@# wait for upgrade height + a few blocks of post-upgrade settling
	@LUMERA_UPGRADED_FROM=v1.11.1 bash devnet/tests/everlight/everlight_test.sh
```

Note: `devnet-upgrade-1120` uses `../bin` (not `../bin-v1.12.0`) because v1.12.0 is the
current head version — same convention as `devnet-upgrade-1101` today.

### 4. Pre-upgrade snapshot helper

New file `devnet/tests/everlight/_pre_upgrade_snapshot.sh` that captures pre-upgrade
chain state into `/tmp/lumera-devnet-1/shared/pre_upgrade_snapshot.json`:

- `supernode.params` (full object) — used by S9.2 to confirm `reward_distribution` is
  absent before the upgrade.
- Current SN states (`lumerad query supernode list-supernodes`) — used by S9.3 to
  confirm states survive the upgrade unchanged.
- Current applied-upgrade list (`lumerad query upgrade applied`) — used by S9.1 to
  diff against the post-upgrade list.

The snapshot is keyed on chain height at capture time so post-upgrade comparisons can
verify the snapshot was taken before the upgrade height (defensive against orchestration
ordering bugs).

### 5. Fill `scenario_stubs` in `devnet/tests/everlight/everlight_test.sh:1909-1918`

All gated on the `LUMERA_UPGRADED_FROM` environment variable. When unset, S9/S10 stay
`skip` exactly as they do today, so existing `make devnet-tests-everlight` runs are
unaffected.

| ID    | Check                                                                                          | Mapped AT |
| ----- | ---------------------------------------------------------------------------------------------- | --------- |
| S9.1  | `lumerad query upgrade applied "v1.12.0"` returns non-zero height                              | (sanity)  |
| S9.2  | `supernode.params.reward_distribution` absent pre, default-shaped post                         | AT42      |
| S9.3  | SN-state set in pre-snapshot survives unchanged across upgrade height                          | AT43      |
| S9.4  | `audit.params.storage_truth_enforcement_mode == STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW` post    | (LEP-6 guardrail) |
| S10   | End-to-end Cascade register + finalize via `tests_validator -test.run TestLEP5CascadeAvailabilityCommitment` | (lifecycle) |

S9.4 is the LEP-6 default-flip guardrail: if a future change accidentally flips the
default storage-truth enforcement mode out of SHADOW, this scenario fails before the
release ships, even though full LEP-6 functional tests live outside BRIDGE.

## Out of scope

- LEP-6 functional devnet tests — handled by other devs outside BRIDGE.
- IBC / Hermes / relayers (S16 uses `CONFIG_JSON=config/config-no-hermes.json`).
- Phase 4 / F17 (block-reward routing).
- Any chain-side code changes — this is pure devnet/test plumbing.
- Backfilling `devnet-upgrade-1110` / `devnet-upgrade-1111` for the v1.10.1 → v1.11.0 →
  v1.11.1 chain. The gap exists but is orthogonal to the v1.12.0 tag.

## Acceptance

- `make devnet-tests-everlight-upgrade` runs green end-to-end on a clean machine.
- AT42 demonstrated via S9.2 evidence (`reward_distribution` defaults appear after the
  upgrade plan applies).
- AT43 demonstrated via S9.3 evidence (pre-upgrade SN states unchanged post-upgrade).
- `make devnet-tests-everlight` (the existing target, no upgrade) continues to pass —
  S9/S10 stay skipped because `LUMERA_UPGRADED_FROM` is unset.

## Requirements impact

Stays inside `requirements.json` `scope.in_scope` — verifies the already-shipped F18
upgrade handler. No requirements.json change needed.

## Risks

- **`start.sh` / `supernode-setup.sh` may issue v1.12.0-only queries during bootstrap.**
  Examples to look for: `lumerad query supernode pool-state`,
  `lumerad query supernode reward-distribution-state`, any `RewardDistribution` field
  reads. If present, they will fail against a v1.11.1 binary and break
  `devnet-new-1111`. The architect should grep both scripts up front and fix-on-find
  rather than design around the hypothetical.

- **Working tree must be clean before staging the v1.12.0 binary.**
  The repo currently has uncommitted LEP-6 audit module changes that must NOT end up
  baked into the v1.12.0 binary. `make build` reads the working tree, so the
  implementer must build v1.12.0 from a clean tree (stash or worktree) before staging
  into `devnet/bin/`. Operational gate, not a code task.

- **Genesis-export procedure is one-time and untracked.**
  Re-running it would overwrite the committed fixture. Documented as a manual
  procedure in `devnet/Readme.md`, not a Make target.

## Delegation plan (post-approval)

1. **bridge-architect** — design the thin contract:
   - Pre-upgrade snapshot JSON shape (fields, capture-time metadata, file path).
   - Scenario S9.x / S10 entry contract (env var, snapshot path, exit codes,
     pass/fail/skip mapping into the existing `RESULTS` array).
   - Grep `devnet/scripts/start.sh` and `devnet/scripts/supernode-setup.sh` for
     v1.12.0-only queries; report findings before coder begins.
   - Output: short architecture note in `docs/contracts/` or appended to this plan.

2. **bridge-coder** — implementation:
   - `Makefile.devnet` targets (Section 3 above).
   - `devnet/tests/everlight/_pre_upgrade_snapshot.sh`.
   - `scenario_stubs` replacement in `everlight_test.sh:1909-1918`.
   - `devnet/Readme.md` refresh procedure for `devnet-genesis-v1.11.1.json` and
     `devnet/bin-v1.11.1/`.
   - Any fixes required by the architect's `start.sh` / `supernode-setup.sh` grep.

3. **Manual operator step** (cannot be delegated):
   - Bootstrap v1.11.1, export genesis, commit `devnet-genesis-v1.11.1.json`.
   - Build v1.11.1 binaries from the tag, stage `devnet/bin-v1.11.1/`.
   - Build v1.12.0 binary from a clean working tree, stage `devnet/bin/`.

4. **bridge-auditor** — gate run after coder + manual prep complete.

## Open questions

None. All earlier questions resolved:
- Scope: S16 only (LEP-6 devnet tests dropped — other devs outside BRIDGE).
- Hermes: out (no-hermes config used).
- Genesis source: export from a real v1.11.1 chain.
- Working-tree changes: do not land in v1.12.0 (tag is cut from `451f8a8` with a clean
  working tree at build time).
