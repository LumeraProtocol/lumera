package common

import (
	"reflect"
	"testing"
)

func TestPlanFundingBatches(t *testing.T) {
	t.Run("single full batch", func(t *testing.T) {
		got := PlanFundingBatches(5, 10, 10)
		want := []FundingBatch{{Sequences: []uint64{5, 6, 7, 8, 9, 10, 11, 12, 13, 14}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("chunks with contiguous ascending sequences", func(t *testing.T) {
		got := PlanFundingBatches(0, 23, 10)
		if len(got) != 3 {
			t.Fatalf("batches = %d, want 3", len(got))
		}
		// Flatten and assert the sequence is contiguous 0..22 across batches.
		var flat []uint64
		for _, b := range got {
			flat = append(flat, b.Sequences...)
		}
		if len(flat) != 23 {
			t.Fatalf("total sequences = %d, want 23", len(flat))
		}
		for i, seq := range flat {
			if seq != uint64(i) {
				t.Fatalf("sequence[%d] = %d, want %d", i, seq, i)
			}
		}
		if len(got[2].Sequences) != 3 {
			t.Errorf("last batch size = %d, want 3", len(got[2].Sequences))
		}
	})

	t.Run("zero count yields no batches", func(t *testing.T) {
		if got := PlanFundingBatches(0, 0, 10); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("non-positive batch size means one batch", func(t *testing.T) {
		got := PlanFundingBatches(100, 4, 0)
		want := []FundingBatch{{Sequences: []uint64{100, 101, 102, 103}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}
