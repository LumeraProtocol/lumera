// Package common holds devnet-test primitives shared between the migration
// tooling in tests/evmigration and the activity generator in tests/gen-activity.
//
// It owns the stable inner layer described in the gen-activity design: account
// identity types, per-account activity record types, activity tracking with
// deduplication, key-style detection, coin parsing, and registry load/save.
// Tool-specific top-level registry envelopes stay in their respective packages.
package common
