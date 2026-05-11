// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../interfaces/ISupernode.sol";

/// @title SupernodeClient — Example contract that interacts with Lumera's Supernode precompile
/// @notice Demonstrates how to query supernode status, list nodes, check metrics,
///         and build on-chain logic around the supernode network from Solidity.
/// @dev Deploy this to Lumera's EVM and it will call the supernode precompile at 0x0902.
contract SupernodeClient {
    /// @notice The supernode precompile lives at this fixed address on Lumera.
    ISupernode public constant SUPERNODE = ISupernode(0x0000000000000000000000000000000000000902);

    /// @notice Supernode states as named constants for readability.
    uint8 public constant STATE_ACTIVE     = 1;
    uint8 public constant STATE_DISABLED   = 2;
    uint8 public constant STATE_STOPPED    = 3;
    uint8 public constant STATE_PENALIZED  = 4;
    uint8 public constant STATE_POSTPONED  = 5;

    // -----------------------------------------------------------------------
    // Query examples
    // -----------------------------------------------------------------------

    /// @notice Get the supernode module parameters.
    function getModuleParams()
        external
        view
        returns (
            uint256 minimumStake,
            uint64 reportingThreshold,
            uint64 slashingThreshold,
            string memory minSupernodeVersion,
            uint64 minCpuCores,
            uint64 minMemGb,
            uint64 minStorageGb
        )
    {
        return SUPERNODE.getParams();
    }

    /// @notice Check if a specific validator has a registered supernode.
    /// @return exists True if a supernode is found (non-empty validator address)
    /// @return info   The supernode info (empty struct if not found)
    function checkSupernode(string calldata validatorAddress)
        external
        view
        returns (bool exists, ISupernode.SuperNodeInfo memory info)
    {
        info = SUPERNODE.getSuperNode(validatorAddress);
        // A found supernode will have a non-empty validatorAddress
        exists = bytes(info.validatorAddress).length > 0;
    }

    /// @notice List active supernodes with pagination.
    function listNodes(uint64 offset, uint64 limit)
        external
        view
        returns (ISupernode.SuperNodeInfo[] memory nodes, uint64 total)
    {
        return SUPERNODE.listSuperNodes(offset, limit);
    }

    /// @notice Get the top-ranked supernodes for the current block.
    /// @param limit Max number of nodes to return
    /// @return nodes Ranked by XOR-distance from block hash
    function topNodesForCurrentBlock(int32 limit)
        external
        view
        returns (ISupernode.SuperNodeInfo[] memory nodes)
    {
        return SUPERNODE.getTopSuperNodesForBlock(
            int32(int256(block.number)),
            limit,
            0 // 0 = all states
        );
    }

    /// @notice Get the top active supernodes for a specific block.
    function topActiveNodesForBlock(int32 blockHeight, int32 limit)
        external
        view
        returns (ISupernode.SuperNodeInfo[] memory nodes)
    {
        return SUPERNODE.getTopSuperNodesForBlock(blockHeight, limit, STATE_ACTIVE);
    }

    /// @notice Get the latest metrics for a supernode and check freshness.
    /// @return metrics          The latest hardware metrics
    /// @return reportCount      How many reports have been submitted
    /// @return lastReportHeight Block height of the latest report
    /// @return isFresh          True if last report was within 100 blocks
    function getNodeHealth(string calldata validatorAddress)
        external
        view
        returns (
            ISupernode.MetricsReport memory metrics,
            uint64 reportCount,
            int64 lastReportHeight,
            bool isFresh
        )
    {
        (metrics, reportCount, lastReportHeight) = SUPERNODE.getMetrics(validatorAddress);
        isFresh = (int256(block.number) - int256(lastReportHeight)) < 100;
    }

    /// @notice Count how many supernodes are registered (total from first page).
    function totalSupernodeCount() external view returns (uint64) {
        (, uint64 total) = SUPERNODE.listSuperNodes(0, 1);
        return total;
    }
}
