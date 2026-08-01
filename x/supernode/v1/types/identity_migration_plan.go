package types

import (
	"bytes"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// IdentityMigrationPlan is an opaque, immutable snapshot of the SuperNode
// continuity writes validated by the keeper. The unexported method seals the
// interface so callers outside this package cannot forge implementations.
type IdentityMigrationPlan interface {
	Preconditions() []IdentityMigrationRow
	PrefixPreconditions() []IdentityMigrationPrefix
	Writes() []IdentityMigrationWrite
	identityMigrationPlan()
}

// IdentityMigrationRow is a read-only copy of one exact key/value precondition.
// A nil Value means the key must be absent.
type IdentityMigrationRow struct {
	Key   []byte
	Value []byte
}

// IdentityMigrationPrefix is a read-only copy of a complete bounded prefix
// snapshot. Rows are in store iteration order.
type IdentityMigrationPrefix struct {
	Prefix []byte
	Rows   []IdentityMigrationRow
}

// IdentityMigrationWrite is one set or delete operation. A nil Value denotes a
// delete; continuity state never stores nil values.
type IdentityMigrationWrite struct {
	Key   []byte
	Value []byte
}

type identityMigrationPlan struct {
	preconditions       []IdentityMigrationRow
	prefixPreconditions []IdentityMigrationPrefix
	writes              []IdentityMigrationWrite
}

// NewIdentityMigrationPlan constructs the only supported continuity operation:
// moving source metrics and/or distribution state to an empty destination. It
// derives every key itself and takes deep copies of all supplied state, so the
// public constructor cannot be used to forge arbitrary module writes.
func NewIdentityMigrationPlan(
	sourceValidator, destinationValidator sdk.ValAddress,
	sourceMetrics, destinationMetrics, movedMetrics []byte,
	rdistRows []IdentityMigrationRow,
	sourceDist []byte,
) IdentityMigrationPlan {
	preconditions := []IdentityMigrationRow{
		{Key: GetMetricsStateKey(sourceValidator), Value: sourceMetrics},
		{Key: GetMetricsStateKey(destinationValidator), Value: destinationMetrics},
	}
	prefixPreconditions := []IdentityMigrationPrefix{{Prefix: SNDistStatePrefix, Rows: rdistRows}}
	writes := make([]IdentityMigrationWrite, 0, 4)
	if sourceMetrics != nil {
		writes = append(writes,
			IdentityMigrationWrite{Key: GetMetricsStateKey(sourceValidator)},
			IdentityMigrationWrite{Key: GetMetricsStateKey(destinationValidator), Value: movedMetrics},
		)
	}
	if sourceDist != nil {
		writes = append(writes,
			IdentityMigrationWrite{Key: SNDistStateKey(sourceValidator.String())},
			IdentityMigrationWrite{Key: SNDistStateKey(destinationValidator.String()), Value: sourceDist},
		)
	}
	return &identityMigrationPlan{
		preconditions:       cloneMigrationRows(preconditions),
		prefixPreconditions: cloneMigrationPrefixes(prefixPreconditions),
		writes:              cloneMigrationWrites(writes),
	}
}

func (*identityMigrationPlan) identityMigrationPlan() {}

func (p *identityMigrationPlan) Preconditions() []IdentityMigrationRow {
	if p == nil {
		return nil
	}
	return cloneMigrationRows(p.preconditions)
}

func (p *identityMigrationPlan) PrefixPreconditions() []IdentityMigrationPrefix {
	if p == nil {
		return nil
	}
	return cloneMigrationPrefixes(p.prefixPreconditions)
}

func (p *identityMigrationPlan) Writes() []IdentityMigrationWrite {
	if p == nil {
		return nil
	}
	return cloneMigrationWrites(p.writes)
}

func cloneMigrationRows(rows []IdentityMigrationRow) []IdentityMigrationRow {
	if rows == nil {
		return nil
	}
	out := make([]IdentityMigrationRow, len(rows))
	for i, row := range rows {
		out[i] = IdentityMigrationRow{Key: bytes.Clone(row.Key), Value: bytes.Clone(row.Value)}
	}
	return out
}

func cloneMigrationPrefixes(prefixes []IdentityMigrationPrefix) []IdentityMigrationPrefix {
	if prefixes == nil {
		return nil
	}
	out := make([]IdentityMigrationPrefix, len(prefixes))
	for i, snapshot := range prefixes {
		out[i] = IdentityMigrationPrefix{
			Prefix: bytes.Clone(snapshot.Prefix),
			Rows:   cloneMigrationRows(snapshot.Rows),
		}
	}
	return out
}

func cloneMigrationWrites(writes []IdentityMigrationWrite) []IdentityMigrationWrite {
	if writes == nil {
		return nil
	}
	out := make([]IdentityMigrationWrite, len(writes))
	for i, write := range writes {
		out[i] = IdentityMigrationWrite{Key: bytes.Clone(write.Key), Value: bytes.Clone(write.Value)}
	}
	return out
}
