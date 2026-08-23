# Standard EVM Precompiles

Lumera enables 8 standard precompiles from the Cosmos EVM v0.6.0 framework. These precompiles expose core Cosmos SDK modules to Solidity contracts, enabling EVM-native interaction with staking, governance, IBC, and other chain functionality.

All precompiles are registered in [`app/evm/precompiles.go`](../../../app/evm/precompiles.go) and use the upstream implementations from `github.com/cosmos/evm/precompiles/`.

> **Note:** The Vesting precompile (`0x0000000000000000000000000000000000000803`) is intentionally excluded because Cosmos EVM's `DefaultStaticPrecompiles` registry does not currently provide an implementation for it.

---

## Common Types

All precompiles that accept pagination or return coins share the following Solidity types (from `common/Types.sol`):

```solidity
struct Coin {
    string denom;
    uint256 amount;
}

struct DecCoin {
    string denom;
    uint256 amount;
    uint8 precision;
}

struct PageRequest {
    bytes key;
    uint64 offset;
    uint64 limit;
    bool countTotal;
    bool reverse;
}

struct PageResponse {
    bytes nextKey;
    uint64 total;
}

struct Height {
    uint64 revisionNumber;
    uint64 revisionHeight;
}
```

---

## 1. P256 Precompile

| | |
|---|---|
| **Address** | `0x0000000000000000000000000000000000000100` |
| **Package** | `github.com/cosmos/evm/precompiles/p256` |
| **Gas** | Fixed 3,450 |

Implements NIST P-256 (secp256r1) elliptic curve signature verification per [EIP-7212](https://eips.ethereum.org/EIPS/eip-7212).

### Input/Output

No Solidity interface — uses raw `STATICCALL` with exactly **160 bytes** of input:

| Offset | Size | Field |
|--------|------|-------|
| 0 | 32 | Message hash |
| 32 | 32 | Signature `r` |
| 64 | 32 | Signature `s` |
| 96 | 32 | Public key `x` |
| 128 | 32 | Public key `y` |

Returns `uint256(1)` (32 bytes) on success, empty data on failure.

### Use Cases

- WebAuthn / Passkeys authentication
- Hardware Security Module (HSM) signature verification
- Mobile Secure Enclave integration

### Example

```solidity
function verifyP256(
    bytes32 hash,
    bytes32 r,
    bytes32 s,
    bytes32 x,
    bytes32 y
) external view returns (bool) {
    bytes memory input = abi.encodePacked(hash, r, s, x, y);
    (bool ok, bytes memory result) = address(0x100).staticcall(input);
    return ok && result.length == 32 && abi.decode(result, (uint256)) == 1;
}
```

---

## 2. Bech32 Precompile

| | |
|---|---|
| **Address** | `0x0000000000000000000000000000000000000400` |
| **Package** | `github.com/cosmos/evm/precompiles/bech32` |
| **Interface** | `Bech32I.sol` |

Converts addresses between hex (EIP-55) and Bech32 (Cosmos) formats. Essential for contracts that need to interact with Cosmos-native addresses.

### Solidity Interface

```solidity
address constant Bech32_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000000400;

interface Bech32I {
    /// @dev Convert hex address to bech32 format.
    function hexToBech32(
        address addr,
        string memory prefix
    ) external returns (string memory bech32Address);

    /// @dev Convert bech32 address to hex format.
    function bech32ToHex(
        string memory bech32Address
    ) external returns (address addr);
}
```

### Example

```solidity
import {BECH32_CONTRACT} from "Bech32I.sol";

// Convert EVM address to Lumera bech32 address
string memory lumeraAddr = BECH32_CONTRACT.hexToBech32(msg.sender, "lumera");

// Convert Lumera bech32 address back to hex
address evmAddr = BECH32_CONTRACT.bech32ToHex("lumera1abc...");
```

---

## 3. Staking Precompile

| | |
|---|---|
| **Address** | `0x0000000000000000000000000000000000000800` |
| **Package** | `github.com/cosmos/evm/precompiles/staking` |
| **Interface** | `StakingI.sol` |

Full EVM interface to the Cosmos SDK staking module. Supports validator creation, delegation lifecycle, and queries.

### Transaction Methods

| Method | Description |
|--------|-------------|
| `createValidator(Description, CommissionRates, uint256, address, string, uint256)` | Create a new validator |
| `editValidator(Description, address, int256, int256)` | Modify validator description/commission |
| `delegate(address, string, uint256)` | Delegate tokens to a validator |
| `undelegate(address, string, uint256)` | Initiate undelegation (returns `completionTime`) |
| `redelegate(address, string, string, uint256)` | Move delegation between validators |
| `cancelUnbondingDelegation(address, string, uint256, uint256)` | Cancel in-progress undelegation |

### Query Methods

| Method | Description |
|--------|-------------|
| `delegation(address, string)` | Get delegation shares and balance |
| `unbondingDelegation(address, string)` | Get unbonding delegation entries |
| `validator(address)` | Get validator info |
| `validators(string, PageRequest)` | List validators by status |
| `redelegation(address, string, string)` | Get specific redelegation |
| `redelegations(address, string, string, PageRequest)` | List redelegations with pagination |

### Events

- `CreateValidator(address indexed validatorAddress, uint256 value)`
- `Delegate(address indexed delegatorAddress, address indexed validatorAddress, uint256 amount, uint256 newShares)`
- `Unbond(address indexed delegatorAddress, address indexed validatorAddress, uint256 amount, uint256 completionTime)`
- `Redelegate(address indexed delegatorAddress, address indexed validatorSrcAddress, address indexed validatorDstAddress, uint256 amount, uint256 completionTime)`
- `CancelUnbondingDelegation(address indexed delegatorAddress, address indexed validatorAddress, uint256 amount, uint256 creationHeight)`
- `EditValidator(address indexed validatorAddress, int256 commissionRate, int256 minSelfDelegation)`

### Example

```solidity
import {STAKING_CONTRACT} from "StakingI.sol";

// Delegate 100 LUME to a validator
STAKING_CONTRACT.delegate(
    msg.sender,
    "lumeravaloper1abc...",
    100 * 1e18  // amount in alume (18 decimals)
);
```

---

## 4. Distribution Precompile

| | |
|---|---|
| **Address** | `0x0000000000000000000000000000000000000801` |
| **Package** | `github.com/cosmos/evm/precompiles/distribution` |
| **Interface** | `DistributionI.sol` |

Handles staking reward distribution — claiming rewards, setting withdrawal addresses, and querying reward balances.

### Transaction Methods

| Method | Description |
|--------|-------------|
| `claimRewards(address, uint32)` | Claim rewards from up to N validators |
| `setWithdrawAddress(address, string)` | Set custom withdrawal address |
| `withdrawDelegatorRewards(address, string)` | Withdraw rewards from a specific validator |
| `withdrawValidatorCommission(string)` | Withdraw validator commission |
| `fundCommunityPool(address, Coin[])` | Contribute to community pool |
| `depositValidatorRewardsPool(address, string, Coin[])` | Deposit to validator rewards pool |

### Query Methods

| Method | Description |
|--------|-------------|
| `validatorDistributionInfo(string)` | Validator commission and self-bond rewards |
| `validatorOutstandingRewards(string)` | Outstanding rewards for a validator |
| `validatorCommission(string)` | Accumulated commission |
| `validatorSlashes(string, uint64, uint64, PageRequest)` | Slash events in a height range |
| `delegationRewards(address, string)` | Rewards for a specific delegation |
| `delegationTotalRewards(address)` | Total rewards across all delegations |
| `delegatorValidators(address)` | List validators delegated to |
| `delegatorWithdrawAddress(address)` | Current withdrawal address |
| `communityPool()` | Community pool balance |

### Events

- `ClaimRewards(address indexed delegatorAddress, uint256 amount)`
- `SetWithdrawerAddress(address indexed caller, string withdrawerAddress)`
- `WithdrawDelegatorReward(address indexed delegatorAddress, address indexed validatorAddress, uint256 amount)`
- `WithdrawValidatorCommission(string indexed validatorAddress, uint256 commission)`
- `FundCommunityPool(address indexed depositor, string denom, uint256 amount)`
- `DepositValidatorRewardsPool(address indexed depositor, address indexed validatorAddress, string denom, uint256 amount)`

### Example

```solidity
import {DISTRIBUTION_CONTRACT} from "DistributionI.sol";

// Claim rewards from up to 10 validators
DISTRIBUTION_CONTRACT.claimRewards(msg.sender, 10);

// Check rewards for a specific delegation
DecCoin[] memory rewards = DISTRIBUTION_CONTRACT.delegationRewards(
    msg.sender, "lumeravaloper1abc..."
);
```

---

## 5. ICS20 Precompile (IBC Transfer)

| | |
|---|---|
| **Address** | `0x0000000000000000000000000000000000000802` |
| **Package** | `github.com/cosmos/evm/precompiles/ics20` |
| **Interface** | `ICS20I.sol` |

Enables IBC fungible token transfers directly from Solidity contracts. This is the primary way EVM contracts interact with cross-chain token movement.

### Transaction Methods

| Method | Description |
|--------|-------------|
| `transfer(string, string, string, uint256, address, string, Height, uint64, string)` | Execute an IBC transfer |

Transfer parameters:
- `sourcePort` / `sourceChannel` — IBC routing (e.g., `"transfer"`, `"channel-0"`)
- `denom` / `amount` — token denomination and amount
- `sender` (hex) / `receiver` (bech32) — source and destination addresses
- `timeoutHeight` / `timeoutTimestamp` — timeout configuration (set to 0 to disable)
- `memo` — optional IBC memo (for PFM forwarding, wasm hooks, etc.)

### Query Methods

| Method | Description |
|--------|-------------|
| `denom(string)` | Get denomination info by trace hash |
| `denoms(PageRequest)` | List all known IBC denominations |
| `denomHash(string)` | Get hash for a denomination trace |

### Events

- `IBCTransfer(address indexed sender, string indexed receiver, string sourcePort, string sourceChannel, string denom, uint256 amount, string memo)`

### Example

```solidity
import {ICS20_CONTRACT, Height} from "ICS20I.sol";

// Send 50 LUME to Osmosis
ICS20_CONTRACT.transfer(
    "transfer",
    "channel-0",
    "ulume",
    50_000_000,          // 50 LUME in ulume
    msg.sender,
    "osmo1receiver...",
    Height(0, 0),        // no height timeout
    uint64(block.timestamp + 600) * 1_000_000_000, // 10 min timeout
    ""                   // no memo
);
```

---

## 6. Bank Precompile

| | |
|---|---|
| **Address** | `0x0000000000000000000000000000000000000804` |
| **Package** | `github.com/cosmos/evm/precompiles/bank` |
| **Interface** | `IBank.sol` |

Read-only precompile for querying native token balances and supply. No transaction methods are exposed — token transfers use the standard EVM `transfer` or the staking/distribution precompiles.

### Query Methods

| Method | Description |
|--------|-------------|
| `balances(address)` | All native token balances for an account |
| `totalSupply()` | Total supply of all native tokens |
| `supplyOf(address)` | Total supply of a specific token (by ERC20 address) |

### Solidity Interface

```solidity
struct Balance {
    address contractAddress;  // ERC20 contract address
    uint256 amount;
}

interface IBank {
    function balances(address account) external view returns (Balance[] memory);
    function totalSupply() external view returns (Balance[] memory);
    function supplyOf(address erc20Address) external view returns (uint256);
}
```

### Example

```solidity
import {IBANK_CONTRACT} from "IBank.sol";

// Query all balances for an account
Balance[] memory bals = IBANK_CONTRACT.balances(msg.sender);

// Query total supply of a specific token
uint256 supply = IBANK_CONTRACT.supplyOf(erc20TokenAddress);
```

---

## 7. Governance (Gov) Precompile

| | |
|---|---|
| **Address** | `0x0000000000000000000000000000000000000805` |
| **Package** | `github.com/cosmos/evm/precompiles/gov` |
| **Interface** | `IGov.sol` |

Full EVM interface to the Cosmos SDK governance module. Submit proposals, vote, deposit, and query governance state from Solidity.

### Transaction Methods

| Method | Description |
|--------|-------------|
| `submitProposal(address, bytes, Coin[])` | Submit a governance proposal (protoJSON body) |
| `cancelProposal(address, uint64)` | Cancel an active proposal |
| `deposit(address, uint64, Coin[])` | Deposit funds to a proposal |
| `vote(address, uint64, VoteOption, string)` | Vote on a proposal |
| `voteWeighted(address, uint64, WeightedVoteOption[], string)` | Weighted/split vote |

`VoteOption` enum: `Unspecified(0)`, `Yes(1)`, `Abstain(2)`, `No(3)`, `NoWithVeto(4)`

### Query Methods

| Method | Description |
|--------|-------------|
| `getVote(uint64, address)` | Get a voter's vote on a proposal |
| `getVotes(uint64, PageRequest)` | List all votes for a proposal |
| `getDeposit(uint64, address)` | Get deposit info for a depositor |
| `getDeposits(uint64, PageRequest)` | List all deposits for a proposal |
| `getTallyResult(uint64)` | Get voting tally (yes/no/abstain/veto) |
| `getProposal(uint64)` | Get proposal details |
| `getProposals(uint32, address, address, PageRequest)` | List proposals with filters |
| `getParams()` | Get governance parameters |
| `getConstitution()` | Get on-chain constitution text |

### Events

- `SubmitProposal(address indexed proposer, uint64 proposalId)`
- `CancelProposal(address indexed proposer, uint64 proposalId)`
- `Deposit(address indexed depositor, uint64 proposalId, Coin[] amount)`
- `Vote(address indexed voter, uint64 proposalId, uint8 option)`
- `VoteWeighted(address indexed voter, uint64 proposalId, WeightedVoteOption[] options)`

### Example

```solidity
import {GOV_CONTRACT, VoteOption} from "IGov.sol";

// Vote Yes on proposal #1
GOV_CONTRACT.vote(msg.sender, 1, VoteOption.Yes, "Voting from EVM");
```

---

## 8. Slashing Precompile

| | |
|---|---|
| **Address** | `0x0000000000000000000000000000000000000806` |
| **Package** | `github.com/cosmos/evm/precompiles/slashing` |
| **Interface** | `ISlashing.sol` |

Manages validator slashing and jail status. Validators can unjail themselves from EVM, and signing info can be queried on-chain.

### Transaction Methods

| Method | Description |
|--------|-------------|
| `unjail(address)` | Unjail a validator after downtime jail period |

### Query Methods

| Method | Description |
|--------|-------------|
| `getSigningInfo(address)` | Signing info for a validator (missed blocks, jail status) |
| `getSigningInfos(PageRequest)` | Signing info for all validators |
| `getParams()` | Slashing module parameters |

### Events

- `ValidatorUnjailed(address indexed validator)`

### Solidity Interface

```solidity
struct SigningInfo {
    address validatorAddress;
    int64 startHeight;
    int64 indexOffset;
    int64 jailedUntil;
    bool tombstoned;
    int64 missedBlocksCounter;
}

interface ISlashing {
    function unjail(address validatorAddress) external returns (bool success);
    function getSigningInfo(address consAddress) external view returns (SigningInfo memory);
    function getSigningInfos(PageRequest calldata pagination) external view returns (SigningInfo[] memory, PageResponse memory);
    function getParams() external view returns (Params memory);
}
```

### Example

```solidity
import {SLASHING_CONTRACT} from "ISlashing.sol";

// Unjail a validator
SLASHING_CONTRACT.unjail(validatorConsAddress);

// Check if a validator is jailed
SigningInfo memory info = SLASHING_CONTRACT.getSigningInfo(consAddress);
bool isJailed = info.jailedUntil > int64(uint64(block.timestamp));
```

---

## Address Summary

| # | Precompile | Address | Type |
|---|------------|---------|------|
| 1 | P256 | `0x...100` | Cryptographic |
| 2 | Bech32 | `0x...400` | Utility |
| 3 | Staking | `0x...800` | Module |
| 4 | Distribution | `0x...801` | Module |
| 5 | ICS20 | `0x...802` | IBC |
| 6 | ~~Vesting~~ | `0x...803` | *Excluded* |
| 7 | Bank | `0x...804` | Module |
| 8 | Gov | `0x...805` | Module |
| 9 | Slashing | `0x...806` | Module |
