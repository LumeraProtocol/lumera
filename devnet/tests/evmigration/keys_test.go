package main

import "testing"

func TestPrepareRuntimeAllowed(t *testing.T) {
	if !prepareRuntimeAllowed(118) {
		t.Fatal("expected coin-type 118 to allow prepare mode")
	}
	if prepareRuntimeAllowed(60) {
		t.Fatal("expected coin-type 60 to disable prepare mode")
	}
}
