package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gen/tests/common"
)

// multisigSpec describes one multisig account to generate.
type multisigSpec struct {
	Prefix    string // name infix, e.g. "msig23"
	Threshold int    // K
	Signers   int    // N
}

// multisigPlan expands the 2-of-3 and 3-of-5 counts into a flat list of specs.
func multisigPlan(num23, num35 int) []multisigSpec {
	var plan []multisigSpec
	for range num23 {
		plan = append(plan, multisigSpec{Prefix: "msig23", Threshold: 2, Signers: 3})
	}
	for range num35 {
		plan = append(plan, multisigSpec{Prefix: "msig35", Threshold: 3, Signers: 5})
	}
	return plan
}

// memberNames returns deterministic member key names for a composite.
func memberNames(composite string, signers int) []string {
	names := make([]string, signers)
	for i := range signers {
		names[i] = fmt.Sprintf("%s-signer-%d", composite, i+1)
	}
	return names
}

// generateMultisigAccounts creates member keys + composite keys for the planned
// multisig accounts, returning new AccountRecords (with Multisig info set). It is
// rerun-safe: existing keys/composites are reused. Member keys use the detected
// key style; the composite is key-type agnostic.
func generateMultisigAccounts(cli *common.ChainCLI, reg *ActivityRegistry, accountPrefix string, specs []multisigSpec, keyStyle common.KeyStyle) []*AccountRecord {
	ms := common.NewMultisig(cli)
	now := time.Now().UTC().Format(time.RFC3339)
	var recs []*AccountRecord

	for _, spec := range specs {
		names := reg.AllocateNames(accountPrefix+"-"+spec.Prefix, 1)
		composite := names[0]
		members := memberNames(composite, spec.Signers)

		memberKeys, err := ensureMembers(cli, members, keyStyle)
		if err != nil {
			log.Printf("  WARN: multisig %s members: %v", composite, err)
			continue
		}
		addr, err := ms.CreateMultisigKey(composite, members, spec.Threshold)
		if err != nil {
			log.Printf("  WARN: create multisig %s: %v", composite, err)
			continue
		}
		rec := &AccountRecord{
			AccountIdentity: common.AccountIdentity{Name: composite, Address: addr, KeyStyle: keyStyle.Name()},
			Multisig:        &MultisigInfo{MemberNames: members, Members: memberKeys, Threshold: spec.Threshold, Signers: spec.Signers},
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		reg.UpsertAccount(rec)
		recs = append(recs, rec)
		log.Printf("  created %d-of-%d multisig %s (%s)", spec.Threshold, spec.Signers, composite, addr)
	}
	return recs
}

// ensureMembers creates any missing member keys with the detected key style and
// returns the member key material. Newly created keys carry their mnemonic (so
// migrate mode can re-import them into a fresh keyring); reused keys carry only
// their name+address because the seed is no longer available.
func ensureMembers(cli *common.ChainCLI, names []string, keyStyle common.KeyStyle) ([]MultisigMember, error) {
	members := make([]MultisigMember, 0, len(names))
	for _, name := range names {
		if cli.HasKey(name) {
			addr, err := cli.ShowAddress(name)
			if err != nil {
				return nil, fmt.Errorf("resolve existing member key %s: %w", name, err)
			}
			members = append(members, MultisigMember{Name: name, Address: addr})
			continue
		}
		gk, err := cli.AddKeyWithStyle(name, keyStyle)
		if err != nil {
			return nil, fmt.Errorf("add member key %s: %w", name, err)
		}
		members = append(members, MultisigMember{Name: name, Address: gk.Address, Mnemonic: gk.Mnemonic})
	}
	return members, nil
}

// multisigExerciser performs one multisign tx for a multisig account. It is an
// interface so the recording logic is testable without a live chain.
type multisigExerciser interface {
	// MultisigBankSend builds, signs (K members), and broadcasts a bank send
	// from the multisig account to `to`, returning the tx hash.
	MultisigBankSend(rec *AccountRecord, to, amount string) (string, error)
}

// exerciseMultisig runs one multisign bank-send for the account and records it.
func exerciseMultisig(ex multisigExerciser, rec *AccountRecord, peer, amount string) error {
	if rec.Multisig == nil {
		return fmt.Errorf("account %s is not a multisig account", rec.Name)
	}
	txHash, err := ex.MultisigBankSend(rec, peer, amount)
	if err != nil {
		return err
	}
	rec.AddBankSend(common.BankSendActivity{To: peer, Amount: amount, TxHash: txHash})
	return nil
}

// cliMultisigExerciser is the production exerciser backed by common.Multisig.
type cliMultisigExerciser struct {
	cli *common.ChainCLI
	ms  *common.Multisig
}

func newCLIMultisigExerciser(cli *common.ChainCLI) *cliMultisigExerciser {
	return &cliMultisigExerciser{cli: cli, ms: common.NewMultisig(cli)}
}

func (e *cliMultisigExerciser) MultisigBankSend(rec *AccountRecord, to, amount string) (string, error) {
	accNum, seq, err := e.cli.AccountNumberAndSequence(rec.Address)
	if err != nil {
		return "", fmt.Errorf("query account number/sequence for %s: %w", rec.Address, err)
	}
	unsigned, err := os.CreateTemp("", "msig-unsigned-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp unsigned file: %w", err)
	}
	_ = unsigned.Close()
	defer func() { _ = os.Remove(unsigned.Name()) }()

	args := e.ms.GenBankSendArgs(rec.Name, rec.Address, to, amount, accNum, seq)
	if err := e.ms.BuildUnsignedToFile(unsigned.Name(), args); err != nil {
		return "", err
	}
	return e.ms.SignAndBroadcastFile(unsigned.Name(), rec.Name, rec.Address,
		rec.Multisig.MemberNames, rec.Multisig.Threshold, accNum, seq)
}

// exerciseMultisigAccounts registers each funded multisig composite's pubkey
// on-chain (via the shared ceremony's self-send) and runs one multisign
// bank-send to a regular peer address. Failures are logged, never fatal.
func exerciseMultisigAccounts(cli *common.ChainCLI, reg *ActivityRegistry, newMultisig []*AccountRecord, cfg *Config) {
	targets := newMultisig
	if cfg.ActivityExisting {
		targets = nil
		for _, rec := range reg.Accounts {
			if rec.Multisig != nil && rec.Funded {
				targets = append(targets, rec)
			}
		}
	}
	if len(targets) == 0 {
		return
	}

	peer := firstRegularPeer(reg)
	if peer == "" {
		log.Printf("  no regular peer account to receive multisig sends; skipping multisig exercise")
		return
	}

	ms := common.NewMultisig(cli)
	ex := newCLIMultisigExerciser(cli)
	amount := common.Coin{Amount: 1, Denom: common.ChainDenom}.String()

	for _, rec := range targets {
		if !rec.Funded {
			continue
		}
		if err := registerMultisigPubkey(cli, ms, rec); err != nil {
			log.Printf("  WARN: register pubkey for %s: %v", rec.Name, err)
			continue
		}
		if err := exerciseMultisig(ex, rec, peer, amount); err != nil {
			log.Printf("  WARN: exercise multisig %s: %v", rec.Name, err)
		}
	}
}

// registerMultisigPubkey publishes a multisig composite's pubkey on-chain via a
// 1-ulume self-send (required before it can be queried for account number).
func registerMultisigPubkey(cli *common.ChainCLI, ms *common.Multisig, rec *AccountRecord) error {
	accNum, seq, err := cli.AccountNumberAndSequence(rec.Address)
	if err != nil {
		return fmt.Errorf("query account number/sequence for %s: %w", rec.Address, err)
	}
	unsigned, err := os.CreateTemp("", "msig-selfsend-*.json")
	if err != nil {
		return fmt.Errorf("create temp unsigned file: %w", err)
	}
	_ = unsigned.Close()
	defer func() { _ = os.Remove(unsigned.Name()) }()

	args := ms.GenBankSendArgs(rec.Name, rec.Address, rec.Address, "1"+common.ChainDenom, accNum, seq)
	if err := ms.BuildUnsignedToFile(unsigned.Name(), args); err != nil {
		return err
	}
	_, err = ms.SignAndBroadcastFile(unsigned.Name(), rec.Name, rec.Address,
		rec.Multisig.MemberNames, rec.Multisig.Threshold, accNum, seq)
	return err
}

// firstRegularPeer returns the address of the first funded non-multisig account
// to serve as a recipient for multisig sends.
func firstRegularPeer(reg *ActivityRegistry) string {
	for _, rec := range reg.Accounts {
		if rec.Multisig == nil && rec.Funded && rec.Address != "" {
			return rec.Address
		}
	}
	return ""
}
