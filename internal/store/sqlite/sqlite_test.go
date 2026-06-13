package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/bderrly/daily-soap/internal/store"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	schema := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		verification_token TEXT,
		timezone TEXT NOT NULL DEFAULT 'UTC',
		is_admin INTEGER DEFAULT 0 NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
		verified_at DATETIME
	);
	CREATE TABLE sessions (
		token TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);
	CREATE TABLE journal (
		user_id INTEGER NOT NULL,
		date TEXT NOT NULL,
		observation TEXT NOT NULL,
		application TEXT NOT NULL,
		prayer TEXT NOT NULL,
		selected_verses TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, date),
		FOREIGN KEY(user_id) REFERENCES users(id)
	);
	CREATE TABLE esv_cache (
		reference TEXT PRIMARY KEY,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE password_reset_tokens (
		token TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);
	CREATE TABLE queued_emails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		recipient TEXT NOT NULL,
		subject TEXT NOT NULL,
		body_html TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		attempts INTEGER NOT NULL DEFAULT 0,
		last_attempt_at DATETIME,
		next_attempt_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	CREATE INDEX idx_queued_emails_status_next_attempt ON queued_emails(status, next_attempt_at);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestStore_GetUserFromSession(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	// Setup: Create a user and a session
	_, err := db.Exec("INSERT INTO users (id, email, password_hash, timezone, verified_at) VALUES (1, 'test@example.com', 'hash', 'UTC', CURRENT_TIMESTAMP)")
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	token := "valid-token"
	expiresAt := time.Now().Add(1 * time.Hour)
	_, err = db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, 1, ?)", token, expiresAt)
	if err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}

	t.Run("Valid session", func(t *testing.T) {
		user, err := s.GetUserFromSession(ctx, token)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if user == nil || user.Email != "test@example.com" {
			t.Errorf("expected user test@example.com, got %v", user)
		}
	})

	t.Run("Expired session", func(t *testing.T) {
		expiredToken := "expired-token"
		_, err = db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, 1, ?)", expiredToken, time.Now().Add(-1*time.Hour))
		if err != nil {
			t.Fatalf("failed to insert expired session: %v", err)
		}

		user, err := s.GetUserFromSession(ctx, expiredToken)
		if err == nil {
			t.Error("expected error for expired session, got nil")
		}
		if user != nil {
			t.Errorf("expected nil user for expired session, got %v", user)
		}
	})
}

func TestStore_GetSOAPData(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, err := db.Exec("INSERT INTO users (id, email, password_hash, verified_at) VALUES (1, 'test@example.com', 'hash', CURRENT_TIMESTAMP)")
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	date := "2026-02-18"
	selectedVerses := []string{"Gen 1:1", "Gen 1:2"}
	versesJSON, _ := json.Marshal(selectedVerses)
	_, err = db.Exec("INSERT INTO journal (user_id, date, observation, application, prayer, selected_verses) VALUES (1, ?, 'obs', 'app', 'pry', ?)", date, string(versesJSON))
	if err != nil {
		t.Fatalf("failed to insert journal entry: %v", err)
	}

	t.Run("Existing SOAP data", func(t *testing.T) {
		data, err := s.GetSOAPData(ctx, 1, date)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if data.Observation != "obs" {
			t.Errorf("unexpected soap data: %+v", data)
		}
	})

	t.Run("Non-existent SOAP data", func(t *testing.T) {
		data, err := s.GetSOAPData(ctx, 1, "2000-01-01")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if data.Observation != "" {
			t.Errorf("expected empty soap data, got %+v", data)
		}
	})
}

func TestStore_SaveSOAPData(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, err := db.Exec("INSERT INTO users (id, email, password_hash, verified_at) VALUES (1, 'test@example.com', 'hash', CURRENT_TIMESTAMP)")
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	soapData := &store.SOAPData{
		Date:           "2026-02-18",
		Observation:    "new-obs",
		Application:    "new-app",
		Prayer:         "new-pry",
		SelectedVerses: []string{"John 3:16"},
	}

	err = s.SaveSOAPData(ctx, 1, soapData)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify update
	soapData.Observation = "updated-obs"
	err = s.SaveSOAPData(ctx, 1, soapData)
	if err != nil {
		t.Errorf("expected no error on update, got %v", err)
	}
}

func TestStore_UserOperations(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	t.Run("Create and Get User", func(t *testing.T) {
		err := s.CreateUser(ctx, "new@example.com", "hash", "token", "UTC")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		user, err := s.GetUserByEmail(ctx, "new@example.com")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if user.Email != "new@example.com" {
			t.Errorf("expected email new@example.com, got %s", user.Email)
		}
	})

	t.Run("Get Auth User", func(t *testing.T) {
		id, hash, verified, tz, err := s.GetAuthUser(ctx, "new@example.com")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if id == 0 || hash != "hash" || verified || tz != "UTC" {
			t.Errorf("unexpected auth user data: %d, %s, %v, %s", id, hash, verified, tz)
		}
	})

	t.Run("Update Password Hash", func(t *testing.T) {
		user, _ := s.GetUserByEmail(ctx, "new@example.com")
		err := s.UpdateUserPasswordHash(ctx, user.ID, "newhash")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		_, hash, _, _, _ := s.GetAuthUser(ctx, "new@example.com")
		if hash != "newhash" {
			t.Errorf("expected newhash, got %s", hash)
		}
	})

	t.Run("Update Password", func(t *testing.T) {
		user, _ := s.GetUserByEmail(ctx, "new@example.com")
		err := s.UpdateUserPassword(ctx, user.ID, "finalhash")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		_, hash, _, _, _ := s.GetAuthUser(ctx, "new@example.com")
		if hash != "finalhash" {
			t.Errorf("expected finalhash, got %s", hash)
		}
	})
}

func TestStore_SessionOperations(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 's@example.com', 'h')")

	t.Run("Create and Delete Expired Sessions", func(t *testing.T) {
		err := s.CreateSession(ctx, "token-1", 1, time.Now().Add(1*time.Hour))
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		err = s.CreateSession(ctx, "token-expired", 1, time.Now().Add(-1*time.Hour))
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		err = s.DeleteExpiredSessions(ctx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		// Verify token-1 exists, token-expired does not
		_, err = s.GetUserFromSession(ctx, "token-1")
		if err != nil {
			t.Errorf("expected token-1 to exist, got %v", err)
		}
		_, err = s.GetUserFromSession(ctx, "token-expired")
		if err == nil {
			t.Error("expected token-expired to be gone")
		}
	})
}

func TestStore_PasswordResetOperations(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 'p@example.com', 'h')")

	t.Run("Reset Token lifecycle", func(t *testing.T) {
		expires := time.Now().Add(1 * time.Hour).Round(time.Second)
		err := s.CreatePasswordResetToken(ctx, "reset-token", 1, expires)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		userID, exp, err := s.GetPasswordResetToken(ctx, "reset-token")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if userID != 1 || !exp.Equal(expires) {
			t.Errorf("expected userID 1 and expires %v, got %d and %v", expires, userID, exp)
		}

		err = s.DeletePasswordResetToken(ctx, "reset-token")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		_, _, err = s.GetPasswordResetToken(ctx, "reset-token")
		if err == nil {
			t.Error("expected error for deleted token")
		}
	})
}

func TestStore_CreateUser(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	err := s.CreateUser(ctx, "new@example.com", "hash", "token", "UTC")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestStore_UpdateUserTimezone(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 'tz@example.com', 'h')")
	err := s.UpdateUserTimezone(ctx, 1, "Asia/Tokyo")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	var tz string
	_ = db.QueryRow("SELECT timezone FROM users WHERE id = 1").Scan(&tz)
	if tz != "Asia/Tokyo" {
		t.Errorf("expected Asia/Tokyo, got %s", tz)
	}
}

func TestStore_ConfirmUser(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	t.Run("Standard User", func(t *testing.T) {
		_, _ = db.Exec("INSERT INTO users (id, email, password_hash, verification_token) VALUES (1, 'c@example.com', 'h', 'token123')")
		userID, email, err := s.ConfirmUser(ctx, "token123")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if userID != 1 {
			t.Errorf("expected userID 1, got %d", userID)
		}
		if email != "c@example.com" {
			t.Errorf("expected email c@example.com, got %s", email)
		}

		var verifiedAt sql.NullTime
		var isAdmin int
		err = db.QueryRow("SELECT verified_at, is_admin FROM users WHERE id = 1").Scan(&verifiedAt, &isAdmin)
		if err != nil {
			t.Fatalf("failed to query user: %v", err)
		}
		if !verifiedAt.Valid {
			t.Error("expected verified_at to be set, got null")
		}
		if isAdmin != 0 {
			t.Errorf("expected is_admin 0, got %d", isAdmin)
		}
	})

	t.Run("Admin User", func(t *testing.T) {
		adminEmail := "admin@example.com"
		t.Setenv("ADMIN_EMAIL", adminEmail)

		_, _ = db.Exec("INSERT INTO users (id, email, password_hash, verification_token) VALUES (2, ?, 'h', 'admin_token')", adminEmail)
		userID, email, err := s.ConfirmUser(ctx, "admin_token")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if userID != 2 {
			t.Errorf("expected userID 2, got %d", userID)
		}
		if email != adminEmail {
			t.Errorf("expected email %s, got %s", adminEmail, email)
		}

		var isAdmin int
		err = db.QueryRow("SELECT is_admin FROM users WHERE id = 2").Scan(&isAdmin)
		if err != nil {
			t.Fatalf("failed to query user: %v", err)
		}
		if isAdmin != 1 {
			t.Errorf("expected is_admin 1, got %d", isAdmin)
		}
	})
}

func TestStore_ESVCache(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	err := s.SaveCachedESV(ctx, "John 1:1", "In the beginning...")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	content, err := s.GetCachedESV(ctx, "John 1:1")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if content != "In the beginning..." {
		t.Errorf("expected content, got %s", content)
	}
}

func TestStore_QueueEmail(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 'u@example.com', 'h')")

	email := &store.QueuedEmail{
		UserID:    1,
		Recipient: "u@example.com",
		Subject:   "Test Subject",
		BodyHTML:  "<p>Hello</p>",
		Status:    "pending",
	}

	err := s.QueueEmail(ctx, email)
	if err != nil {
		t.Fatalf("QueueEmail failed: %v", err)
	}

	if email.ID == 0 {
		t.Error("expected email.ID to be set")
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM queued_emails").Scan(&count)
	if err != nil || count != 1 {
		t.Errorf("expected 1 email in queue, got %d (err: %v)", count, err)
	}
}

func TestStore_GetPendingEmails(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 'u@example.com', 'h')")

	now := time.Now().UTC()
	// Future email
	_, _ = db.Exec("INSERT INTO queued_emails (user_id, recipient, subject, body_html, status, next_attempt_at) VALUES (1, 'u@example.com', 'future', 'body', 'pending', ?)", now.Add(1*time.Hour))
	// Past email
	_, _ = db.Exec("INSERT INTO queued_emails (user_id, recipient, subject, body_html, status, next_attempt_at) VALUES (1, 'u@example.com', 'past', 'body', 'pending', ?)", now.Add(-1*time.Hour))
	// Sent email
	_, _ = db.Exec("INSERT INTO queued_emails (user_id, recipient, subject, body_html, status, next_attempt_at) VALUES (1, 'u@example.com', 'sent', 'body', 'sent', ?)", now.Add(-1*time.Hour))

	emails, err := s.GetPendingEmails(ctx, 10)
	if err != nil {
		t.Fatalf("GetPendingEmails failed: %v", err)
	}

	if len(emails) != 1 {
		t.Errorf("expected 1 pending email, got %d", len(emails))
	} else if emails[0].Subject != "past" {
		t.Errorf("expected subject 'past', got %s", emails[0].Subject)
	}
}

func TestStore_UpdateEmailStatus(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 'u@example.com', 'h')")
	_, _ = db.Exec("INSERT INTO queued_emails (id, user_id, recipient, subject, body_html, status, attempts) VALUES (1, 1, 'u@example.com', 'subject', 'body', 'pending', 0)")

	nextAttempt := time.Now().UTC().Add(2 * time.Hour)
	err := s.UpdateEmailStatus(ctx, 1, "failed", &nextAttempt)
	if err != nil {
		t.Fatalf("UpdateEmailStatus failed: %v", err)
	}

	var status string
	var attempts int
	var lastAttemptAt *time.Time
	var nextAttemptAt time.Time
	err = db.QueryRow("SELECT status, attempts, last_attempt_at, next_attempt_at FROM queued_emails WHERE id = 1").Scan(&status, &attempts, &lastAttemptAt, &nextAttemptAt)
	if err != nil {
		t.Fatalf("failed to query email: %v", err)
	}

	if status != "failed" {
		t.Errorf("expected status 'failed', got %s", status)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
	if lastAttemptAt == nil {
		t.Error("expected last_attempt_at to be set")
	}
	// SQLite driver might return time slightly different due to precision, but let's check it's around what we set
	if nextAttemptAt.Before(nextAttempt.Add(-1*time.Second)) || nextAttemptAt.After(nextAttempt.Add(1*time.Second)) {
		t.Errorf("expected next_attempt_at around %v, got %v", nextAttempt, nextAttemptAt)
	}
}

func TestStore_MarkEmailSent(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 'u@example.com', 'h')")
	_, _ = db.Exec("INSERT INTO queued_emails (id, user_id, recipient, subject, body_html, status) VALUES (1, 1, 'u@example.com', 'subject', 'body', 'pending')")

	err := s.MarkEmailSent(ctx, 1)
	if err != nil {
		t.Fatalf("MarkEmailSent failed: %v", err)
	}

	var status string
	var lastAttemptAt *time.Time
	err = db.QueryRow("SELECT status, last_attempt_at FROM queued_emails WHERE id = 1").Scan(&status, &lastAttemptAt)
	if err != nil {
		t.Fatalf("failed to query email: %v", err)
	}

	if status != "sent" {
		t.Errorf("expected status 'sent', got %s", status)
	}
	if lastAttemptAt == nil {
		t.Error("expected last_attempt_at to be set")
	}
}

func TestStore_AdminQueries(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)

	// Helper to insert user
	insertUser := func(id int, email string, createdAt, verifiedAt *time.Time) {
		cAt := now
		if createdAt != nil {
			cAt = *createdAt
		}
		var vAt interface{}
		if verifiedAt != nil {
			vAt = *verifiedAt
		}

		_, err := db.Exec(`INSERT INTO users (id, email, password_hash, created_at, verified_at)
			VALUES (?, ?, 'hash', ?, ?)`, id, email, cAt.Format("2006-01-02 15:04:05"), vAt)
		if err != nil {
			t.Fatalf("failed to insert user %d: %v", id, err)
		}
	}

	// Helper to insert journal
	insertJournal := func(userId int, date string, ts *time.Time) {
		tAt := now
		if ts != nil {
			tAt = *ts
		}
		_, err := db.Exec(`INSERT INTO journal (user_id, date, observation, application, prayer, timestamp)
			VALUES (?, ?, 'o', 'a', 'p', ?)`, userId, date, tAt.Format("2006-01-02 15:04:05"))
		if err != nil {
			t.Fatalf("failed to insert journal for user %d: %v", userId, err)
		}
	}

	// User 1: verified exactly 48h after creation (completed), active last 7 days
	c1 := now.Add(-10 * 24 * time.Hour)
	v1 := c1.Add(48 * time.Hour)
	insertUser(1, "u1@example.com", &c1, &v1)
	ts1 := now.Add(-3 * 24 * time.Hour) // active 3 days ago
	insertJournal(1, "2026-05-01", &ts1)

	// User 2: verified 48h + 1s after creation (failed), not active last 7 days
	c2 := now.Add(-10 * 24 * time.Hour)
	v2 := c2.Add(48*time.Hour + time.Second)
	insertUser(2, "u2@example.com", &c2, &v2)
	ts2 := now.Add(-8 * 24 * time.Hour) // active 8 days ago
	insertJournal(2, "2026-05-01", &ts2)

	// User 3: unverified and past 48h (failed), active last 7 days
	c3 := now.Add(-3 * 24 * time.Hour)
	insertUser(3, "u3@example.com", &c3, nil)
	ts3 := now.Add(-1 * time.Hour) // active 1 hour ago
	insertJournal(3, "2026-05-01", &ts3)

	// User 4: unverified and within 48h (neither), not active
	c4 := now.Add(-24 * time.Hour)
	insertUser(4, "u4@example.com", &c4, nil)

	// Test GetAdminStats
	stats, err := s.GetAdminStats(ctx)
	if err != nil {
		t.Fatalf("GetAdminStats failed: %v", err)
	}
	if stats.TotalUsers != 4 {
		t.Errorf("expected TotalUsers = 4, got %d", stats.TotalUsers)
	}
	if stats.CompletedWithinDeadline != 1 {
		t.Errorf("expected CompletedWithinDeadline = 1, got %d", stats.CompletedWithinDeadline)
	}
	if stats.FailedWithinDeadline != 2 {
		t.Errorf("expected FailedWithinDeadline = 2, got %d", stats.FailedWithinDeadline)
	}
	if stats.ActiveLast7Days != 2 {
		t.Errorf("expected ActiveLast7Days = 2, got %d", stats.ActiveLast7Days)
	}

	// Test GetAdminUserDirectory
	dir, err := s.GetAdminUserDirectory(ctx)
	if err != nil {
		t.Fatalf("GetAdminUserDirectory failed: %v", err)
	}
	if len(dir) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(dir))
	}

	// Should be ordered by creation date desc (User 4, User 3, User 1, User 2 (or 2 then 1))
	if dir[0].Email != "u4@example.com" {
		t.Errorf("expected first user to be u4, got %s", dir[0].Email)
	}
	if dir[1].Email != "u3@example.com" {
		t.Errorf("expected second user to be u3, got %s", dir[1].Email)
	}
	// u1 and u2 have same creation date, order is non-deterministic or stable based on insert, let's just check activity

	activeCount := 0
	for _, entry := range dir {
		if entry.ActiveLast7 {
			activeCount++
		}
	}
	if activeCount != 2 {
		t.Errorf("expected 2 active users, got %d", activeCount)
	}
}
