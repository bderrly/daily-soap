package cache_expunger

import (
	"database/sql"
	"log/slog"
	"time"
)

// Start initializes the cache expunger service.
// It runs an initial expunge immediately in a background goroutine and then schedules
// it to run every 24 hours.
func Start(db *sql.DB) {
	go func() {
		slog.Debug("starting initial cache expunge")
		if err := Expunge(db); err != nil {
			slog.Error("failed to expunge cache", "error", err)
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			slog.Debug("starting scheduled cache expunge")
			if err := Expunge(db); err != nil {
				slog.Error("failed to expunge cache", "error", err)
			}
		}
	}()
}

// Expunge removes old and excess entries from the esv_cache table.
// It enforces two rules:
// 1. Remove entries older than 28 days.
// 2. Keep at most 500 entries (removing oldest first).
func Expunge(db *sql.DB) error {
	// 1. Time-based purge: remove entries older than 28 days
	// SQLite 'now' is in UTC by default, make sure we use the same consistent time handling
	// The table defaults created_at to CURRENT_TIMESTAMP which is UTC.
	_, err := db.Exec("DELETE FROM esv_cache WHERE created_at < datetime('now', '-28 days')")
	if err != nil {
		return err
	}

	// 2. Count-based purge: ensure not more than 500 entries
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM esv_cache").Scan(&count)
	if err != nil {
		return err
	}

	if count > 500 {
		limit := count - 500
		// Delete the 'limit' oldest records
		// We identify them by selecting the oldest ones first (ORDER BY created_at ASC)
		query := `
			DELETE FROM esv_cache 
			WHERE reference IN (
				SELECT reference 
				FROM esv_cache 
				ORDER BY created_at ASC 
				LIMIT ?
			)
		`
		_, err = db.Exec(query, limit)
		if err != nil {
			return err
		}
		slog.Info("expunged excess cache entries", "removed_count", limit)
	}

	return nil
}
