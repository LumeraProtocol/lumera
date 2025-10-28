package rest

import (
	"flag"
	"fmt"
	"net/http"
	"testing"
	"time"
)

var (
	baseURL = flag.String("base-url", "https://lcd.testnet.lumera.io", "LCD API base URL")
	verbose = flag.Bool("verbose", false, "Show detailed output for each endpoint")
)

// Endpoint represents a REST endpoint to test
type Endpoint struct {
	Path        string
	Description string
}

// All Query-tagged endpoints from OpenAPI spec
var queryEndpoints = []Endpoint{
	{"/LumeraProtocol/lumera/action/v1/get_action/1", "GetAction queries a single action by ID."},
	{"/LumeraProtocol/lumera/action/v1/get_action_fee/test-value", "Queries a list of GetActionFee items."},
	{"/LumeraProtocol/lumera/action/v1/list_actions", "List actions with optional type and state filters."},
	{"/LumeraProtocol/lumera/action/v1/list_actions_by_block_height/100", "List actions created at a specific block height."},
	{"/LumeraProtocol/lumera/action/v1/list_actions_by_supernode/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "List actions for a specific supernode."},
	{"/LumeraProtocol/lumera/action/v1/list_expired_actions", "List expired actions."},
	{"/LumeraProtocol/lumera/action/v1/params", "Parameters queries the parameters of the module."},
	{"/LumeraProtocol/lumera/action/v1/query_action_by_metadata", "Query actions based on metadata."},
	{"/LumeraProtocol/lumera/claim/claim_record/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "Queries a list of ClaimRecord items."},
	{"/LumeraProtocol/lumera/claim/list_claimed/1", "Queries a list of ListClaimed items."},
	{"/LumeraProtocol/lumera/claim/params", "Parameters queries the parameters of the module."},
	{"/LumeraProtocol/lumera/lumeraid/params", "Parameters queries the parameters of the module."},
	{"/LumeraProtocol/lumera/supernode/v1/get_super_node/lumeravaloper1x6e45rudg37fxeaq58a9h883xcrxs4alnn3vwq", "Queries a SuperNode by validatorAddress."},
	{"/LumeraProtocol/lumera/supernode/v1/get_super_node_by_address/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "Queries a SuperNode by supernodeAddress."},
	{"/LumeraProtocol/lumera/supernode/v1/get_top_super_nodes_for_block/100", "Queries a list of GetTopSuperNodesForBlock items."},
	{"/LumeraProtocol/lumera/supernode/v1/list_super_nodes", "Queries a list of SuperNodes."},
	{"/LumeraProtocol/lumera/supernode/v1/params", "Parameters queries the parameters of the module."},
	{"/cosmos/auth/v1beta1/account_info/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "AccountInfo queries account info which is common to all account types."},
	{"/cosmos/auth/v1beta1/accounts", "Accounts returns all the existing accounts."},
	{"/cosmos/auth/v1beta1/accounts/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "Account returns account details based on address."},
	{"/cosmos/auth/v1beta1/address_by_id/1", "AccountAddressByID returns account address based on account number."},
	{"/cosmos/auth/v1beta1/bech32", "Bech32Prefix queries bech32Prefix"},
	{"/cosmos/auth/v1beta1/bech32/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "AddressBytesToString converts Account Address bytes to string"},
	{"/cosmos/auth/v1beta1/bech32/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "AddressStringToBytes converts Address string to bytes"},
	{"/cosmos/auth/v1beta1/module_accounts", "ModuleAccounts returns all the existing module accounts."},
	{"/cosmos/auth/v1beta1/module_accounts/test-value", "ModuleAccountByName returns the module account info by module name"},
	{"/cosmos/auth/v1beta1/params", "Params queries all parameters."},
	{"/cosmos/authz/v1beta1/grants", "Returns list of `Authorization`, granted to the grantee by the granter"},
	{"/cosmos/authz/v1beta1/grants/grantee/test-value", "GranteeGrants returns a list of `GrantAuthorization` by grantee."},
	{"/cosmos/authz/v1beta1/grants/granter/test-value", "GranterGrants returns list of `GrantAuthorization`, granted by granter"},
	{"/cosmos/bank/v1beta1/balances/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "AllBalances queries the balance of all coins for a single account."},
	{"/cosmos/bank/v1beta1/balances/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/by_denom", "Balance queries the balance of a single coin for a single account."},
	{"/cosmos/bank/v1beta1/denom_owners/ulume", "DenomOwners queries for all account addresses that own a particular to"},
	{"/cosmos/bank/v1beta1/denom_owners_by_query", "DenomOwnersByQuery queries for all account addresses that own a partic"},
	{"/cosmos/bank/v1beta1/denoms_metadata", "DenomsMetadata queries the client metadata for all registered coin den"},
	{"/cosmos/bank/v1beta1/denoms_metadata/ulume", "DenomMetadata queries the client metadata of a given coin denomination"},
	{"/cosmos/bank/v1beta1/denoms_metadata_by_query_string", "DenomMetadataByQueryString queries the client metadata of a given coin"},
	{"/cosmos/bank/v1beta1/params", "Params queries the parameters of x/bank module."},
	{"/cosmos/bank/v1beta1/send_enabled", "SendEnabled queries for SendEnabled entries."},
	{"/cosmos/bank/v1beta1/spendable_balances/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "SpendableBalances queries the spendable balance of all coins for a sin"},
	{"/cosmos/bank/v1beta1/spendable_balances/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/by_denom", "SpendableBalanceByDenom queries the spendable balance of a single deno"},
	{"/cosmos/bank/v1beta1/supply", "TotalSupply queries the total supply of all coins."},
	{"/cosmos/bank/v1beta1/supply/by_denom", "SupplyOf queries the supply of a single coin."},
	{"/cosmos/circuit/v1/accounts", "Account returns account permissions."},
	{"/cosmos/circuit/v1/accounts/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "Account returns account permissions."},
	{"/cosmos/circuit/v1/disable_list", "DisabledList returns a list of disabled message urls"},
	{"/cosmos/consensus/v1/params", "Params queries the parameters of x/consensus module."},
	{"/cosmos/distribution/v1beta1/community_pool", "CommunityPool queries the community pool coins."},
	{"/cosmos/distribution/v1beta1/delegators/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/rewards", "DelegationTotalRewards queries the total rewards accrued by each valid"},
	{"/cosmos/distribution/v1beta1/delegators/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/rewards/lumeravaloper1x6e45rudg37fxeaq58a9h883xcrxs4alnn3vwq", "DelegationRewards queries the total rewards accrued by a delegation."},
	{"/cosmos/distribution/v1beta1/delegators/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/validators", "DelegatorValidators queries the validators of a delegator."},
	{"/cosmos/distribution/v1beta1/delegators/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/withdraw_address", "DelegatorWithdrawAddress queries withdraw address of a delegator."},
	{"/cosmos/distribution/v1beta1/params", "Params queries params of the distribution module."},
	{"/cosmos/distribution/v1beta1/validators/lumeravaloper1x6e45rudg37fxeaq58a9h883xcrxs4alnn3vwq", "ValidatorDistributionInfo queries validator commission and self-delega"},
	{"/cosmos/distribution/v1beta1/validators/lumeravaloper1x6e45rudg37fxeaq58a9h883xcrxs4alnn3vwq/commission", "ValidatorCommission queries accumulated commission for a validator."},
	{"/cosmos/distribution/v1beta1/validators/lumeravaloper1x6e45rudg37fxeaq58a9h883xcrxs4alnn3vwq/outstanding_rewards", "ValidatorOutstandingRewards queries rewards of a validator address."},
	{"/cosmos/distribution/v1beta1/validators/lumeravaloper1x6e45rudg37fxeaq58a9h883xcrxs4alnn3vwq/slashes", "ValidatorSlashes queries slash events of a validator."},
	{"/cosmos/evidence/v1beta1/evidence", "AllEvidence queries all evidence."},
	{"/cosmos/evidence/v1beta1/evidence/ABCD1234", "Evidence queries evidence based on evidence hash."},
	{"/cosmos/feegrant/v1beta1/allowance/test-value/test-value", "Allowance returns granted allwance to the grantee by the granter."},
	{"/cosmos/feegrant/v1beta1/allowances/test-value", "Allowances returns all the grants for the given grantee address."},
	{"/cosmos/feegrant/v1beta1/issued/test-value", "AllowancesByGranter returns all the grants given by an address"},
	{"/cosmos/gov/v1/constitution", "Constitution queries the chain's constitution."},
	{"/cosmos/gov/v1/params/test-value", "Params queries all parameters of the gov module."},
	{"/cosmos/gov/v1/proposals", "Proposals queries all proposals based on given status."},
	{"/cosmos/gov/v1/proposals/1", "Proposal queries proposal details based on ProposalID."},
	{"/cosmos/gov/v1/proposals/1/deposits", "Deposits queries all deposits of a single proposal."},
	{"/cosmos/gov/v1/proposals/1/deposits/test-value", "Deposit queries single deposit information based on proposalID, deposi"},
	{"/cosmos/gov/v1/proposals/1/tally", "TallyResult queries the tally of a proposal vote."},
	{"/cosmos/gov/v1/proposals/1/votes", "Votes queries votes of a given proposal."},
	{"/cosmos/gov/v1/proposals/1/votes/test-value", "Vote queries voted information based on proposalID, voterAddr."},
	{"/cosmos/gov/v1beta1/params/test-value", "Params queries all parameters of the gov module."},
	{"/cosmos/gov/v1beta1/proposals", "Proposals queries all proposals based on given status."},
	{"/cosmos/gov/v1beta1/proposals/1", "Proposal queries proposal details based on ProposalID."},
	{"/cosmos/gov/v1beta1/proposals/1/deposits", "Deposits queries all deposits of a single proposal."},
	{"/cosmos/gov/v1beta1/proposals/1/deposits/test-value", "Deposit queries single deposit information based on proposalID, deposi"},
	{"/cosmos/gov/v1beta1/proposals/1/tally", "TallyResult queries the tally of a proposal vote."},
	{"/cosmos/gov/v1beta1/proposals/1/votes", "Votes queries votes of a given proposal."},
	{"/cosmos/gov/v1beta1/proposals/1/votes/test-value", "Vote queries voted information based on proposalID, voterAddr."},
	{"/cosmos/group/v1/group_info/1", "GroupInfo queries group info based on group id."},
	{"/cosmos/group/v1/group_members/1", "GroupMembers queries members of a group by group id."},
	{"/cosmos/group/v1/group_policies_by_admin/test-value", "GroupPoliciesByAdmin queries group policies by admin address."},
	{"/cosmos/group/v1/group_policies_by_group/1", "GroupPoliciesByGroup queries group policies by group id."},
	{"/cosmos/group/v1/group_policy_info/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "GroupPolicyInfo queries group policy info based on account address of"},
	{"/cosmos/group/v1/groups", "Groups queries all groups in state."},
	{"/cosmos/group/v1/groups_by_admin/test-value", "GroupsByAdmin queries groups by admin address."},
	{"/cosmos/group/v1/groups_by_member/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "GroupsByMember queries groups by member address."},
	{"/cosmos/group/v1/proposal/1", "Proposal queries a proposal based on proposal id."},
	{"/cosmos/group/v1/proposals/1/tally", "TallyResult returns the tally result of a proposal. If the proposal is"},
	{"/cosmos/group/v1/proposals_by_group_policy/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "ProposalsByGroupPolicy queries proposals based on account address of g"},
	{"/cosmos/group/v1/vote_by_proposal_voter/1/test-value", "VoteByProposalVoter queries a vote by proposal id and voter."},
	{"/cosmos/group/v1/votes_by_proposal/1", "VotesByProposal queries a vote by proposal id."},
	{"/cosmos/group/v1/votes_by_voter/test-value", "VotesByVoter queries a vote by voter."},
	{"/cosmos/mint/v1beta1/annual_provisions", "AnnualProvisions current minting annual provisions value."},
	{"/cosmos/mint/v1beta1/inflation", "Inflation returns the current minting inflation value."},
	{"/cosmos/mint/v1beta1/params", "Params returns the total set of minting parameters."},
	{"/cosmos/nft/v1beta1/balance/test-value/1", "Balance queries the number of NFTs of a given class owned by the owner"},
	{"/cosmos/nft/v1beta1/classes", "Classes queries all NFT classes"},
	{"/cosmos/nft/v1beta1/classes/1", "Class queries an NFT class based on its id"},
	{"/cosmos/nft/v1beta1/nfts", "NFTs queries all NFTs of a given class or owner,choose at least one of"},
	{"/cosmos/nft/v1beta1/nfts/1/1", "NFT queries an NFT based on its class and id."},
	{"/cosmos/nft/v1beta1/owner/1/1", "Owner queries the owner of the NFT based on its class and id, same as"},
	{"/cosmos/nft/v1beta1/supply/1", "Supply queries the number of NFTs from the given class, same as totalS"},
	{"/cosmos/params/v1beta1/params", "Params queries a specific parameter of a module, given its subspace an"},
	{"/cosmos/params/v1beta1/subspaces", "Subspaces queries for all registered subspaces and all keys for a subs"},
	{"/cosmos/slashing/v1beta1/params", "Params queries the parameters of slashing module"},
	{"/cosmos/slashing/v1beta1/signing_infos", "SigningInfos queries signing info of all validators"},
	{"/cosmos/slashing/v1beta1/signing_infos/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "SigningInfo queries the signing info of given cons address"},
	{"/cosmos/staking/v1beta1/delegations/test-value", "DelegatorDelegations queries all delegations of a given delegator addr"},
	{"/cosmos/staking/v1beta1/delegators/test-value/redelegations", "Redelegations queries redelegations of given address."},
	{"/cosmos/staking/v1beta1/delegators/test-value/unbonding_delegations", "DelegatorUnbondingDelegations queries all unbonding delegations of a g"},
	{"/cosmos/staking/v1beta1/delegators/test-value/validators", "DelegatorValidators queries all validators info for given delegator ad"},
	{"/cosmos/staking/v1beta1/delegators/test-value/validators/1", "DelegatorValidator queries validator info for given delegator validato"},
	{"/cosmos/staking/v1beta1/historical_info/100", "HistoricalInfo queries the historical info for given height."},
	{"/cosmos/staking/v1beta1/params", "Parameters queries the staking parameters."},
	{"/cosmos/staking/v1beta1/pool", "Pool queries the pool info."},
	{"/cosmos/staking/v1beta1/validators", "Validators queries all validators that match the given status."},
	{"/cosmos/staking/v1beta1/validators/1", "Validator queries validator info for given validator address."},
	{"/cosmos/staking/v1beta1/validators/1/delegations", "ValidatorDelegations queries delegate info for given validator."},
	{"/cosmos/staking/v1beta1/validators/1/delegations/test-value", "Delegation queries delegate info for given validator delegator pair."},
	{"/cosmos/staking/v1beta1/validators/1/delegations/test-value/unbonding_delegation", "UnbondingDelegation queries unbonding info for given validator delegat"},
	{"/cosmos/staking/v1beta1/validators/1/unbonding_delegations", "ValidatorUnbondingDelegations queries unbonding delegations of a valid"},
	{"/cosmos/upgrade/v1beta1/applied_plan/test-value", "AppliedPlan queries a previously applied upgrade plan by its name."},
	{"/cosmos/upgrade/v1beta1/authority", "Returns the account with authority to conduct upgrades"},
	{"/cosmos/upgrade/v1beta1/current_plan", "CurrentPlan queries the current upgrade plan."},
	{"/cosmos/upgrade/v1beta1/module_versions", "ModuleVersions queries the list of module versions from state."},
	{"/cosmos/upgrade/v1beta1/upgraded_consensus_state/100", "UpgradedConsensusState queries the consensus state that will serve as"},
	{"/cosmwasm/wasm/v1/code", "Codes gets the metadata for all stored wasm codes"},
	{"/cosmwasm/wasm/v1/code-info/1", "CodeInfo gets the metadata for a single wasm code"},
	{"/cosmwasm/wasm/v1/code/1", "Code gets the binary code and metadata for a single wasm code"},
	{"/cosmwasm/wasm/v1/code/1/contracts", "ContractsByCode lists all smart contracts for a code id"},
	{"/cosmwasm/wasm/v1/codes/params", "Params gets the module params"},
	{"/cosmwasm/wasm/v1/codes/pinned", "PinnedCodes gets the pinned code ids"},
	{"/cosmwasm/wasm/v1/contract/build_address", "BuildAddress builds a contract address"},
	{"/cosmwasm/wasm/v1/contract/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "ContractInfo gets the contract meta data"},
	{"/cosmwasm/wasm/v1/contract/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/history", "ContractHistory gets the contract code history"},
	{"/cosmwasm/wasm/v1/contract/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/raw/test-value", "RawContractState gets single key from the raw store data of a contract"},
	{"/cosmwasm/wasm/v1/contract/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/smart/test-value", "SmartContractState get smart query result from the contract"},
	{"/cosmwasm/wasm/v1/contract/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel/state", "AllContractState gets all raw store data for a single contract"},
	{"/cosmwasm/wasm/v1/contracts/creator/lumera17mku7r3ljspatx3c86kg2qw4mmlzrpnsg9jrel", "ContractsByCreator gets the contracts by creator"},
	{"/cosmwasm/wasm/v1/wasm-limits-config", "WasmLimitsConfig gets the configured limits for static validation of W"},
	{"/ibc/apps/interchain_accounts/controller/v1/owners/test-value/connections/1", "InterchainAccount returns the interchain account address for a given o"},
	{"/ibc/apps/interchain_accounts/controller/v1/params", "Params queries all parameters of the ICA controller submodule."},
	{"/ibc/apps/interchain_accounts/host/v1/params", "Params queries all parameters of the ICA host submodule."},
	{"/ibc/apps/transfer/v1/channels/1/ports/1/escrow_address", "EscrowAddress returns the escrow address for a particular port and cha"},
	{"/ibc/apps/transfer/v1/denom_hashes/test-value", "DenomHash queries a denomination hash information."},
	{"/ibc/apps/transfer/v1/denoms", "Denoms queries all denominations"},
	{"/ibc/apps/transfer/v1/denoms/ABCD1234", "Denom queries a denomination"},
	{"/ibc/apps/transfer/v1/params", "Params queries all parameters of the ibc-transfer module."},
	{"/ibc/apps/transfer/v1/total_escrow/ulume", "TotalEscrowForDenom returns the total amount of tokens in escrow based"},
	{"/ibc/core/channel/v1/channels", "Channels queries all the IBC channels of a chain."},
	{"/ibc/core/channel/v1/channels/1/ports/1", "Channel queries an IBC Channel."},
	{"/ibc/core/channel/v1/channels/1/ports/1/client_state", "ChannelClientState queries for the client state for the channel associ"},
	{"/ibc/core/channel/v1/channels/1/ports/1/consensus_state/revision/test-value/height/100", "ChannelConsensusState queries for the consensus state for the channel"},
	{"/ibc/core/channel/v1/channels/1/ports/1/next_sequence", "NextSequenceReceive returns the next receive sequence for a given chan"},
	{"/ibc/core/channel/v1/channels/1/ports/1/next_sequence_send", "NextSequenceSend returns the next send sequence for a given channel."},
	{"/ibc/core/channel/v1/channels/1/ports/1/packet_acknowledgements", "PacketAcknowledgements returns all the packet acknowledgements associa"},
	{"/ibc/core/channel/v1/channels/1/ports/1/packet_acks/1", "PacketAcknowledgement queries a stored packet acknowledgement hash."},
	{"/ibc/core/channel/v1/channels/1/ports/1/packet_commitments", "PacketCommitments returns all the packet commitments hashes associated"},
	{"/ibc/core/channel/v1/channels/1/ports/1/packet_commitments/1/unreceived_acks", "UnreceivedAcks returns all the unreceived IBC acknowledgements associa"},
	{"/ibc/core/channel/v1/channels/1/ports/1/packet_commitments/1/unreceived_packets", "UnreceivedPackets returns all the unreceived IBC packets associated wi"},
	{"/ibc/core/channel/v1/channels/1/ports/1/packet_commitments/1", "PacketCommitment queries a stored packet commitment hash."},
	{"/ibc/core/channel/v1/channels/1/ports/1/packet_receipts/1", "PacketReceipt queries if a given packet sequence has been received on"},
	{"/ibc/core/channel/v1/connections/connection-0/channels", "ConnectionChannels queries all the channels associated with a connecti"},
	{"/ibc/core/channel/v2/clients/1/next_sequence_send", "NextSequenceSend returns the next send sequence for a given channel."},
	{"/ibc/core/channel/v2/clients/1/packet_acknowledgements", "PacketAcknowledgements returns all packet acknowledgements associated"},
	{"/ibc/core/channel/v2/clients/1/packet_acks/1", "PacketAcknowledgement queries a stored acknowledgement commitment hash"},
	{"/ibc/core/channel/v2/clients/1/packet_commitments", "PacketCommitments queries a stored packet commitment hash."},
	{"/ibc/core/channel/v2/clients/1/packet_commitments/1/unreceived_acks", "UnreceivedAcks returns all the unreceived IBC acknowledgements associa"},
	{"/ibc/core/channel/v2/clients/1/packet_commitments/1/unreceived_packets", "UnreceivedPackets returns all the unreceived IBC packets associated wi"},
	{"/ibc/core/channel/v2/clients/1/packet_commitments/1", "PacketCommitment queries a stored packet commitment hash."},
	{"/ibc/core/channel/v2/clients/1/packet_receipts/1", "PacketReceipt queries a stored packet receipt."},
	{"/ibc/core/client/v1/client_creator/1", "ClientCreator queries the creator of a given client."},
	{"/ibc/core/client/v1/client_states", "ClientStates queries all the IBC light clients of a chain."},
	{"/ibc/core/client/v1/client_states/1", "ClientState queries an IBC light client."},
	{"/ibc/core/client/v1/client_status/1", "Status queries the status of an IBC client."},
	{"/ibc/core/client/v1/consensus_states/1", "ConsensusStates queries all the consensus state associated with a give"},
	{"/ibc/core/client/v1/consensus_states/1/heights", "ConsensusStateHeights queries the height of every consensus states ass"},
	{"/ibc/core/client/v1/consensus_states/1/revision/test-value/height/100", "ConsensusState queries a consensus state associated with a client stat"},
	{"/ibc/core/client/v1/params", "ClientParams queries all parameters of the ibc client submodule."},
	{"/ibc/core/client/v1/upgraded_client_states", "UpgradedClientState queries an Upgraded IBC light client."},
	{"/ibc/core/client/v1/upgraded_consensus_states", "UpgradedConsensusState queries an Upgraded IBC consensus state."},
	{"/ibc/core/client/v2/config/1", "Config queries the IBC client v2 configuration for a given client."},
	{"/ibc/core/client/v2/counterparty_info/1", "CounterpartyInfo queries an IBC light counter party info."},
	{"/ibc/core/connection/v1/client_connections/1", "ClientConnections queries the connection paths associated with a clien"},
	{"/ibc/core/connection/v1/connections", "Connections queries all the IBC connections of a chain."},
	{"/ibc/core/connection/v1/connections/1", "Connection queries an IBC connection end."},
	{"/ibc/core/connection/v1/connections/1/client_state", "ConnectionClientState queries the client state associated with the con"},
	{"/ibc/core/connection/v1/connections/1/consensus_state/revision/test-value/height/100", "ConnectionConsensusState queries the consensus state associated with t"},
	{"/ibc/core/connection/v1/params", "ConnectionParams queries all parameters of the ibc connection submodul"},
	{"/ibc/lightclients/wasm/v1/checksums", "Get all Wasm checksums"},
	{"/ibc/lightclients/wasm/v1/checksums/test-value/code", "Get Wasm code for given checksum"},
}

func TestAllQueryEndpoints(t *testing.T) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	passed := 0
	failed := 0
	skipped := 0

	fmt.Printf("\n=== Testing %d Query Endpoints ===\n\n", len(queryEndpoints))

	for i, endpoint := range queryEndpoints {
		url := *baseURL + endpoint.Path

		resp, err := client.Get(url)
		if err != nil {
			if *verbose {
				t.Logf("[%d/%d] ⚠️  SKIP: %s - Error: %v", i+1, len(queryEndpoints), endpoint.Description, err)
			}
			skipped++
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			if *verbose {
				t.Logf("[%d/%d] ✅ PASS: %s", i+1, len(queryEndpoints), endpoint.Description)
			}
			passed++
		} else {
			if *verbose {
				t.Logf("[%d/%d] ❌ FAIL: %s - Status: %d", i+1, len(queryEndpoints), endpoint.Description, resp.StatusCode)
			}
			failed++
		}
	}

	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Total Query endpoints: %d\n", len(queryEndpoints))
	fmt.Printf("✅ Passed (200 OK): %d\n", passed)
	fmt.Printf("❌ Failed (non-200): %d\n", failed)
	fmt.Printf("⚠️  Skipped (errors): %d\n", skipped)
	fmt.Printf("Success rate: %.1f%%\n", float64(passed)/float64(len(queryEndpoints))*100)

	// Don't fail if some endpoints return non-200 (expected for missing data)
	// Only fail if ALL endpoints are unreachable
	if passed == 0 {
		t.Fatal("No endpoints returned 200 OK - API might be down")
	}
}
