package common

import (
	"slices"
	"testing"
)

func TestParseActionStates(t *testing.T) {
	t.Run("parses canonical list", func(t *testing.T) {
		got, err := ParseActionStates("pending,done,approved")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []ActionState{ActionStatePending, ActionStateDone, ActionStateApproved}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("trims whitespace and is case-insensitive", func(t *testing.T) {
		got, err := ParseActionStates(" Pending , DONE ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []ActionState{ActionStatePending, ActionStateDone}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("deduplicates preserving first-seen order", func(t *testing.T) {
		got, err := ParseActionStates("done,pending,done")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []ActionState{ActionStateDone, ActionStatePending}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("rejects unknown state", func(t *testing.T) {
		if _, err := ParseActionStates("pending,bogus"); err == nil {
			t.Error("expected error for unknown state, got nil")
		}
	})

	t.Run("rejects empty list", func(t *testing.T) {
		if _, err := ParseActionStates("   "); err == nil {
			t.Error("expected error for empty list, got nil")
		}
	})
}
