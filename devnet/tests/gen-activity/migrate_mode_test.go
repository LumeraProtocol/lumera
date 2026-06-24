package main

import (
	"testing"

	"gen/tests/common"
)

func TestMigrationPreflight(t *testing.T) {
	t.Run("enabled passes", func(t *testing.T) {
		if err := migrationPreflightAt(common.MigrationParams{EnableMigration: true, MaxMigrationsPerBlock: 50}, 1000); err != nil {
			t.Errorf("unexpected preflight error: %v", err)
		}
	})
	t.Run("disabled migration fails", func(t *testing.T) {
		if err := migrationPreflightAt(common.MigrationParams{EnableMigration: false}, 1000); err == nil {
			t.Error("expected preflight error when migration disabled")
		}
	})
	t.Run("closed migration window fails", func(t *testing.T) {
		err := migrationPreflightAt(common.MigrationParams{EnableMigration: true, MigrationEndTime: 999}, 1000)
		if err == nil {
			t.Error("expected preflight error when migration window is closed")
		}
	})
	t.Run("open migration window passes", func(t *testing.T) {
		err := migrationPreflightAt(common.MigrationParams{EnableMigration: true, MigrationEndTime: 1001}, 1000)
		if err != nil {
			t.Errorf("unexpected preflight error: %v", err)
		}
	})
}
