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
