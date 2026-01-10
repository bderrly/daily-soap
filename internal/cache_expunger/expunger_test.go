package cache_expunger

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	query := `
	CREATE TABLE esv_cache (
		reference TEXT PRIMARY KEY,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	return db
}

func TestExpunge_TimeLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert an old record (30 days ago)
	_, err := db.Exec(`INSERT INTO esv_cache (reference, content, created_at) VALUES ('old', 'content', datetime('now', '-30 days'))`)
	if err != nil {
		t.Fatalf("failed to insert old record: %v", err)
	}

	// Insert a new record (1 day ago)
	_, err = db.Exec(`INSERT INTO esv_cache (reference, content, created_at) VALUES ('new', 'content', datetime('now', '-1 days'))`)
	if err != nil {
		t.Fatalf("failed to insert new record: %v", err)
	}

	// Run Expunge
	if err := Expunge(db); err != nil {
		t.Fatalf("Expunge failed: %v", err)
	}

	// Verify old record is gone
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM esv_cache WHERE reference = 'old'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query count: %v", err)
	}
	if count != 0 {
		t.Errorf("old record should have been deleted")
	}

	// Verify new record is present
	err = db.QueryRow("SELECT COUNT(*) FROM esv_cache WHERE reference = 'new'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query count: %v", err)
	}
	if count != 1 {
		t.Errorf("new record should have been preserved")
	}
}

func TestExpunge_CountLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert 510 records
	// Make them have different timestamps so we can predict which ones get deleted
	// 10 oldest records (from 10 days ago)
	for i := 0; i < 10; i++ {
		_, err := db.Exec(fmt.Sprintf(`INSERT INTO esv_cache (reference, content, created_at) VALUES ('old_%d', 'content', datetime('now', '-10 days', '+%d seconds'))`, i, i))
		if err != nil {
			t.Fatalf("failed to insert record: %v", err)
		}
	}

	// 500 newer records (from 1 day ago)
	for i := 0; i < 500; i++ {
		_, err := db.Exec(fmt.Sprintf(`INSERT INTO esv_cache (reference, content, created_at) VALUES ('new_%d', 'content', datetime('now', '-1 days', '+%d seconds'))`, i, i))
		if err != nil {
			t.Fatalf("failed to insert record: %v", err)
		}
	}

	// Run Expunge
	if err := Expunge(db); err != nil {
		t.Fatalf("Expunge failed: %v", err)
	}

	// Should be exactly 500 records left
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM esv_cache").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query total count: %v", err)
	}
	if count != 500 {
		t.Errorf("expected 500 records, got %d", count)
	}

	// The 10 "old_*" records should be gone because they were the oldest
	// and we needed to remove 10 to get back to 500 (510 - 10 = 500)
	err = db.QueryRow("SELECT COUNT(*) FROM esv_cache WHERE reference LIKE 'old_%'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query old records count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected all 10 old records to be deleted, found %d", count)
	}
}
