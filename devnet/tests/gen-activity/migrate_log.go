package main

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// migrationLogger writes correlated, timestamped, single-line log records for
// migrate mode. Every line is emitted under a mutex so concurrent workers never
// interleave mid-line, and each line carries a correlation id (the per-account
// work tag) so an interleaved run is still traceable.
type migrationLogger struct {
	mu  sync.Mutex
	out io.Writer
	now func() time.Time
}

func newMigrationLogger(out io.Writer) *migrationLogger {
	return &migrationLogger{out: out, now: time.Now}
}

// logf writes one whole line: "<RFC3339 UTC ts> [<corrID>] <message>".
func (l *migrationLogger) logf(corrID, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := l.now().UTC().Format(time.RFC3339)
	line := fmt.Sprintf("%s [%s] %s\n", ts, corrID, msg)
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.out, line)
}
