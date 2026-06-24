package text

import (
	"testing"
)

func TestParseAppOptionBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  interface{}
		want bool
	}{
		{name: "nil", raw: nil, want: false},
		{name: "bool true", raw: true, want: true},
		{name: "bool false", raw: false, want: false},
		{name: "string true", raw: "true", want: true},
		{name: "string false", raw: "false", want: false},
		{name: "string 1", raw: "1", want: true},
		{name: "string 0", raw: "0", want: false},
		{name: "string invalid", raw: "maybe", want: false},
		{name: "int 1", raw: 1, want: true},
		{name: "int 0", raw: 0, want: false},
		{name: "int64 1", raw: int64(1), want: true},
		{name: "int64 0", raw: int64(0), want: false},
		{name: "uint 1", raw: uint(1), want: true},
		{name: "uint 0", raw: uint(0), want: false},
		{name: "unknown type", raw: struct{}{}, want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ParseAppOptionBool(tc.raw)
			if got != tc.want {
				t.Fatalf("ParseAppOptionBool(%v)=%v want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestEnvOrDefault(t *testing.T) {
	const key = "TEST_ENV_OR_DEFAULT_KEY"

	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv(key, "custom")
		if got := EnvOrDefault(key, "fallback"); got != "custom" {
			t.Fatalf("EnvOrDefault(%q, %q) = %q, want %q", key, "fallback", got, "custom")
		}
	})

	t.Run("returns fallback when empty", func(t *testing.T) {
		t.Setenv(key, "")
		if got := EnvOrDefault(key, "fallback"); got != "fallback" {
			t.Fatalf("EnvOrDefault(%q, %q) = %q, want %q", key, "fallback", got, "fallback")
		}
	})

	t.Run("returns fallback when unset", func(t *testing.T) {
		// key not set in this subtest
		if got := EnvOrDefault("DEFINITELY_UNSET_12345", "fb"); got != "fb" {
			t.Fatalf("EnvOrDefault(%q, %q) = %q, want %q", "DEFINITELY_UNSET_12345", "fb", got, "fb")
		}
	})
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		name  string
		value string // empty means unset
		want  bool
	}{
		{name: "true", value: "true", want: true},
		{name: "TRUE", value: "TRUE", want: true},
		{name: "1", value: "1", want: true},
		{name: "false", value: "false", want: false},
		{name: "0", value: "0", want: false},
		{name: "empty", value: "", want: false},
		{name: "invalid", value: "nope", want: false},
		{name: "whitespace true", value: "  true  ", want: true},
	}

	const key = "TEST_ENV_BOOL_KEY"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == "" {
				t.Setenv(key, "")
			} else {
				t.Setenv(key, tc.value)
			}
			got := EnvBool(key)
			if got != tc.want {
				t.Fatalf("EnvBool(%q) with value %q = %v, want %v", key, tc.value, got, tc.want)
			}
		})
	}
}
