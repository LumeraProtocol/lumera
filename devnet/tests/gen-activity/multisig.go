package main

import (
	"fmt"
	"log"
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
//
//nolint:unused // wired into run() in a later task
func generateMultisigAccounts(cli *common.ChainCLI, reg *ActivityRegistry, accountPrefix string, specs []multisigSpec, keyStyle common.KeyStyle) []*AccountRecord {
	ms := common.NewMultisig(cli)
	now := time.Now().UTC().Format(time.RFC3339)
	var recs []*AccountRecord

	for _, spec := range specs {
		names := reg.AllocateNames(accountPrefix+"-"+spec.Prefix, 1)
		composite := names[0]
		members := memberNames(composite, spec.Signers)

		if err := ensureMembers(cli, members, keyStyle); err != nil {
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
			Multisig:        &MultisigInfo{MemberNames: members, Threshold: spec.Threshold, Signers: spec.Signers},
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		reg.UpsertAccount(rec)
		recs = append(recs, rec)
		log.Printf("  created %d-of-%d multisig %s (%s)", spec.Threshold, spec.Signers, composite, addr)
	}
	return recs
}

// ensureMembers creates any missing member keys with the detected key style.
//
//nolint:unused // wired into run() in a later task
func ensureMembers(cli *common.ChainCLI, names []string, keyStyle common.KeyStyle) error {
	for _, name := range names {
		if cli.HasKey(name) {
			continue
		}
		if _, err := cli.AddKeyWithStyle(name, keyStyle); err != nil {
			return fmt.Errorf("add member key %s: %w", name, err)
		}
	}
	return nil
}
