package cmd

import "testing"

func TestNewRootCmd_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewRootCmd panicked: %v", r)
		}
	}()

	if cmd := NewRootCmd(); cmd == nil {
		t.Fatal("NewRootCmd returned nil")
	}
}
