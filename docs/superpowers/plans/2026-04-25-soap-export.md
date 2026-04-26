# SOAP Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a reliable, multi-recipient SOAP journal export system with HTML/Markdown download and HTML-via-email delivery.

**Architecture:** Use a persistent database-backed queue (`queued_emails`) for email delivery to ensure reliability with exponential backoff retries. The server handles export generation and orchestration between the store, scripture cache, and email system.

**Tech Stack:** Go (Standard Library, `html/template`), SQLite, Mailgun (via existing `internal/email`), Vanilla JS/CSS.

---

### Task 1: Database Migration

**Files:**
- Create: `internal/migrations/20260425000000_add_queued_emails.sql`

- [ ] **Step 1: Create the migration file**
Create the file with the following schema:
```sql
CREATE TABLE queued_emails (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    recipient TEXT NOT NULL,
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, sent, failed
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
git add internal/migrations/20260425000000_add_queued_emails.sql
git commit -m "db: add queued_emails table for export retries"
```

---

### Task 2: Store Interface & SQLite Implementation

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/sqlite.go`
- Test: `internal/store/sqlite/sqlite_test.go`

- [ ] **Step 1: Update `internal/store/store.go`**
Add the `QueuedEmail` struct and methods to the `Store` interface:
```go
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

// In Store interface:
QueueEmail(ctx context.Context, email *QueuedEmail) error
GetPendingEmails(ctx context.Context, limit int) ([]*QueuedEmail, error)
UpdateEmailStatus(ctx context.Context, id int64, status string, nextAttempt *time.Time) error
MarkEmailSent(ctx context.Context, id int64) error
```

- [ ] **Step 2: Implement methods in `internal/store/sqlite/sqlite.go`**
Implement the SQL queries for queuing, retrieving, and updating email tasks.

- [ ] **Step 3: Write tests in `internal/store/sqlite/sqlite_test.go`**
Verify that `QueueEmail` saves data and `GetPendingEmails` retrieves it correctly based on `next_attempt_at`.

- [ ] **Step 4: Commit**
```bash
git add internal/store/store.go internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go
git commit -m "store: implement queued_emails persistence"
```

---

### Task 3: Email Templates & Worker Logic

**Files:**
- Create: `internal/email/template.go`
- Create: `internal/email/worker.go`
- Modify: `internal/email/email.go`

- [ ] **Step 1: Create `internal/email/template.go`**
Define a template that uses inline CSS for the SOAP export.
```go
const ExportEmailTemplate = `
<div style="font-family: sans-serif; line-height: 1.6; max-width: 600px; margin: 0 auto; border: 1px solid #eee; padding: 20px;">
    <h1 style="border-bottom: 2px solid #333; padding-bottom: 10px;">SOAP Journal Entry</h1>
    <p><strong>Date:</strong> {{.Date}}</p>
    <div style="margin-top: 20px;">
        <h2 style="color: #555;">Scripture</h2>
        <div style="font-style: italic; background: #f9f9f9; padding: 15px; border-left: 5px solid #ccc;">{{.Scripture}}</div>
    </div>
    <div style="margin-top: 20px;">
        <h2 style="color: #555;">Observation</h2>
        <p>{{.Observation}}</p>
    </div>
    <div style="margin-top: 20px;">
        <h2 style="color: #555;">Application</h2>
        <p>{{.Application}}</p>
    </div>
    <div style="margin-top: 20px;">
        <h2 style="color: #555;">Prayer</h2>
        <p>{{.Prayer}}</p>
    </div>
</div>
`
```

- [ ] **Step 2: Implement `internal/email/worker.go`**
Implement `StartWorker(ctx context.Context, s store.Store, client *Client)`. Use a `time.Ticker` to poll `GetPendingEmails` every minute and call `client.send` for each.

- [ ] **Step 3: Update `internal/email/email.go`**
Add `QueueExportEmail(ctx context.Context, s store.Store, user *store.User, date string, recipients []string, body string) error`. It should create one `QueuedEmail` record per recipient.

- [ ] **Step 4: Commit**
```bash
git add internal/email/
git commit -m "email: implement background worker and inline templates"
```

---

### Task 4: Server Integration (API Handler)

**Files:**
- Modify: `internal/server/server.go`

- [ ] **Step 1: Implement `handleExport`**
Extract `date`, `format`, `method`, and `recipients` from JSON body. Fetch SOAP data and Scripture. Generate content using `internal/export`. If method is `download`, return file. If `email`, call `email.QueueExportEmail`.

- [ ] **Step 2: Register route and Start Worker**
In `Muxer()`, add `mux.HandleFunc("/export", authMiddleware(handleExport))`.
In `InitDB()`, call `go email.StartWorker(ctx, appStore, emailClient)`.

- [ ] **Step 3: Commit**
```bash
git add internal/server/server.go
git commit -m "server: add /export endpoint and start email worker"
```

---

### Task 5: Frontend UI

**Files:**
- Modify: `internal/server/web/index.html`
- Modify: `internal/server/web/style.css`
- Modify: `internal/server/web/app.js`

- [ ] **Step 1: Add Share Button and Modal to `index.html`**
Add a "Share" button next to the SOAP form. Add a `<dialog>` or `<div>` for the modal with inputs for format and a comma-separated list of emails.

- [ ] **Step 2: Style the modal in `style.css`**
Add styles for the modal overlay, container, and form elements.

- [ ] **Step 3: Implement interaction in `app.js`**
Handle click on Share button to open modal. On "Export" click, send `POST /export` with the selected options. Show a toast/message for success or failure.

- [ ] **Step 4: Commit**
```bash
git add internal/server/web/
git commit -m "ui: add share modal and export integration"
```
