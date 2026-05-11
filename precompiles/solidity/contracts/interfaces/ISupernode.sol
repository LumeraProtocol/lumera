// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title ISupernode — Lumera Supernode Module Precompile Interface
/// @notice Precompile at address 0x0000000000000000000000000000000000000902
/// @dev Call this interface to interact with Lumera's supernode module directly
///      from Solidity. Supernodes are validator-operated service nodes that
///      process actions, report hardware metrics, and earn fee shares.
interface ISupernode {
    // -----------------------------------------------------------------------
    // Structs
    // -----------------------------------------------------------------------

    /// @notice On-chain information about a registered supernode.
    struct SuperNodeInfo {
        string validatorAddress;   // Bech32 lumeravaloper... address
        string supernodeAccount;   // Bech32 lumera... account address
        uint8 currentState;        // 1=Active, 2=Disabled, 3=Stopped, 4=Penalized, 5=Postponed
        int64 stateHeight;         // Block height of last state transition
        string ipAddress;          // Current IP address
        string p2pPort;            // P2P listening port
        string note;               // Operator-set note
        uint64 evidenceCount;      // Number of misbehavior evidence records
    }

    /// @notice Hardware metrics reported by a supernode.
    /// @dev Integer representation — float64 fields from protobuf are rounded
    ///      to uint64/uint32. Percentages are whole numbers (e.g., 45 = 45%).
    struct MetricsReport {
        uint32 versionMajor;
        uint32 versionMinor;
        uint32 versionPatch;
        uint32 cpuCoresTotal;
        uint64 cpuUsagePercent;
        uint64 memTotalGb;
        uint64 memUsagePercent;
        uint64 memFreeGb;
        uint64 diskTotalGb;
        uint64 diskUsagePercent;
        uint64 diskFreeGb;
        uint64 uptimeSeconds;
        uint32 peersCount;
    }

    // NOTE: Transaction methods (registerSupernode, deregisterSupernode,
    // startSupernode, stopSupernode, updateSupernode, reportMetrics) are
    // omitted from this interface. These are operator-internal operations —
    // supernodes and validators submit Cosmos SDK transactions directly,
    // not through the EVM.

    // -----------------------------------------------------------------------
    // Queries
    // -----------------------------------------------------------------------

    /// @notice Get supernode info by validator address.
    function getSuperNode(
        string calldata validatorAddress
    ) external view returns (SuperNodeInfo memory info);

    /// @notice Get supernode info by its operator account address.
    function getSuperNodeByAccount(
        string calldata supernodeAddress
    ) external view returns (SuperNodeInfo memory info);

    /// @notice List all registered supernodes with pagination.
    /// @param offset Starting index
    /// @param limit  Max results to return (capped at 100)
    /// @return nodes Array of supernode info structs
    /// @return total Total number of registered supernodes
    function listSuperNodes(
        uint64 offset,
        uint64 limit
    ) external view returns (SuperNodeInfo[] memory nodes, uint64 total);

    /// @notice Get top supernodes for a block by XOR-distance ranking.
    /// @param blockHeight Block height to rank against
    /// @param limit       Max results
    /// @param state       Filter by state (0 = all states)
    /// @return nodes      Ranked supernode info structs
    function getTopSuperNodesForBlock(
        int32 blockHeight,
        int32 limit,
        uint8 state
    ) external view returns (SuperNodeInfo[] memory nodes);

    /// @notice Get the latest metrics for a supernode.
    /// @param validatorAddress Bech32 lumeravaloper... address
    /// @return metrics          Latest reported metrics
    /// @return reportCount      Total number of metric reports submitted
    /// @return lastReportHeight Block height of the most recent report
    function getMetrics(
        string calldata validatorAddress
    ) external view returns (
        MetricsReport memory metrics,
        uint64 reportCount,
        int64 lastReportHeight
    );

    /// @notice Get the supernode module parameters.
    /// @return minimumStake        Min stake to register (ulume)
    /// @return reportingThreshold  Blocks between required metric reports
    /// @return slashingThreshold   Missed reports before slashing
    /// @return minSupernodeVersion Min software version string (e.g., "1.0.0")
    /// @return minCpuCores         Min CPU cores required
    /// @return minMemGb            Min RAM in GB
    /// @return minStorageGb        Min disk storage in GB
    function getParams() external view returns (
        uint256 minimumStake,
        uint64 reportingThreshold,
        uint64 slashingThreshold,
        string memory minSupernodeVersion,
        uint64 minCpuCores,
        uint64 minMemGb,
        uint64 minStorageGb
    );
}
