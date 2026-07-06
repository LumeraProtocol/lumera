package keeper

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cosmossdk.io/collections"
	"cosmossdk.io/x/feegrant"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

var _ types.QueryServer = queryServer{}

// NewQueryServerImpl returns an implementation of the QueryServer interface
// for the provided Keeper.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k: k}
}

type queryServer struct {
	types.UnimplementedQueryServer
	k Keeper
}

type legacyAccountStatus struct {
	balances       sdk.Coins
	hasDelegations bool
	isValidator    bool
}

// isLegacyPubKey reports whether pk is a key type migratable by the
// evmigration module: either a plain secp256k1.PubKey or a flat multisig
// whose sub-keys are all secp256k1. Nested multisig and non-secp256k1
// sub-keys are rejected.
func isLegacyPubKey(pk cryptotypes.PubKey) bool {
	switch key := pk.(type) {
	case *secp256k1.PubKey:
		return true
	case *kmultisig.LegacyAminoPubKey:
		for _, sub := range key.GetPubKeys() {
			if _, ok := sub.(*secp256k1.PubKey); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (qs queryServer) remainingLegacyAccountStatus(ctx sdk.Context, acc sdk.AccountI) (legacyAccountStatus, bool) {
	status := legacyAccountStatus{}

	// Skip module accounts — they cannot be migrated.
	if _, ok := acc.(sdk.ModuleAccountI); ok {
		return status, false
	}

	pk := acc.GetPubKey()
	if pk != nil && !isLegacyPubKey(pk) {
		// Non-legacy key type (eth_secp256k1, ed25519, non-secp256k1 multisig, etc.).
		return status, false
	}
	// pk == nil OR pk is a legacy-compatible key (secp256k1 or flat multisig of secp256k1).
	// Nil-pubkey accounts are funded-but-never-signed — still legacy-migratable for single-key.

	addr := acc.GetAddress()
	addrStr := addr.String()

	if hasMigrated, err := qs.k.MigrationRecords.Has(ctx, addrStr); err == nil && hasMigrated {
		return status, false
	}

	// Exclude migration destination accounts (new EVM addresses created by migration).
	if isNewAddr, err := qs.k.MigrationRecordByNewAddress.Has(ctx, addrStr); err == nil && isNewAddr {
		return status, false
	}

	status.balances = qs.k.bankKeeper.GetAllBalances(ctx, addr)
	if dels, err := qs.k.stakingKeeper.GetDelegatorDelegations(ctx, addr, 1); err == nil && len(dels) > 0 {
		status.hasDelegations = true
	}
	if _, err := qs.k.stakingKeeper.GetValidator(ctx, sdk.ValAddress(addr)); err == nil {
		status.isValidator = true
	}

	// "Remaining legacy" means the account still has state that actually needs migration.
	if status.balances.IsZero() && !status.hasDelegations && !status.isValidator {
		return status, false
	}

	return status, true
}

func (qs queryServer) MigrationRecord(ctx context.Context, req *types.QueryMigrationRecordRequest) (*types.QueryMigrationRecordResponse, error) {
	record, err := qs.k.MigrationRecords.Get(ctx, req.LegacyAddress)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return &types.QueryMigrationRecordResponse{}, nil
		}
		return nil, err
	}
	return &types.QueryMigrationRecordResponse{Record: &record}, nil
}

func (qs queryServer) MigrationRecordByNewAddress(ctx context.Context, req *types.QueryMigrationRecordByNewAddressRequest) (*types.QueryMigrationRecordByNewAddressResponse, error) {
	legacyAddress, err := qs.k.MigrationRecordByNewAddress.Get(ctx, req.NewAddress)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return &types.QueryMigrationRecordByNewAddressResponse{}, nil
		}
		return nil, err
	}

	record, err := qs.k.MigrationRecords.Get(ctx, legacyAddress)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return &types.QueryMigrationRecordByNewAddressResponse{}, nil
		}
		return nil, err
	}

	return &types.QueryMigrationRecordByNewAddressResponse{Record: &record}, nil
}

func (qs queryServer) MigrationRecords(ctx context.Context, req *types.QueryMigrationRecordsRequest) (*types.QueryMigrationRecordsResponse, error) {
	records, pageResp, err := query.CollectionPaginate(
		ctx,
		qs.k.MigrationRecords,
		req.Pagination,
		func(_ string, record types.MigrationRecord) (types.MigrationRecord, error) {
			return record, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &types.QueryMigrationRecordsResponse{
		Records:    records,
		Pagination: pageResp,
	}, nil
}

func (qs queryServer) MigrationEstimate(goCtx context.Context, req *types.QueryMigrationEstimateRequest) (*types.QueryMigrationEstimateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	addr, err := sdk.AccAddressFromBech32(req.LegacyAddress)
	if err != nil {
		return nil, err
	}

	resp := &types.QueryMigrationEstimateResponse{}

	// Fetch params once for use in validator and multisig preflight checks.
	params, err := qs.k.Params.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("load params for migration estimate: %w", err)
	}

	// Check if validator.
	valAddr := sdk.ValAddress(addr)
	val, valErr := qs.k.stakingKeeper.GetValidator(ctx, valAddr)
	if valErr == nil {
		resp.IsValidator = true
		// Count delegations TO this validator.
		if dels, err := qs.k.stakingKeeper.GetValidatorDelegations(ctx, valAddr); err == nil {
			resp.ValDelegationCount = uint64(len(dels))
		}
		if ubds, err := qs.k.stakingKeeper.GetUnbondingDelegationsFromValidator(ctx, valAddr); err == nil {
			resp.ValUnbondingCount = uint64(len(ubds))
		}
		// Count redelegations where the validator is source OR destination
		// (both are re-keyed during migration). Unlike the delegation/unbonding
		// keeper reads above, the scoped redelegation lookup can fail on a
		// corrupt src/dst index (an index row pointing at a missing record). We
		// must NOT swallow that: undercounting redelegations would shrink
		// totalRecords and could flip WouldSucceed to true for a validator whose
		// real footprint exceeds MaxValidatorDelegations. Fail the estimate loudly.
		reds, err := qs.k.redelegationsForValidator(ctx, valAddr)
		if err != nil {
			return nil, fmt.Errorf("count redelegations for validator %s: %w", valAddr, err)
		}
		resp.ValRedelegationCount = uint64(len(reds))

		// Surface the validator's BondStatus + Jailed flag so callers can
		// display the actionable cause when WouldSucceed is false. Jailed
		// implies Status ∈ {Unbonding, Unbonded} but the reverse isn't
		// true (a validator can voluntarily unbond without being jailed),
		// hence both fields are returned.
		resp.ValidatorStatus = val.Status.String()
		resp.ValidatorJailed = val.Jailed

		// Check would_succeed. The rejection_reason is callable-readable
		// English; consumers should rely on validator_jailed /
		// validator_status for programmatic dispatch.
		totalRecords := resp.ValDelegationCount + resp.ValUnbondingCount + resp.ValRedelegationCount
		switch {
		case totalRecords > params.MaxValidatorDelegations:
			resp.WouldSucceed = false
			resp.RejectionReason = "too many delegators"
		case val.Jailed:
			resp.WouldSucceed = false
			resp.RejectionReason = fmt.Sprintf(
				"validator is jailed (status: %s); restart the node, wait for catch-up, then `lumerad tx slashing unjail`",
				strings.ToLower(strings.TrimPrefix(val.Status.String(), "BOND_STATUS_")),
			)
		case val.Status == stakingtypes.Unbonding:
			// Reject only Unbonding — an Unbonding validator still holds a live
			// unbonding-validator-queue entry keyed by the old operator address;
			// migrating would orphan it and halt the chain at maturity. Once the
			// unbonding period elapses the validator becomes Unbonded (queue entry
			// dequeued) and IS migratable, so the guidance is to wait, not re-bond.
			// Unbonded (not jailed) falls through to the default → would_succeed=true.
			resp.WouldSucceed = false
			resp.RejectionReason = "validator is unbonding; wait for the unbonding period to complete, then migrate"
		default:
			resp.WouldSucceed = true
		}
	} else {
		resp.WouldSucceed = true
	}

	// Count delegations FROM this address.
	if dels, err := qs.k.stakingKeeper.GetDelegatorDelegations(ctx, addr, ^uint16(0)); err == nil {
		resp.DelegationCount = uint64(len(dels))
	}
	if ubds, err := qs.k.stakingKeeper.GetUnbondingDelegations(ctx, addr, ^uint16(0)); err == nil {
		resp.UnbondingCount = uint64(len(ubds))
	}
	if reds, err := qs.k.stakingKeeper.GetRedelegations(ctx, addr, ^uint16(0)); err == nil {
		resp.RedelegationCount = uint64(len(reds))
	}

	// Count authz grants.
	qs.k.authzKeeper.IterateGrants(ctx, func(granter, grantee sdk.AccAddress, _ authz.Grant) bool {
		if granter.Equals(addr) || grantee.Equals(addr) {
			resp.AuthzGrantCount++
		}
		return false
	})

	// Count feegrant allowances.
	_ = qs.k.feegrantKeeper.IterateAllFeeAllowances(ctx, func(grant feegrant.Grant) bool {
		granterAddr, err := sdk.AccAddressFromBech32(grant.Granter)
		if err != nil {
			return false
		}
		granteeAddr, err := sdk.AccAddressFromBech32(grant.Grantee)
		if err != nil {
			return false
		}
		if granterAddr.Equals(addr) || granteeAddr.Equals(addr) {
			resp.FeegrantCount++
		}
		return false
	})

	// Count action records touched by this account migration. A single action is
	// counted once even if the same address appears as both creator and supernode.
	_ = qs.k.actionKeeper.IterateActions(ctx, func(action *actiontypes.Action) bool {
		if action.Creator == req.LegacyAddress {
			resp.ActionCount++
			return false
		}
		for _, sn := range action.SuperNodes {
			if sn == req.LegacyAddress {
				resp.ActionCount++
				break
			}
		}
		return false
	})

	// Balance summary.
	balances := qs.k.bankKeeper.GetAllBalances(ctx, addr)
	if !balances.IsZero() {
		resp.BalanceSummary = balances.String()
	}

	// Check supernode registration.
	if _, found := qs.k.supernodeKeeper.QuerySuperNode(ctx, sdk.ValAddress(addr)); found {
		resp.HasSupernode = true
	}

	resp.TotalTouched = resp.DelegationCount + resp.UnbondingCount + resp.RedelegationCount +
		resp.AuthzGrantCount + resp.FeegrantCount + resp.ActionCount +
		resp.ValDelegationCount + resp.ValUnbondingCount + resp.ValRedelegationCount

	// Check if already migrated.
	if has, _ := qs.k.MigrationRecords.Has(ctx, req.LegacyAddress); has {
		resp.WouldSucceed = false
		resp.RejectionReason = "already migrated"
	}

	// Multisig feasibility preflight.
	if acc := qs.k.accountKeeper.GetAccount(ctx, addr); acc != nil {
		if pk := acc.GetPubKey(); pk != nil {
			if ms, ok := pk.(*kmultisig.LegacyAminoPubKey); ok {
				resp.IsMultisig = true
				resp.Threshold = uint32(ms.Threshold)
				resp.NumSigners = uint32(len(ms.GetPubKeys()))

				// Reject nested / non-secp256k1 sub-keys, and duplicate sub-keys.
				// SDK multisig construction itself permits duplicates, but the
				// migration verifier (MultisigProof.validateBasic) rejects them
				// at consensus — without this preflight, an existing duplicate-
				// sub-key legacy multisig would report would_succeed=true and
				// only fail after the K-of-N signing ceremony.
				seen := make(map[string]int, len(ms.GetPubKeys()))
				for i, sub := range ms.GetPubKeys() {
					if _, ok := sub.(*secp256k1.PubKey); !ok {
						resp.WouldSucceed = false
						resp.RejectionReason = "multisig contains non-secp256k1 sub-key (unsupported)"
						break
					}
					key := string(sub.Bytes())
					if prior, dup := seen[key]; dup {
						resp.WouldSucceed = false
						resp.RejectionReason = fmt.Sprintf(
							"multisig sub_pub_keys[%d] duplicates sub_pub_keys[%d] (would fail ValidateBasic)", i, prior)
						break
					}
					seen[key] = i
				}
				// Size cap against MaxMultisigSubKeys.
				if resp.WouldSucceed && resp.NumSigners > params.MaxMultisigSubKeys {
					resp.WouldSucceed = false
					resp.RejectionReason = fmt.Sprintf("multisig has %d sub-keys; max is %d",
						resp.NumSigners, params.MaxMultisigSubKeys)
				}
			}
		}
		// Nil pubkey: cannot distinguish single-key vs multisig from the account
		// alone. Preflight does not flag this case; detection is deferred to
		// the CLI's generate-proof-payload command (see design spec).
	}

	return resp, nil
}

func (qs queryServer) MigrationStats(goCtx context.Context, _ *types.QueryMigrationStatsRequest) (*types.QueryMigrationStatsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	resp := &types.QueryMigrationStatsResponse{}

	// O(1) counters from state.
	if count, err := qs.k.MigrationCounter.Get(ctx); err == nil {
		resp.TotalMigrated = count
	}
	if count, err := qs.k.ValidatorMigrationCounter.Get(ctx); err == nil {
		resp.TotalValidatorsMigrated = count
	}

	// Computed on-the-fly: count remaining legacy accounts that still need migration.
	qs.k.accountKeeper.IterateAccounts(ctx, func(acc sdk.AccountI) bool {
		status, shouldCount := qs.remainingLegacyAccountStatus(ctx, acc)
		if shouldCount {
			resp.TotalLegacy++
			if acc.GetPubKey() == nil {
				resp.TotalLegacyWithoutPubkey++
			} else {
				resp.TotalLegacyWithPubkey++
			}
			if status.hasDelegations {
				resp.TotalLegacyStaked++
			}
			if status.isValidator {
				resp.TotalValidatorsLegacy++
			}
		}
		return false
	})

	return resp, nil
}

func (qs queryServer) LegacyAccounts(goCtx context.Context, req *types.QueryLegacyAccountsRequest) (*types.QueryLegacyAccountsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Collect all legacy accounts, then paginate in memory.
	// This is a node-side query, not consensus-critical.
	var accounts []types.LegacyAccountInfo
	qs.k.accountKeeper.IterateAccounts(ctx, func(acc sdk.AccountI) bool {
		status, shouldInclude := qs.remainingLegacyAccountStatus(ctx, acc)
		if !shouldInclude {
			return false
		}

		addr := acc.GetAddress()
		info := types.LegacyAccountInfo{
			Address: addr.String(),
		}

		// Balance summary.
		if !status.balances.IsZero() {
			info.BalanceSummary = status.balances.String()
		}
		info.HasDelegations = status.hasDelegations
		info.IsValidator = status.isValidator

		// Populate multisig metadata when the on-chain pubkey is multisig.
		if pk := acc.GetPubKey(); pk != nil {
			if ms, ok := pk.(*kmultisig.LegacyAminoPubKey); ok {
				info.IsMultisig = true
				info.Threshold = uint32(ms.Threshold)
				info.NumSigners = uint32(len(ms.GetPubKeys()))
			}
		}

		accounts = append(accounts, info)
		return false
	})

	// Simple offset/limit pagination. The response emits NextKey as a
	// big-endian-encoded offset, so when a client passes Pagination.Key from a
	// prior response we decode it back to an offset. Explicit Pagination.Offset
	// still works for clients that prefer it; Key wins when both are set
	// (matches Cosmos SDK idiom of "use Key when present, otherwise Offset").
	start := 0
	if req.Pagination != nil {
		switch {
		case len(req.Pagination.Key) == 8:
			start = int(sdk.BigEndianToUint64(req.Pagination.Key))
		case len(req.Pagination.Key) > 0:
			return nil, fmt.Errorf("invalid pagination key length: got %d bytes, want 8", len(req.Pagination.Key))
		default:
			start = int(req.Pagination.Offset)
		}
	}

	limit := 100
	if req.Pagination != nil && req.Pagination.Limit > 0 {
		limit = int(req.Pagination.Limit)
	}

	end := start + limit
	if end > len(accounts) {
		end = len(accounts)
	}
	if start > len(accounts) {
		start = len(accounts)
	}

	total := uint64(len(accounts))
	var nextKey []byte
	if uint64(end) < total {
		// Use the next offset as the key for simplicity (in-memory pagination).
		nextKey = sdk.Uint64ToBigEndian(uint64(end))
	}

	return &types.QueryLegacyAccountsResponse{
		Accounts: accounts[start:end],
		Pagination: &query.PageResponse{
			Total:   total,
			NextKey: nextKey,
		},
	}, nil
}

func (qs queryServer) MigratedAccounts(ctx context.Context, req *types.QueryMigratedAccountsRequest) (*types.QueryMigratedAccountsResponse, error) {
	records, pageResp, err := query.CollectionPaginate(
		ctx,
		qs.k.MigrationRecords,
		req.Pagination,
		func(_ string, record types.MigrationRecord) (types.MigrationRecord, error) {
			return record, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &types.QueryMigratedAccountsResponse{
		Records:    records,
		Pagination: pageResp,
	}, nil
}
