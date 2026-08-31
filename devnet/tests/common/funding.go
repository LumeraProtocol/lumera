package common

// FundingBatch is a group of funder transfers to broadcast as one burst, each
// signed with an explicit, increasing account sequence number. The batcher
// waits for chain inclusion between batches.
type FundingBatch struct {
	Sequences []uint64
}

// PlanFundingBatches lays out the funder's transfers as batches of contiguous,
// ascending sequence numbers starting at startSeq. count is the total number of
// transfers; batchSize caps each batch. A non-positive batchSize places all
// transfers in a single batch. A non-positive count yields no batches.
//
// Centralizing sequence assignment here is what lets the single funder signer
// broadcast a burst without colliding on its account sequence.
func PlanFundingBatches(startSeq uint64, count, batchSize int) []FundingBatch {
	if count <= 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = count
	}
	var batches []FundingBatch
	seq := startSeq
	for remaining := count; remaining > 0; {
		n := min(batchSize, remaining)
		seqs := make([]uint64, n)
		for i := range n {
			seqs[i] = seq
			seq++
		}
		batches = append(batches, FundingBatch{Sequences: seqs})
		remaining -= n
	}
	return batches
}
