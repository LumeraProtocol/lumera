# Legacy Account Migration (`x/evmigration`)

The EVM integration changes coin type from 118 (`secp256k1`) to 60 (`eth_secp256k1`). Existing accounts derived with coin type 118 produce different addresses than the same mnemonic with coin type 60. The `x/evmigration` module provides a claim-and-move mechanism: users submit `MsgClaimLegacyAccount` signed by both old and new keys, atomically migrating on-chain state.

Module structure

```text
x/evmigration/
  keeper/
    keeper.go                      # Keeper struct, 9 external keeper deps
    msg_server.go                  # MsgServer wrapper
    msg_server_claim_legacy.go     # MsgClaimLegacyAccount handler
    msg_server_migrate_validator.go # MsgMigrateValidator handler (Phase 5)
    verify.go                      # Dual-signature verification
    migrate_auth.go                # Account record migration (vesting-aware)
    migrate_bank.go                # Coin balance transfer
    migrate_distribution.go        # Reward withdrawal
    migrate_staking.go             # Delegation/unbonding/redelegation re-keying
    migrate_authz.go               # Grant re-keying
    migrate_feegrant.go            # Fee allowance re-keying
    migrate_supernode.go           # Supernode account field update
    migrate_action.go              # Action creator/supernode update
    migrate_claim.go               # Claim destAddress update
    migrate_validator.go           # Validator record re-key (Phase 5)
    query.go                       # gRPC query stubs
    genesis.go                     # InitGenesis/ExportGenesis
  types/
    keys.go, errors.go, params.go, events.go, expected_keepers.go, codec.go
  module/
    module.go, depinject.go, autocli.go
```

### Messages

| Message                   | Signer                          | Purpose                           |
| ------------------------- | ------------------------------- | --------------------------------- |
| `MsgClaimLegacyAccount` | `new_address` (eth_secp256k1) | Migrate regular account state     |
| `MsgMigrateValidator`   | `new_address` (eth_secp256k1) | Migrate validator + account state |
| `MsgUpdateParams`       | governance authority            | Update migration params           |

### Params

| Param                         | Default  | Description                            |
| ----------------------------- | -------- | -------------------------------------- |
| `enable_migration`          | `true` | Master switch                          |
| `migration_end_time`        | `0`    | Unix timestamp deadline                |
| `max_migrations_per_block`  | `50`   | Rate limit                             |
| `max_validator_delegations` | `2000` | Max delegators for validator migration |

### Fee waiving

`ante/evmigration_fee_decorator.go` waives gas fees for migration txs (new address has zero balance before migration). Wired in `app/evm/ante.go` after `DelayedClaimFeeDecorator`.

### Migration sequence (MsgClaimLegacyAccount)

1. Pre-checks (params, window, rate limit, dual-signature verification).
   Legacy signature is`secp256k1_sign(SHA256("lumera-evm-migration:<legacy_address>:<new_address>"))`
2. Withdraw distribution rewards → legacy bank balance
3. Re-key staking (delegations, unbonding, redelegations + UnbondingID indexes)
4. Migrate auth account (vesting-aware: remove lock before bank transfer)
5. Transfer bank balances
6. Finalize vesting account at new address (if applicable)
7. Re-key authz grants
8. Re-key feegrant allowances
9. Update supernode account field
10. Update action creator/supernode references
11. Update claim destAddress
12. Store MigrationRecord, increment counters, emit event

### Queries

| Query                 | Description                         |
| --------------------- | ----------------------------------- |
| `Params`            | Current migration parameters        |
| `MigrationRecord`   | Single legacy address lookup        |
| `MigrationRecords`  | Paginated list of all records       |
| `MigrationEstimate` | Dry-run estimate of migration scope |
| `MigrationStats`    | Aggregate counters                  |
| `LegacyAccounts`    | Accounts needing migration          |
| `MigratedAccounts`  | Completed migrations                |

### Implementation status

| Phase | Description                     | Status      |
| ----- | ------------------------------- | ----------- |
| 1     | Proto + Types + Module Skeleton | Complete    |
| 2     | Verification + Core Handler     | Complete    |
| 3     | SDK Module Migrations           | Complete    |
| 4     | Lumera Module Migrations        | Complete    |
| 5     | Validator Migration             | Complete    |
| 6     | Queries + Genesis               | Complete    |
| 7     | Testing                         | In Progress |

---
