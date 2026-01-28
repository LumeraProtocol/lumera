# Benefits of Upgrading IBC for Lumera

## Benefits of upgrading to IBC v2 (IBC-Go v10)

- **Backward compatibility**: Keep IBC v1 channels while enabling IBC v2, so Lumera can upgrade without forcing counterparties to move immediately.
- **Simplified handshakes**: Reduce connection and channel setup time, lowering relayer overhead and speeding new integrations.
- **Unified routing**: Support multiple Lumera services (Cascade, Sense, Inference, NFT metadata) over a single connection, improving composability and reducing channel sprawl.
- **Payload flexibility**: Allow app-specific encodings and multi-action workflows (e.g., payment + service request in one flow) with less protocol friction.
- **Faster feature adoption**: Enable ICA, queries, and cross-chain calls without reworking the IBC stack as new apps are added.
- **Lower maintenance risk**: Trim the IBC module stack, reducing upgrade risk during the Cosmos SDK 0.53.5 transition while improving long-term scalability.

## Benefits of upgrading to IBC-Go v10.5.0

- **Upstream fixes**: Pick up v10-series fixes and maintenance improvements, reducing the risk of carrying known IBC bugs.
- **Developer ergonomics**: Use newer helper APIs (including v1/v2 event parsing) to reduce custom code in Lumera.
- **Ecosystem alignment**: Stay on the latest v10 patch level for relayer compatibility and support.
