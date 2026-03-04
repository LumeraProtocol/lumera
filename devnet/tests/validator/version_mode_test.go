package validator

import (
	"testing"

	pkgversion "github.com/LumeraProtocol/lumera/pkg/version"
)

func TestVersionGTE(t *testing.T) {
	tests := []struct {
		name    string
		current string
		floor   string
		want    bool
	}{
		{name: "equal", current: "v1.12.0", floor: "v1.12.0", want: true},
		{name: "greater patch", current: "v1.12.1", floor: "v1.12.0", want: true},
		{name: "greater minor", current: "v1.13.0", floor: "v1.12.0", want: true},
		{name: "lower patch", current: "v1.11.9", floor: "v1.12.0", want: false},
		{name: "suffix handled", current: "v1.12.0-rc1", floor: "v1.12.0", want: true},
		{name: "plus metadata handled", current: "v1.12.0+build1", floor: "v1.12.0", want: true},
		{name: "fallback string compare", current: "vnext", floor: "v1.12.0", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := pkgversion.GTE(tc.current, tc.floor)
			if got != tc.want {
				t.Fatalf("GTE(%q, %q) = %v, want %v", tc.current, tc.floor, got, tc.want)
			}
		})
	}
}
