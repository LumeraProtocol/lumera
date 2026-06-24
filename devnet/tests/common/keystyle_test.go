package common

import "testing"

func TestExtractSemver(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"plain version", "1.11.0", "v1.11.0", true},
		{"leading v", "v1.11.0", "v1.11.0", true},
		{"trailing newline", "1.11.0\n", "v1.11.0", true},
		{"labelled long output", "name: lumerad\nversion: 1.12.3\ncommit: abc", "v1.12.3", true},
		{"skips sdk_version line", "cosmos_sdk_version: 0.53.6\nversion: 2.0.0", "v2.0.0", true},
		{"no version", "no semantic version here", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ExtractSemver(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (got %q)", ok, tc.ok, got)
			}
			if ok && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.11.0", "v1.11.0", 0},
		{"v1.12.0", "v1.11.0", 1},
		{"v1.10.9", "v1.11.0", -1},
		{"2.0.0", "v1.99.99", 1},
		{"v1.11.0", "v1.11.1", -1},
	}
	for _, tc := range cases {
		got, err := CompareSemver(tc.a, tc.b)
		if err != nil {
			t.Fatalf("CompareSemver(%q,%q) error: %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("CompareSemver(%q,%q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}

	if _, err := CompareSemver("not-a-version", "v1.0.0"); err == nil {
		t.Error("expected error comparing invalid semver, got nil")
	}
}

func TestKeyStyleForVersion(t *testing.T) {
	const cutover = "v1.11.0"

	t.Run("pre-EVM version is legacy secp256k1 coin118", func(t *testing.T) {
		ks, err := KeyStyleForVersion("v1.10.0", cutover)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ks.CoinType != 118 || ks.Algo != "secp256k1" || ks.EVM {
			t.Errorf("got %+v, want legacy secp256k1/118", ks)
		}
		if ks.Name() != "legacy" {
			t.Errorf("Name() = %q, want legacy", ks.Name())
		}
	})

	t.Run("cutover version is EVM eth_secp256k1 coin60", func(t *testing.T) {
		ks, err := KeyStyleForVersion("v1.11.0", cutover)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ks.CoinType != 60 || ks.Algo != "eth_secp256k1" || !ks.EVM {
			t.Errorf("got %+v, want evm eth_secp256k1/60", ks)
		}
		if ks.Name() != "evm" {
			t.Errorf("Name() = %q, want evm", ks.Name())
		}
	})

	t.Run("post-cutover version is EVM", func(t *testing.T) {
		ks, err := KeyStyleForVersion("v2.0.0", cutover)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ks.EVM {
			t.Errorf("got %+v, want evm", ks)
		}
	})

	t.Run("unparseable version errors", func(t *testing.T) {
		if _, err := KeyStyleForVersion("garbage", cutover); err == nil {
			t.Error("expected error for unparseable version, got nil")
		}
	})
}
