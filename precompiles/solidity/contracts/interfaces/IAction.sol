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
    /// @return actionId    The generated action ID
    function requestCascade(
        string calldata dataHash,
        string calldata fileName,
        uint64 rqIdsIc,
        string calldata signatures,
        uint256 price,
        int64 expirationTime,
        uint64 fileSizeKbs
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

    // NOTE: finalizeCascade / finalizeSense are omitted from this interface.
    // Finalization is a supernode-internal operation — supernodes submit
    // MsgFinalizeAction via Cosmos SDK transactions, not through the EVM.

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
    /// @return baseActionFee       Fixed base fee per action (ulume)
    /// @return feePerKbyte         Per-kilobyte fee rate (ulume)
    /// @return maxActionsPerBlock  Max actions allowed in a single block
    /// @return minSuperNodes       Min supernodes required to process an action
    /// @return expirationDuration  Default expiry duration (seconds)
    /// @return superNodeFeeShare   Supernode fee share (decimal string, e.g. "0.85")
    /// @return foundationFeeShare  Foundation fee share (decimal string, e.g. "0.15")
    function getParams() external view returns (
        uint256 baseActionFee,
        uint256 feePerKbyte,
        uint64 maxActionsPerBlock,
        uint64 minSuperNodes,
        int64 expirationDuration,
        string memory superNodeFeeShare,
        string memory foundationFeeShare
    );
}
