package main

import (
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
