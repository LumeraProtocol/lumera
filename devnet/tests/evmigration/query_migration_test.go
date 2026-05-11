package main

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseIncorrectAccountSequence(t *testing.T) {
	err := errors.New("tx rejected code=32 raw_log=account sequence mismatch, expected 7, got 6: incorrect account sequence")

	expected, got, ok := parseIncorrectAccountSequence(err)
	if !ok {
		t.Fatal("expected incorrect account sequence error to be detected")
	}
	if expected != 7 || got != 6 {
		t.Fatalf("unexpected parsed sequence mismatch: expected=%d got=%d", expected, got)
	}
}

func TestParseIncorrectAccountSequenceRejectsOtherErrors(t *testing.T) {
	if _, _, ok := parseIncorrectAccountSequence(errors.New("some other error")); ok {
		t.Fatal("expected unrelated error to be ignored")
	}
}

func TestAuthAccountLooksVesting(t *testing.T) {
	t.Run("proto vesting type", func(t *testing.T) {
		out := `{"account":{"@type":"/cosmos.vesting.v1beta1.DelayedVestingAccount","base_vesting_account":{"base_account":{"address":"lumera1test"}}}}`
		if !authAccountLooksVesting(out) {
			t.Fatal("expected delayed vesting account to be detected")
		}
	})

	t.Run("legacy amino vesting type", func(t *testing.T) {
		out := `{"account":{"type":"cosmos-sdk/ContinuousVestingAccount","value":{"base_vesting_account":{"base_account":{"address":"lumera1test"}}}}}`
		if !authAccountLooksVesting(out) {
			t.Fatal("expected legacy vesting account to be detected")
		}
	})

	t.Run("base account", func(t *testing.T) {
		out := `{"account":{"@type":"/cosmos.auth.v1beta1.BaseAccount","address":"lumera1test"}}`
		if authAccountLooksVesting(out) {
			t.Fatal("expected base account not to be detected as vesting")
		}
	})
}

func TestAuthAccountLooksPermanentLocked(t *testing.T) {
	t.Run("proto permanent locked type", func(t *testing.T) {
		out := `{"account":{"@type":"/cosmos.vesting.v1beta1.PermanentLockedAccount","base_vesting_account":{"base_account":{"address":"lumera1test"}}}}`
		if !authAccountLooksPermanentLocked(out) {
			t.Fatal("expected permanent locked account to be detected")
		}
	})

	t.Run("legacy amino permanent locked type", func(t *testing.T) {
		out := `{"account":{"type":"cosmos-sdk/PermanentLockedAccount","value":{"base_vesting_account":{"base_account":{"address":"lumera1test"}}}}}`
		if !authAccountLooksPermanentLocked(out) {
			t.Fatal("expected legacy permanent locked account to be detected")
		}
	})

	t.Run("different vesting type", func(t *testing.T) {
		out := `{"account":{"@type":"/cosmos.vesting.v1beta1.DelayedVestingAccount","base_vesting_account":{"base_account":{"address":"lumera1test"}}}}`
		if authAccountLooksPermanentLocked(out) {
			t.Fatal("expected delayed vesting account not to be treated as permanent locked")
		}
	})
}

// TestParseAuthAccountNumberAndSequence covers every SDK response shape the
// helper has to tolerate. The vesting cases are regression coverage for a
// prepare-mode crash where supernova_validator_2's multisig was wrapped in a
// PermanentLockedAccount and the parser only knew BaseAccount/ModuleAccount
// shapes — funder bootstrap then died with "account_number missing".
func TestParseAuthAccountNumberAndSequence(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		wantNum uint64
		wantSeq uint64
	}{
		{
			name:    "BaseAccount proto-JSON",
			out:     `{"account":{"@type":"/cosmos.auth.v1beta1.BaseAccount","address":"lumera1test","account_number":"42","sequence":"7"}}`,
			wantNum: 42,
			wantSeq: 7,
		},
		{
			name:    "BaseAccount amino-JSON",
			out:     `{"account":{"type":"cosmos-sdk/BaseAccount","value":{"address":"lumera1test","account_number":"42","sequence":"7"}}}`,
			wantNum: 42,
			wantSeq: 7,
		},
		{
			name:    "ModuleAccount nested base_account",
			out:     `{"account":{"@type":"/cosmos.auth.v1beta1.ModuleAccount","name":"distribution","base_account":{"address":"lumera1mod","account_number":"3","sequence":"0"}}}`,
			wantNum: 3,
			wantSeq: 0,
		},
		{
			name:    "PermanentLockedAccount amino-JSON (the failing devnet case)",
			out:     `{"account":{"type":"/cosmos.vesting.v1beta1.PermanentLockedAccount","value":{"base_vesting_account":{"base_account":{"address":"lumera1s4audz3q5syqfjd2r7e7jny67dlat0cqqh76m8","account_number":"99","sequence":"0"}}}}}`,
			wantNum: 99,
			wantSeq: 0,
		},
		{
			name:    "ContinuousVestingAccount proto-JSON",
			out:     `{"account":{"@type":"/cosmos.vesting.v1beta1.ContinuousVestingAccount","base_vesting_account":{"base_account":{"address":"lumera1cv","account_number":"17","sequence":"4"}}}}`,
			wantNum: 17,
			wantSeq: 4,
		},
		{
			name:    "DelayedVestingAccount amino-JSON, sequence omitted (fresh acct)",
			out:     `{"account":{"type":"cosmos-sdk/DelayedVestingAccount","value":{"base_vesting_account":{"base_account":{"address":"lumera1dv","account_number":"5"}}}}}`,
			wantNum: 5,
			wantSeq: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotNum, gotSeq, err := parseAuthAccountNumberAndSequence(tc.out)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotNum != tc.wantNum {
				t.Fatalf("account_number: got %d, want %d", gotNum, tc.wantNum)
			}
			if gotSeq != tc.wantSeq {
				t.Fatalf("sequence: got %d, want %d", gotSeq, tc.wantSeq)
			}
		})
	}
}

func TestParseAuthAccountNumberAndSequence_MissingAccount(t *testing.T) {
	// Account body present but no account_number anywhere — must error so the
	// caller's wait-for-account-on-chain retry can kick in.
	_, _, err := parseAuthAccountNumberAndSequence(`{"account":{"type":"/cosmos.auth.v1beta1.BaseAccount","value":{"address":"lumera1test"}}}`)
	if err == nil {
		t.Fatal("expected error when account_number is missing")
	}
}

// TestAuthAccountPayloadTypeName_IgnoresNestedPubkeyType verifies that a
// BaseAccount with a nested public_key.@type doesn't leak the pubkey type as
// the "account type" — regression test for a previous map-iteration-order bug
// where Go's random map walk could surface /cosmos.crypto.secp256k1.PubKey
// instead of the surrounding /cosmos.auth.v1beta1.BaseAccount.
func TestAuthAccountPayloadTypeName_IgnoresNestedPubkeyType(t *testing.T) {
	raw := `{"account":{"@type":"/cosmos.auth.v1beta1.BaseAccount","address":"lumera1test","public_key":{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AAA"}}}`
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Run many times to exercise Go's randomized map iteration order.
	for i := 0; i < 50; i++ {
		if got := authAccountPayloadTypeName(parsed); got != "/cosmos.auth.v1beta1.BaseAccount" {
			t.Fatalf("iteration %d: expected BaseAccount @type, got %q", i, got)
		}
	}
}
