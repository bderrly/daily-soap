// Package store defines the data store interfaces and common types for the application.
package store

import (
	"context"
	"time"
)

// User represents a system user.
type User struct {
	ID         int64
	Email      string
	Timezone   string
	IsAdmin    bool
	CreatedAt  time.Time
	VerifiedAt *time.Time
}

// IsVerified returns true if the user's email has been verified.
func (u *User) IsVerified() bool {
	return u.VerifiedAt != nil
}

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

// SOAPData represents the SOAP journal entry.
type SOAPData struct {
	Date           string   `json:"date"`
	Observation    string   `json:"observation"`
	Application    string   `json:"application"`
	Prayer         string   `json:"prayer"`
	SelectedVerses []string `json:"selectedVerses"`
}

// AdminStats contains aggregated metrics for user administration.
type AdminStats struct {
	TotalUsers              int
	CompletedWithinDeadline int
	FailedWithinDeadline    int
	ActiveLast7Days         int
}

// AdminUserDirEntry represents a user record tailored for the administration directory.
type AdminUserDirEntry struct {
	Email       string
	IsAdmin     bool
	CreatedAt   time.Time
	VerifiedAt  *time.Time
	ActiveLast7 bool
}

// IsVerified returns true if the user's email has been verified.
func (e *AdminUserDirEntry) IsVerified() bool {
	return e.VerifiedAt != nil
}

// Store defines the interface for database operations.
type Store interface {
	ConfirmUser(ctx context.Context, token string) (userID int64, email string, err error) // returns userID, email
	CreatePasswordResetToken(ctx context.Context, token string, userID int64, expiresAt time.Time) error
	CreateSession(ctx context.Context, token string, userID int64, expiresAt time.Time) error
	CreateUser(ctx context.Context, email, passwordHash, token, timezone string) error
	DeleteExpiredSessions(ctx context.Context) error
	DeletePasswordResetToken(ctx context.Context, token string) error
	ExpungeCache(ctx context.Context, olderThan time.Duration, keepMax int) error
	GetAuthUser(ctx context.Context, email string) (id int64, passwordHash string, isVerified bool, timezone string, err error)
	GetCachedESV(ctx context.Context, key string) (string, error)
	GetPasswordResetToken(ctx context.Context, token string) (int64, time.Time, error) // returns userID, expiresAt
	GetPendingEmails(ctx context.Context, limit int) ([]*QueuedEmail, error)
	GetSOAPData(ctx context.Context, userID int64, dateStr string) (*SOAPData, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserFromSession(ctx context.Context, token string) (*User, error)
	MarkEmailSent(ctx context.Context, id int64) error
	QueueEmail(ctx context.Context, email *QueuedEmail) error
	SaveCachedESV(ctx context.Context, key string, content string) error
	SaveSOAPData(ctx context.Context, userID int64, soapData *SOAPData) error
	UpdateEmailStatus(ctx context.Context, id int64, status string, nextAttempt *time.Time) error
	UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error
	UpdateUserPasswordHash(ctx context.Context, userID int64, newHash string) error
	UpdateUserTimezone(ctx context.Context, userID int64, timezone string) error
	GetSOAPDataRange(ctx context.Context, userID int64, startDate string, endDate string) ([]*SOAPData, error)
	// GetAdminStats retrieves user administration and metric statistics.
	GetAdminStats(ctx context.Context) (*AdminStats, error)
	// GetAdminUserDirectory retrieves the list of users for the administrator directory.
	GetAdminUserDirectory(ctx context.Context) ([]*AdminUserDirEntry, error)
	// PromoteUserToAdmin upgrades a user to the admin role by email.
	PromoteUserToAdmin(ctx context.Context, email string) (int64, error)
}
