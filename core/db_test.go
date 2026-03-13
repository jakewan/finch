package core_test

import (
	"path/filepath"
	"testing"

	"github.com/jakewan/finch/core"
)

func openTestDB(t *testing.T) *core.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := core.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := openTestDB(t)
	_ = db
}

func TestMigrateIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db1, err := core.Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	_ = db1.Close()

	// Opening again should not fail — migrations already applied.
	db2, err := core.Open(dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	_ = db2.Close()
}
