package main

import "math/rand"

// SelectValidators picks up to n distinct validators from the given set using
// the provided RNG. The RNG is injected so activity planning is deterministic
// under test. n is clamped to the number of validators available.
func SelectValidators(validators []string, n int, rng *rand.Rand) []string {
	if n <= 0 || len(validators) == 0 {
		return nil
	}
	n = min(n, len(validators))
	perm := rng.Perm(len(validators))
	out := make([]string, n)
	for i := range n {
		out[i] = validators[perm[i]]
	}
	return out
}

// SelectRedelegationPair picks a distinct (source, destination) validator pair
// for a redelegation. It returns ok=false when fewer than two validators are
// available.
func SelectRedelegationPair(validators []string, rng *rand.Rand) (src, dst string, ok bool) {
	if len(validators) < 2 {
		return "", "", false
	}
	pair := SelectValidators(validators, 2, rng)
	return pair[0], pair[1], true
}
