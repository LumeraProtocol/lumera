package main

import (
	"math/rand"

	"gen/tests/common"
)

// vestingWindow bounds the random vesting end time: now + [1h, 30 days].
const (
	vestingMinSeconds int64 = 3600
	vestingMaxSeconds int64 = 30 * 24 * 3600
)

// planVesting randomly designates floor(len*percent/100) of recs as vesting
// accounts, assigning each a continuous or delayed type and a random end time in
// the vesting window. rng and nowUnix are injected for deterministic tests.
// Returns the selected records (whose .Vesting was set in place).
func planVesting(recs []*AccountRecord, percent int, lockedAmount string, rng *rand.Rand, nowUnix int64) []*AccountRecord {
	if percent <= 0 || len(recs) == 0 {
		return nil
	}
	count := len(recs) * percent / 100
	if count == 0 {
		return nil
	}
	// Random subset via partial permutation of indices.
	idx := rng.Perm(len(recs))[:count]
	selected := make([]*AccountRecord, 0, count)
	for _, i := range idx {
		rec := recs[i]
		typ := common.VestingContinuous
		if rng.Intn(2) == 0 {
			typ = common.VestingDelayed
		}
		endTime := nowUnix + vestingMinSeconds + rng.Int63n(vestingMaxSeconds-vestingMinSeconds+1)
		rec.Vesting = &VestingInfo{
			Type:         string(typ),
			EndTime:      endTime,
			LockedAmount: lockedAmount,
		}
		selected = append(selected, rec)
	}
	return selected
}

// newPermanentLockedInfo builds the VestingInfo for a dedicated PermanentLocked
// account (no end time).
func newPermanentLockedInfo(lockedAmount string) *VestingInfo {
	return &VestingInfo{
		Type:         string(common.VestingPermanentLocked),
		LockedAmount: lockedAmount,
	}
}

// generatePermanentLockedAccounts creates `count` fresh keys named
// "<prefix>-plock-NNNN", tags each with a permanent-locked VestingInfo, upserts
// them into the registry, and returns the new records. Keys are created with the
// detected key style; the accounts are NOT yet on-chain (the funding phase
// creates them via create-permanent-locked-account). Rerun-safe.
//
//nolint:unused // wired into run() in a later task
func generatePermanentLockedAccounts(cli *common.ChainCLI, reg *ActivityRegistry, accountPrefix string, count int, keyStyle common.KeyStyle, lockedAmount, now string) []*AccountRecord {
	if count <= 0 {
		return nil
	}
	names := reg.AllocateNames(accountPrefix+"-plock", count)
	var recs []*AccountRecord
	for _, name := range names {
		var addr string
		if cli.HasKey(name) {
			a, err := cli.ShowAddress(name)
			if err != nil {
				continue
			}
			addr = a
		} else {
			gk, err := cli.AddKeyWithStyle(name, keyStyle)
			if err != nil {
				continue
			}
			addr = gk.Address
		}
		rec := &AccountRecord{
			AccountIdentity: common.AccountIdentity{Name: name, Address: addr, KeyStyle: keyStyle.Name()},
			Vesting:         newPermanentLockedInfo(lockedAmount),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		reg.UpsertAccount(rec)
		recs = append(recs, rec)
	}
	return recs
}
