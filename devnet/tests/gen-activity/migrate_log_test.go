package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixedClock() func() time.Time {
	ts := time.Date(2026, 6, 16, 18, 25, 4, 0, time.UTC)
	return func() time.Time { return ts }
}

func TestMigrationLoggerFormatsCorrelatedLine(t *testing.T) {
	var buf bytes.Buffer
	lg := newMigrationLogger(&buf)
	lg.now = fixedClock()

	lg.logf("migrate gen-0001 #1", "submitting claim tx %s", "ABC123")

	line := strings.TrimSpace(buf.String())
	for _, want := range []string{"2026-06-16T18:25:04Z", "[migrate gen-0001 #1]", "submitting claim tx ABC123"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line missing %q:\n%s", want, line)
		}
	}
}

func TestMigrationLoggerConcurrentLinesAreWhole(t *testing.T) {
	var buf bytes.Buffer
	lg := newMigrationLogger(&buf)
	lg.now = fixedClock()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			lg.logf("worker", "line number %d with several words to widen the write", n)
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 50 {
		t.Fatalf("got %d lines, want 50 (interleaving/corruption)", len(lines))
	}
	for _, ln := range lines {
		if !strings.Contains(ln, "[worker]") || !strings.Contains(ln, "line number ") {
			t.Errorf("corrupted/interleaved line: %q", ln)
		}
	}
}
