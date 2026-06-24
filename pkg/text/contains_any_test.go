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

func TestLastNonEmptyLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "single line", input: "hello", want: "hello"},
		{name: "multiple lines", input: "first\nsecond\nthird", want: "third"},
		{name: "trailing newlines", input: "first\nsecond\n\n\n", want: "second"},
		{name: "leading newlines", input: "\n\nfirst\nsecond", want: "second"},
		{name: "whitespace lines", input: "first\n   \n  \n", want: "first"},
		{name: "empty string", input: "", want: ""},
		{name: "only whitespace", input: "   \n  \n  ", want: ""},
		{name: "trims result", input: "first\n  second  \n", want: "second"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := LastNonEmptyLine(tc.input)
			if got != tc.want {
				t.Fatalf("LastNonEmptyLine(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
