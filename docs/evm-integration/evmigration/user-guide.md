# EVM Legacy Account Migration - User Guide

**Last updated**: 2026-04-02
**Applies to**: Lumera chain with `x/evmigration` module enabled (post-EVM upgrade)

---

## Why Migration Is Needed

The Lumera chain upgraded from a standard Cosmos SDK chain to an EVM-compatible chain. This changed the underlying cryptography used for account addresses:

- **Before the upgrade (legacy)**: accounts used **coin-type 118** with `secp256k1` keys and Cosmos-style address hashing (`ripemd160(sha256(pubkey))`)
- **After the upgrade (EVM)**: accounts use **coin-type 60** with `eth_secp256k1` keys and Ethereum-style address hashing (`keccak256(pubkey)[12:]`)

Because the address derivation changed, the same mnemonic now produces a **different Lumera address**. Your funds, delegations, and other on-chain state remain at the old (legacy) address. Migration moves all of that state to your new EVM-compatible address.

### What Gets Migrated

Migration transfers **all** on-chain state from your legacy address to your new address in a single atomic transaction:

- **Bank balances** (all denominations)
- **Staking delegations** (active delegations to validators)
- **Unbonding delegations**
- **Redelegations**
- **Authz grants** (both as granter and grantee)
- **Feegrant allowances** (both as granter and grantee)
- **Action records** (creator and supernode references)
- **Claim records**
- **Supernode registration** (if applicable)
- **Vesting schedules** (if applicable)

For **validators**, migration additionally re-keys:

- Validator operator address
- All delegations pointing to the validator (from all delegators)
- Validator distribution state (commission tracking)
- Supernode record tied to the validator

### What Happens to the Legacy Account

After migration:

- The legacy account is removed from the auth module
- All balances are transferred to the new address (legacy balance becomes 0)
- A migration record is created on-chain linking the legacy and new addresses
- The legacy address cannot be migrated again

### Important Notes

- Migration is **irreversible** - once completed, it cannot be undone
- Migration is **fee-free** - no LUME is required on either address to submit the transaction
- Both addresses must come from the **same mnemonic** (same seed phrase)
- The migration transaction is unsigned at the Cosmos tx layer; authentication is embedded in the message as dual cryptographic proofs

---

## Method 1: Portal + Keplr (Recommended)

This is the easiest method. The Lumera Portal provides a guided wizard that handles address derivation, signing, and broadcasting.

### Prerequisites

- [Keplr browser extension](https://www.keplr.app/) installed
- Your mnemonic (recovery phrase) imported in Keplr

### Step-by-Step Guide

#### 1. Connect Your Wallet and Check Migration Status

Navigate to the Lumera Portal and go to the **Claim** page. The **EVM Account Migration** section appears automatically when the chain has the `x/evmigration` module enabled.

Click **Connect Keplr**. If the Lumera chain is not yet added to Keplr, the portal will prompt you to approve it (screenshot 9 below shows this dialog).

After connecting, the portal automatically checks your wallet address against the chain and shows your migration status. If you have a legacy (coin-type 118) account with on-chain state, you will see the green **"Ready to Migrate"** badge with a summary of your assets:

![Portal main page showing legacy account ready for migration](assets/evmigration-1.jpg)

The status panel shows:

- **Balance** — your available LUME balance
- **Delegations** — active staking delegations
- **Unbonding** — pending unbonding entries
- **Authz Grants / Feegrants** — authorization and fee grant counts
- **Supernode** — whether this account runs a supernode

The top progress bar shows the overall migration progress across all accounts on the chain. Click **START MIGRATION WIZARD** to begin.

#### 2. Step 1: Review

The wizard opens with a review of what will be migrated. Verify that all the information is correct:

![Step 1: Review — eligibility, addresses, and balance summary](assets/evmigration-2.jpg)

Key things to check:

- **"Eligible for migration"** badge at the top (green) with your account type (Standard Account or Validator)
- **Legacy Address (coin-type 118)** — your current Lumera address, shown in cyan
- **New Address (coin-type 60)** — your destination address, shown in both Lumera bech32 and Ethereum hex format. This address is derived automatically from Keplr's Ethereum provider using the same mnemonic
- **Balance, Delegations, Unbonding, Authz/Feegrant, Supernode** — summary of all state that will be moved

The note at the bottom reminds you that both addresses must come from the **same mnemonic**, derived on different coin-type paths (118 to 60).

If you need to migrate a different account, expand **"Check a different legacy address"** at the bottom.

**For validators**: additional pre-migration confirmations appear here — you must confirm your maintenance window is planned, your node is stopped, and you have copied the post-migration restart commands.

Click **NEXT** when ready.

#### 3. Step 2: Sign & Confirm

This step collects two cryptographic proofs that authenticate you as the owner of both the legacy and new addresses. No private keys leave your device — all signing happens locally in Keplr.

![Step 2: Sign & Confirm — proofs needed](assets/evmigration-3.jpg)

Click the **SIGN MIGRATION PROOFS** button. Keplr will open **two signature popups** in sequence:

**First popup — Legacy proof (ADR-036 signArbitrary):**

![Keplr signature request for legacy proof — ADR-036 format](assets/evmigration-4.jpg)

This is the legacy account proof. Notice:

- **"Signing with"** shows your Keplr wallet name (e.g., "pre-evm-acc")
- **"on lumera-devnet"** — the Lumera chain
- **"with lumera1qnue33..."** — your legacy address
- **Message** contains the migration payload: `lumera-evm-migration:{chainID}:{evmChainID}:claim:{legacyAddr}:{newAddr}`
- **Advanced** section shows the full ADR-036 JSON sign doc with `sign/MsgSignData` — this is the standard Cosmos arbitrary message format

Click **Approve** to sign with your legacy key.

**Second popup — New proof (EIP-191 personal_sign):**

![Keplr signature request for new proof — Ethereum personal_sign](assets/evmigration-5.jpg)

This is the new address proof. Notice the differences:

- **"on Ethereum"** — this time Keplr uses its Ethereum signing provider, not the Cosmos one
- **"with 0x9a56927056..."** — your Ethereum hex address (the EVM-compatible address)
- **Message** contains the same migration payload as above

Click **Approve** to sign with your new (coin-type 60) key.

After both signatures are collected, the portal shows green checkmarks next to each proof:

![Step 2 completed — both proofs signed, ready to migrate](assets/evmigration-6.jpg)

Both proofs are now signed:

- **Legacy proof (ADR-036 signArbitrary)** — green checkmark
- **New proof (EIP-191 personal_sign)** — green checkmark

The transaction summary shows the **From** (legacy) and **To** (new) addresses, and confirms **Fee: None (fee-free)**.

Check the **"I understand this is irreversible and all on-chain state will move to my new address"** confirmation checkbox. Then click **MIGRATE**.

#### 4. Migration Result

The portal broadcasts the transaction and waits for confirmation (typically one block, 5-6 seconds). On success:

![Migration Successful — post-migration checklist and transaction hash](assets/evmigration-7.jpg)

The result screen shows:

1. **New Lumera address** — your new bech32 address with a copy button
2. **Ethereum hex address** — your 0x-prefixed address with a copy button
3. **Switch to the new Lumera chain definition in Keplr** — instructions to add the coin-type 60 chain definition (see next section)
4. **Transaction hash** — the on-chain tx hash for verification

**For validators**: an urgent section shows the restart command (`systemctl start lumerad`). Restart your validator promptly to avoid missed blocks and jailing.

Click **DONE** to close the wizard. The main page now shows your migration record and updated progress counters:

![Post-migration main page — migration record visible, Keplr still on old derivation](assets/evmigration-8.jpg)

However, notice that Keplr is still using the old coin-type 118 chain definition. The next step switches it to the new EVM-compatible definition.

#### 5. Switch to the New Lumera Chain Definition in Keplr

After migration, your on-chain state lives at the new coin-type 60 address, but Keplr still has the old Lumera chain definition (coin-type 118) cached. You need to add the **new Lumera chain definition** (coin-type 60, EVM-compatible) to Keplr.

The chain registry provides two Lumera definitions for this purpose:

- **Lumera (Legacy)** — coin-type 118, `secp256k1` — the pre-migration chain definition that existing users already have in Keplr
- **Lumera** — coin-type 60, `eth_secp256k1`, EVM features enabled — the post-migration chain definition

The Portal prompts you to add the new chain definition via Keplr's `suggestChain` mechanism:

![Keplr suggest chain dialog — adding Lumera with coin-type 60 and EVM features](assets/evmigration-9.jpg)

Review the chain configuration — you should see coin-type 60 and Ethereum-compatible settings — and click **Approve**. This adds the new Lumera definition to Keplr alongside the legacy one.

After approving, disconnect and reconnect your wallet in the Portal. The Portal will now connect through the new chain definition, and Keplr will derive your address using coin-type 60.

If you skip this step and reconnect without switching, you will see a **"Wallet Derivation Path Mismatch"** warning because Keplr is still using the old coin-type 118 derivation:

![Post-migration with stale Keplr — derivation path mismatch warning](assets/evmigration-10.jpg)

This shows Keplr using the old address while the Portal knows your correct EVM address. To resolve it, go back and add the new chain definition as described above.

**Alternative (manual re-import):** If the suggest chain flow is not available, you can manually re-import your mnemonic in Keplr:

1. **Disconnect** your wallet in the Portal first
2. Open Keplr, click your wallet name (top-left) to open the wallet list
3. Click the **+** button (top-right of the wallet list)
4. Choose **Import an existing wallet** > **Use recovery phrase or private key**
5. Enter the **same mnemonic** seed phrase you are currently using
6. Select the new wallet profile and reconnect to the Portal

After switching to the new chain definition (or re-importing), the Portal shows a clean state with the correct EVM address and your migration record:

![After switching to new chain definition — clean state with migration record](assets/evmigration-11.jpg)

Notice the badges at the top now show **"chain coin-type 60"** and **"wallet coin-type 60"** — both aligned. Your migration record is displayed with the legacy address, new Lumera address, and Ethereum hex address. The old Lumera (Legacy) chain entry can be removed from Keplr.

### Troubleshooting

**Portal shows "Wallet Derivation Path Mismatch" warning:**

This warning appears when Keplr is still using the old Lumera chain definition (coin-type 118) instead of the new one (coin-type 60). When a legacy account is detected as ready for migration, the mismatch is expected and the warning is suppressed. If you see it after migration, switch to the new Lumera chain definition in Keplr as described in section 5 above.

**Balance shows 0 after migration:**

Your funds are safe. Keplr is showing the balance of the old coin-type 118 address, not your migrated coin-type 60 address. Switch to the new Lumera chain definition in Keplr as described in section 5 above.

**"Keplr account changed since the Review step" error:**

You switched Keplr accounts or profiles between wizard steps. Go back to Step 1 and reconnect your wallet.

---

## Method 2: Lumera CLI

The CLI requires both keys (legacy and new) in the keyring. It handles address derivation, proof signing, gas simulation, and broadcasting automatically.

### Prerequisites

- `lumerad` binary (post-EVM upgrade version)
- Your mnemonic (recovery phrase)
- Access to a running Lumera node (local or remote RPC endpoint)

### CLI Step-by-Step

Both keys must be in the keyring. The CLI extracts the public key, generates both proofs, and broadcasts automatically.

#### 1. Pre-flight: Check Migration Eligibility

```bash
# Check if migration is enabled
lumerad query evmigration params --node <rpc-endpoint>

# Check migration estimate for your legacy address
lumerad query evmigration migration-estimate <legacy-address> --node <rpc-endpoint>
```

The estimate response shows `would_succeed: true` if migration is possible. If `would_succeed: false`, the `rejection_reason` field explains why.

```bash
# Check overall migration statistics
lumerad query evmigration migration-stats --node <rpc-endpoint>
```

#### 2. Import Both Keys from the Same Mnemonic

**Import the legacy key (coin-type 118, secp256k1):**

```bash
lumerad keys add legacy-key \
  --recover \
  --coin-type 118 \
  --algo secp256k1 \
  --keyring-backend test
```

Enter your mnemonic when prompted.

**Import the new EVM key (coin-type 60, eth_secp256k1):**

```bash
lumerad keys add new-key \
  --recover \
  --coin-type 60 \
  --algo eth_secp256k1 \
  --keyring-backend test
```

Enter the **same mnemonic** when prompted.

**Verify the addresses:**

```bash
lumerad keys show legacy-key -a --keyring-backend test
lumerad keys show new-key -a --keyring-backend test
```

The legacy address should match your known pre-EVM address on chain.

#### 3. Run the Migration

**For regular account migration:**

```bash
lumerad tx evmigration claim-legacy-account legacy-key new-key \
  --keyring-backend test \
  --chain-id lumera-mainnet-1 \
  --node tcp://localhost:26657 \
```

**For validator migration:**

```bash
lumerad tx evmigration migrate-validator legacy-validator-key new-validator-evm-key \
  --keyring-backend test \
  --chain-id lumera-mainnet-1 \
  --node tcp://localhost:26657 \
```

The CLI will:

1. Read both keys from the keyring, extract public keys, and derive bech32 addresses
2. Verify the legacy key is `secp256k1` (coin-type 118)
3. Build the migration payload and sign `SHA256(payload)` with the legacy key
4. Sign the new proof with the new key (must be `eth_secp256k1`)
5. Build an unsigned, fee-free Cosmos transaction
6. Simulate gas usage automatically
7. Prompt for confirmation (unless `--yes` flag is used)
8. Broadcast the transaction

#### 4. Verify the Migration

```bash
# Check that the migration record exists
lumerad query evmigration migration-record <legacy-address> --node <rpc-endpoint>

# Verify balances moved to the new address
lumerad query bank balances <new-address> --node <rpc-endpoint>

# Confirm legacy address has zero balance
lumerad query bank balances <legacy-address> --node <rpc-endpoint>
```

#### 5. Post-Migration for Validators

After a successful validator migration, update your node immediately:

```bash
# 1. Import the new key into the node's production keyring if not already present
lumerad keys add new-operator-key \
  --recover \
  --coin-type 60 \
  --algo eth_secp256k1 \
  --keyring-backend file

# 2. Restart the validator node
systemctl start lumerad
```

> **Warning:** Your validator will miss blocks and may be jailed if you do not restart promptly after migration. Plan a maintenance window before initiating validator migration.

#### 6. Clean Up

After verifying the migration was successful:

```bash
lumerad keys delete legacy-key --keyring-backend test
```

---

## Quick Reference: Query Commands

These queries are useful before, during, and after migration:

```bash
# Module parameters (is migration enabled? deadline?)
lumerad query evmigration params

# Pre-flight estimate (what will be migrated, will it succeed?)
lumerad query evmigration migration-estimate <legacy-address>

# Migration record (has this address been migrated?)
lumerad query evmigration migration-record <legacy-address>

# Reverse lookup (find migration record by new address)
lumerad query evmigration migration-record-by-new-address <new-address>

# Global statistics (how many accounts migrated/remaining?)
lumerad query evmigration migration-stats

# List legacy accounts still needing migration
lumerad query evmigration legacy-accounts --limit 100

# List completed migrations
lumerad query evmigration migrated-accounts --limit 100
```

---

## Migration Parameters

The following chain parameters govern migration behavior. These are set by governance:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `enable_migration` | `true` | Master on/off switch. When `false`, all migration messages are rejected. |
| `migration_end_time` | `0` (no deadline) | Optional Unix timestamp deadline. If non-zero and current block time is past this, migration is rejected. |
| `max_migrations_per_block` | `50` | Rate limit for `MsgClaimLegacyAccount` per block. Prevents excessive gas consumption. |
| `max_validator_delegations` | `2000` | Safety cap for `MsgMigrateValidator`. Rejects if total delegation + unbonding + redelegation records exceed this. |

---

## Validator Migration Guide

Validators **must** use `MsgMigrateValidator` (not `MsgClaimLegacyAccount`). The chain explicitly rejects `claim-legacy-account` for validator operator addresses. Validator migration is a superset of regular account migration — it re-keys the validator record, all delegations pointing to the validator, distribution state, supernode registration, and action references in a single atomic transaction.

### What Gets Re-keyed

In addition to everything in a regular account migration (balances, authz, feegrants, claims, vesting), validator migration handles:

- **Validator record** — operator address updated in both the primary record and power indices
- **All delegations** — every delegator's active delegation to this validator is re-keyed to the new valoper
- **Unbonding delegations** — all pending unbonds from this validator
- **Redelegations** — where the validator is source or destination
- **Distribution state** — current rewards, accumulated commission, outstanding rewards, historical rewards, slash events
- **Supernode record** — if the validator runs a supernode, the validator address is updated; the supernode account field is updated only if it matches the validator's legacy address (see Supernode section below)
- **Action records** — any action module records referencing this validator
- **Pending rewards** — all delegator rewards and validator commission are withdrawn before re-keying

### Pre-Migration Checklist

1. **Plan a maintenance window.** Your validator will miss blocks between migration and restart.
2. **Stop your validator node** before submitting the migration transaction:

   ```bash
   systemctl stop lumerad
   ```

3. **Verify eligibility:**

   ```bash
   lumerad query evmigration migration-estimate <legacy-validator-address> --node <rpc-endpoint>
   ```

   The estimate shows `would_succeed: true` if migration is possible. Common rejection reasons:
   - Validator is in Unbonding or Unbonded status (must be Bonded)
   - Total delegation + unbonding + redelegation records exceed `max_validator_delegations` (default 2000)
4. **Prepare both keys** (see Method 2 above for key import instructions)

### Migration via CLI (Recommended for Validators)

```bash
# 1. Import legacy key (coin-type 118)
lumerad keys add val-legacy --recover --coin-type 118 --algo secp256k1 --keyring-backend file

# 2. Import new EVM key (coin-type 60, same mnemonic)
lumerad keys add val-new --recover --coin-type 60 --algo eth_secp256k1 --keyring-backend file

# 3. Stop the validator
systemctl stop lumerad

# 4. Submit validator migration
lumerad tx evmigration migrate-validator val-legacy val-new \
  --keyring-backend file \
  --chain-id lumera-mainnet-1 \
  --node tcp://<trusted-rpc>:26657 \

# 5. Verify the migration succeeded
lumerad query evmigration migration-record <legacy-address> --node tcp://<trusted-rpc>:26657

# 6. Restart the validator
systemctl start lumerad
```

> **Warning:** Restart your validator promptly after migration. Extended downtime leads to missed blocks and potential jailing. Use a trusted external RPC endpoint for the migration transaction since your own node is stopped.

### Example: Full CLI Validator Migration Session

Below is a complete example of a validator migration on a running chain, showing every step and its output.

#### Step 1: Check migration parameters

```bash
lumerad query evmigration params
```

```json
{
  "params": {
    "enable_migration": true,
    "max_migrations_per_block": "50",
    "max_validator_delegations": "2000"
  }
}
```

#### Step 2: Identify the legacy validator key in your keyring

```bash
lumerad keys list --keyring-backend test
```

```json
[
  {
    "name": "validator_key",
    "type": "local",
    "address": "lumera10zqqmf3ulzumkyvh76lm3cdp9g8y8d4m45vj3p",
    "pubkey": "{\"@type\":\"/cosmos.crypto.secp256k1.PubKey\",\"key\":\"Apemo8WOh/LNcvP/RmU8A0pbtIsc4WihNkfAxk9cArUo\"}"
  }
]
```

Note the key type is `secp256k1` (coin-type 118 / legacy).

#### Step 3: Run the migration estimate to verify eligibility

```bash
lumerad query evmigration migration-estimate lumera10zqqmf3ulzumkyvh76lm3cdp9g8y8d4m45vj3p
```

```json
{
  "is_validator": true,
  "delegation_count": "1",
  "total_touched": "2",
  "would_succeed": true,
  "val_delegation_count": "1",
  "balance_summary": "1000000ulume",
  "has_supernode": true
}
```

`would_succeed: true` means the migration can proceed. Check `val_delegation_count` against `max_validator_delegations` — if it exceeds the limit, the transaction will fail.

#### Step 4: Import the new EVM key (same mnemonic, coin-type 60)

```bash
lumerad keys add validator_key_evm --recover --coin-type 60 --algo eth_secp256k1 --keyring-backend test
```

```text
> Enter your bip39 mnemonic
......
```

```json
{
  "name": "validator_key_evm",
  "type": "local",
  "address": "lumera10rlkmnqhsfwnwff8yyl2epl677rnjlwg9ljy7m",
  "pubkey": "{\"@type\":\"/cosmos.evm.crypto.v1.ethsecp256k1.PubKey\",\"key\":\"A+VIiUjpTUj2vO/zByw7iKZXFcRdPsVw32+8aSm4l5w3\"}"
}
```

Note the key type is now `ethsecp256k1` (coin-type 60 / EVM). The address will be different from the legacy key — this is expected.

#### Step 5: Stop the validator and submit the migration transaction

```bash
systemctl stop lumerad

lumerad tx evmigration migrate-validator validator_key validator_key_evm \
    --chain-id lumera-mainnet-1 \
    --keyring-backend test
```

```text
gas estimate: 570445
confirm transaction before broadcasting [y/N]: y
```

The CLI:

1. Reads both keys from the keyring
2. Derives both addresses and builds the migration payload
3. Signs the legacy proof with `validator_key` (secp256k1)
4. Signs the new proof with `validator_key_evm` (eth_secp256k1)
5. Simulates gas, asks for confirmation, and broadcasts

On success you will see a response with `"code":0` and the `migrate_validator` event:

```json
{
  "height": "8121",
  "txhash": "A4C1416FF0DF6E93A7A9E9A5116BA433BFD65C2170678B5010CFF1894A75B76C",
  "code": 0,
  "gas_used": "383726"
}
```

#### Step 6: Verify the migration record

```bash
lumerad query evmigration migration-record lumera10zqqmf3ulzumkyvh76lm3cdp9g8y8d4m45vj3p
```

```json
{
  "record": {
    "legacy_address": "lumera10zqqmf3ulzumkyvh76lm3cdp9g8y8d4m45vj3p",
    "new_address": "lumera10rlkmnqhsfwnwff8yyl2epl677rnjlwg9ljy7m",
    "migration_time": "1775174579",
    "migration_height": "8121"
  }
}
```

#### Step 7: Restart the validator immediately

```bash
systemctl start lumerad
```

### Migration via Portal + Keplr

Validators can also use the Portal wizard (Method 1). The Portal automatically detects validator status and adds a pre-migration checklist to the wizard:

1. Maintenance window planned
2. Validator node stopped
3. Post-migration commands copied

The wizard shows the restart commands on the success screen. See Method 1 above for the full Portal flow.

### Post-Migration

After the transaction is confirmed:

```bash
# Restart the validator node
systemctl start lumerad

# Verify the validator is signing blocks
lumerad query staking validator <new-valoper-address> --node tcp://localhost:26657
```

If your validator was also a supernode, see the Supernode section below for additional steps.

---

## Supernode Operator Migration Guide

Supernode operators have two scenarios depending on whether they are also validators.

### Scenario A: Supernode Operator Who Is Also a Validator

If the validator address and the supernode account are the same entity (the most common setup), **validator migration handles everything automatically**:

- `MsgMigrateValidator` re-keys the supernode's validator address to the new valoper
- If the supernode account matches the validator's legacy address, it is also updated to the new address
- Supernode evidence records and metrics state are migrated
- Migration history is appended to the supernode record

**Follow the Validator Migration Guide above.** After migration, restart both the validator and the supernode:

```bash
# Restart validator
systemctl start lumerad

# Restart supernode (update config.yml key_name if needed)
systemctl restart supernode
```

If the supernode binary was set up with `evm_key_name` in `config.yml` (as done by the devnet setup scripts), the supernode process may handle the key switch automatically on startup. Otherwise, update `config.yml` manually:

```yaml
supernode:
  key_name: <new-evm-key-name>    # The EVM key (coin-type 60)
  identity: <new-lumera-address>   # Your new bech32 address
  # Remove evm_key_name if present — no longer needed post-migration
```

### Scenario B: Standalone Supernode Operator (Non-Validator)

If the supernode account is a separate address from the validator (less common), the operator migrates independently using `MsgClaimLegacyAccount`. The on-chain supernode record's `SupernodeAccount` field is automatically updated.

#### Option 1: Portal + Keplr (Recommended)

Follow the standard Portal flow in Method 1. The Portal detects supernode status and shows a post-migration reminder to update your supernode configuration.

1. Open the Lumera Portal and connect Keplr
2. The portal shows your legacy account as eligible for migration with supernode status noted
3. Complete the 3-step wizard (Review → Sign → Submit)
4. After success, update your supernode configuration (see below)

#### Option 2: CLI

```bash
# 1. Import legacy key (coin-type 118)
lumerad keys add sn-legacy --recover --coin-type 118 --algo secp256k1 --keyring-backend file

# 2. Import new EVM key (coin-type 60, same mnemonic)
lumerad keys add sn-new --recover --coin-type 60 --algo eth_secp256k1 --keyring-backend file

# 3. Migrate (no need to stop the supernode first)
lumerad tx evmigration claim-legacy-account sn-legacy sn-new \
  --keyring-backend file \
  --chain-id lumera-mainnet-1 \
  --node tcp://localhost:26657 \

# 4. Verify
lumerad query evmigration migration-record <legacy-address>
```

### Post-Migration: Update Supernode Configuration

After migration (either scenario), update the supernode's local configuration to use the new EVM key:

1. **Update `~/.supernode/config.yml`:**

   ```yaml
   supernode:
     key_name: <new-evm-key-name>
     identity: <new-lumera-address>
   ```

2. **If using `sncli`, update its config too:**

   ```bash
   crudini --set ~/.sncli/config.ini supernode address "<new-lumera-address>"
   ```

3. **Restart the supernode process:**

   ```bash
   systemctl restart supernode
   ```

4. **Verify the supernode is healthy:**

   ```bash
   lumerad query supernode get-supernode <valoper-address>
   ```

   Confirm the `supernode_account` field shows your new address.

### Migration Order for Validator-Supernodes

If the validator and supernode account are the same entity, only one migration is needed (`MsgMigrateValidator`). If they are different entities:

1. **Migrate the supernode account first** via `claim-legacy-account`
2. **Then migrate the validator** via `migrate-validator`

This order works because `MigrateValidatorSupernode` checks whether the supernode account matches the validator's legacy address. If it was already migrated independently, the supernode account field is preserved and not overwritten.

The reverse order also works — the code handles both sequences correctly. But migrating the supernode first means less disruption, since standalone supernode migration doesn't require stopping the validator.

---

## FAQ

**Q: Do I need LUME on my new address to pay for migration?**

No. Migration transactions are fee-free. The transaction carries a gas limit for internal processing, but no fee is charged.

**Q: Can I migrate to any address?**

No. The new address must be derived from the **same mnemonic** as the legacy address using coin-type 60 and eth_secp256k1. The chain verifies this through the dual-signature proof.

**Q: What if I'm a validator - should I use `claim-legacy-account` or `migrate-validator`?**

Validators **must** use `migrate-validator`. The `claim-legacy-account` command explicitly rejects validator operator addresses. `migrate-validator` handles the additional complexity of re-keying all delegations pointing to your validator.

**Q: Can I migrate back to the legacy address?**

No. Migration is irreversible. The legacy account is removed from the chain's auth module after migration.

**Q: What happens to my staking rewards during migration?**

All pending staking rewards and validator commission are automatically withdrawn and included in the bank balance transfer during migration.

**Q: Is there a deadline for migration?**

Check the `migration_end_time` parameter. If it's `0`, there is no deadline (only the `enable_migration` flag controls availability). Governance can set or extend the deadline.

**Q: My validator has too many delegators and migration is rejected. What do I do?**

The `max_validator_delegations` parameter (default 2000) limits how many records can be re-keyed in one transaction. If your validator exceeds this, governance may increase the limit, or delegators can redelegate before validator migration.

---

## Migrating a multisig account

Multisig legacy accounts (flat K-of-N secp256k1) use an offline, coordinator-driven flow with four commands. The portal wizard does not support multisig — use the CLI.

See [legacy-migration.md](./legacy-migration.md#multisig-account-migration) for the architecture and wire-format reference.

### Precondition: ensure the pubkey is on-chain

If the multisig account has never signed a transaction, its pubkey is nil on-chain and migration will fail. Submit any transaction from the multisig account first, for example a 1-ulume self-send. Then confirm the key is stored:

```bash
lumerad query auth account <multisig-legacy-address>
```

The response must show a `multisig` key listing all sub-keys.

### Step 1: Coordinator generates the proof payload template

The coordinator (any co-signer who will drive the flow) creates a JSON template that describes the migration and contains the unsigned payload:

```bash
lumerad tx evmigration generate-proof-payload \
  --legacy <multisig-bech32> \
  --new <new-eth-bech32> \
  --kind claim \
  --chain-id lumera-devnet-1 \
  --out proof.json
```

`--kind` is `claim` for `MsgClaimLegacyAccount` and `validator` for `MsgMigrateValidator`. The output `proof.json` contains the canonical payload string, the multisig pub-key structure, and empty signature slots for each co-signer.

Distribute `proof.json` to all co-signers who will provide signatures.

### Step 2: Each co-signer signs on their own machine

Each participating co-signer imports their individual sub-key and runs:

```bash
lumerad tx evmigration sign-proof proof.json \
  --from <my-sub-key> --keyring-backend test \
  --out my-partial.json
```

`sign-proof` is idempotent — re-running it overwrites the partial output file with a fresh signature. Each co-signer produces their own `<name>-partial.json` file. Send all partial files back to the coordinator.

### Step 3: Coordinator merges the threshold-many partials

The coordinator collects at least K partial files (where K is the multisig threshold) and merges them:

```bash
lumerad tx evmigration combine-proof \
  alice-partial.json bob-partial.json \
  --out tx.json --chain-id lumera-devnet-1
```

`combine-proof` validates cross-file consistency before merging: it checks that all partials share the same `chain_id`, `legacy_address`, `new_address`, multisig threshold, and `sub_pub_keys` list. It rejects mismatched files before writing `tx.json`.

### Step 4: Broadcast the assembled transaction

The coordinator broadcasts using the new EVM key as the transaction signer:

```bash
lumerad tx evmigration submit-proof tx.json \
  --from <new-eth-key> \
  --chain-id lumera-devnet-1 --keyring-backend test -y
```

On success, verify the migration record:

```bash
lumerad query evmigration migration-record <multisig-legacy-address>
```

### Notes

- **Cold-wallet / nil-pubkey single-sig accounts**: if a single-key legacy account has never signed a transaction (nil pubkey on-chain), use the `--legacy-key` flag with `sign-proof` to supply the key directly from the keyring. The standard `claim-legacy-account` command does not handle nil pubkeys.
- `combine-proof` requires exactly threshold-many partials — passing fewer raises an error; passing more is accepted and the first K valid partials are used.
- After a successful migration, follow the same post-migration steps as for any other account (add the new Lumera EVM chain definition to Keplr, verify balances at the new address, etc.).
