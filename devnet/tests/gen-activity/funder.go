package main

import (
	"fmt"
	"log"

	"gen/tests/common"
)

// fundingChain is the subset of chain operations the funder batcher needs. It
// is an interface so the batching/retry logic is testable against a fake.
type fundingChain interface {
	// FunderAccountSequence returns the funder's current account number and
	// sequence, queried fresh at the start of each attempt.
	FunderAccountSequence() (accNum, seq uint64, err error)
	// SendFromFunder signs and broadcasts a bank send from the funder to addr
	// using the explicit account number and sequence, without waiting for
	// inclusion (burst broadcast).
	SendFromFunder(accNum, seq uint64, toAddr, amount string) error
	// WaitForNextBlock blocks until the chain advances one block, the barrier
	// between batches.
	WaitForNextBlock() error
	// IsFunded reports whether the address now holds a balance.
	IsFunded(addr string) (bool, error)
}

// FundAccounts funds the given accounts from a single funder signer. Because one
// account signs every transfer, sequence assignment is centralized: each attempt
// queries the funder's current sequence, plans contiguous sequence numbers
// (common.PlanFundingBatches), bursts each batch, waits a block, then confirms
// which accounts funded. Unconfirmed accounts are retried with a refreshed
// sequence up to maxAttempts. It returns the number of accounts confirmed funded.
func FundAccounts(chain fundingChain, targets []*AccountRecord, amountFor func(*AccountRecord) string, batchSize, maxAttempts int) (int, error) {
	remaining := make([]*AccountRecord, 0, len(targets))
	for _, t := range targets {
		if !t.Funded {
			remaining = append(remaining, t)
		}
	}
	funded := len(targets) - len(remaining)
	if len(remaining) == 0 {
		log.Printf("funding: no unfunded bank account(s)")
		return funded, nil
	}
	log.Printf("funding: starting %d bank account(s), already funded %d, batch-size %d, max-attempts %d", len(remaining), funded, batchSize, maxAttempts)

	for attempt := 1; attempt <= maxAttempts && len(remaining) > 0; attempt++ {
		accNum, startSeq, err := chain.FunderAccountSequence()
		if err != nil {
			return funded, fmt.Errorf("query funder sequence: %w", err)
		}

		// Burst each planned batch, then wait a block before the next batch.
		batches := common.PlanFundingBatches(startSeq, len(remaining), batchSize)
		log.Printf("funding: attempt %d/%d with %d remaining account(s), account-number %d, start-sequence %d, %d batch(es)",
			attempt, maxAttempts, len(remaining), accNum, startSeq, len(batches))
		idx := 0
		for batchIdx, batch := range batches {
			batchStart := idx + 1
			log.Printf("funding: attempt %d batch %d/%d submitting %d transfer(s) (remaining positions %d-%d)",
				attempt, batchIdx+1, len(batches), len(batch.Sequences), batchStart, batchStart+len(batch.Sequences)-1)
			for _, seq := range batch.Sequences {
				rec := remaining[idx]
				idx++
				if sendErr := chain.SendFromFunder(accNum, seq, rec.Address, amountFor(rec)); sendErr != nil {
					// A burst send error (often a sequence mismatch) is not fatal:
					// confirmation below is the source of truth, and unfunded
					// accounts are retried next attempt.
					log.Printf("  WARN: funding send to %s failed (seq %d): %v", rec.Name, seq, sendErr)
				}
			}
			if waitErr := chain.WaitForNextBlock(); waitErr != nil {
				log.Printf("  WARN: wait for next block after funding batch: %v", waitErr)
			}
			log.Printf("funding: attempt %d batch %d/%d submitted; checking next batch after block wait", attempt, batchIdx+1, len(batches))
		}

		// Confirm and collect the accounts that still need funding.
		log.Printf("funding: confirming %d account(s) after attempt %d", len(remaining), attempt)
		still := remaining[:0:0]
		attemptFunded := 0
		for _, rec := range remaining {
			ok, confErr := chain.IsFunded(rec.Address)
			if confErr != nil {
				log.Printf("  WARN: confirm funding for %s: %v", rec.Name, confErr)
			}
			if ok {
				rec.Funded = true
				rec.HasBalance = true
				funded++
				attemptFunded++
			} else {
				still = append(still, rec)
			}
		}
		log.Printf("funding: attempt %d confirmed %d newly funded account(s); total funded %d/%d",
			attempt, attemptFunded, funded, len(targets))
		remaining = still
		if len(remaining) > 0 {
			log.Printf("  INFO: %d account(s) unfunded after attempt %d/%d", len(remaining), attempt, maxAttempts)
		}
	}

	if len(remaining) > 0 {
		return funded, fmt.Errorf("%d account(s) still unfunded after %d attempts", len(remaining), maxAttempts)
	}
	return funded, nil
}
