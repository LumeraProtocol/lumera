package text

import (
	"testing"
)

func TestContainsAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		needles []string
		want    bool
	}{
		{
			name:    "matches one needle",
			value:   "insufficient fee",
			needles: []string{"timeout", "insufficient fee"},
			want:    true,
		},
		{
			name:    "no matches",
			value:   "ok",
			needles: []string{"error", "fail"},
			want:    false,
		},
		{
			name:    "empty needles",
			value:   "anything",
			needles: nil,
			want:    false,
		},
		{
			name:    "empty needle matches by strings.Contains behavior",
			value:   "abc",
			needles: []string{""},
			want:    true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ContainsAny(tc.value, tc.needles...)
			if got != tc.want {
				t.Fatalf("ContainsAny(%q, %v)=%v want %v", tc.value, tc.needles, got, tc.want)
			}
		})
	}
}
