# Queued Emails Store Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `QueuedEmail` persistence layer in the `Store` interface and its SQLite implementation.

**Architecture:** Add a new `QueuedEmail` struct to the `store` package and update the `Store` interface with methods for managing the email queue. Implement these methods in the `sqlite` package using standard SQL queries.

**Tech Stack:** Go, SQLite

---

### Task 1: Update `internal/store/store.go`

**Files:**
- Modify: `internal/store/store.go`

- [ ] **Step 1: Add `QueuedEmail` struct**

```go
// QueuedEmail represents an email message in the delivery queue.
type QueuedEmail struct {
	ID            int64
	UserID        int64
	Recipient     string
	Subject       string
	BodyHTML      string
	Status        string
	Attempts      int
	LastAttemptAt *time.Time
	NextAttemptAt time.Time
}
```

- [ ] **Step 2: Update `Store` interface**

```go
type Store interface {
    // ... existing methods ...
	QueueEmail(ctx context.Context, email *QueuedEmail) error
	GetPendingEmails(ctx context.Context, limit int) ([]*QueuedEmail, error)
	UpdateEmailStatus(ctx context.Context, id int64, status string, nextAttempt *time.Time) error
	MarkEmailSent(ctx context.Context, id int64) error
}
```

- [ ] **Step 3: Run `gofumpt`**

Run: `gofumpt -w internal/store/store.go`

- [ ] **Step 4: Commit**

```bash
git add internal/store/store.go
git commit -m "store: add QueuedEmail struct and Store interface methods"
```

### Task 2: Update `internal/store/sqlite/sqlite_test.go` Schema

**Files:**
- Modify: `internal/store/sqlite/sqlite_test.go`

- [ ] **Step 1: Update `setupTestDB` schema**

Add the `queued_emails` table to the `schema` string in `setupTestDB`.

```go
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/store/sqlite/sqlite_test.go
git commit -m "test: update test schema to include queued_emails"
```

### Task 3: Implement `QueueEmail` in `internal/store/sqlite/sqlite.go`

**Files:**
- Modify: `internal/store/sqlite/sqlite.go`
- Test: `internal/store/sqlite/sqlite_test.go`

- [ ] **Step 1: Write failing test for `QueueEmail`**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go -run TestStore_QueueEmail`
Expected: FAIL (method not implemented)

- [ ] **Step 3: Implement `QueueEmail`**

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go -run TestStore_QueueEmail`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go
git commit -m "store/sqlite: implement QueueEmail"
```

### Task 4: Implement `GetPendingEmails` in `internal/store/sqlite/sqlite.go`

**Files:**
- Modify: `internal/store/sqlite/sqlite.go`
- Test: `internal/store/sqlite/sqlite_test.go`

- [ ] **Step 1: Write failing test for `GetPendingEmails`**

```go
func TestStore_GetPendingEmails(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 'u@example.com', 'h')")

	now := time.Now()
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go -run TestStore_GetPendingEmails`
Expected: FAIL

- [ ] **Step 3: Implement `GetPendingEmails`**

```go
// GetPendingEmails retrieves a list of emails that are pending and due for delivery.
func (s *Store) GetPendingEmails(ctx context.Context, limit int) ([]*store.QueuedEmail, error) {
	query := `
		SELECT id, user_id, recipient, subject, body_html, status, attempts, last_attempt_at, next_attempt_at
		FROM queued_emails
		WHERE status = 'pending' AND next_attempt_at <= ?
		ORDER BY next_attempt_at ASC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("querying pending emails: %w", err)
	}
	defer rows.Close()

	var emails []*store.QueuedEmail
	for rows.Next() {
		var e store.QueuedEmail
		err := rows.Scan(&e.ID, &e.UserID, &e.Recipient, &e.Subject, &e.BodyHTML, &e.Status, &e.Attempts, &e.LastAttemptAt, &e.NextAttemptAt)
		if err != nil {
			return nil, fmt.Errorf("scanning queued email: %w", err)
		}
		emails = append(emails, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return emails, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go -run TestStore_GetPendingEmails`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go
git commit -m "store/sqlite: implement GetPendingEmails"
```

### Task 5: Implement `UpdateEmailStatus` and `MarkEmailSent` in `internal/store/sqlite/sqlite.go`

**Files:**
- Modify: `internal/store/sqlite/sqlite.go`
- Test: `internal/store/sqlite/sqlite_test.go`

- [ ] **Step 1: Write failing test for `UpdateEmailStatus` and `MarkEmailSent`**

```go
func TestStore_UpdateEmailOperations(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO users (id, email, password_hash) VALUES (1, 'u@example.com', 'h')")
	_, _ = db.Exec("INSERT INTO queued_emails (id, user_id, recipient, subject, body_html, status) VALUES (1, 1, 'u@example.com', 's', 'b', 'pending')")

	t.Run("UpdateEmailStatus", func(t *testing.T) {
		next := time.Now().Add(1 * time.Hour)
		err := s.UpdateEmailStatus(ctx, 1, "failed", &next)
		if err != nil {
			t.Errorf("UpdateEmailStatus failed: %v", err)
		}

		var status string
		var attempts int
		err = db.QueryRow("SELECT status, attempts FROM queued_emails WHERE id = 1").Scan(&status, &attempts)
		if err != nil || status != "failed" || attempts != 1 {
			t.Errorf("unexpected status %s or attempts %d", status, attempts)
		}
	})

	t.Run("MarkEmailSent", func(t *testing.T) {
		err := s.MarkEmailSent(ctx, 1)
		if err != nil {
			t.Errorf("MarkEmailSent failed: %v", err)
		}

		var status string
		err = db.QueryRow("SELECT status FROM queued_emails WHERE id = 1").Scan(&status)
		if err != nil || status != "sent" {
			t.Errorf("expected status 'sent', got %s", status)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go -run TestStore_UpdateEmailOperations`
Expected: FAIL

- [ ] **Step 3: Implement `UpdateEmailStatus`**

```go
// UpdateEmailStatus updates the status and retry information for an email.
func (s *Store) UpdateEmailStatus(ctx context.Context, id int64, status string, nextAttempt *time.Time) error {
	query := `
		UPDATE queued_emails
		SET status = ?, attempts = attempts + 1, last_attempt_at = ?, next_attempt_at = ?
		WHERE id = ?
	`
	now := time.Now()
	_, err := s.db.ExecContext(ctx, query, status, now, nextAttempt, id)
	if err != nil {
		return fmt.Errorf("updating email status: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Implement `MarkEmailSent`**

```go
// MarkEmailSent marks an email as successfully sent.
func (s *Store) MarkEmailSent(ctx context.Context, id int64) error {
	query := `
		UPDATE queued_emails
		SET status = 'sent', last_attempt_at = ?
		WHERE id = ?
	`
	now := time.Now()
	_, err := s.db.ExecContext(ctx, now, id)
	if err != nil {
		return fmt.Errorf("marking email as sent: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -v internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go -run TestStore_UpdateEmailOperations`
Expected: PASS

- [ ] **Step 6: Final check - run all tests in the package**

Run: `go test -v ./internal/store/sqlite/...`

- [ ] **Step 7: Commit**

```bash
git add internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go
git commit -m "store/sqlite: implement UpdateEmailStatus and MarkEmailSent"
```
