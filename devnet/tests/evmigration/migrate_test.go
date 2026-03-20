package main

import "testing"

func TestMigratedAccountBaseName(t *testing.T) {
	cases := map[string]string{
		"pre-evm-val1-000":    "evm-val1-000",
		"pre-evmex-val1-003":  "evmex-val1-003",
		"evm_test_val1_000":   "evm-val1-000",
		"evm_testex_val1_004": "evmex-val1-004",
		"legacy_000":          "evm-000",
		"extra_000":           "evmex-000",
		"custom_name_example": "evm-custom-name-example",
	}

	for input, want := range cases {
		got := migratedAccountBaseName(input, true)
		if input == "extra_000" || input == "pre-evmex-val1-003" || input == "evm_testex_val1_004" {
			got = migratedAccountBaseName(input, false)
		}
		if got != want {
			t.Fatalf("migratedAccountBaseName(%q) = %q, want %q", input, got, want)
		}
	}
}
