# EVM Legacy Account Migration - Portal UI and Wallet Rollout

**Last updated**: 2026-03-21
**Chain module**: `x/evmigration`
**Portal UI**: `lumera-portal/src/modules/[chain]/claim`

This document describes the current implementation, not the earlier design draft. It also records the current Keplr constraints that matter for mainnet rollout.

## 1. Current Protocol

### 1.1 Migration Payload

Both migration messages use the same canonical payload string:

```text
lumera-evm-migration:<chain_id>:<evm_chain_id>:<kind>:<legacy_address>:<new_address>
```

Examples:

```text
lumera-evm-migration:lumera-mainnet-1:76857769:claim:lumera1legacy...:lumera1new...
lumera-evm-migration:lumera-mainnet-1:76857769:validator:lumera1legacy...:lumera1new...
```

`kind` is `claim` for `MsgClaimLegacyAccount` and `validator` for `MsgMigrateValidator`.

### 1.2 Message Shape

`MsgClaimLegacyAccount` and `MsgMigrateValidator` no longer carry `new_pub_key`.

Current fields:

- `new_address`
- `legacy_address`
- `legacy_pub_key`
- `legacy_signature`
- `new_signature`

Proto field numbers: `new_address=1`, `legacy_address=2`, `legacy_pub_key=3`, `legacy_signature=4`, `new_signature=5`.

Relevant files:

- [tx.proto](/home/akobrin/p/lumera/proto/lumera/evmigration/tx.proto)
- [verify.go](/home/akobrin/p/lumera/x/evmigration/keeper/verify.go)

### 1.3 Verification Rules

#### Legacy proof

The legacy proof still requires `legacy_pub_key` because the legacy flow supports both CLI/keyring signing and wallet ADR-036 signing.

Accepted legacy signature formats:

1. CLI/keyring path:
   - sign over `SHA256(payload)`
   - verification passes `SHA256(payload)` into SDK secp256k1 `VerifySignature`
2. Wallet path:
   - Keplr/Leap `signArbitrary`
   - chain reconstructs the ADR-036 canonical sign doc and verifies that

#### New proof

The new proof no longer needs `new_pub_key`.

The chain now recovers the new signer directly from `new_signature` and checks that the recovered address equals `new_address`.

Accepted new signature formats:

1. CLI/keyring path:
   - sign over `Keccak256(payload)`
2. Wallet path:
   - Keplr/Leap Ethereum provider `personal_sign`
   - chain verifies against `Keccak256("\x19Ethereum Signed Message:\n" + len(payload) + payload)`

Implementation notes:

- 64-byte and 65-byte ECDSA signatures are both accepted
- recovery ID normalization is handled in `verify.go`

### 1.4 Unsigned Cosmos Tx

Migration transactions remain unsigned at the Cosmos tx layer:

- zero signer infos
- zero fee amount
- non-zero gas limit

Fee-free is not gasless. Ante still consumes tx-size gas, so `gas_limit` must be set.

Current portal constants:

- claim migration gas limit: `1_500_000`
- validator migration gas limit: `5_000_000`

Relevant file:

- [migrationTx.ts](/home/akobrin/p/lumera-portal/src/modules/[chain]/claim/migrationTx.ts)

## 2. Query Surface

Current evmigration queries:

- `GET /lumera/evmigration/params`
- `GET /lumera/evmigration/migration_record/{legacy_address}`
- `GET /lumera/evmigration/migration_record_by_new_address/{new_address}`
- `GET /lumera/evmigration/migration_records`
- `GET /lumera/evmigration/migration_estimate/{legacy_address}`
- `GET /lumera/evmigration/migration_stats`
- `GET /lumera/evmigration/legacy_accounts`
- `GET /lumera/evmigration/migrated_accounts`

Important change:

- `migration_record_by_new_address` is now implemented on chain and used by the portal to detect that the connected wallet is already the migrated destination address.

Relevant files:

- [query.proto](/home/akobrin/p/lumera/proto/lumera/evmigration/query.proto)
- [query.go](/home/akobrin/p/lumera/x/evmigration/keeper/query.go)

## 3. Portal Implementation

### 3.1 Where The UI Lives

The migration UI is integrated into the Claim page, not a separate module.

Main files:

- [index.vue](/home/akobrin/p/lumera-portal/src/modules/[chain]/claim/index.vue)
- [migrationState.ts](/home/akobrin/p/lumera-portal/src/modules/[chain]/claim/migrationState.ts)
- [migrationTypes.ts](/home/akobrin/p/lumera-portal/src/modules/[chain]/claim/migrationTypes.ts)
- [migrationWallet.ts](/home/akobrin/p/lumera-portal/src/modules/[chain]/claim/migrationWallet.ts)
- [migrationTx.ts](/home/akobrin/p/lumera-portal/src/modules/[chain]/claim/migrationTx.ts)

### 3.2 Runtime EVM Detection

The portal does not key EVM support off the app version.

Instead, it probes:

- `GET /lumera/evmigration/params`

If that query succeeds, the effective runtime coin type becomes `60`. Otherwise it falls back to the configured coin type.

Relevant file:

- [useBlockchain.ts](/home/akobrin/p/lumera-portal/src/stores/useBlockchain.ts)

### 3.3 Connected Wallet Status Card

The claim page now checks the connected wallet automatically.

Current behavior:

1. Display the connected wallet address from `walletStore.currentAddress`
2. Query `migration_record/{address}`
3. If not found, query `migration_record_by_new_address/{address}`
4. If still not found, query `migration_estimate/{address}`

This produces three main states:

- legacy account ready for migration
- legacy account not migratable, with `rejection_reason`
- new or already migrated account, with migration record details when available

The `Start Migration Wizard` button is only enabled when the connected address is a migratable legacy address.

The card also uses the live wallet balance from the portal wallet store for display.

### 3.4 Stats Card

Migration stats are loaded from `migration_stats` and auto-refresh every 5 minutes.

The UI also includes a manual refresh action.

Chain-side semantics were tightened so `total_legacy` counts only unmigrated legacy accounts that still have relevant state.

### 3.5 Wizard Flow

#### Step 1: Check legacy account

The wizard shows:

- legacy address
- live balance
- delegations
- unbondings
- authz grants
- feegrant count
- supernode status
- validator status
- `would_succeed`
- `rejection_reason` when `would_succeed = false`

#### Step 2: Show new address

The wizard now shows both representations of the new coin-type-60 address:

- Lumera bech32
- Ethereum hex

These are the same underlying destination address bytes in two formats.

The step also reminds the user that the new address must come from the same mnemonic as the legacy address.

#### Step 3: Sign proofs

Current signing path:

- legacy proof: `keplr.signArbitrary(chainId, legacyAddr, payload)`
- new proof: `keplr.ethereum.request({ method: 'personal_sign', ... })`

The portal no longer gathers or stores `newPubKey`.

#### Step 4: Review

The review step explains:

- migration tx is fee-free
- no LUME is required on the new address to submit migration
- the tx still carries an internal gas limit for ante processing

#### Step 5: Broadcast and result

Portal behavior:

- build raw unsigned protobuf tx
- broadcast through `POST /cosmos/tx/v1beta1/txs`
- poll `GET /cosmos/tx/v1beta1/txs/{hash}`
- refresh migration status after success

After a successful migration, the UI should show a migration record instead of a ready-to-migrate state.

## 4. Wallet Details

### 4.1 Current Lumera Keplr Shape

For post-EVM Lumera, the portal currently suggests Keplr with:

```json
{
  "bip44": { "coinType": 60 },
  "features": [
    "eth-address-gen",
    "eth-key-sign",
    "eth-secp256k1-cosmos"
  ]
}
```

Example local config:

- [lumera-devnet.json](/home/akobrin/p/lumera-portal/chains/mainnet/lumera-devnet.json)

### 4.2 Why `alternativeBIP44s` Is Not Used

Keplr's public `ChainInfo` type still exposes `alternativeBIP44s`, but the current Keplr validator explicitly rejects:

- primary `bip44.coinType = 60`
- any non-empty `alternativeBIP44s`

Relevant upstream sources:

- Keplr `suggest-chain` docs: `ChainInfo` still includes `alternativeBIP44s`
- Keplr validator: `coin type 60 can't have alternative BIP44s`

See references in section 8.

### 4.3 Current Practical Constraint

Keplr's signing and key APIs are chain-id scoped:

- `getKey(chainId)`
- `getOfflineSigner(chainId)`
- `signArbitrary(chainId, signer, ...)`

They do not expose a supported API such as "give me the 118 key and the 60 key for this same chain ID".

This matters for migration:

- the new address can be derived from Keplr's Ethereum provider
- the legacy proof still depends on Keplr being able to sign for the legacy address on the Cosmos side

## 5. Mainnet Upgrade and Registry Rollout

This section is partly implementation fact and partly rollout guidance. Where it is guidance rather than an implemented guarantee, treat it as recommendation.

### 5.1 What Happens If Lumera Registry Is Changed To 60-Only

#### Fresh clients

Fresh Keplr clients that add Lumera after the registry becomes 60-only are expected to derive the new coin-type-60 address by default.

That is good for normal post-migration use, but it creates a migration risk:

- the current portal flow still needs a legacy ADR-036 signature
- Keplr does not currently expose both derivation paths for a `coinType: 60` Lumera chain entry

So a fresh 60-only Keplr client may be able to connect as the new address, but may not be able to produce the legacy proof through the current wallet flow.

#### Existing clients

Do not assume a registry update alone will switch existing Keplr users to the new address.

Reasons:

- `experimentalSuggestChain()` is documented to no-op if the same chain is already added
- existing Keplr installs can retain a sticky chain entry and address binding

Operationally, existing users should be treated as needing at least one of:

- extension refresh/update
- re-enable / reconnect
- remove Lumera chain entry and add it again

### 5.2 Should Registry Be Updated Before Or After Upgrade

Recommendation: do **not** update the public Keplr Lumera entry to 60-only before the chain upgrade, and do not rely on a same-day 60-only switch unless a separate legacy signing path is available.

Why:

1. Before upgrade:
   - Keplr would show the new 60 address while the chain still holds user state on legacy 118 accounts.
   - Users would see the wrong working address for the live chain state.
2. Immediately after upgrade:
   - fresh users could connect as 60-only and then fail to complete migration because the portal still needs a legacy `signArbitrary` proof.

### 5.3 Safer Mainnet Procedure

With the current implementation, the safer operational sequence is:

1. Upgrade the chain and enable `x/evmigration`
2. Keep the migration portal available immediately
3. Keep the Keplr-facing Lumera config usable for legacy signing during the migration window
4. Migrate users
5. Switch the public Lumera Keplr registration to 60-only only after the migration window, or after a separate legacy-signing path exists

In other words:

- migration phase: prioritize legacy signing
- post-migration phase: prioritize 60-only normal usage

### 5.4 If A One-Step Wallet Switch Is Desired

The current portal cannot guarantee a one-click Keplr switch for already-added chains.

The lowest-friction reliable path today is:

1. complete migration
2. reconnect wallet
3. if Keplr still shows the old Lumera address, remove the Lumera chain from Keplr
4. reconnect so Keplr adds Lumera again with the post-upgrade 60-only config

If an even smoother path is required, one of these needs to be added:

- a dedicated legacy-signing alias or temporary secondary chain entry for the migration window
- another supported legacy signing path outside the current Keplr chain-id-scoped API surface

### 5.5 Chain ID Bump Consideration

Keplr documents special handling when chain IDs follow the `{identifier}-{version}` format.

Changing `lumera-mainnet-1` to `lumera-mainnet-2` at the network upgrade may help Keplr treat the upgraded chain as a fresh entry, but that alone does not solve legacy signing:

- it may simplify switching normal usage to 60-only
- it does not by itself give the portal a way to sign the legacy 118 proof unless the old entry remains accessible or another legacy signing path exists

## 6. Recommended User Communication

### 6.1 During Migration

Suggested message:

> Migration uses two addresses derived from the same mnemonic:
> your legacy Lumera address (coin-type 118) and your new EVM-compatible Lumera address (coin-type 60).
> The wizard verifies the legacy address and shows the new address in both Lumera bech32 and Ethereum hex form.

### 6.2 After Successful Migration

Suggested message:

> Your legacy address now has 0 LUME.
> Your funds and migrated state are now on your new coin-type-60 address.
> If Keplr still shows the old Lumera address, reconnect first. If it still does not switch, remove Lumera from Keplr and add it again.

### 6.3 Portal CTA

The portal should expose a short post-success checklist:

1. Copy / confirm the new Lumera bech32 address
2. Copy / confirm the matching Ethereum hex address
3. Reconnect Keplr
4. If the displayed Lumera address is still old, remove Lumera from Keplr and reconnect

## 7. Implementation Checklist

Current chain-side implementation:

- `new_pub_key` removed from both migration messages
- legacy verifier accepts raw CLI and ADR-036 wallet signatures
- new verifier recovers signer from signature and accepts raw or EIP-191 wallet signatures
- reverse migration lookup by `new_address` added
- migration stats semantics tightened

Current portal-side implementation:

- claim page auto-detects connected wallet state
- migration record detection supports both legacy and new addresses
- wizard displays both bech32 and hex forms of the new address
- unsigned tx broadcaster sets a non-zero gas limit
- local/devnet chains disable public registry-dependent asset list behavior

Known operational limitation:

- the current wallet flow is still constrained by Keplr's chain-id-scoped key APIs
- therefore the mainnet wallet-switch experience must be planned as an operational rollout, not assumed to happen automatically from a registry JSON change

## 8. External References

These links are relevant because Keplr behavior here is an external dependency.

- Keplr `experimentalSuggestChain` docs:
  - https://raw.githubusercontent.com/chainapsis/keplr-wallet/develop/docs/docs/guide/suggest-chain.md
- Keplr wallet type surface (`getKey`, `getOfflineSigner`, `signArbitrary`, `ethereum` provider):
  - https://raw.githubusercontent.com/chainapsis/keplr-wallet/develop/packages/types/src/wallet/keplr.ts
- Keplr chain validator rule rejecting `coinType: 60` with `alternativeBIP44s`:
  - https://raw.githubusercontent.com/chainapsis/keplr-wallet/develop/packages/chain-validator/src/schema.ts
- Keplr community-driven chain registry guidelines:
  - https://github.com/chainapsis/keplr-chain-registry
