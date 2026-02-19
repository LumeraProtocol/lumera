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

// TestTagRequiredMessage returns the canonical guidance for running EVM tests.
func TestTagRequiredMessage() string {
	return testTagRequiredMessage
}
