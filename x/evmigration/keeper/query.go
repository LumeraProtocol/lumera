package keeper

import (
	"context"

	"errors"

	"cosmossdk.io/collections"
	"cosmossdk.io/x/feegrant"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
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
		// (both are re-keyed during migration).
		var redCount uint64
		_ = qs.k.stakingKeeper.IterateRedelegations(ctx, func(_ int64, red stakingtypes.Redelegation) bool {
			if red.ValidatorSrcAddress == valAddr.String() || red.ValidatorDstAddress == valAddr.String() {
				redCount++
			}
			return false
		})
		resp.ValRedelegationCount = redCount

		// Check would_succeed.
		params, _ := qs.k.Params.Get(ctx)
		totalRecords := resp.ValDelegationCount + resp.ValUnbondingCount + resp.ValRedelegationCount
		if totalRecords > params.MaxValidatorDelegations {
			resp.WouldSucceed = false
			resp.RejectionReason = "too many delegators"
		} else if val.Status == stakingtypes.Unbonding || val.Status == stakingtypes.Unbonded {
			resp.WouldSucceed = false
			resp.RejectionReason = "validator is unbonding or unbonded"
		} else {
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

	resp.TotalTouched = resp.DelegationCount + resp.UnbondingCount + resp.RedelegationCount +
		resp.AuthzGrantCount + resp.FeegrantCount + resp.ActionCount +
		resp.ValDelegationCount + resp.ValUnbondingCount + resp.ValRedelegationCount

	// Check if already migrated.
	if has, _ := qs.k.MigrationRecords.Has(ctx, req.LegacyAddress); has {
		resp.WouldSucceed = false
		resp.RejectionReason = "already migrated"
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

	// Computed on-the-fly: count legacy accounts (secp256k1 pubkey).
	qs.k.accountKeeper.IterateAccounts(ctx, func(acc sdk.AccountI) bool {
		pk := acc.GetPubKey()
		if pk == nil {
			return false
		}
		if _, ok := pk.(*secp256k1.PubKey); ok {
			resp.TotalLegacy++
			// Check if has delegations.
			if dels, err := qs.k.stakingKeeper.GetDelegatorDelegations(ctx, acc.GetAddress(), 1); err == nil && len(dels) > 0 {
				resp.TotalLegacyStaked++
			}
		}
		return false
	})

	// Count legacy validators.
	qs.k.accountKeeper.IterateAccounts(ctx, func(acc sdk.AccountI) bool {
		pk := acc.GetPubKey()
		if pk == nil {
			return false
		}
		if _, ok := pk.(*secp256k1.PubKey); ok {
			valAddr := sdk.ValAddress(acc.GetAddress())
			if _, err := qs.k.stakingKeeper.GetValidator(ctx, valAddr); err == nil {
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
		pk := acc.GetPubKey()
		if pk == nil {
			return false
		}
		if _, ok := pk.(*secp256k1.PubKey); !ok {
			return false
		}

		addr := acc.GetAddress()
		info := types.LegacyAccountInfo{
			Address: addr.String(),
		}

		// Balance summary.
		balances := qs.k.bankKeeper.GetAllBalances(ctx, addr)
		if !balances.IsZero() {
			info.BalanceSummary = balances.String()
		}

		// Check delegations.
		if dels, err := qs.k.stakingKeeper.GetDelegatorDelegations(ctx, addr, 1); err == nil && len(dels) > 0 {
			info.HasDelegations = true
		}

		// Check if validator.
		valAddr := sdk.ValAddress(addr)
		if _, err := qs.k.stakingKeeper.GetValidator(ctx, valAddr); err == nil {
			info.IsValidator = true
		}

		accounts = append(accounts, info)
		return false
	})

	// Simple offset/limit pagination.
	start := 0
	if req.Pagination != nil && len(req.Pagination.Key) > 0 {
		// Key-based pagination not supported for this in-memory list.
		// Fall back to offset.
		start = int(req.Pagination.Offset)
	} else if req.Pagination != nil {
		start = int(req.Pagination.Offset)
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
