package evm

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
func IsChainConfigAlreadySetPanic(v any) bool {
	if err, ok := v.(error); ok {
		return err.Error() == "chainConfig already set. Cannot set again the chainConfig"
	}
	return false
}

// TestTagRequiredMessage returns the canonical guidance for running EVM tests.
func TestTagRequiredMessage() string {
	return testTagRequiredMessage
}
