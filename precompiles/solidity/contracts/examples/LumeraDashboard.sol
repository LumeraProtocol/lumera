// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../interfaces/IAction.sol";
import "../interfaces/ISupernode.sol";

/// @title LumeraDashboard — Aggregates data from multiple Lumera precompiles
/// @notice A single contract that queries both the Action and Supernode modules,
///         demonstrating how DApps can build rich on-chain views across Lumera's
///         native modules. Deploy once and use eth_call for gas-free reads.
contract LumeraDashboard {
    IAction public constant ACTION = IAction(0x0000000000000000000000000000000000000901);
    ISupernode public constant SUPERNODE = ISupernode(0x0000000000000000000000000000000000000902);

    /// @notice Complete network overview in a single eth_call.
    struct NetworkOverview {
        // Action module
        uint256 baseActionFee;
        uint256 feePerKbyte;
        uint64 maxActionsPerBlock;
        uint64 minSuperNodes;
        // Supernode module
        uint256 minimumStake;
        uint64 totalSupernodes;
        string minSupernodeVersion;
        uint64 minCpuCores;
        uint64 minMemGb;
        uint64 minStorageGb;
    }

    /// @notice Fee estimate with context.
    struct FeeEstimate {
        uint64 dataSizeKbs;
        uint256 baseFee;
        uint256 perKbFee;
        uint256 totalFee;
        uint64 availableSupernodes;
    }

    // -----------------------------------------------------------------------
    // Aggregated queries
    // -----------------------------------------------------------------------

    /// @notice Get a full network overview combining both modules.
    /// @dev Single eth_call — no gas cost. Useful for dashboards and monitoring.
    function getNetworkOverview() external view returns (NetworkOverview memory overview) {
        // Action params
        (
            overview.baseActionFee,
            overview.feePerKbyte,
            overview.maxActionsPerBlock,
            overview.minSuperNodes,
            ,  // expirationDuration
            ,  // superNodeFeeShare
               // foundationFeeShare
        ) = ACTION.getParams();

        // Supernode params + count
        (
            overview.minimumStake,
            ,  // reportingThreshold
            ,  // slashingThreshold
            overview.minSupernodeVersion,
            overview.minCpuCores,
            overview.minMemGb,
            overview.minStorageGb
        ) = SUPERNODE.getParams();

        (, overview.totalSupernodes) = SUPERNODE.listSuperNodes(0, 1);
    }

    /// @notice Estimate fees with network context — tells the caller both the
    ///         cost and whether there are enough supernodes to process the action.
    function estimateFeeWithContext(uint64 dataSizeKbs)
        external
        view
        returns (FeeEstimate memory estimate)
    {
        estimate.dataSizeKbs = dataSizeKbs;

        (estimate.baseFee, estimate.perKbFee, estimate.totalFee) =
            ACTION.getActionFee(dataSizeKbs);

        (, estimate.availableSupernodes) = SUPERNODE.listSuperNodes(0, 1);
    }

    /// @notice Check if the network is ready to process actions.
    /// @return ready           True if enough supernodes are active
    /// @return totalSupernodes Total registered supernode count
    /// @return minRequired     Minimum supernodes required per action
    function isNetworkReady()
        external
        view
        returns (bool ready, uint64 totalSupernodes, uint64 minRequired)
    {
        (, , , minRequired, , , ) = ACTION.getParams();
        (, totalSupernodes) = SUPERNODE.listSuperNodes(0, 1);
        ready = totalSupernodes >= minRequired;
    }

    /// @notice Get a summary of a supernode's operational status.
    /// @return info            Supernode registration info
    /// @return metrics         Latest hardware metrics
    /// @return reportCount     Total metric reports submitted
    /// @return lastReportBlock Block of most recent metric report
    /// @return isActive        True if state == 1 (Active)
    function getSupernodeSummary(string calldata validatorAddress)
        external
        view
        returns (
            ISupernode.SuperNodeInfo memory info,
            ISupernode.MetricsReport memory metrics,
            uint64 reportCount,
            int64 lastReportBlock,
            bool isActive
        )
    {
        info = SUPERNODE.getSuperNode(validatorAddress);
        isActive = info.currentState == 1;
        (metrics, reportCount, lastReportBlock) = SUPERNODE.getMetrics(validatorAddress);
    }
}
