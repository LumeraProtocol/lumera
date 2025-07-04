# Proposal: Upgrading Lumera IBC to v2 (ibc-go v10) and Cross-Chain Service Integration

## Background and Rationale

Lumera’s blockchain currently uses the Cosmos **IBC-Go v8** implementation (IBC spec v1) for inter-chain communication. We propose upgrading to **IBC-Go v10** which introduces **IBC v2** support, while retaining backward compatibility with IBC v1 (classic). IBC v2 (launching with Cosmos SDK 0.50) significantly simplifies the IBC protocol without sacrificing functionality. By upgrading, Lumera can leverage improved cross-chain composability, more flexible packet handling, and better performance for interchain interactions. This upgrade will position Lumera to connect more easily with diverse blockchains (Cosmos chains and beyond) and to expose its unique services (Cascade, Sense, Inference, NFT metadata) to the broader interchain ecosystem.

Key motivations for the upgrade include:

- **Enhanced Cross-Chain Compatibility**: IBC v2 reduces handshake complexity, making it easier to connect with non-Cosmos ecosystems (e.g. Ethereum, Solana). This broadens Lumera’s potential integration partners.
- **Future-Proofing**: Aligning with the latest interchain standards ensures Lumera remains compatible with upcoming Cosmos SDK improvements and IBC features (like cross-chain smart contract calls).
- **Facilitating New Services**: IBC v2’s design is more accommodating to custom application protocols, which is critical for Lumera to offer storage, AI, and NFT services over IBC channels.

In summary, upgrading to IBC-Go v10 (IBC v2) will modernize Lumera’s interop layer, simplify cross-chain interactions, and enable Lumera to act as a provider of advanced interchain services.

## Technical Upgrade Plan (IBC-Go v8 to v10)

Upgrading the IBC module requires careful changes to Lumera’s codebase and configuration. Below we detail the specific technical steps and modifications needed for moving from ibc-go v8 (IBC v1) to ibc-go v10 (IBC v2):

### Dependency and Module Updates

- **Bump IBC-Go Version**: Update Lumera’s Go modules to use ibc-go/v10 (replacing any v8 references).
- **Remove Capability Module**: IBC v2 no longer uses the Cosmos SDK Capability keeper for channel scoping. We will remove the capabilitykeeper from the app, including any scoped keepers for IBC ports. This cleans up app.go by dropping the CapabilityKeeper and all Scoped*Keeper instances related to IBC.
- **Remove Legacy Client Proposal Handler**: Eliminate the registration of the legacy 02-Client proposal handler in governance. In app.go, the upgrade client proposal route (used for updating IBC clients in IBC v1) should be removed, as IBC v2 handles client updates differently.
- **Integrate New Light Clients**: Register the updated light client modules with the IBC client keeper. For example, instantiate the Tendermint light client module via ibctm.NewLightClientModule and add it to the client router. Do the same for any other client types in use (e.g., 08-wasm if Lumera supports Wasm-based clients). This ensures Lumera’s IBC can support the new client abstractions (and any future clients like Ethereum light clients).
- **IBC Module Interface Changes**: Update any IBC keeper constructor calls to match new function signatures in v10. For instance, the IBC keeper and routing setup might now require fewer parameters (since capability and fee middleware are removed) and use a unified router abstraction for channels.

### IBC Application Stack Adjustments

- **IBC Transfer Module (ICS-20) v2**: Wire up the new transfer v2 stack for token transfers. The ibc-transfer module in v10 has a revised implementation (transferv2) to work with IBC v2. In Lumera’s app.go, instead of the old transfer.NewIBCModule, use transferv2.NewIBCModule(app.TransferKeeper) and register it on the IBC router. This ensures that fungible token transfers use the IBC v2 packet format and routing.
- **Remove Fee Middleware (ICS-29)**: In IBC-Go v10, the fee middleware (ICS-29) is no longer wired in the same way. The migration guidelines explicitly instruct chains to remove the ibc-fee module from the stack. Accordingly, delete any IBCFeeKeeper initialization and omit wrapping IBC modules with ibcfee.NewIBCMiddleware. (Relayer incentivization can be revisited later if needed via updated middleware patterns, but for now v2 simplifies the stack.)
- **Callbacks Middleware**: The IBC Callbacks middleware module has moved into the core ibc-go module and its initialization signature has changed. We will import ibccallbacks/v10 and wrap it appropriately around the transfer module if those hooks are needed (ensuring it uses the new ChannelKeeperV2 where required).
- **Interchain Accounts (ICS-27) Modules**: Lumera’s Interchain Accounts host/controller modules should be updated to the v10 API. In practice, this means using the latest icahost and icacontroller implementations and removing any dummy modules or fee wrapping. For example, previously a “no-authz” dummy module and fee middleware wrapped the ICA controller – these should be simplified to just icacontroller.NewIBCMiddleware(app.ICAControllerKeeper). Likewise, instantiate the ICA host with icahost.NewIBCModule alone (no fee wrapper). This aligns ICA with IBC v2’s streamlined channel management.

### Configuration and State Considerations

- **Genesis / AppState**: No explicit genesis field changes are expected solely for IBC v2, since the upgrade is mainly code-level. However, because we remove the Capability module, we should ensure that the on-chain capability store (if present) can be left inert or safely migrated. The upgrade handler can simply drop capability references since IBC channel locking will be handled via the new router logic.
- **Version Negotiation**: IBC-Go v10 introduces an application version negotiation interface for handshake (the NegotiateAppVersion method). Ensure Lumera’s custom IBC app modules (if any) implement this if needed, to agree on versions during channel setup. The standard modules (transfer, ICA, etc.) already handle this.
- **Backward Compatibility Mode**: Out of the box, ibc-go v10 supports both IBC v1 (classic) and v2 protocols for interoperability. We will configure the IBC keeper to support connections with IBC v1 chains. This means Lumera can still open **IBC Classic channels** to chains that have not yet upgraded, ensuring continuity of existing connections. No special config is required beyond not disabling the legacy channel handshake logic present in ibc-go v10 (it remains enabled by default to maintain compatibility [ccvalidators.com] (https://ccvalidators.com/blog/ibc-eureka-the-endgame-of-blockchain-interoperability/)).
- **Testing the Upgrade Path**: Write an upgrade handler (if using Cosmos SDK in-place upgrade) to perform any needed state migrations. This includes removing the now-unused fee module account permission (if defined) and possibly clearing capability indices. We will simulate an upgrade in a dev environment to ensure that after restarting with the new binary, all IBC connections and channels are functional.

### Validation and Testing

After implementing the above changes, thorough testing is essential:

- **Unit Tests**: Run the IBC module’s test suite and add new tests for the modified handshake. Test that opening a channel to a known IBC v1 counterparty works (the handshake should gracefully fall back to v1).
- **Integration Tests**: Set up a local testnet with two Lumera nodes using the new code and attempt to establish IBC connectivity with another chain (or a loopback connection). Validate that ICS-20 token transfers still succeed end-to-end.
- **Relayer Compatibility**: Test with the common IBC relayer (e.g. Hermes or Go Relayer) to ensure it can relay packets to/from Lumera after the upgrade. Relayers may need minor config updates (like recognizing the new channel version string for v2 channels), so we’ll verify this in advance.
- **Performance Benchmarking**: Compare the handshake time and packet throughput before vs. after. We expect improved performance (faster channel setup due to fewer steps). Any issues will be addressed before mainnet deployment.

By completing these technical steps, Lumera’s codebase will be ready to run IBC v2, unlocking the new capabilities while remaining compatible with existing IBC networks.

## Benefits of Upgrading to IBC v2

Upgrading to IBC v2 (ibc-go v10) offers several **concrete benefits** for Lumera, improving both the developer experience and cross-chain functionality:

- **Simplified Handshake & Improved Performance**: IBC v2 reduces the connection handshake from a 10-step process (4 steps each for connection and channel, plus client setup) to just 3 steps. This streamlined startup means faster time-to-connect and less overhead for relayers. It also condenses channels and connections into a single abstraction (the new Router), eliminating the lengthy channel open/confirm dance. The result is quicker integration with other chains and lower latency for starting cross-chain operations.
- **Greater Composability and Upgradability**: In IBC v2, all applications run over a single connection, with dynamic routing by port ID. This means Lumera and a counterparty chain can use one connection to support multiple services or module interactions simultaneously, without needing separate channels for each service. New IBC applications (or new versions) can be added without coordinating a new channel handshake, greatly improving extensibility. For example, Lumera could add a new interchain service post-upgrade and immediately route packets over the existing connection. This unified routing model also avoids the fragmentation of multiple channels and the reliance on off-chain “canonical channel” conventions.
- **Custom Packet Flexibility (Payloads)**: IBC v2 introduces a Payload abstraction in packets. Instead of a single opaque bytes field tied to a specific port/channel, a packet can carry a list of payloads, each with its own destination port, app version, and encoding type. This unlocks flexibility for custom packet designs. Lumera can define custom packet types for its services (if needed) with domain-specific encoding (e.g., using efficient binary codecs or Ethereum ABI encoding for certain interactions). The payload structure even allows combining multiple operations in one packet (e.g., a token transfer plus a callback invocation in one go), enabling atomic cross-chain composability in future releases. In short, Lumera gains more control over how data is packaged and interpreted across chains.
- **Advanced IBC Features Availability**: By moving to the latest IBC implementation, Lumera can easily adopt new IBC features such as Interchain Accounts, Interchain Queries, and upcoming cross-chain contract calls. These existed in IBC v1 but are further refined or easier to use in v2. For instance, Interchain Accounts (ICS-27) allow a chain to control an account on another chain via IBC, which Lumera can use to let other chains invoke its services (or vice versa). Cross-Chain Queries (ICS-31/32) enable direct reading of state from another chain over IBC, useful for fetching NFT metadata or verification results from Lumera. Upgrading ensures Lumera is fully compatible with these modules, which improves cross-chain composability (chains can seamlessly incorporate Lumera’s functionality into their own workflows).
- **Performance and Scalability**: Beyond handshake improvements, IBC v2 is designed with broader ecosystem scalability in mind. The lighter protocol is more amenable to varied environments (even non-Tendermint chains), meaning Lumera can potentially handle more connections in parallel with less overhead. The elimination of redundant fields (like port IDs in the packet, and using only timestamps for timeouts) trims packet size and verification work, which could slightly improve throughput and reduce gas costs per packet on Lumera.
- **Cross-Chain Service Enablement**: Perhaps most importantly, IBC v2 better enables cross-chain services – a core goal for Lumera. With the new capabilities, Lumera can serve as an interchain utility chain providing storage and AI services. For example, improved routing means a single IBC connection to another chain can carry both token transfers and service requests back-and-forth, making it easier to compose multi-step cross-chain transactions (e.g., sending payment and a service request together). This composability was more clunky in IBC v1, where separate channels or sequential transactions were needed.

In summary, upgrading to IBC v2 brings Lumera speed, flexibility, and access to richer interchain functionality. It lays a strong foundation for Lumera to not just participate in the interchain, but to become a **key service provider** within it, thanks to these improvements in protocol design and capabilities.

## Exposing Lumera Features as IBC Services

One of the strategic advantages of upgrading is the ability to expose Lumera’s unique features – **Cascade** (distributed storage), **Sense** (near-duplicate NFT detection), **Inference** (AI/ML agents), and **NFT Metadata** services – as **IBC-compatible services**. This means other blockchains can seamlessly invoke Lumera’s capabilities through standard IBC channels and packets. Below we propose how each feature can be integrated into the IBC framework:

### Cascade: Distributed Storage Service

Cascade is Lumera’s distributed, permanent storage protocol for NFT data, built on RaptorQ fountain codes and a Kademlia DHT. It breaks files into redundant chunks and stores them across the network, ensuring durable NFT storage. To offer Cascade as an interchain service, we can leverage IBC in the following ways:

- **IBC Application Module for Storage**: Implement a custom IBC application (port) on Lumera named, for example, "cascade" or "storage". External chains can open an IBC channel to this port. Through this channel, they send storage requests as IBC packets (containing the data or references to data that needs storage). Lumera’s module on receiving a request will process it by encoding the asset into chunks and storing it via Cascade’s logic. Upon success, Lumera can return an acknowledgement packet with a **storage receipt** – e.g., a content ID or storage tx hash.
- **Data Transfer Considerations**: NFT images or files can be large, so the design must handle chunking. IBC v2’s packet payload improvements allow flexible encoding; we could define a chunked transfer protocol (sending multiple ordered packets carrying file chunks) under the Cascade channel. Alternatively, the requesting chain might first upload the asset to a known location (or send via ICS-20 if it’s tokenized), and then just send a reference (like a hash or URL) to Lumera for retrieval. Lumera’s Cascade module could have the capability to fetch external data given a reference (if security permits) or rely on the relayer to carry the actual payload.
- **Interchain Accounts Alternative**: Another approach is to utilize ICS-27 (Interchain Accounts). A chain could create an account on Lumera and invoke a MsgStoreFile transaction on Lumera through that account. The Cascade logic would execute as if the user directly submitted it on Lumera. The result (e.g., a stored file ID) would be committed to Lumera’s state. The requesting chain can then query this via an interchain query or wait for the transaction result in the acknowledgement. Using ICS-27 ensures we reuse Lumera’s existing transaction flow, though it requires the counterparty chain to support the controller side of interchain accounts.
- **Benefits**: By exposing Cascade over IBC, any NFT-centric chain in the Cosmos ecosystem (and beyond) can utilize permanent, decentralized storage for their NFT media. This mitigates the common problem of NFT assets disappearing from centralized hosts. Lumera could charge fees in PSTL (Pastel token) or another token for storage, possibly handled via an ICS-20 transfer accompanying the request (e.g., attach payment and the store request in one flow). IBC v2’s multi-payload packets might even allow combining payment and request in one packet in the future.

### Sense: Near-Duplicate NFT Detection

Sense is Lumera’s AI-driven protocol for near-duplicate NFT detection and rareness scoring. It generates a fingerprint vector for an image and compares it against a database to produce a Relative Rareness Score (0% = identical, 100% = completely unique). To provide Sense as an IBC service:

- **IBC Request/Response Workflow**: Similar to Cascade, define an IBC application port (e.g., "sense"). Other chains (especially NFT marketplaces or minting platforms) can send a SenseRequest packet containing either the NFT data (image) or a fingerprint of the image. Lumera’s Sense module, upon receiving the request, computes the rareness score by comparing the NFT’s fingerprint against its extensive dataset (which includes NFTs from Pastel/Lumera and possibly other sources). It then returns the result in the packet acknowledgement – e.g., the score and perhaps a list of any detected duplicates or their IDs.
- **Data Input Options**: If sending full image data over IBC is too heavy, an alternative is to require the requesting chain to store the image via Cascade first, then send a Sense request referencing the stored image’s content ID. Since Lumera would then have the image data internally, the Sense module can access it directly for analysis. This two-step approach (store, then analyze) can be orchestrated via IBC: first an ICS-27 call or Cascade packet to store, then an ack or subsequent message triggers Sense. We can also consider allowing a perceptual hash or fingerprint vector to be sent instead of the raw image (if the external chain can compute the same fingerprint algorithm). However, given Sense’s sophisticated deep learning model (10,000+ dimensional fingerprint), it may be preferable to let Lumera compute it for consistency.
- **Interchain Account / Query Integration**: A chain could use an interchain account to call a MsgCheckImage on Lumera (passing an image hash or ID), which triggers Sense. The result could be written to Lumera’s state (e.g., as an event or a temporary record). The requesting chain could then use ICS-31 (cross-chain query) to read the result back if not present in the ack. Alternatively, the transaction’s success ack could carry the primary result.
- **Benefits**: Exposing Sense via IBC means any NFT minting operation on any chain can vet the uniqueness of the NFT before finalizing a mint. For example, an NFT marketplace chain could, upon mint, automatically query Lumera’s Sense service to ensure the content isn’t a copy of an existing NFT. This enhances trust and provenance across the interchain NFT ecosystem. Lumera’s service would effectively act as a decentralized plagiarism checker for digital art, accessible through a trust-minimized channel. This also drives usage of Lumera’s AI capabilities, potentially with fee payment attached per request (which could be handled via a token transfer or deduction of credits – see below).

### Inference: AI Agents

**Inference** is Lumera’s AI/LLM layer that brings advanced machine learning functionalities (like language models and AI-driven analysis) into the network. It integrates with providers such as OpenAI, Anthropic, and others to enable intelligent data processing and content generation in dApps. To make Inference available to other chains via IBC:

- **Unified Service Interface**: Provide an IBC port (e.g., "inference"). Other chains can send AI task requests to this port. A request might specify the type of model or service (e.g., “run text through GPT-4 for summarization” or “classify image for NSFW content”), along with the input data or reference. Lumera’s Inference module would receive the task, route it to an appropriate supernode or external API as configured (Lumera leverages specialized nodes and possibly off-chain APIs for AI), then return the result.
- **Request Payload Format**: Because inference tasks can vary widely, we’d design a flexible message schema – including fields like model_type (or model ID), parameters, and payload. IBC v2’s ability to handle different encoding types is useful here: for instance, we might use JSON or MsgPack encoding for AI requests for readability, or a binary format if needed. The response might be textual (for chatbot replies, etc.) or binary data (for images or other media generated). Splitting large responses over multiple packets may be needed for very large outputs.
- **Stateful vs Stateless Agents**: If Lumera supports stateful AI agents (e.g., an agent that maintains context over multiple calls), we could implement a session identifier in the requests. This would allow a chain to initiate an AI agent session and continue feeding it inputs via subsequent IBC messages. However, managing long-lived sessions over IBC can be complex (timeouts, ordering, etc.), so initially we may focus on stateless query-style requests (each request yields an independent result).
- **Payment and Access Control**: The Inference protocol on Lumera uses a credit system for users (buying inference credits with PSTL). For cross-chain use, Lumera could require that the calling account (or chain) has sufficient credits or attaches payment. One approach is to use ICS-20 token transfers alongside the request: for example, a chain could escrow some PSTL on Lumera or include a fee in the request packet (if we extend the packet format to carry a fee, or simply do two IBC operations back-to-back: one token transfer, one inference request). Another approach is ICS-27: an interchain account from the requester could hold PSTL and spend the credits when making the call on Lumera’s chain.
- **Benefits**: By offering AI as a service, Lumera can become the AI co-processor for the interchain. Chains that lack the resources or desire to integrate large AI models can offload tasks to Lumera. For example, a social media chain might use Lumera to perform content moderation (by sending images/text for analysis), or a DeFi chain might use it for AI-based risk assessments. All of this can happen trustlessly via IBC. Lumera’s inference service thus expands the realm of what cross-chain applications can do, bringing Web2 AI power into Web3 ecosystems without centralized intermediaries.

### NFT Metadata: Interchain Metadata Hub

Lumera can also position itself as a metadata repository and verification layer for NFTs across chains. NFT metadata includes attributes, provenance info, and ownership records that might benefit from being standardized or permanently stored.

- **Metadata Storage and Retrieval**: Lumera could maintain a registry of NFT metadata that is submitted to it via IBC. For example, when an NFT is minted on Chain A, it could send the metadata (properties, description, etc.) to Lumera for permanent storage (possibly alongside the asset itself via Cascade). Lumera would index this under a composite key (like <originChain>/<nftID>). Other chains could later query this metadata to verify an NFT’s details or to display them consistently.
- **Authenticity and Provenance Service**: Because Lumera can aggregate NFT information from multiple chains, it could serve as an authentication oracle. Via IBC, a chain could ask Lumera questions like “Does NFT X on chain A have a valid registration and what’s its metadata hash?” and Lumera can respond with the canonical data. This can prevent malicious actors from forging NFT details on another chain – since Lumera’s records (if widely accepted) become a source of truth. Essentially, Lumera acts as a trustless NFT metadata catalogue for the interchain.
- **Using IBC Features**: Implementing this may use a mix of ICS-27 and ICS-31. To register metadata, an interchain account on Lumera could submit a transaction (MsgRegisterMetadata) carrying the NFT’s info (or a hash of it plus a link to full content stored via Cascade). The transaction would commit the data on Lumera. Then, to retrieve metadata, ICS-31 cross-chain queries could allow any chain to fetch that data by key. If ICS-31 is not yet widely available, we could alternatively allow a direct query request packet (similar to how Sense returns a score) where a chain sends a MetadataQuery packet and Lumera replies with the data in the ack.
- **Integration with Cascade and Sense**: These services complement each other. For instance, upon storing an image via Cascade, Lumera can automatically generate a fingerprint and update the Sense index. Similarly, when metadata is registered, Lumera can ensure the corresponding asset is stored and link them. Other chains might simply call a higher-level “Register NFT” IBC method that triggers storing the asset (Cascade), storing metadata, and running Sense detection – all coordinated on Lumera’s side. Thanks to IBC v2’s design, such a complex flow could be done over one robust connection and even in one combined packet in the future (leveraging multiple payloads for different apps in one atomic submission).
- **Benefits**: Chains get immutable storage and verification of their NFT metadata, enhancing security (no more lost metadata if source chain data is pruned or altered). It fosters an interoperable NFT ecosystem: NFTs on different chains can be verified against a common reference point (Lumera), enabling cross-chain NFT marketplaces or bridges to trust the metadata consistency. Lumera in turn becomes a critical infrastructure node for NFTs, increasing usage of its network and token.

In all of the above, the overarching idea is to transform Lumera into a suite of interchain services accessible via standard IBC channels. Each service (storage, duplicate detection, AI compute, metadata registry) would be exposed in a modular way, either through distinct IBC application ports or via general frameworks like interchain accounts. The upgrade to IBC v2 greatly aids this by making it simpler to manage multiple services over IBC and by providing the necessary flexibility in packet handling.

## Staged Deployment Plan and Backward Compatibility

Upgrading a live blockchain and rolling out new cross-chain services is a complex process. We propose a **phased deployment plan** to ensure a smooth transition:

1. **Development and Internal Testing**:
In this initial phase, the dev team will implement the ibc-go v10 changes on a development branch. Extensive testing (unit tests, simulation, local networks) is conducted as described earlier. The team will also write migration scripts or an upgrade handler to automate state changes (e.g., removing module accounts for fee, capability cleanup). We will verify that a node can upgrade from the old binary to the new one without issues, and that core functionality (transactions, existing modules) remains unaffected. This phase is strictly internal and uses no external connections – focusing on code stability.
2. **Testnet Deployment (IBC v2 readiness)**:
Once confident, we will deploy the new version to a Lumera testnet. If an official public testnet exists, coordinate a specific upgrade height via governance or simply relaunch the testnet if ephemeral. On this testnet, we will:
   - **Connect with Counterparty Chains**: Establish IBC connections with other testnets (e.g., Cosmos Hub’s testnet, Osmosis testnet) to ensure interoperability. This tests backward compatibility: Lumera (v2) should successfully open channels with v1 counterparties, proving that relayers and handshake downgrading work.
   - **Try New Features**: If feasible, spin up a small dummy chain (or use an existing one) to act as a client of Lumera’s services. For example, demonstrate an NFT stored on Chain A being sent to Lumera for Cascade storage and Sense analysis, all on testnet. This will likely involve deploying and testing ICS-27 and ICS-31 functionality in a controlled setting.
   - **Bug Fixes**: Use the findings from testnet to fix any issues. For instance, if relayers encountered errors with the new handshake, adjust parameters or inform relayer developers. Monitor the network for any instability introduced by the new code. 
3. **Audit and Review (Optional)**:
Given the scope of changes (IBC being critical infrastructure), it may be prudent to have an **external audit** of the upgrade implementation, especially the custom logic for new service modules. Auditors would review the IBC upgrade diff as well as the new IBC application modules for Cascade, Sense, etc. This can run in parallel with extended testnet runs. Community testing and bug bounty programs on the testnet could further harden the code.
4. **Mainnet Upgrade Preparation**:
Prior to scheduling the upgrade on mainnet, coordinate with all stakeholders:
    - **Validator Coordination**: Communicate the planned upgrade (block height or governance proposal) well in advance. Provide upgrade instructions and binaries to validators. Emphasize that this upgrade is significant (new IBC version) and encourage them to test their nodes on the testnet or a dry-run.
    - **Relayer Coordination**: Reach out to major relayer operators in the Cosmos ecosystem (who facilitate Lumera’s IBC transactions) and inform them of the upgrade timeline. Since ibc-go v10 still supports IBC classic, relayers can continue operating normally for existing channels. However, to use the new IBC v2 features (like connecting to Ethereum via ZK light clients, etc.), they might need to update their relayer software to versions that know about IBC v2. We ensure that at least one well-maintained relayer (like Hermes) has a release supporting ibc-go v10.
    - **Governance Proposal**: If Lumera uses on-chain governance for upgrades, draft a proposal describing the upgrade (including benefits like those outlined above). Highlight that from an operations perspective, nothing drastic will change initially – e.g., “IBC will continue to function as before for users” to alleviate concerns. Garner support by outlining new possibilities post-upgrade.
5. **Mainnet Upgrade Execution**:
At the scheduled time (or height), the Lumera mainnet will undergo the upgrade. Validators will switch to the new binary containing ibc-go v10. Post-upgrade:
    - **Stability Checks**: Confirm that the chain produces blocks and that all modules (bank, staking, etc.) are running fine. Then specifically check the IBC module: all existing IBC channels and clients should be listed and active. Because we removed the capability module, we will verify that existing channel state is accessible (ibc-go v10 should internally manage channel references now). If any existing channel is non-functional, we might coordinate with the connected chain to reopen a connection using the new protocol, but this is unlikely since backward support is in place.
    - **Backward Compatibility**: Ensure that **IBC Classic channels continue seamlessly**. According to IBC developers, Cosmos chains upgrading to v2 see no interruption in normal IBC operations – light clients still verify proofs, relayers still relay packets in the same way. We will test a token transfer over an old channel as soon as the chain is up to confirm this. In the unlikely event of an issue (say, the channel capability was lost), our fallback plan would involve using governance or admin authority to recreate channel bindings or ask users to migrate to a new channel. However, given the design of ibc-go v10, such disruption is not expected.
    - **Monitoring**: Closely monitor the mempool and blockchain for any IBC errors, and watch relayer logs. It’s critical to catch any unexpected bugs early. We’ll also keep communication open with relayers for quick troubleshooting.
6. **Phased Rollout of New Services**:
With the core upgrade live, we can begin enabling the new IBC services of Lumera in phases:
    - **Enable ICS-27 Interchain Accounts**: Activate the ICA host module on Lumera via a parameter change or simply by usage (if compiled in, it may be live by default). Announce that other chains can now open interchain accounts on Lumera. Initially, whitelist some partner chains to try out controlling Lumera accounts to use Cascade or Sense. This careful approach ensures the feature is used in a controlled manner before broad exposure.
    - **Offer Cascade and Sense on Testnet/Mainnet**: Possibly launch a beta program where select projects integrate with Cascade or Sense via IBC. For example, an NFT marketplace could try storing their new mints on Lumera and retrieving the Sense score. Their feedback will help refine the modules. On-chain, this might involve opening dedicated IBC channels for those services (if not using ICA). We will monitor these channels for correct behavior (packet flow, acknowledgements). Over time, as confidence grows, make these services publicly available to any chain that connects.
    - **Documentation and SDKs**: To encourage adoption, produce documentation and possibly SDK support for Lumera’s IBC endpoints (e.g., a developer guide on how another chain can send a Cascade storage request, including code examples). This will likely be done as we enable the services.
    - **Inference Service Rollout**: The AI inference service might be introduced a bit later given its complexity. We may run a pilot where Lumera connects with a specific chain (or an application) that wants to leverage AI. During this stage, we’ll fine-tune pricing, performance, and security (since calling external AI APIs has different considerations). Once stable, open it up as an interchain service similar to Cascade and Sense.
    - **NFT Metadata and Query Services**: Establish Lumera as an interchain NFT metadata hub by onboarding a few chains to push their metadata to Lumera. Ensure that our cross-chain query implementation (if using ICS-31) is working so those chains can fetch the info. Over time, advertise this as a general service.
7. **Post-Upgrade Evaluation and Maintenance**:
After full deployment, conduct an evaluation of the upgrade’s impact:
    - **Performance Metrics**: Review if block processing times for IBC transactions improved and if the throughput of IBC packets increased. Check if relayer fees or latencies dropped due to the more efficient protocol.
    - **Reliability**: Gather data on any IBC packet failures or unexpected behavior. If the new multi-app single-connection approach caused any hiccups, patch as needed. For instance, ensure that if one service’s logic fails, it doesn’t block the whole connection’s packets (the router should isolate apps by port).
    - **Backward Compatibility Period**: Plan to support IBC classic channels for as long as needed. Cosmos chains will gradually all upgrade to v2, but in the interim, Lumera may operate with some classic channels. We’ll maintain code paths for IBC v1 as provided by ibc-go v10 until we are sure they are no longer used. Thereafter, we might deprecate those for cleanliness (perhaps in a future release).
    - **Security Monitoring**: New features mean new surface area. Keep a close watch on cross-chain transactions invoking Lumera’s services to detect any abuse or vulnerabilities (e.g., denial-of-service via spammy requests, or attempts to exploit the AI services). Rate limiting or fee requirements may be adjusted based on real usage patterns.
  
Throughout this process, **communication is key**. We will keep the Lumera community and partners informed at each stage, with clear documentation of changes. By proceeding methodically through testnet trials, mainnet upgrade, and gradual feature rollout, we aim to minimize disruptions and ensure that the transition to IBC v2 and the launch of cross-chain services are successful. Lumera will emerge from this process not only up-to-date with the latest interchain protocol, but positioned as a **leader in cross-chain utility**, providing valuable services across the interchain ecosystem.

## Setting Up IBC Communication with Other Chains

After Lumera is upgraded to use IBC-Go v10 (IBC v2), establishing connections with other chains requires configuring ports, using relayers, and setting up IBC clients, connections, and channels. Here's a step-by-step guide:

### ✅ Prerequisites (for Lumera and the remote chain)

- Both chains must:
  - Be running Cosmos SDK with IBC-Go (v1 or v2).
  - Enable required IBC modules in `app.go`.
  - Be publicly accessible via RPC, GRPC, and P2P ports.
  - Have relayer access (Hermes or Go Relayer).

### 🔧 Step 1: Configure IBC Ports and Applications in `app.go`

```go
ibcRouter.AddRoute("transfer", transferIBCModule)
ibcRouter.AddRoute("cascade", cascadeIBCModule)
ibcRouter.AddRoute("sense", senseIBCModule)
ibcRouter.AddRoute("inference", inferenceIBCModule)
ibcRouter.AddRoute(icacontrollertypes.ModuleName, icacontrollerIBCModule)
ibcRouter.AddRoute(icahosttypes.ModuleName, icahostIBCModule)
```

### 🔁 Step 2: Relayer Configuration (Hermes example)

#### Create Hermes config

The official website for the Hermes IBC Relayer is [hermes.informal.systems](https://hermes.informal.systems).
```toml
[[chains]]
id = "lumera"
rpc_addr = "http://<lumera-node>:26657"
grpc_addr = "http://<lumera-node>:9090"
websocket_addr = "ws://<lumera-node>:26657/websocket"
account_prefix = "lum"  # or "tP" for testnet
...

[[chains]]
id = "osmosis-1"
rpc_addr = "https://rpc-osmosis.blockapsis.com:443"
grpc_addr = "https://grpc-osmosis.blockapsis.com:443"
websocket_addr = "wss://rpc-osmosis.blockapsis.com/websocket"
account_prefix = "osmo"
...
```

#### Add keys

```bash
hermes keys add --chain lumera --mnemonic-file lumera.mnemonic
hermes keys add --chain osmosis-1 --mnemonic-file osmosis.mnemonic
```

### 🔌 Step 3: Create IBC Client, Connection, Channel with Osmosis

```bash
# Create client
hermes create client --host-chain lumera --counterparty-chain osmosis-1

# Create connection
hermes create connection --a-chain lumera --b-chain osmosis-1

# Create channel for ICS-20
hermes create channel --a-chain lumera --a-port transfer --b-chain osmosis-1 --b-port transfer --order unordered --channel-version transfer

# Or for a custom service (e.g., cascade)
hermes create channel --a-chain lumera --a-port cascade --b-chain osmosis-1 --b-port cascade --order ordered --channel-version 1
```

### 🧪 Step 4: Test Packet Relay

- Send a transaction (ICS-20, Cascade request, etc.)
- Observe relayer processing packet and acknowledgment

### 📦 Step 5: Optional — Set Up Interchain Accounts (ICS-27)

An **Interchain Account** allows Chain A (*the controller chain*) to:

- Create and control an account on Chain B (*the host chain*)
- Submit arbitrary Cosmos SDK messages (like MsgSend, MsgDelegate, or even custom messages)
- Do this over IBC, securely and permissionlessly

So instead of Chain A asking users to send tokens or interact with Chain B manually, Chain A can act on their behalf directly on Chain B.
For example, some other chain could create an ICA on Lumera, then invoke MsgStoreFile (Cascade), MsgAnalyzeImage (Sense) or MsgRunInference (Inference).

1. **Create ICA channel**:

    ```bash
    hermes create channel --a-port icacontroller --b-port icahost --order ordered --channel-version icacontroller-1
    ```

2. Send `MsgSendTx` from controller chain to Lumera via ICA
3. Lumera processes and emits event or result

### ✅ Summary
| Task | Who | Command / Config |
|------|-----|------------------|
| Enable IBC apps | Lumera devs | `app.go`, add routes |
| Run relayer | Operator | Hermes / Go Relayer setup |
| Add keys | Operator | `hermes keys add` |
| Open connection/channel | Operator | Hermes CLI |
| Test services | Any | Send IBC tx, confirm ack |

These steps ensure a smooth and secure IBC setup for cross-chain communication between Lumera and partner chains like Osmosis.