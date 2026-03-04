package version

import "testing"

func TestGTE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		floor   string
		want    bool
	}{
		{name: "equal", current: "v1.12.0", floor: "v1.12.0", want: true},
		{name: "greater patch", current: "v1.12.1", floor: "v1.12.0", want: true},
		{name: "greater minor", current: "v1.13.0", floor: "v1.12.0", want: true},
		{name: "greater major", current: "v2.0.0", floor: "v1.99.99", want: true},
		{name: "lower patch", current: "v1.11.9", floor: "v1.12.0", want: false},
		{name: "lower minor", current: "v1.11.0", floor: "v1.12.0", want: false},
		{name: "lower major", current: "v0.9.0", floor: "v1.0.0", want: false},
		{name: "suffix handled", current: "v1.12.0-rc1", floor: "v1.12.0", want: true},
		{name: "plus metadata handled", current: "v1.12.0+build1", floor: "v1.12.0", want: true},
		{name: "no v prefix", current: "1.12.0", floor: "1.12.0", want: true},
		{name: "two-part version", current: "v1.12", floor: "v1.12.0", want: true},
		{name: "fallback string compare equal", current: "vnext", floor: "vnext", want: true},
		{name: "fallback string compare mismatch", current: "vnext", floor: "v1.12.0", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := GTE(tc.current, tc.floor)
			if got != tc.want {
				t.Fatalf("GTE(%q, %q) = %v, want %v", tc.current, tc.floor, got, tc.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		input             string
		wantMaj, wantMin  int
		wantPatch         int
		wantOk            bool
	}{
		{name: "full", input: "v1.12.3", wantMaj: 1, wantMin: 12, wantPatch: 3, wantOk: true},
		{name: "no prefix", input: "1.2.3", wantMaj: 1, wantMin: 2, wantPatch: 3, wantOk: true},
		{name: "two parts", input: "v1.2", wantMaj: 1, wantMin: 2, wantPatch: 0, wantOk: true},
		{name: "pre-release stripped", input: "v1.2.3-rc1", wantMaj: 1, wantMin: 2, wantPatch: 3, wantOk: true},
		{name: "build metadata stripped", input: "v1.2.3+build", wantMaj: 1, wantMin: 2, wantPatch: 3, wantOk: true},
		{name: "uppercase V", input: "V1.0.0", wantMaj: 1, wantMin: 0, wantPatch: 0, wantOk: true},
		{name: "whitespace trimmed", input: "  v1.0.0  ", wantMaj: 1, wantMin: 0, wantPatch: 0, wantOk: true},
		{name: "single part", input: "v1", wantOk: false},
		{name: "non-numeric", input: "vnext", wantOk: false},
		{name: "empty", input: "", wantOk: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			maj, min, patch, ok := Parse(tc.input)
			if ok != tc.wantOk {
				t.Fatalf("Parse(%q) ok = %v, want %v", tc.input, ok, tc.wantOk)
			}
			if !ok {
				return
			}
			if maj != tc.wantMaj || min != tc.wantMin || patch != tc.wantPatch {
				t.Fatalf("Parse(%q) = (%d, %d, %d), want (%d, %d, %d)",
					tc.input, maj, min, patch, tc.wantMaj, tc.wantMin, tc.wantPatch)
			}
		})
	}
}
