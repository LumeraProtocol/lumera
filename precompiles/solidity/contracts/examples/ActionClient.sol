// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../interfaces/IAction.sol";

/// @title ActionClient — Example contract that interacts with Lumera's Action precompile
/// @notice Demonstrates how to query action fees, request Cascade storage actions,
///         and build on-chain logic around the action module from Solidity.
/// @dev Deploy this to Lumera's EVM and it will call the action precompile at 0x0901.
contract ActionClient {
    /// @notice The action precompile lives at this fixed address on Lumera.
    IAction public constant ACTION = IAction(0x0000000000000000000000000000000000000901);

    /// @notice Emitted when this contract estimates a fee for a caller.
    event FeeEstimated(address indexed caller, uint64 dataSizeKbs, uint256 totalFee);

    /// @notice Emitted when this contract submits a Cascade request.
    event CascadeRequested(address indexed caller, string actionId, uint256 feePaid);

    // -----------------------------------------------------------------------
    // Query examples
    // -----------------------------------------------------------------------

    /// @notice Estimate the total fee for storing `dataSizeKbs` of data.
    /// @dev Pure read — does not modify state. Can be called via eth_call.
    /// @return baseFee  The fixed base component
    /// @return perKbFee Per-kilobyte rate
    /// @return totalFee Total cost: baseFee + perKbFee * dataSizeKbs
    function estimateFee(uint64 dataSizeKbs)
        external
        view
        returns (uint256 baseFee, uint256 perKbFee, uint256 totalFee)
    {
        return ACTION.getActionFee(dataSizeKbs);
    }

    /// @notice Get all module parameters in one call.
    function getModuleParams()
        external
        view
        returns (
            uint256 baseActionFee,
            uint256 feePerKbyte,
            uint64 maxActionsPerBlock,
            uint64 minSuperNodes,
            int64 expirationDuration,
            string memory superNodeFeeShare,
            string memory foundationFeeShare
        )
    {
        return ACTION.getParams();
    }

    /// @notice Look up an existing action by its ID.
    function lookupAction(string calldata actionId)
        external
        view
        returns (IAction.ActionInfo memory)
    {
        return ACTION.getAction(actionId);
    }

    /// @notice List the caller's actions with pagination.
    function myActions(uint64 offset, uint64 limit)
        external
        view
        returns (IAction.ActionInfo[] memory actions, uint64 total)
    {
        return ACTION.getActionsByCreator(msg.sender, offset, limit);
    }

    // -----------------------------------------------------------------------
    // Transaction examples
    // -----------------------------------------------------------------------

    /// @notice Request a Cascade storage action through this contract.
    /// @dev The precompile charges the fee from the tx sender (msg.sender of
    ///      the original EVM tx, i.e., tx.origin in Lumera's precompile model).
    ///      This demonstrates how a DApp contract can orchestrate action requests.
    function requestCascadeStorage(
        string calldata dataHash,
        string calldata fileName,
        uint64 rqIdsIc,
        string calldata signatures,
        uint256 price,
        int64 expirationTime,
        uint64 fileSizeKbs
    ) external returns (string memory actionId) {
        actionId = ACTION.requestCascade(
            dataHash,
            fileName,
            rqIdsIc,
            signatures,
            price,
            expirationTime,
            fileSizeKbs
        );

        emit CascadeRequested(msg.sender, actionId, price);
    }

    /// @notice Convenience: estimate fee first, then submit Cascade request.
    /// @dev Shows a pattern where a contract reads chain state and acts on it
    ///      within a single transaction.
    function estimateAndRequestCascade(
        string calldata dataHash,
        string calldata fileName,
        uint64 rqIdsIc,
        string calldata signatures,
        int64 expirationTime,
        uint64 fileSizeKbs
    ) external returns (string memory actionId, uint256 totalFee) {
        // Step 1: Query the current fee
        (, , totalFee) = ACTION.getActionFee(fileSizeKbs);

        emit FeeEstimated(msg.sender, fileSizeKbs, totalFee);

        // Step 2: Submit the action using the queried fee
        actionId = ACTION.requestCascade(
            dataHash,
            fileName,
            rqIdsIc,
            signatures,
            totalFee,
            expirationTime,
            fileSizeKbs
        );

        emit CascadeRequested(msg.sender, actionId, totalFee);
    }
}
