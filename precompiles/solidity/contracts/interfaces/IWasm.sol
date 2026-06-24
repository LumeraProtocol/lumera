// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IWasm — Lumera CosmWasm Precompile Interface
/// @notice Precompile at address 0x0000000000000000000000000000000000000903
/// @dev Call this interface to interact with CosmWasm contracts directly
///      from Solidity. Enables EVM contracts to execute and query CosmWasm
///      contracts on Lumera's dual-runtime chain.
///      Phase 1: non-payable execute, query, contractInfo, rawQuery.
interface IWasm {
    // -----------------------------------------------------------------------
    // Events
    // -----------------------------------------------------------------------

    /// @notice Emitted when a CosmWasm contract is successfully executed.
    /// @param caller The EVM address of the calling contract/account.
    /// @param contractAddr The bech32 address of the target CosmWasm contract.
    /// @param response The raw JSON response from the CosmWasm execution.
    event WasmExecuted(
        address indexed caller,
        string contractAddr,
        bytes response
    );

    // -----------------------------------------------------------------------
    // State-changing methods
    // -----------------------------------------------------------------------

    /// @notice Execute a CosmWasm contract (non-payable, no funds transfer).
    /// @param contractAddr The bech32 address of the target CosmWasm contract.
    /// @param msg The JSON-encoded execute message.
    /// @return response The raw JSON response bytes from the CosmWasm execution.
    function execute(
        string calldata contractAddr,
        bytes calldata msg
    ) external returns (bytes memory response);

    // -----------------------------------------------------------------------
    // Read-only query methods
    // -----------------------------------------------------------------------

    /// @notice Query a CosmWasm contract (read-only).
    /// @param contractAddr The bech32 address of the target CosmWasm contract.
    /// @param msg The JSON-encoded query message.
    /// @return response The raw JSON response bytes.
    function query(
        string calldata contractAddr,
        bytes calldata msg
    ) external view returns (bytes memory response);

    /// @notice Get metadata about a CosmWasm contract.
    /// @param contractAddr The bech32 address of the target CosmWasm contract.
    /// @return codeId The code ID used to instantiate the contract.
    /// @return creator The bech32 address of the contract creator.
    /// @return admin The bech32 address of the contract admin (empty if none).
    /// @return label The human-readable label assigned at instantiation.
    function contractInfo(
        string calldata contractAddr
    )
        external
        view
        returns (
            uint64 codeId,
            string memory creator,
            string memory admin,
            string memory label
        );

    /// @notice Query a raw storage key from a CosmWasm contract.
    /// @param contractAddr The bech32 address of the target CosmWasm contract.
    /// @param key The raw storage key bytes.
    /// @return value The raw storage value bytes.
    function rawQuery(
        string calldata contractAddr,
        bytes calldata key
    ) external view returns (bytes memory value);
}
