package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

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

// splitFundingTargets divides unfunded accounts into those funded by the normal
// batched bank-send (regular + multisig composites) and those funded by a
// vesting create tx (any account carrying VestingInfo).
func splitFundingTargets(reg *ActivityRegistry) (bank, vesting []*AccountRecord) {
	for _, rec := range reg.Accounts {
		if rec.Funded {
			continue
		}
		if rec.Vesting != nil {
			vesting = append(vesting, rec)
		} else {
			bank = append(bank, rec)
		}
	}
	return bank, vesting
}

// fundVestingAccounts creates each vesting/locked account on-chain via the
// appropriate create-* tx (funding the locked amount), then tops it up with a
// small liquid amount so it can pay gas. Marks Funded on success. Failures are
// logged and skipped (never fatal). Returns the count funded.
func fundVestingAccounts(cli *common.ChainCLI, funderKey, funderAddr string, targets []*AccountRecord, liquidTopUp string) int {
	funded := 0
	for _, rec := range targets {
		if rec.Vesting == nil {
			continue
		}
		if err := createVestingOnChain(cli, funderKey, rec); err != nil {
			log.Printf("  WARN: create vesting account %s: %v", rec.Name, err)
			continue
		}
		// Liquid top-up so the locked account can pay fees / make small sends.
		if _, err := cli.SubmitTx("tx", "bank", "send", funderAddr, rec.Address, liquidTopUp, "--from", funderKey); err != nil {
			log.Printf("  WARN: top up vesting account %s: %v", rec.Name, err)
			// Account exists and is locked; mark funded so it is recorded, but it
			// may be unable to pay gas. Still counts as created.
		}
		rec.Funded = true
		rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		funded++
		log.Printf("  funded %s vesting account %s (%s)", rec.Vesting.Type, rec.Name, rec.Address)
	}
	return funded
}

// createVestingOnChain dispatches the correct create-* tx for the account's
// vesting type.
func createVestingOnChain(cli *common.ChainCLI, funderKey string, rec *AccountRecord) error {
	switch rec.Vesting.Type {
	case string(common.VestingContinuous):
		_, err := cli.CreateVestingAccount(funderKey, rec.Address, rec.Vesting.LockedAmount, rec.Vesting.EndTime, false)
		return err
	case string(common.VestingDelayed):
		_, err := cli.CreateVestingAccount(funderKey, rec.Address, rec.Vesting.LockedAmount, rec.Vesting.EndTime, true)
		return err
	case string(common.VestingPermanentLocked):
		_, err := cli.CreatePermanentLockedAccount(funderKey, rec.Address, rec.Vesting.LockedAmount)
		return err
	default:
		return fmt.Errorf("unknown vesting type %q for %s", rec.Vesting.Type, rec.Name)
	}
}

// permanentLockedRecord builds a permanent-locked AccountRecord from a generated
// key, preserving the mnemonic and pubkey so migrate mode can later derive the
// EVM destination / re-import the key (regular accounts keep these too).
func permanentLockedRecord(gk common.GeneratedKey, keyStyle common.KeyStyle, lockedAmount, now string) *AccountRecord {
	return &AccountRecord{
		AccountIdentity: common.AccountIdentity{
			Name:      gk.Name,
			Address:   gk.Address,
			Mnemonic:  gk.Mnemonic,
			PubKeyB64: gk.PubKey,
			KeyStyle:  keyStyle.Name(),
		},
		Vesting:   newPermanentLockedInfo(lockedAmount),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// generatePermanentLockedAccounts creates `count` fresh keys named
// "<prefix>-plock-NNNN", tags each with a permanent-locked VestingInfo, upserts
// them into the registry, and returns the new records. Keys are created with the
// detected key style; the accounts are NOT yet on-chain (the funding phase
// creates them via create-permanent-locked-account). Rerun-safe.
func generatePermanentLockedAccounts(cli *common.ChainCLI, reg *ActivityRegistry, accountPrefix string, count int, keyStyle common.KeyStyle, lockedAmount, now string) []*AccountRecord {
	if count <= 0 {
		return nil
	}
	names := reg.AllocateNames(accountPrefix+"-plock", count)
	var recs []*AccountRecord
	for _, name := range names {
		var gk common.GeneratedKey
		if cli.HasKey(name) {
			// Reused key: address only; its mnemonic is no longer available.
			addr, err := cli.ShowAddress(name)
			if err != nil {
				continue
			}
			gk = common.GeneratedKey{Name: name, Address: addr}
		} else {
			created, err := cli.AddKeyWithStyle(name, keyStyle)
			if err != nil {
				continue
			}
			gk = created
		}
		rec := permanentLockedRecord(gk, keyStyle, lockedAmount, now)
		reg.UpsertAccount(rec)
		recs = append(recs, rec)
	}
	return recs
}
