package main

import (
	"math/rand"
	"slices"
	"testing"
)

func newRNG() *rand.Rand { return rand.New(rand.NewSource(1)) }

func TestSelectValidators(t *testing.T) {
	vals := []string{"v1", "v2", "v3", "v4"}

	t.Run("returns n distinct validators", func(t *testing.T) {
		got := SelectValidators(vals, 3, newRNG())
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
		seen := map[string]bool{}
		for _, v := range got {
			if seen[v] {
				t.Errorf("duplicate validator %q in %v", v, got)
			}
			seen[v] = true
			if !slices.Contains(vals, v) {
				t.Errorf("selected %q not in source set", v)
			}
		}
	})

	t.Run("clamps n to the number available", func(t *testing.T) {
		got := SelectValidators(vals, 10, newRNG())
		if len(got) != len(vals) {
			t.Errorf("len = %d, want %d", len(got), len(vals))
		}
	})

	t.Run("non-positive n yields none", func(t *testing.T) {
		if got := SelectValidators(vals, 0, newRNG()); len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})

	t.Run("empty source yields none", func(t *testing.T) {
		if got := SelectValidators(nil, 3, newRNG()); len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
}

func TestSelectRedelegationPair(t *testing.T) {
	t.Run("returns distinct src and dst when >= 2 validators", func(t *testing.T) {
		src, dst, ok := SelectRedelegationPair([]string{"v1", "v2", "v3"}, newRNG())
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if src == dst {
			t.Errorf("src == dst (%q)", src)
		}
	})

	t.Run("not ok with fewer than 2 validators", func(t *testing.T) {
		if _, _, ok := SelectRedelegationPair([]string{"v1"}, newRNG()); ok {
			t.Error("ok = true for single validator, want false")
		}
		if _, _, ok := SelectRedelegationPair(nil, newRNG()); ok {
			t.Error("ok = true for empty set, want false")
		}
	})
}
