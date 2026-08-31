package cmd

import "testing"

func TestLoopbackAddrForPublicIsStableAndUnique(t *testing.T) {
	t.Parallel()

	tests := []struct {
		public string
		want   string
	}{
		{public: "127.0.0.1:8545", want: "127.0.0.1:42336"},
		{public: "0.0.0.0:8645", want: "127.0.0.1:42436"},
		{public: "[::]:8745", want: "127.0.0.1:42536"},
		{public: "localhost:32768", want: "127.0.0.1:2047"},
		{public: "127.0.0.1:32863", want: "127.0.0.1:2142"},
	}

	seen := make(map[string]string, len(tests))
	for _, tc := range tests {
		t.Run(tc.public, func(t *testing.T) {
			got, err := loopbackAddrForPublic(tc.public)
			if err != nil {
				t.Fatalf("loopbackAddrForPublic(%q): %v", tc.public, err)
			}
			if got != tc.want {
				t.Fatalf("loopbackAddrForPublic(%q) = %q, want %q", tc.public, got, tc.want)
			}
			if previous, exists := seen[got]; exists {
				t.Fatalf("public addresses %q and %q mapped to the same internal address %q", previous, tc.public, got)
			}
			seen[got] = tc.public
		})
	}
}

func TestLoopbackAddrForPublicRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	for _, publicAddr := range []string{"", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:65536"} {
		if _, err := loopbackAddrForPublic(publicAddr); err == nil {
			t.Fatalf("loopbackAddrForPublic(%q) unexpectedly succeeded", publicAddr)
		}
	}
}
