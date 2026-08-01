// Package main provides a script to generate a user for local development, bypassing email verification.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"

	"github.com/bderrly/daily-soap/internal/auth"
	"github.com/bderrly/daily-soap/internal/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("failed to generate dev user", "error", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	flag.CommandLine.Init(os.Args[0], flag.ContinueOnError)

	emailFlag := flag.String("email", "dev@example.com", "User email address")
	passwordFlag := flag.String("password", "password123", "User password")
	timezoneFlag := flag.String("timezone", "UTC", "User timezone")
	adminFlag := flag.Bool("admin", false, "Grant admin privileges to the user")
	helpFlag := flag.Bool("help", false, "Show usage help")

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		return nil
	}

	if *helpFlag {
		flag.Usage()
		return nil
	}

	email := strings.TrimSpace(strings.ToLower(*emailFlag))
	password := *passwordFlag
	timezone := strings.TrimSpace(*timezoneFlag)

	if email == "" || password == "" {
		return fmt.Errorf("email and password cannot be empty")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/journal.db"
	}

	// Ensure parent directory exists for local DB if using file path
	if dir := filepath.Clean(filepath.Dir(dbPath)); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("creating database directory %q: %w", dir, err)
		}
	}

	u, err := url.Parse(dbPath)
	if err != nil {
		return fmt.Errorf("parsing database path %q: %w", dbPath, err)
	}

	q := u.Query()
	q.Set("_foreign_keys", "on")
	u.RawQuery = q.Encode()

	ctx := context.Background()

	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := migrations.Run(ctx, db); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	isAdminInt := 0
	if *adminFlag {
		isAdminInt = 1
	}

	var existingID int64
	err = db.QueryRowContext(ctx, "SELECT id FROM users WHERE LOWER(email) = ?", email).Scan(&existingID)
	if err == nil {
		query := `
			UPDATE users
			SET password_hash = ?,
				verification_token = NULL,
				verified_at = COALESCE(verified_at, CURRENT_TIMESTAMP),
				timezone = ?,
				is_admin = CASE WHEN ? = 1 THEN 1 ELSE is_admin END
			WHERE id = ?`
		_, err = db.ExecContext(ctx, query, hashedPassword, timezone, isAdminInt, existingID)
		if err != nil {
			return fmt.Errorf("updating dev user %q: %w", email, err)
		}
		slog.Info("updated existing dev user", "email", email, "id", existingID, "admin", *adminFlag)
		return nil
	}

	insertQuery := `
		INSERT INTO users (email, password_hash, verification_token, timezone, is_admin, verified_at)
		VALUES (?, ?, NULL, ?, ?, CURRENT_TIMESTAMP)`
	res, err := db.ExecContext(ctx, insertQuery, email, hashedPassword, timezone, isAdminInt)
	if err != nil {
		return fmt.Errorf("creating dev user %q: %w", email, err)
	}

	id, _ := res.LastInsertId()
	slog.Info("successfully created dev user", "email", email, "id", id, "admin", *adminFlag)
	return nil
}
