// Package sqlite provides a SQLite implementation of the store.Store interface.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"derrclan.com/moravian-soap/internal/store"
)

// Store implements the store.Store interface using SQLite.
type Store struct {
	db *sql.DB
}

// New creates a new SQLite store.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// GetUserFromSession retrieves a user associated with a given session token.
func (s *Store) GetUserFromSession(ctx context.Context, token string) (*store.User, error) {
	var user store.User
	var expiresAt time.Time

	query := `
		SELECT u.id, u.email, u.is_verified, u.timezone, s.expires_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ?`

	err := s.db.QueryRowContext(ctx, query, token).Scan(&user.ID, &user.Email, &user.IsVerified, &user.Timezone, &expiresAt)
	if err != nil {
		return nil, fmt.Errorf("getting user from session: %w", err)
	}

	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	return &user, nil
}

// GetSOAPData retrieves SOAP data from the database for a given user and date.
func (s *Store) GetSOAPData(ctx context.Context, userID int64, dateStr string) (*store.SOAPData, error) {
	var soapData store.SOAPData
	var selectedVersesJSON sql.NullString
	soapData.Date = dateStr

	query := `SELECT observation, application, prayer, selected_verses FROM journal WHERE user_id = ? AND date = ?`
	err := s.db.QueryRowContext(ctx, query, userID, dateStr).Scan(&soapData.Observation, &soapData.Application, &soapData.Prayer, &selectedVersesJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			soapData.SelectedVerses = []string{}
			return &soapData, nil
		}
		return nil, fmt.Errorf("retrieving SOAP journal data: %w", err)
	}

	if selectedVersesJSON.Valid && selectedVersesJSON.String != "" {
		if err := json.Unmarshal([]byte(selectedVersesJSON.String), &soapData.SelectedVerses); err != nil {
			slog.Error("failed to unmarshal (JSON) selected verses", "error", err, "userID", userID, "verses", selectedVersesJSON.String)
			soapData.SelectedVerses = []string{}
		}
	} else {
		soapData.SelectedVerses = []string{}
	}
	return &soapData, nil
}

// SaveSOAPData saves SOAP data to the database.
func (s *Store) SaveSOAPData(ctx context.Context, userID int64, soapData *store.SOAPData) error {
	selectedVersesJSON, err := json.Marshal(soapData.SelectedVerses)
	if err != nil {
		return fmt.Errorf("JSON marshaling selected verses: %w", err)
	}

	query := `
		INSERT INTO journal (user_id, date, observation, application, prayer, selected_verses)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, date) DO UPDATE SET
			observation = excluded.observation,
			application = excluded.application,
			prayer = excluded.prayer,
			selected_verses = excluded.selected_verses,
			timestamp = CURRENT_TIMESTAMP
	`
	_, err = s.db.ExecContext(ctx, query, userID, soapData.Date, soapData.Observation, soapData.Application, soapData.Prayer, selectedVersesJSON)
	if err != nil {
		return fmt.Errorf("saving SOAP data: %w", err)
	}
	return nil
}

// CreateUser inserts a new user into the database.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash, token, timezone string) error {
	if timezone == "" {
		timezone = "UTC"
	}

	_, err := s.db.ExecContext(ctx, "INSERT INTO users (email, password_hash, is_verified, verification_token, timezone) VALUES (?, ?, 0, ?, ?)", email, passwordHash, token, timezone)
	if err != nil {
		return fmt.Errorf("inserting user %q: %w", email, err)
	}
	return nil
}

// UpdateUserTimezone updates a user's timezone.
func (s *Store) UpdateUserTimezone(ctx context.Context, userID int64, timezone string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET timezone = ? WHERE id = ?", timezone, userID)
	if err != nil {
		return fmt.Errorf("updating user timezone: %w", err)
	}
	return nil
}

// ConfirmUser verifies a user by token.
func (s *Store) ConfirmUser(ctx context.Context, token string) (int64, error) {
	result, err := s.db.ExecContext(ctx, "UPDATE users SET is_verified = 1, verification_token = NULL WHERE verification_token = ?", token)
	if err != nil {
		return 0, fmt.Errorf("verifying user: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("getting rows affected: %w", err)
	}
	return n, nil
}

// GetUserByEmail retrieves a user by their email.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*store.User, error) {
	var user store.User
	err := s.db.QueryRowContext(ctx, "SELECT id, email, is_verified, timezone FROM users WHERE email = ?", email).Scan(&user.ID, &user.Email, &user.IsVerified, &user.Timezone)
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return &user, nil
}

// CreatePasswordResetToken saves a password reset token.
func (s *Store) CreatePasswordResetToken(ctx context.Context, token string, userID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("saving reset token: %w", err)
	}
	return nil
}

// GetPasswordResetToken retrieves a password reset token's information.
func (s *Store) GetPasswordResetToken(ctx context.Context, token string) (userID int64, expiresAt time.Time, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT user_id, expires_at FROM password_reset_tokens WHERE token = ?", token).Scan(&userID, &expiresAt)
	if err != nil {
		return userID, expiresAt, fmt.Errorf("getting reset token: %w", err)
	}
	return
}

// UpdateUserPassword updates a user's password hash.
func (s *Store) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	return nil
}

// DeletePasswordResetToken deletes a password reset token.
func (s *Store) DeletePasswordResetToken(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM password_reset_tokens WHERE token = ?", token)
	if err != nil {
		return fmt.Errorf("deleting reset token: %w", err)
	}
	return nil
}

// GetAuthUser retrieves authentication-related information for a user.
func (s *Store) GetAuthUser(ctx context.Context, email string) (userID int64, passwordHash string, isVerified bool, timezone string, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT id, password_hash, is_verified, timezone FROM users WHERE email = ?", email).Scan(&userID, &passwordHash, &isVerified, &timezone)
	if err != nil {
		return 0, "", false, "", fmt.Errorf("getting auth user: %w", err)
	}
	return
}

// UpdateUserPasswordHash updates a user's password hash for migration.
func (s *Store) UpdateUserPasswordHash(ctx context.Context, userID int64, newHash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", newHash, userID)
	if err != nil {
		return fmt.Errorf("updating user %d password hash: %w", userID, err)
	}
	return nil
}

// CreateSession creates a new session token.
func (s *Store) CreateSession(ctx context.Context, token string, userID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("saving session for user %d: %w", userID, err)
	}
	return nil
}

// DeleteExpiredSessions removes expired session tokens.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", time.Now())
	if err != nil {
		return fmt.Errorf("cleaning up expired sessions: %w", err)
	}
	return nil
}

// ExpungeCache removes old and excess entries from the esv_cache table.
func (s *Store) ExpungeCache(ctx context.Context, olderThan time.Duration, keepMax int) error {
	// The terms of use for api.esv.org requires keeping no more than 500 passages and for none for longer than 30 days.

	// Time-based purge
	cutoff := time.Now().Add(-olderThan)
	_, err := s.db.ExecContext(ctx, "DELETE FROM esv_cache WHERE created_at < ?", cutoff)
	if err != nil {
		return fmt.Errorf("purging old ESV cache entries: %w", err)
	}

	// Count-based purge
	var count int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM esv_cache").Scan(&count)
	if err != nil {
		return fmt.Errorf("counting ESV cache entries: %w", err)
	}

	if count > keepMax {
		limit := count - keepMax
		query := `
			DELETE FROM esv_cache
			WHERE reference IN (
				SELECT reference
				FROM esv_cache
				ORDER BY created_at ASC
				LIMIT ?
			)
		`
		_, err = s.db.ExecContext(ctx, query, limit)
		if err != nil {
			return fmt.Errorf("expunging %d excess ESV cache entries: %w", limit, err)
		}
		slog.Info("expunged excess ESV cache entries", "removed_count", limit)
	}
	return nil
}

// GetCachedESV retrieves a cached ESV response.
func (s *Store) GetCachedESV(ctx context.Context, key string) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx, "SELECT content FROM esv_cache WHERE reference = ?", key).Scan(&content)
	if err != nil {
		return "", fmt.Errorf("getting cached ESV content (key=%s): %w", key, err)
	}
	return content, nil
}

// SaveCachedESV saves an ESV response to the cache.
func (s *Store) SaveCachedESV(ctx context.Context, key string, content string) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR REPLACE INTO esv_cache (reference, content) VALUES (?, ?)", key, content)
	if err != nil {
		return fmt.Errorf("saving to ESV cache (key=%s): %w", key, err)
	}
	return nil
}

// QueueEmail inserts a new email into the delivery queue.
func (s *Store) QueueEmail(ctx context.Context, email *store.QueuedEmail) error {
	query := `
		INSERT INTO queued_emails (user_id, recipient, subject, body_html, status, attempts, last_attempt_at, next_attempt_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := s.db.ExecContext(ctx, query,
		email.UserID, email.Recipient, email.Subject, email.BodyHTML,
		email.Status, email.Attempts, email.LastAttemptAt, email.NextAttemptAt,
	)
	if err != nil {
		return fmt.Errorf("queuing email: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	email.ID = id

	return nil
}
