package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"derrclan.com/moravian-soap/internal/store"
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
		is_verified INTEGER DEFAULT 0,
		verification_token TEXT,
		timezone TEXT NOT NULL DEFAULT 'UTC'
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
	_, err := db.Exec("INSERT INTO users (id, email, password_hash, is_verified, timezone) VALUES (1, 'test@example.com', 'hash', 1, 'UTC')")
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

	_, err := db.Exec("INSERT INTO users (id, email, password_hash, is_verified) VALUES (1, 'test@example.com', 'hash', 1)")
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

	_, err := db.Exec("INSERT INTO users (id, email, password_hash, is_verified) VALUES (1, 'test@example.com', 'hash', 1)")
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

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash, verification_token) VALUES (1, 'c@example.com', 'h', 'token123')")
	rows, err := s.ConfirmUser(ctx, "token123")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if rows != 1 {
		t.Errorf("expected 1 row affected, got %d", rows)
	}
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
