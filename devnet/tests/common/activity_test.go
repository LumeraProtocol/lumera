package common

import "testing"

func TestActivityLogDelegationDedup(t *testing.T) {
	var log ActivityLog
	log.AddDelegation("valA", "100ulume")
	log.AddDelegation("valA", "") // duplicate validator, must not append
	log.AddDelegation("valB", "50ulume")

	if got := len(log.Delegations); got != 2 {
		t.Fatalf("delegations = %d, want 2", got)
	}
	// Empty amount on a duplicate must not clobber an existing amount.
	if log.Delegations[0].Amount != "100ulume" {
		t.Errorf("delegation amount = %q, want 100ulume", log.Delegations[0].Amount)
	}
	// An empty validator is ignored.
	log.AddDelegation("", "1ulume")
	if got := len(log.Delegations); got != 2 {
		t.Errorf("delegations after empty-validator add = %d, want 2", got)
	}
}

func TestActivityLogRedelegationDedup(t *testing.T) {
	var log ActivityLog
	log.AddRedelegation("valA", "valB", "10ulume")
	log.AddRedelegation("valA", "valB", "")       // duplicate pair
	log.AddRedelegation("valA", "valA", "5ulume") // src==dst rejected
	log.AddRedelegation("valB", "valC", "7ulume")

	if got := len(log.Redelegations); got != 2 {
		t.Fatalf("redelegations = %d, want 2", got)
	}
}

func TestActivityLogAuthzAndFeegrantDedup(t *testing.T) {
	var log ActivityLog
	log.AddAuthzGrant("grantee1", BankSendMsgType)
	log.AddAuthzGrant("grantee1", BankSendMsgType) // dup
	log.AddFeegrant("grantee1", "5ulume")
	log.AddFeegrant("grantee1", "") // dup

	if len(log.AuthzGrants) != 1 {
		t.Errorf("authz grants = %d, want 1", len(log.AuthzGrants))
	}
	if len(log.Feegrants) != 1 {
		t.Errorf("feegrants = %d, want 1", len(log.Feegrants))
	}
}

func TestActivityLogActionDedup(t *testing.T) {
	var log ActivityLog
	log.AddAction(ActionActivity{ActionID: "1", State: "pending"})
	log.AddAction(ActionActivity{ActionID: "1", State: "done"}) // dup ID, ignored
	log.AddAction(ActionActivity{ActionID: "2", State: "pending"})
	log.AddAction(ActionActivity{ActionID: "", State: "pending"}) // empty ID ignored

	if got := len(log.Actions); got != 2 {
		t.Fatalf("actions = %d, want 2", got)
	}

	// updateActionState mutates an existing record in place.
	if !log.UpdateActionState("1", "approved") {
		t.Error("UpdateActionState returned false for existing action")
	}
	if log.Actions[0].State != "approved" {
		t.Errorf("action 1 state = %q, want approved", log.Actions[0].State)
	}
	if log.UpdateActionState("missing", "done") {
		t.Error("UpdateActionState returned true for missing action")
	}
}

func TestActivityLogBankSendsAreEvents(t *testing.T) {
	var log ActivityLog
	// Bank sends are events, not state: repeated sends to the same recipient
	// each produce a distinct record.
	log.AddBankSend(BankSendActivity{To: "addrB", Amount: "1ulume", TxHash: "AAA"})
	log.AddBankSend(BankSendActivity{To: "addrB", Amount: "1ulume", TxHash: "BBB"})
	if got := len(log.BankSends); got != 2 {
		t.Fatalf("bank sends = %d, want 2", got)
	}

	// But the same tx hash must not be recorded twice (retry safety).
	log.AddBankSend(BankSendActivity{To: "addrB", Amount: "1ulume", TxHash: "AAA"})
	if got := len(log.BankSends); got != 2 {
		t.Errorf("bank sends after duplicate tx hash = %d, want 2", got)
	}

	// A send with no tx hash is always appended (cannot be deduplicated).
	log.AddBankSend(BankSendActivity{To: "addrC", Amount: "2ulume"})
	log.AddBankSend(BankSendActivity{To: "addrC", Amount: "2ulume"})
	if got := len(log.BankSends); got != 4 {
		t.Errorf("bank sends after hashless adds = %d, want 4", got)
	}
}

func TestActivityLogWithdrawAddressSkipsConsecutiveDupes(t *testing.T) {
	var log ActivityLog
	log.AddWithdrawAddress("addrW")
	log.AddWithdrawAddress("addrW") // consecutive dup skipped
	log.AddWithdrawAddress("addrX")
	log.AddWithdrawAddress("addrW") // not consecutive, recorded
	if got := len(log.WithdrawAddresses); got != 3 {
		t.Errorf("withdraw addresses = %d, want 3", got)
	}
}
