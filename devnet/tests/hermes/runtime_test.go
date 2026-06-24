package hermes

import "testing"

func TestIsHermesContainerRuntime(t *testing.T) {
	t.Setenv(hermesContainerEnv, "")
	if isHermesContainerRuntime() {
		t.Fatal("empty runtime marker should not allow Hermes tests")
	}

	for _, value := range []string{"true", "TRUE", "1"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(hermesContainerEnv, value)
			if !isHermesContainerRuntime() {
				t.Fatalf("%s=%q should allow Hermes tests", hermesContainerEnv, value)
			}
		})
	}
}
