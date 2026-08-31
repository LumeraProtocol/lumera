package common

import "testing"

func TestParseCoin(t *testing.T) {
	t.Run("valid amount and denom", func(t *testing.T) {
		c, err := ParseCoin("10000000ulume")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Amount != 10000000 {
			t.Errorf("amount = %d, want 10000000", c.Amount)
		}
		if c.Denom != "ulume" {
			t.Errorf("denom = %q, want %q", c.Denom, "ulume")
		}
	})

	t.Run("surrounding whitespace tolerated", func(t *testing.T) {
		c, err := ParseCoin("  42ulume  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Amount != 42 || c.Denom != "ulume" {
			t.Errorf("got %d%s, want 42ulume", c.Amount, c.Denom)
		}
	})

	t.Run("rejects missing denom", func(t *testing.T) {
		if _, err := ParseCoin("100"); err == nil {
			t.Error("expected error for missing denom, got nil")
		}
	})

	t.Run("rejects missing amount", func(t *testing.T) {
		if _, err := ParseCoin("ulume"); err == nil {
			t.Error("expected error for missing amount, got nil")
		}
	})

	t.Run("rejects negative amount", func(t *testing.T) {
		if _, err := ParseCoin("-5ulume"); err == nil {
			t.Error("expected error for negative amount, got nil")
		}
	})

	t.Run("rejects empty string", func(t *testing.T) {
		if _, err := ParseCoin(""); err == nil {
			t.Error("expected error for empty string, got nil")
		}
	})
}

func TestValidateMaxAccountAmount(t *testing.T) {
	t.Run("accepts positive ulume", func(t *testing.T) {
		c, err := ValidateMaxAccountAmount("10000000ulume")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Amount != 10000000 || c.Denom != "ulume" {
			t.Errorf("got %d%s, want 10000000ulume", c.Amount, c.Denom)
		}
	})

	t.Run("rejects zero amount", func(t *testing.T) {
		if _, err := ValidateMaxAccountAmount("0ulume"); err == nil {
			t.Error("expected error for zero max-account-amount, got nil")
		}
	})

	t.Run("rejects non-ulume denom", func(t *testing.T) {
		if _, err := ValidateMaxAccountAmount("100uatom"); err == nil {
			t.Error("expected error for non-ulume denom, got nil")
		}
	})
}
