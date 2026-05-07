# Design: Admin Notification on New User Verification

## Purpose
Send an email to an admin when a new user signs up and successfully verifies their account. The recipient email address should be configurable via an environment variable.

## Architecture & Data Flow

1. **Store Layer Modifications**:
   - `store.Store.ConfirmUser` signature will be updated to return the user's ID and email, rather than just the number of affected rows:
     `ConfirmUser(ctx context.Context, token string) (userID int64, email string, err error)`
   - `sqlite.Store.ConfirmUser` will be updated to use the `RETURNING id, email` clause in its `UPDATE` statement.
   - If the token is invalid (i.e., no rows are updated), the query will return `sql.ErrNoRows`. The method will catch this and return `0, "", nil` to allow the HTTP handler to gracefully display an "invalid token" message.

2. **HTTP Handler (`handleConfirm`)**:
   - Call the updated `appStore.ConfirmUser(..., token)`.
   - If `userID > 0`, the token is valid, and the user is confirmed.
   - Proceed to queue the admin notification.

3. **Queue Notification Email**:
   - Read the `ADMIN_EMAIL` environment variable.
   - If the variable is set and not empty, construct a notification email:
     - **Recipient**: The value of `ADMIN_EMAIL`
     - **Subject**: "New User Registration: [user email]"
     - **BodyHTML**: "A new user has registered and verified their account: [user email]"
   - Call `appStore.QueueEmail` to schedule the email for sending via the background worker.

## Error Handling
- If the `ADMIN_EMAIL` environment variable is not set, no email will be queued (silent bypass).
- If queuing the notification email fails, an error will be logged via `slog.Error`, but it will not prevent the user from seeing the success page for their verification.

## Testing
- Update any existing unit tests for `store.Store.ConfirmUser` to reflect the new signature and return values.
- Update tests for `handleConfirm` to ensure it still correctly handles valid and invalid tokens with the new `ConfirmUser` behavior.