# Design: Promote Existing Admin User on Startup

## Purpose
Ensure that any existing user whose email matches the `ADMIN_EMAIL` environment variable is promoted to the administrator role (`is_admin = 1`) on application startup. This addresses cases where users registered and verified before the administrator field was introduced and backfilled.

---

## Technical Approach

### 1. Store Interface Upgrade
We will add `PromoteUserToAdmin` to the `store.Store` interface in `internal/store/store.go` to support upgrading users to administrators by email.

### 2. SQLite Store Implementation
We will implement `PromoteUserToAdmin` in `internal/store/sqlite/sqlite.go`. It will perform a case-insensitive update:
```sql
UPDATE users SET is_admin = 1 WHERE LOWER(email) = LOWER(?)
```

### 3. Application Startup Integration
In `InitDB` in `internal/server/server.go`, immediately after initializing `appStore`, we will retrieve `ADMIN_EMAIL` from the environment. If it is configured, we will call `PromoteUserToAdmin`.

---

## Testing Plan
1. **Unit Tests**:
   - Add a unit test in `internal/store/sqlite/sqlite_test.go` that inserts a standard user, calls `PromoteUserToAdmin` with a case-insensitive email match, and asserts that the user's `is_admin` field becomes `1`.
2. **Integration / Run Tests**:
   - Run `go test ./...` to verify everything compiles and all tests pass.
