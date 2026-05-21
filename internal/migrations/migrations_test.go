package migrations_test

import (
	"context"
	"database/sql"
	"embed"
	"testing"
	"time"

	"github.com/bderrly/daily-soap/internal/migrations"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var testEmbedMigrations embed.FS

func TestRun(t *testing.T) {
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_foreign_keys=on")
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

	// Verify users table schema changes
	rows, err := db.Query("PRAGMA table_info(users)")
	if err != nil {
		t.Fatalf("failed to query users table info: %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	type colInfo struct {
		name    string
		dflt    *string
		notnull int
	}
	cols := make(map[string]colInfo)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltVal *string
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltVal, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		cols[name] = colInfo{name: name, dflt: dfltVal, notnull: notnull}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}

	// Verify is_admin column
	if info, ok := cols["is_admin"]; !ok {
		t.Error("missing is_admin column in users table")
	} else if info.notnull != 1 {
		t.Error("expected is_admin to be NOT NULL")
	} else if info.dflt == nil || *info.dflt != "0" {
		t.Errorf("expected is_admin default '0', got %v", info.dflt)
	}

	// Verify created_at column
	if info, ok := cols["created_at"]; !ok {
		t.Error("missing created_at column in users table")
	} else if info.notnull != 1 {
		t.Error("expected created_at to be NOT NULL")
	} else if info.dflt == nil || *info.dflt != "CURRENT_TIMESTAMP" {
		t.Errorf("expected created_at default 'CURRENT_TIMESTAMP', got %v", info.dflt)
	}

	// Verify verified_at column
	if info, ok := cols["verified_at"]; !ok {
		t.Error("missing verified_at column in users table")
	} else if info.notnull != 0 {
		t.Error("expected verified_at to be nullable")
	}
}

func TestBackfill(t *testing.T) {
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_foreign_keys=on")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	goose.SetBaseFS(testEmbedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("failed to set dialect: %v", err)
	}

	// Run up to the previous migration
	if err := goose.UpToContext(ctx, db, ".", 20260425000000); err != nil {
		t.Fatalf("failed to migrate to 20260425000000: %v", err)
	}

	// Insert a verified user and an unverified user
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, is_verified, verification_token, timezone)
		VALUES
			(1, 'verified@example.com', 'hash', 1, 'token1', 'UTC'),
			(2, 'unverified@example.com', 'hash', 0, 'token2', 'UTC')
	`)
	if err != nil {
		t.Fatalf("failed to insert test users: %v", err)
	}

	// Run remaining migrations
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("failed to finish migrations: %v", err)
	}

	// Verify backfill logic for verified user
	var verifiedCreatedAt, verifiedVerifiedAt time.Time
	var verifiedIsAdmin int
	err = db.QueryRowContext(ctx, "SELECT created_at, verified_at, is_admin FROM users WHERE id = 1").Scan(&verifiedCreatedAt, &verifiedVerifiedAt, &verifiedIsAdmin)
	if err != nil {
		t.Fatalf("failed to query verified user: %v", err)
	}
	if verifiedIsAdmin != 0 {
		t.Errorf("expected is_admin default 0, got %d", verifiedIsAdmin)
	}
	if verifiedVerifiedAt.IsZero() {
		t.Error("expected verified_at to be populated for verified user")
	}
	if !verifiedVerifiedAt.Equal(verifiedCreatedAt) {
		t.Errorf("expected verified_at (%v) to equal created_at (%v)", verifiedVerifiedAt, verifiedCreatedAt)
	}

	// Verify unverified user has NULL verified_at
	var unverifiedCreatedAt time.Time
	var unverifiedVerifiedAt *time.Time
	var unverifiedIsAdmin int
	err = db.QueryRowContext(ctx, "SELECT created_at, verified_at, is_admin FROM users WHERE id = 2").Scan(&unverifiedCreatedAt, &unverifiedVerifiedAt, &unverifiedIsAdmin)
	if err != nil {
		t.Fatalf("failed to query unverified user: %v", err)
	}
	if unverifiedIsAdmin != 0 {
		t.Errorf("expected is_admin default 0, got %d", unverifiedIsAdmin)
	}
	if unverifiedVerifiedAt != nil {
		t.Errorf("expected verified_at to be NULL for unverified user, got %v", *unverifiedVerifiedAt)
	}
}
