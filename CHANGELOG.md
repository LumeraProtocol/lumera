# Changelog



## 1.7.2 

Changes included since `v1.7.0` (range: `v1.7.0..HEAD`).

Added
- On-chain upgrade handler `v1.7.2` wired and registered; migrations only, no store key changes (app/upgrades/v1_7_2/upgrade.go; app/app.go).
- Supernode account history recorded on register/update (proto/lumera/supernode/supernode_account_history.proto; x/supernode/v1/keeper/msg_server_update_supernode.go).
- Supernode messages support `p2p_port` (update and register) with keeper handling (proto/lumera/supernode/tx.proto; x/supernode/v1/keeper/msg_server_update_supernode.go; x/supernode/v1/keeper/msg_server_register_supernode.go).
- Action metadata adds `public` flag (proto/lumera/action/metadata.proto; x/action/v1/types/metadata.pb.go).

Changed
- Supernode type field `version` renamed to `note` in chain types and handlers (proto/lumera/supernode/super_node.proto; x/supernode/v1/types/super_node.go; x/supernode/v1/types/message_update_supernode.go).
- Supernode state transitions and event attributes standardized across keeper and msg servers (x/supernode/v1/keeper/supernode.go; x/supernode/v1/keeper/hooks.go; x/supernode/v1/types/events.go).


Fixed
- Supernode staking hooks correctness for eligibility-driven activation/stop (x/supernode/v1/keeper/hooks.go).
- Action fee distribution panic avoided (x/action/v1/module/module.go).

CLI
- Supernode CLI:
  - Added query: `get-supernode-by-address [supernode-address]` (x/supernode/v1/module/autocli.go).
  - Standardized command names: `get-supernode`, `list-supernodes`, `get-top-supernodes-for-block` (x/supernode/v1/module/autocli.go).
  - `update-supernode` switched positional arg from `version` to `note`; added optional `--p2p-port` flag. `register-supernode` also supports optional `--p2p-port` (x/supernode/v1/module/autocli.go).
- Action CLI:
  - Added `action [action-id]` query (x/action/v1/module/autocli.go).
  - `finalize-action` now takes `[action-id] [action-type] [metadata]` (x/action/v1/module/autocli.go).
- Testnet CLI: default denom set to `ulume` for gas price and initial balances (cmd/lumera/cmd/testnet.go).

## 1.7.0 

Added
- SuperNode Dual-Source Stake Validation: eligibility can be met by self-delegation plus supernode-account delegation (x/supernode/v1/keeper/supernode.go: CheckValidatorSupernodeEligibility).

Changed
- App wiring and upgrade handler for `v1.7.0` (migrations only; no store upgrades) (app/upgrades/v1_7_0/upgrade.go; app/app.go).
