package main

import (
	"log"
	"math/rand"
	"time"

	"gen/tests/common"
)

// cliFundingChain adapts a common.ChainCLI to the fundingChain interface the
// funder batcher depends on. It is a thin shell-out layer; the batching/retry
// logic it serves is unit-tested separately against a fake.
type cliFundingChain struct {
	cli        *common.ChainCLI
	funderKey  string
	funderAddr string
	blockWait  time.Duration
}

func (f *cliFundingChain) FunderAccountSequence() (uint64, uint64, error) {
	return f.cli.AccountNumberAndSequence(f.funderAddr)
}

func (f *cliFundingChain) SendFromFunder(accNum, seq uint64, to, amount string) error {
	_, err := f.cli.SendBankNoWait(f.funderKey, accNum, seq, to, amount)
	return err
}

func (f *cliFundingChain) WaitForNextBlock() error {
	return f.cli.WaitForNextBlock(f.blockWait)
}

func (f *cliFundingChain) IsFunded(addr string) (bool, error) {
	bal, err := f.cli.Balance(addr)
	if err != nil {
		return false, err
	}
	return bal > 0, nil
}

// randomFundingAmount picks a per-account funding amount in the safe range
// [maxAmount/2, maxAmount], so every account gets a usable balance without
// exceeding the configured cap. maxAmount <= 0 yields 0.
func randomFundingAmount(maxAmount int64, rng *rand.Rand) int64 {
	if maxAmount <= 0 {
		return 0
	}
	half := maxAmount / 2
	if half <= 0 {
		return maxAmount
	}
	return half + rng.Int63n(maxAmount-half+1)
}

// generateAccounts creates fresh keyring keys for the given names and returns
// new account records carrying their mnemonics. Existing keys (from an
// interrupted prior run) are adopted with a warning rather than failing.
func generateAccounts(cli *common.ChainCLI, names []string, keyStyle string) []*AccountRecord {
	now := time.Now().UTC().Format(time.RFC3339)
	recs := make([]*AccountRecord, 0, len(names))
	for _, name := range names {
		if cli.HasKey(name) {
			addr, err := cli.ShowAddress(name)
			if err != nil {
				log.Printf("  WARN: skip %s: key exists but address lookup failed: %v", name, err)
				continue
			}
			log.Printf("  WARN: key %s already exists; adopting (mnemonic unknown)", name)
			recs = append(recs, newAccountRecord(name, addr, "", "", keyStyle, now))
			continue
		}
		gk, err := cli.AddKey(name)
		if err != nil {
			log.Printf("  WARN: generate key %s failed: %v", name, err)
			continue
		}
		recs = append(recs, newAccountRecord(name, gk.Address, gk.Mnemonic, gk.PubKey, keyStyle, now))
	}
	return recs
}

func newAccountRecord(name, addr, mnemonic, pubkey, keyStyle, now string) *AccountRecord {
	return &AccountRecord{
		AccountIdentity: common.AccountIdentity{
			Name:      name,
			Address:   addr,
			Mnemonic:  mnemonic,
			PubKeyB64: pubkey,
			KeyStyle:  keyStyle,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// unfundedTargets returns the records that still need funding.
func unfundedTargets(reg *ActivityRegistry) []*AccountRecord {
	var out []*AccountRecord
	for _, rec := range reg.Accounts {
		if !rec.Funded {
			out = append(out, rec)
		}
	}
	return out
}
