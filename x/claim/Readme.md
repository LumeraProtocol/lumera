# Claim Module

## Contents
1. [Abstract](#abstract)
2. [Overview](#overview)
3. [Genesis State Implementation](#genesis-state-implementation)
4. [Components](#components)
5. [State Transitions](#state-transitions)
6. [Messages](#messages)
7. [Events](#events)
8. [Parameters](#parameters)
9. [Client](#client)

## Abstract

The Claim module enables secure migration of token balances from a Bitcoin-based blockchain to a Cosmos SDK-based blockchain. It allows users to claim their existing coins on the new chain by proving ownership of their old addresses through cryptographic verification.

## Overview

The claim module serves as a bridge between the old chain and new chain by:
- Storing initial balances from the old chain as claim records
- Verifying ownership through public key cryptography
- Managing secure transfer of tokens to new addresses
- Enforcing claim period and rate limiting constraints
- Handling unclaimed balances after period expiration

## Genesis State Implementation

The Genesis State defines the initial state of the claiming module. Below is a detailed breakdown of its components and implementation details.

### Genesis State Structure

```go
type GenesisState struct {
    Params               Params       
    ClaimRecords         []ClaimRecord 
    ModuleAccount        string        
    TotalClaimableAmount uint64    
}
```

## Components

### 1. Params

```go
type Params struct {
    EnableClaims      bool   
    ClaimEndTime      int64 
    MaxClaimsPerBlock uint64 
}
```

The module parameters are managed in the following ways:
- Initially hardcoded in `types/params.go`
- Default values are used when generating the initial genesis file
- Can be modified in `genesis.json` before chain start
- Post-chain start, parameters can only be updated through governance proposals

### 2. ClaimRecords

```go
type ClaimRecord struct {
    OldAddress string                                
    Balance    sdk.Coins 
    Claimed    bool          
    ClaimTime  int64       
}
```

Important implementation details:
- ClaimRecord in the genesis state serves only as a representation
- Records are not directly added to `genesis.json`
- Instead, they are loaded from a required CSV file during application startup
- The CSV file must:
  - Be named `claims.csv`
  - Be located in the same dir as geneis file `{homeDir}/.lumera/config`
  - Follow this format:
    ```csv
    oldAddress,balance
    PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi,99999999ulumen
    ```
- The application performs validation:
  - Panics if CSV file is not found
  - Verifies that the sum of all balances matches `TotalClaimableAmount`
  - Panics if balance validation fails

### 3. ModuleAccount

The ModuleAccount field:
- Stores the genesis module address
- Is initialized in `types/genesis.go` within the `DefaultGenesis` function
- Note: This field may be redundant since module addresses are deterministically derived from module names
- Currently useful for quick balance lookups in `genesis.json`

### 4. TotalClaimableAmount

This field serves as a validation checkpoint:
- Used to verify the total sum of balances in `claims.csv`
- Can be initialized in `types/genesis.go`
- Can be updated after chain initialization but before chain start
- Must match the sum of all claim record balances

### Claiming Process

Each unclaimed balance is tracked through a ClaimRecord object:

The claim process involves these steps:

1. User generates proof materials:
   ```bash
   # Using old chain CLI
   lumerad signclaim "old-address" "new-address"
   
   # Or using web portal with WASM wallet
   ```

2. Submit claim transaction with:
   - Old address: Base58 format, 34 characters  
     Example: `PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi`
   - New address: Bech32 format with 'lumera' prefix  
     Example: `lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp`
   - Public key: 33-byte compressed SECP256K1 public key in hex format  
     Example: `0309331fc3d23ca17d91eec40ee7711efcd56facf949d46cbfa6393d43f2747e90`
   - Signature: 65-byte hex format, signs message "{old_address}.{pub_key}.{new_address}"  
     Example: `1f46b3a2129047a0d7a6bf91e2879e940ed3db06a2cafaaaabacc337141146f43e4932d357b435bbf2c48227f5c2f738df23a2ebc221dd11cb14ed4b83bd2a95c7`

3. Module validates:
   - Claim record exists
   - Not already claimed
   - Within claim period
   - Under block limit
   - Public key matches old address
   - Valid signature
   - Signer matches new address

4. If valid:
   - Transfers tokens to new address
   - Marks record as claimed
   - Emits claim event

### Rate Limiting

Claims are rate-limited via:
- `max_claims_per_block` parameter
- Block claim counter
- Resets in evey new block

This prevents network congestion during high claim volume.



## State Transitions

### Claim Processing

When processing MsgClaim:
1. Validate prerequisites
2. Verify cryptographic proofs
3. Transfer tokens
4. Update record
5. Emit events

### Block Claim Tracking

For each block:
1. Count incremented on claim
2. Enforced against max limit
3. Cleaned up after 100 blocks

## Messages

### MsgClaim

Claims tokens by proving ownership:

```protobuf
message MsgClaim {
    string old_address = 1; // Original address (base58)
    string new_address = 2; // Destination Cosmos address (bech32)
    string pub_key = 3;    // Public key of old address (hex)
    string signature = 4;   // Signature proving ownership (hex)
}
```

Required fields:
- `old_address`: Base58 address (e.g. "PtqHAEacynVd3V821NPhgxu9K4Ab6kAguHi")
- `new_address`: Bech32 address (e.g. "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp")
- `pub_key`: 33-byte compressed public key (hex)
- `signature`: 65-byte signature of "old_address.pub_key.new_address"

Validation:
- Claim record exists and unclaimed
- Within claim period
- Block limit not exceeded
- Public key hashes to old address
- Valid signature
- Signer matches new address

### MsgUpdateParams

Updates module parameters through governance:

```protobuf
message MsgUpdateParams {
    string authority = 1;
    Params params = 2;
}
```

Requirements:
- Authority must be gov module
- Valid parameter values
- Passes governance process

## Events

### ClaimProcessed

Emitted on successful claim:
```go
EventTypeClaimProcessed = "claim_processed"
Attributes:
- old_address: Original address
- new_address: Destination address
- amount: Claimed amount
- claim_time: Processing timestamp
```

### ClaimPeriodEnd

Emitted when claim period expires:
```go
EventTypeClaimPeriodEnd = "claim_period_end"
Attributes:
- end_time: When period ended
```

## Parameters

```protobuf
message Params {
    bool enable_claims = 1;           // Whether claiming is enabled
    Duration claim_duration = 2;      // How long claims are valid
    uint64 max_claims_per_block = 3;  // Rate limiting per block
}
```

Default values:
- `enable_claims`: true
- `claim_duration`: 6 months
- `max_claims_per_block`: 100

Parameter update governance proposal:
```json
{
    "messages": [{
        "@type": "/lumera.claim.MsgUpdateParams",
        "authority": "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp",
        "params": {
            "enable_claims": true,
            "claim_duration": "4380h",
            "max_claims_per_block": "100"
        }
    }],
    "metadata": "ipfs://CID",
    "deposit": "10000ulumen",
    "title": "Update Claim Parameters",
    "summary": "Enable claims and set duration to 6 months"
}
```

## Client

### CLI

Query commands:
```bash
# Query module parameters
lumerad query claim params

# Get claim record for address
lumerad query claim claim-record [address]

# List all claim records with pagination
lumerad query claim claim-records

# Get module account balance
lumerad query bank balances $(lumerad keys show -a claim)
```

Transaction commands:
```bash
# Submit claim
lumerad tx claim claim \
  [old-address] \
  [new-address] \
  [pub-key] \
  [signature] \
  --from [key] \
  --chain-id [chain-id]

# Submit parameter change proposal
lumerad tx gov submit-proposal [proposal-file] \
  --from [key] \
  --chain-id [chain-id]
```

### gRPC

```protobuf
service Query {
    // Parameters queries the parameters of the module.
    rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
        option (google.api.http).get = "/lumera/claim/params";
    }

    // ClaimRecord queries the claim record for an address.  
    rpc ClaimRecord(QueryClaimRecordRequest) returns (QueryClaimRecordResponse) {
        option (google.api.http).get = "/lumera/claim/claim_record/{address}";
    }
}

service Msg {
    // Claim submits a claim transaction.
    rpc Claim(MsgClaim) returns (MsgClaimResponse);

    // UpdateParams updates the module parameters.
    rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}
```

### REST

Endpoints are mounted on the REST API:

```
GET /lumera/claim/params            # Query parameters
GET /lumera/claim/claim_record/{address} # Get claim record
```