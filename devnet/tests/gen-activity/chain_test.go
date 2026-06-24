package main

import (
	"math/rand"
	"testing"
)

func TestRandomFundingAmount(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	t.Run("stays within [max/2, max]", func(t *testing.T) {
		const max int64 = 10000000
		for range 1000 {
			got := randomFundingAmount(max, rng)
			if got < max/2 || got > max {
				t.Fatalf("amount %d out of range [%d, %d]", got, max/2, max)
			}
		}
	})

	t.Run("non-positive max yields zero", func(t *testing.T) {
		if got := randomFundingAmount(0, rng); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("tiny max yields max", func(t *testing.T) {
		if got := randomFundingAmount(1, rng); got != 1 {
			t.Errorf("got %d, want 1", got)
		}
	})
}

func TestUnfundedTargets(t *testing.T) {
	reg := NewRegistry("c", "f", "addr", "evm", "t0")
	a := newRec("a", "a")
	a.Funded = true
	b := newRec("b", "b")
	reg.UpsertAccount(a)
	reg.UpsertAccount(b)

	got := unfundedTargets(reg)
	if len(got) != 1 || got[0].Name != "b" {
		t.Errorf("unfunded = %v, want [b]", got)
	}
}

func TestNewChainCLIUsesFixedGasForOfflineFunding(t *testing.T) {
	cli := newChainCLI(&Config{
		Bin:            "lumerad",
		ChainID:        "lumera-devnet-1",
		RPC:            "tcp://localhost:26657",
		KeyringBackend: "test",
	})

	if cli.Gas == "auto" {
		t.Fatal("newChainCLI must not use gas=auto with offline sequence-signed funding txs")
	}
	if cli.Gas == "" {
		t.Fatal("newChainCLI should set an explicit funding-safe gas limit")
	}
}
