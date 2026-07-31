package keeper

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"time"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/gogoproto/proto"
)

type retainedStatePlan struct {
	authz      []authzGrantMove
	votes      []govVoteMove
	deposits   []govDepositMove
	withdraw   []withdrawAddressMove
	legacyAddr sdk.AccAddress
	newAddr    sdk.AccAddress
}

type authzGrantMove struct {
	oldGranter, oldGrantee sdk.AccAddress
	newGranter, newGrantee sdk.AccAddress
	msgType                string
	source                 authz.Grant
	authorization          authz.Authorization
	expiration             *time.Time
}

type govVoteMove struct {
	proposalID  uint64
	source      govv1.Vote
	destination *govv1.Vote
	result      govv1.Vote
	collapse    bool
}

type govDepositMove struct {
	proposalID  uint64
	source      govv1.Deposit
	destination *govv1.Deposit
	result      govv1.Deposit
}

type withdrawAddressMove struct {
	key      []byte
	oldValue []byte
}

// buildRetainedStatePlan discovers every retained SDK reference before a
// production migration performs its first write. Missing dependencies are a
// configuration error: continuity must never silently degrade.
func (k Keeper) buildRetainedStatePlan(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress, cap uint64) (retainedStatePlan, error) {
	plan := retainedStatePlan{legacyAddr: bytes.Clone(legacyAddr), newAddr: bytes.Clone(newAddr)}
	if legacyAddr.Equals(newAddr) {
		return plan, fmt.Errorf("retained state source and destination must differ")
	}
	if cap == 0 {
		return plan, fmt.Errorf("retained state scan cap must be positive")
	}
	if k.authzKeeper == nil {
		return plan, fmt.Errorf("retained state authz keeper is not configured")
	}
	if k.govKeeper == nil {
		return plan, fmt.Errorf("retained state governance keeper is not configured")
	}
	if k.distributionStoreHandle == nil || k.distributionStoreHandle.svc == nil {
		return plan, fmt.Errorf("retained state distribution store is not configured")
	}

	var err error
	plan.authz, err = k.buildAuthzPlan(ctx, legacyAddr, newAddr, cap)
	if err != nil {
		return plan, fmt.Errorf("build retained authz plan: %w", err)
	}
	plan.votes, plan.deposits, err = k.buildGovernancePlan(ctx, legacyAddr, newAddr, cap)
	if err != nil {
		return plan, fmt.Errorf("build retained governance plan: %w", err)
	}
	plan.withdraw, err = k.buildWithdrawAddressPlan(ctx, legacyAddr, cap)
	if err != nil {
		return plan, fmt.Errorf("build retained distribution plan: %w", err)
	}
	return plan, nil
}

func (k Keeper) applyRetainedStatePlan(ctx sdk.Context, plan retainedStatePlan) error {
	// Verify every snapshot before the first retained-state write. This catches
	// stale plans deterministically and avoids partially applying direct calls.
	if err := k.verifyRetainedStatePlan(ctx, plan); err != nil {
		return err
	}
	for _, move := range plan.authz {
		if err := k.authzKeeper.DeleteGrant(ctx, move.oldGrantee, move.oldGranter, move.msgType); err != nil {
			return err
		}
		if err := k.authzKeeper.SaveGrant(ctx, move.newGrantee, move.newGranter, move.authorization, move.expiration); err != nil {
			return err
		}
	}
	for _, move := range plan.votes {
		if err := k.govKeeper.Votes.Remove(ctx, collections.Join(move.proposalID, plan.legacyAddr)); err != nil {
			return err
		}
		if !move.collapse {
			if err := k.govKeeper.Votes.Set(ctx, collections.Join(move.proposalID, plan.newAddr), move.result); err != nil {
				return err
			}
		}
	}
	for _, move := range plan.deposits {
		if err := k.govKeeper.Deposits.Remove(ctx, collections.Join(move.proposalID, plan.legacyAddr)); err != nil {
			return err
		}
		if err := k.govKeeper.Deposits.Set(ctx, collections.Join(move.proposalID, plan.newAddr), move.result); err != nil {
			return err
		}
	}
	if len(plan.withdraw) > 0 {
		store := k.distributionStoreHandle.svc.OpenKVStore(ctx)
		for _, move := range plan.withdraw {
			if err := store.Set(move.key, plan.newAddr.Bytes()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (k Keeper) verifyRetainedStatePlan(ctx sdk.Context, plan retainedStatePlan) error {
	if len(plan.authz) > 0 {
		if k.authzKeeper == nil {
			return fmt.Errorf("retained state authz keeper is not configured")
		}
		current := make(map[string]authz.Grant)
		k.authzKeeper.IterateGrants(ctx, func(granter, grantee sdk.AccAddress, grant authz.Grant) bool {
			a, err := grant.GetAuthorization()
			if err == nil {
				current[authzIdentity(granter, grantee, a.MsgTypeURL())] = grant
			}
			return false
		})
		for _, move := range plan.authz {
			sourceID := authzIdentity(move.oldGranter, move.oldGrantee, move.msgType)
			grant, ok := current[sourceID]
			if !ok {
				return fmt.Errorf("stale authz source grant %s", sourceID)
			}
			// Bytes comparison, not proto.Equal - see verifyCollectionValue.
			same, cmpErr := protoBytesEqual(&grant, &move.source)
			if cmpErr != nil {
				return cmpErr
			}
			if !same {
				return fmt.Errorf("stale authz source grant %s", sourceID)
			}
			targetID := authzIdentity(move.newGranter, move.newGrantee, move.msgType)
			if targetID != sourceID {
				if _, exists := current[targetID]; exists {
					return fmt.Errorf("stale authz destination grant %s", targetID)
				}
			}
		}
	}
	if len(plan.votes) > 0 || len(plan.deposits) > 0 {
		if k.govKeeper == nil {
			return fmt.Errorf("retained state governance keeper is not configured")
		}
	}
	for _, move := range plan.votes {
		if err := verifyCollectionValue(ctx, k.govKeeper.Votes, collections.Join(move.proposalID, plan.legacyAddr), &move.source); err != nil {
			return fmt.Errorf("stale governance vote source for proposal %d: %w", move.proposalID, err)
		}
		if err := verifyOptionalCollectionValue(ctx, k.govKeeper.Votes, collections.Join(move.proposalID, plan.newAddr), move.destination); err != nil {
			return fmt.Errorf("stale governance vote destination for proposal %d: %w", move.proposalID, err)
		}
	}
	for _, move := range plan.deposits {
		if err := verifyCollectionValue(ctx, k.govKeeper.Deposits, collections.Join(move.proposalID, plan.legacyAddr), &move.source); err != nil {
			return fmt.Errorf("stale governance deposit source for proposal %d: %w", move.proposalID, err)
		}
		if err := verifyOptionalCollectionValue(ctx, k.govKeeper.Deposits, collections.Join(move.proposalID, plan.newAddr), move.destination); err != nil {
			return fmt.Errorf("stale governance deposit destination for proposal %d: %w", move.proposalID, err)
		}
	}
	if len(plan.withdraw) > 0 {
		if k.distributionStoreHandle == nil || k.distributionStoreHandle.svc == nil {
			return fmt.Errorf("retained state distribution store is not configured")
		}
		store := k.distributionStoreHandle.svc.OpenKVStore(ctx)
		for _, move := range plan.withdraw {
			value, err := store.Get(move.key)
			if err != nil {
				return err
			}
			if !bytes.Equal(value, move.oldValue) {
				return fmt.Errorf("stale distribution withdraw address at %X", move.key)
			}
		}
	}
	return nil
}

// MigrateRetainedState is atomic for direct keeper callers.
func (k Keeper) MigrateRetainedState(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress, cap uint64) error {
	plan, err := k.buildRetainedStatePlan(ctx, legacyAddr, newAddr, cap)
	if err != nil {
		return err
	}
	cacheCtx, commit := ctx.CacheContext()
	if err := k.applyRetainedStatePlan(cacheCtx, plan); err != nil {
		return err
	}
	commit()
	return nil
}

func (k Keeper) buildAuthzPlan(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress, cap uint64) ([]authzGrantMove, error) {
	if k.authzKeeper == nil {
		return nil, fmt.Errorf("authz keeper is not configured")
	}
	type observed struct {
		granter, grantee sdk.AccAddress
		grant            authz.Grant
		authorization    authz.Authorization
		msgType          string
		embedded         bool
	}
	var all []observed
	var buildErr error
	var count uint64
	k.authzKeeper.IterateGrants(ctx, func(granter, grantee sdk.AccAddress, grant authz.Grant) bool {
		count++
		if count > cap {
			buildErr = fmt.Errorf("authz grant scan exceeds max %d", cap)
			return true
		}
		authorization, err := grant.GetAuthorization()
		if err != nil {
			buildErr = fmt.Errorf("decode grant %s/%s: %w", granter, grantee, err)
			return true
		}
		// Keeper implementations may return pointers backed by caches. Clone before
		// rewriting so planning remains read-only.
		cloned, ok := proto.Clone(authorization).(authz.Authorization)
		if !ok {
			buildErr = fmt.Errorf("clone unsupported authorization %T", authorization)
			return true
		}
		msgType := cloned.MsgTypeURL()
		embedded, err := rewriteStakeAuthorization(cloned, sdk.ValAddress(legacyAddr), sdk.ValAddress(newAddr))
		if err != nil {
			buildErr = fmt.Errorf("validate stake authorization %s/%s/%s: %w", granter, grantee, msgType, err)
			return true
		}
		grantCopy := proto.Clone(&grant).(*authz.Grant)
		all = append(all, observed{bytes.Clone(granter), bytes.Clone(grantee), *grantCopy, cloned, msgType, embedded})
		return false
	})
	if buildErr != nil {
		return nil, buildErr
	}

	existing := make(map[string]struct{}, len(all))
	for _, row := range all {
		existing[authzIdentity(row.granter, row.grantee, row.msgType)] = struct{}{}
	}
	targets := make(map[string]struct{})
	moves := make([]authzGrantMove, 0)
	for _, row := range all {
		if !row.granter.Equals(legacyAddr) && !row.grantee.Equals(legacyAddr) && !row.embedded {
			continue
		}
		newGranter, newGrantee := row.granter, row.grantee
		if newGranter.Equals(legacyAddr) {
			newGranter = newAddr
		}
		if newGrantee.Equals(legacyAddr) {
			newGrantee = newAddr
		}
		sourceID := authzIdentity(row.granter, row.grantee, row.msgType)
		targetID := authzIdentity(newGranter, newGrantee, row.msgType)
		if targetID != sourceID {
			if _, found := existing[targetID]; found {
				return nil, fmt.Errorf("authz destination grant already exists for %s", targetID)
			}
		}
		if _, duplicate := targets[targetID]; duplicate {
			return nil, fmt.Errorf("duplicate authz destination semantics for %s", targetID)
		}
		targets[targetID] = struct{}{}
		moves = append(moves, authzGrantMove{
			oldGranter: row.granter, oldGrantee: row.grantee,
			newGranter: bytes.Clone(newGranter), newGrantee: bytes.Clone(newGrantee),
			msgType: row.msgType, source: row.grant, authorization: row.authorization,
			expiration: cloneTime(row.grant.Expiration),
		})
	}
	return moves, nil
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	copy := *t
	return &copy
}

func authzIdentity(granter, grantee sdk.AccAddress, msgType string) string {
	return granter.String() + "\x00" + grantee.String() + "\x00" + msgType
}

func rewriteStakeAuthorization(authorization authz.Authorization, oldVal, newVal sdk.ValAddress) (bool, error) {
	stakeAuth, ok := authorization.(*stakingtypes.StakeAuthorization)
	if !ok {
		return false, nil
	}
	var addresses *[]string
	switch validators := stakeAuth.Validators.(type) {
	case nil:
		return false, nil
	case *stakingtypes.StakeAuthorization_AllowList:
		if validators.AllowList == nil {
			return false, fmt.Errorf("nil allow list")
		}
		addresses = &validators.AllowList.Address
	case *stakingtypes.StakeAuthorization_DenyList:
		if validators.DenyList == nil {
			return false, fmt.Errorf("nil deny list")
		}
		addresses = &validators.DenyList.Address
	default:
		return false, fmt.Errorf("unknown validator list type %T", validators)
	}
	seen := make(map[string]struct{}, len(*addresses))
	result := make([]string, 0, len(*addresses))
	foundOld := false
	for _, encoded := range *addresses {
		addr, err := sdk.ValAddressFromBech32(encoded)
		if err != nil || encoded != addr.String() {
			return false, fmt.Errorf("malformed validator address %q", encoded)
		}
		canonical := addr.String()
		if addr.Equals(oldVal) {
			foundOld = true
			canonical = newVal.String()
		}
		if _, duplicate := seen[canonical]; duplicate {
			// An old+new pair intentionally collapses to one destination entry.
			if foundOld && canonical == newVal.String() {
				continue
			}
			return false, fmt.Errorf("duplicate validator address %q", canonical)
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	if foundOld {
		*addresses = result
	}
	return foundOld, nil
}

func (k Keeper) buildGovernancePlan(ctx sdk.Context, legacyAddr, newAddr sdk.AccAddress, cap uint64) ([]govVoteMove, []govDepositMove, error) {
	if k.govKeeper == nil {
		return nil, nil, fmt.Errorf("governance keeper is not configured")
	}
	votes := make([]govVoteMove, 0)
	var scanned uint64
	err := k.govKeeper.Votes.Walk(ctx, nil, func(key collections.Pair[uint64, sdk.AccAddress], vote govv1.Vote) (bool, error) {
		scanned++
		if scanned > cap {
			return true, fmt.Errorf("governance row scan exceeds max %d", cap)
		}
		if !key.K2().Equals(legacyAddr) {
			return false, nil
		}
		if vote.Voter != legacyAddr.String() || vote.ProposalId != key.K1() {
			return true, fmt.Errorf("vote key and embedded value differ for proposal %d", key.K1())
		}
		result := *proto.Clone(&vote).(*govv1.Vote)
		result.Voter = newAddr.String()
		move := govVoteMove{proposalID: key.K1(), source: vote, result: result}
		destination, getErr := k.govKeeper.Votes.Get(ctx, collections.Join(key.K1(), newAddr))
		if getErr == nil {
			destCopy := *proto.Clone(&destination).(*govv1.Vote)
			move.destination = &destCopy
			// Bytes comparison, not proto.Equal - see verifyCollectionValue.
			sameVote, cmpErr := protoBytesEqual(&result, &destination)
			if cmpErr != nil {
				return true, cmpErr
			}
			if !sameVote {
				return true, fmt.Errorf("conflicting destination vote for proposal %d", key.K1())
			}
			move.collapse = true
		} else if !errors.Is(getErr, collections.ErrNotFound) {
			return true, getErr
		}
		votes = append(votes, move)
		return false, nil
	})
	if err != nil {
		return nil, nil, err
	}

	deposits := make([]govDepositMove, 0)
	err = k.govKeeper.Deposits.Walk(ctx, nil, func(key collections.Pair[uint64, sdk.AccAddress], deposit govv1.Deposit) (bool, error) {
		scanned++ // governance cap is total rows across votes and deposits
		if scanned > cap {
			return true, fmt.Errorf("governance row scan exceeds max %d", cap)
		}
		if !key.K2().Equals(legacyAddr) {
			return false, nil
		}
		if deposit.Depositor != legacyAddr.String() || deposit.ProposalId != key.K1() {
			return true, fmt.Errorf("deposit key and embedded value differ for proposal %d", key.K1())
		}
		if !sdk.Coins(deposit.Amount).IsValid() {
			return true, fmt.Errorf("source deposit has invalid coins for proposal %d", key.K1())
		}
		result := cloneGovDeposit(deposit)
		result.Depositor = newAddr.String()
		move := govDepositMove{proposalID: key.K1(), source: deposit, result: result}
		if destination, getErr := k.govKeeper.Deposits.Get(ctx, collections.Join(key.K1(), newAddr)); getErr == nil {
			if destination.Depositor != newAddr.String() || destination.ProposalId != key.K1() || !sdk.Coins(destination.Amount).IsValid() {
				return true, fmt.Errorf("destination deposit is malformed for proposal %d", key.K1())
			}
			destCopy := cloneGovDeposit(destination)
			move.destination = &destCopy
			// Valid sdk.Coins are sorted and unique, so Add preserves canonical order
			// without changing deposit semantics.
			move.result.Amount = sdk.Coins(destination.Amount).Add(deposit.Amount...)
		}
		deposits = append(deposits, move)
		return false, nil
	})
	return votes, deposits, err
}

func (k Keeper) buildWithdrawAddressPlan(ctx sdk.Context, legacyAddr sdk.AccAddress, cap uint64) ([]withdrawAddressMove, error) {
	if k.distributionStoreHandle == nil || k.distributionStoreHandle.svc == nil {
		return nil, fmt.Errorf("distribution store is not configured")
	}
	store := k.distributionStoreHandle.svc.OpenKVStore(ctx)
	prefix := distrtypes.DelegatorWithdrawAddrPrefix
	iterator, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil, err
	}
	defer func() { _ = iterator.Close() }()
	moves := make([]withdrawAddressMove, 0)
	var scanned uint64
	for ; iterator.Valid(); iterator.Next() {
		scanned++
		if scanned > cap {
			return nil, fmt.Errorf("distribution withdraw-address scan exceeds max %d", cap)
		}
		key := iterator.Key()
		// Exact SDK key: 0x03 | one-byte address length | delegator bytes.
		if len(key) < 3 || key[0] != distrtypes.DelegatorWithdrawAddrPrefix[0] || int(key[1]) != len(key)-2 {
			return nil, fmt.Errorf("malformed distribution withdraw-address key %X", key)
		}
		value := iterator.Value()
		if len(value) == 0 {
			return nil, fmt.Errorf("malformed distribution withdraw-address value at %X", key)
		}
		if bytes.Equal(value, legacyAddr.Bytes()) {
			moves = append(moves, withdrawAddressMove{key: bytes.Clone(key), oldValue: bytes.Clone(value)})
		}
	}
	return moves, nil
}

// These helpers keep stale checks exact while allowing the concrete collection
// value type to remain inferred from the SDK keeper fields.
// verifyCollectionValue re-reads a value and asserts it still matches the
// expectation captured when the plan was built.
//
// WHY NOT proto.Equal: gogoproto's reflection-based comparison is unreliable for
// messages carrying sdk.Coin, because Coin.Amount is an sdkmath.Int wrapping
// *big.Int with an unexported `abs []big.Word`. proto.Equal returns FALSE for two
// byte-identical gov Deposits (proved by TestProtoEqualOnIdenticalGovDeposits) --
// the same reflection family that outright panics in proto.Clone.
//
// Using it here produced a FALSE staleness positive: migrating a legacy account
// holding a governance deposit failed with
//
//	stale governance deposit source for proposal 2: value changed
//
// on a deposit nothing had touched. Fail-closed, so it was safe, but it blocked
// a legitimate migration and the message pointed at concurrent mutation that
// never happened.
//
// Marshalled-bytes comparison is the deterministic, consensus-relevant notion of
// "unchanged" -- it is exactly what the store holds -- and it sidesteps
// reflection entirely.
func verifyCollectionValue[K, V any](ctx sdk.Context, m collections.Map[K, V], key K, expected proto.Message) error {
	value, err := m.Get(ctx, key)
	if err != nil {
		return err
	}
	actual, ok := any(&value).(proto.Message)
	if !ok {
		return fmt.Errorf("value is not a proto.Message")
	}
	same, err := protoBytesEqual(actual, expected)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("value changed")
	}
	return nil
}

// protoBytesEqual compares two proto messages by their marshalled bytes.
// See verifyCollectionValue for why proto.Equal is not used.
func protoBytesEqual(a, b proto.Message) (bool, error) {
	ab, err := proto.Marshal(a)
	if err != nil {
		return false, fmt.Errorf("marshal actual: %w", err)
	}
	bb, err := proto.Marshal(b)
	if err != nil {
		return false, fmt.Errorf("marshal expected: %w", err)
	}
	return bytes.Equal(ab, bb), nil
}

// verifyOptionalCollectionValue asserts that an optional destination entry is
// still in the state the plan expected: absent if the plan saw it absent, or
// byte-identical if the plan captured a value.
//
// TYPED-NIL TRAP: callers pass a concrete typed pointer (e.g. *govv1.Deposit)
// for `expected`. When that pointer is nil it is wrapped in a NON-nil
// proto.Message interface, so a plain `expected == nil` check is FALSE and the
// "absent is fine" branch never runs. The function then treated a legitimately
// absent destination as an error and surfaced:
//
//	stale governance deposit destination for proposal 2:
//	  collections: not found: key '("2","lumera1k7del...")' of type ...gov.v1.Deposit
//
// i.e. it demanded the destination exist and then failed because it did not.
// isNilMessage uses reflection on the interface's underlying value to detect the
// typed-nil case correctly.
//
// Comparison is by marshalled bytes, not proto.Equal - see verifyCollectionValue
// for why gogoproto's reflective equality is unreliable for Coin-bearing values.
func verifyOptionalCollectionValue[K, V any](ctx sdk.Context, m collections.Map[K, V], key K, expected proto.Message) error {
	value, err := m.Get(ctx, key)
	if isNilMessage(expected) {
		if errors.Is(err, collections.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("destination appeared")
	}
	if err != nil {
		return err
	}
	actual, ok := any(&value).(proto.Message)
	if !ok {
		return fmt.Errorf("value is not a proto.Message")
	}
	same, cmpErr := protoBytesEqual(actual, expected)
	if cmpErr != nil {
		return cmpErr
	}
	if !same {
		return fmt.Errorf("value changed")
	}
	return nil
}

// isNilMessage reports whether a proto.Message interface is nil OR holds a nil
// typed pointer. A bare `m == nil` misses the second case, which is how an
// absent optional destination was misread as a hard error.
func isNilMessage(m proto.Message) bool {
	if m == nil {
		return true
	}
	v := reflect.ValueOf(m)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// cloneGovDeposit deep-copies a gov Deposit without going through proto.Clone.
//
// WHY NOT proto.Clone: govv1.Deposit.Amount is []sdk.Coin, whose Amount is an
// sdkmath.Int wrapping *big.Int. gogoproto's reflection-based table merge walks
// into big.Int's unexported `abs []big.Word` slice, finds no registered merger
// for big.Word, and PANICS:
//
//	panic: recovered: merger not found for type:big.Word
//	  gogoproto/proto.(*mergeInfo).computeMergeInfo
//	  x/gov/types/v1.(*Deposit).XXX_Merge
//	  gogoproto/proto.Clone
//	  keeper.buildGovernancePlan  (migrate_retained.go)
//
// Observed on a mainnet-shaped devnet: any legacy account holding an ACTIVE
// governance deposit failed to migrate with an opaque "merger not found" error.
// The tx aborts so no state is corrupted, but that account can never migrate
// while the deposit exists, and the operator-facing error explains nothing.
//
// Coins are value types holding an immutable Int, so an explicit element copy is
// a correct deep copy and avoids reflection entirely.
func cloneGovDeposit(src govv1.Deposit) govv1.Deposit {
	out := govv1.Deposit{
		ProposalId: src.ProposalId,
		Depositor:  src.Depositor,
	}
	if src.Amount != nil {
		out.Amount = make([]sdk.Coin, len(src.Amount))
		for i, c := range src.Amount {
			// Coin.Amount is an immutable sdkmath.Int; copying the struct is safe.
			out.Amount[i] = sdk.NewCoin(c.Denom, c.Amount)
		}
	}
	return out
}
