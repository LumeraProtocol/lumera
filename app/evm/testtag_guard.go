package evm

import "strings"

const testTagRequiredMessage = "EVM tests require the 'test' build tag: go test -tags=test ./..."

type testTagRequiredPanic struct{}

func (testTagRequiredPanic) Error() string {
	return testTagRequiredMessage
}

func panicTestTagRequired() {
	panic(testTagRequiredPanic{})
}

// IsTestTagRequiredPanic reports whether a recovered panic value indicates
// the missing '-tags=test' EVM test build tag.
func IsTestTagRequiredPanic(v any) bool {
	_, ok := v.(testTagRequiredPanic)
	return ok
}

// IsChainConfigAlreadySetPanic reports whether a recovered panic value is
// the "chainConfig already set" error from cosmos-evm's global chain config.
// Without '-tags=test', a second App instantiation in the same process
// triggers this because the prod SetChainConfig is not resettable.
// We match a stable prefix rather than the full message to avoid breakage
// if upstream rewrites the error text.
func IsChainConfigAlreadySetPanic(v any) bool {
	if err, ok := v.(error); ok {
		return strings.Contains(err.Error(), "chainConfig already set")
	}
	return false
}

// TestTagRequiredMessage returns the canonical guidance for running EVM tests.
func TestTagRequiredMessage() string {
	return testTagRequiredMessage
}
