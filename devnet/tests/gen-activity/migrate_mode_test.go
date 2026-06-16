package main

import (
	"testing"

	"gen/tests/common"
)

func TestMigrationPreflight(t *testing.T) {
	t.Run("enabled passes", func(t *testing.T) {
		if err := migrationPreflight(common.MigrationParams{EnableMigration: true, MaxMigrationsPerBlock: 50}); err != nil {
			t.Errorf("unexpected preflight error: %v", err)
		}
	})
	t.Run("disabled migration fails", func(t *testing.T) {
		if err := migrationPreflight(common.MigrationParams{EnableMigration: false}); err == nil {
			t.Error("expected preflight error when migration disabled")
		}
	})
}
