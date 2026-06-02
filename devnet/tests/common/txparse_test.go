package common

import (
	"errors"
	"slices"
	"testing"
)

func TestParseIncorrectAccountSequence(t *testing.T) {
	t.Run("extracts expected and got", func(t *testing.T) {
		err := errors.New("rpc error: account sequence mismatch, incorrect account sequence: expected 7, got 5: unauthorized")
		exp, got, ok := ParseIncorrectAccountSequence(err)
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if exp != 7 || got != 5 {
			t.Errorf("expected=%d got=%d, want 7/5", exp, got)
		}
	})

	t.Run("nil and unrelated errors are not matched", func(t *testing.T) {
		if _, _, ok := ParseIncorrectAccountSequence(nil); ok {
			t.Error("ok = true for nil error")
		}
		if _, _, ok := ParseIncorrectAccountSequence(errors.New("insufficient funds")); ok {
			t.Error("ok = true for unrelated error")
		}
	})
}

func TestParseSignatureMismatchAccountNumber(t *testing.T) {
	err := errors.New("signature verification failed; please verify account number (76) and chain-id (lumera-devnet-1): unauthorized")
	n, ok := ParseSignatureMismatchAccountNumber(err)
	if !ok || n != 76 {
		t.Errorf("got %d ok=%v, want 76 true", n, ok)
	}
	if _, ok := ParseSignatureMismatchAccountNumber(errors.New("nope")); ok {
		t.Error("ok = true for unrelated error")
	}
}

func TestExtractJSONPayload(t *testing.T) {
	t.Run("pulls the JSON object out of mixed output", func(t *testing.T) {
		out := "gas estimate: 12345\n{\"code\":0,\"txhash\":\"ABC\"}\n"
		got, ok := ExtractJSONPayload(out)
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if got != `{"code":0,"txhash":"ABC"}` {
			t.Errorf("got %q", got)
		}
	})

	t.Run("no braces yields not-ok", func(t *testing.T) {
		if _, ok := ExtractJSONPayload("plain text"); ok {
			t.Error("ok = true for brace-less output")
		}
	})
}

func TestParseAuthAccountNumberAndSequence(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		wantNum uint64
		wantSeq uint64
	}{
		{
			name:    "proto-json base account",
			out:     `{"account":{"account_number":"42","sequence":"7"}}`,
			wantNum: 42, wantSeq: 7,
		},
		{
			name:    "amino-json envelope",
			out:     `{"account":{"value":{"account_number":"5","sequence":"2"}}}`,
			wantNum: 5, wantSeq: 2,
		},
		{
			name:    "module-account nested base_account",
			out:     `{"account":{"base_account":{"account_number":"9","sequence":"0"}}}`,
			wantNum: 9, wantSeq: 0,
		},
		{
			name:    "vesting proto-json",
			out:     `{"account":{"base_vesting_account":{"base_account":{"account_number":"11","sequence":"3"}}}}`,
			wantNum: 11, wantSeq: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			num, seq, err := ParseAuthAccountNumberAndSequence(tc.out)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if num != tc.wantNum || seq != tc.wantSeq {
				t.Errorf("got num=%d seq=%d, want %d/%d", num, seq, tc.wantNum, tc.wantSeq)
			}
		})
	}

	if _, _, err := ParseAuthAccountNumberAndSequence("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseBankBalance(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want int64
	}{
		{"singular balance shape", `{"balance":{"denom":"ulume","amount":"1000"}}`, 1000},
		{"flat amount shape", `{"amount":"500","denom":"ulume"}`, 500},
		{"zero/empty balance", `{"balance":null}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBankBalance(tc.out)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}

	if _, err := ParseBankBalance("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseRedelegationCount(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want int
	}{
		{"plural responses", `{"redelegation_responses":[{"x":1},{"y":2}]}`, 2},
		{"singular non-null", `{"redelegation":{"delegator_address":"a"}}`, 1},
		{"singular null", `{"redelegation":null}`, 0},
		{"empty", `{"redelegation_responses":[]}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRedelegationCount(tc.out)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseAuthzGrantCount(t *testing.T) {
	if n, _ := ParseAuthzGrantCount(`{"grants":[{"authorization":{}}]}`); n != 1 {
		t.Errorf("got %d, want 1", n)
	}
	if n, _ := ParseAuthzGrantCount(`{"grants":[]}`); n != 0 {
		t.Errorf("got %d, want 0", n)
	}
	if _, err := ParseAuthzGrantCount("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseFeegrantExists(t *testing.T) {
	if ok, _ := ParseFeegrantExists(`{"allowance":{"granter":"g","grantee":"e"}}`); !ok {
		t.Error("expected allowance to exist")
	}
	if ok, _ := ParseFeegrantExists(`{"allowance":null}`); ok {
		t.Error("expected no allowance")
	}
	if _, err := ParseFeegrantExists("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseValidatorAddresses(t *testing.T) {
	jsonOut := `{"validators":[{"operator_address":"lumeravaloper1aaa"},{"operator_address":"lumeravaloper1bbb"}]}`
	got, err := ParseValidatorAddresses([]byte(jsonOut))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"lumeravaloper1aaa", "lumeravaloper1bbb"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if _, err := ParseValidatorAddresses([]byte("not json")); err == nil {
		t.Error("expected error parsing invalid JSON")
	}
}
