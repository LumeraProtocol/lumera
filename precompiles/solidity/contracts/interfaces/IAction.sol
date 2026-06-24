// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IAction — Lumera Action Module Precompile Interface
/// @notice Precompile at address 0x0000000000000000000000000000000000000901
/// @dev Call this interface to interact with Lumera's action module directly
///      from Solidity. Actions represent distributed GPU compute jobs (Cascade
///      storage, Sense analysis) processed by the supernode network.
interface IAction {
    // -----------------------------------------------------------------------
    // Structs
    // -----------------------------------------------------------------------

    /// @notice LEP 5 availability commitment for Cascade storage verification.
    struct AvailabilityCommitment {
        string commitmentType;    // e.g. "merkle_blake3"
        uint8 hashAlgo;           // 0 = unspecified, 1 = BLAKE3, 2 = SHA256
        uint32 chunkSize;         // bytes per chunk
        uint64 totalSize;         // total file size in bytes
        uint32 numChunks;         // number of chunks
        bytes root;               // Merkle root hash
        uint32[] challengeIndices; // chunk indices the supernode must prove
    }

    /// @notice LEP 5 Merkle inclusion proof for a single challenged chunk.
    struct ChunkProof {
        uint32 chunkIndex;        // which chunk this proves
        bytes leafHash;           // hash of the chunk data
        bytes[] pathHashes;       // sibling hashes along the Merkle path
        bool[] pathDirections;    // true = right sibling, false = left
    }

    /// @notice Represents a single action on the Lumera chain.
    struct ActionInfo {
        string actionId;
        address creator;
        uint8 actionType;   // 1 = Cascade, 2 = Sense
        uint8 state;        // 0 = Pending, 1 = Processing, 2 = Done, 3 = Failed
        string metadata;    // JSON metadata (type-specific)
        uint256 price;      // Fee paid in ulume
        int64 expirationTime;
        int64 blockHeight;
        address[] superNodes;
    }

    // -----------------------------------------------------------------------
    // Events
    // -----------------------------------------------------------------------

    event ActionRequested(
        string indexed actionId,
        address indexed creator,
        uint8 actionType,
        uint256 price
    );

    event ActionApproved(
        string indexed actionId,
        address indexed creator
    );

    // -----------------------------------------------------------------------
    // Transactions
    // -----------------------------------------------------------------------

    /// @notice Request a Cascade storage action.
    /// @param dataHash     Hash of the data to store
    /// @param fileName     Original file name
    /// @param rqIdsIc      Number of redundancy IDs for IC
    /// @param signatures   Dot-delimited "Base64(rq_ids).creator_signature" string
    /// @param price        Fee to pay (ulume)
    /// @param expirationTime Unix timestamp for action expiry
    /// @param fileSizeKbs  File size in kilobytes (used for fee calculation)
    /// @param commitment   LEP 5 availability commitment (pass empty root to skip)
    /// @return actionId    The generated action ID
    function requestCascade(
        string calldata dataHash,
        string calldata fileName,
        uint64 rqIdsIc,
        string calldata signatures,
        uint256 price,
        int64 expirationTime,
        uint64 fileSizeKbs,
        AvailabilityCommitment calldata commitment
    ) external returns (string memory actionId);

    /// @notice Request a Sense analysis action.
    /// @param dataHash              Hash of the data to analyze
    /// @param ddAndFingerprintsIc   Number of DD & fingerprint IDs
    /// @param price                 Fee to pay (ulume)
    /// @param expirationTime        Unix timestamp for action expiry
    /// @param fileSizeKbs           File size in kilobytes
    /// @return actionId             The generated action ID
    function requestSense(
        string calldata dataHash,
        uint64 ddAndFingerprintsIc,
        uint256 price,
        int64 expirationTime,
        uint64 fileSizeKbs
    ) external returns (string memory actionId);

    /// @notice Finalize a Cascade action with storage proofs.
    /// @param actionId    The action to finalize
    /// @param rqIdsIds    Redundancy IDs assigned during storage
    /// @param chunkProofs LEP 5 Merkle proofs for challenged chunks (pass empty for pre-LEP5)
    /// @return success    True if the action reached Done state
    function finalizeCascade(
        string calldata actionId,
        string[] calldata rqIdsIds,
        ChunkProof[] calldata chunkProofs
    ) external returns (bool success);

    /// @notice Finalize a Sense analysis action with results.
    /// @param actionId              The action to finalize
    /// @param ddAndFingerprintsIds  DD and fingerprint result IDs
    /// @param signatures            Supernode signatures
    /// @return success              True if the action reached Done state
    function finalizeSense(
        string calldata actionId,
        string[] calldata ddAndFingerprintsIds,
        string calldata signatures
    ) external returns (bool success);

    /// @notice Approve a pending action (governance/creator approval).
    /// @param actionId The action to approve
    /// @return success True if approval succeeded
    function approveAction(
        string calldata actionId
    ) external returns (bool success);

    // -----------------------------------------------------------------------
    // Queries
    // -----------------------------------------------------------------------

    /// @notice Get details of a specific action by ID.
    function getAction(
        string calldata actionId
    ) external view returns (ActionInfo memory action);

    /// @notice Calculate the fee for an action of the given data size.
    /// @param dataSizeKbs Data size in kilobytes
    /// @return baseFee    The fixed base fee component
    /// @return perKbFee   The per-kilobyte fee rate
    /// @return totalFee   baseFee + perKbFee * dataSizeKbs
    function getActionFee(
        uint64 dataSizeKbs
    ) external view returns (uint256 baseFee, uint256 perKbFee, uint256 totalFee);

    /// @notice List actions created by a specific address.
    function getActionsByCreator(
        address creator,
        uint64 offset,
        uint64 limit
    ) external view returns (ActionInfo[] memory actions, uint64 total);

    /// @notice List actions filtered by state.
    function getActionsByState(
        uint8 state,
        uint64 offset,
        uint64 limit
    ) external view returns (ActionInfo[] memory actions, uint64 total);

    /// @notice List actions assigned to a specific supernode.
    function getActionsBySuperNode(
        address superNode,
        uint64 offset,
        uint64 limit
    ) external view returns (ActionInfo[] memory actions, uint64 total);

    /// @notice Get the action module parameters.
    /// @return baseActionFee              Fixed base fee per action (ulume)
    /// @return feePerKbyte                Per-kilobyte fee rate (ulume)
    /// @return maxActionsPerBlock         Max actions allowed in a single block
    /// @return minSuperNodes              Min supernodes required to process an action
    /// @return expirationDuration         Default expiry duration (seconds)
    /// @return superNodeFeeShare          Supernode fee share (decimal string, e.g. "0.85")
    /// @return foundationFeeShare         Foundation fee share (decimal string, e.g. "0.15")
    /// @return svcChallengeCount          LEP 5: number of chunks to challenge
    /// @return svcMinChunksForChallenge   LEP 5: minimum chunks required for SVC
    function getParams() external view returns (
        uint256 baseActionFee,
        uint256 feePerKbyte,
        uint64 maxActionsPerBlock,
        uint64 minSuperNodes,
        int64 expirationDuration,
        string memory superNodeFeeShare,
        string memory foundationFeeShare,
        uint32 svcChallengeCount,
        uint32 svcMinChunksForChallenge
    );
}
