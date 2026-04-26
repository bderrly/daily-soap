package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"derrclan.com/moravian-soap/internal/migrations"
	_ "github.com/mattn/go-sqlite3"
)

func TestRun(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify that queued_emails table exists
	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='queued_emails'").Scan(&name)
	if err != nil {
		t.Errorf("failed to find queued_emails table: %v", err)
	}
	if name != "queued_emails" {
		t.Errorf("expected queued_emails table, got %s", name)
	}

	// Verify index exists
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_queued_emails_status_next_attempt'").Scan(&name)
	if err != nil {
		t.Errorf("failed to find index: %v", err)
	}
}
