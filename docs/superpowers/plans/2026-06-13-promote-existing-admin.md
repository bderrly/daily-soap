# Promote Existing Admin User Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure any existing user matching `ADMIN_EMAIL` is promoted to admin on server startup.

**Architecture:** Add `PromoteUserToAdmin` to `store.Store` interface and `sqlite.Store`. Call it during `InitDB` in `server.go`.

**Tech Stack:** Go, SQLite, goose

---

### Task 1: Update Store Interface

**Files:**
- Modify: `internal/store/store.go`

- [ ] **Step 1: Declare PromoteUserToAdmin in Store interface**
  Add the declaration at the end of the `Store` interface.

  Code to add:
  ```go
  // PromoteUserToAdmin upgrades a user to the admin role by email.
  PromoteUserToAdmin(ctx context.Context, email string) error
  ```

- [ ] **Step 2: Commit Task 1**
  Run:
  ```bash
  git add internal/store/store.go
  git commit -m "store: add PromoteUserToAdmin to Store interface"
  ```

---

### Task 2: Implement Store Method and Test

**Files:**
- Modify: `internal/store/sqlite/sqlite.go`
- Test: `internal/store/sqlite/sqlite_test.go`

- [ ] **Step 1: Write failing unit test in sqlite_test.go**
  Add `TestStore_PromoteUserToAdmin` at the end of `internal/store/sqlite/sqlite_test.go`.

  Code to add:
  ```go
  func TestStore_PromoteUserToAdmin(t *testing.T) {
  	db := setupTestDB(t)
  	s := New(db)
  	ctx := context.Background()

  	// Insert standard user
  	_, err := db.Exec("INSERT INTO users (id, email, password_hash, verification_token, is_admin) VALUES (10, 'existing-user@example.com', 'hash', 'token', 0)")
  	if err != nil {
  		t.Fatalf("failed to insert user: %v", err)
  	}

  	// Promote with case-insensitive email
  	err = s.PromoteUserToAdmin(ctx, "EXISTING-user@EXAMPLE.com")
  	if err != nil {
  		t.Fatalf("failed to promote user: %v", err)
  	}

  	// Verify is_admin is 1
  	var isAdmin int
  	err = db.QueryRow("SELECT is_admin FROM users WHERE id = 10").Scan(&isAdmin)
  	if err != nil {
  		t.Fatalf("failed to query user: %v", err)
  	}
  	if isAdmin != 1 {
  		t.Errorf("expected is_admin 1, got %d", isAdmin)
  	}
  }
  ```

- [ ] **Step 2: Verify compilation fails or test fails**
  Run from `mise exec --` context:
  ```bash
  mise exec -- go test -v ./internal/store/sqlite -run TestStore_PromoteUserToAdmin
  ```
  Expected: Compile error because `sqlite.Store` doesn't implement `PromoteUserToAdmin` yet.

- [ ] **Step 3: Implement PromoteUserToAdmin in sqlite.go**
  Add the method implementation to `internal/store/sqlite/sqlite.go`.

  Code to add:
  ```go
  // PromoteUserToAdmin upgrades a user to the admin role by email.
  func (s *Store) PromoteUserToAdmin(ctx context.Context, email string) error {
  	_, err := s.db.ExecContext(ctx, "UPDATE users SET is_admin = 1 WHERE LOWER(email) = LOWER(?)", email)
  	if err != nil {
  		return fmt.Errorf("promoting user to admin: %w", err)
  	}
  	return nil
  }
  ```

- [ ] **Step 4: Verify test passes**
  Run:
  ```bash
  mise exec -- go test -v ./internal/store/sqlite -run TestStore_PromoteUserToAdmin
  ```
  Expected: PASS

- [ ] **Step 5: Run formatting and linting tools**
  Run:
  ```bash
  mise exec -- gofumpt -w internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go
  mise exec -- golangci-lint run ./internal/store/sqlite/...
  ```
  Expected: No lint errors, code properly formatted.

- [ ] **Step 6: Commit Task 2**
  Run:
  ```bash
  git add internal/store/sqlite/sqlite.go internal/store/sqlite/sqlite_test.go
  git commit -m "store/sqlite: implement PromoteUserToAdmin with tests"
  ```

---

### Task 3: Integrate into Server Startup

**Files:**
- Modify: `internal/server/server.go`

- [ ] **Step 1: Call PromoteUserToAdmin on startup**
  In `internal/server/server.go`, inside the `InitDB` function, call `PromoteUserToAdmin` if `ADMIN_EMAIL` is set.

  Code block modification inside `InitDB` after `appStore = sqlite.New(db)`:
  ```go
  	// Promote existing user matching ADMIN_EMAIL to admin if configured
  	adminEmail := os.Getenv("ADMIN_EMAIL")
  	if adminEmail != "" {
  		if err := appStore.PromoteUserToAdmin(ctx, adminEmail); err != nil {
  			slog.Error("failed to promote admin user", "error", err, "admin_email", adminEmail)
  		} else {
  			slog.Info("checked and promoted admin user", "admin_email", adminEmail)
  		}
  	}
  ```

- [ ] **Step 2: Verify all tests in the codebase pass**
  Run:
  ```bash
  mise exec -- go test ./...
  ```
  Expected: All tests pass.

- [ ] **Step 3: Run formatting and linting tools on server package**
  Run:
  ```bash
  mise exec -- gofumpt -w internal/server/server.go
  mise exec -- golangci-lint run ./internal/server/...
  ```
  Expected: No lint errors, code properly formatted.

- [ ] **Step 4: Commit Task 3**
  Run:
  ```bash
  git add internal/server/server.go
  git commit -m "server: promote existing admin user on DB initialization"
  ```
