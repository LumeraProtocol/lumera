# Unit Tests: EVM Migration CLI

Purpose: validates the `x/evmigration` CLI commands — two positional args (`<legacy-key> <new-key>`), key type enforcement, signature generation, address derivation, and command arg validation.

File: `x/evmigration/client/cli/tx_test.go`

| Test | Description |
| --- | --- |
| `TestSignLegacyProof_ValidKeys` | Happy path: generates proof from keyring, verifies signature against SHA256(payload) with returned pubkey. |
| `TestSignLegacyProof_ValidatorKind` | Verifies "validator" kind produces correct payload and valid signature. |
| `TestSignLegacyProof_LegacyKeyNotFound` | Rejects when legacy key name is not in keyring. |
| `TestSignLegacyProof_NewKeyNotFound` | Rejects when new key name is not in keyring. |
| `TestSignLegacyProof_WrongKeyType_EthSecp256k1` | Rejects eth_secp256k1 key as legacy key; must be secp256k1 (coin-type 118). |
| `TestSignLegacyProof_SameAddressRejected` | Rejects when both key names resolve to the same address. |
| `TestSignLegacyProof_DifferentMnemonics` | Allows different mnemonics for legacy and new keys (chain enforces same-mnemonic, not CLI). |
| `TestSignLegacyProof_ChainIDInPayload` | Verifies chain ID is bound in payload: correct chain ID verifies, wrong chain ID fails. |
| `TestSignNewProof_ValidEVMKey` | Verifies new proof generation from eth_secp256k1 key succeeds. |
| `TestSignNewProof_WrongKeyType_Secp256k1` | Rejects secp256k1 key as new key; must be eth_secp256k1. |
| `TestSignNewProof_KeyNotFound` | Rejects when new key is not in keyring. |
| `TestClaimLegacyAccount_RequiresExactlyTwoArgs` | Verifies claim command requires exactly 2 positional args. |
| `TestMigrateValidator_RequiresExactlyTwoArgs` | Verifies validator command requires exactly 2 positional args. |
| `TestClaimLegacyAccount_TxTimeoutFlag` | Verifies `--tx-timeout` flag is registered on claim command with default 30s. |
| `TestMigrateValidator_TxTimeoutFlag` | Verifies `--tx-timeout` flag is registered on migrate-validator command with default 30s. |
| `TestTxTimeoutFlag_CustomValue` | Verifies `--tx-timeout` accepts custom duration values (e.g. "2m"). |
| `TestGasAdjustment_DefaultOverriddenTo1_5` | Guards against SDK default gas adjustment (1.0) — ensures our override condition stays correct. |
| `TestSignLegacyProof_SignatureVerifiesWithPubKey` | Full round-trip: generate proof, reconstruct pubkey, verify signature independently. |
| `TestSignLegacyProof_PubKeyDerivedAddressMatchesReturned` | Returned pubkey derives to exactly the returned legacy address. |
| `TestSignNewProof_OutputIsEthSecp256k1` | Verifies new proof signature is 64-65 bytes (eth_secp256k1 format). |
| `TestSignLegacyProof_MultipleKeysInKeyring` | Multiple legacy keys in keyring: each key produces its own correct legacy address. |
| `TestSignLegacyProof_DifferentKindsDifferentSignatures` | "claim" and "validator" kinds produce different signatures (kind is in payload). |
| `TestSignNewProof_RejectsNonEVMKey` | Rejects secp256k1 key for new proof with descriptive error. |
| `TestSignLegacyProof_ReturnedPubKeyIsSecp256k1` | Returned pubkey is 33 bytes with 0x02/0x03 compressed prefix. |
| `TestSignNewProof_ReturnedSigFromEVMKey` | Verifies new proof from EVM key has correct minimum length. |
| `TestSignNewProof_UsesLegacyAminoSignMode` | Sign mode consistency: function output matches direct keyring.Sign with same mode. |
