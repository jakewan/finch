package core

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// migrateTo applies migrations up to and including the given version, leaving the
// database in the state a prior release would have left it. This lets a migration be
// exercised against pre-existing data rather than only against a fresh schema.
func migrateTo(t *testing.T, conn *sql.DB, maxVersion int) {
	t.Helper()

	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL UNIQUE)`); err != nil {
		t.Fatalf("create schema_version: %v", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, entry := range entries {
		version, err := parseMigrationVersion(entry.Name())
		if err != nil {
			t.Fatalf("parse migration version %s: %v", entry.Name(), err)
		}
		if version > maxVersion {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		if _, err := conn.Exec(string(data)); err != nil {
			t.Fatalf("exec migration %s: %v", entry.Name(), err)
		}
		if _, err := conn.Exec("INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
			t.Fatalf("record version %d: %v", version, err)
		}
	}
}

// TestMigration004BackfillsTransfersFromEventLog exercises the migration against a
// database that already holds transfers recorded before the is_transfer column
// existed. The read model has to stay derivable from the event log, so the backfill
// reads TransferCreated rather than guessing from amounts or names.
func TestMigration004BackfillsTransfersFromEventLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pre004.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Bring the database to the schema that shipped before this change.
	migrateTo(t, conn, 3)

	now := time.Now().Unix()
	for _, a := range []struct{ id, name string }{{"acct-checking", "Checking"}, {"acct-savings", "Savings"}} {
		if _, err := conn.Exec(
			`INSERT INTO accounts (id, name, type, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
			a.id, a.name, now, now); err != nil {
			t.Fatalf("insert account: %v", err)
		}
	}

	// A transfer's two legs, plus an ordinary expense that must stay unmarked.
	for _, txn := range []struct {
		id, account string
		amount      int64
		name        string
	}{
		{"txn-source", "acct-checking", -100000, "Transfer to Savings"},
		{"txn-dest", "acct-savings", 100000, "Transfer to Savings"},
		{"txn-rent", "acct-checking", -150000, "Rent"},
	} {
		if _, err := conn.Exec(
			`INSERT INTO transactions (id, account_id, date, amount, name, status)
			 VALUES (?, ?, '2025-02-15', ?, ?, 3)`,
			txn.id, txn.account, txn.amount, txn.name); err != nil {
			t.Fatalf("insert transaction: %v", err)
		}
	}

	payload, err := json.Marshal(TransferCreatedPayload{
		SourceAccountID:      "acct-checking",
		DestinationAccountID: "acct-savings",
		Amount:               100000,
		Date:                 "2025-02-15",
		SourceTransactionID:  "txn-source",
		DestTransactionID:    "txn-dest",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO events (aggregate_type, aggregate_id, event_type, payload, recorded_at, sequence)
		 VALUES ('transaction', 'txn-source', 'TransferCreated', ?, ?, 2)`,
		string(payload), now); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopening runs the pending migration against that existing data.
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open (migrating existing data): %v", err)
	}
	defer func() { _ = db.Close() }()

	marked := map[string]bool{}
	rows, err := db.conn.Query("SELECT id, is_transfer FROM transactions")
	if err != nil {
		t.Fatalf("query transactions: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		var isTransfer int
		if err := rows.Scan(&id, &isTransfer); err != nil {
			t.Fatalf("scan: %v", err)
		}
		marked[id] = isTransfer != 0
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	for id, want := range map[string]bool{"txn-source": true, "txn-dest": true, "txn-rent": false} {
		if marked[id] != want {
			t.Errorf("transaction %s is_transfer = %v, want %v", id, marked[id], want)
		}
	}
}

// TestMigration004PreservesRecurringRulesAcrossRebuild guards the table rebuild that
// adds the missing foreign key: rows must survive it intact.
func TestMigration004PreservesRecurringRulesAcrossRebuild(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pre004rules.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	migrateTo(t, conn, 3)

	now := time.Now().Unix()
	for _, a := range []struct{ id, name string }{{"acct-checking", "Checking"}, {"acct-savings", "Savings"}} {
		if _, err := conn.Exec(
			`INSERT INTO accounts (id, name, type, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
			a.id, a.name, now, now); err != nil {
			t.Fatalf("insert account: %v", err)
		}
	}
	if _, err := conn.Exec(
		`INSERT INTO recurring_rules (id, account_id, name, amount, frequency, start_date,
		 day_of_month, is_transfer, transfer_target_account_id, paused)
		 VALUES ('rule-1', 'acct-checking', 'To Savings', 100000, 4, '2025-01-01', 1, 1, 'acct-savings', 0)`,
	); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open (migrating existing data): %v", err)
	}
	defer func() { _ = db.Close() }()

	rules, err := db.ListRecurringRules(t.Context(), "acct-checking")
	if err != nil {
		t.Fatalf("ListRecurringRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules after rebuild, want 1", len(rules))
	}
	if !rules[0].IsTransfer || rules[0].TransferTargetAccountID != "acct-savings" || rules[0].Amount != 100000 {
		t.Errorf("rule not preserved across rebuild: %+v", rules[0])
	}

	// The new constraint must actually be enforced, not merely declared.
	_, err = db.conn.ExecContext(t.Context(),
		`INSERT INTO recurring_rules (id, account_id, name, amount, frequency, start_date,
		 day_of_month, is_transfer, transfer_target_account_id, paused)
		 VALUES ('rule-2', 'acct-checking', 'Dangling', 100000, 4, '2025-01-01', 1, 1, 'no-such-account', 0)`)
	if err == nil {
		t.Error("insert with a dangling transfer target succeeded; the foreign key is not enforced")
	}
}
