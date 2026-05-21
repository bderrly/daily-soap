# Design: User Administration Page & Metrics

## Purpose
Add an administration page (`/admin`) to the application that displays user analytics and a directory. The system will securely allow the first user (the developer) to have the 'admin' role set, enforce a 48-hour registration deadline, and calculate key metrics such as:
1. Total number of users.
2. Number of users who completed registration within the 48-hour deadline.
3. Number of users who did not complete registration within the 48-hour deadline.
4. Number of users who have been active (created or modified at least one SOAP journal entry) in the previous seven days.

---

## Technical Approach

### 1. Database Schema Changes (Goose Migration)
A new Goose SQL migration (`internal/migrations/20260520000000_add_admin_and_deadlines.sql`) will be created:
* **Table Alteration**:
  ```sql
  -- +goose Up
  ALTER TABLE users ADD COLUMN is_admin INTEGER DEFAULT 0 NOT NULL;
  ALTER TABLE users ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL;
  ALTER TABLE users ADD COLUMN verified_at DATETIME;

  -- Backfill existing verified users to have verified_at match created_at
  -- so they are counted as verifying within the 48-hour deadline.
  UPDATE users SET verified_at = created_at WHERE is_verified = 1;

  -- +goose Down
  ALTER TABLE users DROP COLUMN is_admin;
  ALTER TABLE users DROP COLUMN created_at;
  ALTER TABLE users DROP COLUMN verified_at;
  ```

### 2. Secure Admin Auto-Promotion
* During the verification process in `ConfirmUser` (`internal/store/sqlite/sqlite.go`), we will update the `ConfirmUser` logic.
* First, we retrieve the user's email matching the token.
* If the user's email matches the configured `ADMIN_EMAIL` environment variable, we set `is_admin = 1` in the database.
* Along with setting `is_verified = 1`, we will record `verified_at = CURRENT_TIMESTAMP`.

### 3. Store Layer & Struct Updates
* Update the `store.User` struct in `internal/store/store.go` to include:
  ```go
  IsAdmin    bool
  CreatedAt  time.Time
  VerifiedAt *time.Time
  ```
* Define structs for holding metrics and user directory entries:
  ```go
  type AdminStats struct {
      TotalUsers         int
      CompletedWithin48h int
      FailedWithin48h    int
      ActiveLast7Days    int
  }

  type AdminUserDirEntry struct {
      Email       string
      IsAdmin     bool
      IsVerified  bool
      CreatedAt   time.Time
      VerifiedAt  *time.Time
      ActiveLast7 bool
  }
  ```
* Add the following methods to `store.Store`:
  ```go
  GetAdminStats(ctx context.Context) (*AdminStats, error)
  GetAdminUserDirectory(ctx context.Context) ([]*AdminUserDirEntry, error)
  ```

#### SQLite Queries
* **Total Users**:
  ```sql
  SELECT COUNT(*) FROM users;
  ```
* **Completed Registration within 48h**:
  ```sql
  SELECT COUNT(*) FROM users
  WHERE is_verified = 1
    AND verified_at <= datetime(created_at, '+48 hours');
  ```
* **Did Not Complete Registration within 48h** (Includes both unverified users whose 48-hour window has expired and verified users who verified late):
  ```sql
  SELECT COUNT(*) FROM users
  WHERE (is_verified = 0 AND created_at < datetime(CURRENT_TIMESTAMP, '-48 hours'))
     OR (is_verified = 1 AND verified_at > datetime(created_at, '+48 hours'));
  ```
* **Active in Previous 7 Days** (Users who created/updated journal entries in the last 7 days):
  ```sql
  SELECT COUNT(DISTINCT user_id) FROM journal
  WHERE timestamp >= datetime(CURRENT_TIMESTAMP, '-7 days');
  ```
* **User Directory**:
  ```sql
  SELECT
      u.email,
      u.is_admin,
      u.is_verified,
      u.created_at,
      u.verified_at,
      EXISTS (
          SELECT 1 FROM journal j
          WHERE j.user_id = u.id
            AND j.timestamp >= datetime(CURRENT_TIMESTAMP, '-7 days')
      ) AS active_last_7
  FROM users u
  ORDER BY u.created_at DESC;
  ```

### 4. HTTP Server Protection & Navigation
* **Admin Verification**: Add check in HTTP route handlers to ensure only authorized administrators can access the admin console.
* **Route Protection Middleware**:
  * Implement an admin check inside the server handlers. If the authenticated session user's `IsAdmin` field is false, return `403 Forbidden`.
* **Templates**:
  * Create `internal/server/web/admin.html` containing the styled dashboard layout.
  * Conditionally show the "Admin" link in `internal/server/web/head.gotmpl` or navigation menu if `User.IsAdmin` is true.

---

## Testing Plan
1. **Migration Testing**: Verify the migration applies successfully and cleanly rollbacks without data loss issues on SQLite.
2. **Store Logic Unit Tests**:
   * Test `ConfirmUser` auto-promotion to admin for emails matching `ADMIN_EMAIL`.
   * Test registration deadline status calculations and stats matching different user scenarios (verified early, verified late, unverified pending, unverified expired).
   * Test 7-day active user count logic with varying journal entry timestamps.
3. **Integration / HTTP Tests**:
   * Verify `/admin` returns `403 Forbidden` for non-logged-in users and standard users.
   * Verify `/admin` is fully accessible and displays correct statistics for authenticated admin users.
